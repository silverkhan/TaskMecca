package maintenance

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPresenceIsReadOnlyForAbsentDeniedAndNonDirectoryPaths(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "absent", "project")
	if got := projectPresence(missing); got != "missing" {
		t.Fatalf("absent presence %s", got)
	}
	if _, err := os.Stat(filepath.Dir(missing)); !os.IsNotExist(err) {
		t.Fatal("absence probe created path", err)
	}
	file := filepath.Join(dir, "regular-file")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := projectPresence(file); got != "unavailable" {
		t.Fatalf("file presence %s", got)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if got := projectPresence(link); got != "unavailable" {
		t.Fatalf("symlink presence %s", got)
	}
	if os.Geteuid() == 0 {
		t.Skip("permission denial requires an unprivileged process")
	}
	denied := filepath.Join(dir, "denied")
	child := filepath.Join(denied, "child")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(denied, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(denied, 0700)
	if got := projectPresence(child); got != "unavailable" {
		t.Fatalf("denied presence %s", got)
	}
}

func TestExplicitRegistrationAndResumeAfterHistoryDeletion(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	t.Setenv("TASK_MECCA_HOME", filepath.Join(dir, "home"))
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	record, err := RemoveProject(project)
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteRemovalHistory(record.ID); err != nil {
		t.Fatal(err)
	}
	if err := RegisterWebProject(project); err != nil {
		t.Fatal(err)
	}
	managed, err := ManagedProjects()
	if err != nil || len(managed) != 0 {
		t.Fatalf("automatic registration resurrected project: %+v %v", managed, err)
	}
	if err := RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	if ProjectMonitoringAllowed(project) {
		t.Fatal("explicit registration unexpectedly resumed monitoring")
	}
	if err := SetProjectMonitoring(project, true); err != nil {
		t.Fatal(err)
	}
	if !ProjectMonitoringAllowed(project) {
		t.Fatal("explicit resume did not enable monitoring")
	}
}

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

func TestCleanupPartialHistoryFailureReportsNativeRecovery(t *testing.T) {
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
	locator := systemTrashLocator(destination, source)
	trash, err := moveRemovedProjectToTrash(record.ID, source, func(path string) (string, error) { return locator, os.Rename(path, destination) }, func(projectRegistry) error { return errors.New("fixture write failure") })
	if err == nil || trash != locator || !strings.Contains(err.Error(), "history could not be updated") {
		t.Fatalf("partial failure: %s %v", trash, err)
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatal("recovery folder not preserved", err)
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
