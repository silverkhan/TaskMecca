package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHubProtectsRecordedRootAndMissingPath(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("TASK_MECCA_HOME", home)
	if err := os.MkdirAll(filepath.Join(root, "_task_mecca"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	handler, err := Handler(root, "", "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", filepath.Join(home, "web"), filepath.Join(root, "missing-project")} {
		registry, _ := json.Marshal(map[string]any{"removal_history": []map[string]string{{"id": "removed-fixture", "path": path, "folder_outcome": "preserved"}}})
		if err := os.WriteFile(filepath.Join(home, "projects.json"), registry, 0600); err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(map[string]string{"action": "trash", "history_id": "removed-fixture", "path": path, "confirm_path": path})
		req := httptest.NewRequest(http.MethodPost, "/api/hub/projects", strings.NewReader(string(body)))
		req.Header.Set("X-Task-Mecca-Action", "1")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("protected path %s accepted: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	if _, err := os.Stat(filepath.Join(root, "missing-project")); !os.IsNotExist(err) {
		t.Fatal("missing target recreated", err)
	}
}

func TestHubRejectsMalformedManagementContracts(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", filepath.Join(root, "home"))
	if err := os.MkdirAll(filepath.Join(root, "_task_mecca"), 0755); err != nil {
		t.Fatal(err)
	}
	handler, err := Handler(root, "", "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{"action":"delete-history"}`, `{"action":"trash","path":"/","confirm_path":"/"}`, `{"action":"trash","history_id":"fake","path":"/","confirm_path":"/other"}`, `{"action":"remove","patth":"/"}`, `{"action":"remove"} {"action":"remove"}`, `{"action":`} {
		req := httptest.NewRequest(http.MethodPost, "/api/hub/projects", strings.NewReader(body))
		req.Header.Set("X-Task-Mecca-Action", "1")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("contract %s: status=%d body=%s", body, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/hub/projects", strings.NewReader(`{"action":"remove"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("missing confirmation header: %d", rec.Code)
	}
}
