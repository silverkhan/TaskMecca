package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/silverkhan/TaskMecca/internal/maintenance"
)

func TestProjectCLISeparateConfirmationAndHistoryBinding(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", filepath.Join(dir, "management"))
	project := filepath.Join(dir, "project")
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := maintenance.RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	var out, errors bytes.Buffer
	if code := runProjectCLI([]string{"archive", "--path", project}, &out, &errors); code != 2 {
		t.Fatal("missing confirmation accepted", code)
	}
	out.Reset()
	errors.Reset()
	if code := runProjectCLI([]string{"archive", "--path", project, "--confirm-path", project}, &out, &errors); code != 0 {
		t.Fatal(code, errors.String())
	}
	var record maintenance.RemovalRecord
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errors.Reset()
	if code := runProjectCLI([]string{"forget", "--path", project, "--confirm-path", project, "--archive-id", "wrong"}, &out, &errors); code != 1 {
		t.Fatal("wrong ID accepted", code)
	}
	if code := runProjectCLI([]string{"forget", "--path", project, "--confirm-path", project, "--archive-id", record.ID}, &out, &errors); code != 0 {
		t.Fatal(code, errors.String())
	}
	if _, err := os.Stat(project); err != nil {
		t.Fatal("metadata deletion touched files", err)
	}
}
