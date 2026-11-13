package webui

import (
	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"testing"
	"time"
)

// Run this with the two read-model source files only; no Handler/server or
// service-control test helpers are compiled or executed.
func TestActivityWorkflowReadPendingSnapshotAndTerminalCache(t *testing.T) {
	project := t.TempDir()
	now := time.Now().UTC()
	start := now.Add(-time.Minute)
	event := runtimeobs.ExecutionEvent{EventID: "start", EventKind: "state", AttemptID: "attempt", RuntimeAgentID: "native", SessionID: "session", Provider: "codex", ObservedAt: start.Format(time.RFC3339Nano), State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}
	if err := runtimeobs.AppendExecutionEvent(project, event); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.BindAttempt(project, "attempt", "T", "/root/controller/w", "dispatch", "", map[string]string{"assignment_id": "assignment"}, start); err != nil {
		t.Fatal(err)
	}
	facts := []activity.LifecycleFact{{EventID: "s", TaskID: "T", Kind: "started", Actor: "/root/controller/w", AttemptID: "attempt", AssignmentID: "assignment", OccurredAt: start}}
	read := func() activity.Workflow {
		return readActivityWorkflow(project, "T", nil, activity.Summarize("T", nil), facts)
	}
	first := read()
	again := read()
	if !first.ComputedAt.Equal(again.ComputedAt) {
		t.Fatal("unchanged sources not cached")
	}
	if first.Snapshot["trust"] != "pending" {
		t.Fatalf("unexpected control snapshot %+v", first.Snapshot)
	}
	event.EventID = "end"
	event.Terminal = true
	event.State = runtimeobs.StateCompleted
	event.ObservedAt = now.Add(-time.Second).Format(time.RFC3339Nano)
	if err := runtimeobs.AppendExecutionEvent(project, event); err != nil {
		t.Fatal(err)
	}
	terminal := read()
	if terminal.Current.Phase != "execution_ended" || terminal.Rows[0].Intervals[0].EndEvidenceSource != runtimeobs.EvidenceHook {
		t.Fatalf("terminal hidden by pending snapshot/cache %+v", terminal)
	}
	if terminal.ComputedAt.Equal(first.ComputedAt) {
		t.Fatal("terminal did not invalidate response cache")
	}
	// Later live evidence on the same attempt cannot reuse the terminal response.
	event.EventID = "resume"
	event.Terminal = false
	event.State = runtimeobs.StateRunning
	event.ObservedAt = now.Format(time.RFC3339Nano)
	if err := runtimeobs.AppendExecutionEvent(project, event); err != nil {
		t.Fatal(err)
	}
	resumed := read()
	if resumed.Current.Phase == "execution_ended" || resumed.Rows[0].Intervals[0].End != nil {
		t.Fatalf("stale terminal survived resume %+v", resumed)
	}
}
