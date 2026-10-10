package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/silverkhan/TaskMecca/internal/maintenance"
)

func TestAttentionFeedNeverRecreatesPausedRemovedOrMissingProject(t *testing.T) {
	for _, state := range []string{"paused", "removed", "history_deleted", "missing"} {
		t.Run(state, func(t *testing.T) {
			dir := t.TempDir()
			project := filepath.Join(dir, "project")
			backlogDir := filepath.Join(project, "_task_mecca", "data", "backlog")
			t.Setenv("TASK_MECCA_HOME", filepath.Join(dir, "home"))
			if err := os.MkdirAll(backlogDir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(backlogDir, "000001.B-1.fixture.todo.md"), []byte("# B-1 Fixture\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := maintenance.RegisterProject(project); err != nil {
				t.Fatal(err)
			}
			feed := &attentionFeed{project: project, root: backlogDir, subscribers: map[chan []byte]struct{}{}}
			feed.refresh()
			if state == "paused" {
				if err := maintenance.SetProjectMonitoring(project, false); err != nil {
					t.Fatal(err)
				}
			}
			if state == "removed" || state == "history_deleted" {
				record, err := maintenance.RemoveProject(project)
				if err != nil {
					t.Fatal(err)
				}
				if state == "history_deleted" {
					if err := maintenance.DeleteRemovalHistory(record.ID); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := os.Rename(project, filepath.Join(dir, "preserved-project")); err != nil {
				t.Fatal(err)
			}
			feed.refresh()
			feed.refresh()
			if _, err := os.Stat(project); !os.IsNotExist(err) {
				t.Fatalf("%s feed recreated path: %v", state, err)
			}
			if state == "removed" || state == "history_deleted" {
				if got := operationProjects(project); len(got) != 0 {
					t.Fatalf("removed primary fallback returned %+v", got)
				}
			}
		})
	}
}

func TestRemovedPrimaryReadAPIsAndWebRegistrationStaySuppressed(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	t.Setenv("TASK_MECCA_HOME", filepath.Join(dir, "home"))
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := maintenance.RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	handler, err := Handler(project, "", "test")
	if err != nil {
		t.Fatal(err)
	}
	record, err := maintenance.RemoveProject(project)
	if err != nil {
		t.Fatal(err)
	}
	if err := maintenance.DeleteRemovalHistory(record.ID); err != nil {
		t.Fatal(err)
	}
	if err := maintenance.RegisterWebProject(project); err != nil {
		t.Fatal(err)
	}
	managed, err := maintenance.ManagedProjects()
	if err != nil || len(managed) != 0 {
		t.Fatalf("Web re-registered removed primary: %+v %v", managed, err)
	}
	if got := operationProjects(project); len(got) != 0 {
		t.Fatalf("history-deleted primary monitored: %+v", got)
	}
	if err := os.Rename(project, filepath.Join(dir, "preserved-project")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/attention", "/api/hub", "/api/hub/projects", "/api/snapshot", "/api/workload", "/api/notifications/deliveries", "/api/notifications/telegram"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if _, err := os.Stat(project); !os.IsNotExist(err) {
			t.Fatalf("%s recreated source: %v", path, err)
		}
	}
}
