package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
