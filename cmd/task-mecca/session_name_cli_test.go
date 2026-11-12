package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/orchestration"
)

func TestSessionNameCLIFakeProofAndForeignScopeRejected(t *testing.T) {
	root := t.TempDir()
	g, err := orchestration.Configure(root, "codex", "root-exact", orchestration.ModeA, orchestration.Capabilities{SpawnSubagent: "supported"}, false, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_THREAD_ID", "own-exact")
	t.Setenv("T3CODE_THREAD_ID", "t3-own")
	base := []string{"verify", "--project", root, "--group", g.GroupID, "--role", "controller", "--provider", "codex", "--execution", "existingSession", "--session-id", "foreign", "--json"}
	for _, extra := range [][]string{nil, {"--receipt", `{"verified":true,"applied":true}`}, {"--capabilities", `{"naming":"supported"}`}, {"--owned", "true"}} {
		var out, stderr bytes.Buffer
		rc := runSessionNameCLI(append(append([]string{}, base...), extra...), &out, &stderr)
		if rc == 0 || strings.Contains(out.String(), `"verified":true`) {
			t.Fatalf("fake proof/foreign scope passed: %d %s", rc, out.String())
		}
	}
	// Metadata generation alone remains compatible, with an explicit pending gate.
	var out, stderr bytes.Buffer
	rc := runOrchestrationCLI([]string{"name", "--project", root, "--group", g.GroupID, "--role", "worker", "--worker", "pikachyu", "--generation", "1", "--json"}, &out, &stderr)
	var v map[string]any
	_ = json.Unmarshal(out.Bytes(), &v)
	if rc != 0 || v["display_name"] != g.DisplayPrefix+"Worker_pikachyu_G1" || v["naming_verified"] != false {
		t.Fatalf("%d %s %s", rc, out.String(), stderr.String())
	}
}
