package backlog

import (
	"sort"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// A provider turn boundary is not a product result. Keep the last measured
// Worker phase, exact actor and original provider state alongside recovery advice.
func workerProgress(a runtimeobs.Attempt, events []activity.Event) map[string]any {
	phase := "assigned_prepared"
	sort.SliceStable(events, func(i, j int) bool { return events[i].OccurredAt.Before(events[j].OccurredAt) })
	for _, event := range events {
		if a.BindingEvidence["assignment_id"] == "" || event.Role != activity.RoleWorker || event.TaskID != a.TaskID || event.RuntimeAttemptID != a.AttemptID || event.AssignmentID != a.BindingEvidence["assignment_id"] {
			continue
		}
		switch event.PhaseEvent {
		case "requirements_started", "research_started", "implementation_started", "targeted_test_started", "full_regression_started", "build_started", "worker_completed":
			phase = event.PhaseEvent
		}
	}
	remaining := "collect_current_phase"
	switch phase {
	case "assigned_prepared":
		remaining = "worker_START_and_current_phase"
	case "requirements_started", "research_started":
		remaining = "implementation"
	case "implementation_started", "targeted_test_started", "full_regression_started", "build_started":
		remaining = "worker_completion_report"
	case "worker_completed":
		remaining = "completion_handoff_and_review"
	}
	return map[string]any{"phase": phase, "attempt_id": a.AttemptID, "task_id": a.TaskID,
		"assignment_id": a.BindingEvidence["assignment_id"], "native_id": a.SessionID,
		"runtime_state": a.CurrentState, "turn_ended": a.Terminal, "technical_crash": "unproven",
		"action_owner": "/root/controller", "remaining_stage": remaining,
		"resume_condition": "Inspect original task/contract, exact native attempt and existing assignment/handoff/report. Collect actual current phase or directly resume/reassign through supported transport after fresh Full Access; preserve errors and changes. No backfill, automatic wake or Root ACK gate."}
}

func assignmentCollectionDue(signal map[string]any, now time.Time) bool {
	at, e := time.Parse(time.RFC3339Nano, toString(signal["assigned_at"]))
	if e != nil {
		at, e = time.Parse(time.RFC3339Nano, toString(signal["binding_at"]))
	}
	return e == nil && !now.Before(at.Add(runtimeobs.AssignmentObservationGrace))
}
