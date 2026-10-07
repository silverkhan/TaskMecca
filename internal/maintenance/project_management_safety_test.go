package maintenance

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupProtectsSharedManagementAndOriginalRoots(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", filepath.Join(dir, "home"))
	for _, path := range []string{"/", "/Users", "/Users/Shared", os.TempDir(), filepath.Join(dir, "home"), filepath.Join(dir, "home", "web"), filepath.Join(dir, ".Trash", "saved"), filepath.Join(dir, "project", "_task_mecca", "data"), dir} {
		if _, err := safeCleanupPath(path); err == nil {
			t.Fatalf("protected path accepted: %s", path)
		}
	}
	repo := filepath.Join(dir, "original")
	child := filepath.Join(repo, "child")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := safeCleanupPath(child); err == nil {
		t.Fatal("original repository child accepted")
	}
}

func TestCleanupPartialHistoryFailureRestoresFolder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", filepath.Join(dir, "home"))
	source := filepath.Join(dir, "project")
	destination := filepath.Join(dir, "fixture-trash")
	if err := os.MkdirAll(filepath.Join(source, "_task_mecca"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := RegisterProject(source); err != nil {
		t.Fatal(err)
	}
	record, err := RemoveProject(source)
	if err != nil {
		t.Fatal(err)
	}
	trash, err := moveRemovedProjectToTrash(record.ID, source, func(path string) (string, error) { return destination, os.Rename(path, destination) }, func(projectRegistry) error { return errors.New("fixture write failure") })
	if err == nil || trash != "" || !strings.Contains(err.Error(), "folder restored") {
		t.Fatalf("partial failure: %s %v", trash, err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("folder not restored", err)
	}
	history, err := RemovalHistory()
	if err != nil || len(history) != 1 || history[0].FolderOutcome != "preserved" {
		t.Fatalf("history: %+v %v", history, err)
	}
}

func TestDuplicateCleanupRejectsAlreadyMovedRecord(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", filepath.Join(dir, "home"))
	source := filepath.Join(dir, "project")
	destination := filepath.Join(dir, "fixture-trash")
	if err := os.MkdirAll(filepath.Join(source, "_task_mecca"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := RegisterProject(source); err != nil {
		t.Fatal(err)
	}
	record, err := RemoveProject(source)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	move := func(path string) (string, error) { calls++; return destination, os.Rename(path, destination) }
	if _, err := moveRemovedProjectToTrash(record.ID, source, move, writeProjectRegistry); err != nil {
		t.Fatal(err)
	}
	if _, err := moveRemovedProjectToTrash(record.ID, source, move, writeProjectRegistry); err == nil {
		t.Fatal("duplicate accepted")
	}
	if calls != 1 {
		t.Fatalf("move called %d times", calls)
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatal(err)
	}
}
