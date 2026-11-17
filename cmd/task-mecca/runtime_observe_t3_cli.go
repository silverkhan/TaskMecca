package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"io"
	"time"
)

func runRuntimeObserveT3CLI(args []string, out, stderr io.Writer) int {
	f := flag.NewFlagSet("runtime observe-t3", flag.ContinueOnError)
	f.SetOutput(stderr)
	project := f.String("project", ".", "canonical project (Controller sole writer)")
	thread := f.String("thread-id", "", "exact T3 thread ID")
	session := f.String("session-id", "", "exact native CODEX_THREAD_ID")
	agent := f.String("agent-id", "", "exact native runtime ID (Codex session ID)")
	run := f.String("run-id", "", "optional expected current T3 run; rejects a changed run")
	assignment := f.String("assignment", "", "exact canonical Worker assignment; Controller START integration")
	task := f.String("task-id", "", "exact task from START")
	worker := f.String("worker-path", "", "exact internal Worker path from START")
	role := f.String("role", "", "formal collection role: controller, registrar or worker; never implementation authorization")
	agentPath := f.String("agent-path", "", "exact assigned role path for formal collection")
	parent := f.String("parent-thread-id", "", "exact managed parent verified against the live host store")
	handoff := f.String("handoff", "", "exact already claimed Controller handoff, never a claim request")
	phase := f.String("phase", "requirements_started", "actual current Worker phase reminder only; never records Activity")
	source := f.String("source-worktree", "", "source cwd, never the canonical ledger destination")
	dry := f.Bool("dry-run", false, "verify actual source without writing any project ledger")
	asJSON := f.Bool("json", false, "JSON result")
	if err := f.Parse(args); err != nil {
		return 2
	}
	if f.NArg() != 0 || *thread == "" || *session == "" || *agent == "" {
		fmt.Fprintln(stderr, "--thread-id, --session-id and --agent-id required; state/receipt JSON is not accepted")
		return 2
	}
	sel := runtimeobs.T3Selector{ThreadID: *thread, SessionID: *session, RuntimeAgentID: *agent, RunID: *run}
	formalMode := *role != "" || *agentPath != "" || *parent != ""
	startMode := *assignment != "" || *task != "" || *worker != "" || *source != "" || formalMode
	if formalMode && (*worker != "" || (*role != "controller" && *role != "worker" && *role != "registrar") || *agentPath == "" || *parent == "") {
		fmt.Fprintln(stderr, "formal collection requires --role controller|registrar|worker --agent-path EXACT --parent-thread-id EXACT; do not mix --worker-path")
		return 2
	}
	if *handoff != "" && (!formalMode || *role != "controller") {
		fmt.Fprintln(stderr, "--handoff is only an existing formal Controller claim")
		return 2
	}
	if startMode && (*assignment == "" || *task == "" || (!formalMode && *worker == "") || *source == "" || *project == ".") {
		fmt.Fprintln(stderr, "START integration requires explicit --project --assignment --task-id --worker-path --source-worktree")
		return 2
	}
	if startMode && (!formalMode || *role == "worker") {
		switch *phase {
		case "requirements_started", "implementation_started", "targeted_test_started", "full_regression_started", "build_started", "worker_completed":
		default:
			fmt.Fprintln(stderr, "unsupported Worker phase; use actual supported activity report event")
			return 2
		}
	}
	if *dry {
		if formalMode {
			fmt.Fprintln(stderr, "formal role collection dry-run is unsupported; use read-only runtime query/diagnose and exact official host reads. No binding was attempted.")
			return 2
		}
		receipt, err := runtimeobs.ReadT3Execution(sel, time.Now())
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return writeT3Result(out, *asJSON, map[string]any{"verified": true, "written": false, "receipt": receipt})
	}
	if startMode {
		var result runtimeobs.WorkerStartResult
		var err error
		if formalMode {
			result, err = runtimeobs.ObserveT3RoleStart(runtimeobs.RoleStart{CanonicalProject: *project, SourceWorktree: *source, TaskID: *task, AssignmentID: *assignment, Role: *role, AgentPath: *agentPath, ParentThreadID: *parent, Selector: sel}, time.Now())
		} else {
			result, err = runtimeobs.ObserveT3WorkerStart(runtimeobs.WorkerStart{CanonicalProject: *project, SourceWorktree: *source, TaskID: *task, AssignmentID: *assignment, WorkerPath: *worker, Selector: sel}, time.Now())
		}
		response := map[string]any{"verified": err == nil, "worker_start": result, "binding_verified": err == nil, "activity_written": false}
		if formalMode {
			delete(response, "worker_start")
			response["role_start"] = result
			response["role"] = *role
			response["implementation_authorized"] = false
			response["handoff_claimed"] = false
		}
		if err != nil {
			response["collection_gap"] = err.Error()
			response["next_action"] = "Controller: diagnose exact identity/binding, retry current START; Worker: report current phase only after exact bound query. Partial observation may have persisted."
			if !formalMode && *worker == "/root/controller" {
				response["failure_kind"] = "worker_role_path_mismatch"
				response["next_action"] = "Controller is not a Worker child. Retain this original error; use formal --role controller --agent-path /root/controller --parent-thread-id EXACT_PARENT_T3 with the exact existing assignment and source cwd. Opaque T3 IDs are not required to be UUIDs. No binding, Hook delivery or claim is implied."
			}
			writeT3Result(out, *asJSON, response)
			return 1
		}
		if formalMode && *role == "controller" {
			response["next_action"] = "Operational Controller binding only. Query exact attempt, recover the SAME existing handoff, directly deliver, then claim using supported current context. Delivery/steered is not claim/applied; Worker implementation authorization remains separate."
			if *handoff == "" {
				response["activity_reporting"] = map[string]string{"status": "unverified", "diagnostic": "Exact current claimed handoff required for Controller Activity; binding alone is not claim or measured coordination."}
				response["collection_gap"] = "Controller phase reporting requires its exact current claimed handoff; no Activity was written"
			} else {
				reporting := activity.CheckReporting(result.Context.CanonicalProject, *task, activity.RoleController, result.Attempt.AttemptID, "", *handoff, *phase, time.Now())
				response["activity_reporting"] = reporting
				if reporting.Status == "unverified" || reporting.Status == "error" {
					response["collection_gap"] = reporting.Diagnostic
					writeT3Result(out, *asJSON, response)
					return 1
				}
			}
			return writeT3Result(out, *asJSON, response)
		}
		reportRole, reportAssignment := activity.RoleWorker, *assignment
		if formalMode && *role == "registrar" {
			reportRole, reportAssignment = activity.RoleRegistrar, ""
		}
		reporting := activity.CheckReporting(result.Context.CanonicalProject, *task, reportRole, result.Attempt.AttemptID, reportAssignment, "", *phase, time.Now())
		response["activity_reporting"] = reporting
		if reporting.Status == "unverified" || reporting.Status == "error" {
			response["collection_gap"] = reporting.Diagnostic
			writeT3Result(out, *asJSON, response)
			return 1
		}
		response["next_action"] = "Deliver exact attempt and reporting requirement to Worker; Worker reports its current actual phase, not a historical start. Report any transport/report failure as collection_gap."
		return writeT3Result(out, *asJSON, response)
	}
	resolved, err := runtimeobs.ResolveProject(*project)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	attempt, receipt, err := runtimeobs.ObserveT3Execution(resolved, sel, time.Now())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeT3Result(out, *asJSON, map[string]any{"verified": true, "written": true, "attempt": attempt, "receipt": receipt})
}
func writeT3Result(out io.Writer, asJSON bool, v map[string]any) int {
	if asJSON {
		_ = json.NewEncoder(out).Encode(v)
	} else {
		fmt.Fprintln(out, v)
	}
	return 0
}
