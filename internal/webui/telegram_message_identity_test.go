package webui

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/notify"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestOperationMonitoringTelegramPreservesProjectAndEvidence(t *testing.T) {
	project := filepath.Join(t.TempDir(), "운영-A-31-프로젝트")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}
	registerWebFixture(t, project)
	configDir := filepath.Join(project, "_task_mecca", ".runtime", "notifications")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	config, _ := json.Marshal(map[string]any{"token": "local-test", "chat_id": 7, "enabled": true, "kinds": map[string]bool{"runtime_unknown": true, "intervention": true, "completed": true}, "activated_at": now.Add(-time.Hour).Format(time.RFC3339Nano)})
	if err := os.WriteFile(filepath.Join(configDir, "telegram.json"), config, 0600); err != nil {
		t.Fatal(err)
	}
	var messages []string
	previous := http.DefaultTransport
	http.DefaultTransport = lifecycleFakeTelegramTransport{fallback: previous, telegram: func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		messages = append(messages, body["text"].(string))
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1}}`)), Request: r}, nil
	}}
	defer func() { http.DefaultTransport = previous }()
	evidence := "A-31 실행 근거 https://example.test/A-31?task=A-31"
	action := "A-31과 A-32를 확인 https://example.test/A-31"
	// Monitoring authority is a genuine exact request/action. Opaque display
	// evidence below must not be used to mint a human-action proof.
	reason := "Repository access required"
	next := "Owner: grant repository access"
	installA83MockCurrentExecution(t)
	a83TypedActionFixture(t, project, "A-31", reason, next, runtimeobs.StateRunning)
	journal, err := scanOperationProject(project, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(journal.Actions) != 1 {
		t.Fatal("missing typed human cause", journal)
	}
	actionID := journal.Actions[0].ID

	decisions, decisionErr := currentNotificationDecisions(project, "")
	decision, found := decisions["A-31"]
	if decisionErr != nil || !found || decision.EventID == "" || decision.Condition["message"] != reason || decision.Condition["resume_condition"] != next {
		t.Fatal("fixture lacks a verified current action", decisions, decisionErr)
	}
	deliveryID := decision.EventID // delivery identity is shared; operation source actionID stays unchanged

	diagnostic := operationIncident{ID: "identity-operation", TaskID: "A-31", AttemptID: "old-unverified-worker", Kind: "no_signal", Evidence: "diagnostic silence only", DetectedAt: now.Format(time.RFC3339Nano)}
	journal.Incidents = append(journal.Incidents, diagnostic)
	if err := writeOperationJournal(project, journal); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		deliverOperationIncidents(project, journal)
	}
	if len(messages) != 1 {
		t.Fatalf("operation sends=%d want1", len(messages))
	}
	if !strings.Contains(messages[0], "[운영-A-31-프로젝트] A-31") || !strings.Contains(messages[0], reason) || !strings.Contains(messages[0], next) {
		t.Fatalf("operation project/evidence changed: %s", messages[0])
	}
	records, err := notify.DeliveryRecords(project, "A-31", deliveryID)
	if err != nil || len(records) != 1 || records[0].State != "sent" || records[0].Attempts != 1 {
		t.Fatalf("ledger=%+v err=%v", records, err)
	}
	diagnostics, err := notify.DeliveryRecords(project, "A-31", diagnostic.ID)
	if err != nil || len(diagnostics) != 0 {
		t.Fatal("diagnostic masqueraded as external action", diagnostics, err)
	}
	stored, err := readOperationJournal(project)
	if err != nil || len(stored.Incidents) == 0 || stored.Incidents[len(stored.Incidents)-1].ID != diagnostic.ID || len(stored.Actions) != 1 || stored.Actions[0].ID != actionID {
		t.Fatal("diagnostic/source identity lost", stored, err)
	}
	// Exercise the original arbitrary ID/URL evidence through the actual
	// formatter and same fake Telegram transport, using a neutral lifecycle
	// fixture rather than promoting unsupported prose to operation authority.
	opaque := notify.Event{ID: "opaque-formatting-only", TaskID: "A-31", Kind: "completed", Title: "A-31", Message: evidence, ResumeCondition: action, At: time.Now().UTC().Format(time.RFC3339Nano)}
	if errs := notify.Deliver(project, []notify.Event{opaque}); len(errs) != 0 {
		t.Fatal(errs)
	}
	if len(messages) != 2 {
		t.Fatalf("formatter sends=%d want2 total", len(messages))
	}
	if !strings.Contains(messages[1], "[운영-A-31-프로젝트] A-31") || !strings.Contains(messages[1], evidence) || !strings.Contains(messages[1], action) {
		t.Fatalf("operation project/evidence changed: %s", messages[1])
	}
	if errs := notify.Deliver(project, []notify.Event{opaque}); len(errs) != 0 || len(messages) != 2 {
		t.Fatal("opaque fixture retry lost identity", errs, messages)
	}
}
