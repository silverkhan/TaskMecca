package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/silverkhan/TaskMecca/internal/orchestration"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func runSessionNameCLI(args []string, out, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "orchestration session-name plan|apply|verify --project DIR --group DMMDD_NN --role ROLE --provider PROVIDER --execution newSession|existingSession [--session-id EXACT] --json")
		return 2
	}
	action := args[0]
	if action != "plan" && action != "apply" && action != "verify" {
		fmt.Fprintln(stderr, "unknown session-name action")
		return 2
	}
	fs := flag.NewFlagSet("session-name "+action, flag.ContinueOnError)
	fs.SetOutput(stderr)
	project := fs.String("project", ".", "original canonical project (read only)")
	group := fs.String("group", "", "existing orchestration group")
	role := fs.String("role", "", "root|registrar|controller|worker")
	worker := fs.String("worker", "", "existing Worker name")
	generation := fs.Int("generation", 1, "retained Worker generation, never increment on resume")
	provider := fs.String("provider", "", "actual selected provider")
	execution := fs.String("execution", "", "newSession|existingSession")
	id := fs.String("session-id", "", "exact native session ID, not display name")
	agent := fs.String("runtime-agent-id", "", "runtime identity (preserved, never inferred)")
	attempt := fs.String("attempt-id", "", "exact existing attempt")
	workerPath := fs.String("worker-path", "", "internal logical role path")
	t3 := fs.String("t3-thread-id", "", "exact actual T3 thread, separately tool verified")
	_ = fs.Bool("json", false, "structured output (always JSON)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		return 2
	}
	abs, err := filepath.Abs(*project)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	g, err := orchestration.FindGroupByID(abs, *group)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	target := orchestration.NativeNameTarget{GroupID: *group, Role: *role, Worker: *worker, Generation: *generation, Provider: *provider, SessionID: *id, RuntimeAgentID: *agent, AttemptID: *attempt, WorkerPath: *workerPath, T3ThreadID: *t3, Execution: *execution}
	directive, err := orchestration.SessionNamingDirective(g, target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if action == "plan" {
		support := runtimeobs.SessionNamingSupport(context.Background(), *provider, abs)
		var prior *runtimeobs.NameReceipt
		if support == "supported" && *execution == "existingSession" {
			var observed runtimeobs.NameReceipt
			if *provider == "codex" {
				observed = runtimeobs.CodexSessionName(context.Background(), *id, directive.DisplayName, false)
			} else {
				observed = runtimeobs.ClaudeSessionName(context.Background(), *id, directive.DisplayName, abs, false)
			}
			prior = &observed
		}
		if err := json.NewEncoder(out).Encode(map[string]any{"directive": directive, "naming_support": support, "prior_native_readback": prior, "dispatch_authorized": false, "verified": false}); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if prior != nil && prior.Status != "verified" && prior.Status != "mismatch" {
			return 1
		}
		if support != "supported" {
			return 1
		}
		return 0
	}
	// Mutation and current verification are deliberately scoped to this process's
	// own native session. No --owned=true / JSON receipt can bypass this check.
	own := os.Getenv("CODEX_THREAD_ID")
	if *provider == "claude" {
		own = os.Getenv("CLAUDE_SESSION_ID")
	}
	if strings.TrimSpace(*id) == "" || own == "" || *id != own {
		fmt.Fprintln(stderr, "exact current native session required; foreign/history targets and caller ownership assertions are rejected")
		return 2
	}
	if actual := os.Getenv("T3CODE_THREAD_ID"); *t3 != "" && actual != "" && *t3 != actual {
		fmt.Fprintln(stderr, "T3 thread differs from current managed thread")
		return 2
	}
	var receipt runtimeobs.NameReceipt
	if *provider == "codex" {
		receipt = runtimeobs.CodexSessionName(context.Background(), *id, directive.DisplayName, action == "apply")
	} else {
		receipt = runtimeobs.ClaudeSessionName(context.Background(), *id, directive.DisplayName, abs, action == "apply")
	}
	t3Status := "not_in_use"
	if *t3 != "" || os.Getenv("T3CODE_THREAD_ID") != "" {
		t3Status = "agent_tool_read_required"
	}
	if err := json.NewEncoder(out).Encode(map[string]any{"directive": directive, "native_receipt": receipt, "t3_status": t3Status, "naming_complete": receipt.Verified && t3Status == "not_in_use", "operational_written": false}); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !receipt.Verified {
		return 1
	}
	return 0
}
