package handoff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func writeSimpleTask(t *testing.T, project, id string) {
	t.Helper()
	dir := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	text := "# " + id + " handoff test\n\n## 작업 개요\n\n- 등록자: user\n- Agent: -\n- 변경범위: -\n\n## 작업 정의\n\n### 목표\n\n이벤트 기반 인계를 검증한다.\n\n### 수용 기준\n\n- [ ] 중복 인계가 한 번만 처리된다.\n\n## 실행 정보\n\n## 작업 노트\n\n## 결과\n\n## 검증\n"
	path := filepath.Join(dir, "000001."+id+".handoff-test.todo.md")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendControllerAttempt(t *testing.T, project, attemptID, sessionID, runtimeID string, state runtimeobs.CanonicalState, at time.Time) {
	t.Helper()
	terminal := state == runtimeobs.StateCompleted || state == runtimeobs.StateInterrupted || state == runtimeobs.StateErrored || state == runtimeobs.StateShutdown
	event := runtimeobs.ExecutionEvent{
		EventKind: "state", ObservedAt: at.UTC().Format(time.RFC3339Nano), AttemptID: attemptID,
		Provider: "codex", SessionID: sessionID, RuntimeAgentID: runtimeID,
		State: state, Terminal: terminal, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved,
	}
	if err := runtimeobs.AppendExecutionEvent(project, event); err != nil {
		t.Fatal(err)
	}
	binding := runtimeobs.ExecutionEvent{
		EventKind: "binding", ObservedAt: at.Add(time.Millisecond).UTC().Format(time.RFC3339Nano), AttemptID: attemptID,
		TaskID: "A-1", AgentPath: "/root/controller", BindingSource: "test",
		EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative,
	}
	if err := runtimeobs.AppendExecutionEvent(project, binding); err != nil {
		t.Fatal(err)
	}
}

func TestContractFingerprintIgnoresLineEndingAndTrailingSpace(t *testing.T) {
	a := "# A-1\n\n## 작업 정의\n\n### 목표\nhello  \n\n### 수용 기준\n- [ ] ok\n\n## 결과\nnone\n"
	b := "# A-1\r\n\r\n## 작업 정의\r\n\r\n### 목표\r\nhello\r\n\r\n### 수용 기준\r\n- [ ] ok\r\n\r\n## 결과\r\nchanged\r\n"
	one, err := ContractFingerprint(a)
	if err != nil {
		t.Fatal(err)
	}
	two, err := ContractFingerprint(b)
	if err != nil {
		t.Fatal(err)
	}
	if one != two {
		t.Fatalf("contract fingerprint changed: %s != %s", one, two)
	}
}

func TestPrepareIsDeterministicAndResumesCompletedController(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	appendControllerAttempt(t, project, "run-controller", "session-1", "controller-1", runtimeobs.StateCompleted, now)

	req := PrepareRequest{
		Project: project, TaskID: "A-1", EventType: EventRegistrationReady,
		SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller",
		TargetAttemptID: "run-controller", ExecutionAuthorized: true,
	}
	first, err := Prepare(req, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if first.Action != ActionResumeCompleted || !first.RequiresFreshPreflight {
		t.Fatalf("prepare=%+v", first)
	}
	second, err := Prepare(req, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !second.Duplicate || first.HandoffID != second.HandoffID {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
}

func TestPrepareFailsClosedOnAmbiguousController(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	appendControllerAttempt(t, project, "run-one", "session-1", "controller-1", runtimeobs.StateCompleted, now)
	appendControllerAttempt(t, project, "run-two", "session-2", "controller-2", runtimeobs.StateCompleted, now.Add(time.Second))

	got, err := Prepare(PrepareRequest{
		Project: project, TaskID: "A-1", EventType: EventRegistrationReady,
		SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", ExecutionAuthorized: true,
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != ActionHold || got.Reason != "target_ambiguous" {
		t.Fatalf("prepare=%+v", got)
	}
}

func TestClaimAndMarkAreIdempotent(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	got, err := Prepare(PrepareRequest{
		Project: project, TaskID: "A-1", EventType: EventRegistrationReady,
		SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", ExecutionAuthorized: false,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := Claim(project, got.HandoffID, "/root/controller", "run-controller", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !claim.Claimed {
		t.Fatalf("claim=%+v", claim)
	}
	again, err := Claim(project, got.HandoffID, "/root/controller", "run-controller", now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !again.AlreadyClaimed {
		t.Fatalf("claim=%+v", again)
	}
	conflict, err := Claim(project, got.HandoffID, "/root/other", "run-other", now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !conflict.ClaimConflict {
		t.Fatalf("claim=%+v", conflict)
	}

	marked, err := Mark(project, got.HandoffID, "dispatch", "ok", "delivered", now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if marked.Duplicate {
		t.Fatal("first mark reported duplicate")
	}
	duplicate, err := Mark(project, got.HandoffID, "dispatch", "ok", "delivered", now.Add(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate.Duplicate {
		t.Fatalf("mark=%+v", duplicate)
	}
	if _, err := Mark(project, got.HandoffID, "dispatch", "failed", "retry", now.Add(6*time.Second)); err == nil {
		t.Fatal("conflicting evidence must not overwrite prior step")
	}
	applied, err := Mark(project, got.HandoffID, "applied", "ok", "done", now.Add(7*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied {
		t.Fatalf("mark=%+v", applied)
	}
	post, err := Claim(project, got.HandoffID, "/root/controller", "run-controller", now.Add(8*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !post.AlreadyApplied {
		t.Fatalf("claim=%+v", post)
	}
}


func TestClaimRejectsChangedContract(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	got, err := Prepare(PrepareRequest{
		Project: project, TaskID: "A-1", EventType: EventRegistrationReady,
		SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", ExecutionAuthorized: false,
	}, now)
	if err != nil { t.Fatal(err) }

	path := filepath.Join(project, "_task_mecca", "data", "backlog", "000001.A-1.handoff-test.todo.md")
	body, err := os.ReadFile(path); if err != nil { t.Fatal(err) }
	changed := string(body)
	changed = strings.Replace(changed, "이벤트 기반 인계를 검증한다.", "변경된 계약을 검증한다.", 1)
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil { t.Fatal(err) }

	claim, err := Claim(project, got.HandoffID, "/root/controller", "run-controller", now.Add(time.Second))
	if err != nil { t.Fatal(err) }
	if !claim.ContractChanged || claim.Claimed || claim.Reason != "contract_changed" {
		t.Fatalf("claim=%+v", claim)
	}
}


func TestClaimConflictsAcrossControllerAttempts(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	got, err := Prepare(PrepareRequest{
		Project: project, TaskID: "A-1", EventType: EventRegistrationReady,
		SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", ExecutionAuthorized: false,
	}, now)
	if err != nil { t.Fatal(err) }
	first, err := Claim(project, got.HandoffID, "/root/controller", "run-controller-1", now.Add(time.Second))
	if err != nil { t.Fatal(err) }
	if !first.Claimed { t.Fatalf("claim=%+v", first) }
	second, err := Claim(project, got.HandoffID, "/root/controller", "run-controller-2", now.Add(2*time.Second))
	if err != nil { t.Fatal(err) }
	if !second.ClaimConflict || second.Reason != "claimed_by_other_runtime_attempt" {
		t.Fatalf("claim=%+v", second)
	}
}
