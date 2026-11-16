package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

func TestA78RoleApprovalIsSeparateAndDigestBound(t *testing.T) {
	project := makeDecisionFixture(t)
	p := rolepolicy.Policy{SchemaVersion: 1, Method: "controller_exception", Implementation: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Reviewer: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/reviewer", NativeID: "pending-actual-native"}, Workspace: "/isolated/product", Scope: []string{"source.go"}, Exception: &rolepolicy.Exception{Reason: "bounded repair", Evidence: "actual user decision ref", Scope: []string{"source.go"}, ApprovalEventID: "role-confirmed", ResumeWhen: "observe actual separate reviewer"}}
	b, _ := json.Marshal(p)
	body := filepath.Join(t.TempDir(), "policy.md")
	os.WriteFile(body, []byte(rolepolicy.Fence+string(b)+"\n```"), 0600)
	args := append(decisionRecordArgs(project, "policy-supplement", "supplement"), "--body-file", body, "--user-confirmed")
	code, report, msg := decisionInvoke(t, args...)
	if code != 0 || report.RolePolicy == nil || report.RoleExceptionApproved {
		t.Fatal(code, report, msg)
	}
	code, report, msg = decisionInvoke(t, append(decisionRecordArgs(project, "general", "execution_approved"), "--user-confirmed")...)
	if code != 0 || !report.ExecutionAuthorized || report.RoleExceptionApproved {
		t.Fatal(code, report, msg)
	}
	base := append(decisionRecordArgs(project, "role-confirmed", "policy_confirmed"), "--policy-key", "implementation-role-exception", "--user-confirmed")
	if code, _, _ := decisionInvoke(t, base...); code != 3 {
		t.Fatal("general confirmation granted exception", code)
	}
	if code, _, _ := decisionInvoke(t, append(base, "--role-policy-sha256", strings.Repeat("0", 64))...); code != 3 {
		t.Fatal("wrong digest accepted", code)
	}
	code, report, msg = decisionInvoke(t, append(base, "--role-policy-sha256", report.RolePolicySHA256)...)
	if code != 0 || !report.RoleExceptionApproved {
		t.Fatal(code, report, msg)
	}
	// Policy recording never fabricates actual reviewer execution or result.
	var out, errs bytes.Buffer
	code = runRolePolicyCLI([]string{"check", "--project", project, "--task", "B-1", "--attempt", "unbound-imported-json", "--source-worktree", p.Workspace, "--review", "--json"}, &out, &errs)
	if code != 3 || !strings.Contains(out.String(), "exact_task_attempt_required") {
		t.Fatal(code, out.String(), errs.String())
	}
}

func TestA78ManifestKeepsStructuredPolicyAndOriginalSource(t *testing.T) {
	p := &rolepolicy.Policy{SchemaVersion: 1, Method: "worker", Workspace: "/isolated/source", Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/kkobugi"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Reviewer: rolepolicy.Owner{Role: "worker"}, Scope: []string{"source.go"}}
	task := contractTask{Key: "A-78", Title: "policy", Kind: "defined", Goal: "preserve source", AcceptanceCriteria: []string{"exact sources"}, RolePolicy: p, SourceText: "[B-638](/original/B-638.md) SHA256:42ea44b02b75308707c66760f8bb11a1d0238a60c49dfe59ed0c2149a93359d7"}
	text := renderContract(task)
	parsed, e := rolepolicy.Parse(text)
	if e != nil || parsed == nil || parsed.Method != "worker" || !strings.Contains(text, task.SourceText) {
		t.Fatal(text, e)
	}
}
