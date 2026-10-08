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
)

func TestRecoveredMonitoringGapsStayDiagnosticAcrossResumes(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	project := testOperationProject(t)
	now := time.Now().UTC()
	journal, err := scanOperationProject(project, now)
	if err != nil {
		t.Fatal(err)
	}
	oldDeliver := operationDeliver
	defer func() { operationDeliver = oldDeliver }()
	var events []notify.Event
	operationDeliver = func(_ string, value []notify.Event) []error { events = append(events, value...); return nil }
	// The exact threshold is not a gap; a 200µs delay and repeated resumes are.
	for _, delta := range []time.Duration{operationGapAfter, operationGapAfter + 200*time.Microsecond, operationGapAfter + time.Second} {
		now = now.Add(delta)
		journal, err = scanOperationProject(project, now)
		if err != nil {
			t.Fatal(err)
		}
		deliverOperationIncidents(project, journal)
	}
	if len(journal.Incidents) != 2 {
		t.Fatalf("diagnostic gaps: %+v", journal.Incidents)
	}
	for _, incident := range journal.Incidents {
		if incident.Kind != "monitor_gap" || incident.RecoveredAt == "" || incident.EndedAt != "" {
			t.Fatalf("diagnostic semantics changed: %+v", incident)
		}
	}
	if len(events) != 0 {
		t.Fatalf("recovered gaps delivered: %+v", events)
	}
	reloaded, err := readOperationJournal(project)
	if err != nil || len(reloaded.Incidents) != 2 {
		t.Fatalf("history lost: %+v %v", reloaded, err)
	}
	deliverOperationIncidents(project, reloaded)
	if len(events) != 0 {
		t.Fatalf("historical gaps replayed: %+v", events)
	}
}

func TestRecoveredGapRealTelegramConsumerPreservesUnresolvedAlerts(t *testing.T) {
	project := testOperationProject(t)
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "")
	now := time.Now().UTC()
	folder := filepath.Join(project, "_task_mecca", ".runtime", "notifications")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatal(err)
	}
	config := map[string]any{"enabled": true, "token": "local-test", "chat_id": 7, "activated_at": now.Add(-time.Hour).Format(time.RFC3339Nano), "kinds": map[string]bool{"runtime_unknown": true, "interrupted": true, "intervention": true}, "delivered": []string{}}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(filepath.Join(folder, "telegram.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	defer func() { http.DefaultTransport = original }()
	var messages []string
	http.DefaultTransport = lifecycleFakeTelegramTransport{fallback: original, telegram: func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		messages = append(messages, payload["text"].(string))
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1}}`))}, nil
	}}
	stamp := now.Format(time.RFC3339Nano)
	journal := operationJournal{Incidents: []operationIncident{
		{ID: "old-gap", Kind: "monitor_gap", Evidence: "recovered scan", DetectedAt: stamp, RecoveredAt: stamp},
		{ID: "error", TaskID: "A-1", Kind: "errored", Quality: "confirmed", Evidence: "actual failure", DetectedAt: stamp},
		{ID: "shutdown", TaskID: "A-4", Kind: "shutdown", Quality: "confirmed", Evidence: "actual shutdown", DetectedAt: stamp},
		{ID: "interrupted", TaskID: "A-5", Kind: "interrupted", Quality: "confirmed", Evidence: "actual interruption", DetectedAt: stamp},
		{ID: "unknown", TaskID: "A-2", Kind: "runtime_unknown", Evidence: "unresolved evidence", DetectedAt: stamp},
	}}
	deliverOperationIncidents(project, journal)
	errs := notify.Deliver(project, []notify.Event{{ID: "user", TaskID: "A-3", Kind: "intervention", Message: "actual user decision", At: stamp}})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if len(messages) != 5 {
		t.Fatalf("actual consumer sends=%d: %v", len(messages), messages)
	}
	for _, message := range messages {
		if strings.Contains(message, "recovered scan") {
			t.Fatalf("gap sent: %s", message)
		}
	}
	deliverOperationIncidents(project, journal)
	if len(messages) != 5 {
		t.Fatal("dedupe changed")
	}
	records, err := notify.DeliveryRecords(project, "", "old-gap")
	if err != nil || len(records) != 0 {
		t.Fatalf("recovered gap consumed: %v %v", records, err)
	}
}
