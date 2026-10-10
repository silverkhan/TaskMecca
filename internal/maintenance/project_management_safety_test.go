package maintenance

import (
	"os"
	"path/filepath"
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
	if !ProjectMonitoringAllowed(project) {
		t.Fatal("explicit add did not resume monitoring")
	}
	if err := SetProjectMonitoring(project, true); err != nil {
		t.Fatal(err)
	}
	if !ProjectMonitoringAllowed(project) {
		t.Fatal("explicit resume did not enable monitoring")
	}
}
