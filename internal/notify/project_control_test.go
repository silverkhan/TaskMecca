package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProjectNotificationPauseIsLocalAndDoesNotBackfill(t *testing.T) {
	projectA, projectB := t.TempDir(), t.TempDir()
	sent := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"message_id": sent}})
	}))
	defer server.Close()
	old := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = old }()
	active := true
	for _, project := range []string{projectA, projectB} {
		if err := saveTelegram(project, TelegramConfig{Token: "token", ChatID: 1, Enabled: true, Kinds: defaultKinds(), ActivatedAt: "2026-10-06T00:00:00Z", ProjectEnabled: &active}); err != nil {
			t.Fatal(err)
		}
	}
	if status, err := SetProjectNotificationsEnabled(projectA, false); err != nil || status.ProjectEnabled {
		t.Fatalf("pause status=%+v err=%v", status, err)
	}
	paused := Event{ID: "paused", TaskID: "A-8", Kind: "started", At: "2026-10-06T01:00:00Z"}
	other := Event{ID: "other", TaskID: "B-1", Kind: "started", At: "2026-10-06T01:00:00Z"}
	if errs := Deliver(projectA, []Event{paused}); len(errs) != 0 {
		t.Fatal(errs)
	}
	if errs := Deliver(projectB, []Event{other}); len(errs) != 0 {
		t.Fatal(errs)
	}
	if sent != 1 {
		t.Fatalf("only the other project should send, got %d", sent)
	}
	if _, err := SetProjectNotificationsEnabled(projectA, true); err != nil {
		t.Fatal(err)
	}
	if errs := Deliver(projectA, []Event{paused}); len(errs) != 0 {
		t.Fatal(errs)
	}
	if sent != 1 {
		t.Fatalf("paused event must not backfill, got %d", sent)
	}
	cfg, err := loadTelegram(projectA)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "token" || !cfg.Kinds["started"] {
		t.Fatalf("legacy destination/kinds changed: %+v", cfg)
	}
}

func TestLegacyGlobalDisableMigratesToProjectDefault(t *testing.T) {
	project := t.TempDir()
	if err := saveTelegram(project, TelegramConfig{
		Token: "token", ChatID: 1, Enabled: false, Kinds: defaultKinds(), ActivatedAt: "2026-10-06T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	status, err := TelegramStatusFor(project)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || !status.ProjectEnabled {
		t.Fatalf("legacy disabled configuration should migrate to the enabled project default: %+v", status)
	}
	cfg, err := loadTelegram(project)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || cfg.ProjectEnabled != nil {
		t.Fatalf("migration must retain a default project policy without inventing an explicit override: %+v", cfg)
	}
}
