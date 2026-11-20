package rolepolicy

import (
	"encoding/hex"
	"reflect"
	"strings"
)

// These helpers check supplied evidence structurally. They do not observe a
// native session, attest a judgment, launch a Reviewer, or grant acceptance.
type Counterexample struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Input    string `json:"input"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Result   string `json:"result"` // pass | fail | not_run
	Evidence string `json:"evidence"`
}

type CounterexampleMatrix struct {
	TaskID   string           `json:"task_id"`
	Source   Source           `json:"source"`
	Required []string         `json:"required"` // chosen for the actual change/risk
	Cases    []Counterexample `json:"cases"`
}

type CounterexampleAdvice struct {
	Advisory bool             `json:"advisory"`
	Ready    bool             `json:"ready"`
	Findings []string         `json:"findings"`
	Cases    []Counterexample `json:"cases"`
}

func preflightHash(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32
}

func preflightSource(s Source) bool {
	if len(s.Files) == 0 {
		return false
	}
	for path, hash := range s.Files {
		if strings.TrimSpace(path) == "" || (hash != "absent" && !preflightHash(hash)) {
			return false
		}
	}
	return true
}

func CheckCounterexamples(m CounterexampleMatrix, task string, source Source) CounterexampleAdvice {
	a := CounterexampleAdvice{Advisory: true, Findings: []string{}, Cases: m.Cases}
	if task == "" || m.TaskID != task || !preflightSource(source) || m.Source.Digest() != source.Digest() {
		a.Findings = append(a.Findings, "counterexample_task_or_source_mismatch")
	}
	if len(m.Required) == 0 {
		a.Findings = append(a.Findings, "relevant_counterexamples_not_defined")
	}
	seen := map[string]Counterexample{}
	for _, c := range m.Cases {
		if _, ok := seen[c.ID]; ok || strings.TrimSpace(c.ID) == "" {
			a.Findings = append(a.Findings, "counterexample_duplicate_or_blank_id")
		}
		seen[c.ID] = c
		if strings.TrimSpace(c.Question) == "" || strings.TrimSpace(c.Input) == "" || strings.TrimSpace(c.Expected) == "" || !oneOf(c.Result, "pass", "fail", "not_run") {
			a.Findings = append(a.Findings, "counterexample_incomplete:"+c.ID)
		}
		if c.Result == "pass" && (strings.TrimSpace(c.Actual) == "" || strings.TrimSpace(c.Evidence) == "") {
			a.Findings = append(a.Findings, "counterexample_pass_without_result_evidence:"+c.ID)
		}
		if c.Result == "fail" || c.Result == "not_run" {
			a.Findings = append(a.Findings, "counterexample_unresolved:"+c.ID)
		}
	}
	for _, id := range m.Required {
		c, ok := seen[id]
		if strings.TrimSpace(id) == "" || !ok || c.Result != "pass" || c.Actual == "" || c.Evidence == "" {
			a.Findings = append(a.Findings, "counterexample_unresolved:"+id)
		}
	}
	a.Ready = len(a.Findings) == 0
	return a
}

type ReviewPreflightInput struct {
	Request               ReviewRequest    `json:"request"`
	Policy                *Policy          `json:"policy"`
	OwnIdentity           Identity         `json:"own_identity"`
	Workspace             string           `json:"workspace"`
	ContractSHA256        string           `json:"contract_sha256"`
	CurrentSource         Source           `json:"current_source"`
	ImplementationHistory []Identity       `json:"implementation_history"`
	Statement             *ReviewStatement `json:"statement"`
	ReportSHA256          string           `json:"report_sha256"` // independently computed from the actual report
}

type ReviewPreflightAdvice struct {
	Advisory            bool            `json:"advisory"`
	ExecutionAuthorized bool            `json:"execution_authorized"`
	AcceptanceGranted   bool            `json:"acceptance_granted"`
	Ready               bool            `json:"ready"` // structural readiness only, including before terminal
	Requirement         *ReviewDecision `json:"requirement,omitempty"`
	Findings            []string        `json:"findings"`
	FreshChecks         []string        `json:"fresh_checks"`
}

func PreflightReview(in ReviewPreflightInput) ReviewPreflightAdvice {
	a := ReviewPreflightAdvice{Advisory: true, Findings: []string{}, FreshChecks: []string{
		"fresh authorization, actual own native binding and complete current source",
		"Reviewer-owned original judgment and native final statement provenance",
		"independent terminal observation before collecting a final result",
	}}
	r := in.Request
	if r.TaskID == "" || r.AssignmentID == "" || !preflightHash(in.ContractSHA256) || r.ContractSHA256 != in.ContractSHA256 {
		a.Findings = append(a.Findings, "review_request_contract_or_assignment_unverified")
	}
	if !reflect.DeepEqual(r.Reviewer, in.OwnIdentity) {
		a.Findings = append(a.Findings, "review_own_identity_mismatch")
	}
	if in.OwnIdentity.AttemptID == "" {
		a.Findings = append(a.Findings, "review_own_attempt_missing")
	}
	if len(in.CurrentSource.Files) == 0 || r.Source.Digest() != in.CurrentSource.Digest() {
		a.Findings = append(a.Findings, "review_request_source_changed")
	}
	if in.Policy == nil || in.Policy.ReviewRequirement == nil || !reflect.DeepEqual(*in.Policy.ReviewRequirement, r.Requirement) {
		a.Findings = append(a.Findings, "review_requirement_not_current")
	} else if d, err := r.Requirement.Decide(); err != nil {
		a.Findings = append(a.Findings, err.Error())
	} else {
		a.Requirement = &d
	}
	v := EvaluateReviewer(in.Policy, in.OwnIdentity, in.Workspace, in.CurrentSource, in.ImplementationHistory)
	if !v.Allowed {
		a.Findings = append(a.Findings, v.Reason)
	}
	i := r.Implementation
	if in.Policy == nil || !i.Bound || !i.Observed || i.TaskID != r.TaskID || i.NativeID == "" || i.AttemptID == "" || i.Provider == "" || in.Policy.Implementation.NativeID != i.NativeID || in.Policy.Implementation.AgentPath != i.AgentPath {
		a.Findings = append(a.Findings, "review_implementation_identity_unverified")
	}
	found := false
	for _, h := range in.ImplementationHistory {
		if reflect.DeepEqual(h, i) {
			found = true
		}
	}
	if !found {
		a.Findings = append(a.Findings, "review_implementation_history_missing")
	}
	if err := ValidateImplementationHistory(r.TaskID, in.OwnIdentity, in.ImplementationHistory); err != nil {
		a.Findings = append(a.Findings, err.Error())
	}
	s := in.Statement
	if s == nil {
		a.Findings = append(a.Findings, "review_statement_missing")
	} else {
		if s.TaskID != r.TaskID || s.ReviewerAttemptID != in.OwnIdentity.AttemptID || s.ImplementationNativeID != i.NativeID {
			a.Findings = append(a.Findings, "review_statement_identity_mismatch")
		}
		if !oneOf(s.Result, "accepted", "needs_changes") {
			a.Findings = append(a.Findings, "review_judgment_missing_or_unknown")
		}
		if !preflightHash(in.ReportSHA256) || s.ReportSHA256 != in.ReportSHA256 {
			a.Findings = append(a.Findings, "review_report_hash_mismatch")
		}
		if s.Source.Digest() != in.CurrentSource.Digest() {
			a.Findings = append(a.Findings, "review_statement_source_changed")
		}
	}
	a.Ready = len(a.Findings) == 0
	return a
}

type ReviewFollowupInput struct {
	Cause                  string               `json:"cause"` // report_transport | source_change | new_defect | identity_recovery
	Original               ReviewPreflightInput `json:"original"`
	CurrentSource          Source               `json:"current_source"`
	CurrentContractSHA256  string               `json:"current_contract_sha256"`
	OriginalState          string               `json:"original_state"`
	OriginalNativeFinalRef string               `json:"original_native_final_ref"`
	OriginalTerminalRef    string               `json:"original_terminal_ref"`
	OriginalErrors         []string             `json:"original_errors"`
}

type ReviewFollowupAdvice struct {
	Advisory            bool                `json:"advisory"`
	ExecutionAuthorized bool                `json:"execution_authorized"`
	AcceptanceGranted   bool                `json:"acceptance_granted"`
	Action              string              `json:"action"`
	Reasons             []string            `json:"reasons"`
	Original            ReviewFollowupInput `json:"original"` // retain actual statement, identities, hashes, references and failures
	FreshChecks         []string            `json:"fresh_checks"`
}

// A transport supplement carries an already attested result. It is not a new
// code review or permission to resume a terminal Reviewer. Provenance still
// needs independent readback: supplied references are not attestations.
func ClassifyReviewFollowup(in ReviewFollowupInput) ReviewFollowupAdvice {
	a := ReviewFollowupAdvice{Advisory: true, Action: "blocked", Reasons: []string{}, Original: in,
		FreshChecks: []string{"re-read original native final statement, terminal evidence and report hash", "fresh current authorization, complete source and independence/history", "use existing collection/acceptance gates; advice grants neither"}}
	if !oneOf(in.Cause, "report_transport", "source_change", "new_defect", "identity_recovery") {
		a.Reasons = append(a.Reasons, "followup_cause_unknown")
		return a
	}
	if !preflightSource(in.CurrentSource) || !preflightHash(in.CurrentContractSHA256) || !preflightSource(in.Original.CurrentSource) || !preflightHash(in.Original.ContractSHA256) {
		a.Reasons = append(a.Reasons, "original_or_current_source_or_contract_unknown")
		return a
	}
	if in.Cause == "new_defect" || in.Cause == "source_change" || in.CurrentSource.Digest() != in.Original.CurrentSource.Digest() || in.CurrentContractSHA256 != in.Original.ContractSHA256 {
		a.Action = "new_review"
		a.Reasons = append(a.Reasons, "source_contract_change_or_new_defect_requires_new_judgment; T3 new review/recheck requires new delegate_task")
		return a
	}
	if in.Cause == "identity_recovery" {
		a.Reasons = append(a.Reasons, "recover_and_verify_actual_original_identity_before_classifying; no_inferred_independence")
		return a
	}
	p := PreflightReview(in.Original)
	a.Reasons = append(a.Reasons, p.Findings...)
	if in.OriginalErrors == nil {
		a.Reasons = append(a.Reasons, "original_errors_unknown_preserve_before_supplement")
	}
	if in.OriginalState != "completed" || strings.TrimSpace(in.OriginalNativeFinalRef) == "" || strings.TrimSpace(in.OriginalTerminalRef) == "" {
		a.Reasons = append(a.Reasons, "original_native_final_and_terminal_evidence_required; running_reviewer_is_not_final")
	}
	// Reuse the existing independent review validator; do not invent a missing
	// accepted judgment or replace a needs_changes result for transport purposes.
	if s := in.Original.Statement; s != nil {
		if _, err := ValidateReview(in.Original.Request.TaskID, in.Original.Request.Implementation, in.Original.OwnIdentity, in.Original.Policy, *s, in.Original.ReportSHA256, in.CurrentSource); err != nil {
			a.Reasons = append(a.Reasons, err.Error())
		}
	}
	if len(a.Reasons) == 0 {
		a.Action = "verify_original_then_supplement_transport"
		a.Reasons = append(a.Reasons, "unchanged_source_already_attested_result; no_new_Reviewer_judgment_requested; never_resume_terminal_Reviewer_as_new_review")
	}
	return a
}
