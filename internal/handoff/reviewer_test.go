package handoff

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestA90OptionalAcceptanceIsOmittedAndSourceBound(t *testing.T) {
	project, source, p, now := a78PolicyFixture(t)
	file := filepath.Join(project, "_task_mecca/data/backlog/000001.A-1.handoff-test.todo.md")
	write := func() {
		raw, _ := os.ReadFile(file)
		a := strings.Index(string(raw), rolepolicy.Fence)
		b := strings.Index(string(raw)[a+len(rolepolicy.Fence):], "\n```") + a + len(rolepolicy.Fence)
		policy, _ := json.Marshal(p)
		os.WriteFile(file, []byte(string(raw)[:a]+rolepolicy.Fence+string(policy)+string(raw)[b:]), 0600)
	}
	p.ReviewRequirement = &rolepolicy.ReviewRequirement{Scope: p.Scope, Rationale: "small correction validated by existing check", Omit: true, OmissionReason: "Controller confirms local verified effect", Risk: rolepolicy.ReviewRisk{Impact: "low", Complexity: "simple", Reversibility: "easy", Verifiability: "established"}}
	p.Reviewer = rolepolicy.Owner{}
	write()
	a, err := AssessPolicy(project, "A-1", "worker-run", source, true, now.Add(5*time.Second))
	if err != nil || a.Review != "omitted" || !a.Allowed || a.Statement != nil || a.Source == nil {
		t.Fatal(a, err)
	}
	if err := revalidatePolicyAssessment(project, "A-1", source, &a, now.Add(6*time.Second)); err != nil || a.Review != "omitted" {
		t.Fatal(a, err)
	}
	os.WriteFile(filepath.Join(source, "source.go"), []byte("package candidate\nfunc Changed(){}\n"), 0600)
	if err := revalidatePolicyAssessment(project, "A-1", source, &a, now.Add(7*time.Second)); err == nil {
		t.Fatal("omission receipt copied to new source")
	}
	p.ReviewRequirement.Risk.ExplicitRequired = true
	write()
	if _, err := AssessPolicy(project, "A-1", "worker-run", source, true, now.Add(8*time.Second)); err == nil {
		t.Fatal("explicit review silently omitted")
	}
}
