package activity

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestB1NativeFirstActivityDelayRequiresExactEvidence(t *testing.T) {
	b, at := workflowFixture()
	a := runtimeobs.Attempt{AttemptID: "attempt", TaskID: "T", AgentPath: "/root/controller/w", RuntimeAgentID: "native", SessionID: "native", Provider: "codex", TurnID: "turn", LastObservedAt: at(11).Format(time.RFC3339Nano), BindingState: runtimeobs.BindingBound, BindingEvidence: map[string]string{"assignment_id": "assignment"}, CurrentState: runtimeobs.StateCompleted, Terminal: true, StateEvidenceSource: runtimeobs.EvidenceT3, StateObservationQuality: runtimeobs.QualityObserved, NativeReceipt: &runtimeobs.T3Receipt{Origin: "t3_local_projection_v2", ThreadID: "thread", RunID: "run", NativeAttemptID: "turn", SessionID: "native", Provider: "codex", State: runtimeobs.StateCompleted, StartedAt: b.Format(time.RFC3339Nano), CompletedAt: at(10).Format(time.RFC3339Nano), ObservedAt: at(11).Format(time.RFC3339Nano)}}
	first := Event{EventID: "first", TaskID: "T", Role: RoleWorker, AgentID: "native", RuntimeAttemptID: "attempt", AssignmentID: "assignment", Action: Start, Type: Research, OccurredAt: at(3), RecordedAt: at(3)}
	cases := []struct {
		name     string
		change   func(*Event)
		observed bool
		delay    int64
	}{
		{"exact", func(*Event) {}, true, 180000},
		{"zeroDelay", func(e *Event) { e.OccurredAt = b }, true, 0},
		{"wrongTask", func(e *Event) { e.TaskID = "other" }, false, 0},
		{"wrongNative", func(e *Event) { e.AgentID = "other" }, false, 0},
		{"wrongAttempt", func(e *Event) { e.RuntimeAttemptID = "other" }, false, 0},
		{"wrongAssignment", func(e *Event) { e.AssignmentID = "other" }, false, 0},
		{"wrongRole", func(e *Event) { e.Role = RoleReviewer }, false, 0},
		{"stopOnly", func(e *Event) { e.Action = Stop }, false, 0},
		{"beforeNative", func(e *Event) { e.OccurredAt = b.Add(-time.Minute) }, false, 0},
		{"afterTerminal", func(e *Event) { e.OccurredAt = at(11) }, false, 0},
		{"foreignProvider", func(e *Event) { e.Provider = "claude" }, false, 0},
		{"foreignSession", func(e *Event) { e.SessionID = "other" }, false, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := first
			c.change(&e)
			// An earlier unrelated event cannot win by timestamp alone.
			foreign := first
			foreign.EventID = "foreign"
			foreign.AgentID = "foreign"
			foreign.OccurredAt = at(1)
			in := WorkflowInput{TaskID: "T", Now: at(12), Events: []Event{foreign, e}, ExecutionObservations: []runtimeobs.Attempt{a}}
			out := summarizeRuntimeTiming(in, WorkflowAxis{}, LifecycleCoverage{})
			if len(out.Intervals) != 1 {
				t.Fatalf("native evidence lost %+v", out)
			}
			raw, err := json.Marshal(out.Intervals[0])
			if err != nil {
				t.Fatal(err)
			}
			var v map[string]any
			if err = json.Unmarshal(raw, &v); err != nil {
				t.Fatal(err)
			}
			if c.observed {
				if v["first_activity_event_id"] != e.EventID || v["first_activity_status"] != "observed" || v["first_activity_delay_ms"] != float64(c.delay) || v["first_activity_at"] != e.OccurredAt.Format(time.RFC3339Nano) {
					t.Fatalf("missing exact delay %+v", v)
				}
			} else if v["first_activity_status"] != "not_recorded" || v["first_activity_at"] != nil || v["first_activity_delay_ms"] != nil || v["first_activity_event_id"] != nil {
				t.Fatalf("invented first activity %+v", v)
			}
			if out.DurationMillis != nil || out.ReportedWaitOverlapMillis != 0 {
				t.Fatal("report delay became duration or wait")
			}
		})
	}
	first.SchemaVersion = 1
	first.Confidence = Confirmed
	correction := first
	correction.Action = Correct
	correction.EventID = "corrected"
	correction.SupersedesID = first.EventID
	correction.OccurredAt = at(4)
	correction.RecordedAt = at(5)
	corrected := summarizeRuntimeTiming(WorkflowInput{TaskID: "T", Now: at(12), Events: []Event{first, correction}, ExecutionObservations: []runtimeobs.Attempt{a}}, WorkflowAxis{}, LifecycleCoverage{})
	correctionRaw, _ := json.Marshal(corrected.Intervals[0])
	var correctionView map[string]any
	json.Unmarshal(correctionRaw, &correctionView)
	if correctionView["first_activity_event_id"] != "corrected" || correctionView["first_activity_delay_ms"] != float64(240000) {
		t.Fatalf("superseded report used %+v", correctionView)
	}

	// A running receipt may precede the first report. Its observed prefix stays unchanged.
	running := a
	running.Terminal = false
	running.CurrentState = runtimeobs.StateRunning
	r := *a.NativeReceipt
	r.State = runtimeobs.StateRunning
	r.CompletedAt = ""
	r.ObservedAt = at(1).Format(time.RFC3339Nano)
	running.NativeReceipt = &r
	in := WorkflowInput{TaskID: "T", Now: at(12), Events: []Event{first}, ExecutionObservations: []runtimeobs.Attempt{running}}
	out := summarizeRuntimeTiming(in, WorkflowAxis{Start: &b, End: &in.Now}, LifecycleCoverage{})
	if len(out.Intervals) != 1 {
		t.Fatalf("running native evidence lost %+v", out)
	}
	raw, _ := json.Marshal(out.Intervals[0])
	var v map[string]any
	json.Unmarshal(raw, &v)
	if v["first_activity_event_id"] != "first" || out.DurationMillis == nil || *out.DurationMillis != 60000 {
		t.Fatalf("running report changed native prefix %+v %+v", v, out)
	}
	in.Events = nil
	out = summarizeRuntimeTiming(in, WorkflowAxis{}, LifecycleCoverage{})
	raw, _ = json.Marshal(out.Intervals[0])
	// Reinitialize before checking optional fields: JSON omission must remain omission.
	v = map[string]any{}
	json.Unmarshal(raw, &v)
	if v["first_activity_status"] != "not_recorded" || v["first_activity_delay_ms"] != nil {
		t.Fatalf("missing report became zero %+v", v)
	}
}

func TestB1FirstActivityRequiresNonemptyAssignment(t *testing.T) {
	b, at := workflowFixture()
	for _, actor := range []struct {
		path string
		role Role
	}{
		{"/root/controller/w", RoleWorker},
		{"/root/controller/reviewer/pikachyu", RoleReviewer},
	} {
		for _, c := range []struct {
			name, binding, event string
			observed             bool
		}{
			{"bothMissing", "", "", false},
			{"bindingMissing", "", "assignment", false},
			{"eventMissing", "assignment", "", false},
			{"mismatch", "assignment", "other", false},
			{"exact", "assignment", "assignment", true},
		} {
			t.Run(string(actor.role)+"/"+c.name, func(t *testing.T) {
				a := runtimeobs.Attempt{AttemptID: "attempt", TaskID: "T", AgentPath: actor.path, RuntimeAgentID: "native", SessionID: "native", Provider: "codex", TurnID: "turn", LastObservedAt: at(11).Format(time.RFC3339Nano), BindingState: runtimeobs.BindingBound, BindingEvidence: map[string]string{"assignment_id": c.binding, "runtime_agent_id": "native"}, CurrentState: runtimeobs.StateCompleted, Terminal: true, StateEvidenceSource: runtimeobs.EvidenceT3, StateObservationQuality: runtimeobs.QualityObserved, NativeReceipt: &runtimeobs.T3Receipt{Origin: "t3_local_projection_v2", ThreadID: "thread", RunID: "run", NativeAttemptID: "turn", SessionID: "native", Provider: "codex", State: runtimeobs.StateCompleted, StartedAt: b.Format(time.RFC3339Nano), CompletedAt: at(10).Format(time.RFC3339Nano), ObservedAt: at(11).Format(time.RFC3339Nano)}}
				e := validEvent()
				e.EventID = "first"
				e.TaskID = "T"
				e.Role = actor.role
				e.AgentID = "native"
				e.RuntimeAttemptID = "attempt"
				e.AssignmentID = c.event
				e.OccurredAt = at(2)
				e.RecordedAt = at(3)
				if err := e.Validate(); err != nil {
					t.Fatal(err)
				}
				in := WorkflowInput{TaskID: "T", Now: at(12), Events: []Event{e}, ExecutionObservations: []runtimeobs.Attempt{a}}
				out := summarizeRuntimeTiming(in, WorkflowAxis{Start: &b, End: &in.Now}, LifecycleCoverage{})
				if len(out.Intervals) != 1 || out.DurationMillis == nil || *out.DurationMillis != 600000 || out.ReportedWaitOverlapMillis != 0 {
					t.Fatalf("assignment gap changed native timing %+v", out)
				}
				phase := out.Intervals[0]
				if c.observed {
					if phase.FirstActivityStatus != "observed" || phase.FirstActivityEventID != e.EventID || phase.FirstActivityAt == nil || phase.FirstActivityDelayMillis == nil || *phase.FirstActivityDelayMillis != 120000 {
						t.Fatalf("exact assignment rejected %+v", phase)
					}
				} else if phase.FirstActivityStatus != "not_recorded" || phase.FirstActivityAt != nil || phase.FirstActivityEventID != "" || phase.FirstActivityDelayMillis != nil {
					t.Fatalf("missing/mismatched assignment promoted %+v", phase)
				}
			})
		}
	}
}
