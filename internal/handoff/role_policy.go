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
	rolepolicy.View
	ImplementationIdentity rolepolicy.Identity         `json:"implementation_identity"`
	ReviewIdentity         rolepolicy.Identity         `json:"review_identity"`
	Source                 *rolepolicy.Source          `json:"source,omitempty"`
	Statement              *rolepolicy.ReviewStatement `json:"statement,omitempty"`
	PolicySHA256           string                      `json:"policy_sha256"`
	CheckedAt              string                      `json:"checked_at"`
}

func exactPolicyAttempt(project, task, id string, now time.Time) (runtimeobs.Attempt, error) {
	l, e := runtimeobs.BuildLedger(project, 20, now)
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

func assessPolicy(project, task, attempt, workspace string, review bool, now time.Time, readNative func(context.Context, runtimeobs.Attempt, string, string, time.Time) (rolepolicy.ReviewStatement, error)) (PolicyAssessment, error) {
	out := PolicyAssessment{CheckedAt: now.UTC().Format(time.RFC3339Nano)}
	i, e := backlog.Inspect(project, "", task)
	if e != nil {
		return out, e
	}
	raw, _ := i["raw_markdown"].(string)
	if i["exists"] != true || i["duplicate_count"] != 1 {
		return out, errors.New("unique_current_contract_required")
	}
	p, approved, e := rolepolicy.Load(project, task, raw)
	if e != nil {
		return out, e
	}
	a, e := exactPolicyAttempt(project, task, attempt, now)
	if e != nil {
		return out, e
	}
	out.ImplementationIdentity = runtimeobs.PolicyIdentity(a)
	out.View = rolepolicy.Evaluate(p, out.ImplementationIdentity, workspace, approved)
	out.PolicySHA256 = rolepolicy.PolicyDigest(p)
	if out.Allowed && p != nil && p.Method == "controller_exception" {
		ledger, err := runtimeobs.BuildLedger(project, 20, now)
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
	source, e := rolepolicy.Snapshot(p.Review.SourceWorkspace, p.Scope)
	if e != nil {
		return out, e
	}
	out.Source = &source
	statement, e := readNative(context.Background(), r, task, h, now)
	if e != nil {
		return out, e
	}
	freshSource, e := rolepolicy.Snapshot(p.Review.SourceWorkspace, p.Scope)
	if e != nil {
		return out, e
	}
	freshReport, e := rolepolicy.HashFile(report)
	if e != nil {
		return out, e
	}
	if freshSource.Digest() != source.Digest() || freshReport != h {
		return out, errors.New("source_or_report_changed_during_review_check")
	}
	freshReviewer, e := exactPolicyAttempt(project, task, p.Review.AttemptID, now)
	if e != nil {
		return out, e
	}
	if runtimeobs.PolicyIdentity(freshReviewer) != out.ReviewIdentity {
		return out, errors.New("review_identity_changed_during_check")
	}
	out.Statement = &statement
	out.Review, e = rolepolicy.ValidateReview(task, out.ImplementationIdentity, out.ReviewIdentity, p, statement, h, source)
	if e != nil {
		return out, e
	}
	return out, nil
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
	return string(b), e
}
