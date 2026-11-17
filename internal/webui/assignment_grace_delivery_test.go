package webui

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/notify"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAssignmentFirstHookGraceScanConsumerAndRecovery(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "")
	project := testOperationProject(t)
	registerWebFixture(t, project)
	base := time.Now().UTC().Add(-10 * time.Minute)
	if err := os.WriteFile(filepath.Join(project, "_task_mecca", "data", "backlog", "000044.A-44.first-hook.doing.md"), []byte("# A-44 First hook\n- Agent: /root/controller/worker\n- 변경범위: test\n- 선행: -\n- 연관: -\n"), 0600); err != nil {
		t.Fatal(err)
	}
	assignment, err := runtimeobs.RecordAssignment(project, "assignment-first-hook", "A-44", "/root/controller/worker", base)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(project, "_task_mecca", ".runtime", "notifications")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	config, _ := json.Marshal(map[string]any{"token": "mock-only", "chat_id": 7, "enabled": true, "kinds": map[string]bool{"runtime_unknown": true, "stalled": true, "interrupted": true}, "activated_at": base.Add(-time.Hour).Format(time.RFC3339Nano)})
	if err := os.WriteFile(filepath.Join(directory, "telegram.json"), config, 0600); err != nil {
		t.Fatal(err)
	}
	count := 0
	priorTransport := http.DefaultTransport
	http.DefaultTransport = lifecycleFakeTelegramTransport{fallback: priorTransport, telegram: func(r *http.Request) (*http.Response, error) {
		count++
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1}}`)), Request: r}, nil
	}}
	defer func() { http.DefaultTransport = priorTransport }()
	scan := func(at time.Time) operationJournal {
		t.Helper()
		journal, err := scanOperationProject(project, at)
		if err != nil {
			t.Fatal(err)
		}
		return journal
	}
	before := scan(base.Add(runtimeobs.AssignmentObservationGrace - time.Nanosecond))
	deliverOperationIncidents(project, before)
	if count != 0 {
		t.Fatal("normal delay sent", count)
	}
	threshold := base.Add(runtimeobs.AssignmentObservationGrace)
	after := scan(threshold)
	deliverOperationIncidents(project, after)
	if count != 0 || len(after.Incidents) != 1 || after.Incidents[0].Kind != "handoff_stalled" || after.Incidents[0].Quality != "verification_required" {
		t.Fatalf("expired unverified observation must remain diagnostic without external delivery: %d %+v", count, after)
	}
	deliverOperationIncidents(project, scan(threshold.Add(time.Second)))
	if count != 0 {
		t.Fatal("poll sent unverified diagnostic", count)
	}
	if _, err := runtimeobs.BindRuntimeAgentForAssignment(project, assignment.AssignmentID, "first-hook-worker", "", threshold.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	hook := testOperationHook(t, project, `{"session_id":"first-hook-session","turn_id":"first-hook-turn","hook_event_name":"SubagentStart","agent_id":"first-hook-worker"}`, threshold.Add(3*time.Second))
	verifier := operationHookVerifier
	operationHookVerifier = func(string, string) bool { return true }
	defer func() { operationHookVerifier = verifier }()
	recovered := scan(threshold.Add(4 * time.Second))
	deliverOperationIncidents(project, recovered)
	if count != 0 {
		t.Fatal("recovered first hook sent diagnostic", count)
	}
	for _, item := range recovered.Incidents {
		if item.Kind == "no_signal" && item.RecoveredAt == "" {
			t.Fatal("late hook leaves warning", item)
		}
	}
	if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", AttemptID: hook.AttemptID, State: runtimeobs.StateErrored, Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved, ObservedAt: threshold.Add(5 * time.Second).Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	failed := scan(threshold.Add(6 * time.Second))
	deliverOperationIncidents(project, failed)
	if count != 0 {
		t.Fatal("unverified current ownership must not send an external error", count)
	}
	confirmed := false
	for _, item := range failed.Incidents {
		if item.AttemptID == hook.AttemptID && item.Kind == "errored" && item.Quality == "confirmed" {
			confirmed = true
		}
	}
	if !confirmed {
		t.Fatal("actual observed failure diagnostic hidden", failed)
	}
	oldRecords, err := notify.DeliveryRecords(project, "A-44", "")
	if err != nil || len(oldRecords) != 0 {
		t.Fatal("diagnostics consumed as delivery", oldRecords, err)
	}
	// Positive control: a different actual human cause with current typed
	// implementation/assignment evidence reaches the real mocked consumer once.
	installA83MockCurrentExecution(t)
	a83TypedActionFixture(t, project, "A-45", "Owner permission revoked", "Owner: restore requested permission", runtimeobs.StateErrored)
	current := scan(time.Now().UTC())
	deliverOperationIncidents(project, current)
	deliverOperationIncidents(project, current)
	if count != 1 {
		t.Fatalf("verified actionable error must deliver once, got%d %+v", count, current)
	}
	records, err := notify.DeliveryRecords(project, "A-45", "")
	if err != nil || len(records) != 1 || records[0].State != "sent" || records[0].Kind != "interrupted" || records[0].Attempts != 1 {
		t.Fatal("human error/idempotency lost", records, err)
	}
}
