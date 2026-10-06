package runtimeobs

import (
	"testing"
	"time"
)

func TestAssignmentSurvivesRestartAndBindsLateAttempt(t *testing.T) {
	project := t.TempDir()
	assignedAt := time.Date(2026, 10, 6, 0, 21, 53, 0, time.UTC)
	assignment, err := RecordAssignment(project, "dispatch-b446", "B-446", "/root/controller/worker-446", assignedAt)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := RecordAssignment(project, "dispatch-b446", "B-446", "/root/controller/worker-446", assignedAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if repeated != assignment {
		t.Fatalf("assignment retry changed durable record: %+v != %+v", repeated, assignment)
	}
	// Reopen from the journal, as after a Controller restart.
	loaded, err := FindAssignment(project, "dispatch-b446")
	if err != nil || loaded != assignment {
		t.Fatalf("assignment not durable: %+v %v", loaded, err)
	}
	bound, err := BindRuntimeAgentForAssignment(project, assignment.AssignmentID, "worker-446", "", assignedAt.Add(91*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if bound.TaskID != "B-446" || bound.AgentPath != "/root/controller/worker-446" || bound.BindingEvidence["assignment_id"] != assignment.AssignmentID {
		t.Fatalf("late attempt lost assignment evidence: %+v", bound)
	}
	if bound.StartedAt != "" {
		t.Fatalf("dispatch alone must not prove start: %+v", bound)
	}
	items, err := ListAssignments(project)
	if err != nil || len(items) != 1 {
		t.Fatalf("assignment retry duplicated journal: %+v %v", items, err)
	}
}

func TestAssignmentIDCannotBeReusedForOtherTask(t *testing.T) {
	project := t.TempDir()
	now := time.Now().UTC()
	if _, err := RecordAssignment(project, "dispatch-one", "B-1", "/root/controller/worker", now); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordAssignment(project, "dispatch-one", "B-2", "/root/controller/worker", now); err == nil {
		t.Fatal("conflicting assignment ID accepted")
	}
}

func TestDistinctAssignmentsCannotClaimOneLiveAttempt(t *testing.T) {
	project := t.TempDir()
	now := time.Now().UTC()
	for _, id := range []string{"dispatch-one", "dispatch-two"} {
		if _, err := RecordAssignment(project, id, "B-1", "/root/controller/worker", now); err != nil {
			t.Fatal(err)
		}
	}
	first, err := BindRuntimeAgentForAssignment(project, "dispatch-one", "worker", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BindRuntimeAgentForAssignment(project, "dispatch-two", "worker", "", now.Add(time.Minute)); err == nil {
		t.Fatal("second assignment claimed first assignment's live attempt")
	}
	ledger, err := BuildLedger(project, 20, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Attempts) != 1 || ledger.Attempts[0].BindingEvidence["assignment_id"] != "dispatch-one" || ledger.Attempts[0].AttemptID != first.AttemptID {
		t.Fatalf("failed claim mutated live attempt: %+v", ledger.Attempts)
	}
}
