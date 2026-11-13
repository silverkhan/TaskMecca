package activity

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
				for _, r := range reported.Rows {
					for _, v := range r.Intervals {
						if v.Kind == "execution_report" && (v.RuntimeTerminal == nil || !v.RuntimeTerminal.ObservedAt.Equal(at(10)) || !v.End.Equal(at(5))) {
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
