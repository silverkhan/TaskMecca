package analysisdepth

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"strings"
	"testing"
)

func TestB2PhaseCategoriesKeepUnknownAndMetricsSeparate(t *testing.T) {
	for _, phase := range []string{"requirements", "research", "environment_identity", "implementation", "verification", "independent_review", "result_readback", "actual_wait"} {
		in := small()
		in.CurrentPhase, in.NextPhase = phase, phase
		in.Evidence = []Evidence{{ContractRef: "contract", ArtifactRef: "actual-phase", ReferenceRef: "original", Phase: phase, Purpose: "repeat", Finding: "same source confirmation"}}
		a := Evaluate(in)
		if contains(a.Reasons, "phase_gap_or_unknown_preserve_unmeasured") || a.Classifications[0].Category != "repeated_confirmation" {
			t.Fatal(phase, a)
		}
	}
	in := small()
	in.CurrentPhase = "unknown"
	if !contains(Evaluate(in).Reasons, "phase_gap_or_unknown_preserve_unmeasured") {
		t.Fatal("unknown phase became measured")
	}
}

func TestB2BoundedComparableAnalysisWork(t *testing.T) {
	in := small()
	in.CompletedChecks = nil
	baseline := Evaluate(in)
	in.Prior = prior(in)
	unchanged := Evaluate(in)
	if len(baseline.RemainingChecks) != 2 || len(unchanged.RemainingChecks) != 0 || unchanged.Reuse != "exact" || len(unchanged.FreshChecks) != len(baseline.FreshChecks) {
		t.Fatal(baseline, unchanged)
	}
	// A real delta must still be verified. This control makes no savings claim.
	in.Current.Sources = map[string]string{"source.go": strings.Repeat("d", 64)}
	changed := Evaluate(in)
	if changed.Reuse != "changed_sources_only" || len(changed.ChangedSources) != 1 || len(changed.RemainingChecks) != 2 {
		t.Fatal(changed)
	}
	for _, a := range []Advice{baseline, unchanged, changed} {
		if a.ExecutionAuthorized || a.Written {
			t.Fatal(a)
		}
	}
	t.Logf("bounded planning counts: unchanged analysis required checks %d -> %d; changed-source control checks=%d changed paths=%d; fresh checks retained=%d; no host-call or elapsed-time gain measured", len(baseline.RemainingChecks), len(unchanged.RemainingChecks), len(changed.RemainingChecks), len(changed.ChangedSources), len(unchanged.FreshChecks))
}

func TestB2ReviewAdviceCannotEscapeAnalysisContext(t *testing.T) {
	in := small()
	in.ReviewPreflight = &rolepolicy.ReviewPreflightInput{}
	in.ReviewFollowup = &rolepolicy.ReviewFollowupInput{Cause: "report_transport"}
	a := Evaluate(in)
	if a.ReviewPreflight.Ready || a.ReviewFollowup.Action != "blocked" || !contains(a.ReviewPreflight.Findings, "analysis_review_context_mismatch") {
		t.Fatal(a)
	}
	b, err := json.Marshal(a)
	if err != nil || !strings.Contains(string(b), `"acceptance_granted":false`) {
		t.Fatal(string(b), err)
	}
}

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
