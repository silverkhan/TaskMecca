package activity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func workflowFixture() (time.Time, func(int) time.Time) {
	b := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	return b, func(m int) time.Time { return b.Add(time.Duration(m) * time.Minute) }
}
func TestWorkflowUnmeasuredHistoryDoesNotBecomeZeroOrWork(t *testing.T) {
	b, at := workflowFixture()
	facts := []LifecycleFact{{EventID: "registered", TaskID: "A-1", Kind: "registered", OccurredAt: b}}
	handoffs := map[string]handoffjournal.Handoff{}
	for i := 0; i < 5; i++ {
		facts = append(facts, LifecycleFact{EventID: fmt.Sprint("start-", i), TaskID: "A-1", Kind: "started", Actor: "worker", AttemptID: fmt.Sprint("attempt-", i), OccurredAt: at(5 + i*25)})
		if i < 4 {
			stamp := at(24 + i*25).Format(time.RFC3339Nano)
			id := fmt.Sprint("h-", i)
			handoffs[id] = handoffjournal.Handoff{HandoffID: id, TaskID: "A-1", EventType: handoffjournal.EventWorkerDone, SourceAttemptID: fmt.Sprint("attempt-", i), PreparedAt: at(22 + i*25).Format(time.RFC3339Nano), ClaimedBy: "/root/controller", ClaimedAttemptID: "controller", ClaimedAt: at(23 + i*25).Format(time.RFC3339Nano), Steps: map[string]handoffjournal.StepState{"acceptance": {Result: "failed", ObservedAt: stamp, Evidence: "real correction"}}, Applied: true}
		}
	}
	facts = append(facts, LifecycleFact{EventID: "completed", TaskID: "A-1", Kind: "completed", OccurredAt: at(139)})
	w := SummarizeWorkflow(WorkflowInput{TaskID: "A-1", Facts: facts, Handoffs: handoffs, Summary: Summarize("A-1", nil), Now: at(140)})
	if w.Axis.ElapsedMillis == nil || *w.Axis.ElapsedMillis != 139*60000 || w.Measurement.Worker.Millis != nil || w.Measurement.Controller.Millis != nil || w.Measurement.Tests.Millis != nil {
		t.Fatalf("lost boundary or fabricated zero: %+v", w)
	}
	if len(w.Rounds) != 5 || w.Reporting.ExecutionsWithoutActivity != 5 || w.Reporting.ReviewsWithoutActivity != 4 || len(w.Unmeasured) != 1 {
		t.Fatalf("missing grounded round/reporting facts: %+v", w)
	}
	exportWorkflowFixture(t, "synthetic-unmeasured-history.json", w, nil)
	if len(w.Rows) != 9 {
		t.Fatalf("must retain execution/review facts without measured activity: %d", len(w.Rows))
	}
	for _, r := range w.Rows {
		for _, v := range r.Intervals {
			if v.Measured || v.DurationMillis != nil {
				t.Fatal("execution fact promoted to work")
			}
		}
	}
}
func TestWorkflowGroupsActivityTransitionsWithoutNewRounds(t *testing.T) {
	b, at := workflowFixture()
	events := []Event{}
	for i := 0; i < 20; i++ {
		action := Switch
		if i == 0 {
			action = Start
		}
		if i == 19 {
			action = Stop
		}
		kind := Implementation
		if i%2 == 1 {
			kind = TargetedTest
		}
		events = append(events, Event{EventID: fmt.Sprint(i), TaskID: "B-1", Role: RoleWorker, AgentID: "worker", RuntimeAttemptID: "one-attempt", Action: action, Type: kind, OccurredAt: at(10 + i), RecordedAt: at(10 + i)})
	}
	facts := []LifecycleFact{{EventID: "registered", TaskID: "B-1", Kind: "registered", OccurredAt: b}, {EventID: "completed", TaskID: "B-1", Kind: "completed", OccurredAt: at(60)}}
	w := SummarizeWorkflow(WorkflowInput{TaskID: "B-1", Events: events, Summary: Summarize("B-1", events), Facts: facts, Now: at(60)})
	exportWorkflowFixture(t, "synthetic-continuous-activity.json", w, events)
	if len(w.Rows) != 1 || len(w.Rows[0].Intervals) != 19 || len(w.Rounds) != 1 || w.Rows[0].Round == nil || *w.Rows[0].Round != 0 {
		t.Fatalf("activity transitions became rounds/rows: %+v", w.Rows)
	}
	if w.Axis.ElapsedMillis == nil || *w.Axis.ElapsedMillis != 60*60000 || w.Measurement.Status != "partial" {
		t.Fatalf("axis cropped to Activity: %+v", w)
	}
}
func TestWorkflowDeduplicatesReviewGroundsAndPreservesOverlap(t *testing.T) {
	b, at := workflowFixture()
	events := []Event{
		{TaskID: "B-1", EventID: "review-start", Role: RoleController, AgentID: "c", RuntimeAttemptID: "c1", HandoffID: "h1", Action: Start, Type: ControllerReview, OccurredAt: at(10)},
		{TaskID: "B-1", EventID: "reject", Role: RoleController, AgentID: "c", RuntimeAttemptID: "c1", HandoffID: "h1", Action: Stop, Type: ControllerReview, ReviewOutcome: "needs_changes", Evidence: "scope correction", OccurredAt: at(20)},
		{TaskID: "B-1", EventID: "work", Role: RoleWorker, AgentID: "w", RuntimeAttemptID: "w1", Action: Start, Type: Implementation, OccurredAt: at(21)},
		{TaskID: "B-1", EventID: "work-end", Role: RoleWorker, AgentID: "w", RuntimeAttemptID: "w1", Action: Stop, Type: Implementation, OccurredAt: at(30)},
		{TaskID: "B-1", EventID: "overlap", Role: RoleWorker, AgentID: "w", RuntimeAttemptID: "w2", Action: Start, Type: Implementation, OccurredAt: at(22)},
		{TaskID: "B-1", EventID: "overlap-end", Role: RoleWorker, AgentID: "w", RuntimeAttemptID: "w2", Action: Stop, Type: Implementation, OccurredAt: at(28)},
	}
	h := handoffjournal.Handoff{TaskID: "B-1", HandoffID: "h1", Steps: map[string]handoffjournal.StepState{"acceptance": {Result: "failed", ObservedAt: at(20).Format(time.RFC3339Nano)}}}
	w := SummarizeWorkflow(WorkflowInput{TaskID: "B-1", Events: events, Summary: Summarize("B-1", events), Facts: []LifecycleFact{{EventID: "r", TaskID: "B-1", Kind: "registered", OccurredAt: b}}, Handoffs: map[string]handoffjournal.Handoff{"h1": h}, Now: at(40)})
	if len(w.Rounds) != 2 || w.Measurement.AgentTimeMillis <= w.Measurement.ObservedWallMillis {
		t.Fatalf("duplicate round or lost concurrency: %+v", w)
	}
	for _, row := range w.Rows {
		if row.Role == "worker" {
			if len(row.Intervals) != 2 || row.Intervals[1].Lane != 1 || row.Round == nil || *row.Round != 1 {
				t.Fatalf("overlap lost: %+v", row)
			}
		}
	}
}
func TestWorkflowMeasuredZeroAndReopenedCompletion(t *testing.T) {
	b, at := workflowFixture()
	events := []Event{{EventID: "a-start", TaskID: "B-1", Role: RoleWorker, AgentID: "w", RuntimeAttemptID: "a", Action: Start, Type: Implementation, OccurredAt: at(1)}, {EventID: "b-end", TaskID: "B-1", Role: RoleWorker, AgentID: "w", RuntimeAttemptID: "a", Action: Stop, Type: Implementation, OccurredAt: at(1)}}
	facts := []LifecycleFact{{EventID: "r", TaskID: "B-1", Kind: "registered", OccurredAt: b}, {EventID: "c", TaskID: "B-1", Kind: "completed", OccurredAt: at(10)}, {EventID: "reopen", TaskID: "B-1", Kind: "resumed", Actor: "w", AttemptID: "new", OccurredAt: at(20)}}
	w := SummarizeWorkflow(WorkflowInput{TaskID: "B-1", Events: events, Summary: Summarize("B-1", events), Facts: facts, Now: at(30)})
	exportWorkflowFixture(t, "synthetic-zero.json", w, events)
	for _, row := range w.Rows {
		for _, interval := range row.Intervals {
			if interval.Kind == "activity_open" {
				t.Fatal("zero-duration Stop left the closed activity open")
			}
		}
	}
	if w.Measurement.Worker.Millis == nil || *w.Measurement.Worker.Millis != 0 || w.Measurement.Review.Millis != nil {
		t.Fatalf("measured zero conflated with absent: %+v", w.Measurement)
	}
	if !w.Axis.Open || w.Axis.ElapsedMillis == nil || *w.Axis.ElapsedMillis != 30*60000 || w.Lifecycle.CompletedAt != nil || len(w.Lifecycle.CompletionHistory) != 1 || len(w.Rounds) != 1 {
		t.Fatalf("reopen cropped lifecycle or invented rework: %+v", w)
	}
}

func exportWorkflowFixture(t *testing.T, name string, w Workflow, evidence []Event) {
	t.Helper()
	dir := os.Getenv("A57_WORKFLOW_EVIDENCE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(map[string]any{"source": "synthetic regression fixture; contains no user evidence", "workflow": w, "summary": Summarize(w.TaskID, evidence), "events": evidence}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, name), append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowShortNonzeroZeroAndUnmeasuredRemainDistinct(t *testing.T) {
	b, at := workflowFixture()
	events := []Event{
		{EventID: "short-s", TaskID: "B-1", Role: RoleWorker, AgentID: "w", RuntimeAttemptID: "short", Action: Start, Type: Implementation, OccurredAt: at(1)},
		{EventID: "short-e", TaskID: "B-1", Role: RoleWorker, AgentID: "w", RuntimeAttemptID: "short", Action: Stop, Type: Implementation, OccurredAt: at(1).Add(250 * time.Millisecond)},
		{EventID: "zero-s", TaskID: "B-1", Role: RoleWorker, AgentID: "w", RuntimeAttemptID: "zero", Action: Start, Type: TargetedTest, OccurredAt: at(2)},
		{EventID: "zero-e", TaskID: "B-1", Role: RoleWorker, AgentID: "w", RuntimeAttemptID: "zero", Action: Stop, Type: TargetedTest, OccurredAt: at(2), RecordedAt: at(2).Add(time.Millisecond)},
	}
	facts := []LifecycleFact{{EventID: "r", TaskID: "B-1", Kind: "registered", OccurredAt: b}, {EventID: "s", TaskID: "B-1", Kind: "started", Actor: "w", AttemptID: "unmeasured", OccurredAt: at(3)}, {EventID: "c", TaskID: "B-1", Kind: "completed", OccurredAt: at(60)}}
	w := SummarizeWorkflow(WorkflowInput{TaskID: "B-1", Events: events, Summary: Summarize("B-1", events), Facts: facts, Now: at(60)})
	exportWorkflowFixture(t, "synthetic-short.json", w, events)
	if w.Measurement.Worker.Millis == nil || *w.Measurement.Worker.Millis != 250 || w.Measurement.Review.Millis != nil {
		t.Fatalf("short/zero/absent measurements changed: %+v", w.Measurement)
	}
	foundShort, foundZero, foundUnknown := false, false, false
	for _, row := range w.Rows {
		for _, interval := range row.Intervals {
			if interval.Measured && interval.DurationMillis != nil {
				foundShort = foundShort || *interval.DurationMillis == 250
				foundZero = foundZero || *interval.DurationMillis == 0
			} else if interval.Kind == "execution_report" && interval.End == nil {
				foundUnknown = true
			}
		}
	}
	if !foundShort || !foundZero || !foundUnknown {
		t.Fatalf("short, zero or unmeasured lost: %+v", w.Rows)
	}
}

func TestWorkflowMissingBoundsRemainsUnmeasured(t *testing.T) {
	_, at := workflowFixture()
	w := SummarizeWorkflow(WorkflowInput{TaskID: "B-1", Summary: Summarize("B-1", nil), Now: at(60)})
	exportWorkflowFixture(t, "synthetic-empty.json", w, nil)
	if w.Axis.Start != nil || w.Axis.End != nil || w.Measurement.Worker.Millis != nil || len(w.Rows) != 0 {
		t.Fatalf("missing evidence became a period or measured zero: %+v", w)
	}
}

func TestWorkflowExplicitCIWaitIsNotUserHoldAndClosedWaitDoesNotPersist(t *testing.T) {
	_, at := workflowFixture()
	events := []Event{{TaskID: "B-1", EventID: "a-start", Action: Start, Role: RoleController, AgentID: "c", RuntimeAttemptID: "a", Type: Other, PhaseEvent: "external_ci_wait_started", OccurredAt: at(1)}}
	open := SummarizeWorkflow(WorkflowInput{TaskID: "B-1", Events: events, Summary: Summarize("B-1", events), Now: at(2)})
	if open.Current.Phase != "external_ci_wait" {
		t.Fatalf("explicit CI wait lost: %+v", open.Current)
	}
	events = append(events, Event{TaskID: "B-1", EventID: "b-end", Action: Stop, Role: RoleController, AgentID: "c", RuntimeAttemptID: "a", Type: Other, OccurredAt: at(3)})
	closed := SummarizeWorkflow(WorkflowInput{TaskID: "B-1", Events: events, Summary: Summarize("B-1", events), Now: at(4)})
	if closed.Current.Phase == "external_ci_wait" || closed.Current.Phase == "user_decision_wait" {
		t.Fatalf("closed wait or CI relabelled as user hold: %+v", closed.Current)
	}
}

func TestWorkflowExecutionEndRequiresWorkerReportOrObservedTerminal(t *testing.T) {
	b, at := workflowFixture()
	facts := []LifecycleFact{{EventID: "r", TaskID: "B-1", Kind: "registered", OccurredAt: b}, {EventID: "s", TaskID: "B-1", Kind: "started", Actor: "/root/controller/pairi", AttemptID: "a", OccurredAt: at(1)}, {EventID: "c", TaskID: "B-1", Kind: "completed", OccurredAt: at(60)}}
	partial := handoffjournal.Handoff{HandoffID: "partial", TaskID: "B-1", EventType: handoffjournal.EventCompletionReport, SourceAttemptID: "a", SourceAgentPath: "/root/controller/pairi", PreparedAt: at(10).Format(time.RFC3339Nano)}
	input := WorkflowInput{TaskID: "B-1", Facts: facts, Summary: Summarize("B-1", nil), Now: at(61), Handoffs: map[string]handoffjournal.Handoff{"partial": partial}}
	w := SummarizeWorkflow(input)
	v := w.Rows[0].Intervals[0]
	if v.End != nil || v.EndEvidence != "unobserved" {
		t.Fatalf("task completion or partial milestone fabricated Worker end: %+v", v)
	}
	done := partial
	done.EventType = handoffjournal.EventWorkerDone
	done.PreparedAt = at(20).Format(time.RFC3339Nano)
	input.Handoffs["done"] = done
	w = SummarizeWorkflow(input)
	v = w.Rows[0].Intervals[0]
	if v.End == nil || !v.End.Equal(at(20)) || v.EndEvidence != "worker_done_report" || v.Measured || v.DurationMillis != nil {
		t.Fatalf("actual Worker report boundary lost/promoted to work: %+v", v)
	}
	delete(input.Handoffs, "done")
	native := runtimeobs.Attempt{AttemptID: "a", TaskID: "B-1", AgentPath: "/root/controller/pairi", RuntimeAgentID: "native", SessionID: "session", Provider: "codex", BindingState: runtimeobs.BindingBound, CurrentState: runtimeobs.StateCompleted, Terminal: true, EndedAt: at(25).Format(time.RFC3339Nano), StateEvidenceSource: runtimeobs.EvidenceHook, StateObservationQuality: runtimeobs.QualityObserved}
	input.ExecutionObservations = []runtimeobs.Attempt{native}
	w = SummarizeWorkflow(input)
	v = w.Rows[0].Intervals[0]
	if v.End == nil || !v.End.Equal(at(25)) || v.EndEvidence != "native_terminal" {
		t.Fatalf("observed terminal boundary lost: %+v", v)
	}
	native.BindingState = runtimeobs.BindingAmbiguous
	input.ExecutionObservations = []runtimeobs.Attempt{native}
	w = SummarizeWorkflow(input)
	if w.Rows[0].Intervals[0].End != nil {
		t.Fatal("ambiguous native identity closed execution")
	}
}
func TestWorkflowBrowserOverlapFixtureUsesConsistentMeasuredSummary(t *testing.T) {
	b, at := workflowFixture()
	events := []Event{}
	add := func(id, agent, attempt string, role Role, kind Type, start, end int, outcome string) {
		events = append(events, Event{EventID: id + "-s", TaskID: "B-1", Role: role, AgentID: agent, RuntimeAttemptID: attempt, Action: Start, Type: kind, OccurredAt: at(start)}, Event{EventID: id + "-e", TaskID: "B-1", Role: role, AgentID: agent, RuntimeAttemptID: attempt, Action: Stop, Type: kind, OccurredAt: at(end), ReviewOutcome: outcome, Evidence: "synthetic correction"})
	}
	add("review", "reviewer", "c", RoleController, ControllerReview, 10, 20, "needs_changes")
	add("work", "worker", "a1", RoleWorker, Implementation, 21, 30, "")
	add("parallel", "worker", "a2", RoleWorker, TargetedTest, 22, 28, "")
	add("other", "a-second-very-long-named-agent-for-accessibility-check", "a3", RoleWorker, Implementation, 22, 27, "")
	w := SummarizeWorkflow(WorkflowInput{TaskID: "B-1", Events: events, Summary: Summarize("B-1", events), Facts: []LifecycleFact{{EventID: "r", TaskID: "B-1", Kind: "registered", OccurredAt: b}, {EventID: "c", TaskID: "B-1", Kind: "completed", OccurredAt: at(60)}}, Now: at(60)})
	if w.Measurement.Review.Millis == nil || *w.Measurement.Review.Millis != 10*60000 {
		t.Fatalf("review summary absent: %+v", w.Measurement)
	}
	found := false
	for _, row := range w.Rows {
		if row.Role == "controller" {
			if len(row.Intervals) != 1 || row.Intervals[0].DurationMillis == nil || *row.Intervals[0].DurationMillis != *w.Measurement.Review.Millis || row.Intervals[0].Outcome != "needs_changes" {
				t.Fatalf("row and summary disagree: %+v", row)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing Controller row")
	}
	exportWorkflowFixture(t, "synthetic-overlap.json", w, events)
}
