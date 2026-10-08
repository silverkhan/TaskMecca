package webui

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/notify"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNotificationHistoryUsesDeliveryEvidenceAndOmitsPrivateFields(t *testing.T) {
	project := t.TempDir()
	directory := filepath.Join(project, "_task_mecca", ".runtime", "notifications")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	events := `{"version":4,"events":[{"id":"sent","task_id":"A-1","kind":"completed","at":"2026-10-08T00:00:00Z","title":"실제 완료","message":"검증 완료"},{"id":"event-only","task_id":"A-2","kind":"started","at":"2026-10-08T00:00:01Z","title":"이벤트만 존재"}]}`
	ledger := `{"version":1,"token":"secret-token","chat_id":123456,"records":{"sent":{"event_id":"sent","task_id":"A-1","kind":"completed","channel":"telegram","state":"sent","attempts":1,"last_response_at":"2026-10-08T00:00:02Z","last_error":"secret-error-token","chat_id":98765},"failed":{"event_id":"failed","task_id":"A-3","kind":"intervention","channel":"telegram","state":"failed","attempts":2},"suppressed":{"event_id":"suppressed","task_id":"A-4","kind":"started","channel":"telegram","state":"suppressed_project_disabled","attempts":0}}}`
	journalPath := filepath.Join(filepath.Dir(directory), "notification_events.json")
	ledgerPath := filepath.Join(directory, "delivery_ledger.json")
	for path, body := range map[string]string{journalPath: events, ledgerPath: ledger} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := notificationHistory(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("dedupe/join: %#v", rows)
	}
	states := map[string]string{}
	for _, row := range rows {
		states[row["event_id"].(string)] = row["telegram"].(map[string]any)["state"].(string)
	}
	if states["sent"] != "sent" || states["event-only"] != "unknown" || states["failed"] != "failed" || states["suppressed"] != "suppressed_project_disabled" {
		t.Fatal(states)
	}
	data, _ := json.Marshal(rows)
	for _, secret := range []string{"secret-token", "secret-error-token", "chat_id", "123456", "98765"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("private field leaked: %s", secret)
		}
	}
	for path, body := range map[string]string{journalPath: events, ledgerPath: ledger} {
		after, _ := os.ReadFile(path)
		if string(after) != body {
			t.Fatal("history read mutated evidence")
		}
	}
}

func TestNotificationSettingsDefaultsExistingOffAndProjectChannelConsumer(t *testing.T) {
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "")
	project := t.TempDir()
	directory := filepath.Join(project, "_task_mecca", ".runtime", "notifications")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	config := `{"token":"mock-only","chat_id":7,"enabled":true,"kinds":{},"activated_at":"2026-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(directory, "telegram.json"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	status, err := notify.TelegramStatusFor(project)
	if err != nil {
		t.Fatal(err)
	}
	for kind, on := range status.Kinds {
		if !on {
			t.Fatalf("initial type disabled: %s", kind)
		}
	}
	count := 0
	previous := http.DefaultTransport
	http.DefaultTransport = lifecycleFakeTelegramTransport{fallback: previous, telegram: func(r *http.Request) (*http.Response, error) {
		count++
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1}}`)), Request: r}, nil
	}}
	defer func() { http.DefaultTransport = previous }()
	at := time.Now().UTC().Add(time.Minute)
	event := func(id, kind string) notify.Event {
		return notify.Event{ID: id, TaskID: id, Kind: kind, Title: "설정 보존", At: at.Format(time.RFC3339Nano)}
	}
	events := []notify.Event{}
	for kind := range status.Kinds {
		events = append(events, event("initial-"+kind, kind))
	}
	if errors := notify.Deliver(project, events); len(errors) > 0 {
		t.Fatal(errors)
	}
	if count != len(status.Kinds) {
		t.Fatalf("initial all on: %d", count)
	}
	status.Kinds["registered"] = false
	status.Kinds["completed"] = false
	if _, err := notify.UpdateTelegramKinds(project, status.Kinds); err != nil {
		t.Fatal(err)
	}
	preserved, err := notify.TelegramStatusFor(project)
	if err != nil {
		t.Fatal(err)
	}
	if preserved.Kinds["registered"] || preserved.Kinds["completed"] || !preserved.Kinds["started"] {
		t.Fatal(preserved)
	}
	baseline := count
	if errors := notify.Deliver(project, []notify.Event{event("off-registered", "registered"), event("off-completed", "completed"), event("on-started", "started")}); len(errors) > 0 {
		t.Fatal(errors)
	}
	if count != baseline+1 {
		t.Fatal("type settings ignored", count)
	}
	if _, err := notify.SetProjectNotificationsEnabled(project, false); err != nil {
		t.Fatal(err)
	}
	baseline = count
	notify.Deliver(project, []notify.Event{event("channel-off", "started")})
	if count != baseline {
		t.Fatal("disabled project/channel sent")
	}
	if _, err := notify.SetProjectNotificationsEnabled(project, true); err != nil {
		t.Fatal(err)
	}
	preserved, _ = notify.TelegramStatusFor(project)
	if preserved.Kinds["registered"] || preserved.Kinds["completed"] {
		t.Fatal("channel toggle overwrote type off")
	}
	notify.Deliver(project, []notify.Event{event("channel-off", "started"), event("new-channel-on", "started")})
	if count != baseline+1 {
		t.Fatalf("disabled episode replay or new channel ignored: %d", count)
	}
}
