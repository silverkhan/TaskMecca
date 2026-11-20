package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silverkhan/TaskMecca/internal/analysisdepth"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

func TestB2AnalysisCLIExposesStructuralAdviceWithoutAuthority(t *testing.T) {
	p := filepath.Join(t.TempDir(), "b2-analysis.json")
	in := analysisdepth.Input{
		Current:        analysisdepth.Context{Task: "B-2", ContractSHA256: strings.Repeat("a", 64), AcceptanceSHA256: strings.Repeat("b", 64), Sources: map[string]string{"product.go": strings.Repeat("c", 64)}, Decisions: []string{}, OpenIssues: []string{}, ContextID: "current"},
		Risk:           rolepolicy.ReviewRisk{Impact: "moderate", Complexity: "complex", Reversibility: "easy", Verifiability: "established"},
		RequiredChecks: []string{"source"}, CompletedChecks: []string{"source"}, CurrentPhase: "environment_identity", NextPhase: "independent_review",
		Counterexamples: &rolepolicy.CounterexampleMatrix{TaskID: "B-2", Source: rolepolicy.Source{Files: map[string]string{"product.go": strings.Repeat("c", 64)}}, Required: []string{"negative"}, Cases: []rolepolicy.Counterexample{{ID: "negative", Question: "Can negative intent grant authority?", Input: "do not act", Expected: "deny", Result: "not_run"}}},
		ReviewPreflight: &rolepolicy.ReviewPreflightInput{}, ReviewFollowup: &rolepolicy.ReviewFollowupInput{Cause: "identity_recovery"},
	}
	body, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p, body, 0600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := runAnalysisDepthCLI(p, "B-2", &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var a analysisdepth.Advice
	if err = json.Unmarshal(out.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if !a.Advisory || a.ExecutionAuthorized || a.Written || a.Counterexamples.Ready || a.Counterexamples.Cases[0].Result != "not_run" || a.ReviewPreflight.Ready || a.ReviewPreflight.AcceptanceGranted || a.ReviewFollowup.Action != "blocked" {
		t.Fatal(a)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(body, after) {
		t.Fatal("input was changed")
	}
}

func TestA94FlowAnalysisReadOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), "analysis.json")
	body := `{"current":{"task":"A-94","contract_sha256":"` + strings.Repeat("a", 64) + `","acceptance_sha256":"` + strings.Repeat("b", 64) + `","sources":{"source.go":"` + strings.Repeat("c", 64) + `"},"decisions":[],"open_issues":[],"context_id":"current"},"risk":{"impact":"low","complexity":"simple","reversibility":"easy","verifiability":"established"},"required_checks":["contract"],"completed_checks":["contract"],"current_phase":"research","next_phase":"implementation"}`
	if e := os.WriteFile(p, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	var out, errs bytes.Buffer
	code := runFlowNextCLI([]string{"next", "--role", "worker", "--event", "analysis_requested", "--task", "A-94", "--analysis-input", p, "--json"}, &out, &errs)
	if code != 0 {
		t.Fatal(code, errs.String())
	}
	var a map[string]any
	if e := json.Unmarshal(out.Bytes(), &a); e != nil {
		t.Fatal(e)
	}
	if a["advisory"] != true || a["execution_authorized"] != false || a["written"] != false {
		t.Fatal(a)
	}
	after, _ := os.ReadFile(p)
	if string(after) != body {
		t.Fatal("input changed")
	}
	for _, bad := range []string{body + ` {}`, strings.Replace(body, `"current_phase"`, `"elapsed_ms":932901,"current_phase"`, 1), strings.Replace(body, `"task":"A-94"`, `"task":"A-93"`, 1)} {
		if e := os.WriteFile(p, []byte(bad), 0600); e != nil {
			t.Fatal(e)
		}
		out.Reset()
		errs.Reset()
		if code := runAnalysisDepthCLI(p, "A-94", &out, &errs); code != 2 {
			t.Fatal(code, out.String(), errs.String())
		}
	}
}

func TestA94CurrentMaintenanceEvidence(t *testing.T) {
	p := os.Getenv("A94_CURRENT_FIXTURE")
	if p == "" {
		t.Skip("current maintenance fixture supplied only by isolated selected run")
	}
	var out, errs bytes.Buffer
	if code := runFlowNextCLI([]string{"next", "--role", "worker", "--event", "analysis_requested", "--task", "A-94", "--analysis-input", p, "--json"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var a map[string]any
	if err := json.Unmarshal(out.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if a["depth"] != "focused_research" || a["reuse"] != "exact" || a["execution_authorized"] != false {
		t.Fatal(a)
	}
	t.Log("current maintenance advice", out.String())
}

func TestA94OversizedInputRejected(t *testing.T) {
	p := filepath.Join(t.TempDir(), "large.json")
	if err := os.WriteFile(p, []byte(`{}`+strings.Repeat(" ", 2<<20)), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := runAnalysisDepthCLI(p, "A-94", &out, &errs); code != 2 {
		t.Fatal(code)
	}
}
