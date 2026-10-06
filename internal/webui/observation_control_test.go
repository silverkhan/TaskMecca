package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silverkhan/TaskMecca/goassets"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestHookControlsClientContract(t *testing.T) {
	data, err := goassets.Template.ReadFile(embeddedRoot + "/web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	app := string(data)
	for _, required := range []string{`data-scope="device"`, `runtimeHookApproveSetup`, `runtimeHookDisabled`, `aria-expanded=`, `aria-controls=`, `function toggleRuntimeHookGuide`, `setRuntimeHookGuide(provider,state.runtimeHookGuideProvider!==provider)`} {
		if !strings.Contains(app, required) {
			t.Fatalf("Hook control UI missing %q", required)
		}
	}
	for _, obsolete := range []string{`data-scope="project"`, `data-scope="global"`, `scopeRow('project'`, `scopeRow('global'`} {
		if strings.Contains(app, obsolete) {
			t.Fatalf("obsolete scope selector remains: %q", obsolete)
		}
	}
}

func TestDeviceHookActionsKeepProviderStatesIndependent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.EnsureHooks(project, "codex"); err != nil {
		t.Fatal(err)
	}
	handler, err := Handler(project, "", "test")
	if err != nil {
		t.Fatal(err)
	}
	stopProjectAttentionFeedsForTest(t, project)
	post := func(provider, action string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/runtime/hooks", strings.NewReader(`{"provider":"`+provider+`","action":"`+action+`","scope":"device"}`))
		req.Header.Set("X-Task-Mecca-Action", "1")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: %d %s", provider, action, rec.Code, rec.Body.String())
		}
	}
	post("codex", "disable")
	codex, err := runtimeobs.ObservationStatus("codex")
	if err != nil || codex.Enabled {
		t.Fatalf("codex still enabled: %+v %v", codex, err)
	}
	claude, err := runtimeobs.ObservationStatus("claude")
	if err != nil || !claude.Enabled {
		t.Fatalf("claude changed: %+v %v", claude, err)
	}
	projectHook, err := runtimeobs.HookStatus(project, "codex")
	if err != nil || !projectHook.Installed {
		t.Fatalf("project hook removed: %+v %v", projectHook, err)
	}
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/runtime/hooks", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET: %d %s", get.Code, get.Body.String())
	}
	var payload struct {
		Hooks []struct {
			Provider           string `json:"provider"`
			State              string `json:"state"`
			ObservationEnabled bool   `json:"observation_enabled"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, hook := range payload.Hooks {
		if hook.Provider == "codex" && (hook.State != "disabled" || hook.ObservationEnabled) {
			t.Fatalf("disabled UI mismatch: %+v", hook)
		}
	}
	post("codex", "enable")
	codex, err = runtimeobs.ObservationStatus("codex")
	if err != nil || !codex.Enabled {
		t.Fatalf("codex not resumed: %+v %v", codex, err)
	}
	global, err := runtimeobs.GlobalHookStatus("codex")
	if err != nil || !global.Installed {
		t.Fatalf("global hook absent: %+v %v", global, err)
	}
	get = httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/runtime/hooks", nil))
	if err := json.Unmarshal(get.Body.Bytes(), &payload); err != nil { t.Fatal(err) }
	for _,hook := range payload.Hooks {
		if hook.Provider=="codex" && (hook.State!="verification_required"||!hook.ObservationEnabled) {
			t.Fatalf("setup claimed observation without a new event: %+v",hook)
		}
	}
}
