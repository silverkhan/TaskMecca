package handoff

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func a78RevisionFixture(t *testing.T) (string, string, *rolepolicy.Policy, time.Time, rolepolicy.ReviewStatement) {
	t.Helper()
	project, source, p, now := a78PolicyFixture(t)
	if b, e := exec.Command("git", "init", "-q", source).CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	os.MkdirAll(filepath.Join(source, "product"), 0700)
	os.WriteFile(filepath.Join(source, "product/source.go"), []byte("package product\n"), 0600)
	os.WriteFile(filepath.Join(source, ".gitignore"), []byte("generated.go\n"), 0600)
	report := filepath.Join(source, "review.md")
	os.WriteFile(report, []byte("sanitized distinct actual reviewer fixture"), 0600)
	p.Implementation.NativeID = "worker-native"
	p.Scope = []string{"product"}
	p.Review = &rolepolicy.Review{AttemptID: "review-run", ReportFile: report, SourceWorkspace: source}
	file := filepath.Join(project, "_task_mecca/data/backlog/000001.A-1.handoff-test.todo.md")
	b, _ := os.ReadFile(file)
	start := strings.Index(string(b), rolepolicy.Fence)
	end := strings.Index(string(b)[start+len(rolepolicy.Fence):], "\n```") + start + len(rolepolicy.Fence) + 4
	encoded, _ := json.Marshal(p)
	os.WriteFile(file, []byte(string(b[:start])+rolepolicy.Fence+string(encoded)+"\n```"+string(b[end:])), 0600)
	for _, e := range []runtimeobs.ExecutionEvent{
		{EventKind: "state", ObservedAt: now.Add(-10 * time.Second).Format(time.RFC3339Nano), AttemptID: "review-run", Provider: "codex", SessionID: "reviewer-native", RuntimeAgentID: "reviewer-native", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved},
		{EventKind: "binding", ObservedAt: now.Add(-9 * time.Second).Format(time.RFC3339Nano), AttemptID: "review-run", TaskID: "A-1", AgentPath: p.Reviewer.AgentPath, BindingSource: "dispatch", EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative},
	} {
		if err := runtimeobs.AppendExecutionEvent(project, e); err != nil {
			t.Fatal(err)
		}
	}
	scope, e := rolepolicy.Snapshot(source, p.Scope)
	if e != nil {
		t.Fatal(e)
	}
	h, e := rolepolicy.HashFile(report)
	if e != nil {
		t.Fatal(e)
	}
	return project, source, p, now, rolepolicy.ReviewStatement{TaskID: "A-1", ReviewerAttemptID: "review-run", ImplementationNativeID: "worker-native", Source: scope, ReportSHA256: h, Result: "accepted"}
}

func a78AppendDecisionFixture(t *testing.T, project, source string, p *rolepolicy.Policy, kind, task string, now time.Time) {
	t.Helper()
	file := filepath.Join(project, "_task_mecca/data/backlog/000001.A-1.handoff-test.todo.md")
	raw, _ := os.ReadFile(file)
	anchor := filepath.Join(source, "decision-anchor")
	os.WriteFile(anchor, []byte(rolepolicy.DecisionScopeRaw(string(raw))), 0600)
	base, e := rolepolicy.HashFile(anchor)
	if e != nil {
		t.Fatal(e)
	}
	event := map[string]any{"event_id": "current-change", "kind": kind, "task_id": task, "actor": "/root/controller", "evidence": "actual sanitized fixture decision/facts", "base_sha256": base, "effective_sha256": "fixture-existing-consent-hash", "recorded_at": now.Format(time.RFC3339Nano)}
	if kind == "worker_metadata" {
		event["worker_metadata"] = p
	} else if kind == "policy_pending" {
		event["user_confirmed"] = false
		event["policy_key"] = "region"
	} else {
		event["user_confirmed"] = true
		if kind == "supplement" {
			event["text"] = "material supplemental requirement"
		}
	}
	dir := filepath.Join(project, "_task_mecca/data/contract_decisions/events")
	os.MkdirAll(dir, 0700)
	payload, _ := json.Marshal(event)
	if e = os.WriteFile(filepath.Join(dir, "current-change.json"), payload, 0600); e != nil {
		t.Fatal(e)
	}
	if kind == "worker_metadata" {
		loaded, _, e := rolepolicy.Load(project, task, string(raw))
		if e != nil || loaded.Reviewer.NativeID != p.Reviewer.NativeID {
			t.Fatal("valid typed metadata fixture failed", loaded, e)
		}
	}
}

func TestA78CurrentInputsRevalidatedAfterNativeRead(t *testing.T) {
	for _, name := range []string{"reviewer-native", "review-reference", "scope", "consent-revoked", "policy-pending", "material-supplement", "goal-source", "acceptance-checkbox", "report", "ignored-source", "operational-note", "other-task"} {
		t.Run(name, func(t *testing.T) {
			project, source, p, now, statement := a78RevisionFixture(t)
			reader := func(_ context.Context, _ runtimeobs.Attempt, task, hash string, _ time.Time) (rolepolicy.ReviewStatement, error) {
				if task != "A-1" || hash != statement.ReportSHA256 {
					t.Fatal("wrong actual native query target")
				}
				switch name {
				case "reviewer-native":
					p.Reviewer.NativeID = "new-required-reviewer-native"
					a78AppendDecisionFixture(t, project, source, p, "worker_metadata", "A-1", now)
				case "review-reference":
					p.Review.AttemptID = "new-reviewer-attempt"
					a78AppendDecisionFixture(t, project, source, p, "worker_metadata", "A-1", now)
				case "scope":
					p.Scope = append(p.Scope, "extra.go")
					os.WriteFile(filepath.Join(source, "extra.go"), []byte("new source"), 0600)
					a78AppendDecisionFixture(t, project, source, p, "worker_metadata", "A-1", now)
				case "consent-revoked":
					a78AppendDecisionFixture(t, project, source, p, "execution_revoked", "A-1", now)
				case "policy-pending":
					a78AppendDecisionFixture(t, project, source, p, "policy_pending", "A-1", now)
				case "acceptance-checkbox":
					file := filepath.Join(project, "_task_mecca/data/backlog/000001.A-1.handoff-test.todo.md")
					b, _ := os.ReadFile(file)
					os.WriteFile(file, []byte(strings.Replace(string(b), "- [ ]", "- [x]", 1)), 0600)
				case "material-supplement":
					a78AppendDecisionFixture(t, project, source, p, "supplement", "A-1", now)
				case "goal-source":
					file := filepath.Join(project, "_task_mecca/data/backlog/000001.A-1.handoff-test.todo.md")
					b, _ := os.ReadFile(file)
					os.WriteFile(file, append(b, []byte("\n## 원천 전문 (변경 금지)\nchanged material source\n")...), 0600)
				case "report":
					os.WriteFile(p.Review.ReportFile, []byte("changed report"), 0600)
				case "ignored-source":
					os.WriteFile(filepath.Join(source, "product/generated.go"), []byte("package product\nfunc Unreviewed() {}\n"), 0600)
				case "operational-note":
					file := filepath.Join(project, "_task_mecca/data/backlog/000001.A-1.handoff-test.todo.md")
					b, _ := os.ReadFile(file)
					os.WriteFile(file, append(b, []byte("\noperational progress note only\n")...), 0600)
				case "other-task":
					a78AppendDecisionFixture(t, project, source, p, "supplement", "A-99", now)
				}
				return statement, nil
			}
			a, e := assessPolicy(project, "A-1", "worker-run", source, true, now, reader)
			if name == "operational-note" || name == "other-task" || name == "acceptance-checkbox" {
				if e != nil || a.Review != "independent" {
					t.Fatal("unrelated/operational change invalidated review", a, e)
				}
				return
			}
			if e == nil || a.Review == "independent" {
				t.Fatal("stale native verification accepted", name, a, e)
			}
		})
	}
}

func TestA78FinalAcceptanceAppendRechecksAssessment(t *testing.T) {
	project, source, p, now, statement := a78RevisionFixture(t)
	reader := func(context.Context, runtimeobs.Attempt, string, string, time.Time) (rolepolicy.ReviewStatement, error) {
		return statement, nil
	}
	a, e := assessPolicy(project, "A-1", "worker-run", source, true, now, reader)
	if e != nil || a.Review != "independent" {
		t.Fatal(a, e)
	}
	row := Handoff{TaskID: "A-1", EventType: EventWorkerDone, SourceAttemptID: "worker-run", SourceWorkspace: source, Target: Target{SessionID: "controller-native"}}
	b, _ := json.Marshal(map[string]any{"detail": "supported assessment fixture", "role_policy_review": a})
	if e = validateAcceptanceEvidenceCurrent(project, row, string(b), now); e != nil {
		t.Fatal("unchanged supported assessment rejected", e)
	}
	p.Reviewer.NativeID = "different-required-native"
	a78AppendDecisionFixture(t, project, source, p, "worker_metadata", "A-1", now)
	if e = validateAcceptanceEvidenceCurrent(project, row, string(b), now); e == nil {
		t.Fatal("final acceptance/applied append reused stale assessment")
	}
}
