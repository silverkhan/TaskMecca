package rolepolicy

import (
	"encoding/hex"
	"errors"
	"strings"
)

// ReviewLevel describes necessity, never a review result or a dispatch command.
type ReviewLevel string

const (
	ReviewRequired    ReviewLevel = "required"
	ReviewRecommended ReviewLevel = "recommended"
	ReviewOptional    ReviewLevel = "optional"
)

// ReviewRisk records actual effects. Unknown dimensions cannot establish low risk.
// Non-behavioral documentation may be optional even in a sensitive directory.
type ReviewRisk struct {
	Impact             string   `json:"impact"`        // low|moderate|high|unknown
	Complexity         string   `json:"complexity"`    // simple|complex|unknown
	Reversibility      string   `json:"reversibility"` // easy|difficult|unknown
	Verifiability      string   `json:"verifiability"` // established|limited|unknown
	BehaviorChanged    bool     `json:"behavior_changed"`
	ExplicitRequired   bool     `json:"explicit_required"`
	TrustBoundary      bool     `json:"trust_boundary"`
	FinancialIntegrity bool     `json:"financial_integrity"`
	Destructive        bool     `json:"destructive"`
	PriorFindings      []string `json:"prior_findings,omitempty"`
}

type ReviewRequirement struct {
	Risk           ReviewRisk `json:"risk"`
	Scope          []string   `json:"scope"`
	Rationale      string     `json:"rationale"`
	Omit           bool       `json:"omit,omitempty"`
	OmissionReason string     `json:"omission_reason,omitempty"`
}

type ReviewDecision struct {
	Level          ReviewLevel `json:"level"`
	Reason         string      `json:"reason"`
	Status         string      `json:"status"`
	CreateReviewer bool        `json:"create_reviewer"` // always false: Controller dispatches explicitly
}

func oneOf(value string, allowed ...string) bool {
	for _, a := range allowed {
		if value == a {
			return true
		}
	}
	return false
}

func ValidateRequirementScope(p *Policy) error {
	if p == nil || p.ReviewRequirement == nil {
		return nil
	}
	a, b := map[string]bool{}, map[string]bool{}
	for _, path := range p.Scope {
		a[path] = true
	}
	for _, path := range p.ReviewRequirement.Scope {
		b[path] = true
	}
	if len(a) != len(b) {
		return errors.New("review_requirement_must_cover_policy_scope")
	}
	for path := range a {
		if !b[path] {
			return errors.New("review_requirement_must_cover_policy_scope")
		}
	}
	return nil
}

// Routine execution facts cannot amend a confirmed implementation/review
// contract. Load applies this only to worker_metadata, not a genuine confirmed
// current-contract supplement. A new operational entry is not new consent.
func ValidateRequirementTransition(previous, next *Policy) error {
	if previous == nil {
		return nil
	}
	if previous.Method == "controller_exception" {
		return errors.New("routine_metadata_cannot_replace_controller_exception")
	}
	if previous.ReviewRequirement == nil {
		return nil
	}
	before, err := previous.ReviewRequirement.Decide()
	if err != nil {
		return err
	}
	if before.Level != ReviewRequired {
		return nil
	}
	if next == nil || next.ReviewRequirement == nil {
		return errors.New("routine_metadata_cannot_drop_required_review")
	}
	after, err := next.ReviewRequirement.Decide()
	if err != nil {
		return err
	}
	if after.Level != ReviewRequired || (previous.ReviewRequirement.Risk.ExplicitRequired && !next.ReviewRequirement.Risk.ExplicitRequired) {
		return errors.New("routine_metadata_cannot_weaken_required_review")
	}
	return nil
}

func (r ReviewRequirement) Decide() (ReviewDecision, error) {
	d := ReviewDecision{Level: ReviewRecommended, Status: "unassigned", Reason: "incomplete_or_complex_evidence"}
	x := r.Risk
	if !validScope(r.Scope) || strings.TrimSpace(r.Rationale) == "" ||
		!oneOf(x.Impact, "low", "moderate", "high", "unknown") ||
		!oneOf(x.Complexity, "simple", "complex", "unknown") ||
		!oneOf(x.Reversibility, "easy", "difficult", "unknown") ||
		!oneOf(x.Verifiability, "established", "limited", "unknown") {
		return d, errors.New("review_risk_scope_or_dimensions_invalid")
	}
	if x.ExplicitRequired || (x.BehaviorChanged && (x.Impact == "high" || x.TrustBoundary || x.FinancialIntegrity || x.Destructive || x.Reversibility == "difficult")) {
		d.Level, d.Reason = ReviewRequired, "explicit_or_high_risk_effect"
	} else if x.Impact == "low" && x.Complexity == "simple" && x.Reversibility == "easy" && x.Verifiability == "established" && len(x.PriorFindings) == 0 && !(x.BehaviorChanged && (x.TrustBoundary || x.FinancialIntegrity || x.Destructive)) {
		d.Level, d.Reason = ReviewOptional, "local_reversible_verified_effect"
	}
	if r.Omit {
		if d.Level == ReviewRequired {
			return d, errors.New("required_review_cannot_be_omitted")
		}
		if strings.TrimSpace(r.OmissionReason) == "" {
			return d, errors.New("review_omission_reason_required")
		}
		d.Status = "omitted"
	}
	return d, nil
}

// ReviewRequest is the shared A90/A91/A92 boundary. IDs, source hashes and
// observations are evidence references; none authorize execution or acceptance.
// Continuity/context remain separately observed, optional A91-owned payloads.
type ReviewRequest struct {
	TaskID         string            `json:"task_id"`
	AssignmentID   string            `json:"assignment_id"`
	ContractSHA256 string            `json:"contract_sha256"`
	Requirement    ReviewRequirement `json:"requirement"`
	Source         Source            `json:"source"`
	Implementation Identity          `json:"implementation"`
	Reviewer       Identity          `json:"reviewer"`
	ReportFile     string            `json:"report_file,omitempty"`
	ContinuityRef  string            `json:"continuity_ref,omitempty"`
	ContextRef     string            `json:"context_ref,omitempty"`
}

func ReviewerPath(path string) bool {
	const prefix = "/root/controller/reviewer/"
	return strings.HasPrefix(path, prefix) && metadataWorkerPath.MatchString("/root/controller/"+strings.TrimPrefix(path, prefix))
}

// ValidateImplementationHistory consumes verified runtime/Activity identities,
// not names or a caller's claim of independence. A changed turn is immaterial.
func ValidateImplementationHistory(task string, reviewer Identity, history []Identity) error {
	if reviewer.TaskID != task {
		return errors.New("reviewer_task_mismatch")
	}
	for _, h := range history {
		if h.Bound && h.Observed && ((h.NativeID != "" && h.NativeID == reviewer.NativeID) || (h.RuntimeAgentID != "" && h.RuntimeAgentID == reviewer.RuntimeAgentID)) {
			return errors.New("reviewer_has_product_implementation_history")
		}
	}
	return nil
}

// EvaluateReviewer authorizes only review collection. Evaluate remains the
// implementation gate and rejects this recipient's different role/identity.
func EvaluateReviewer(p *Policy, reviewer Identity, workspace string, source Source, history []Identity) View {
	v := View{Policy: p, Implementation: "reviewer_readonly", Review: "unknown", Reason: "reviewer_identity_or_source_unverified"}
	if p == nil || !reviewer.Bound || !reviewer.Observed || reviewer.NativeID == "" || reviewer.Provider == "" || reviewer.TaskID == "" ||
		p.Reviewer.Role != "reviewer" || !ReviewerPath(reviewer.AgentPath) || p.Reviewer.AgentPath != reviewer.AgentPath || p.Reviewer.NativeID != reviewer.NativeID ||
		p.Implementation.NativeID == "" || p.Implementation.NativeID == reviewer.NativeID || p.Workspace != workspace || !validScope(p.Scope) {
		return v
	}
	if p.ReviewRequirement != nil {
		if err := ValidateRequirementScope(p); err != nil {
			v.Reason = err.Error()
			return v
		}
		d, err := p.ReviewRequirement.Decide()
		if err != nil {
			v.Reason = err.Error()
			return v
		}
		if d.Status == "omitted" {
			v.Reason = "review_is_omitted"
			return v
		}
	}
	implementationVerified := false
	for _, h := range history {
		if h.Bound && h.Observed && h.TaskID == reviewer.TaskID && h.NativeID == p.Implementation.NativeID && h.AgentPath == p.Implementation.AgentPath {
			implementationVerified = true
		}
	}
	if !implementationVerified {
		v.Reason = "implementation_identity_unverified"
		return v
	}
	if err := ValidateImplementationHistory(reviewer.TaskID, reviewer, history); err != nil {
		v.Reason = err.Error()
		return v
	}
	if len(source.Files) == 0 {
		return v
	}
	present := false
	for path, hash := range source.Files {
		if !Covers(p.Scope, path) {
			return v
		}
		if hash != "absent" {
			_, err := hex.DecodeString(hash)
			if len(hash) != 64 || err != nil {
				return v
			}
			present = true
		}
	}
	for _, path := range p.Scope {
		covered := false
		for file := range source.Files {
			if Covers([]string{path}, file) {
				covered = true
			}
		}
		if !covered {
			return v
		}
	}
	if !present {
		return v
	}
	v.Allowed, v.Review, v.Reason = true, "requested", "reviewer_readonly_source_verified"
	return v
}
