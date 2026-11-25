package activity

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestA102OpenPhaseDoesNotEndAtLastMilestone(t *testing.T) {
	b, at := workflowFixture()
	events := []Event{{EventID: "fixture-open", TaskID: "T", Role: RoleController, AgentID: "fixture-controller", RuntimeAttemptID: "fixture-attempt", Action: Start, Type: ControllerReview, OccurredAt: at(2)}}
	w := SummarizeWorkflow(WorkflowInput{TaskID: "T", Events: events, Summary: Summarize("T", events), Facts: []LifecycleFact{{EventID: "registration", TaskID: "T", Kind: "registered", OccurredAt: b}}, Now: at(10)})
	found := false
	for _, r := range w.Rows {
		if r.AgentID != "fixture-controller" {
			continue
		}
		found = true
		if r.Participation == nil || r.Participation.Status != "open_current_phase" || !r.Participation.End.Equal(at(10)) {
			t.Fatalf("open participation ended at point: %+v", r.Participation)
		}
		for _, g := range r.Gaps {
			if g.Status == "after_participation" {
				t.Fatal("running open phase declared absent", g)
			}
		}
	}
	if !found || w.Measurement.Controller.Millis != nil {
		t.Fatal("open phase lost or promoted to closed Activity", w)
	}
}

func TestA102NativeStateUsesNativeEndNotDelayedObservation(t *testing.T) {
	at := a100at
	a := runtimeobs.Attempt{BindingAt: at(0).Format(time.RFC3339Nano), AttemptID: "fixture-run", TaskID: "T", AgentPath: "/root/controller/fixture", RuntimeAgentID: "fixture-native", SessionID: "fixture-native", Provider: "codex", TurnID: "fixture-turn", BindingState: runtimeobs.BindingBound, BindingEvidence: map[string]string{"assignment_id": "fixture-assignment"}, StartedAt: at(5).Format(time.RFC3339Nano), EndedAt: at(10).Format(time.RFC3339Nano), LastObservedAt: at(11).Format(time.RFC3339Nano), CurrentState: runtimeobs.StateCompleted, Terminal: true, StateEvidenceSource: runtimeobs.EvidenceT3, StateObservationQuality: runtimeobs.QualityObserved, NativeReceipt: &runtimeobs.T3Receipt{Origin: "t3_local_projection_v2", ThreadID: "fixture-t3", RunID: "fixture-host-run", NativeAttemptID: "fixture-turn", SessionID: "fixture-native", Provider: "codex", State: runtimeobs.StateCompleted, StartedAt: at(0).Format(time.RFC3339Nano), CompletedAt: at(9).Format(time.RFC3339Nano), ObservedAt: at(11).Format(time.RFC3339Nano)}}
	w := SummarizeWorkflow(WorkflowInput{TaskID: "T", Summary: Summarize("T", nil), ExecutionObservations: []runtimeobs.Attempt{a}, Facts: []LifecycleFact{{EventID: "r", TaskID: "T", Kind: "registered", OccurredAt: at(0)}, {EventID: "c", TaskID: "T", Kind: "completed", OccurredAt: at(10)}}, Now: at(12)})
	if w.Coverage.StateCoveredMillis != 9000 || w.Coverage.UnaccountedStateMillis != 1000 || w.Measurement.Activity.Millis != nil {
		t.Fatalf("observation delay became state or phase duration: %+v", w.Coverage)
	}
	if a.StartedAt != at(5).Format(time.RFC3339Nano) || a.NativeReceipt.CompletedAt != at(9).Format(time.RFC3339Nano) {
		t.Fatal("source evidence mutated")
	}
}

func TestWorkflowTerminalProvenanceAndUnreportedActivity(t *testing.T) {
	b, at := workflowFixture()
	base := runtimeobs.Attempt{AttemptID: "attempt", TaskID: "T", AgentPath: "/root/controller/w", RuntimeAgentID: "native", SessionID: "native", Provider: "codex", TurnID: "turn", BindingState: runtimeobs.BindingBound, BindingEvidence: map[string]string{"assignment_id": "assignment"}, Terminal: true, CurrentState: runtimeobs.StateCompleted, EndedAt: at(10).Format(time.RFC3339Nano), LastObservedAt: at(11).Format(time.RFC3339Nano), StateEvidenceSource: runtimeobs.EvidenceT3, StateObservationQuality: runtimeobs.QualityObserved, NativeReceipt: &runtimeobs.T3Receipt{Origin: "t3_local_projection_v2", ThreadID: "thread", RunID: "run", NativeAttemptID: "turn", SessionID: "native", Provider: "codex", State: runtimeobs.StateCompleted, ObservedAt: at(11).Format(time.RFC3339Nano), CompletedAt: at(10).Format(time.RFC3339Nano)}}
	cases := []struct {
		name    string
		change  func(*runtimeobs.Attempt)
		trusted bool
	}{
		{"t3", func(*runtimeobs.Attempt) {}, true},
		{"t3ObservedAfterNativeCompletion", func(a *runtimeobs.Attempt) {
			r := *a.NativeReceipt
			r.CompletedAt = at(9).Format(time.RFC3339Nano)
			a.NativeReceipt = &r
		}, true},
		{"hook", func(a *runtimeobs.Attempt) { a.StateEvidenceSource = runtimeobs.EvidenceHook; a.NativeReceipt = nil }, true},
		{"missingTerminal", func(a *runtimeobs.Attempt) { a.Terminal = false }, false},
		{"differentAttempt", func(a *runtimeobs.Attempt) { a.AttemptID = "other" }, false},
		{"differentTask", func(a *runtimeobs.Attempt) { a.TaskID = "other" }, false},
		{"differentWorker", func(a *runtimeobs.Attempt) { a.RuntimeAgentID = "other" }, false},
		{"differentBinding", func(a *runtimeobs.Attempt) { a.BindingEvidence = map[string]string{"assignment_id": "other"} }, false},
		{"untrusted", func(a *runtimeobs.Attempt) { a.StateEvidenceSource = runtimeobs.EvidenceReconciled }, false},
		{"missingReceipt", func(a *runtimeobs.Attempt) { a.NativeReceipt = nil }, false},
		{"receiptWrongEnd", func(a *runtimeobs.Attempt) {
			r := *a.NativeReceipt
			r.CompletedAt = at(13).Format(time.RFC3339Nano)
			a.NativeReceipt = &r
		}, false},
		{"resume", func(a *runtimeobs.Attempt) { a.Terminal = false; a.CurrentState = runtimeobs.StateRunning }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := base
			c.change(&a)
			events := []Event{{EventID: "activity", TaskID: "T", Role: RoleWorker, AgentID: "native", RuntimeAttemptID: "attempt", AssignmentID: "assignment", Action: Start, Type: Requirements, OccurredAt: at(1)}}
			facts := []LifecycleFact{{EventID: "r", TaskID: "T", Kind: "registered", OccurredAt: b}, {EventID: "s", TaskID: "T", Kind: "started", Actor: "/root/controller/w", AttemptID: "attempt", AssignmentID: "assignment", OccurredAt: at(1)}}
			input := WorkflowInput{TaskID: "T", Events: events, Summary: Summarize("T", events), Facts: facts, ExecutionObservations: []runtimeobs.Attempt{a}, Now: at(12)}
			w := SummarizeWorkflow(input)
			found := 0
			for _, row := range w.Rows {
				for _, v := range row.Intervals {
					if v.Kind == "execution_report" {
						if (v.End != nil) != c.trusted {
							t.Fatalf("execution %+v", v)
						}
						if c.trusted && v.EndEvidenceSource != a.StateEvidenceSource {
							t.Fatalf("source %+v", v)
						}
						found++
					}
					if v.Kind == "activity_unreported" {
						if !c.trusted || v.Measured || v.DurationMillis != nil {
							t.Fatalf("invented phase end %+v", v)
						}
					}
					if c.trusted && v.Kind == "activity_open" {
						t.Fatal("terminal Activity still shown ongoing")
					}
				}
			}
			if found != 1 || w.Measurement.Worker.Millis != nil || input.Summary.OpenActivities != 1 {
				t.Fatalf("measurements changed %+v", w)
			}
			if c.trusted && w.Current.Phase != "execution_ended" {
				t.Fatalf("still running %+v", w.Current)
			}
			if !c.trusted && w.Current.Phase != "implementation" {
				t.Fatalf("false end %+v", w.Current)
			}
			if c.trusted {
				input.Handoffs = map[string]handoffjournal.Handoff{"done": {TaskID: "T", EventType: handoffjournal.EventWorkerDone, SourceAttemptID: "attempt", PreparedAt: at(5).Format(time.RFC3339Nano)}}
				reported := SummarizeWorkflow(input)
				terminalObserved := at(10)
				if a.NativeReceipt != nil {
					terminalObserved, _ = workflowTime(a.NativeReceipt.ObservedAt)
				}
				for _, r := range reported.Rows {
					for _, v := range r.Intervals {
						if v.Kind == "execution_report" && (v.RuntimeTerminal == nil || !v.RuntimeTerminal.ObservedAt.Equal(terminalObserved) || !v.End.Equal(at(5))) {
							t.Fatalf("handoff hid terminal or changed report boundary %+v", v)
						}
					}
				}
				input.Handoffs = nil
			}
			// A later resume/replacement must not inherit an earlier execution end.
			input.Facts = append(input.Facts, LifecycleFact{EventID: "resume", TaskID: "T", Kind: "resumed", Actor: "/root/controller/w", AttemptID: "replacement", OccurredAt: at(11)})
			input.Events = nil
			input.Summary = Summarize("T", nil)
			resumed := SummarizeWorkflow(input)
			if resumed.Current.Phase == "execution_ended" {
				t.Fatal("old terminal ended replacement")
			}
			input.Facts = input.Facts[:2]
			input.ExecutionObservations = append(input.ExecutionObservations, a)
			duplicate := SummarizeWorkflow(input)
			for _, r := range duplicate.Rows {
				for _, v := range r.Intervals {
					if v.End != nil {
						t.Fatal("ambiguous attempt accepted")
					}
				}
			}
		})
	}
}

func TestReadWorkflowExecutionsInvalidatesRuntimeCacheWithoutWrites(t *testing.T) {
	project := t.TempDir()
	b, at := workflowFixture()
	path := runtimeobs.ExecutionJournalPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	start := runtimeobs.ExecutionEvent{EventID: "start", EventKind: "state", AttemptID: "attempt", TaskID: "T", AgentPath: "/root/controller/w", RuntimeAgentID: "native", Provider: "codex", SessionID: "session", State: runtimeobs.StateRunning, ObservedAt: b.Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}
	raw, _ := json.Marshal(start)
	if err := os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	binding := start
	binding.EventID, binding.EventKind, binding.BindingSource = "bind", "binding", "dispatch"
	binding.BindingEvidence = map[string]string{"assignment_id": "assignment"}
	bindingRaw, _ := json.Marshal(binding)
	bf, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	bf.Write(append(bindingRaw, '\n'))
	bf.Close()
	first, err := ReadWorkflowExecutions(project, "T", at(12))
	if err != nil || len(first) != 1 || first[0].Terminal {
		t.Fatalf("initial %+v %v", first, err)
	}
	end := start
	end.EventID = "end"
	end.State = runtimeobs.StateCompleted
	end.Terminal = true
	end.ObservedAt = at(10).Format(time.RFC3339Nano)
	raw, _ = json.Marshal(end)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write(append(raw, '\n'))
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	second, err := ReadWorkflowExecutions(project, "T", at(12))
	if err != nil || len(second) != 1 || !second[0].Terminal {
		t.Fatalf("stale %+v %v", second, err)
	}
	again, err := ReadWorkflowExecutions(project, "T", at(12))
	if err != nil || len(again) != 1 || !again[0].Terminal {
		t.Fatal("non-idempotent read")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("read wrote runtime journal")
	}
	if _, err := os.Stat((Ledger{Project: project}).dir()); !os.IsNotExist(err) {
		t.Fatal("read created Activity ledger")
	}
}

func TestA102LateBindingNeverBackfillsTaskStateFromNativeLifetime(t *testing.T) {
	_, at := workflowFixture()
	for _, binding := range []string{at(6).Format(time.RFC3339Nano), "", at(0).Format(time.RFC3339Nano)} {
		a := a97Attempt("late", "native", "/root/controller/pairi")
		a.StateEvidenceSource = runtimeobs.EvidenceT3
		a.TurnID = "native-attempt"
		a.BindingAt = binding
		a.StartedAt = at(7).Format(time.RFC3339Nano)
		a.FirstObservedAt = a.StartedAt
		a.LastObservedAt = at(8).Format(time.RFC3339Nano)
		a.NativeReceipt = &runtimeobs.T3Receipt{Origin: "t3_local_projection_v2", Provider: a.Provider, SessionID: a.SessionID, ThreadID: "actual-thread", RunID: "actual-run", NativeAttemptID: a.TurnID, State: a.CurrentState, StartedAt: at(1).Format(time.RFC3339Nano), ObservedAt: a.LastObservedAt}
		in := WorkflowInput{TaskID: "T", Facts: []LifecycleFact{{EventID: "registered", TaskID: "T", Kind: "registered", OccurredAt: at(0)}}, ExecutionObservations: []runtimeobs.Attempt{a}, Now: at(10)}
		w := SummarizeWorkflow(in)
		workflowDiagnoseCoverage(&w)
		want := int64(120000)
		if binding == "" {
			want = 60000
		}
		if binding == at(0).Format(time.RFC3339Nano) {
			want = 420000
		}
		if w.Coverage.StateCoveredMillis != want || w.Coverage.ClosedActivityWallMillis != 0 {
			t.Fatalf("late native prefix became task work: binding=%s %+v", binding, w.Coverage)
		}
		if len(w.RuntimeTiming.Intervals) != 1 || !w.RuntimeTiming.Intervals[0].NativeStartedAt.Equal(at(1)) || w.RuntimeTiming.DurationMillis == nil || *w.RuntimeTiming.DurationMillis != 420000 {
			t.Fatal("raw native lifetime changed", w.RuntimeTiming)
		}
	}
}

func TestA102DelayedTerminalDisplayUsesNativeEndNotArrival(t *testing.T) {
	base := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	for _, mode := range []string{"forty_minute_delay", "subsecond492ms", "end_before_activity", "end_before_binding", "end_after_observer", "future", "foreign_native", "legacy_hook"} {
		t.Run(mode, func(t *testing.T) {
			nativeStart, phaseStart, nativeEnd, arrival, now := base, base.Add(30*time.Second), base.Add(time.Minute), base.Add(41*time.Minute), base.Add(42*time.Minute)
			a := runtimeobs.Attempt{AttemptID: "attempt", TaskID: "T", AgentPath: "/root/controller/w", RuntimeAgentID: "native", SessionID: "native", Provider: "codex", TurnID: "native-episode", BindingState: runtimeobs.BindingBound, BindingAt: base.Format(time.RFC3339Nano), BindingEvidence: map[string]string{"assignment_id": "assignment"}, CurrentState: runtimeobs.StateErrored, Terminal: true, StartedAt: base.Format(time.RFC3339Nano), EndedAt: arrival.Format(time.RFC3339Nano), LastObservedAt: arrival.Format(time.RFC3339Nano), StateEvidenceSource: runtimeobs.EvidenceT3, StateObservationQuality: runtimeobs.QualityObserved}
			switch mode {
			case "subsecond492ms":
				phaseStart = base.Add(100 * time.Millisecond)
				nativeEnd = base.Add(492 * time.Millisecond)
			case "end_before_activity":
				nativeEnd = base.Add(20 * time.Second)
			case "end_before_binding":
				a.BindingAt = base.Add(2 * time.Minute).Format(time.RFC3339Nano)
			case "end_after_observer":
				nativeEnd = arrival.Add(time.Second)
			case "future":
				nativeEnd = now.Add(time.Minute)
			}
			r := runtimeobs.T3Receipt{Origin: "t3_local_projection_v2", ThreadID: "actual-thread", RunID: "actual-run", NativeAttemptID: a.TurnID, SessionID: "native", Provider: "codex", State: a.CurrentState, StartedAt: nativeStart.Format(time.RFC3339Nano), CompletedAt: nativeEnd.Format(time.RFC3339Nano), ObservedAt: arrival.Format(time.RFC3339Nano)}
			a.NativeReceipt = &r
			if mode == "foreign_native" {
				r.SessionID = "foreign"
			}
			if mode == "legacy_hook" {
				a.StateEvidenceSource = runtimeobs.EvidenceHook
				a.NativeReceipt = nil
			}
			e := Event{EventID: "activity", TaskID: "T", Role: RoleWorker, AgentID: "native", RuntimeAttemptID: "attempt", AssignmentID: "assignment", Action: Start, Type: Implementation, OccurredAt: phaseStart, RecordedAt: phaseStart}
			in := WorkflowInput{TaskID: "T", Now: now, Events: []Event{e}, Summary: Summarize("T", []Event{e}), Facts: []LifecycleFact{{EventID: "registered", TaskID: "T", Kind: "registered", OccurredAt: base}, {EventID: "started", TaskID: "T", Kind: "started", Actor: a.AgentPath, AttemptID: a.AttemptID, AssignmentID: "assignment", OccurredAt: phaseStart}}, ExecutionObservations: []runtimeobs.Attempt{a}}
			before := runtimeobs.T3Receipt{}
			if a.NativeReceipt != nil {
				before = *a.NativeReceipt
			}
			w := SummarizeWorkflow(in)
			valid := mode == "forty_minute_delay" || mode == "subsecond492ms" || mode == "legacy_hook"
			found := false
			for _, row := range w.Rows {
				if !valid && mode != "end_before_activity" {
					for _, gap := range row.Gaps {
						if gap.Status == "native_observed" {
							t.Fatalf("invalid native end promoted state coverage: %+v", gap)
						}
					}
				}
				for _, v := range row.Intervals {
					if v.Kind != "activity_unreported" && v.Kind != "native_observed" {
						continue
					}
					if !valid {
						t.Fatalf("invalid terminal displayed: %+v", v)
					}
					if v.End == nil {
						t.Fatal("verified terminal missing display bound")
					}
					want := nativeEnd
					if mode == "legacy_hook" {
						want = arrival
					}
					if !v.End.Equal(want) || v.Measured || v.DurationMillis != nil {
						t.Fatalf("arrival promoted/display or measurement changed: %+v want %s", v, want)
					}
					if v.Kind == "activity_unreported" {
						found = true
						if v.RuntimeTerminal == nil || !v.RuntimeTerminal.ObservedAt.Equal(arrival) {
							t.Fatalf("arrival lost %+v", v)
						}
						if mode != "legacy_hook" && (v.RuntimeTerminal.SourceAt == nil || !v.RuntimeTerminal.SourceAt.Equal(nativeEnd)) {
							t.Fatalf("actual source end missing %+v", v)
						}
					}
				}
			}
			if valid && !found {
				t.Fatal("missing unreported Activity display")
			}
			if w.Measurement.ObservedWallMillis != 0 || in.Summary.OpenActivities != 1 {
				t.Fatal("missing stop promoted to measured work")
			}
			if a.NativeReceipt != nil && !reflect.DeepEqual(before, *a.NativeReceipt) {
				t.Fatal("raw native receipt changed")
			}
		})
	}
}
