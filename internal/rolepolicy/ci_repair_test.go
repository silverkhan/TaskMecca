package rolepolicy

import (
	"strings"
	"testing"
)

func TestHistoricalWorkerReceiptKeepsStoredRoleAndFormalBoundary(t *testing.T) {
	implementation := workerIdentity()
	reviewer := Identity{AttemptID: "historical-review", NativeID: "historical-native", RuntimeAgentID: "historical-native", Provider: "codex", AgentPath: "/root/controller/reviewer", TaskID: "A-78", Bound: true, Observed: true}
	source := Source{Files: map[string]string{"product.go": strings.Repeat("a", 64)}}
	policy := workerPolicy("/isolated/source")
	policy.Implementation.NativeID = implementation.NativeID
	policy.Reviewer = Owner{Role: "worker", AgentPath: reviewer.AgentPath, NativeID: reviewer.NativeID}
	policy.Review = &Review{AttemptID: reviewer.AttemptID, ReportFile: "review.md", SourceWorkspace: policy.Workspace}
	statement := ReviewStatement{TaskID: reviewer.TaskID, ReviewerAttemptID: reviewer.AttemptID, ImplementationNativeID: implementation.NativeID, Source: source, ReportSHA256: "report", Result: "accepted"}
	if got, err := ValidateReview("A-78", implementation, reviewer, policy, statement, "report", source); err != nil || got != "independent" {
		t.Fatal(got, err)
	}
	if policy.Reviewer.Role != "worker" || Role(reviewer.AgentPath) != "unknown" || WorkerPath(reviewer.AgentPath) || ReviewerPath(reviewer.AgentPath) {
		t.Fatal("historical evidence changed current role classification")
	}
	if got := EvaluateReviewer(policy, reviewer, policy.Workspace, source, []Identity{implementation}); got.Allowed {
		t.Fatal("historical Worker admitted as fresh formal Reviewer", got)
	}
	for _, name := range []string{"missing-review", "different-attempt", "formal-role-old-path", "new-requirement", "other-path", "same-native", "unobserved", "unbound", "task", "designated-native", "report", "source"} {
		t.Run(name, func(t *testing.T) {
			p := *policy
			r := reviewer
			s := statement
			actual := source
			hash := "report"
			switch name {
			case "missing-review":
				p.Review = nil
			case "different-attempt":
				review := *p.Review
				review.AttemptID = "other"
				p.Review = &review
			case "formal-role-old-path":
				p.Reviewer.Role = "reviewer"
			case "new-requirement":
				p.ReviewRequirement = &ReviewRequirement{}
			case "other-path":
				r.AgentPath = "/root/controller/reviewer/other"
			case "same-native":
				r.NativeID = implementation.NativeID
			case "unobserved":
				r.Observed = false
			case "unbound":
				r.Bound = false
			case "task":
				r.TaskID = "A-79"
			case "designated-native":
				p.Reviewer.NativeID = "another-native"
			case "report":
				hash = "changed"
			case "source":
				actual = Source{Files: map[string]string{"product.go": strings.Repeat("b", 64)}}
			}
			if got, err := ValidateReview("A-78", implementation, r, &p, s, hash, actual); err == nil || got == "independent" {
				t.Fatal("invalid historical receipt accepted", got, err)
			}
		})
	}
	formal := reviewer
	formal.AgentPath = "/root/controller/reviewer/raichyu"
	p := *policy
	p.Reviewer = Owner{Role: "reviewer", AgentPath: formal.AgentPath, NativeID: formal.NativeID}
	if got, err := ValidateReview("A-78", implementation, formal, &p, statement, "report", source); err != nil || got != "independent" {
		t.Fatal("formal Reviewer receipt rejected", got, err)
	}
	if got := EvaluateReviewer(&p, formal, p.Workspace, source, []Identity{implementation}); !got.Allowed {
		t.Fatal("fresh formal Reviewer rejected", got)
	}
}
