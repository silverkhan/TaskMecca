package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoff"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

func runRolePolicyCLI(args []string, out, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "review-decision" {
		f := flag.NewFlagSet("review-decision", flag.ContinueOnError)
		f.SetOutput(stderr)
		input := f.String("input", "", "typed ReviewRequirement JSON file")
		f.Bool("json", false, "JSON output")
		if f.Parse(args[1:]) != nil || f.NArg() != 0 || *input == "" {
			return 2
		}
		raw, err := os.Open(*input)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer raw.Close()
		d := json.NewDecoder(io.LimitReader(raw, 1<<20))
		d.DisallowUnknownFields()
		var requirement rolepolicy.ReviewRequirement
		if err = d.Decode(&requirement); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			fmt.Fprintln(stderr, "one typed requirement required")
			return 2
		}
		decision, err := requirement.Decide()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 3
		}
		json.NewEncoder(out).Encode(map[string]any{"review_requirement": decision, "dispatch_authorized": false, "external_tool_performed": false})
		return 0
	}
	if len(args) == 0 || args[0] == "help" {
		fmt.Fprintln(out, "contract role-policy check|snapshot --project CANONICAL --task ID --attempt EXACT --source-worktree ACTUAL [--review] --json")
		return 0
	}
	if args[0] != "check" && args[0] != "snapshot" {
		fmt.Fprintln(stderr, "role-policy action must be check or snapshot")
		return 2
	}
	f := flag.NewFlagSet("role-policy", flag.ContinueOnError)
	f.SetOutput(stderr)
	project := f.String("project", ".", "canonical project")
	task := f.String("task", "", "exact task")
	attempt := f.String("attempt", "", "exact implementation attempt")
	workspace := f.String("source-worktree", "", "actual implementation workspace")
	review := f.Bool("review", false, "verify actual native review and current source")
	recipientRole := f.String("recipient-role", "worker", "worker implementation or reviewer read-only recipient")
	f.Bool("json", false, "JSON output")
	if e := f.Parse(args[1:]); e != nil {
		return 2
	}
	if f.NArg() != 0 || *task == "" || *attempt == "" || *workspace == "" {
		fmt.Fprintln(stderr, "exact --task --attempt and --source-worktree required")
		return 2
	}
	decision, e := projectContractDecision(*project, *task)
	if e != nil {
		fmt.Fprintln(stderr, e)
		return 1
	}
	if *recipientRole != "worker" && *recipientRole != "reviewer" {
		fmt.Fprintln(stderr, "recipient role must be worker or reviewer")
		return 2
	}
	var assessment handoff.PolicyAssessment
	if *recipientRole == "reviewer" {
		if *review {
			fmt.Fprintln(stderr, "reviewer recipient check is not an accepted outcome check")
			return 2
		}
		assessment, e = handoff.AssessReviewerPolicy(*project, *task, *attempt, *workspace, time.Now())
	} else {
		assessment, e = handoff.AssessPolicy(*project, *task, *attempt, *workspace, *review, time.Now())
	}
	result := map[string]any{"execution_authorized": decision.ExecutionAuthorized, "role_policy": assessment, "ok": e == nil && decision.ExecutionAuthorized}
	if e != nil {
		result["error"] = e.Error()
	}
	if !decision.ExecutionAuthorized {
		result["authorization_error"] = "existing_execution_authorization_required"
	}
	if args[0] == "snapshot" && e == nil {
		if assessment.Policy == nil {
			result["error"] = "structured_policy_scope_required"
			e = fmt.Errorf("structured policy required")
		} else {
			source, err := rolepolicy.Snapshot(*workspace, assessment.Policy.Scope)
			if err != nil {
				result["error"] = err.Error()
				e = err
			} else {
				result["source"] = source
				result["source_sha256"] = source.Digest()
			}
		}
	}
	result["ok"] = e == nil && decision.ExecutionAuthorized
	if err := json.NewEncoder(out).Encode(result); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if e != nil || !decision.ExecutionAuthorized {
		return 3
	}
	return 0
}
