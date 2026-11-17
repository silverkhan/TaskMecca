package runtimeobs

import (
	"context"
	"path/filepath"
	"strings"
	"time"
)

// CurrentExecutionIdentity must come from the current effective implementation
// policy and its durable assignment, never a display name or recent-use sort.
// This projection does not grant execution authority.
type CurrentExecutionIdentity struct {
	Project      string `json:"project"`
	TaskID       string `json:"task_id"`
	AgentPath    string `json:"agent_path"`
	NativeID     string `json:"native_id"`
	AssignmentID string `json:"assignment_id"`
	Provider     string `json:"provider"`
}

// CurrentExecutionSnapshot keeps observation freshness separate from task
// progress. Running is not proof of progress or absence of a human blocker.
type CurrentExecutionSnapshot struct {
	ImplementationAgentPath string                   `json:"implementation_agent_path,omitempty"`
	ReviewAgentPath         string                   `json:"review_agent_path,omitempty"`
	ReviewNativeID          string                   `json:"review_native_id,omitempty"`
	Identity                CurrentExecutionIdentity `json:"identity"`
	AttemptID               string                   `json:"attempt_id,omitempty"`
	SessionID               string                   `json:"session_id,omitempty"`
	RuntimeAgentID          string                   `json:"runtime_agent_id,omitempty"`
	TurnID                  string                   `json:"turn_id,omitempty"`
	Receipt                 *T3Receipt               `json:"receipt,omitempty"`
	State                   CanonicalState           `json:"state"`
	Source                  EvidenceSource           `json:"source,omitempty"`
	ObservedAt              string                   `json:"observed_at,omitempty"`
	ProgressAt              string                   `json:"progress_at,omitempty"`
	WaitReason              string                   `json:"wait_reason,omitempty"`
	Matched                 bool                     `json:"matched"`
	Fresh                   bool                     `json:"fresh"`
	OfficialCurrent         bool                     `json:"official_current"`
	FailedStage             string                   `json:"failed_stage,omitempty"`
	ActionOwner             string                   `json:"action_owner,omitempty"`
	Limitation              string                   `json:"limitation,omitempty"`
}

func executionDiagnostic(s CurrentExecutionSnapshot, stage, limitation string) CurrentExecutionSnapshot {
	s.Fresh, s.OfficialCurrent = false, false
	s.FailedStage, s.ActionOwner, s.Limitation = stage, "/root/controller", limitation
	return s
}

// EvaluateCurrentExecution is pure. A stored receipt is not a live certificate:
// callers refresh at their bounded observation/delivery boundary. More than one
// matching open episode is ambiguous, even when one was observed more recently.
func EvaluateCurrentExecution(id CurrentExecutionIdentity, attempts []Attempt, now time.Time) CurrentExecutionSnapshot {
	s := CurrentExecutionSnapshot{Identity: id, State: StateRuntimeUnknown}
	if !filepath.IsAbs(id.Project) || id.TaskID == "" || id.AgentPath == "" || id.NativeID == "" || id.AssignmentID == "" || id.Provider == "" {
		return executionDiagnostic(s, "implementation_identity", "current typed implementation native/provider and durable assignment required")
	}
	var candidates []Attempt
	for _, a := range attempts {
		if !strings.EqualFold(a.TaskID, id.TaskID) || a.AgentPath != id.AgentPath || a.BindingState != BindingBound || a.BindingEvidence["assignment_id"] != id.AssignmentID || a.Provider != id.Provider || a.SessionID != id.NativeID || a.RuntimeAgentID != id.NativeID {
			continue
		}
		candidates = append(candidates, a)
	}
	if len(candidates) == 0 {
		return executionDiagnostic(s, "attempt_selection", "no exact implementation attempt; reviewer, other native and old assignments cannot substitute")
	}
	var selected *Attempt
	open := 0
	for i := range candidates {
		a := &candidates[i]
		if !a.Terminal {
			selected = a
			open++
		}
	}
	if open > 1 {
		return executionDiagnostic(s, "attempt_selection", "multiple open exact episodes; current episode unresolved")
	}
	if open == 0 {
		// Terminal history is useful diagnostics. Only a fresh official read can
		// establish that the newest native boundary is still current.
		for i := range candidates {
			a := &candidates[i]
			at, err := time.Parse(time.RFC3339Nano, a.StartedAt)
			if err != nil || at.After(now) {
				return executionDiagnostic(s, "attempt_selection", "invalid execution boundary")
			}
			if selected == nil {
				selected = a
				continue
			}
			prior, _ := time.Parse(time.RFC3339Nano, selected.StartedAt)
			if at.Equal(prior) {
				return executionDiagnostic(s, "attempt_selection", "ambiguous terminal execution boundary")
			}
			if at.After(prior) {
				selected = a
			}
		}
	}
	a := *selected
	s.AttemptID, s.SessionID, s.RuntimeAgentID, s.TurnID = a.AttemptID, a.SessionID, a.RuntimeAgentID, a.TurnID
	s.State, s.Source, s.ObservedAt = a.CurrentState, a.StateEvidenceSource, a.LastObservedAt
	if a.NativeReceipt != nil {
		r := *a.NativeReceipt
		s.Receipt = &r
		s.ObservedAt = r.ObservedAt
	}
	if !HasObservedExecution(a) || a.AttemptID == "" || a.TurnID == "" {
		return executionDiagnostic(s, "execution_provenance", "exact attempt lacks observed execution provenance")
	}
	s.Matched = true
	if a.HookActivityCount > 0 {
		s.ProgressAt = a.LastHookActivityAt
	}
	if a.CurrentState == StateWaitingUser || a.CurrentState == StateWaitingApproval {
		for i := len(a.RecentTransitions) - 1; i >= 0; i-- {
			tr := a.RecentTransitions[i]
			if tr.State == a.CurrentState && (tr.ObservationQuality == QualityObserved || tr.ObservationQuality == QualityAuthoritative) {
				s.WaitReason = tr.Reason
				break
			}
		}
	}
	at, err := time.Parse(time.RFC3339Nano, s.ObservedAt)
	if err != nil || at.IsZero() || at.After(now) || now.Sub(at) > 2*time.Minute {
		return executionDiagnostic(s, "observation_freshness", "stored observation stale or invalid; recent use is not current execution")
	}
	s.Fresh = true
	s.Limitation = "stored observation only; official current run requires bounded refresh; running does not establish task progress"
	return s
}

// RefreshCurrentExecution makes at most one read, with the selected exact run
// pinned. It never appends runtime/Activity evidence, renames or updates usage.
func RefreshCurrentExecution(s CurrentExecutionSnapshot, now time.Time) CurrentExecutionSnapshot {
	return RefreshCurrentExecutionContext(context.Background(), s, now)
}

// RefreshCurrentExecutionContext inherits one caller deadline into the real
// SQLite child processes. No background reader survives a cancelled request.
func RefreshCurrentExecutionContext(ctx context.Context, s CurrentExecutionSnapshot, now time.Time) CurrentExecutionSnapshot {
	return refreshCurrentExecution(s, now, func(sel T3Selector, at time.Time) (T3Receipt, error) {
		if err := ctx.Err(); err != nil {
			return T3Receipt{}, err
		}
		store, err := localT3Store()
		if err != nil {
			return T3Receipt{}, err
		}
		store.ctx = ctx
		return store.read(sel, false, at)
	})
}

func refreshCurrentExecution(s CurrentExecutionSnapshot, now time.Time, read func(T3Selector, time.Time) (T3Receipt, error)) CurrentExecutionSnapshot {
	if !s.Matched || s.Receipt == nil || s.Source != EvidenceT3 || s.Receipt.ThreadID == "" || s.Receipt.RunID == "" {
		return executionDiagnostic(s, "official_observation", "supported official current execution selector unavailable; stored Hook state remains diagnostic")
	}
	r, err := read(T3Selector{ThreadID: s.Receipt.ThreadID, SessionID: s.SessionID, RuntimeAgentID: s.RuntimeAgentID, RunID: s.Receipt.RunID, NativeAttemptID: s.TurnID}, now)
	if err != nil {
		return executionDiagnostic(s, "official_observation", "official current execution read failed: "+err.Error())
	}
	at, e := time.Parse(time.RFC3339Nano, r.ObservedAt)
	if r.ThreadID != s.Receipt.ThreadID || r.RunID != s.Receipt.RunID || r.NativeAttemptID != s.TurnID || r.SessionID != s.Identity.NativeID || r.Provider != s.Identity.Provider || r.Origin != "t3_local_projection_v2" || e != nil || at.After(now) || now.Sub(at) > 2*time.Minute {
		return executionDiagnostic(s, "official_revalidation", "current native/run identity or observation conflict; refresh runtime through supported owner")
	}
	s.Receipt = &r
	s.State = r.State
	s.ObservedAt = r.ObservedAt
	s.Fresh, s.OfficialCurrent = true, true
	s.FailedStage, s.ActionOwner = "", ""
	s.Limitation = "official execution observation only; no inferred task progress, usage, Activity or authority"
	return s
}
