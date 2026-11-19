package analysisdepth

import (
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"strings"
	"testing"
)

func small() Input {
	return Input{Current: Context{Task: "A-94", ContractSHA256: strings.Repeat("a", 64), AcceptanceSHA256: strings.Repeat("b", 64), Sources: map[string]string{"source.go": strings.Repeat("c", 64)}, Decisions: []string{}, OpenIssues: []string{}, ContextID: "current-context"}, Risk: rolepolicy.ReviewRisk{Impact: "low", Complexity: "simple", Reversibility: "easy", Verifiability: "established"}, RequiredChecks: []string{"contract-ac", "source"}, CompletedChecks: []string{"contract-ac", "source"}, CurrentPhase: "research", NextPhase: "implementation"}
}
func prior(in Input) *Prior {
	return &Prior{Context: in.Current, Risk: in.Risk, Artifact: "analysis/report.json", ResolvedChecks: []string{"contract-ac", "source"}}
}
func TestA94SmallFixAndMandatorySafety(t *testing.T) {
	in := small()
	a := Evaluate(in)
	if a.Depth != "minimal_confirmation" || a.NextAction != "proceed_to_implementation_with_fresh_authority" || a.ExecutionAuthorized || a.Written {
		t.Fatalf("small: %+v", a)
	}
	in.Risk.Destructive = true
	a = Evaluate(in)
	if a.Depth != "safety_investigation" || a.NextAction == "proceed_to_implementation_with_fresh_authority" {
		t.Fatalf("missing safety: %+v", a)
	}
	in.SafetyChecks = []string{"deletion-lineage"}
	a = Evaluate(in)
	if !contains(a.RemainingChecks, "deletion-lineage") {
		t.Fatal(a)
	}
	in.CompletedChecks = append(in.CompletedChecks, "deletion-lineage")
	if Evaluate(in).NextAction != "proceed_to_implementation_with_fresh_authority" {
		t.Fatal("resolved safety should permit advice")
	}
}
func TestA94ResumeAndChangedSource(t *testing.T) {
	in := small()
	in.Prior = prior(in)
	in.CompletedChecks = nil
	if a := Evaluate(in); a.Reuse != "exact" || len(a.RemainingChecks) != 0 {
		t.Fatal(a)
	}
	in.Current.Sources = map[string]string{"source.go": strings.Repeat("d", 64), "new.go": strings.Repeat("e", 64)}
	a := Evaluate(in)
	if a.Reuse != "changed_sources_only" || len(a.ChangedSources) != 2 || len(a.RemainingChecks) != 2 {
		t.Fatal(a)
	}
}
func TestA94StaleContractHashContextAndDecisions(t *testing.T) {
	cases := map[string]func(*Input){
		"contract_changed":       func(in *Input) { in.Current.ContractSHA256 = strings.Repeat("e", 64) },
		"acceptance_changed":     func(in *Input) { in.Current.AcceptanceSHA256 = strings.Repeat("e", 64) },
		"hash_mismatch":          func(in *Input) { in.Current.Sources = map[string]string{"source.go": "not-a-hash"} },
		"context_unknown":        func(in *Input) { in.Current.ContextID = "" },
		"context_rotation":       func(in *Input) { in.Current.ContextID = "new-context" },
		"prior_decision_changed": func(in *Input) { in.Current.Decisions = []string{"new-decision"} },
		"open_issue_changed":     func(in *Input) { in.Current.OpenIssues = []string{"unresolved"} },
		"task_changed":           func(in *Input) { in.Current.Task = "A-93" },
		"risk_changed":           func(in *Input) { in.Risk.Destructive = true },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			in := small()
			in.Prior = prior(in)
			in.CompletedChecks = nil
			change(&in)
			a := Evaluate(in)
			if a.Reuse != "none" || len(a.RemainingChecks) != 2 {
				t.Fatal(a)
			}
		})
	}
}
func TestA94ComplexReferencesPhaseGapAndDrift(t *testing.T) {
	in := small()
	in.Risk.Complexity = "complex"
	in.PhaseGap = true
	in.Evidence = []Evidence{{ContractRef: "original-contract", ArtifactRef: "proposal", ReferenceRef: "reference-index", Phase: "requirements", Purpose: "reference", Finding: "reference lookup recorded as requirements"}, {Phase: "research", Purpose: "repeat"}}
	in.Drift = &Drift{Cause: "lineage dependency discovered", CurrentArtifact: "proposal", RemainingChecks: []string{"lineage"}, NextAction: "Controller bound investigation"}
	a := Evaluate(in)
	if a.Depth != "focused_research" || !contains(a.Reasons, "phase_gap_or_unknown_preserve_unmeasured") || a.Classifications[0].Category != "research_reported_as_requirements" || a.Classifications[1].Category != "insufficient_evidence" || a.ControllerCoordination == nil {
		t.Fatal(a)
	}
	if in.Evidence[0].Phase != "requirements" {
		t.Fatal("rewrote historical phase")
	}
}
func TestA94B651CountsAloneDoNotEstablishOveranalysis(t *testing.T) {
	in := small()
	in.Evidence = []Evidence{{ContractRef: "B651 original pending comparison", ArtifactRef: "B651 proposal:21 targets/7933 metadata references", Phase: "requirements", Purpose: "repeat", Finding: "932901ms reported"}}
	a := Evaluate(in)
	if a.Classifications[0].Category != "insufficient_evidence" {
		t.Fatal(a)
	}
	for _, purpose := range []string{"safety", "repeat", "resume", "reference", "requirement"} {
		in.Evidence = []Evidence{{ContractRef: "contract", ArtifactRef: "actual-output", ReferenceRef: "actual-reference", Phase: "research", Purpose: purpose, Finding: "compared actual evidence"}}
		if Evaluate(in).Classifications[0].Category == "insufficient_evidence" {
			t.Fatal(purpose)
		}
	}
}

func TestA94BlankChecksUnknownAndDeletedSource(t *testing.T) {
	in := small()
	in.RequiredChecks = []string{" "}
	in.CompletedChecks = []string{" "}
	if Evaluate(in).NextAction == "proceed_to_implementation_with_fresh_authority" {
		t.Fatal("blank requirement accepted")
	}
	in = small()
	in.Current.ContextID = "unknown"
	if Evaluate(in).NextAction == "proceed_to_implementation_with_fresh_authority" {
		t.Fatal("unknown context accepted")
	}
	in = small()
	in.Prior = prior(in)
	in.Current.Sources = map[string]string{"new.go": strings.Repeat("c", 64)}
	if a := Evaluate(in); a.Reuse != "changed_sources_only" || len(a.ChangedSources) != 2 {
		t.Fatal(a)
	}
}
