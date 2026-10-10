package webui

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/install"
	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchivedMigrationChoicePreservesRegistryAndUserData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", home)
	project := t.TempDir()
	if err := install.Init(project, "old"); err != nil {
		t.Fatal(err)
	}
	maintenance.RegisterProject(project)
	item, err := maintenance.ArchiveProject(project)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(project, "_task_mecca")
	custom := filepath.Join(target, "framework", "web", "app.js")
	os.WriteFile(custom, []byte("custom"), 0600)
	for _, rel := range []string{"data/backlog/private.md", "config.toml", ".runtime/notifications/telegram.json", "credentials.json"} {
		path := filepath.Join(target, rel)
		os.MkdirAll(filepath.Dir(path), 0700)
		os.WriteFile(path, []byte("private"), 0600)
	}
	before, _ := os.ReadFile(filepath.Join(home, "projects.json"))
	h, err := Handler(project, "", "new")
	if err != nil {
		t.Fatal(err)
	}
	for _, choice := range []string{"", "cancel", "backup"} {
		body, _ := json.Marshal(map[string]string{"project": project, "choice": choice})
		req := httptest.NewRequest(http.MethodPost, "/api/migrate", strings.NewReader(string(body)))
		req.Header.Set("X-Task-Mecca-Action", "1")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if choice == "" {
			if rec.Code != 409 || !strings.Contains(rec.Body.String(), "choice_required") {
				t.Fatal(rec.Code, rec.Body.String())
			}
			if _, err := os.Stat(filepath.Join(target, "backups")); !os.IsNotExist(err) {
				t.Fatal("backup before choice")
			}
		} else if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body.String())
		}
		after, _ := os.ReadFile(filepath.Join(home, "projects.json"))
		if string(before) != string(after) {
			t.Fatal("migration changed registry")
		}
	}
	archives, _ := maintenance.ArchivedProjects()
	if len(archives) != 1 || archives[0].ID != item.ID || maintenance.ProjectMonitoringAllowed(project) {
		t.Fatal("migration lost archive")
	}
	for _, rel := range []string{"data/backlog/private.md", "config.toml", ".runtime/notifications/telegram.json", "credentials.json"} {
		data, err := os.ReadFile(filepath.Join(target, rel))
		if err != nil || string(data) != "private" {
			t.Fatal("user data changed", rel, err)
		}
	}
}

func TestUnregisteredPrimaryWebNeverMonitors(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	project := t.TempDir()
	install.Init(project, "old")
	maintenance.RegisterWebProject(project)
	if maintenance.ProjectMonitoringAllowed(project) || len(operationProjects(project)) != 0 {
		t.Fatal("unregistered primary fallback monitored")
	}
	if len(maintenance.ListProjects()) != 0 {
		t.Fatal("Web registered project")
	}
}
