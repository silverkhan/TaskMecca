package maintenance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectManagementRemovalKeepsFolderAndHistory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", filepath.Join(dir, "task-mecca-home"))
	project := filepath.Join(dir, "worktree")
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	if err := SetProjectMonitoring(project, false); err != nil {
		t.Fatal(err)
	}
	states, err := ManagedProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || states[0].Monitoring {
		t.Fatalf("paused project state = %#v", states)
	}
	record, err := RemoveProject(project)
	if err != nil {
		t.Fatal(err)
	}
	if record.ID == "" {
		t.Fatalf("archive ID = %q", record.ID)
	}
	if _, err := os.Stat(project); err != nil {
		t.Fatalf("removal must retain folder: %v", err)
	}
	history, err := RemovalHistory()
	if err != nil || len(history) != 1 || history[0].Path != project {
		t.Fatalf("history = %#v, %v", history, err)
	}
	if err := DeleteRemovalHistory(record.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(project); err != nil {
		t.Fatalf("history deletion must retain folder: %v", err)
	}
}
