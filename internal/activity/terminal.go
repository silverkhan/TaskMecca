package activity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// RecordTerminal conservatively closes an Activity left open by a terminal
// Runtime Hook. It is called only on terminal state events, never every tool
// hook. The resulting interval is explicitly UNMEASURED (Abandon).
func RecordTerminal(project string, hook runtimeobs.ExecutionEvent) (bool, error) {
	if !hook.Terminal || hook.EventKind != "state" || hook.AttemptID == "" {
		return false, nil
	}
	// Most projects have no Activity records yet. One cheap stat avoids
	// rebuilding the expensive Runtime ledger for such terminal hooks.
	if _, statErr := os.Stat((Ledger{Project: project}).dir()); errors.Is(statErr, os.ErrNotExist) {
		return false, nil
	} else if statErr != nil {
		return false, statErr
	}
	when, err := time.Parse(time.RFC3339Nano, hook.ObservedAt)
	if err != nil {
		return false, nil
	}
	runtimeLedger, err := runtimeobs.BuildLedger(project, 5, time.Now())
	if err != nil {
		return false, err
	}
	var bound *runtimeobs.Attempt
	for i := range runtimeLedger.Attempts {
		item := &runtimeLedger.Attempts[i]
		if item.AttemptID != hook.AttemptID {
			continue
		}
		if bound != nil {
			return false, nil
		} // ambiguous
		bound = item
	}
	if bound == nil || !bound.Terminal || bound.BindingState != runtimeobs.BindingBound ||
		bound.TaskID == "" || bound.RuntimeAgentID == "" || bound.RuntimeAgentID != hook.RuntimeAgentID {
		return false, nil
	}
	// Native completion bounds execution, not successful phase work. Keep
	// collection arrival in RecordedAt and preserve the missing phase-end report.
	if bound.NativeReceipt != nil && bound.StateEvidenceSource == runtimeobs.EvidenceT3 && runtimeobs.HasObservedExecution(*bound) {
		r := bound.NativeReceipt
		if r.SessionID != bound.SessionID || r.Provider != bound.Provider || r.NativeAttemptID != bound.TurnID {
			return false, fmt.Errorf("%w: terminal native receipt mismatch", ErrUnverifiedActor)
		}
		completed, parseErr := time.Parse(time.RFC3339Nano, r.CompletedAt)
		if parseErr != nil || completed.After(when) {
			return false, fmt.Errorf("%w: terminal native completion unavailable", ErrUnverifiedActor)
		}
		when = completed
	}
	var role Role
	switch {
	case bound.AgentPath == "/root/controller":
		role = RoleController
	case bound.AgentPath == "/root/registrar":
		role = RoleRegistrar
	case strings.HasPrefix(bound.AgentPath, "/root/controller/sub-"):
		role = RoleSubController
	case rolepolicy.ReviewerPath(bound.AgentPath):
		role = RoleReviewer
	case strings.HasPrefix(bound.AgentPath, "/root/controller/"):
		role = RoleWorker
	default:
		return false, nil
	}
	store := Ledger{Project: project}
	rows, err := store.List(bound.TaskID)
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256([]byte("runtime-terminal\x00" + hook.AttemptID + "\x00" + hook.EventID))
	eventID := "terminal-" + hex.EncodeToString(sum[:12])
	if prior, readErr := store.Read(eventID); readErr == nil {
		return false, reconcileTerminalACK(project, prior)
	} else if !os.IsNotExist(readErr) {
		return false, readErr
	}
	var active *Event
	for i := len(rows) - 1; i >= 0; i-- {
		r := &rows[i]
		if r.Role == role && r.AgentID == bound.RuntimeAgentID && r.RuntimeAttemptID == hook.AttemptID && r.Action != Correct {
			if r.Action == Start || r.Action == Switch {
				active = r
			}
			break
		}
	}
	if active == nil || when.Before(active.OccurredAt) {
		return false, nil
	}
	event := Event{
		SchemaVersion: 1, EventID: eventID,
		TaskID: active.TaskID, Action: Abandon, Role: active.Role,
		AgentID: active.AgentID, RuntimeAttemptID: active.RuntimeAttemptID,
		AssignmentID: active.AssignmentID, HandoffID: active.HandoffID,
		Round: active.Round, Type: active.Type, Explanation: active.Explanation, ExplanationLanguage: active.ExplanationLanguage, ExplanationOrigin: active.ExplanationOrigin,
		Confidence: Unknown, Source: "runtime_terminal", Evidence: fmt.Sprintf("runtime event %s terminal state %s", hook.EventID, hook.State),
		OccurredAt: when, RecordedAt: time.Now().UTC(),
	}
	pendingWritten := false
	inserted, writeErr := store.ApplyVerified(event, func() error {
		fresh, err := runtimeobs.BuildLedger(project, 5, time.Now())
		if err != nil {
			return err
		}
		for _, attempt := range fresh.Attempts {
			if attempt.AttemptID == hook.AttemptID && attempt.Terminal && attempt.TaskID == event.TaskID &&
				attempt.RuntimeAgentID == event.AgentID && attempt.BindingState == runtimeobs.BindingBound {
				_, receiptErr := runtimeobs.RecordCollectionReceipt(project, terminalCollectionReceipt(event, "pending", ""))
				pendingWritten = receiptErr == nil
				return receiptErr
			}
		}
		return fmt.Errorf("%w: terminal attempt no longer verified", ErrUnverifiedActor)
	})
	if pendingWritten {
		status, problem := "ack", ""
		if writeErr != nil {
			status = "failed"
			problem = writeErr.Error()
		}
		_, receiptErr := runtimeobs.RecordCollectionReceipt(project, terminalCollectionReceipt(event, status, problem))
		writeErr = errors.Join(writeErr, receiptErr)
	}
	return inserted, writeErr
}

func terminalCollectionReceipt(e Event, status, problem string) runtimeobs.CollectionReceipt {
	raw, _ := json.Marshal(e)
	hash := sha256.Sum256(raw)
	return runtimeobs.CollectionReceipt{Key: "terminal-activity:" + e.TaskID + ":" + e.EventID, Status: status, At: time.Now().UTC(), Error: problem, Owner: "activity_terminal_collector", ResumeCondition: "Reconcile the exact immutable abandon event by content hash and retry the same native terminal event; retain missing phase-end evidence and original failure.", Activity: &runtimeobs.CollectionActivity{Role: string(e.Role), EventID: e.EventID, TaskID: e.TaskID, AgentID: e.AgentID, AttemptID: e.RuntimeAttemptID, PayloadSHA256: hex.EncodeToString(hash[:])}}
}
func reconcileTerminalACK(project string, e Event) error {
	rows, err := runtimeobs.ReadCollectionReceipts(project)
	if err != nil {
		return err
	}
	want := terminalCollectionReceipt(e, "ack", "")
	latest := ""
	for _, r := range rows {
		if r.Key != want.Key || r.Activity == nil {
			continue
		}
		if r.Activity.PayloadSHA256 != want.Activity.PayloadSHA256 {
			return errors.New("terminal collector content hash conflict")
		}
		latest = r.Status
	}
	if latest == "" || latest == "ack" {
		return nil
	}
	_, err = runtimeobs.RecordCollectionReceipt(project, want)
	return err
}
