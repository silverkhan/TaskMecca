package activity

import (
	"errors"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"testing"
	"time"
)

func TestA72CurrentPhaseReportFailureRetryAndCompletion(t *testing.T) {
	p := t.TempDir()
	now := time.Date(2026, 11, 13, 3, 0, 0, 0, time.UTC)
	_, err := runtimeobs.RecordAssignment(p, "assignment", "A-72", "/root/controller/isanghaessi", now)
	if err != nil {
		t.Fatal(err)
	}
	if err = runtimeobs.AppendExecutionEvent(p, runtimeobs.ExecutionEvent{EventID: "start", EventKind: "state", AttemptID: "attempt", RuntimeAgentID: "native", SessionID: "native", Provider: "codex", ObservedAt: now.Format(time.RFC3339Nano), State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
		t.Fatal(err)
	}
	if _, err = runtimeobs.BindRuntimeAgentForAssignment(p, "assignment", "native", "", now); err != nil {
		t.Fatal(err)
	}
	for _, assignment := range []string{"other", "assignment"} {
		r := CheckReporting(p, "A-72", RoleWorker, "attempt", assignment, "", "implementation_started", now)
		if assignment == "other" && (r.Status != "unverified" || len(r.Arguments) != 0) {
			t.Fatalf("mismatch actionable %+v", r)
		}
		if assignment == "assignment" && (r.Status != "not_recorded" || len(r.Arguments) == 0) {
			t.Fatalf("current phase lost %+v", r)
		}
	}
	l := Ledger{Project: p}
	e := Event{SchemaVersion: 1, EventID: "current-implementation", TaskID: "A-72", Action: Start, Role: RoleWorker, AgentID: "native", RuntimeAttemptID: "attempt", AssignmentID: "assignment", Source: "agent_report", PhaseEvent: "implementation_started", Type: Implementation, Confidence: Inferred, Evidence: "current phase after binding, earlier requirements unmeasured", OccurredAt: now, RecordedAt: now}
	if _, err = l.ApplyVerified(e, func() error { return errors.New("report transport/storage failure") }); err == nil {
		t.Fatal("lost failure")
	}
	r := CheckReporting(p, "A-72", RoleWorker, "attempt", "assignment", "", "implementation_started", now)
	if r.Status != "not_recorded" {
		t.Fatalf("invented report after failure %+v", r)
	}
	if ok, err := l.Apply(e); err != nil || !ok {
		t.Fatalf("retry %v %v", ok, err)
	}
	if ok, err := l.Apply(e); err != nil || ok {
		t.Fatalf("duplicate report %v %v", ok, err)
	}
	if r = CheckReporting(p, "A-72", RoleWorker, "attempt", "assignment", "", "implementation_started", now); r.Status != "recorded" {
		t.Fatalf("receipt missing %+v", r)
	}
	if err = runtimeobs.AppendExecutionEvent(p, runtimeobs.ExecutionEvent{EventID: "end", EventKind: "state", AttemptID: "attempt", RuntimeAgentID: "native", SessionID: "native", Provider: "codex", ObservedAt: now.Add(time.Minute).Format(time.RFC3339Nano), State: runtimeobs.StateCompleted, Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
		t.Fatal(err)
	}
	r = CheckReporting(p, "A-72", RoleWorker, "attempt", "assignment", "", "targeted_test_started", now.Add(time.Minute))
	if r.Status != "not_recorded" || len(r.Arguments) != 0 || r.Diagnostic == "" {
		t.Fatalf("terminal backfill permitted %+v", r)
	}
}

func TestA72BindingWithoutExecutionCannotSuggestPhase(t *testing.T) {
	p := t.TempDir()
	now := time.Now().UTC()
	if _, err := runtimeobs.RecordAssignment(p, "assignment", "A-72", "/root/controller/isanghaessi", now); err != nil {
		t.Fatal(err)
	}
	a, err := runtimeobs.BindRuntimeAgentForAssignment(p, "assignment", "native", "", now)
	if err != nil {
		t.Fatal(err)
	}
	r := CheckReporting(p, "A-72", RoleWorker, a.AttemptID, "assignment", "", "requirements_started", now)
	if r.Status != "unverified" || len(r.Arguments) != 0 {
		t.Fatalf("binding became execution %+v", r)
	}
}

func TestA72LateSTARTKeepsRuntimeWaitAndActivityIndependent(t *testing.T) {
	b := time.Date(2026, 11, 13, 2, 0, 0, 0, time.UTC)
	at := func(m int) time.Time { return b.Add(time.Duration(m) * time.Minute) }
	a := runtimeobs.Attempt{AttemptID: "attempt", TaskID: "A-72", AgentPath: "/root/controller/isanghaessi", RuntimeAgentID: "native", SessionID: "native", Provider: "codex", TurnID: "turn", BindingState: runtimeobs.BindingBound, BindingEvidence: map[string]string{"assignment_id": "assignment"}, StartedAt: at(45).Format(time.RFC3339Nano), EndedAt: at(51).Format(time.RFC3339Nano), Terminal: true, CurrentState: runtimeobs.StateCompleted, LastObservedAt: at(51).Format(time.RFC3339Nano), StateEvidenceSource: runtimeobs.EvidenceT3, StateObservationQuality: runtimeobs.QualityObserved, NativeReceipt: &runtimeobs.T3Receipt{Origin: "t3_local_projection_v2", ThreadID: "thread", SessionID: "native", RunID: "run", NativeAttemptID: "turn", Provider: "codex", State: runtimeobs.StateCompleted, StartedAt: at(35).Format(time.RFC3339Nano), CompletedAt: at(49).Format(time.RFC3339Nano), ObservedAt: at(51).Format(time.RFC3339Nano)}}
	facts := []LifecycleFact{{EventID: "registered", TaskID: "A-72", Kind: "registered", OccurredAt: at(0)}, {EventID: "assigned", TaskID: "A-72", Kind: "assigned", OccurredAt: at(31)}, {EventID: "start-relay", TaskID: "A-72", Kind: "started", AttemptID: "attempt", AssignmentID: "assignment", Actor: a.AgentPath, OccurredAt: at(45)}, {EventID: "completed", TaskID: "A-72", Kind: "completed", OccurredAt: at(54)}}
	in := WorkflowInput{TaskID: "A-72", Facts: facts, Summary: Summarize("A-72", nil), ExecutionObservations: []runtimeobs.Attempt{a}, Now: at(55)}
	w := SummarizeWorkflow(in)
	if w.RuntimeTiming.DurationMillis == nil || *w.RuntimeTiming.DurationMillis != 14*60000 || w.RuntimeTiming.ReportedWaitOverlapMillis != 10*60000 {
		t.Fatalf("runtime provenance %+v", w.RuntimeTiming)
	}
	if w.Lifecycle.DispatchWaitMillis != 14*60000 || w.Measurement.Worker.Millis != nil || w.Measurement.UnmeasuredMillis == nil || *w.Measurement.UnmeasuredMillis != 54*60000 || *w.Measurement.CoveragePercent != 0 {
		t.Fatalf("fabricated phase/idle %+v %+v", w.Measurement, w.Lifecycle)
	}
	if w.Reporting.ExecutionsWithoutActivity != 1 {
		t.Fatalf("missing collection hidden %+v", w.Reporting)
	}
	// Duplicate relay cannot inflate expected Worker executions.
	in.Facts = append(in.Facts, LifecycleFact{EventID: "second-relay", TaskID: "A-72", Kind: "started", AttemptID: "attempt", AssignmentID: "assignment", Actor: a.AgentPath, OccurredAt: at(46)})
	if got := SummarizeWorkflow(in); got.Reporting.ExpectedExecutions != 1 {
		t.Fatalf("duplicate relay %+v", got.Reporting)
	}
	// A dropped lifecycle START still exposes exact bound runtime Activity gaps.
	in.Facts = []LifecycleFact{facts[0], facts[3]}
	if got := SummarizeWorkflow(in); got.Reporting.ExecutionsWithoutActivity != 1 {
		t.Fatalf("dropped START hides runtime gap %+v", got.Reporting)
	}
	for _, bad := range []string{"native", "other"} {
		copy := a
		receipt := *a.NativeReceipt
		copy.NativeReceipt = &receipt
		if bad == "native" {
			receipt.ThreadID = bad
		} else {
			receipt.SessionID = bad
		}
		in.ExecutionObservations = []runtimeobs.Attempt{copy}
		if got := SummarizeWorkflow(in); got.RuntimeTiming.DurationMillis != nil || got.RuntimeTiming.Rejected != 1 {
			t.Fatalf("mismatch trusted %+v", got.RuntimeTiming)
		}
	}
}
