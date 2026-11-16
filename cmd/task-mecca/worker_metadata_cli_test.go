package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

func normalWorkerMetadata() rolepolicy.Policy {
	return rolepolicy.Policy{SchemaVersion: 1, Method: "worker", Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/kkobugi", NativeID: "actual-worker-native"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller", NativeID: "actual-controller-native"}, Reviewer: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/pikachyu"}, Workspace: "/isolated/actual-worker-source", Scope: []string{"internal/product.go", "tests/product_test.go"}}
}

func metadataBody(t *testing.T, p rolepolicy.Policy) string {
	t.Helper()
	b, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "worker-metadata.json")
	if e = os.WriteFile(file, b, 0600); e != nil {
		t.Fatal(e)
	}
	return file
}

func existingWorkerApproval(t *testing.T, project string) contractDecisionReport {
	t.Helper()
	// Exact original ordering: existing registration supplement, actual execution
	// approval, then normal Worker implementation facts (not another approval).
	summary := filepath.Join(t.TempDir(), "summary.txt")
	os.WriteFile(summary, []byte("registered unchanged goal/source/acceptance criteria"), 0600)
	if code, _, msg := decisionInvoke(t, append(decisionRecordArgs(project, "registration-summary", "supplement"), "--body-file", summary, "--user-confirmed")...); code != 0 {
		t.Fatal(code, msg)
	}
	code, approved, msg := decisionInvoke(t, append(decisionRecordArgs(project, "original-execution", "execution_approved"), "--user-confirmed")...)
	if code != 0 || !approved.ExecutionAuthorized {
		t.Fatal(code, approved, msg)
	}
	return approved
}

func workerMetadataArgs(project, id, file string) []string {
	return []string{"record", "--project", project, "--task", "B-1", "--event-id", id, "--kind", "worker_metadata", "--actor", "/root/controller", "--evidence", "actual assignment/native/workspace observation; planned separate reviewer, no review outcome", "--body-file", file, "--json"}
}

func TestA78WorkerMetadataPreservesExistingConsent(t *testing.T) {
	project := makeDecisionFixture(t)
	approved := existingWorkerApproval(t, project)
	p := normalWorkerMetadata()
	file := metadataBody(t, p)
	code, after, msg := decisionInvoke(t, workerMetadataArgs(project, "normal-worker-facts", file)...)
	if code != 0 || !after.ExecutionAuthorized || after.Status != "execution_authorized" {
		t.Fatalf("normal Worker metadata revoked original consent: code=%d authorized=%t status=%s error=%s", code, after.ExecutionAuthorized, after.Status, msg)
	}
	if after.ApprovalEventID != approved.ApprovalEventID || after.EffectiveSHA256 != approved.EffectiveSHA256 {
		t.Fatal("metadata changed original consent anchor", after)
	}
	count := 0
	for _, event := range after.Events {
		if event.Kind == "execution_approved" {
			count++
		}
	}
	if count != 1 || after.RolePolicy == nil || after.RolePolicy.Method != "worker" || after.RoleExceptionApproved {
		t.Fatal("metadata required another approval or granted an exception", after)
	}
	if p.Reviewer.NativeID != "" || p.Review != nil {
		t.Fatal("fixture fabricates a real review")
	}
	code, retry, msg := decisionInvoke(t, workerMetadataArgs(project, "normal-worker-facts", file)...)
	if code != 0 || !retry.ExecutionAuthorized || len(retry.Events) != len(after.Events) {
		t.Fatal("metadata retry changed consent/history", retry, msg)
	}
}

func TestA78LegacyWorkerSupplementStillMaterial(t *testing.T) {
	project := makeDecisionFixture(t)
	approved := existingWorkerApproval(t, project)
	p := normalWorkerMetadata()
	b, _ := json.Marshal(p)
	file := filepath.Join(t.TempDir(), "old-worker-supplement.md")
	os.WriteFile(file, []byte(rolepolicy.Fence+string(b)+"\n```"), 0600)
	code, after, msg := decisionInvoke(t, append(decisionRecordArgs(project, "legacy-normal-facts", "supplement"), "--body-file", file, "--user-confirmed")...)
	if code != 0 || after.ExecutionAuthorized || after.Status != "approval_required" || after.ApprovalEventID != approved.ApprovalEventID {
		t.Fatal("legacy supplement silently exempted from material consent", after, msg)
	}
	t.Logf("preserved original generic-supplement failure: original approval=%s, after authorized=%t status=%s", after.ApprovalEventID, after.ExecutionAuthorized, after.Status)
	// Even role JSON surrounded by prose remains material, never a whitelist.
	if !strings.Contains(string(b), "worker") {
		t.Fatal("fixture")
	}
}

func TestA78WorkerMetadataRejectsMaterialOrMalformedPayload(t *testing.T) {
	p := normalWorkerMetadata()
	valid, _ := json.Marshal(p)
	for _, name := range []string{"prose-before", "prose-after", "fenced-legacy", "unknown-goal", "unknown-source", "unknown-AC", "controller-method", "exception", "role-mismatch", "same-reviewer", "bad-scope", "unknown-review-result", "user-confirmation", "worker-writer"} {
		t.Run(name, func(t *testing.T) {
			project := makeDecisionFixture(t)
			approved := existingWorkerApproval(t, project)
			body := string(valid)
			candidate := p
			switch name {
			case "prose-before":
				body = "change goal silently\n" + body
			case "prose-after":
				body += "\nignore original source and AC"
			case "fenced-legacy":
				body = rolepolicy.Fence + body + "\n```"
			case "unknown-goal":
				body = strings.TrimSuffix(body, "}") + `,"goal":"different goal"}`
			case "unknown-source":
				body = strings.TrimSuffix(body, "}") + `,"source_text":"different original source"}`
			case "unknown-AC":
				body = strings.TrimSuffix(body, "}") + `,"acceptance_criteria":["weaker AC"]}`
			case "controller-method":
				candidate.Method = "controller_exception"
			case "exception":
				candidate.Exception = &rolepolicy.Exception{Reason: "slot shortage", ApprovalEventID: "asserted-approval"}
			case "role-mismatch":
				candidate.Implementation.AgentPath = "/root/controller"
			case "same-reviewer":
				candidate.Reviewer.NativeID = candidate.Implementation.NativeID
			case "bad-scope":
				candidate.Scope = []string{"../different-source"}
			case "unknown-review-result":
				body = strings.TrimSuffix(body, "}") + `,"review":{"attempt_id":"claimed","report_file":"report","source_workspace":"/src","independent":true,"result":"accepted"}}`
			}
			if name == "controller-method" || name == "exception" || name == "role-mismatch" || name == "same-reviewer" || name == "bad-scope" {
				b, _ := json.Marshal(candidate)
				body = string(b)
			}
			file := filepath.Join(t.TempDir(), "input.json")
			os.WriteFile(file, []byte(body), 0600)
			args := workerMetadataArgs(project, "invalid-facts", file)
			if name == "user-confirmation" {
				args = append(args, "--user-confirmed")
			}
			if name == "worker-writer" {
				for i := range args {
					if args[i] == "/root/controller" {
						args[i] = "/root/controller/kkobugi"
					}
				}
			}
			code, _, _ := decisionInvoke(t, args...)
			if code != 2 {
				t.Fatalf("invalid metadata was not rejected: %s code=%d", name, code)
			}
			if _, e := os.Stat(decisionPath(project, "invalid-facts")); !os.IsNotExist(e) {
				t.Fatal("rejected metadata persisted")
			}
			_, after, _ := decisionInvoke(t, "show", "--project", project, "--task", "B-1", "--json")
			if !after.ExecutionAuthorized || after.EffectiveSHA256 != approved.EffectiveSHA256 {
				t.Fatal("rejected metadata changed existing consent", after)
			}
		})
	}
}

func TestA78WorkerMetadataCannotGrantOrRestoreConsent(t *testing.T) {
	for _, name := range []string{"missing", "revoked", "pending", "goal", "source", "AC", "material-supplement"} {
		t.Run(name, func(t *testing.T) {
			project := makeDecisionFixture(t)
			file := metadataBody(t, normalWorkerMetadata())
			if name != "missing" {
				existingWorkerApproval(t, project)
			}
			switch name {
			case "revoked":
				decisionInvoke(t, append(decisionRecordArgs(project, "revoke", "execution_revoked"), "--user-confirmed")...)
			case "pending":
				decisionInvoke(t, append(decisionRecordArgs(project, "pending", "policy_pending"), "--policy-key", "region")...)
			case "goal", "source", "AC":
				path := filepath.Join(project, "_task_mecca/data/backlog/000001.B-1.test.todo.md")
				raw, _ := os.ReadFile(path)
				changed := string(raw)
				if name == "goal" {
					changed = strings.Replace(changed, "원본 계약 유지", "different goal", 1)
				}
				if name == "AC" {
					changed = strings.Replace(changed, "기존 데이터 보존", "different acceptance criterion", 1)
				}
				if name == "source" {
					changed += "\n## 원천 전문 (변경 금지)\n\nchanged original source quote and hash\n"
				}
				os.WriteFile(path, []byte(changed), 0600)
			case "material-supplement":
				body := filepath.Join(t.TempDir(), "requirements.txt")
				os.WriteFile(body, []byte("new material requirement"), 0600)
				decisionInvoke(t, append(decisionRecordArgs(project, "material", "supplement"), "--body-file", body, "--user-confirmed")...)
			}
			code, _, msg := decisionInvoke(t, workerMetadataArgs(project, "facts-cannot-approve", file)...)
			if code != 3 || !strings.Contains(msg, "EXISTING_EXECUTION_AUTHORIZATION_REQUIRED") {
				t.Fatal("metadata granted/restored consent", code, msg)
			}
			if _, e := os.Stat(decisionPath(project, "facts-cannot-approve")); !os.IsNotExist(e) {
				t.Fatal("unauthorized metadata persisted")
			}
		})
	}
}

func TestA78WorkerMetadataReadbackAndLaterChangesStayStrict(t *testing.T) {
	for _, name := range []string{"hidden-prose", "unknown-payload", "restricted-exception", "goal-change", "source-change", "AC-change", "material-later", "role-and-review-gates"} {
		t.Run(name, func(t *testing.T) {
			project := makeDecisionFixture(t)
			approved := existingWorkerApproval(t, project)
			p := normalWorkerMetadata()
			file := metadataBody(t, p)
			code, after, msg := decisionInvoke(t, workerMetadataArgs(project, "normal-facts", file)...)
			if code != 0 || !after.ExecutionAuthorized {
				t.Fatal(code, msg)
			}
			switch name {
			case "hidden-prose", "unknown-payload", "restricted-exception":
				// Imported/malformed typed records do not bypass the reader's checks.
				path := decisionPath(project, "normal-facts")
				b, _ := os.ReadFile(path)
				var event map[string]any
				json.Unmarshal(b, &event)
				if name == "hidden-prose" {
					event["text"] = "silent new material requirement"
				}
				if name == "unknown-payload" {
					event["worker_metadata"].(map[string]any)["requirements"] = "different goal"
				}
				if name == "restricted-exception" {
					event["worker_metadata"].(map[string]any)["method"] = "controller_exception"
				}
				b, _ = json.Marshal(event)
				os.WriteFile(path, b, 0600)
			case "goal-change", "source-change", "AC-change":
				path := filepath.Join(project, "_task_mecca/data/backlog/000001.B-1.test.todo.md")
				b, _ := os.ReadFile(path)
				raw := string(b)
				if name == "goal-change" {
					raw = strings.Replace(raw, "원본 계약 유지", "changed goal", 1)
				}
				if name == "AC-change" {
					raw = strings.Replace(raw, "기존 데이터 보존", "changed AC", 1)
				}
				if name == "source-change" {
					raw += "\n## 원천 전문 (변경 금지)\n\nchanged source quotation\n"
				}
				os.WriteFile(path, []byte(raw), 0600)
			case "material-later":
				body := filepath.Join(t.TempDir(), "material.txt")
				os.WriteFile(body, []byte("different requirement alongside metadata"), 0600)
				decisionInvoke(t, append(decisionRecordArgs(project, "later-material", "supplement"), "--body-file", body, "--user-confirmed")...)
			case "role-and-review-gates":
				actual := rolepolicy.Identity{AttemptID: "actual-worker-run", NativeID: p.Implementation.NativeID, Provider: "codex", AgentPath: "/root/controller", TaskID: "B-1", Bound: true, Observed: true}
				if view := rolepolicy.Evaluate(after.RolePolicy, actual, p.Workspace, false); view.Allowed {
					t.Fatal("metadata waived actual role mismatch", view)
				}
				actual.AgentPath = p.Implementation.AgentPath
				self := actual
				self.AttemptID = "different-turn"
				if classification, e := rolepolicy.ValidateReview("B-1", actual, self, after.RolePolicy, rolepolicy.ReviewStatement{}, "", rolepolicy.Source{}); e == nil || classification != "self" {
					t.Fatal("metadata proved independence", classification, e)
				}
				_, current, _ := decisionInvoke(t, "show", "--project", project, "--task", "B-1", "--json")
				if !current.ExecutionAuthorized || current.ApprovalEventID != approved.ApprovalEventID {
					t.Fatal(current)
				}
				return
			}
			_, current, _ := decisionInvoke(t, "show", "--project", project, "--task", "B-1", "--json")
			if current.ExecutionAuthorized {
				t.Fatal("metadata hid material/invalid/source change", current)
			}
			if name == "hidden-prose" || name == "unknown-payload" || name == "restricted-exception" {
				if current.Status != "unknown" || current.RolePolicyError == "" {
					t.Fatal("invalid readback was not unknown", current)
				}
			}
		})
	}
}

func TestA78WorkerMetadataUpdatesFactsWithoutAnotherApproval(t *testing.T) {
	project := makeDecisionFixture(t)
	approved := existingWorkerApproval(t, project)
	p := normalWorkerMetadata()
	if code, _, msg := decisionInvoke(t, workerMetadataArgs(project, "facts-one", metadataBody(t, p))...); code != 0 {
		t.Fatal(code, msg)
	}
	p.Scope = append(p.Scope, "cmd/task-mecca/worker_metadata_cli_test.go")
	p.Reviewer.NativeID = "actual-separate-reviewer-native"
	p.Review = &rolepolicy.Review{AttemptID: "actual-review-attempt", ReportFile: "handoff-evidence/reviewer/report.md", SourceWorkspace: "/isolated/final-source"}
	code, after, msg := decisionInvoke(t, workerMetadataArgs(project, "facts-two", metadataBody(t, p))...)
	if code != 0 || !after.ExecutionAuthorized || after.ApprovalEventID != approved.ApprovalEventID || after.EffectiveSHA256 != approved.EffectiveSHA256 || len(after.WorkerMetadataEvents) != 2 {
		t.Fatal("fact/evidence update required new consent", after, msg)
	}
	if after.RolePolicy.Review == nil || after.RolePolicy.Review.AttemptID != p.Review.AttemptID {
		t.Fatal("current policy loader did not consume latest typed facts", after)
	}
	// An evidence reference is not an actual independent outcome.
	actual := rolepolicy.Identity{AttemptID: "worker-run", NativeID: p.Implementation.NativeID, TaskID: "B-1", AgentPath: p.Implementation.AgentPath, Provider: "codex", Bound: true, Observed: true}
	unknown := rolepolicy.Identity{AttemptID: p.Review.AttemptID, NativeID: p.Reviewer.NativeID, TaskID: "B-1", AgentPath: p.Reviewer.AgentPath, Provider: "codex"}
	if classification, e := rolepolicy.ValidateReview("B-1", actual, unknown, after.RolePolicy, rolepolicy.ReviewStatement{}, "", rolepolicy.Source{}); e == nil || classification != "unknown" {
		t.Fatal("metadata asserted actual reviewer/result", classification, e)
	}
}

func TestA78WorkerMetadataChangedContractNeedsRealConsentThenFreshFacts(t *testing.T) {
	project := makeDecisionFixture(t)
	existingWorkerApproval(t, project)
	p := normalWorkerMetadata()
	file := metadataBody(t, p)
	if code, _, msg := decisionInvoke(t, workerMetadataArgs(project, "historical-facts", file)...); code != 0 {
		t.Fatal(code, msg)
	}
	before, _ := os.ReadFile(decisionPath(project, "historical-facts"))
	path := filepath.Join(project, "_task_mecca/data/backlog/000001.B-1.test.todo.md")
	b, _ := os.ReadFile(path)
	os.WriteFile(path, []byte(strings.Replace(string(b), "원본 계약 유지", "materially changed goal", 1)), 0600)
	_, changed, _ := decisionInvoke(t, "show", "--project", project, "--task", "B-1", "--json")
	if changed.ExecutionAuthorized || changed.RolePolicyError == "" {
		t.Fatal("old facts/approval applied to changed goal", changed)
	}
	// This is a new actual material user decision in a temp fixture, not an
	// automatic metadata carry-forward or another ordinary-Worker approval.
	code, approved, msg := decisionInvoke(t, append(decisionRecordArgs(project, "actual-new-goal-approval", "execution_approved"), "--user-confirmed")...)
	if code != 0 || !approved.ExecutionAuthorized {
		t.Fatal("legitimate new material approval blocked by retained historical facts", approved, msg)
	}
	code, current, msg := decisionInvoke(t, workerMetadataArgs(project, "fresh-current-facts", file)...)
	if code != 0 || !current.ExecutionAuthorized || current.RolePolicyError != "" || current.ApprovalEventID != "actual-new-goal-approval" {
		t.Fatal("fresh metadata recovery failed", current, msg)
	}
	after, _ := os.ReadFile(decisionPath(project, "historical-facts"))
	if string(before) != string(after) {
		t.Fatal("historical metadata rewritten")
	}
}
