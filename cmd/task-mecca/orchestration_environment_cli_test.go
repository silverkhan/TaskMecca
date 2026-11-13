package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoff"
	"github.com/silverkhan/TaskMecca/internal/orchestration"
)

func TestEnvironmentCLICollectsOnlyCurrentProcessMetadata(t *testing.T) {
	t.Setenv("CODEX_THREAD_ID", "native-current")
	t.Setenv("T3CODE_THREAD_ID", "t3-current")
	var out, errOut bytes.Buffer
	code := runOrchestrationCLI([]string{"environment", "--provider", "codex", "--root-session-id", "native-current", "--json"}, &out, &errOut)
	if code != 0 {
		t.Fatal(errOut.String())
	}
	var result struct {
		Environment orchestration.Environment
		Verified    bool `json:"identity_linkage_verified"`
		Authorized  bool `json:"execution_authorized"`
		Written     bool `json:"operational_written"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Environment.RootSessionID != "native-current" || result.Environment.T3ThreadID != "t3-current" || result.Environment.Host != "t3" || result.Verified || result.Authorized || result.Written {
		t.Fatalf("%s", out.String())
	}
	for _, tool := range result.Environment.Tools {
		if tool.Support != "unknown" {
			t.Fatal("shell invented tool support")
		}
	}
	out.Reset()
	if runOrchestrationCLI([]string{"environment", "--provider", "codex", "--root-session-id", "foreign"}, &out, &errOut) == 0 {
		t.Fatal("foreign Root accepted")
	}
}

func TestEnvironmentCLICallerJSONCannotAuthorizeAndSavedBPreserved(t *testing.T) {
	project := t.TempDir()
	now := time.Now().UTC()
	caps := orchestration.Capabilities{SpawnSubagent: "supported", MessageRoot: "supported"}
	for _, name := range []string{"can_spawn_subagent", "can_message_root"} {
		if _, err := handoff.RecordCapability(project, "codex", name, handoff.CapabilitySupported, "fixture:mock-current-support", now); err != nil {
			t.Fatal(err)
		}
	}
	g, err := orchestration.Configure(project, "codex", "root-exact", orchestration.ModeB, caps, false, "", now)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "profile.json")
	e := orchestration.Environment{Host: "t3", Provider: "codex", RootSessionID: "root-exact", T3ThreadID: "t3-exact", Source: "caller:asserted", CheckedAt: now, Tools: map[string]orchestration.ToolEvidence{"delegate_task": {Support: "supported", Worktree: "supported", Source: "caller:asserted", CheckedAt: now}}}
	raw, _ := json.Marshal(map[string]any{"environment": e, "verified": true, "created": true, "execution_authorized": true})
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"decide", "--project", project, "--provider", "codex", "--root-session-id", "root-exact", "--root-t3-thread-id", "t3-exact", "--event", "dispatch_requested", "--environment", path, "--separate-worktree", "--source-cwd", "/fixture/parent", "--base", strings.Repeat("a", 40), "--json"}
	var out, errOut bytes.Buffer
	if runOrchestrationCLI(args, &out, &errOut) != 0 {
		t.Fatal(errOut.String())
	}
	var d orchestration.Decision
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if !d.Allowed || d.Mode != orchestration.ModeB || d.Dispatch == nil || d.Dispatch.Tool != "delegate_task" || d.Dispatch.ExecutionAuthorized {
		t.Fatalf("%s", out.String())
	}
	if !strings.Contains(strings.Join(d.Dispatch.Instructions, " "), "CLI does not invoke MCP") {
		t.Fatal("host boundary omitted")
	}
	out.Reset()
	if runOrchestrationCLI(append(args, "--mode", "A"), &out, &errOut) == 0 {
		t.Fatal("silently changed saved B")
	}
	after, exists, err := orchestration.FindGroup(project, "codex", "root-exact")
	if err != nil || !exists || after.GroupID != g.GroupID || after.Mode != g.Mode || len(after.ModeHistory) != 0 {
		t.Fatalf("saved selection changed: %+v %v", after, err)
	}
}
