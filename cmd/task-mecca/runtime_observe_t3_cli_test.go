package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestObserveT3CLIRejectsUserStateAndNeverWritesOnFailedDryRun(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, args := range [][]string{{"--project", project, "--json"}, {"--project", project, "--thread-id", "t", "--session-id", "native", "--agent-id", "native", "--state", "running"}, {"--project", project, "--thread-id", "t", "--session-id", "native", "--agent-id", "native", "--dry-run", "--json"}} {
		var out, stderr bytes.Buffer
		if code := runRuntimeObserveT3CLI(args, &out, &stderr); code == 0 {
			t.Fatal("unverified CLI input succeeded", out.String())
		}
	}
	if _, err := os.Stat(filepath.Join(project, "_task_mecca")); !os.IsNotExist(err) {
		t.Fatal("failed/dry verification wrote project ledger")
	}
}
