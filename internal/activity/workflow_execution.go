package activity

import (
	"fmt"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// ReadWorkflowExecutions reads only persisted evidence. Unlike reconciliation it
// never scans provider sessions, repairs bindings, appends events or takes a
// monitor lock. A missing/stale Control Tower snapshot cannot hide terminal evidence.
func ReadWorkflowExecutions(project, task string, now time.Time) ([]runtimeobs.Attempt, error) {
	ledger, err := runtimeobs.BuildLedger(project, 0, now)
	if err != nil {
		return nil, err
	}
	for _, f := range ledger.Findings {
		if f.Code == "invalid_event_line" {
			return nil, fmt.Errorf("invalid runtime evidence: %s", f.Code)
		}
	}
	out := []runtimeobs.Attempt{}
	for _, a := range ledger.Attempts {
		if a.TaskID == task {
			out = append(out, a)
		}
	}
	return out, nil
}

// Resolve the exact attempt and binding, never a logical name or another run.
// Duplicate attempt candidates fail closed, including contradictory terminal/live states.
func workflowTerminalLookup(in WorkflowInput) func(string, string, string, string, time.Time) (time.Time, runtimeobs.EvidenceSource, bool) {
	attempts := map[string]runtimeobs.Attempt{}
	ambiguous := map[string]bool{}
	for _, a := range in.ExecutionObservations {
		if _, exists := attempts[a.AttemptID]; exists {
			ambiguous[a.AttemptID] = true
		}
		attempts[a.AttemptID] = a
	}
	return func(id, actor, native, assignment string, start time.Time) (time.Time, runtimeobs.EvidenceSource, bool) {
		a, ok := attempts[id]
		invalid := func() (time.Time, runtimeobs.EvidenceSource, bool) { return time.Time{}, "", false }
		if !ok || id == "" || ambiguous[id] || a.TaskID != in.TaskID || a.BindingState != runtimeobs.BindingBound || !a.Terminal || a.RuntimeAgentID == "" || a.SessionID == "" || a.Provider == "" || !runtimeobs.HasObservedExecution(a) {
			return invalid()
		}
		if (actor != "" && actor != a.AgentPath) || (native != "" && native != a.RuntimeAgentID) || (assignment != "" && assignment != a.BindingEvidence["assignment_id"]) {
			return invalid()
		}
		if boundNative := a.BindingEvidence["runtime_agent_id"]; boundNative != "" && boundNative != a.RuntimeAgentID {
			return invalid()
		}
		switch a.CurrentState {
		case runtimeobs.StateCompleted, runtimeobs.StateErrored, runtimeobs.StateInterrupted, runtimeobs.StateShutdown:
		default:
			return invalid()
		}
		end, valid := workflowTime(a.EndedAt)
		if !valid || end.Before(start) || end.After(in.Now) {
			return invalid()
		}
		if a.StateEvidenceSource == runtimeobs.EvidenceT3 {
			r := a.NativeReceipt
			if r == nil || r.ThreadID == "" || r.RunID == "" {
				return invalid()
			}
			completed, valid := workflowTime(r.CompletedAt)
			// Folded EndedAt is the observation boundary, not native CompletedAt.
			// Preserve that clock: do not backdate an Activity or invent phase time.
			observed, observedOK := workflowTime(r.ObservedAt)
			if !valid || !observedOK || completed.After(observed) || completed.After(end) || observed.After(in.Now) {
				return invalid()
			}
		}
		return end, a.StateEvidenceSource, true
	}
}
