package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTelegramLeadingTaskLabelPreservesMeaningAndURLs(t *testing.T) {
	for _, row := range []struct{ title, want string }{
		{"A-31 작업제목", "작업제목"}, {"A-31 · A-31 작업제목", "작업제목"},
		{"[A-31] · 작업제목", "작업제목"}, {"(a-31): 작업제목", "작업제목"},
		{"A-31 - 작업제목", "작업제목"}, {"A-31", ""},
		{"A-310 작업제목", "A-310 작업제목"}, {"A-31-guide 작업제목", "A-31-guide 작업제목"},
		{"A-31가 포함된 제목", "A-31가 포함된 제목"}, {"다른 A-31과 A-32 비교", "다른 A-31과 A-32 비교"},
		{"A-31 [정책] 나머지 · 보존!  ", "[정책] 나머지 · 보존!  "},
		{"A-31 https://example.test/A-31?task=A-31", "https://example.test/A-31?task=A-31"},
		{"https://example.test/A-31", "https://example.test/A-31"},
	} {
		t.Run(row.title, func(t *testing.T) {
			if got := telegramTaskTitle(row.title, "A-31"); got != row.want {
				t.Fatalf("title=%q want=%q", got, row.want)
			}
		})
	}
}

func TestTelegramActualDeliveryIdentityAndDedup(t *testing.T) {
	var sent []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		sent = append(sent, payload)
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"message_id": len(sent)}})
	}))
	defer server.Close()
	old := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = old }()
	for _, path := range []string{"ledger", "legacy"} {
		for _, kind := range []string{"registered", "started", "intervention", "finalize", "completed"} {
			project := filepath.Join(t.TempDir(), "한글-A-31-"+path+"-"+kind)
			os.MkdirAll(project, 0755)
			now := time.Now().UTC()
			cfg := TelegramConfig{Token: "local-test", ChatID: 7, Enabled: true, Kinds: defaultKinds(), ActivatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano), TaskPhases: map[string]int{}}
			if err := saveTelegram(project, cfg); err != nil {
				t.Fatal(err)
			}
			message := "A-31과 A-32의 의미 있는 비교 https://example.test/A-31?task=A-31"
			resume := "A-31 결과 확인 https://example.test/A-31"
			event := Event{ID: path + kind, TaskID: "A-31", Kind: kind, Title: "A-31 · A-31 작업제목 https://example.test/A-31", Message: message, ResumeCondition: resume, At: now.Format(time.RFC3339Nano)}
			deliver := Deliver
			if path == "legacy" {
				deliver = deliverLegacy
			}
			before := len(sent)
			for i := 0; i < 2; i++ {
				if errs := deliver(project, []Event{event}); len(errs) > 0 {
					t.Fatal(errs)
				}
			}
			if len(sent)-before != 1 {
				t.Fatalf("%s/%s delivered %d times", path, kind, len(sent)-before)
			}
			text := sent[before]["text"].(string)
			header := "[" + filepath.Base(project) + "] A-31 · 작업제목 https://example.test/A-31"
			if !strings.Contains(text, header) || !strings.Contains(text, message) || !strings.Contains(text, "➡️ 다음 조치: "+resume) {
				t.Fatalf("identity or original text changed: %q", text)
			}
			if sent[before]["chat_id"] != float64(7) {
				t.Fatalf("recipient changed: %v", sent[before])
			}
			if path == "ledger" {
				records, err := DeliveryRecords(project, "A-31", event.ID)
				if err != nil || len(records) != 1 || records[0].State != "sent" || records[0].Attempts != 1 {
					t.Fatalf("ledger=%+v err=%v", records, err)
				}
			}
		}
	}
}
