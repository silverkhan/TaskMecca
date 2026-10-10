package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestSuppressedProjectProducersNeverRecreateRuntime(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", filepath.Join(root, "management"))
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := maintenance.RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	if _, err := maintenance.RemoveProject(project); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(project, project+"-recovery"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if len(Deliver(project, []Event{{ID: "event", Kind: "registered"}})) == 0 {
			t.Fatal("delivery did not reject suppression")
		}
		if _, err := TelegramStatusFor(project); err == nil {
			t.Fatal("migrating status allowed")
		}
		if err := saveTelegram(project, TelegramConfig{}); err == nil {
			t.Fatal("settings write allowed")
		}
		if err := saveLedger(project, deliveryLedger{}); err == nil {
			t.Fatal("ledger write allowed")
		}
		if _, err := acquireDeliveryLock(project); err == nil {
			t.Fatal("lock recreation allowed")
		}
		if _, err := ConfigureSharedTelegram([]string{project}, "fixture-token", nil); err == nil {
			t.Fatal("shared config allowed")
		}
		if _, err := runtimeobs.RecordAssignment(project, "fixture-assignment", "A-1", "/root/fixture", time.Now()); err == nil {
			t.Fatal("assignment allowed")
		}
		if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{AttemptID: "fixture"}); err == nil {
			t.Fatal("runtime event allowed")
		}
		if _, err := runtimeobs.Observe(project, "codex", strings.NewReader(`{"hook_event_name":"PostToolUse","agent_id":"fixture"}`), time.Now()); err == nil {
			t.Fatal("hook allowed")
		}
		if _, err := backlog.NotificationEvents(project, map[string]map[string]any{}); err == nil {
			t.Fatal("notification cache allowed")
		}
		if report := backlog.AccessPreflight(project, ""); report["dispatch_allowed"] != false {
			t.Fatal("preflight recreation allowed", report)
		}
		if _, err := os.Stat(project); !os.IsNotExist(err) {
			t.Fatal("source recreated", err)
		}
	}
	if _, err := os.Stat(project + "-recovery"); err != nil {
		t.Fatal("recovery lost", err)
	}
}

func TestDeliveryNestedGuardsCompleteBeforeConcurrentRemoval(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", filepath.Join(root, "management"))
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "")
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := maintenance.RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	respond := make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		close(entered)
		<-respond
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{}})
	}))
	defer server.Close()
	previous := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = previous }()
	if err := saveTelegram(project, TelegramConfig{Token: "fixture-token", ChatID: 1, Enabled: true, Kinds: defaultKinds(), ActivatedAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	delivered := make(chan []error, 1)
	go func() {
		delivered <- Deliver(project, []Event{{ID: "fixture-event", TaskID: "A-1", Kind: "registered", At: time.Now().UTC().Format(time.RFC3339Nano)}})
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("local fixture request not reached")
	}
	removed := make(chan error, 1)
	go func() { _, err := maintenance.RemoveProject(project); removed <- err }()
	close(respond)
	select {
	case errs := <-delivered:
		if len(errs) > 0 {
			t.Fatal(errs)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("nested delivery/remove deadlock")
	}
	select {
	case err := <-removed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("removal stalled")
	}
	if requests.Load() != 1 {
		t.Fatal(requests.Load())
	}
	if errs := Deliver(project, []Event{{ID: "new-event", Kind: "registered"}}); len(errs) == 0 {
		t.Fatal("stale removed delivery allowed")
	}
	if requests.Load() != 1 {
		t.Fatal("removed project sent transport")
	}
}
