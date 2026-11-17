package activity

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"testing"
	"time"
)

// Every event here is a synthetic offline fixture, never production evidence.
func TestA85RoleFactsDoNotBecomeActivity(t *testing.T) {
	b, at := workflowFixture()
	events := []Event{{EventID: "w", TaskID: "A-85", Role: RoleWorker, AgentID: "native-worker", RuntimeAttemptID: "worker", Action: Start, Type: Implementation, OccurredAt: at(5)}, {EventID: "we", TaskID: "A-85", Role: RoleWorker, AgentID: "native-worker", RuntimeAttemptID: "worker", Action: Stop, Type: Implementation, OccurredAt: at(10)}}
	facts := []LifecycleFact{{EventID: "r", TaskID: "A-85", Kind: "registered", Actor: "/root/registrar", OccurredAt: b, EvidenceSource: "registrar_report"}, {EventID: "a", TaskID: "A-85", Kind: "assigned", Actor: "/root/controller", OccurredAt: at(2), EvidenceSource: "controller_report"}}
	w := SummarizeWorkflow(WorkflowInput{TaskID: "A-85", Events: events, Summary: Summarize("A-85", events), Facts: facts, Now: at(20)})
	exportWorkflowFixture(t, "a85-worker-only-with-role-facts.json", w, events)
	roles := map[string]bool{}
	for _, r := range w.Rows {
		roles[r.Role] = true
		if r.Role != "worker" {
			for _, v := range r.Intervals {
				if v.Measured || v.DurationMillis != nil || v.End != nil {
					t.Fatal("role fact became a duration", v)
				}
			}
		}
	}
	if !roles["registrar"] || !roles["controller"] {
		t.Fatalf("missing role facts: %+v", w.Rows)
	}
	if w.Reporting.Status == "recorded" {
		t.Fatal("Worker-only reports claimed complete role coverage")
	}
	if w.Measurement.AgentTimeMillis != 5*60000 || w.Measurement.Registrar.Millis != nil || w.Measurement.Controller.Millis != nil {
		t.Fatal("fabricated measured time", w.Measurement)
	}
}

func TestA85NativeAndWaitGapsRemainUnmeasured(t *testing.T) {
	b, at := workflowFixture()
	a := runtimeobs.Attempt{AttemptID: "registrar", TaskID: "A-85", AgentPath: "/root/registrar", RuntimeAgentID: "native-r", SessionID: "native-r", Provider: "codex", TurnID: "turn", BindingState: runtimeobs.BindingBound, StartedAt: at(2).Format(time.RFC3339Nano), LastObservedAt: at(4).Format(time.RFC3339Nano), CurrentState: runtimeobs.StateRunning, StateEvidenceSource: runtimeobs.EvidenceHook, StateObservationQuality: runtimeobs.QualityObserved}
	facts := []LifecycleFact{{EventID: "r", TaskID: "A-85", Kind: "registered", OccurredAt: b}, {EventID: "wait", TaskID: "A-85", Kind: "waiting", Actor: "/root/controller", OccurredAt: at(5), EvidenceSource: "controller_report"}, {EventID: "resume", TaskID: "A-85", Kind: "resumed", OccurredAt: at(8), EvidenceSource: "worker_report"}}
	w := SummarizeWorkflow(WorkflowInput{TaskID: "A-85", Facts: facts, ExecutionObservations: []runtimeobs.Attempt{a}, Now: at(10)})
	exportWorkflowFixture(t, "a85-native-wait-unmeasured.json", w, nil)
	data, _ := json.Marshal(w)
	var view map[string]any
	_ = json.Unmarshal(data, &view)
	statuses := map[string]bool{}
	for _, g := range view["unmeasured_intervals"].([]any) {
		status, _ := g.(map[string]any)["status"].(string)
		statuses[status] = true
	}
	if !statuses["confirmed_wait"] || !statuses["native_observed"] || !statuses["unmeasured"] {
		t.Fatalf("undifferentiated gaps: %s", data)
	}
	if w.Measurement.Activity.Millis != nil || w.Measurement.UnmeasuredMillis == nil || *w.Measurement.UnmeasuredMillis != 10*60000 {
		t.Fatal("native/wait became work", w.Measurement)
	}
	for _, r := range w.Rows {
		if r.Role == "registrar" {
			if len(r.Intervals) == 0 {
				t.Fatal("native role omitted")
			}
			return
		}
	}
	t.Fatal("verified Registrar execution omitted")
}

func TestA85FailurePointRetryOpenAndConcurrentRoles(t *testing.T) {
	b, at := workflowFixture()
	events := []Event{
		{EventID: "r1", TaskID: "A-85", Role: RoleRegistrar, AgentID: "r", RuntimeAttemptID: "r1", Action: Start, Type: Requirements, OccurredAt: at(2)},
		{EventID: "r2", TaskID: "A-85", Role: RoleRegistrar, AgentID: "r", RuntimeAttemptID: "r1", Action: Stop, Type: Requirements, OccurredAt: at(5)},
		{EventID: "c1", TaskID: "A-85", Role: RoleController, AgentID: "c", RuntimeAttemptID: "c1", Action: Start, Type: Recovery, OccurredAt: at(3)},
		{EventID: "c2", TaskID: "A-85", Role: RoleController, AgentID: "c", RuntimeAttemptID: "c1", Action: Stop, Type: Recovery, OccurredAt: at(7)},
		{EventID: "w1", TaskID: "A-85", Role: RoleWorker, AgentID: "w", RuntimeAttemptID: "w1", Action: Start, Type: Implementation, OccurredAt: at(8)},
	}
	in := WorkflowInput{TaskID: "A-85", Events: events, Summary: Summarize("A-85", events), Facts: []LifecycleFact{{EventID: "registered", TaskID: "A-85", Kind: "registered", OccurredAt: b}}, CollectionFailures: []WorkflowCollectionFailure{{ID: "failed-transport", TaskID: "A-85", Role: "registrar", AgentID: "r", At: at(1), Source: "fixture_transport_receipt", Reason: "original report failed; later successful retry preserves original failure"}}, Now: at(12)}
	w := SummarizeWorkflow(in)
	exportWorkflowFixture(t, "a85-failure-retry-concurrent-open.json", w, events)
	if w.Measurement.AgentTimeMillis != 7*60000 || w.Measurement.ObservedWallMillis != 5*60000 {
		t.Fatal("lost concurrent accounting", w.Measurement)
	}
	foundFailure, foundOpen := false, false
	for _, row := range w.Rows {
		for _, f := range row.RoleFacts {
			if f.Kind == "collection_failed" {
				foundFailure = true
				if f.At != at(1) || f.MeasurementStatus != "unmeasured" {
					t.Fatal("failure rewritten", f)
				}
			}
		}
		for _, v := range row.Intervals {
			if v.Kind == "activity_open" {
				foundOpen = true
				if v.End != nil || v.Measured || v.DurationMillis != nil {
					t.Fatal("open duration invented", v)
				}
			}
		}
	}
	if !foundFailure || !foundOpen {
		t.Fatal("lost failed point or open Activity")
	}
	if len(w.Rounds) != 1 {
		t.Fatal("retry or native role became a revision round")
	}
}

func TestA85WaitEvidenceDoesNotCrossRoleBoundary(t *testing.T) {
	b, at := workflowFixture()
	facts := []LifecycleFact{{EventID: "r", TaskID: "A-85", Kind: "registered", Actor: "/root/registrar", OccurredAt: b}, {EventID: "a", TaskID: "A-85", Kind: "assigned", Actor: "/root/controller", OccurredAt: at(1)}, {EventID: "wait", TaskID: "A-85", Kind: "waiting", Actor: "/root/controller", OccurredAt: at(2)}, {EventID: "resume", TaskID: "A-85", Kind: "resumed", Actor: "/root/controller", OccurredAt: at(4)}}
	w := SummarizeWorkflow(WorkflowInput{TaskID: "A-85", Facts: facts, Now: at(5)})
	for _, row := range w.Rows {
		if row.Role == "registrar" {
			for _, gap := range row.Gaps {
				if gap.Status == "confirmed_wait" {
					t.Fatal("Controller wait attributed to Registrar", gap)
				}
			}
		}
	}
}

func TestA85UnknownRoleIsNotWorker(t *testing.T) {
	b, at := workflowFixture()
	w := SummarizeWorkflow(WorkflowInput{TaskID: "A-85", Facts: []LifecycleFact{{EventID: "r", TaskID: "A-85", Kind: "registered", OccurredAt: b}, {EventID: "s", TaskID: "A-85", Kind: "started", Actor: "opaque", OccurredAt: at(1)}}, Now: at(2)})
	for _, r := range w.Rows {
		if r.AgentID == "opaque" && r.Role == "worker" {
			t.Fatal("unknown execution actor inferred Worker")
		}
	}
}
