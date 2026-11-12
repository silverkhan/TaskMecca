package handoff

import (
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHeldSameAttemptRevalidatesWithoutDuplicateHandoff(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Date(2026, 11, 12, 1, 0, 0, 0, time.UTC)
	pending, err := runtimeobs.BindRuntimeAgent(project, "controller-1", "A-1", "/root/controller", "", now)
	if err != nil {
		t.Fatal(err)
	}
	req := PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", TargetAttemptID: pending.AttemptID, ExecutionAuthorized: true}
	held, err := Prepare(req, now)
	if err != nil || held.Action != ActionHold {
		t.Fatalf("hold=%+v %v", held, err)
	}
	// Real Hook provenance remains a valid independent route; same held attempt
	// must revalidate even when its exact attempt ID did not change.
	appendControllerAttempt(t, project, pending.AttemptID, "session-1", "controller-1", runtimeobs.StateRunning, now.Add(time.Second))
	got, err := Prepare(req, now.Add(2*time.Second))
	if err != nil || got.HandoffID != held.HandoffID || got.Action != ActionMessageRunning {
		t.Fatalf("revalidate=%+v %v", got, err)
	}
	again, err := Prepare(req, now.Add(3*time.Second))
	if err != nil || !again.Duplicate || again.HandoffID != held.HandoffID {
		t.Fatal(again, err)
	}
	ledger, err := BuildLedger(project, now.Add(3*time.Second))
	if err != nil || len(ledger.Handoffs) != 1 {
		t.Fatal(ledger, err)
	}
}
func TestHeldRevalidationContractAuthorizationConflict(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Now().UTC()
	appendControllerAttempt(t, project, "controller", "s", "r", runtimeobs.StateWaitingUser, now)
	req := PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", TargetAttemptID: "controller", ExecutionAuthorized: false}
	noAuth, err := Prepare(req, now)
	if err != nil || noAuth.Action != ActionReportOnly {
		t.Fatal(noAuth, err)
	}
	appendControllerAttempt(t, project, "controller", "s", "r", runtimeobs.StateRunning, now.Add(time.Second))
	got, err := Prepare(req, now.Add(2*time.Second))
	if err != nil || got.Action != ActionReportOnly || got.HandoffID != noAuth.HandoffID {
		t.Fatal("approval bypass", got, err)
	}
	req.ExecutionAuthorized = true
	held, err := Prepare(req, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	req.ExpectedContractSHA256 = held.ContractSHA256
	path := filepath.Join(project, "_task_mecca", "data", "backlog", "000001.A-1.handoff-test.todo.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), "이벤트 기반 인계를 검증한다.", "새로운 계약 목표다."))
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Prepare(req, now.Add(3*time.Second)); err == nil {
		t.Fatal("changed contract accepted")
	}
}
func TestHeldRevalidationConcurrentRetry(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Now().UTC()
	appendControllerAttempt(t, project, "controller", "s", "r", runtimeobs.StateWaitingUser, now)
	req := PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", TargetAttemptID: "controller", ExecutionAuthorized: true}
	held, err := Prepare(req, now)
	if err != nil {
		t.Fatal(err)
	}
	appendControllerAttempt(t, project, "controller", "s", "r", runtimeobs.StateRunning, now.Add(time.Second))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := Prepare(req, now.Add(2*time.Second))
			if e != nil || r.HandoffID != held.HandoffID || r.Action != ActionMessageRunning {
				t.Error(r, e)
			}
		}()
	}
	wg.Wait()
	ledger, err := BuildLedger(project, now.Add(3*time.Second))
	if err != nil || len(ledger.Handoffs) != 1 {
		t.Fatal(ledger, err)
	}
}
