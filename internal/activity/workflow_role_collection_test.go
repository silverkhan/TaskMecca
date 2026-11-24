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
	facts := []LifecycleFact{{EventID: "r", TaskID: "A-85", Kind: "registered", OccurredAt: b}, {EventID: "wait", TaskID: "A-85", Kind: "waiting", Actor: "/root/controller", AttemptID: "controller-attempt", AssignmentID: "controller-assignment", OccurredAt: at(5), EvidenceSource: "controller_report"}, {EventID: "resume", TaskID: "A-85", Kind: "resumed", Actor: "/root/controller", AttemptID: "controller-attempt", AssignmentID: "controller-assignment", OccurredAt: at(8), EvidenceSource: "controller_report"}}
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
	facts := []LifecycleFact{{EventID: "r", TaskID: "A-85", Kind: "registered", Actor: "/root/registrar", OccurredAt: b}, {EventID: "a", TaskID: "A-85", Kind: "assigned", Actor: "/root/controller", OccurredAt: at(1)}, {EventID: "wait", TaskID: "A-85", Kind: "waiting", Actor: "/root/controller", AttemptID: "controller-attempt", AssignmentID: "controller-assignment", OccurredAt: at(2)}, {EventID: "resume", TaskID: "A-85", Kind: "resumed", Actor: "/root/controller", AttemptID: "controller-attempt", AssignmentID: "controller-assignment", OccurredAt: at(4)}}
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

func TestA85RepairWaitResumeRequiresExactIdentity(t *testing.T) {
	_, at := workflowFixture()
	for _, tc := range []struct {
		name      string
		change    func([]LifecycleFact) []LifecycleFact
		confirmed bool
	}{
		{"same_worker_attempt_assignment", nil, true},
		{"same_controller_attempt_assignment", func(f []LifecycleFact) []LifecycleFact {
			f[1].Actor = "/root/controller"
			f[2].Actor = f[1].Actor
			return f
		}, true},
		{"reviewer_cross_actor_repro", func(f []LifecycleFact) []LifecycleFact {
			f[1].Actor = "/root/controller"
			f[1].AttemptID = "controller-attempt"
			f[1].AssignmentID = ""
			f[2].AssignmentID = ""
			return f
		}, false},
		{"different_actor_same_ids", func(f []LifecycleFact) []LifecycleFact { f[2].Actor = "/root/controller/naong"; return f }, false},
		{"same_actor_different_attempt", func(f []LifecycleFact) []LifecycleFact { f[2].AttemptID = "another-native-episode"; return f }, false},
		{"same_actor_different_assignment", func(f []LifecycleFact) []LifecycleFact { f[2].AssignmentID = "another-assignment"; return f }, false},
		{"missing_wait_actor", func(f []LifecycleFact) []LifecycleFact { f[1].Actor = ""; return f }, false},
		{"missing_resume_actor", func(f []LifecycleFact) []LifecycleFact { f[2].Actor = ""; return f }, false},
		{"unknown_actor", func(f []LifecycleFact) []LifecycleFact { f[1].Actor = "opaque"; f[2].Actor = "opaque"; return f }, false},
		{"missing_wait_attempt", func(f []LifecycleFact) []LifecycleFact { f[1].AttemptID = ""; return f }, false},
		{"missing_resume_attempt", func(f []LifecycleFact) []LifecycleFact { f[2].AttemptID = ""; return f }, false},
		{"both_attempts_missing", func(f []LifecycleFact) []LifecycleFact { f[1].AttemptID = ""; f[2].AttemptID = ""; return f }, false},
		{"missing_wait_assignment", func(f []LifecycleFact) []LifecycleFact { f[1].AssignmentID = ""; return f }, false},
		{"missing_resume_assignment", func(f []LifecycleFact) []LifecycleFact { f[2].AssignmentID = ""; return f }, false},
		{"both_assignments_missing", func(f []LifecycleFact) []LifecycleFact { f[1].AssignmentID = ""; f[2].AssignmentID = ""; return f }, false},
		{"duplicate_wait_id", func(f []LifecycleFact) []LifecycleFact { return append(f, f[1]) }, false},
		{"duplicate_resume_id", func(f []LifecycleFact) []LifecycleFact { return append(f, f[2]) }, false},
		{"conflicting_wait_id", func(f []LifecycleFact) []LifecycleFact { c := f[1]; c.Actor = "/root/controller"; return append(f, c) }, false},
		{"foreign_task_resume", func(f []LifecycleFact) []LifecycleFact { f[2].TaskID = "A-86"; return f }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := []LifecycleFact{{EventID: "registered", TaskID: "A-85", Kind: "registered", OccurredAt: at(0)},
				{EventID: "wait", TaskID: "A-85", Kind: "waiting", Actor: "/root/controller/pairi", AttemptID: "worker-attempt", AssignmentID: "worker-assignment", OccurredAt: at(1), EvidenceSource: "worker_report"},
				{EventID: "resume", TaskID: "A-85", Kind: "resumed", Actor: "/root/controller/pairi", AttemptID: "worker-attempt", AssignmentID: "worker-assignment", OccurredAt: at(4), EvidenceSource: "worker_report"}}
			if tc.change != nil {
				facts = tc.change(facts)
			}
			w := SummarizeWorkflow(WorkflowInput{TaskID: "A-85", Facts: facts, Now: at(5)})
			global := false
			for _, g := range w.Unmeasured {
				if g.Status == "confirmed_wait" {
					global = true
					if g.Start != at(1) || g.End != at(4) {
						t.Fatal("wait boundary changed", g)
					}
				}
			}
			if global != tc.confirmed {
				t.Fatalf("confirmed_wait=%v want %v; gaps=%+v", global, tc.confirmed, w.Unmeasured)
			}
			for _, row := range w.Rows {
				for _, g := range row.Gaps {
					if !tc.confirmed && g.Status == "confirmed_wait" {
						t.Fatalf("invalid pair confirmed in %s: %+v", row.ID, g)
					}
				}
			}
			if tc.confirmed {
				found := false
				for _, row := range w.Rows {
					if row.AgentID == facts[1].Actor {
						for _, g := range row.Gaps {
							if g.Status == "confirmed_wait" {
								found = true
							}
						}
					}
				}
				if !found {
					t.Fatal("valid identity wait omitted from its own row")
				}
			}
			if w.Measurement.Activity.Millis != nil || w.Measurement.UnmeasuredMillis == nil || *w.Measurement.UnmeasuredMillis != 5*60000 {
				t.Fatal("wait pair became measured Activity", w.Measurement)
			}
		})
	}
}

func TestA85RepairWaitDoesNotCrossNativeAttemptRows(t *testing.T) {
	_, at := workflowFixture()
	facts := []LifecycleFact{{EventID: "registered", TaskID: "A-85", Kind: "registered", OccurredAt: at(0)},
		{EventID: "wait", TaskID: "A-85", Kind: "waiting", Actor: "/root/controller/pairi", AttemptID: "attempt-a", AssignmentID: "assignment-a", OccurredAt: at(1)},
		{EventID: "resume", TaskID: "A-85", Kind: "resumed", Actor: "/root/controller/pairi", AttemptID: "attempt-a", AssignmentID: "assignment-a", OccurredAt: at(4)}}
	attempts := []runtimeobs.Attempt{}
	for _, id := range []string{"a", "b"} {
		attempts = append(attempts, runtimeobs.Attempt{AttemptID: "attempt-" + id, TaskID: "A-85", AgentPath: "/root/controller/pairi", RuntimeAgentID: "native-" + id, SessionID: "native-" + id, Provider: "codex", BindingState: runtimeobs.BindingBound, BindingEvidence: map[string]string{"assignment_id": "assignment-" + id}, StartedAt: at(0).Format(time.RFC3339Nano), LastObservedAt: at(5).Format(time.RFC3339Nano), CurrentState: runtimeobs.StateRunning, StateEvidenceSource: runtimeobs.EvidenceHook, StateObservationQuality: runtimeobs.QualityObserved})
	}
	w := SummarizeWorkflow(WorkflowInput{TaskID: "A-85", Facts: facts, ExecutionObservations: attempts, Now: at(5)})
	foundA := false
	for _, r := range w.Rows {
		for _, g := range r.Gaps {
			if g.Status != "confirmed_wait" {
				continue
			}
			if r.AgentID == "native-b" {
				t.Fatal("logical name transferred a wait to another native attempt", g)
			}
			if r.AgentID == "native-a" {
				foundA = true
				if len(g.Evidence) < 2 || g.Evidence[1].AttemptID != "attempt-a" {
					t.Fatal("wait lost exact attempt provenance", g)
				}
			}
		}
	}
	if !foundA {
		t.Fatal("supported exact native attempt lost its wait")
	}
}

func TestA85RepairWaitPairRejectsChangedEndpointBoundaries(t *testing.T) {
	_, at := workflowFixture()
	a := LifecycleFact{EventID: "wait", TaskID: "A-85", Kind: "waiting", Actor: "/root/controller/pairi", AttemptID: "attempt", AssignmentID: "assignment", OccurredAt: at(1)}
	b := a
	b.EventID = "resume"
	b.Kind = "resumed"
	b.OccurredAt = at(4)
	v := WaitInterval{Kind: "explicit_wait", StartedAt: at(1), EndedAt: at(4), FromEventID: "wait", ToEventID: "resume"}
	for _, kind := range []string{"start_time", "end_time", "end_kind", "future_endpoint", "missing_endpoint"} {
		t.Run(kind, func(t *testing.T) {
			from, to := a, b
			now := at(5)
			switch kind {
			case "start_time":
				from.OccurredAt = at(2)
			case "end_time":
				to.OccurredAt = at(3)
			case "end_kind":
				to.Kind = "started"
			case "future_endpoint":
				now = at(3)
			case "missing_endpoint":
				to.EventID = "another-id"
			}
			_, reason := workflowWaitPair("A-85", v, map[string][]LifecycleFact{from.EventID: {from}, to.EventID: {to}}, now)
			if reason == "" {
				t.Fatal("unsupported endpoint accepted")
			}
		})
	}
}

func TestA100ParticipationAndExactOrigin(t *testing.T) {
	b, at := workflowFixture()
	events := []Event{{EventID: "start", TaskID: "A-100", Role: RoleWorker, AgentID: "native", RuntimeAttemptID: "run", Action: Start, Type: Implementation, OccurredAt: at(5), Explanation: "구현", ExplanationLanguage: "ko", ExplanationOrigin: "type_default"}, {EventID: "stop", TaskID: "A-100", Role: RoleWorker, AgentID: "native", RuntimeAttemptID: "run", Action: Stop, Type: Implementation, OccurredAt: at(10)}}
	w := SummarizeWorkflow(WorkflowInput{TaskID: "A-100", Events: events, Summary: Summarize("A-100", events), Facts: []LifecycleFact{{EventID: "registration", TaskID: "A-100", Kind: "registered", Actor: "/root/registrar", OccurredAt: b}}, Now: at(20)})
	var found bool
	for _, r := range w.Rows {
		if r.AgentID != "native" {
			continue
		}
		found = true
		if r.Participation == nil || !r.Participation.Start.Equal(at(5)) || !r.Participation.End.Equal(at(10)) {
			t.Fatal(r)
		}
		if len(r.Gaps) != 2 || r.Gaps[0].Status != "before_participation" || r.Gaps[1].Status != "after_participation" {
			t.Fatal(r.Gaps)
		}
		if r.Intervals[0].ExplanationOrigin != "type_default" {
			t.Fatal("origin inferred/lost", r.Intervals)
		}
	}
	if !found {
		t.Fatal("missing row")
	}
	// Explicit text equal to the default remains authored, legacy stays unknown.
	for _, origin := range []string{"written", ""} {
		events[0].ExplanationOrigin = origin
		w = SummarizeWorkflow(WorkflowInput{TaskID: "A-100", Events: events, Summary: Summarize("A-100", events), Now: at(20)})
		for _, r := range w.Rows {
			if r.AgentID == "native" && r.Intervals[0].ExplanationOrigin != origin {
				t.Fatal("origin guessed", r)
			}
		}
	}
}
