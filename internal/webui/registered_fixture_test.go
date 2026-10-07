package webui

import (
	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"os"
	"path/filepath"
	"testing"
)

// Read/projection tests explicitly opt their synthetic project into monitoring.
// Each gets an isolated registry; no fixture can register into the user's home.
func registerWebFixture(t *testing.T, project string) {
	t.Helper()
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := maintenance.RegisterProject(project); err != nil {
		t.Fatal(err)
	}
}
