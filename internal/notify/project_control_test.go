package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestSharedRecipientPersistsForEachProjectWithoutStatusSecrets(t *testing.T) {
	projectA, projectB := t.TempDir(), t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bottest/getMe" {
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"username": "shared_bot"}})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	old := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = old }()
	if _, err := ConfigureSharedTelegram([]string{projectA, projectB}, "test", nil); err != nil {
		t.Fatal(err)
	}
	for _, project := range []string{projectA, projectB} {
		cfg, err := loadTelegram(project)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.RecipientMode != "shared" || cfg.Token != "test" || cfg.ChatID != 0 {
			t.Fatalf("shared config not safely persisted: %+v", cfg)
		}
		status, err := TelegramStatusFor(project)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(status)
		if string(data) == "" || string(data) == "test" || string(data) == "1" || status.RecipientMode != "shared" {
			t.Fatalf("unsafe or invalid status: %s", data)
		}
	}
}

func TestIndividualRecipientNeverFallsBackToSharedConfiguration(t *testing.T) {
	project := t.TempDir()
	if _, err := SetTelegramRecipientMode(project, "individual"); err != nil {
		t.Fatal(err)
	}
	if errs := Deliver(project, []Event{{ID: "individual-missing", TaskID: "A-11", Kind: "started", At: time.Now().UTC().Format(time.RFC3339Nano)}}); len(errs) != 0 {
		t.Fatal(errs)
	}
	records, err := DeliveryRecords(project, "A-11", "individual-missing")
	if err != nil || len(records) != 1 || records[0].State != "suppressed_before_activation" {
		t.Fatalf("incomplete individual setup must fail closed, records=%+v err=%v", records, err)
	}
}

func TestSharedRecipientDiscoveryRoutesOneMockedChatToSharedProjects(t *testing.T) {
	projectA, projectB := t.TempDir(), t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bottest/getMe":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"username": "shared_bot"}})
		case "/bottest/getUpdates":
			updates := []any{map[string]any{"message": map[string]any{"text": "/start", "chat": map[string]any{"id": 44, "type": "private"}}}}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": updates})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	old := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = old }()
	if _, err := ConfigureSharedTelegram([]string{projectA, projectB}, "test", nil); err != nil {
		t.Fatal(err)
	}
	if status, err := DiscoverSharedTelegram([]string{projectA, projectB}, projectA); err != nil || !status.Connected {
		t.Fatalf("shared discovery=%+v err=%v", status, err)
	}
	for _, project := range []string{projectA, projectB} {
		cfg, err := loadTelegram(project)
		if err != nil || cfg.ChatID != 44 || !cfg.Enabled || cfg.RecipientMode != "shared" {
			t.Fatalf("shared route not persisted cfg=%+v err=%v", cfg, err)
		}
	}
}
