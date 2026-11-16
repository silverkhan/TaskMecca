package backlog

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/contractscope"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestA78RoleProjectionUsesCurrentSupportedReceipt(t *testing.T) {
	project, source := t.TempDir(), t.TempDir()
	now := time.Now().Add(-time.Minute)
	if b, e := exec.Command("git", "init", "-q", source).CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	product := filepath.Join(source, "product.go")
	os.WriteFile(product, []byte("reviewed product"), 0600)
	report := filepath.Join(source, "review.md")
	os.WriteFile(report, []byte("separate review fixture"), 0600)
	p := &rolepolicy.Policy{SchemaVersion: 1, Method: "worker", Workspace: source, Scope: []string{"product.go"}, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/kkobugi"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Reviewer: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/reviewer"}, Review: &rolepolicy.Review{AttemptID: "review-run", ReportFile: report, SourceWorkspace: source}}
	b, _ := json.Marshal(p)
	raw := "# A-78 fixture\n\n## 요건 정의서\n\n" + rolepolicy.Fence + string(b) + "\n```\n\n## 실행 정보\n"
	identities := map[string]rolepolicy.Identity{}
	for _, actor := range []struct{ attempt, native, path string }{{"worker-run", "actual-worker", p.Implementation.AgentPath}, {"review-run", "actual-reviewer", p.Reviewer.AgentPath}} {
		for _, event := range []runtimeobs.ExecutionEvent{
			{EventKind: "state", AttemptID: actor.attempt, Provider: "codex", SessionID: actor.native, RuntimeAgentID: actor.native, State: runtimeobs.StateRunning, ObservedAt: now.Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved},
			{EventKind: "binding", AttemptID: actor.attempt, TaskID: "A-78", AgentPath: actor.path, BindingSource: "dispatch", ObservedAt: now.Add(time.Second).Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative},
		} {
			if e := runtimeobs.AppendExecutionEvent(project, event); e != nil {
				t.Fatal(e)
			}
		}
	}
	l, _ := runtimeobs.BuildLedger(project, 20, time.Now())
	for _, a := range l.Attempts {
		identities[a.AttemptID] = runtimeobs.PolicyIdentity(a)
	}
	if v := RolePolicyProjection(project, "A-78", raw); v["review"] != "unknown" {
		t.Fatal(v)
	}
	files, _ := rolepolicy.Snapshot(source, p.Scope)
	hash, _ := rolepolicy.HashFile(report)
	statement := rolepolicy.ReviewStatement{TaskID: "A-78", ReviewerAttemptID: "review-run", ImplementationNativeID: "actual-worker", Source: files, ReportSHA256: hash, Result: "accepted"}
	receipt, _ := json.Marshal(map[string]any{"detail": "sanitized supported acceptance fixture; no real reviewer claim", "role_policy_review": map[string]any{"policy_sha256": rolepolicy.PolicyDigest(p), "implementation_identity": identities["worker-run"], "review_identity": identities["review-run"], "statement": statement}})
	path := handoffjournal.JournalPath(project)
	os.MkdirAll(filepath.Dir(path), 0700)
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	enc := json.NewEncoder(f)
	enc.Encode(handoffjournal.Record{HandoffID: "fixture", RecordKind: "prepared", TaskID: "A-78", SourceAttemptID: "worker-run", ContractSHA256: contractscope.Fingerprint(raw), ObservedAt: now.Format(time.RFC3339Nano)})
	enc.Encode(handoffjournal.Record{HandoffID: "fixture", RecordKind: "step_succeeded", Step: "acceptance", Result: "ok", Evidence: map[string]string{"detail": string(receipt)}, ObservedAt: now.Add(2 * time.Second).Format(time.RFC3339Nano)})
	f.Close()
	if v := RolePolicyProjection(project, "A-78", raw); v["review"] != "independent" {
		t.Fatal(v)
	}
	os.WriteFile(product, []byte("later product change"), 0600)
	if v := RolePolicyProjection(project, "A-78", raw); v["review"] != "unknown" {
		t.Fatal("stale review still independent", v)
	}
	legacy := RolePolicyProjection(project, "A-78", "# legacy\n\n## 작업 정의\nController 직접 구현\n")
	if legacy["review"] != "unknown" || legacy["implementation"] != "worker_default" {
		t.Fatal(legacy)
	}
}
