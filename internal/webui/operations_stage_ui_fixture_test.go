package webui

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in paused synthetic fixture: no monitor, real project or transport.
func TestServeA27StageUIFixture(t *testing.T) {
	root := os.Getenv("TASK_MECCA_A27_UI_FIXTURE_ROOT")
	if root == "" {
		t.Skip("explicit isolated fixture only")
	}
	if !strings.HasPrefix(root, "/private/tmp/a27-isanghaessi-ui-") {
		t.Fatal("unexpected isolated boundary")
	}
	t.Setenv("TASK_MECCA_HOME", filepath.Join(root, "home"))
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "disabled")
	project := filepath.Join(root, "fixture-project")
	if e := os.MkdirAll(filepath.Join(project, "_task_mecca", "data", "backlog"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.MkdirAll(filepath.Join(root, "home"), 0700); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(map[string]any{"projects": []map[string]string{{"path": project, "name": "A27 synthetic fixture"}}, "paused_projects": []string{project}})
	if e := os.WriteFile(filepath.Join(root, "home", "projects.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	handler, e := Handler(project, "", "A27-fixture")
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	stages := []operationStage{{Project: project, TaskID: "A-24", AssignmentID: "synthetic-assignment-review", AttemptID: "synthetic-run", HandoffID: "synthetic-handoff", Stage: "controller_review", Evidence: "synthetic Controller claim · 원천시각 유지", Since: stamp, GraceUntil: now.Add(15 * time.Minute).Format(time.RFC3339Nano)}}
	incident := operationIncident{ID: "synthetic-failure", Project: project, TaskID: "B-458", Kind: "handoff_failed", Quality: "verification_required", Evidence: "Synthetic report delivery failed · 실제 사례의 원장은 변경하지 않습니다.", DetectedAt: stamp, Action: "Controller: 인계 전달 경로와 실패 근거를 확인하세요."}
	payload := map[string]any{"projects": []map[string]string{{"path": project, "last_scan_at": stamp}}, "active": []operationIncident{incident}, "stages": stages, "resolved_observations": []operationIncident{{ID: "synthetic-history", TaskID: "A-25", Kind: "runtime_unknown", Evidence: "historical runtime remains unknown", RecoveredAt: stamp, RecoveryEvidence: "latest assignment observed running; historical runtime unchanged"}}, "telegram_transport_disabled": true}
	wrapper := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/operations" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(payload)
			return
		}
		handler.ServeHTTP(w, r)
	})
	t.Log("synthetic fixture http://127.0.0.1:18927")
	if e := http.ListenAndServe("127.0.0.1:18927", wrapper); e != nil {
		t.Fatal(e)
	}
}
