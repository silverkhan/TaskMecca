package runtimeobs

import (
	"testing"
	"time"
)

func TestReconciledExecutionRequiresHookAndExactBinding(t *testing.T) {
	now := time.Now().UTC()
	a := Attempt{AttemptID: "a", RuntimeAgentID: "native", SessionID: "session", Provider: "codex", AgentPath: "/root/controller", BindingState: BindingBound, BindingSource: "explicit", BindingAt: now.Add(-time.Minute).Format(time.RFC3339Nano), StartedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), LastObservedAt: now.Format(time.RFC3339Nano), StateEvidenceSource: EvidenceReconciled, StateObservationQuality: QualityObserved, HookActivityCount: 1, LastHookActivityAt: now.Format(time.RFC3339Nano)}
	if !HasObservedExecution(a) {
		t.Fatal("real Hook-backed reconciled binding rejected")
	}
	for _, change := range []func(*Attempt){func(a *Attempt) { a.HookActivityCount = 0 }, func(a *Attempt) { a.LastHookActivityAt = "" }, func(a *Attempt) { a.BindingState = BindingAmbiguous }, func(a *Attempt) { a.BindingSource = "inferred" }, func(a *Attempt) { a.SessionID = "" }, func(a *Attempt) { a.BindingEvidence = map[string]string{"runtime_agent_id": "another"} }, func(a *Attempt) { a.Terminal = true }, func(a *Attempt) { a.LastObservedAt = now.Add(-time.Hour).Format(time.RFC3339Nano) }} {
		bad := a
		change(&bad)
		if HasObservedExecution(bad) {
			t.Fatalf("insufficient provenance accepted: %+v", bad)
		}
	}
}

func TestReconciledExecutionFoldRetainsActualHookActivity(t *testing.T) {
	project := t.TempDir()
	now := time.Now().UTC()
	err := AppendExecutionEvent(project, ExecutionEvent{EventID: "actual-hook", EventKind: "activity", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: "attempt", Provider: "codex", SessionID: "session", RuntimeAgentID: "native", EvidenceSource: EvidenceHook, ObservationQuality: QualityObserved})
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := BindAttempt(project, "attempt", "", "/root/controller", "explicit", "", nil, now.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if attempt.StateEvidenceSource != EvidenceReconciled || attempt.HookActivityCount != 1 || !HasObservedExecution(attempt) {
		t.Fatalf("fold lost exact Hook/binding link: %+v", attempt)
	}
}
