package webui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silverkhan/TaskMecca/internal/maintenance"
)

func TestWorkloadSessionProjectNeverFallsBack(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	registered, other := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(registered, "_task_mecca"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := maintenance.RegisterProject(registered); err != nil {
		t.Fatal(err)
	}
	if got, err := workloadSessionProject(registered); err != nil || got != registered {
		t.Fatalf("%q %v", got, err)
	}
	for _, requested := range []string{"", "relative", other} {
		if got, err := workloadSessionProject(requested); err == nil || got != "" {
			t.Fatalf("invalid selection %q fell back to %q: %v", requested, got, err)
		}
	}
}
