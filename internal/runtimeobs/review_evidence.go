package runtimeobs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

// ProductImplementationHistory ties persisted implementation observations to
// actual bound natives. It also includes prior recipients of the implementation
// owner path, so renaming a role or starting another turn cannot reset history.
// Historical implementation observations are retained even after corrections;
// correction of a display/event is not evidence that product work was undone.
func ProductImplementationHistory(project, task string, p *rolepolicy.Policy, now time.Time) ([]rolepolicy.Identity, error) {
	l, err := BuildLedger(project, 0, now)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(project, "_task_mecca", "data", "activity", "events")
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	implemented := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if len(raw) > 1<<20 {
			return nil, errors.New("implementation_history_event_too_large")
		}
		var e struct {
			TaskID       string `json:"task_id"`
			Type         string `json:"type"`
			AttemptID    string `json:"runtime_attempt_id"`
			AgentID      string `json:"agent_id"`
			AssignmentID string `json:"assignment_id"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, err
		}
		if e.Type != "implementation" {
			continue
		}
		for _, a := range l.Attempts {
			if a.TaskID == e.TaskID && a.AttemptID == e.AttemptID && a.RuntimeAgentID == e.AgentID && e.AssignmentID != "" && a.BindingEvidence["assignment_id"] == e.AssignmentID {
				implemented[a.AttemptID] = true
			}
		}
	}
	history := []rolepolicy.Identity{}
	for _, a := range l.Attempts {
		if a.TaskID == task {
			if implemented[a.AttemptID] || (p != nil && a.AgentPath == p.Implementation.AgentPath) {
				history = append(history, PolicyIdentity(a))
			}
			continue
		}
		// A reused native may have implemented these same source paths in another
		// task. Preserve the real originating task; filter by proven scope overlap.
		if p == nil || p.Reviewer.NativeID == "" || a.SessionID != p.Reviewer.NativeID || a.TaskID == "" || a.BindingState != BindingBound {
			continue
		}
		if !implemented[a.AttemptID] && !rolepolicy.WorkerPath(a.AgentPath) {
			continue
		}
		raw, err := rolepolicy.TaskMarkdown(project, a.TaskID)
		if err != nil {
			return nil, errors.New("reviewer_product_history_scope_unknown")
		}
		old, _, err := rolepolicy.Load(project, a.TaskID, raw)
		if err != nil || old == nil {
			return nil, errors.New("reviewer_product_history_scope_unknown")
		}
		if !implemented[a.AttemptID] && a.AgentPath != old.Implementation.AgentPath {
			continue
		}
		overlap := false
		for _, path := range old.Scope {
			if rolepolicy.Covers(p.Scope, path) {
				overlap = true
			}
		}
		for _, path := range p.Scope {
			if rolepolicy.Covers(old.Scope, path) {
				overlap = true
			}
		}
		if overlap {
			history = append(history, PolicyIdentity(a))
		}
	}

	return history, nil
}

func PolicyIdentity(a Attempt) rolepolicy.Identity {
	return rolepolicy.Identity{AttemptID: a.AttemptID, NativeID: a.SessionID, RuntimeAgentID: a.RuntimeAgentID, Provider: a.Provider, AgentPath: a.AgentPath, TaskID: a.TaskID, Bound: a.BindingState == BindingBound, Observed: a.SessionID != "" && a.RuntimeAgentID != "" && (a.StateEvidenceSource == EvidenceHook || a.StateEvidenceSource == EvidenceT3) && (a.StateObservationQuality == QualityObserved || a.StateObservationQuality == QualityAuthoritative)}
}

// ReadNativeReview reads the reviewer's actual final assistant result through
// the official provider interface. JSON files, role labels, Activity outcomes,
// and caller-supplied independent:true are not native review evidence.
// Other providers remain unknown until an equivalent adapter exists.
func ReadNativeReview(ctx context.Context, a Attempt, task, reportSHA string, now time.Time) (rolepolicy.ReviewStatement, error) {
	var zero rolepolicy.ReviewStatement
	identity := PolicyIdentity(a)
	if !identity.Bound || !identity.Observed || a.Provider != "codex" || a.SessionID == "" || a.TaskID != task {
		return zero, errors.New("review_native_identity_unverified_or_unsupported")
	}
	if a.StateEvidenceSource == EvidenceT3 {
		if e := VerifyT3Attempt(a, now); e != nil {
			return zero, e
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	c, e := openCodexMetadata(ctx)
	if e != nil {
		return zero, e
	}
	defer c.close()
	raw, e := c.call("thread/read", map[string]any{"threadId": a.SessionID, "includeTurns": true})
	if e != nil {
		return zero, e
	}
	return nativeReviewFromThread(raw, a.SessionID, task, reportSHA, a.AttemptID)
}

func nativeReviewFromThread(raw []byte, native, task, reportSHA, attempt string) (rolepolicy.ReviewStatement, error) {
	var payload struct {
		Thread struct {
			ID    string `json:"id"`
			Turns []struct {
				Status string `json:"status"`
				Items  []struct {
					Type  string `json:"type"`
					Text  string `json:"text"`
					Phase string `json:"phase"`
				} `json:"items"`
			} `json:"turns"`
		} `json:"thread"`
	}
	var result rolepolicy.ReviewStatement
	if e := json.Unmarshal(raw, &payload); e != nil {
		return result, e
	}
	if payload.Thread.ID != native {
		return result, errors.New("review_native_readback_mismatch")
	}
	count := 0
	for _, turn := range payload.Thread.Turns {
		if turn.Status != "completed" {
			continue
		}
		for _, item := range turn.Items {
			if item.Type != "agentMessage" || item.Phase != "final_answer" {
				continue
			}
			for _, line := range strings.Split(item.Text, "\n") {
				if !strings.HasPrefix(line, rolepolicy.ReviewMarker) {
					continue
				}
				var s rolepolicy.ReviewStatement
				if e := json.Unmarshal([]byte(strings.TrimPrefix(line, rolepolicy.ReviewMarker)), &s); e != nil {
					continue
				}
				if s.TaskID == task && s.ReportSHA256 == reportSHA && s.ReviewerAttemptID == attempt {
					result = s
					count++
				}
			}
		}
	}
	if count != 1 {
		return result, errors.New("exact_native_review_result_missing_or_ambiguous")
	}
	return result, nil
}
