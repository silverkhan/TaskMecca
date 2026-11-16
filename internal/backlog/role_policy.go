package backlog

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"time"

	"github.com/silverkhan/TaskMecca/internal/contractscope"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// RolePolicyProjection never launches a provider or upgrades a caller's review
// assertion. Independent means a supported acceptance receipt still matches
// the bound identities, task, policy, report, and full current source scope.
func RolePolicyProjection(project, task, raw string) map[string]any {
	p, approved, revision, err := projectionPolicyInputs(project, task, raw)
	v := map[string]any{"policy": p, "implementation": "worker_default", "review": "unknown", "legacy": p == nil, "exception_approved": approved, "review_reason": "actual_reviewer_and_source_required"}
	if err != nil {
		v["error"] = err.Error()
		return v
	}
	if p == nil {
		return v
	}
	v["implementation"] = p.Method
	v["policy_sha256"] = rolepolicy.PolicyDigest(p)
	l, err := runtimeobs.BuildLedger(project, 0, time.Now())
	if err != nil {
		v["error"] = err.Error()
		return v
	}
	identities := map[string]rolepolicy.Identity{}
	for _, a := range l.Attempts {
		if a.TaskID == task {
			identities[a.AttemptID] = runtimeobs.PolicyIdentity(a)
		}
	}
	if p.Review != nil {
		r := identities[p.Review.AttemptID]
		for _, a := range identities {
			if a.Bound && a.Observed && r.Bound && r.Observed && a.AgentPath == p.Implementation.AgentPath && a.NativeID != "" && (a.NativeID == r.NativeID || (a.RuntimeAgentID != "" && a.RuntimeAgentID == r.RuntimeAgentID)) {
				v["review"] = "self"
				v["review_reason"] = "same_native_self_review"
				return v
			}
		}
	}
	journal, err := handoffjournal.BuildLedger(project, time.Now())
	if err != nil {
		v["error"] = err.Error()
		return v
	}
	for _, h := range journal.Handoffs {
		if h.TaskID != task || p.Review == nil {
			continue
		}
		if h.EventType != handoffjournal.EventWorkerDone && h.EventType != handoffjournal.EventCompletionReport {
			continue
		}
		if matches, _ := contractscope.MatchesFingerprint(raw, h.ContractSHA256); !matches {
			continue
		}
		step := h.Steps["acceptance"]
		if step.Result != "ok" {
			continue
		}
		var envelope struct {
			Receipt struct {
				InputRevision          string                     `json:"input_revision"`
				ReportSHA256           string                     `json:"report_sha256"`
				Source                 *rolepolicy.Source         `json:"source"`
				PolicySHA256           string                     `json:"policy_sha256"`
				ImplementationIdentity rolepolicy.Identity        `json:"implementation_identity"`
				ReviewIdentity         rolepolicy.Identity        `json:"review_identity"`
				Statement              rolepolicy.ReviewStatement `json:"statement"`
			} `json:"role_policy_review"`
		}
		if json.Unmarshal([]byte(step.Evidence), &envelope) != nil {
			continue
		}
		r := envelope.Receipt
		if r.InputRevision == "" {
			v["review_reason"] = "legacy_receipt_revision_unknown"
			continue
		}
		if r.InputRevision != revision {
			v["review_reason"] = "review_revision_changed"
			continue
		}
		if r.ReportSHA256 == "" || r.Source == nil {
			v["review_reason"] = "receipt_evidence_incomplete"
			continue
		}
		implementation, reviewer := identities[h.SourceAttemptID], identities[p.Review.AttemptID]
		if r.PolicySHA256 != rolepolicy.PolicyDigest(p) || r.ImplementationIdentity != implementation || r.ReviewIdentity != reviewer {
			continue
		}
		if h.SourceAgentPath != implementation.AgentPath {
			v["review_reason"] = "receipt_source_identity_mismatch"
			continue
		}
		if gate := rolepolicy.Evaluate(p, implementation, h.SourceWorkspace, approved); !gate.Allowed {
			v["review_reason"] = gate.Reason
			continue
		}
		if p.Operational.NativeID != "" && p.Operational.NativeID != h.Target.SessionID {
			v["review_reason"] = "operational_owner_native_mismatch"
			continue
		}
		source, e := rolepolicy.Snapshot(p.Review.SourceWorkspace, p.Scope)
		if e != nil {
			v["review_reason"] = e.Error()
			continue
		}
		report := p.Review.ReportFile
		if !filepath.IsAbs(report) {
			report = filepath.Join(project, report)
		}
		hash, e := rolepolicy.HashFile(report)
		if e != nil {
			v["review_reason"] = e.Error()
			continue
		}
		if hash != r.ReportSHA256 || source.Digest() != r.Source.Digest() {
			v["review_reason"] = "receipt_report_or_source_changed"
			continue
		}
		classification, e := rolepolicy.ValidateReview(task, implementation, reviewer, p, r.Statement, hash, source)
		if e != nil {
			v["review_reason"] = e.Error()
			continue
		}
		fresh, e := runtimeobs.BuildLedger(project, 0, time.Now())
		if e != nil {
			v["review_reason"] = e.Error()
			continue
		}
		freshIdentities := map[string]rolepolicy.Identity{}
		for _, a := range fresh.Attempts {
			if a.TaskID == task {
				freshIdentities[a.AttemptID] = runtimeobs.PolicyIdentity(a)
			}
		}
		if freshIdentities[h.SourceAttemptID] != implementation || freshIdentities[p.Review.AttemptID] != reviewer {
			v["review_reason"] = "review_identity_changed_during_projection"
			continue
		}
		// Rendering stays passive, but must not publish an old receipt as
		// current after a decision/policy/contract change during the reads.
		_, _, finalRevision, e := projectionPolicyInputs(project, task, raw)
		if e != nil {
			v["review_reason"] = e.Error()
			continue
		}
		if finalRevision != r.InputRevision {
			v["review_reason"] = "review_revision_changed"
			continue
		}
		v["review"] = classification
		v["review_reason"] = "supported_acceptance_receipt_current_source"
		v["review_checked_at"] = step.ObservedAt
		v["source"] = source
		break
	}
	return v
}

// Inspect/TaskDetail already supply a row, but a cached row is not authority
// for the current material contract. Read canonical text without recursing
// through Inspect and bracket Load with the same core revision as acceptance.
func projectionPolicyInputs(project, task, viewedRaw string) (*rolepolicy.Policy, bool, string, error) {
	raw, e := rolepolicy.TaskMarkdown(project, task)
	if e != nil {
		return nil, false, "", e
	}
	material := func(s string) string { return contractscope.NormalizeAcceptance(rolepolicy.DecisionScopeRaw(s)) }
	if material(raw) != material(viewedRaw) {
		return nil, false, "", errors.New("current_contract_view_changed")
	}
	before, e := rolepolicy.ReviewRevision(project, task, raw)
	if e != nil {
		return nil, false, "", e
	}
	p, approved, e := rolepolicy.Load(project, task, raw)
	if e != nil {
		return nil, false, "", e
	}
	current, e := rolepolicy.TaskMarkdown(project, task)
	if e != nil {
		return nil, false, "", e
	}
	after, e := rolepolicy.ReviewRevision(project, task, current)
	if e != nil {
		return nil, false, "", e
	}
	if before != after {
		return nil, false, "", errors.New("review_inputs_changed_during_projection")
	}
	return p, approved, after, nil
}
