package webui

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "net/url"
    "os"
    "path/filepath"
    "testing"

    "github.com/silverkhan/TaskMecca/internal/install"
    "github.com/silverkhan/TaskMecca/internal/maintenance"
)

func TestGlobalWebRootUsesManagementHomeNotWorkingProject(t *testing.T) {
    management := filepath.Join(t.TempDir(), "management")
    t.Setenv("TASK_MECCA_HOME", management)
    want := filepath.Join(management, "web")
    if got := GlobalWebRoot(); got != want {
        t.Fatalf("global Web root: got %q want %q", got, want)
    }
    if !isGlobalWebRoot(want) || isGlobalWebRoot(t.TempDir()) {
        t.Fatal("global Web project classification is not isolated")
    }
}

func TestGlobalWebStartsEmptyThenDiscoversInitWithoutRestart(t *testing.T) {
    root := t.TempDir()
    t.Setenv("TASK_MECCA_HOME", filepath.Join(root, "management"))
    project := filepath.Join(root, "user-project")
    if err := os.MkdirAll(project, 0755); err != nil { t.Fatal(err) }

    // Web starts from the global directory before any project initialization.
    h, err := Handler(GlobalWebRoot(), "", "test")
    if err != nil { t.Fatal(err) }
    if _, err := os.Stat(filepath.Join(project, "_task_mecca")); !os.IsNotExist(err) {
        t.Fatalf("global Web initialization unexpectedly touched project: %v", err)
    }
    if _, err := os.Stat(filepath.Join(GlobalWebRoot(), "_task_mecca")); !os.IsNotExist(err) {
        t.Fatalf("global Web initialization accidentally installed framework in service dir: %v", err)
    }

    hub := func() map[string]any {
        t.Helper()
        response := httptest.NewRecorder()
        h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/hub", nil))
        if response.Code != http.StatusOK { t.Fatalf("Hub HTTP %d: %s", response.Code, response.Body.String()) }
        var data map[string]any
        if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil { t.Fatal(err) }
        if data["current_project"] != "" { t.Fatalf("global Hub selected implicit project: %#v", data["current_project"]) }
        return data
    }
    if got := len(hub()["projects"].([]any)); got != 0 { t.Fatalf("empty Hub projects: %d", got) }

    if err := install.Init(project, "test"); err != nil { t.Fatal(err) }
    if err := maintenance.RegisterProject(project); err != nil { t.Fatal(err) }

    // Same handler, same process: new project appears without Web restart.
    result := hub()
    projects := result["projects"].([]any)
    if len(projects) != 1 || projects[0].(map[string]any)["path"] != project {
        t.Fatalf("new project not visible without restart: %#v", result["projects"])
    }
    // Explicit UI selection still loads project-specific endpoints.
    rec := httptest.NewRecorder()
    h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/backlog-folders?project="+url.QueryEscape(project), nil))
    if rec.Code != http.StatusOK { t.Fatalf("selected project unavailable: %d %s", rec.Code, rec.Body.String()) }
}

