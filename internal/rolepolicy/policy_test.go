package rolepolicy

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func workerIdentity() Identity {
	return Identity{AttemptID: "worker-run", NativeID: "actual-native-worker", Provider: "codex", AgentPath: "/root/controller/kkobugi", TaskID: "A-78", Bound: true, Observed: true}
}
func workerPolicy(workspace string) *Policy {
	return &Policy{SchemaVersion: 1, Implementation: Owner{Role: "worker", AgentPath: "/root/controller/kkobugi"}, Operational: Owner{Role: "controller", AgentPath: "/root/controller"}, Reviewer: Owner{Role: "worker", AgentPath: "/root/controller/pikachu", NativeID: "actual-native-reviewer"}, Workspace: workspace, Method: "worker", Scope: []string{"product.go"}}
}
func block(p *Policy) string { b, _ := json.Marshal(p); return Fence + string(b) + "\n```" }

func TestA78RoleDefaultAndRestrictedException(t *testing.T) {
	w := workerIdentity()
	p := workerPolicy("/isolated/source")
	if v := Evaluate(nil, w, "/isolated/source", false); !v.Allowed || !v.Legacy {
		t.Fatal(v)
	}
	controller := w
	controller.AgentPath = "/root/controller"
	// Sanitized B-638 shape: prose in a contract grants no product exception.
	phrase, e := Parse("Controller 직접 구현; 생성 실패, 슬롯 부족, worktree 제약")
	if e != nil || phrase != nil {
		t.Fatal(phrase, e)
	}
	if v := Evaluate(phrase, controller, p.Workspace, true); v.Allowed {
		t.Fatal(v)
	}
	if v := Evaluate(p, w, p.Workspace, false); !v.Allowed {
		t.Fatal(v)
	}
	if v := Evaluate(p, controller, p.Workspace, true); v.Allowed || v.Reason != "recipient_role_or_identity_mismatch" {
		t.Fatal(v)
	}
	p.Method = "controller_exception"
	p.Implementation = Owner{Role: "controller", AgentPath: "/root/controller"}
	p.Exception = &Exception{Reason: "approved bounded product repair", Evidence: "user decision ref", Scope: p.Scope, ApprovalEventID: "role-approval", ResumeWhen: "obtain scoped approval and separate actual reviewer"}
	if v := Evaluate(p, controller, p.Workspace, false); v.Allowed || v.ResumeWhen != p.Exception.ResumeWhen {
		t.Fatal(v)
	}
	if v := Evaluate(p, controller, p.Workspace, true); !v.Allowed {
		t.Fatal(v)
	}
	for _, name := range []string{"reason", "evidence", "scope", "reviewer", "workspace", "method", "role", "unbound"} {
		t.Run(name, func(t *testing.T) {
			b, _ := json.Marshal(p)
			var candidate Policy
			json.Unmarshal(b, &candidate)
			id := controller
			workspace := p.Workspace
			switch name {
			case "reason":
				candidate.Exception.Reason = ""
			case "evidence":
				candidate.Exception.Evidence = ""
			case "scope":
				candidate.Exception.Scope = []string{"elsewhere"}
			case "reviewer":
				candidate.Reviewer.NativeID = ""
			case "workspace":
				workspace = "/original"
			case "method":
				candidate.Method = "Controller directly"
			case "role":
				candidate.Implementation.Role = "worker"
			case "unbound":
				id.Bound = false
			}
			if v := Evaluate(&candidate, id, workspace, true); v.Allowed {
				t.Fatal(v)
			}
		})
	}
}

func TestA78ReviewNativeTaskSourceAndResult(t *testing.T) {
	w := workerIdentity()
	r := w
	r.NativeID = "actual-native-reviewer"
	r.AttemptID = "review-run"
	r.AgentPath = "/root/controller/pikachu"
	p := workerPolicy("/source")
	source := Source{Files: map[string]string{"product.go": strings.Repeat("a", 64)}}
	statement := ReviewStatement{TaskID: "A-78", ReviewerAttemptID: r.AttemptID, ImplementationNativeID: w.NativeID, Source: source, ReportSHA256: "report-hash", Result: "accepted"}
	if classification, e := ValidateReview("A-78", w, r, p, statement, "report-hash", source); e != nil || classification != "independent" {
		t.Fatal(classification, e)
	}
	for _, name := range []string{"same-native-different-turn", "different-task", "source-change", "report-change", "unbound-reviewer", "result", "asserted-identity", "scope"} {
		t.Run(name, func(t *testing.T) {
			reviewer := r
			st := statement
			current := source
			report := "report-hash"
			switch name {
			case "same-native-different-turn":
				reviewer.NativeID = w.NativeID
			case "different-task":
				st.TaskID = "B-621"
			case "source-change":
				current = Source{Files: map[string]string{"product.go": "changed"}}
			case "report-change":
				report = "changed"
			case "unbound-reviewer":
				reviewer.Bound = false
			case "result":
				st.Result = "needs_changes"
			case "asserted-identity":
				reviewer.Observed = false
			case "scope":
				current = Source{Files: map[string]string{"other.go": "x"}}
				st.Source = current
			}
			classification, e := ValidateReview("A-78", w, reviewer, p, st, report, current)
			if e == nil || classification == "independent" {
				t.Fatal(classification, e)
			}
			if name == "same-native-different-turn" && classification != "self" {
				t.Fatal(classification)
			}
		})
	}
}

func TestA78DecisionSupplementsApprovalAndLegacyCompatibility(t *testing.T) {
	project := t.TempDir()
	p := workerPolicy("/source")
	p.Method = "controller_exception"
	p.Implementation = Owner{Role: "controller", AgentPath: "/root/controller"}
	p.Exception = &Exception{Reason: "bounded repair", Evidence: "user policy selection", Scope: p.Scope, ApprovalEventID: "role-approval", ResumeWhen: "confirm"}
	raw := "# A-78 contract\n\n## 요건 정의서\n\n### 수용 기준\n\n- [ ] preserve source and links\n\n## 실행 정보\n\n## 원천 전문 (변경 금지)\n[original](/original/B-638.md) SHA256 preserved\n"
	dir := filepath.Join(project, "_task_mecca/data/contract_decisions/events")
	os.MkdirAll(dir, 0700)
	write := func(id, kind, text, base, roleSHA string) {
		t.Helper()
		b, _ := json.Marshal(map[string]any{"event_id": id, "task_id": "A-78", "kind": kind, "text": text, "policy_key": "implementation-role-exception", "evidence": "actual user decision ref", "user_confirmed": true, "base_sha256": base, "role_policy_sha256": roleSHA, "recorded_at": id})
		if e := os.WriteFile(filepath.Join(dir, id+".json"), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	hash := func(s string) string {
		f := filepath.Join(project, "hash")
		os.WriteFile(f, []byte(s), 0600)
		h, _ := HashFile(f)
		return h
	}
	base := hash(DecisionScopeRaw(raw))
	write("1-supplement", "supplement", block(p), base, "")
	write("2-execution", "execution_approved", "", base, "")
	loaded, approved, e := Load(project, "A-78", raw)
	if e != nil || loaded == nil || approved {
		t.Fatal(loaded, approved, e)
	}
	write("role-approval", "policy_confirmed", "", base, PolicyDigest(p))
	_, approved, e = Load(project, "A-78", raw)
	if e != nil || !approved {
		t.Fatal(approved, e)
	}
	checked := strings.Replace(raw, "- [ ]", "- [x]", 1)
	_, approved, e = Load(project, "A-78", checked)
	if e != nil || !approved {
		t.Fatal("checkbox compatibility", approved, e)
	}
	changed := strings.Replace(raw, "preserve source", "change source", 1)
	if _, _, e = Load(project, "A-78", changed); e == nil {
		t.Fatal("stale policy silently reused")
	}
	p.Exception.Scope = []string{"other.go"}
	write("1-supplement", "supplement", block(p), base, "")
	_, approved, e = Load(project, "A-78", raw)
	if e != nil || approved {
		t.Fatal("old role approval granted changed exception", approved, e)
	}
	if strings.Contains(raw, Fence) {
		t.Fatal("original source mutated")
	}
}

func TestA78CompleteSourceSnapshotAndOperationalChanges(t *testing.T) {
	dir := t.TempDir()
	if b, e := exec.Command("git", "init", "-q", dir).CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	os.Mkdir(filepath.Join(dir, "product"), 0700)
	file := filepath.Join(dir, "product", "source.go")
	os.WriteFile(file, []byte("one"), 0600)
	s, e := Snapshot(dir, []string{"product"})
	if e != nil || len(s.Files) != 1 {
		t.Fatal(s, e)
	}
	os.WriteFile(filepath.Join(dir, "operations.md"), []byte("normal ledger/integration note"), 0600)
	same, e := Snapshot(dir, []string{"product"})
	if e != nil || s.Digest() != same.Digest() {
		t.Fatal("operational change invalidated scope", same, e)
	}
	os.WriteFile(file, []byte("two"), 0600)
	changed, _ := Snapshot(dir, []string{"product"})
	if changed.Digest() == s.Digest() {
		t.Fatal("product change hidden")
	}
	os.WriteFile(filepath.Join(dir, "product", "new.go"), []byte("new source"), 0600)
	added, _ := Snapshot(dir, []string{"product"})
	if len(added.Files) != 2 {
		t.Fatal(added)
	}
	os.Symlink(filepath.Join(dir, "operations.md"), filepath.Join(dir, "product", "unsafe.go"))
	if _, e := Snapshot(dir, []string{"product"}); e == nil {
		t.Fatal("source symlink allowed")
	}
	if _, e := Parse(Fence + `{"schema_version":1,"independent":true}` + "\n```"); e == nil {
		t.Fatal("arbitrary assertion accepted")
	}
}
