package handoff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type PolicyAssessment struct {
	InputRevision string `json:"input_revision"`
	ReportSHA256  string `json:"report_sha256"`
	rolepolicy.View
	ImplementationIdentity rolepolicy.Identity         `json:"implementation_identity"`
	ReviewIdentity         rolepolicy.Identity         `json:"review_identity"`
	Source                 *rolepolicy.Source          `json:"source,omitempty"`
	Statement              *rolepolicy.ReviewStatement `json:"statement,omitempty"`
	PolicySHA256           string                      `json:"policy_sha256"`
	CheckedAt              string                      `json:"checked_at"`
}

func exactPolicyAttempt(project, task, id string, now time.Time) (runtimeobs.Attempt, error) {
	l, e := runtimeobs.BuildLedger(project, 0, now)
	if e != nil {
		return runtimeobs.Attempt{}, e
	}
	var found []runtimeobs.Attempt
	for _, a := range l.Attempts {
		if a.AttemptID == id && a.TaskID == task {
			found = append(found, a)
		}
	}
	if len(found) != 1 {
		return runtimeobs.Attempt{}, errors.New("exact_task_attempt_required")
	}
	return found[0], nil
}

// AssessPolicy is shared by the CLI decision, handoff and acceptance gates.
// It only reads source/runtime/provider evidence; it never records an approval.
func AssessPolicy(project, task, attempt, workspace string, review bool, now time.Time) (PolicyAssessment, error) {
	return assessPolicy(project, task, attempt, workspace, review, now, runtimeobs.ReadNativeReview)
}

// AssessReviewerPolicy checks a read-only recipient, never implementation or
// accepted outcome. The underlying task execution consent remains unchanged.
func AssessReviewerPolicy(project, task, attempt, workspace string, now time.Time) (PolicyAssessment, error) {
	out := PolicyAssessment{CheckedAt: now.UTC().Format(time.RFC3339Nano)}
	p, _, revision, err := currentPolicyInputs(project, task)
	if err != nil {
		return out, err
	}
	r, err := exactPolicyAttempt(project, task, attempt, now)
	if err != nil {
		return out, err
	}
	out.InputRevision, out.PolicySHA256 = revision, rolepolicy.PolicyDigest(p)
	out.ReviewIdentity = runtimeobs.PolicyIdentity(r)
	if p == nil {
		return out, errors.New("structured_reviewer_policy_required")
	}
	source, err := rolepolicy.Snapshot(workspace, p.Scope)
	if err != nil {
		return out, err
	}
	out.Source = &source
	history, err := runtimeobs.ProductImplementationHistory(project, task, p, now)
	if err != nil {
		return out, err
	}
	out.View = rolepolicy.EvaluateReviewer(p, out.ReviewIdentity, workspace, source, history)
	if !out.Allowed {
		return out, errors.New(out.Reason)
	}
	if err = validateProductHistory(project, task, p, out.ReviewIdentity, now); err != nil {
		out.Allowed = false
		out.Reason = err.Error()
		return out, err
	}
	_, _, after, err := currentPolicyInputs(project, task)
	if err != nil || after != revision {
		out.Allowed = false
		return out, errors.New("review_inputs_changed_during_verification")
	}
	return out, nil
}

func assessPolicy(project, task, attempt, workspace string, review bool, now time.Time, readNative func(context.Context, runtimeobs.Attempt, string, string, time.Time) (rolepolicy.ReviewStatement, error)) (PolicyAssessment, error) {
	out := PolicyAssessment{CheckedAt: now.UTC().Format(time.RFC3339Nano)}
	p, approved, revision, e := currentPolicyInputs(project, task)
	if e != nil {
		return out, e
	}
	out.InputRevision = revision
	a, e := exactPolicyAttempt(project, task, attempt, now)
	if e != nil {
		return out, e
	}
	out.ImplementationIdentity = runtimeobs.PolicyIdentity(a)
	out.View = rolepolicy.Evaluate(p, out.ImplementationIdentity, workspace, approved)
	out.PolicySHA256 = rolepolicy.PolicyDigest(p)
	if out.Allowed && p != nil && p.Method == "controller_exception" {
		ledger, err := runtimeobs.BuildLedger(project, 0, now)
		if err != nil {
			return out, err
		}
		verified := false
		for _, candidate := range ledger.Attempts {
			i := runtimeobs.PolicyIdentity(candidate)
			if i.Bound && i.Observed && i.TaskID == task && i.NativeID == p.Reviewer.NativeID && i.AgentPath == p.Reviewer.AgentPath && i.NativeID != out.ImplementationIdentity.NativeID && i.RuntimeAgentID != out.ImplementationIdentity.RuntimeAgentID {
				verified = true
			}
		}
		if !verified {
			out.Allowed = false
			out.Reason = "exception_actual_separate_reviewer_unverified"
			out.ResumeWhen = "Bind the designated separate actual reviewer to this task before Controller product implementation."
		}
	}
	if !out.Allowed {
		return out, fmt.Errorf("role_policy_hold: %s; resume: %s", out.Reason, out.ResumeWhen)
	}
	if !review {
		return out, nil
	}
	if p == nil {
		out.Review = "unknown"
		return out, errors.New("legacy_review_unverified: supply a structured role policy and actual reviewer/source evidence")
	}
	if p.ReviewRequirement != nil {
		d, err := p.ReviewRequirement.Decide()
		if err != nil {
			return out, err
		}
		if d.Status == "omitted" {
			if p.Method == "controller_exception" {
				return out, errors.New("controller_exception_requires_independent_review")
			}
			source, err := rolepolicy.Snapshot(workspace, p.Scope)
			if err != nil {
				return out, err
			}
			out.Source, out.Review = &source, "omitted"
			return out, nil
		}
	}
	if p.Review == nil {
		return out, errors.New("independent_review_pending: record exact actual reviewer attempt/report/source workspace in a contract supplement")
	}
	r, e := exactPolicyAttempt(project, task, p.Review.AttemptID, now)
	if e != nil {
		return out, e
	}
	out.ReviewIdentity = runtimeobs.PolicyIdentity(r)
	if (out.ImplementationIdentity.NativeID != "" && out.ImplementationIdentity.NativeID == out.ReviewIdentity.NativeID) || (out.ImplementationIdentity.RuntimeAgentID != "" && out.ImplementationIdentity.RuntimeAgentID == out.ReviewIdentity.RuntimeAgentID) {
		out.Review = "self"
		return out, errors.New("same_native_self_review")
	}
	report := p.Review.ReportFile
	if !filepath.IsAbs(report) {
		report = filepath.Join(project, report)
	}
	h, e := rolepolicy.HashFile(report)
	if e != nil {
		return out, e
	}
	out.ReportSHA256 = h
	source, e := rolepolicy.Snapshot(p.Review.SourceWorkspace, p.Scope)
	if e != nil {
		return out, e
	}
	out.Source = &source
	statement, e := readNative(context.Background(), r, task, h, now)
	if e != nil {
		return out, e
	}
	out.Statement = &statement
	if e = revalidatePolicyAssessment(project, task, workspace, &out, now); e != nil {
		out.Allowed = false
		out.Reason = e.Error()
		out.ResumeWhen = "Re-run verification against the current contract, consent history, policy, reviewer, report and source."
		return out, e
	}
	return out, nil
}

func currentPolicyInputs(project, task string) (*rolepolicy.Policy, bool, string, error) {
	i, e := backlog.Inspect(project, "", task)
	if e != nil {
		return nil, false, "", e
	}
	if i["exists"] != true || i["duplicate_count"] != 1 {
		return nil, false, "", errors.New("unique_current_contract_required")
	}
	raw, _ := i["raw_markdown"].(string)
	before, e := rolepolicy.ReviewRevision(project, task, raw)
	if e != nil {
		return nil, false, "", e
	}
	p, approved, e := rolepolicy.Load(project, task, raw)
	if e != nil {
		return nil, false, "", e
	}
	after, e := rolepolicy.ReviewRevision(project, task, raw)
	if e != nil {
		return nil, false, "", e
	}
	if before != after {
		return nil, false, "", errors.New("review_inputs_changed_during_policy_read")
	}
	return p, approved, after, nil
}

// Reused after the potentially slow native read and immediately before the
// supported journal append. A revision pins the existing consent/contract
// history without inventing another approval manager or trusting caller flags.
func revalidatePolicyAssessment(project, task, workspace string, a *PolicyAssessment, now time.Time) error {
	previousReview := a.Review
	a.Review = "unknown"
	p, approved, revision, e := currentPolicyInputs(project, task)
	if e != nil {
		return e
	}
	if a.InputRevision == "" || revision != a.InputRevision {
		return errors.New("review_inputs_changed_during_verification")
	}
	if previousReview == "omitted" {
		if p == nil || p.ReviewRequirement == nil || p.Method != "worker" || a.Source == nil {
			return errors.New("omission_evidence_required")
		}
		d, err := p.ReviewRequirement.Decide()
		if err != nil {
			return err
		}
		if d.Status != "omitted" {
			return errors.New("review_omission_changed")
		}
		actual, err := exactPolicyAttempt(project, task, a.ImplementationIdentity.AttemptID, now)
		if err != nil {
			return err
		}
		if runtimeobs.PolicyIdentity(actual) != a.ImplementationIdentity {
			return errors.New("implementation_identity_changed")
		}
		v := rolepolicy.Evaluate(p, a.ImplementationIdentity, workspace, approved)
		if !v.Allowed {
			return errors.New(v.Reason)
		}
		source, err := rolepolicy.Snapshot(workspace, p.Scope)
		if err != nil {
			return err
		}
		if source.Digest() != a.Source.Digest() {
			return errors.New("source_changed_during_omission_check")
		}
		_, _, finalRevision, err := currentPolicyInputs(project, task)
		if err != nil {
			return err
		}
		if finalRevision != a.InputRevision {
			return errors.New("review_inputs_changed_during_verification")
		}
		a.Review = "omitted"
		return nil
	}
	if p == nil || p.Review == nil || a.Statement == nil || a.Source == nil {
		return errors.New("current_review_evidence_required")
	}
	implementation, e := exactPolicyAttempt(project, task, a.ImplementationIdentity.AttemptID, now)
	if e != nil {
		return e
	}
	reviewer, e := exactPolicyAttempt(project, task, p.Review.AttemptID, now)
	if e != nil {
		return e
	}
	if runtimeobs.PolicyIdentity(implementation) != a.ImplementationIdentity || runtimeobs.PolicyIdentity(reviewer) != a.ReviewIdentity {
		return errors.New("review_identity_changed_during_check")
	}
	if v := rolepolicy.Evaluate(p, a.ImplementationIdentity, workspace, approved); !v.Allowed {
		return fmt.Errorf("current_role_policy_hold: %s", v.Reason)
	}
	source, e := rolepolicy.Snapshot(p.Review.SourceWorkspace, p.Scope)
	if e != nil {
		return e
	}
	report := p.Review.ReportFile
	if !filepath.IsAbs(report) {
		report = filepath.Join(project, report)
	}
	h, e := rolepolicy.HashFile(report)
	if e != nil {
		return e
	}
	if source.Digest() != a.Source.Digest() || h != a.ReportSHA256 {
		return errors.New("source_or_report_changed_during_review_check")
	}
	if e := validateProductHistory(project, task, p, a.ReviewIdentity, now); e != nil {
		return e
	}
	classification, e := rolepolicy.ValidateReview(task, a.ImplementationIdentity, a.ReviewIdentity, p, *a.Statement, h, source)
	if e != nil {
		return e
	}
	_, _, finalRevision, e := currentPolicyInputs(project, task)
	if e != nil {
		return e
	}
	if finalRevision != a.InputRevision {
		return errors.New("review_inputs_changed_during_verification")
	}
	a.View.Policy = p
	a.PolicySHA256 = rolepolicy.PolicyDigest(p)
	a.Review = classification
	return nil
}

func validateAcceptanceEvidenceCurrent(project string, row Handoff, evidence string, now time.Time) error {
	p, _, _, e := currentPolicyInputs(project, row.TaskID)
	if e != nil {
		return e
	}
	if p == nil || !completionEvent(row.EventType) {
		return nil
	}
	if p.Operational.NativeID != "" && p.Operational.NativeID != row.Target.SessionID {
		return errors.New("operational_owner_native_mismatch")
	}
	var envelope struct {
		Assessment *PolicyAssessment `json:"role_policy_review"`
	}
	if e = json.Unmarshal([]byte(evidence), &envelope); e != nil || envelope.Assessment == nil {
		return errors.New("current_supported_acceptance_assessment_required")
	}
	return revalidatePolicyAssessment(project, row.TaskID, row.SourceWorkspace, envelope.Assessment, now)
}

func acceptancePolicyEvidence(project string, row Handoff, step, evidence string, now time.Time) (string, error) {
	i, e := backlog.Inspect(project, "", row.TaskID)
	if e != nil {
		return "", e
	}
	raw, _ := i["raw_markdown"].(string)
	p, _, e := rolepolicy.Load(project, row.TaskID, raw)
	if e != nil {
		return "", e
	}
	// Existing records remain usable, but carry no independent-review claim.
	if p == nil {
		return evidence, nil
	}
	if row.EventType != EventWorkerDone && row.EventType != EventCompletionReport {
		return evidence, nil
	}
	if p.Operational.NativeID != "" && p.Operational.NativeID != row.Target.SessionID {
		return "", errors.New("operational_owner_native_mismatch")
	}
	if step == "applied" && row.Steps["acceptance"].Result != "ok" {
		return "", errors.New("independent_review_acceptance_required_before_applied")
	}
	a, e := AssessPolicy(project, row.TaskID, row.SourceAttemptID, row.SourceWorkspace, true, now)
	if e != nil {
		return "", e
	}
	// The journal record supplies the observation time. Keep receipt content
	// stable across retries while rechecking native and source evidence above.
	a.CheckedAt = ""
	b, e := json.Marshal(map[string]any{"detail": evidence, "role_policy_review": a})
	// The outer JSONL journal escapes this JSON string again and has a 2 MiB
	// reader bound. Reject an oversized receipt rather than corrupting history.
	if e == nil && len(b) > 512<<10 {
		return "", errors.New("review_receipt_size_limit: use narrower explicit source scope")
	}
	return string(b), e
}

// History is read from bound native attempts and effective canonical Activity,
// never from a role rename or a reviewer-supplied independence flag.
func validateProductHistory(project, task string, p *rolepolicy.Policy, reviewer rolepolicy.Identity, now time.Time) error {
	historyPolicy := *p
	historyPolicy.Reviewer.NativeID = reviewer.NativeID
	history, err := runtimeobs.ProductImplementationHistory(project, task, &historyPolicy, now)
	if err != nil {
		return err
	}
	return rolepolicy.ValidateImplementationHistory(task, reviewer, history)
}
