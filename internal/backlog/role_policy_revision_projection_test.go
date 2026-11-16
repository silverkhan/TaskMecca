package backlog

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/contractscope"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type projectionRevisionFixture struct {
	project, source, raw, contract, report string
	policy                                 *rolepolicy.Policy
	receipt                                map[string]any
	now                                    time.Time
}

func newProjectionRevisionFixture(t *testing.T) *projectionRevisionFixture {
	t.Helper()
	f := &projectionRevisionFixture{project: t.TempDir(), source: t.TempDir(), now: time.Now().Add(-time.Minute)}
	if b, e := exec.Command("git", "init", "-q", f.source).CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	os.MkdirAll(filepath.Join(f.source, "product"), 0700)
	os.WriteFile(filepath.Join(f.source, "product/base.go"), []byte("package product\n"), 0600)
	os.WriteFile(filepath.Join(f.source, ".gitignore"), []byte("generated.go\n"), 0600)
	f.report = filepath.Join(f.source, "review.md")
	os.WriteFile(f.report, []byte("sanitized actual separate review fixture"), 0600)
	f.policy = &rolepolicy.Policy{SchemaVersion: 1, Method: "worker", Workspace: f.source, Scope: []string{"product"}, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/kkobugi", NativeID: "actual-worker"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller", NativeID: "actual-controller"}, Reviewer: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/reviewer", NativeID: "actual-reviewer"}, Review: &rolepolicy.Review{AttemptID: "review-run", ReportFile: f.report, SourceWorkspace: f.source}}
	p, _ := json.Marshal(f.policy)
	f.raw = "# A-78 projection fixture\n\n## 요건 정의서\n\n### 목표\nunchanged goal\n\n" + rolepolicy.Fence + string(p) + "\n```\n\n### 수용 기준\n- [ ] preserve current evidence\n\n## 실행 정보\n\n## 작업 노트\n"
	f.contract = filepath.Join(f.project, "_task_mecca/data/backlog/000078.A-78.fixture.todo.md")
	os.MkdirAll(filepath.Dir(f.contract), 0700)
	os.WriteFile(f.contract, []byte(f.raw), 0600)
	identities := map[string]rolepolicy.Identity{}
	for _, actor := range []struct{ attempt, native, path string }{{"worker-run", "actual-worker", f.policy.Implementation.AgentPath}, {"review-run", "actual-reviewer", f.policy.Reviewer.AgentPath}} {
		for _, event := range []runtimeobs.ExecutionEvent{
			{EventKind: "state", AttemptID: actor.attempt, Provider: "codex", SessionID: actor.native, RuntimeAgentID: actor.native, State: runtimeobs.StateRunning, ObservedAt: f.now.Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved},
			{EventKind: "binding", AttemptID: actor.attempt, TaskID: "A-78", AgentPath: actor.path, BindingSource: "dispatch", ObservedAt: f.now.Add(time.Second).Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative},
		} {
			if e := runtimeobs.AppendExecutionEvent(f.project, event); e != nil {
				t.Fatal(e)
			}
		}
	}
	l, e := runtimeobs.BuildLedger(f.project, 20, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	for _, a := range l.Attempts {
		identities[a.AttemptID] = runtimeobs.PolicyIdentity(a)
	}
	source, e := rolepolicy.Snapshot(f.source, f.policy.Scope)
	if e != nil {
		t.Fatal(e)
	}
	hash, e := rolepolicy.HashFile(f.report)
	if e != nil {
		t.Fatal(e)
	}
	revision, e := rolepolicy.ReviewRevision(f.project, "A-78", f.raw)
	if e != nil {
		t.Fatal(e)
	}
	statement := rolepolicy.ReviewStatement{TaskID: "A-78", ReviewerAttemptID: "review-run", ImplementationNativeID: "actual-worker", Source: source, ReportSHA256: hash, Result: "accepted"}
	f.receipt = map[string]any{"input_revision": revision, "report_sha256": hash, "source": source, "policy_sha256": rolepolicy.PolicyDigest(f.policy), "implementation_identity": identities["worker-run"], "review_identity": identities["review-run"], "statement": statement}
	f.writeReceipt(t)
	return f
}

func (f *projectionRevisionFixture) writeReceipt(t *testing.T) {
	t.Helper()
	payload, e := json.Marshal(map[string]any{"detail": "supported current assessment fixture, no real review claim", "role_policy_review": f.receipt})
	if e != nil {
		t.Fatal(e)
	}
	path := handoffjournal.JournalPath(f.project)
	os.MkdirAll(filepath.Dir(path), 0700)
	file, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.Encode(handoffjournal.Record{HandoffID: "fixture", RecordKind: "prepared", TaskID: "A-78", EventType: handoffjournal.EventWorkerDone, SourceAgentPath: f.policy.Implementation.AgentPath, SourceWorkspace: f.source, SourceAttemptID: "worker-run", ContractSHA256: contractscope.Fingerprint(f.raw), TargetSessionID: "actual-controller", ObservedAt: f.now.Format(time.RFC3339Nano)})
	enc.Encode(handoffjournal.Record{HandoffID: "fixture", RecordKind: "step_succeeded", Step: "acceptance", Result: "ok", Evidence: map[string]string{"detail": string(payload)}, ObservedAt: f.now.Add(2 * time.Second).Format(time.RFC3339Nano)})
}

func (f *projectionRevisionFixture) decision(t *testing.T, kind, task string) {
	t.Helper()
	anchor := filepath.Join(f.source, "anchor")
	os.WriteFile(anchor, []byte(rolepolicy.DecisionScopeRaw(f.raw)), 0600)
	base, e := rolepolicy.HashFile(anchor)
	if e != nil {
		t.Fatal(e)
	}
	event := map[string]any{"event_id": "current-change", "kind": kind, "task_id": task, "actor": "/root/controller", "evidence": "sanitized real changed requirement or observed facts", "base_sha256": base, "effective_sha256": "fixture-consent-hash", "recorded_at": time.Now().Format(time.RFC3339Nano)}
	if kind == "worker_metadata" {
		event["worker_metadata"] = f.policy
	} else if kind == "policy_pending" {
		event["policy_key"] = "region"
	} else {
		event["user_confirmed"] = true
		if kind == "supplement" {
			event["text"] = "new material audit-proof requirement"
		}
	}
	dir := filepath.Join(f.project, "_task_mecca/data/contract_decisions/events")
	os.MkdirAll(dir, 0700)
	b, _ := json.Marshal(event)
	os.WriteFile(filepath.Join(dir, "current-change.json"), b, 0600)
}

func TestA78ProjectionRequiresCurrentReceiptRevision(t *testing.T) {
	for _, name := range []string{"material", "revoked", "pending", "reviewer-native", "review-reference", "scope", "goal-cached-row", "goal-fresh-row", "missing-contract", "legacy-revision", "missing-source", "missing-report-hash", "wrong-report", "wrong-source", "ignored-source", "statement-task", "statement-reviewer", "operational-note", "checkbox", "other-task"} {
		t.Run(name, func(t *testing.T) {
			f := newProjectionRevisionFixture(t)
			if v := RolePolicyProjection(f.project, "A-78", f.raw); v["review"] != "independent" {
				t.Fatal("valid current receipt rejected", v)
			}
			switch name {
			case "material":
				f.decision(t, "supplement", "A-78")
			case "revoked":
				f.decision(t, "execution_revoked", "A-78")
			case "pending":
				f.decision(t, "policy_pending", "A-78")
			case "reviewer-native":
				f.policy.Reviewer.NativeID = "new-required-native"
				f.decision(t, "worker_metadata", "A-78")
			case "review-reference":
				newReport := filepath.Join(f.source, "new-report.md")
				b, _ := os.ReadFile(f.report)
				os.WriteFile(newReport, b, 0600)
				f.policy.Review.ReportFile = newReport
				f.decision(t, "worker_metadata", "A-78")
			case "scope":
				f.policy.Scope = append(f.policy.Scope, "new.go")
				os.WriteFile(filepath.Join(f.source, "new.go"), []byte("package candidate"), 0600)
				f.decision(t, "worker_metadata", "A-78")
			case "goal-cached-row", "goal-fresh-row":
				raw := strings.Replace(f.raw, "unchanged goal", "new material goal", 1)
				os.WriteFile(f.contract, []byte(raw), 0600)
				if name == "goal-fresh-row" {
					f.raw = raw
				}
			case "missing-contract":
				os.Remove(f.contract)
			case "legacy-revision":
				delete(f.receipt, "input_revision")
				f.writeReceipt(t)
			case "missing-source":
				delete(f.receipt, "source")
				f.writeReceipt(t)
			case "missing-report-hash":
				delete(f.receipt, "report_sha256")
				f.writeReceipt(t)
			case "wrong-report":
				os.WriteFile(f.report, []byte("changed review report"), 0600)
			case "wrong-source":
				os.WriteFile(filepath.Join(f.source, "product/base.go"), []byte("changed product"), 0600)
			case "ignored-source":
				os.WriteFile(filepath.Join(f.source, "product/generated.go"), []byte("package product\nfunc Unreviewed(){}\n"), 0600)
			case "statement-task":
				s := f.receipt["statement"].(rolepolicy.ReviewStatement)
				s.TaskID = "B-621"
				f.receipt["statement"] = s
				f.writeReceipt(t)
			case "statement-reviewer":
				s := f.receipt["statement"].(rolepolicy.ReviewStatement)
				s.ReviewerAttemptID = "different-run"
				f.receipt["statement"] = s
				f.writeReceipt(t)
			case "operational-note":
				os.WriteFile(f.contract, []byte(f.raw+"\nnormal operational note\n"), 0600)
			case "checkbox":
				os.WriteFile(f.contract, []byte(strings.Replace(f.raw, "- [ ]", "- [x]", 1)), 0600)
			case "other-task":
				f.decision(t, "supplement", "A-79")
			}
			v := RolePolicyProjection(f.project, "A-78", f.raw)
			if name == "operational-note" || name == "checkbox" || name == "other-task" {
				if v["review"] != "independent" {
					t.Fatal("compatible change lost current receipt", v)
				}
				return
			}
			if v["review"] == "independent" || v["review_reason"] == "supported_acceptance_receipt_current_source" || v["source"] != nil {
				t.Fatal("obsolete/incomplete receipt still projected current independent", name, v)
			}
		})
	}
}
