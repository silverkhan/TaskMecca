package handoff

import (
	"sync"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestPrepareRetargetsOnlyVerifiedControllerRuntime(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Date(2026, 10, 6, 13, 0, 0, 0, time.UTC)
	appendControllerAttempt(t, project, "run-controller-old", "session-1", "controller-1", runtimeobs.StateCompleted, now)

	prepared, err := Prepare(PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", TargetAttemptID: "run-controller-old", ExecutionAuthorized: true}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	appendControllerAttempt(t, project, "run-controller-new", "session-1", "controller-1", runtimeobs.StateRunning, now.Add(2*time.Second))

	retargeted, err := Prepare(PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", TargetAttemptID: "run-controller-new", ExecutionAuthorized: true}, now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !retargeted.Duplicate || retargeted.HandoffID != prepared.HandoffID || retargeted.Target.AttemptID != "run-controller-new" || retargeted.Reason != "retargeted_verified_runtime_identity" {
		t.Fatalf("retarget=%+v prepared=%+v", retargeted, prepared)
	}

	claim, err := Claim(project, prepared.HandoffID, "/root/controller", "run-controller-new", now.Add(4*time.Second))
	if err != nil || !claim.Claimed {
		t.Fatalf("claim=%+v err=%v", claim, err)
	}
	oldClaim, err := Claim(project, prepared.HandoffID, "/root/controller", "run-controller-old", now.Add(5*time.Second))
	if err != nil || !oldClaim.ClaimConflict {
		t.Fatalf("old claim=%+v err=%v", oldClaim, err)
	}
}

func TestRetargetRejectsForeignRuntimeAndLeavesHandoffOnHold(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Date(2026, 10, 6, 13, 0, 0, 0, time.UTC)
	appendControllerAttempt(t, project, "run-controller-old", "session-1", "controller-1", runtimeobs.StateCompleted, now)
	prepared, err := Prepare(PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", TargetAttemptID: "run-controller-old", ExecutionAuthorized: true}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	appendControllerAttempt(t, project, "run-controller-foreign", "session-1", "controller-other", runtimeobs.StateRunning, now.Add(2*time.Second))

	got, err := Prepare(PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", TargetAttemptID: "run-controller-foreign", ExecutionAuthorized: true}, now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != ActionHold || got.Reason == "" {
		t.Fatalf("retarget=%+v", got)
	}
	claim, err := Claim(project, prepared.HandoffID, "/root/controller", "run-controller-old", now.Add(4*time.Second))
	if err != nil || !claim.ClaimConflict || claim.Reason != "handoff_on_hold" {
		t.Fatalf("claim=%+v err=%v", claim, err)
	}
}

func TestRetargetedHandoffHasOneConcurrentClaim(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Date(2026, 10, 6, 13, 0, 0, 0, time.UTC)
	appendControllerAttempt(t, project, "run-controller-old", "session-1", "controller-1", runtimeobs.StateCompleted, now)
	prepared, err := Prepare(PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", TargetAttemptID: "run-controller-old", ExecutionAuthorized: true}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	appendControllerAttempt(t, project, "run-controller-new", "session-1", "controller-1", runtimeobs.StateRunning, now.Add(2*time.Second))
	if _, err := Prepare(PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", TargetAttemptID: "run-controller-new", ExecutionAuthorized: true}, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}

	results := make(chan ClaimResult, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := Claim(project, prepared.HandoffID, "/root/controller", "run-controller-new", now.Add(4*time.Second))
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			results <- got
		}()
	}
	wg.Wait()
	close(results)
	claimed := 0
	already := 0
	for result := range results {
		if result.Claimed {
			claimed++
		}
		if result.AlreadyClaimed {
			already++
		}
	}
	if claimed != 1 || already != 1 {
		t.Fatalf("claimed=%d already=%d", claimed, already)
	}
}
