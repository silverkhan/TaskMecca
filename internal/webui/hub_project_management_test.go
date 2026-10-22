package webui

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHubTrashRejectsPathWithoutRemovalHistory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", filepath.Join(root, "task-mecca-home"))
	if err := os.MkdirAll(filepath.Join(root, "_task_mecca", "data", "backlog"), 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "unrecorded-folder")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	handler, err := Handler(root, "", "test")
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"action":"trash","path":"` + target + `","history_id":"removed-missing","confirm_path":"` + target + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/hub/projects", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Task-Mecca-Action", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("unrecorded folder was touched: %v", err)
	}
}


func TestHubReportsRegistryReadFailureInsteadOfNoProjects(t *testing.T) {
    root := t.TempDir()
    home := filepath.Join(root, "management")
    t.Setenv("TASK_MECCA_HOME", home)
    if err := os.MkdirAll(filepath.Join(root, "_task_mecca"), 0755); err != nil { t.Fatal(err) }
    handler, err := Handler(root, "", "test")
    if err != nil { t.Fatal(err) }

    // Simulate an interrupted upgrade / corrupted management registry.
    if err := os.MkdirAll(home, 0755); err != nil { t.Fatal(err) }
    path := filepath.Join(home, "projects.json")
    broken := []byte("{broken")
    if err := os.WriteFile(path, broken, 0600); err != nil { t.Fatal(err) }
    for _, endpoint := range []string{"/api/hub", "/api/hub/projects"} {
        req := httptest.NewRequest(http.MethodGet, endpoint, nil)
        rec := httptest.NewRecorder()
        handler.ServeHTTP(rec, req)
        if rec.Code != http.StatusInternalServerError {
            t.Fatalf("%s status=%d, expected registry error: %s", endpoint, rec.Code, rec.Body.String())
        }
        if !bytes.Contains(rec.Body.Bytes(), []byte("registry")) {
            t.Fatalf("%s must explain the registry failure: %s", endpoint, rec.Body.String())
        }
    }
    if current, err := os.ReadFile(path); err != nil || !bytes.Equal(current, broken) {
        t.Fatalf("failed Web discovery modified registry: %q %v", current, err)
    }
}
