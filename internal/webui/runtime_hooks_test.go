package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestRuntimeHookGlobalScopeAPI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca", "data", "backlog"), 0755); err != nil {
		t.Fatal(err)
	}
	handler, err := Handler(project, "", "test")
	if err != nil {
		t.Fatal(err)
	}
	post := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/runtime/hooks", strings.NewReader(body))
		request.Header.Set("X-Task-Mecca-Action", "1")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	response := post(`{"provider":"codex","action":"enable","scope":"global"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("enable status=%d body=%s", response.Code, response.Body.String())
	}
	global, err := runtimeobs.GlobalHookStatus("codex")
	if err != nil || !global.Installed {
		t.Fatalf("global=%+v err=%v", global, err)
	}
	local, err := runtimeobs.HookStatus(project, "codex")
	if err != nil || local.Installed {
		t.Fatalf("project unexpectedly configured: %+v err=%v", local, err)
	}
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/runtime/hooks", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", get.Code, get.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	rows := payload["hooks"].([]any)
	found := false
	for _, raw := range rows {
		row := raw.(map[string]any)
		if row["provider"] != "codex" {
			continue
		}
		scopes := row["scopes"].(map[string]any)
		globalScope := scopes["global"].(map[string]any)
		projectScope := scopes["project"].(map[string]any)
		if globalScope["state"] != "verification_required" || globalScope["observed"] != false || projectScope["configured"] != false {
			t.Fatalf("scopes=%+v", scopes)
		}
		found = true
	}
	if !found {
		t.Fatal("codex row missing")
	}
	// Synthetic test event exercises the transition; production must receive
	// an actual provider Hook before reporting this state.
	rawEvent := `{"session_id":"scope-test","turn_id":"turn","hook_event_name":"SubagentStart","agent_id":"worker"}`
	if _, err := runtimeobs.ObserveHook(project, "codex", strings.NewReader(rawEvent), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	get = httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/runtime/hooks", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("observed GET status=%d body=%s", get.Code, get.Body.String())
	}
	if err := json.Unmarshal(get.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	observed := false
	for _, raw := range payload["hooks"].([]any) {
		row := raw.(map[string]any)
		if row["provider"] == "codex" {
			globalScope := row["scopes"].(map[string]any)["global"].(map[string]any)
			observed = globalScope["state"] == "observed" && globalScope["observed"] == true
		}
	}
	if !observed {
		t.Fatalf("actual Hook fixture did not advance observed state: %+v", payload["hooks"])
	}
	response = post(`{"provider":"codex","action":"disable","scope":"global"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", response.Code, response.Body.String())
	}
	global, err = runtimeobs.GlobalHookStatus("codex")
	if err != nil || global.Installed {
		t.Fatalf("global after disable=%+v err=%v", global, err)
	}
}

func TestRuntimeHookOverviewFlagsOnlyProviderActuallyInUse(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	backlogDir := filepath.Join(root, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(backlogDir, 0755); err != nil {
		t.Fatal(err)
	}
	body := []byte("# A-1 Active Codex work\n\n## 작업 개요\n\n- Agent: /root/controller/kkobugi\n- 변경범위: internal/*\n- 선행: -\n- 연관: -\n\n## 실행 정보\n\n- RuntimeProvider: codex\n- Dispatch상태: running\n- 실행근거: test\n- Fallback근거: -\n")
	if err := os.WriteFile(filepath.Join(backlogDir, "000001.A-1.active-codex.doing.md"), body, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.EnsureHooks(root, "claude"); err != nil {
		t.Fatal(err)
	}

	handler, err := Handler(root, "", "test")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/hooks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	payload := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	rows, ok := payload["hooks"].([]any)
	if !ok || len(rows) != 2 {
		t.Fatalf("hooks=%T %+v", payload["hooks"], payload["hooks"])
	}
	seen := map[string]map[string]any{}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		provider, _ := row["provider"].(string)
		seen[provider] = row
	}
	codex := seen["codex"]
	if codex["in_use"] != true || codex["configured"] != false || codex["needs_attention"] != true {
		t.Fatalf("codex status=%+v", codex)
	}
	if codex["applies_from"] != "new_root_session" {
		t.Fatalf("codex applies_from=%v", codex["applies_from"])
	}
	claude := seen["claude"]
	if claude["in_use"] != false || claude["configured"] != true || claude["needs_attention"] != false {
		t.Fatalf("claude status=%+v", claude)
	}
}
