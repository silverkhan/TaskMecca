package activity

import (
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"testing"
)

func TestA90ReviewerActivityAttributionAndReadOnlyBoundary(t *testing.T) {
	e := validEvent()
	e.Role = RoleReviewer
	e.Type = ControllerReview
	e.AssignmentID = "assignment"
	e.AgentID = "native"
	a := runtimeobs.Attempt{AttemptID: e.RuntimeAttemptID, RuntimeAgentID: e.AgentID, TaskID: e.TaskID, AgentPath: "/root/controller/reviewer/raichyu", BindingState: runtimeobs.BindingBound, CurrentState: runtimeobs.StateRunning, BindingEvidence: map[string]string{"assignment_id": e.AssignmentID}}
	assignments := []runtimeobs.Assignment{{AssignmentID: e.AssignmentID, TaskID: e.TaskID, AgentPath: a.AgentPath}}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAttribution(e, []runtimeobs.Attempt{a}, assignments); err != nil {
		t.Fatal(err)
	}
	e.Role = RoleWorker
	if err := ValidateAttribution(e, []runtimeobs.Attempt{a}, assignments); err == nil {
		t.Fatal("Worker label impersonated Reviewer")
	}
	e.Role = RoleReviewer
	e.Type = Implementation
	if err := e.Validate(); err == nil {
		t.Fatal("Reviewer recorded implementation")
	}
	e.Role = RoleWorker
	if err := e.Validate(); err != nil {
		t.Fatal("legacy Worker activity changed", err)
	}
}
