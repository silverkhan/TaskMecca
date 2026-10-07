package webui

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Opt-in synthetic UI evidence. No native Trash or transport actions.
func TestServeA25RemovalUIFixture(t *testing.T) {
	root := os.Getenv("TASK_MECCA_A25_UI_FIXTURE_ROOT")
	if root == "" {
		t.Skip("explicit isolated fixture only")
	}
	if !strings.HasPrefix(root, "/private/tmp/a25-raichyu-ui-") {
		t.Fatal("unexpected fixture boundary")
	}
	home := filepath.Join(root, "management")
	t.Setenv("TASK_MECCA_HOME", home)
	project := filepath.Join(root, "fixture-project")
	source := filepath.Join(root, "removed-worktree-holder", "아주_긴_프로젝트_경로_for_boundary_review", "TaskMecca")
	stage := filepath.Join(filepath.Dir(source), ".task-mecca-recycle-fixture", "TaskMecca")
	for _, dir := range []string{home, filepath.Join(project, "_task_mecca", "data", "backlog"), source, filepath.Dir(stage)} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	history := []map[string]any{
		{"id": "fixture-success", "name": "Trash 이동 후 경로 다시 존재", "path": source, "folder_outcome": "moved_to_trash", "removed_at": "2026-10-07T06:24:34Z", "trash_path": "system-trash:?location=Fixture%20native%20Trash", "staging_path": stage},
		{"id": "fixture-partial", "name": "Partial recovery fixture", "path": filepath.Join(root, "missing-project"), "folder_outcome": "cleanup_partial", "cleanup_error": "Synthetic retained holder review", "retry_hint": "Manual verification required; no automatic retry"},
	}
	raw, err := json.Marshal(map[string]any{"projects": []map[string]string{{"path": project, "name": "Paused synthetic project"}}, "paused_projects": []string{project, source}, "removal_history": history})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "projects.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	h, err := Handler(project, "", "A25-fixture")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:18925")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Log("synthetic fixture http://127.0.0.1:18925; project monitoring suppressed")
	if err := http.Serve(listener, h); err != nil {
		t.Fatal(err)
	}
}
