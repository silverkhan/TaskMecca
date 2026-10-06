package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDeliveryLedgerTracksFailureRetryAndRestart(t *testing.T) {
	project := t.TempDir()
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "secret-chat-123 temporary"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"message_id": 1}})
	}))
	defer server.Close()
	old := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = old }()
	if err := saveTelegram(project, TelegramConfig{Token: "secret-token", ChatID: 123, Enabled: true, Kinds: defaultKinds(), ActivatedAt: "2026-10-05T09:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	e := Event{ID: "event-1", TaskID: "B-1", Kind: "completed", At: "2026-10-05T10:00:00Z"}
	if errs := Deliver(project, []Event{e}); len(errs) != 1 {
		t.Fatalf("first attempt: %v", errs)
	}
	records, err := DeliveryRecords(project, "B-1", "event-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].State != "failed" || records[0].Attempts != 1 || !records[0].DuplicatePossible {
		t.Fatalf("failed record: %+v", records)
	}
	if errs := Deliver(project, []Event{e}); len(errs) != 0 {
		t.Fatal(errs)
	}
	if errs := Deliver(project, []Event{e}); len(errs) != 0 {
		t.Fatal(errs)
	}
	records, err = DeliveryRecords(project, "B-1", "event-1")
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || len(records) != 1 || records[0].State != "sent" || records[0].Attempts != 2 || records[0].LastResponseAt == "" {
		t.Fatalf("final record: %+v, attempts=%d", records, attempts)
	}
	data, err := os.ReadFile(ledgerPath(project))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-token") || strings.Contains(string(data), "secret-chat-123") || strings.Contains(string(data), "\"chat_id\"") {
		t.Fatalf("delivery ledger contains secret: %s", data)
	}
}

func TestDeliveryLedgerRecoversUncertainAttempt(t *testing.T) {
	project := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()
	old := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = old }()
	if err := saveTelegram(project, TelegramConfig{Token: "token", ChatID: 7, Enabled: true, Kinds: defaultKinds(), ActivatedAt: "2026-10-05T09:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	e := Event{ID: "uncertain", TaskID: "B-2", Kind: "completed", At: "2026-10-05T10:00:00Z"}
	ledger := deliveryLedger{Version: 1, Records: map[string]DeliveryRecord{e.ID: {EventID: e.ID, TaskID: e.TaskID, Kind: e.Kind, State: "sending", Attempts: 1}}}
	if err := saveLedger(project, ledger); err != nil {
		t.Fatal(err)
	}
	if errs := Deliver(project, []Event{e}); len(errs) != 0 {
		t.Fatal(errs)
	}
	records, err := DeliveryRecords(project, "B-2", e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].State != "sent" || records[0].Attempts != 2 || !records[0].DuplicatePossible {
		t.Fatalf("uncertain recovery: %+v", records)
	}
}

func TestDeliveryLedgerSuppressesLateLifecycleEvent(t *testing.T) {
	project := t.TempDir()
	sends := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()
	old := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = old }()
	if err := saveTelegram(project, TelegramConfig{Token: "token", ChatID: 7, Enabled: true, Kinds: defaultKinds(), ActivatedAt: "2026-10-05T09:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	done := Event{ID: "done", TaskID: "B-3", Kind: "completed", At: "2026-10-05T10:00:00Z"}
	late := Event{ID: "late", TaskID: "B-3", Kind: "started", At: "2026-10-05T09:30:00Z"}
	if errs := Deliver(project, []Event{done}); len(errs) != 0 {
		t.Fatal(errs)
	}
	if errs := Deliver(project, []Event{done, late}); len(errs) != 0 {
		t.Fatal(errs)
	}
	records, err := DeliveryRecords(project, "B-3", "late")
	if err != nil {
		t.Fatal(err)
	}
	if sends != 1 || len(records) != 1 || records[0].State != "suppressed_regression" {
		t.Fatalf("sends=%d late=%+v", sends, records)
	}
}
