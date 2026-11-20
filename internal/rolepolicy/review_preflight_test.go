package rolepolicy

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func b2Preflight() ReviewPreflightInput {
	i := Identity{TaskID: "B-2", AttemptID: "implementation-attempt", NativeID: "native-author", RuntimeAgentID: "author", Provider: "codex", AgentPath: "/root/controller/jirachi", Bound: true, Observed: true}
	r := Identity{TaskID: "B-2", AttemptID: "review-attempt", NativeID: "native-reviewer", RuntimeAgentID: "reviewer", Provider: "codex", AgentPath: "/root/controller/reviewer/pikachyu", Bound: true, Observed: true}
	s := Source{Files: map[string]string{"product.go": strings.Repeat("a", 64), "removed.go": "absent"}}
	q := ReviewRequirement{Scope: []string{"product.go", "removed.go"}, Rationale: "behavior changes with established selected verification", Risk: ReviewRisk{Impact: "moderate", Complexity: "complex", Reversibility: "easy", Verifiability: "established", BehaviorChanged: true}}
	p := &Policy{SchemaVersion: 1, Method: "worker", Workspace: "/candidate", Scope: q.Scope, Implementation: Owner{Role: "worker", NativeID: i.NativeID, AgentPath: i.AgentPath}, Operational: Owner{Role: "controller", AgentPath: "/root/controller"}, Reviewer: Owner{Role: "reviewer", NativeID: r.NativeID, AgentPath: r.AgentPath}, ReviewRequirement: &q}
	h := strings.Repeat("b", 64)
	return ReviewPreflightInput{Request: ReviewRequest{TaskID: i.TaskID, AssignmentID: "assignment", ContractSHA256: h, Requirement: q, Source: s, Implementation: i, Reviewer: r}, Policy: p, OwnIdentity: r, Workspace: p.Workspace, ContractSHA256: h, CurrentSource: s, ImplementationHistory: []Identity{i}, Statement: &ReviewStatement{TaskID: i.TaskID, ReviewerAttemptID: r.AttemptID, ImplementationNativeID: i.NativeID, Source: s, ReportSHA256: strings.Repeat("c", 64), Result: "accepted"}, ReportSHA256: strings.Repeat("c", 64)}
}

func b2Clone(in ReviewPreflightInput) ReviewPreflightInput {
	b, _ := json.Marshal(in)
	var out ReviewPreflightInput
	_ = json.Unmarshal(b, &out)
	return out
}

func TestB2StructuralPreflightNeverGrantsAcceptance(t *testing.T) {
	in := b2Preflight()
	for _, result := range []string{"accepted", "needs_changes"} {
		in.Statement.Result = result
		a := PreflightReview(in)
		if !a.Ready || !a.Advisory || a.ExecutionAuthorized || a.AcceptanceGranted || a.Requirement.Level != ReviewRecommended || a.Requirement.CreateReviewer || len(a.FreshChecks) == 0 {
			t.Fatal(a)
		}
	}
}

func TestB2PreflightCounterexamplesCatchErrorsBeforeTerminal(t *testing.T) {
	cases := map[string]func(*ReviewPreflightInput){
		"missing-statement": func(in *ReviewPreflightInput) { in.Statement = nil },
		"missing-judgment":  func(in *ReviewPreflightInput) { in.Statement.Result = "" },
		"wrong-hash":        func(in *ReviewPreflightInput) { in.ReportSHA256 = strings.Repeat("d", 64) },
		"invalid-hash":      func(in *ReviewPreflightInput) { in.ReportSHA256 = "unknown"; in.Statement.ReportSHA256 = "unknown" },
		"wrong-own-native":  func(in *ReviewPreflightInput) { in.OwnIdentity.NativeID = "another-native" },
		"wrong-attempt":     func(in *ReviewPreflightInput) { in.Statement.ReviewerAttemptID = "another-attempt" },
		"blank-attempt": func(in *ReviewPreflightInput) {
			in.OwnIdentity.AttemptID = ""
			in.Request.Reviewer.AttemptID = ""
			in.Statement.ReviewerAttemptID = ""
		},
		"unknown-native":     func(in *ReviewPreflightInput) { in.OwnIdentity.Observed = false; in.Request.Reviewer = in.OwnIdentity },
		"wrong-contract":     func(in *ReviewPreflightInput) { in.ContractSHA256 = strings.Repeat("d", 64) },
		"missing-assignment": func(in *ReviewPreflightInput) { in.Request.AssignmentID = "" },
		"changed-source":     func(in *ReviewPreflightInput) { in.CurrentSource.Files["product.go"] = strings.Repeat("d", 64) },
		"incomplete-scope":   func(in *ReviewPreflightInput) { delete(in.CurrentSource.Files, "removed.go") },
		"outside-scope":      func(in *ReviewPreflightInput) { in.CurrentSource.Files["other.go"] = strings.Repeat("d", 64) },
		"missing-history":    func(in *ReviewPreflightInput) { in.ImplementationHistory = nil },
		"historical-self-review": func(in *ReviewPreflightInput) {
			h := in.OwnIdentity
			h.AgentPath = "/root/controller/old-worker"
			in.ImplementationHistory = append(in.ImplementationHistory, h)
		},
		"requirement-mismatch": func(in *ReviewPreflightInput) { in.Request.Requirement.Risk.Impact = "low" },
		"missing-policy":       func(in *ReviewPreflightInput) { in.Policy = nil },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			in := b2Clone(b2Preflight())
			change(&in)
			a := PreflightReview(in)
			if a.Ready || len(a.Findings) == 0 || a.ExecutionAuthorized || a.AcceptanceGranted {
				t.Fatal(a)
			}
		})
	}
}

func TestB2RelevantAuthorMatrixPreservesNotRunAndSource(t *testing.T) {
	s := b2Preflight().CurrentSource
	m := CounterexampleMatrix{TaskID: "B-2", Source: s, Required: []string{"negative", "target-mismatch"}, Cases: []Counterexample{{ID: "negative", Question: "Can a negative request grant authority?", Input: "do not act", Expected: "deny", Actual: "deny", Result: "pass", Evidence: "selected negative case"}, {ID: "target-mismatch", Question: "Does the requested target match?", Input: "different target", Expected: "deny", Result: "not_run"}}}
	a := CheckCounterexamples(m, "B-2", s)
	if a.Ready || a.Cases[1].Result != "not_run" {
		t.Fatal(a)
	}
	m.Cases[1].Actual, m.Cases[1].Result, m.Cases[1].Evidence = "deny", "pass", "selected mismatch case"
	if !CheckCounterexamples(m, "B-2", s).Ready {
		t.Fatal(m)
	}
	m.Cases[1].Evidence = ""
	if CheckCounterexamples(m, "B-2", s).Ready {
		t.Fatal("invented pass without evidence")
	}
	m.Required = nil
	if CheckCounterexamples(m, "B-2", s).Ready {
		t.Fatal("undefined relevant matrix")
	}
}

func TestB2TransportSupplementRetainsOriginalAndDoesNotRequestNewJudgment(t *testing.T) {
	p := b2Preflight()
	in := ReviewFollowupInput{Cause: "report_transport", Original: p, CurrentSource: p.CurrentSource, CurrentContractSHA256: p.ContractSHA256, OriginalState: "completed", OriginalNativeFinalRef: "official original native final item", OriginalTerminalRef: "official completed run", OriginalErrors: []string{"original transport rejected missing envelope field"}}
	a := ClassifyReviewFollowup(in)
	if a.Action != "verify_original_then_supplement_transport" || a.ExecutionAuthorized || a.AcceptanceGranted || !reflect.DeepEqual(a.Original, in) || len(a.FreshChecks) == 0 {
		t.Fatal(a)
	}
	for name, mutate := range map[string]func(*ReviewFollowupInput){
		"running":                    func(x *ReviewFollowupInput) { x.OriginalState = "running" },
		"missing-original-statement": func(x *ReviewFollowupInput) { x.Original.Statement = nil },
		"missing-native-final":       func(x *ReviewFollowupInput) { x.OriginalNativeFinalRef = "" },
		"missing-terminal":           func(x *ReviewFollowupInput) { x.OriginalTerminalRef = "" },
		"unknown-original-errors":    func(x *ReviewFollowupInput) { x.OriginalErrors = nil },
		"unknown-original-source":    func(x *ReviewFollowupInput) { x.Original.CurrentSource = Source{} },
		"unknown-current-contract":   func(x *ReviewFollowupInput) { x.CurrentContractSHA256 = "unknown" },
		"needs-changes":              func(x *ReviewFollowupInput) { x.Original.Statement.Result = "needs_changes" },
		"identity-recovery":          func(x *ReviewFollowupInput) { x.Cause = "identity_recovery" },
		"unknown-cause":              func(x *ReviewFollowupInput) { x.Cause = "unknown" },
	} {
		t.Run(name, func(t *testing.T) {
			x := in
			x.Original = b2Clone(p)
			mutate(&x)
			a := ClassifyReviewFollowup(x)
			if a.Action != "blocked" || len(a.Reasons) == 0 || a.AcceptanceGranted {
				t.Fatal(a)
			}
		})
	}
	for _, cause := range []string{"new_defect", "source_change"} {
		x := in
		x.Cause = cause
		if a := ClassifyReviewFollowup(x); a.Action != "new_review" || !strings.Contains(a.Reasons[0], "new delegate_task") {
			t.Fatal(a)
		}
	}
	x := in
	x.CurrentSource = Source{Files: map[string]string{"product.go": strings.Repeat("e", 64)}}
	if a := ClassifyReviewFollowup(x); a.Action != "new_review" {
		t.Fatal(a)
	}
}
