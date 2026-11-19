package main

import (
	"bytes"
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeBoundaryCLIFailsClosedForForeignNativeAndUnmeasuredList(t *testing.T) {
	p := t.TempDir()
	var out, errOut bytes.Buffer
	code := runRuntimeBoundaryCLI([]string{"record", "--project", p, "--session-id", "foreign", "--thread-id", "thread", "--role", "registrar"}, &out, &errOut)
	if code == 0 || !strings.Contains(errOut.String(), "own current") {
		t.Fatalf("%d %s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(p, "_task_mecca")); !os.IsNotExist(err) {
		t.Fatal("failed identity wrote storage")
	}
	out.Reset()
	errOut.Reset()
	code = runRuntimeBoundaryCLI([]string{"list", "--project", p, "--json"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), `"items":[]`) {
		t.Fatalf("%d %s %s", code, out.String(), errOut.String())
	}
	if _, err := os.Stat(filepath.Join(p, "_task_mecca")); !os.IsNotExist(err) {
		t.Fatal("read-only list wrote storage")
	}
}

func TestRegistrationBoundaryFixtureCurrentLifecycleAndTaskAssociations(t *testing.T) {
	p := t.TempDir()
	var diagnostics bytes.Buffer
	c := &registrationBoundary{project: p, stderr: &diagnostics, base: runtimeobs.OperationBoundary{RequestID: "stable-request", OperationID: "stable-invocation", Provider: "codex", NativeID: "fixture-native", AttemptID: "fixture-attempt", Role: "registrar", Source: "isolated_fixture", Evidence: "fixture; not actual maintenance"}}
	c.write("start", "start", "", nil, "")
	c.phase("registration_apply")
	c.link([]string{"A-1", "A-2"})
	c.finish(0, "")
	rows, err := runtimeobs.ReadOperationBoundaries(p)
	if err != nil || len(rows) != 4 {
		t.Fatalf("%d %v %s", len(rows), err, diagnostics.String())
	}
	for _, b := range rows {
		if b.RequestID != "stable-request" || b.ObservedAt.Before(time.Now().Add(-time.Minute)) {
			t.Fatal("lost request/current observation")
		}
	}
	if rows[2].Kind != "task_link" || len(rows[2].TaskIDs) != 2 {
		t.Fatal("batch association")
	}
}

func TestBoundaryNativeRoleMetadataMatching(t *testing.T) {
	for role, name := range map[string]string{"registrar": "D1112_01_Registrar", "controller": "D1112_01_Controller", "worker": "D1112_01_Worker_naong_G3", "reviewer": "D1112_01_Reviewer_raichyu_G6"} {
		if !boundaryRoleMatches(role, name) || boundaryRoleMatches("registrar", name) && role != "registrar" {
			t.Fatalf("%s %s", role, name)
		}
	}
	if boundaryRoleMatches("reviewer", "D1112_01_Worker_raichyu_G6") {
		t.Fatal("worker cannot claim reviewer")
	}
}

func TestRegistrationBoundaryUnsupportedStatusSurvivesAgentJSON(t *testing.T) {
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("T3CODE_THREAD_ID", "")
	var out, stderr bytes.Buffer
	code := runAgentJSON([]string{"--json"}, func(w, e io.Writer) int {
		c := beginRegistrationBoundary(t.TempDir(), "request", e)
		c.finish(0, "")
		if err := json.NewEncoder(w).Encode(map[string]any{"status": "success", "execution_collection": c.status()}); err != nil {
			return 1
		}
		return 0
	}, &out, &stderr)
	if code != 0 || !strings.Contains(out.String(), `"status":"unmeasured"`) || !strings.Contains(out.String(), "original_error") {
		t.Fatalf("collection error hidden: %d %s %s", code, out.String(), stderr.String())
	}
}
