package webui

import (
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// Setup/prior reception is not execution evidence for a particular attempt.
// The runtime fold owns provenance validation for both Hook and T3 receipts.
func operationObservedExecution(a runtimeobs.Attempt, now time.Time) bool {
	if !runtimeobs.HasObservedExecution(a) || a.BindingState == runtimeobs.BindingAmbiguous {
		return false
	}
	if a.StateEvidenceSource == runtimeobs.EvidenceT3 {
		r := a.NativeReceipt
		if r.ThreadID == "" || r.RunID == "" || r.NativeAttemptID == "" || r.SessionID == "" {
			return false
		}
		receiptAt, e := time.Parse(time.RFC3339Nano, r.ObservedAt)
		started, se := time.Parse(time.RFC3339Nano, r.StartedAt)
		if e != nil || se != nil || receiptAt.IsZero() || started.IsZero() || started.After(receiptAt) || receiptAt.After(now) {
			return false
		}
		if a.Terminal {
			completed, ce := time.Parse(time.RFC3339Nano, r.CompletedAt)
			ended, ee := time.Parse(time.RFC3339Nano, a.EndedAt)
			if ce != nil || ee != nil || completed.Before(started) || completed.After(receiptAt) || ended.Before(completed) {
				return false
			}
		}
	}
	observed, err := time.Parse(time.RFC3339Nano, a.LastObservedAt)
	if err != nil || observed.IsZero() || observed.After(now) {
		return false
	}
	for _, value := range []string{a.StartedAt, a.LastActivityAt, a.EndedAt} {
		if value == "" {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, value)
		if err != nil || at.IsZero() || at.After(observed) {
			return false
		}
	}
	if a.Terminal {
		end, err := time.Parse(time.RFC3339Nano, a.EndedAt)
		if err != nil || end.IsZero() {
			return false
		}
		if a.LastActivityAt != "" {
			activity, _ := time.Parse(time.RFC3339Nano, a.LastActivityAt)
			if activity.After(end) {
				return false
			}
		}
	}
	return true
}

func operationExecutionClock(a runtimeobs.Attempt) string {
	if a.StateEvidenceSource == runtimeobs.EvidenceT3 && a.NativeReceipt != nil {
		return a.NativeReceipt.ObservedAt
	}
	if a.LastActivityAt != "" {
		return a.LastActivityAt
	}
	return a.LastObservedAt
}

// Staleness is an observation gap, not an identity or contract conflict.
// Unscoped parse/corruption findings remain global blockers. Scoped conflicts
// block their exact attempt and task; unrelated historic findings do not.
func operationTaskBlocked(ledger runtimeobs.Ledger, task string) bool {
	for _, f := range ledger.Findings {
		if f.Severity == "info" {
			continue
		}
		if f.AttemptID == "" {
			return true
		}
		if f.Code == "stale" {
			continue
		}
		found := false
		for _, a := range ledger.Attempts {
			if a.AttemptID != f.AttemptID {
				continue
			}
			found = true
			if task != "" && strings.EqualFold(a.TaskID, task) {
				return true
			}
		}
		// An orphan finding cannot safely be assigned to an unrelated identity.
		if !found {
			return true
		}
	}
	return false
}
func operationAttemptBlocked(ledger runtimeobs.Ledger, a runtimeobs.Attempt) bool {
	if operationTaskBlocked(ledger, a.TaskID) {
		return true
	}
	for _, f := range ledger.Findings {
		if f.Severity != "info" && f.Code != "stale" && f.AttemptID == a.AttemptID {
			return true
		}
	}
	return a.BindingState == runtimeobs.BindingAmbiguous
}
func operationUnconflictedAttempts(ledger runtimeobs.Ledger) []runtimeobs.Attempt {
	out := []runtimeobs.Attempt{}
	for _, a := range ledger.Attempts {
		if !operationAttemptBlocked(ledger, a) {
			out = append(out, a)
		}
	}
	return out
}

func operationObservedEvidence(a runtimeobs.Attempt, completed bool) string {
	if a.StateEvidenceSource == runtimeobs.EvidenceT3 {
		if completed {
			return "verified structured execution completed state"
		}
		return "verified structured execution terminal state"
	}
	if completed {
		return "hook completed state"
	}
	return "hook terminal state"
}

func operationRecoveryEvidence(a runtimeobs.Attempt, completed bool) string {
	source := "Hook"
	if a.StateEvidenceSource == runtimeobs.EvidenceT3 {
		source = "execution"
	}
	if completed {
		return "same attempt has confirmed " + source + " completion; previous warning retired without changing runtime evidence"
	}
	return "same attempt has later observed " + source + " running state; previous warning retired without changing runtime evidence"
}

func operationIncidentIdentityMatches(item operationIncident, a runtimeobs.Attempt) bool {
	return (item.Provider == "" || item.Provider == a.Provider) &&
		(item.SessionID == "" || item.SessionID == a.SessionID) &&
		(item.RuntimeAgentID == "" || item.RuntimeAgentID == a.RuntimeAgentID) &&
		(item.TurnID == "" || item.TurnID == a.TurnID)
}
