package notify

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMaintenanceTransportBlocksAllTelegramWithoutChangingSavedState(t *testing.T) {
	project := t.TempDir()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	defer server.Close()
	old := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = old }()
	if err := saveTelegram(project, TelegramConfig{Token: "fixture-token", ChatID: 123, Enabled: true, Kinds: defaultKinds(), ActivatedAt: "2026-10-05T09:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(telegramPath(project))
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "disabled")
	if errors := Deliver(project, []Event{{ID: "safety-test", TaskID: "A-23", Kind: "completed", At: "2026-10-07T05:00:00Z"}}); len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, method := range []string{"getMe", "getUpdates", "sendMessage"} {
		if err := telegramCall("fixture-token", method, map[string]any{}, nil); err == nil {
			t.Fatal("direct transport was not blocked")
		}
	}
	after, _ := os.ReadFile(telegramPath(project))
	if calls != 0 || !bytes.Equal(before, after) {
		t.Fatal("maintenance transmitted or changed saved configuration")
	}
	if _, err := os.Stat(ledgerPath(project)); !os.IsNotExist(err) {
		t.Fatal("maintenance consumed or modified delivery ledger", err)
	}
}
