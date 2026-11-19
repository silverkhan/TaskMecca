package backlog

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"os"
	"path/filepath"
	"testing"
)

func TestA90OmissionProjectionDoesNotInventIndependentAcceptance(t *testing.T) {
	project := t.TempDir()
	p := rolepolicy.Policy{SchemaVersion: 1, Method: "worker", Workspace: t.TempDir(), Scope: []string{"product.go"}, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/isanghaessi"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, ReviewRequirement: &rolepolicy.ReviewRequirement{Scope: []string{"product.go"}, Rationale: "local verified correction", Omit: true, OmissionReason: "Controller verified existing checks", Risk: rolepolicy.ReviewRisk{Impact: "low", Complexity: "simple", Reversibility: "easy", Verifiability: "established"}}}
	raw, _ := json.Marshal(p)
	text := "# A-90\n\n## 요건 정의서\n" + rolepolicy.Fence + string(raw) + "\n```\n\n## 실행 정보\n"
	dir := filepath.Join(project, "_task_mecca/data/backlog")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "000090.A-90.fixture.todo.md"), []byte(text), 0600)
	v := RolePolicyProjection(project, "A-90", text)
	if v["review"] != "omitted" || v["legacy"] != false || v["error"] != nil {
		t.Fatal(v)
	}
	if _, present := v["review_checked_at"]; present {
		t.Fatal("policy omission displayed actual independent acceptance", v)
	}
}

func TestA90ReviewerAliasReservesAllocatorNameWithoutRewritingHistory(t *testing.T) {
	if equivalent("/root/controller/reviewer/pikachyu") != equivalent("/root/controller/pikachyu") {
		t.Fatal("reviewer live identity failed to reserve alias")
	}
	if identityName("/root/controller/reviewer/raichyu") != "raichyu" {
		t.Fatal("management role became name")
	}
}
