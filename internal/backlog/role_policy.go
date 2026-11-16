package backlog

import (
	"encoding/json"
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
	p, approved, err := rolepolicy.Load(project, task, raw)
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
		if matches, _ := contractscope.MatchesFingerprint(raw, h.ContractSHA256); !matches {
			continue
		}
		step := h.Steps["acceptance"]
		if step.Result != "ok" {
			continue
		}
		var envelope struct {
			Receipt struct {
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
		implementation, reviewer := identities[h.SourceAttemptID], identities[p.Review.AttemptID]
		if r.PolicySHA256 != rolepolicy.PolicyDigest(p) || r.ImplementationIdentity != implementation || r.ReviewIdentity != reviewer {
			continue
		}
		source, e := rolepolicy.Snapshot(p.Review.SourceWorkspace, p.Scope)
		if e != nil {
			v["review_reason"] = e.Error()
			continue
		}
		report := p.Review.ReportFile
		if report != "" && report[0] != '/' {
			report = project + "/" + report
		}
		hash, e := rolepolicy.HashFile(report)
		if e != nil {
			v["review_reason"] = e.Error()
			continue
		}
		classification, e := rolepolicy.ValidateReview(task, implementation, reviewer, p, r.Statement, hash, source)
		if e != nil {
			v["review_reason"] = e.Error()
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
