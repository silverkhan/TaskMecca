package main

import (
	"bytes"
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"os"
	"path/filepath"
	"testing"
)

func TestA90ReviewDecisionCLIHasNoDispatchOrToolClaim(t *testing.T) {
	p := filepath.Join(t.TempDir(), "risk.json")
	r := rolepolicy.ReviewRequirement{Scope: []string{"product.go"}, Rationale: "verified reversible local correction", Risk: rolepolicy.ReviewRisk{Impact: "low", Complexity: "simple", Reversibility: "easy", Verifiability: "established"}}
	raw, _ := json.Marshal(r)
	os.WriteFile(p, raw, 0600)
	var out, stderr bytes.Buffer
	if code := runRolePolicyCLI([]string{"review-decision", "--input", p, "--json"}, &out, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	var d map[string]any
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d["dispatch_authorized"] != false || d["external_tool_performed"] != false {
		t.Fatal(d)
	}
	if runtimeQueryRole("/root/controller/reviewer/raichyu") != "reviewer" || runtimeQueryRole("/root/controller/raichyu") != "worker" {
		t.Fatal("query role distinction")
	}
	r.Risk.ExplicitRequired = true
	r.Omit = true
	r.OmissionReason = "speed"
	raw, _ = json.Marshal(r)
	os.WriteFile(p, raw, 0600)
	out.Reset()
	stderr.Reset()
	if code := runRolePolicyCLI([]string{"review-decision", "--input", p}, &out, &stderr); code != 3 {
		t.Fatal(code, stderr.String())
	}
}
