package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/silverkhan/TaskMecca/internal/orchestration"
)

// Only process-owned metadata is observed here. Shell code cannot see the
// agent's MCP inventory. In particular no --verified or receipt JSON is read.
func runOrchestrationEnvironmentCLI(args []string, out, errOut io.Writer) int {
	f := flag.NewFlagSet("orchestration environment", flag.ContinueOnError)
	f.SetOutput(errOut)
	provider := f.String("provider", "", "current provider: codex|claude")
	root := f.String("root-session-id", "", "exact current Root native session")
	_ = f.Bool("json", false, "structured output")
	if err := f.Parse(args); err != nil {
		return 2
	}
	if f.NArg() != 0 || (*provider != "codex" && *provider != "claude") {
		fmt.Fprintln(errOut, "current --provider codex|claude required")
		return 2
	}
	own := os.Getenv("CODEX_THREAD_ID")
	if *provider == "claude" {
		own = os.Getenv("CLAUDE_SESSION_ID")
	}
	if own == "" || *root != own {
		fmt.Fprintln(errOut, "exact current native Root session required; foreign identities cannot be observed from this process")
		return 2
	}
	now := time.Now().UTC()
	host := "native"
	t3 := os.Getenv("T3CODE_THREAD_ID")
	if t3 != "" {
		host = "t3"
	}
	e := orchestration.Environment{Host: host, Provider: *provider, RootSessionID: own, T3ThreadID: t3, Source: "current_process_environment:" + runtime.GOOS, CheckedAt: now, TrackingVisibility: "unknown: independently read actual host/native tools; process identity alone does not verify T3 linkage", Tools: map[string]orchestration.ToolEvidence{}}
	e.IdentityLinkage = "unknown"
	for _, name := range []string{"delegate_task", "spawn_agent", "create_session", "send_message", "followup_task", "t3_thread_send", "t3_thread_read"} {
		e.Tools[name] = orchestration.ToolEvidence{Support: "unknown", Source: "CLI cannot observe host callable tool inventory", CheckedAt: now}
	}
	returnCode := 0
	if err := json.NewEncoder(out).Encode(map[string]any{"environment": e, "identity_linkage_verified": false, "execution_authorized": false, "operational_written": false, "next_action": "Host agent must independently observe exact Root/T3 linkage and current callable creation/communication tool schemas. Pass the environment profile to orchestration decide for diagnostic routing. Actual dispatch requires the host adapter boundary with fresh canonical Full Access, current tools and existing assignment gates; JSON assertions never verify support or creation."}); err != nil {
		fmt.Fprintln(errOut, err)
		returnCode = 1
	}
	return returnCode
}
