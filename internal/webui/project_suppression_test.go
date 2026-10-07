package webui

import (
	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"github.com/silverkhan/TaskMecca/internal/notify"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNestedProductionMonitorChainAndStaleScanAfterRemoval(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", filepath.Join(root, "management"))
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "disabled")
	project := testOperationProject(t)
	if err := maintenance.RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	done := make(chan bool, 1)
	go func() {
		done <- maintenance.WithProjectMonitoring(project, func() {
			_, err := scanOperationProject(project, time.Now())
			if err != nil {
				t.Error(err)
			}
			if errs := notify.Deliver(project, nil); len(errs) > 0 {
				t.Error(errs)
			}
		})
	}()
	select {
	case allowed := <-done:
		if !allowed {
			t.Fatal("active chain rejected")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("monitor notification chain deadlock")
	}
	if _, err := maintenance.RemoveProject(project); err != nil {
		t.Fatal(err)
	}
	recovery := project + "-recovery"
	if err := os.Rename(project, recovery); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(recovery, project)
	for i := 0; i < 5; i++ {
		scanOperationProjects(project, time.Now())
		if _, err := scanOperationProject(project, time.Now()); err == nil {
			t.Fatal("stale direct scan allowed")
		}
		if _, err := os.Stat(project); !os.IsNotExist(err) {
			t.Fatal("runtime recreated source", err)
		}
	}
	if _, err := os.Stat(recovery); err != nil {
		t.Fatal("recovery lost", err)
	}
}
