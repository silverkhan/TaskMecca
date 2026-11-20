// Package analysisdepth provides read-only advice, never execution authority.
package analysisdepth

import (
	"encoding/hex"
	"reflect"
	"sort"
	"strings"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

// Context is the exact identity of analysis, separate from current authority.
type Context struct {
	Task             string            `json:"task"`
	ContractSHA256   string            `json:"contract_sha256"`
	AcceptanceSHA256 string            `json:"acceptance_sha256"`
	Sources          map[string]string `json:"sources"`
	Decisions        []string          `json:"decisions"`
	OpenIssues       []string          `json:"open_issues"`
	ContextID        string            `json:"context_id"`
}

type Prior struct {
	Context        Context               `json:"context"`
	Risk           rolepolicy.ReviewRisk `json:"risk"`
	Artifact       string                `json:"artifact"`
	ResolvedChecks []string              `json:"resolved_checks"`
}

// Evidence preserves the reported phase and distinguishes substantive causes.
// Elapsed time and file counts deliberately are not decision inputs.
type Evidence struct {
	ContractRef  string `json:"contract_ref"`
	ArtifactRef  string `json:"artifact_ref"`
	ReferenceRef string `json:"reference_ref"`
	Phase        string `json:"phase"`
	Purpose      string `json:"purpose"` // requirement|safety|reference|repeat|resume
	Finding      string `json:"finding"`
}

type Drift struct {
	Cause           string   `json:"cause"`
	CurrentArtifact string   `json:"current_artifact"`
	RemainingChecks []string `json:"remaining_checks"`
	NextAction      string   `json:"next_action"`
}

type Input struct {
	Current         Context                          `json:"current"`
	Risk            rolepolicy.ReviewRisk            `json:"risk"`
	Prior           *Prior                           `json:"prior,omitempty"`
	RequiredChecks  []string                         `json:"required_checks"`
	SafetyChecks    []string                         `json:"safety_checks"`
	CompletedChecks []string                         `json:"completed_checks"`
	Evidence        []Evidence                       `json:"evidence"`
	CurrentPhase    string                           `json:"current_phase"`
	NextPhase       string                           `json:"next_phase"`
	PhaseGap        bool                             `json:"phase_gap"`
	Drift           *Drift                           `json:"drift,omitempty"`
	Counterexamples *rolepolicy.CounterexampleMatrix `json:"counterexamples,omitempty"`
	ReviewPreflight *rolepolicy.ReviewPreflightInput `json:"review_preflight,omitempty"`
	ReviewFollowup  *rolepolicy.ReviewFollowupInput  `json:"review_followup,omitempty"`
}

type Classification struct {
	Evidence Evidence `json:"evidence"`
	Category string   `json:"category"`
}

type Advice struct {
	Advisory               bool                              `json:"advisory"`
	ExecutionAuthorized    bool                              `json:"execution_authorized"`
	Written                bool                              `json:"written"`
	Depth                  string                            `json:"depth"`
	NextAction             string                            `json:"next_action"`
	StopCriteria           []string                          `json:"stop_criteria"`
	Reuse                  string                            `json:"reuse"`
	ChangedSources         []string                          `json:"changed_sources"`
	RemainingChecks        []string                          `json:"remaining_checks"`
	Reasons                []string                          `json:"reasons"`
	Classifications        []Classification                  `json:"classifications"`
	ControllerCoordination *Drift                            `json:"controller_coordination,omitempty"`
	FreshChecks            []string                          `json:"fresh_checks"`
	PhaseInstruction       string                            `json:"phase_instruction"`
	Counterexamples        *rolepolicy.CounterexampleAdvice  `json:"counterexamples,omitempty"`
	ReviewPreflight        *rolepolicy.ReviewPreflightAdvice `json:"review_preflight,omitempty"`
	ReviewFollowup         *rolepolicy.ReviewFollowupAdvice  `json:"review_followup,omitempty"`
}

func hash(s string) bool { b, e := hex.DecodeString(s); return e == nil && len(b) == 32 }
func valid(c Context) bool {
	if strings.TrimSpace(c.Task) == "" || strings.TrimSpace(c.ContextID) == "" || strings.EqualFold(strings.TrimSpace(c.ContextID), "unknown") || !hash(c.ContractSHA256) || !hash(c.AcceptanceSHA256) || len(c.Sources) == 0 || c.Decisions == nil || c.OpenIssues == nil {
		return false
	}
	for p, h := range c.Sources {
		if strings.TrimSpace(p) == "" || !hash(h) {
			return false
		}
	}
	return true
}
func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
func phases(s string) bool {
	return contains([]string{"requirements", "research", "environment_identity", "implementation", "verification", "independent_review", "result_readback", "actual_wait", "coordination_handoff"}, s)
}

// Evaluate does not open files or write journals. Its input is explicit evidence;
// the caller must read current contract/source and independently verify authority.
func Evaluate(in Input) Advice {
	a := Advice{Advisory: true, Depth: "focused", Reuse: "none", ChangedSources: []string{}, RemainingChecks: []string{}, Reasons: []string{}, Classifications: []Classification{},
		FreshChecks:      []string{"current instructions and permissions", "actual dispatch/current native binding and strict gate"},
		StopCriteria:     []string{"confirmed contract and acceptance criteria understood", "required safety and task checks resolved with evidence", "remaining uncertainty bounded and next implementation/verification action explicit"},
		PhaseInstruction: "Record only the actual current boundary using supported APIs; preserve original phases, timestamps and unmeasured gaps. Advice writes nothing."}
	currentValid := valid(in.Current)
	if !currentValid {
		a.Reasons = append(a.Reasons, "current_context_unknown_or_invalid")
	}
	resolved := append([]string{}, in.CompletedChecks...)
	if p := in.Prior; p != nil {
		same := currentValid && valid(p.Context) && p.Artifact != "" && reflect.DeepEqual(p.Risk, in.Risk) && p.Context.Task == in.Current.Task && p.Context.ContractSHA256 == in.Current.ContractSHA256 && p.Context.AcceptanceSHA256 == in.Current.AcceptanceSHA256 && p.Context.ContextID == in.Current.ContextID && reflect.DeepEqual(p.Context.Decisions, in.Current.Decisions) && reflect.DeepEqual(p.Context.OpenIssues, in.Current.OpenIssues)
		if same {
			for path, h := range in.Current.Sources {
				if p.Context.Sources[path] != h {
					a.ChangedSources = append(a.ChangedSources, path)
				}
			}
			for path := range p.Context.Sources {
				if _, ok := in.Current.Sources[path]; !ok {
					a.ChangedSources = append(a.ChangedSources, path)
				}
			}
			sort.Strings(a.ChangedSources)
			if len(a.ChangedSources) == 0 {
				a.Reuse = "exact"
				resolved = append(resolved, p.ResolvedChecks...)
			} else {
				a.Reuse = "changed_sources_only"
				a.Reasons = append(a.Reasons, "recheck_changed_sources_and_affected_decisions_checks")
			}
		} else {
			a.Reasons = append(a.Reasons, "prior_identity_contract_decisions_issues_or_context_mismatch")
		}
	}
	for _, c := range append(append([]string{}, in.RequiredChecks...), in.SafetyChecks...) {
		if c != "" && !contains(resolved, c) && !contains(a.RemainingChecks, c) {
			a.RemainingChecks = append(a.RemainingChecks, c)
		}
	}
	r := in.Risk
	known := contains([]string{"low", "moderate", "high"}, r.Impact) && contains([]string{"simple", "complex"}, r.Complexity) && contains([]string{"easy", "difficult"}, r.Reversibility) && contains([]string{"established", "limited"}, r.Verifiability)
	high := r.Impact == "high" || r.Destructive || r.TrustBoundary || r.FinancialIntegrity || r.Reversibility == "difficult"
	if high {
		a.Depth = "safety_investigation"
		a.Reasons = append(a.Reasons, "retain_mandatory_safety_investigation")
	} else if !known || r.Complexity == "complex" || r.Verifiability == "limited" {
		a.Depth = "focused_research"
	} else if r.Impact == "low" && r.Complexity == "simple" {
		a.Depth = "minimal_confirmation"
	}
	if !known {
		a.Reasons = append(a.Reasons, "risk_unknown_requires_clarification")
	}
	if len(in.RequiredChecks) == 0 {
		a.Reasons = append(a.Reasons, "required_checks_not_defined")
	}
	if high && len(in.SafetyChecks) == 0 {
		a.Reasons = append(a.Reasons, "mandatory_safety_checks_not_defined")
	}
	a.NextAction = "resolve_remaining_checks_then_implement"
	checksValid := true
	for _, check := range append(append([]string{}, in.RequiredChecks...), in.SafetyChecks...) {
		if strings.TrimSpace(check) == "" {
			checksValid = false
		}
	}
	if !checksValid {
		a.Reasons = append(a.Reasons, "blank_check_is_not_a_defined_requirement")
	}
	if currentValid && known && checksValid && len(in.RequiredChecks) > 0 && (!high || len(in.SafetyChecks) > 0) && len(a.RemainingChecks) == 0 {
		a.NextAction = "proceed_to_implementation_with_fresh_authority"
	}
	if in.PhaseGap || !phases(in.CurrentPhase) || !phases(in.NextPhase) {
		a.Reasons = append(a.Reasons, "phase_gap_or_unknown_preserve_unmeasured")
	}
	for _, e := range in.Evidence {
		category := "insufficient_evidence"
		if e.ContractRef != "" && e.ArtifactRef != "" && e.ReferenceRef != "" && e.Finding != "" && phases(e.Phase) {
			switch e.Purpose {
			case "safety":
				category = "necessary_safety_investigation"
			case "repeat":
				category = "repeated_confirmation"
			case "resume":
				category = "resume_reconstruction_cost"
			case "reference":
				category = "reference_research"
				if e.Phase == "requirements" {
					category = "research_reported_as_requirements"
				}
			case "requirement":
				category = "requirements_understanding"
			}
		}
		a.Classifications = append(a.Classifications, Classification{e, category})
	}
	if in.Drift != nil {
		a.ControllerCoordination = in.Drift
		a.Reasons = append(a.Reasons, "coordinate_scope_drift_with_controller_not_elapsed_user_stall")
		if in.Drift.Cause == "" || in.Drift.CurrentArtifact == "" || len(in.Drift.RemainingChecks) == 0 || in.Drift.NextAction == "" {
			a.Reasons = append(a.Reasons, "drift_report_incomplete")
		}
	}
	if in.Counterexamples != nil {
		x := rolepolicy.CheckCounterexamples(*in.Counterexamples, in.Current.Task, rolepolicy.Source{Files: in.Current.Sources})
		a.Counterexamples = &x
	}
	if in.ReviewPreflight != nil {
		x := rolepolicy.PreflightReview(*in.ReviewPreflight)
		if !currentValid || in.ReviewPreflight.Request.TaskID != in.Current.Task || in.ReviewPreflight.ContractSHA256 != in.Current.ContractSHA256 || in.ReviewPreflight.CurrentSource.Digest() != (rolepolicy.Source{Files: in.Current.Sources}).Digest() {
			x.Ready = false
			x.Findings = append(x.Findings, "analysis_review_context_mismatch")
		}
		a.ReviewPreflight = &x
	}
	if in.ReviewFollowup != nil {
		x := rolepolicy.ClassifyReviewFollowup(*in.ReviewFollowup)
		if !currentValid || in.ReviewFollowup.Original.Request.TaskID != in.Current.Task || in.ReviewFollowup.CurrentContractSHA256 != in.Current.ContractSHA256 || in.ReviewFollowup.CurrentSource.Digest() != (rolepolicy.Source{Files: in.Current.Sources}).Digest() {
			x.Action = "blocked"
			x.Reasons = append(x.Reasons, "analysis_review_context_mismatch")
		}
		a.ReviewFollowup = &x
	}
	return a
}
