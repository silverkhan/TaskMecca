package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	if *dry {
		receipt, err := runtimeobs.ReadT3Execution(sel, time.Now())
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return writeT3Result(out, *asJSON, map[string]any{"verified": true, "written": false, "receipt": receipt})
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
