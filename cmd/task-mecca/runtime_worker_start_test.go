package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// File-selected CLI test: no main/server/service helpers are compiled.
func TestA72StartCLIGapsAreVisibleWithoutOperatingProject(t *testing.T) {
	p := t.TempDir()
	if err := os.MkdirAll(filepath.Join(p, "_task_mecca"), 0700); err != nil {
		t.Fatal(err)
	}
	base := []string{"--project", p, "--thread-id", "thread", "--session-id", "native", "--agent-id", "native", "--assignment", "missing", "--task-id", "A-72", "--worker-path", "/root/controller/isanghaessi", "--source-worktree", t.TempDir(), "--phase", "implementation_started", "--json"}
	var out, stderr bytes.Buffer
	code := runRuntimeObserveT3CLI(base, &out, &stderr)
	var result struct {
		CollectionGap   string                         `json:"collection_gap"`
		ActivityWritten bool                           `json:"activity_written"`
		WorkerStart     struct{ Status, Stage string } `json:"worker_start"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("missing JSON failure %s %s", out.String(), stderr.String())
	}
	if code != 1 || result.CollectionGap == "" || result.ActivityWritten || result.WorkerStart.Status != "collection_gap" || result.WorkerStart.Stage != "assignment" {
		t.Fatalf("hidden gap code=%d %+v", code, result)
	}
	if _, err := os.Stat(filepath.Join(p, "_task_mecca", "data")); !os.IsNotExist(err) {
		t.Fatal("failed START wrote operating data")
	}
	bad := append([]string{}, base...)
	bad = append(bad, "--phase", "tests_started")
	out.Reset()
	stderr.Reset()
	if code = runRuntimeObserveT3CLI(bad, &out, &stderr); code != 2 {
		t.Fatalf("invented phase accepted: %d %s", code, out.String())
	}
	partial := []string{"--thread-id", "thread", "--session-id", "native", "--agent-id", "native", "--assignment", "missing", "--json"}
	if code = runRuntimeObserveT3CLI(partial, &out, &stderr); code != 2 {
		t.Fatalf("implicit project accepted %d", code)
	}
}
