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
	startMode := *assignment != "" || *task != "" || *worker != "" || *source != ""
	if startMode && (*assignment == "" || *task == "" || *worker == "" || *source == "" || *project == ".") {
		fmt.Fprintln(stderr, "START integration requires explicit --project --assignment --task-id --worker-path --source-worktree")
		return 2
	}
	if startMode {
		switch *phase {
		case "requirements_started", "implementation_started", "targeted_test_started", "full_regression_started", "build_started", "worker_completed":
		default:
			fmt.Fprintln(stderr, "unsupported Worker phase; use actual supported activity report event")
			return 2
		}
	}
	if *dry {
		receipt, err := runtimeobs.ReadT3Execution(sel, time.Now())
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return writeT3Result(out, *asJSON, map[string]any{"verified": true, "written": false, "receipt": receipt})
	}
	if startMode {
		result, err := runtimeobs.ObserveT3WorkerStart(runtimeobs.WorkerStart{CanonicalProject: *project, SourceWorktree: *source, TaskID: *task, AssignmentID: *assignment, WorkerPath: *worker, Selector: sel}, time.Now())
		response := map[string]any{"verified": err == nil, "worker_start": result, "binding_verified": err == nil, "activity_written": false}
		if err != nil {
			response["collection_gap"] = err.Error()
			response["next_action"] = "Controller: diagnose exact identity/binding, retry current START; Worker: report current phase only after exact bound query. Partial observation may have persisted."
			writeT3Result(out, *asJSON, response)
			return 1
		}
		reporting := activity.CheckReporting(result.Context.CanonicalProject, *task, activity.RoleWorker, result.Attempt.AttemptID, *assignment, "", *phase, time.Now())
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
