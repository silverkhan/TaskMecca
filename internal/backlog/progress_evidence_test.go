package backlog

import (
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestA79TurnEndPreservesActualPhaseAndDifferentTaskEvidence(t *testing.T) {
	now := time.Now().UTC()
	a := runtimeobs.Attempt{AttemptID: "worker-current", TaskID: "B-638", SessionID: "actual-native", CurrentState: runtimeobs.StateCompleted, Terminal: true, BindingEvidence: map[string]string{"assignment_id": "assignment-current"}}
	events := []activity.Event{
		{Role: activity.RoleWorker, TaskID: "B-638", RuntimeAttemptID: a.AttemptID, AssignmentID: "assignment-current", PhaseEvent: "requirements_started", OccurredAt: now},
		{Role: activity.RoleWorker, TaskID: "B-621", RuntimeAttemptID: a.AttemptID, AssignmentID: "assignment-current", PhaseEvent: "worker_completed", OccurredAt: now.Add(time.Second)},
		{Role: activity.RoleWorker, TaskID: "B-638", RuntimeAttemptID: "old-worker", AssignmentID: "assignment-old", PhaseEvent: "worker_completed", OccurredAt: now.Add(2 * time.Second)},
	}
	p := workerProgress(a, events)
	if p["phase"] != "requirements_started" || p["remaining_stage"] != "implementation" || p["technical_crash"] != "unproven" || p["action_owner"] != "/root/controller" {
		t.Fatal(p)
	}
	events = append(events, activity.Event{Role: activity.RoleWorker, TaskID: a.TaskID, RuntimeAttemptID: a.AttemptID, AssignmentID: "assignment-current", PhaseEvent: "implementation_started", OccurredAt: now.Add(3 * time.Second)})
	p = workerProgress(a, events)
	if p["phase"] != "implementation_started" || p["remaining_stage"] != "worker_completion_report" {
		t.Fatal(p)
	}
	a.Terminal = false
	a.CurrentState = runtimeobs.StateRunning
	p = workerProgress(a, nil)
	if p["phase"] != "assigned_prepared" || p["turn_ended"] != false {
		t.Fatal("running inferred implementation", p)
	}
}

func TestA79PreparedAssignmentGraceAndWaitingRemainDistinct(t *testing.T) {
	now := time.Now().UTC()
	s := map[string]any{"assigned_at": now.Format(time.RFC3339Nano)}
	if assignmentCollectionDue(s, now) {
		t.Fatal("new assignment treated as missing START")
	}
	if !assignmentCollectionDue(s, now.Add(runtimeobs.AssignmentObservationGrace)) {
		t.Fatal("missing START not recoverable after existing grace")
	}
	if assignmentCollectionDue(map[string]any{}, now) {
		t.Fatal("unknown time synthesized")
	}
	a := runtimeobs.Attempt{TaskID: "B-638", AttemptID: "current", CurrentState: runtimeobs.StateWaitingUser}
	p := workerProgress(a, nil)
	if p["runtime_state"] != runtimeobs.StateWaitingUser || p["technical_crash"] != "unproven" {
		t.Fatal(p)
	}
}
