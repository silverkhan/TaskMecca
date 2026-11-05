package activity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
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
			handoffs[id] = handoffjournal.Handoff{HandoffID: id, TaskID: "A-1", EventType: handoffjournal.EventWorkerDone, ClaimedBy: "/root/controller", ClaimedAttemptID: "controller", ClaimedAt: at(23 + i*25).Format(time.RFC3339Nano), Steps: map[string]handoffjournal.StepState{"acceptance": {Result: "failed", ObservedAt: stamp, Evidence: "real correction"}}, Applied: true}
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
	exportWorkflowFixture(t, "synthetic-unmeasured-history.json", w)
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
	exportWorkflowFixture(t, "synthetic-continuous-activity.json", w)
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
	if w.Measurement.Worker.Millis == nil || *w.Measurement.Worker.Millis != 0 || w.Measurement.Review.Millis != nil {
		t.Fatalf("measured zero conflated with absent: %+v", w.Measurement)
	}
	if !w.Axis.Open || w.Axis.ElapsedMillis == nil || *w.Axis.ElapsedMillis != 30*60000 || w.Lifecycle.CompletedAt != nil || len(w.Lifecycle.CompletionHistory) != 1 || len(w.Rounds) != 1 {
		t.Fatalf("reopen cropped lifecycle or invented rework: %+v", w)
	}
}

func exportWorkflowFixture(t *testing.T, name string, w Workflow) {
	t.Helper()
	dir := os.Getenv("A57_WORKFLOW_EVIDENCE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(map[string]any{"source": "synthetic regression fixture; contains no user evidence", "workflow": w}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, name), append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowExplicitCIWaitIsNotUserHoldAndClosedWaitDoesNotPersist(t *testing.T){
 _,at:=workflowFixture()
 events:=[]Event{{TaskID:"B-1",EventID:"a-start",Action:Start,Role:RoleController,AgentID:"c",RuntimeAttemptID:"a",Type:Other,PhaseEvent:"external_ci_wait_started",OccurredAt:at(1)}}
 open:=SummarizeWorkflow(WorkflowInput{TaskID:"B-1",Events:events,Summary:Summarize("B-1",events),Now:at(2)})
 if open.Current.Phase!="external_ci_wait"{t.Fatalf("explicit CI wait lost: %+v",open.Current)}
 events=append(events,Event{TaskID:"B-1",EventID:"b-end",Action:Stop,Role:RoleController,AgentID:"c",RuntimeAttemptID:"a",Type:Other,OccurredAt:at(3)})
 closed:=SummarizeWorkflow(WorkflowInput{TaskID:"B-1",Events:events,Summary:Summarize("B-1",events),Now:at(4)})
 if closed.Current.Phase=="external_ci_wait"||closed.Current.Phase=="user_decision_wait"{t.Fatalf("closed wait or CI relabelled as user hold: %+v",closed.Current)}
}
