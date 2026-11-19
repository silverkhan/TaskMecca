package backlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNotificationEventsPersistCompletionTransition(t *testing.T) {
	project := t.TempDir()

	doing := map[string]map[string]any{
		"A-23": {
			"id":         "A-23",
			"title":      "Task",
			"file_state": "doing",
			"updated_at": "2026-09-30T10:00:00Z",
		},
	}
	events, err := NotificationEvents(project, doing)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("baseline events=%+v", events)
	}

	done := map[string]map[string]any{
		"A-23": {
			"id":           "A-23",
			"title":        "Task",
			"file_state":   "done",
			"updated_at":   "2026-09-30T10:05:00Z",
			"completed_at": "2026-09-30T10:05:00Z",
		},
	}
	events, err = NotificationEvents(project, done)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events=%+v", events)
	}
	if events[0]["task_id"] != "A-23" || events[0]["kind"] != "completed" {
		t.Fatalf("event=%+v", events[0])
	}

	again, err := NotificationEvents(project, done)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 {
		t.Fatalf("duplicate events=%+v", again)
	}
	if again[0]["id"] != events[0]["id"] {
		t.Fatalf("event id changed: %v != %v", again[0]["id"], events[0]["id"])
	}
}

func TestNotificationEventsDoNotNotifyHistoricalDoneOnFirstObservation(t *testing.T) {
	project := t.TempDir()
	done := map[string]map[string]any{
		"A-22": {
			"id":           "A-22",
			"title":        "Already done",
			"file_state":   "done",
			"updated_at":   "2026-09-29T10:00:00Z",
			"completed_at": "2026-09-29T10:00:00Z",
		},
	}
	events, err := NotificationEvents(project, done)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("historical completion should establish baseline: %+v", events)
	}
}

func TestNotificationEventsPersistAttentionEpisode(t *testing.T) {
	project := t.TempDir()
	items := map[string]map[string]any{
		"AID-39": {"file_state": "doing", "updated_at": "2026-10-03T00:00:00Z", "title": "Runtime sensing"},
	}
	events, err := NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("baseline events=%v", events)
	}

	items["AID-39"]["notification_condition"] = canonicalTaskRequestFixture("AID-39", "Owner access needed", "Owner: grant access", nil)
	events, err = NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events=%v", events)
	}
	if events[0]["kind"] != "intervention" || events[0]["task_id"] != "AID-39" {
		t.Fatalf("event=%v", events[0])
	}
	again, err := NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 {
		t.Fatalf("duplicate attention event: %v", again)
	}

	items["AID-39"]["notification_condition"] = nil
	if _, err = NotificationEvents(project, items); err != nil {
		t.Fatal(err)
	}
	items["AID-39"]["notification_condition"] = canonicalTaskRequestFixture("AID-39", "Owner access needed", "Owner: grant access", nil)
	resumed, err := NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed) != 2 {
		t.Fatalf("reappearing attention should create a new episode, got %v", resumed)
	}
}

func TestNotificationEventsConsumeCanonicalLifecycle(t *testing.T) {
	project := t.TempDir()
	items := map[string]map[string]any{
		"AID-39": {"file_state": "doing", "updated_at": "2026-10-03T03:00:00Z", "title": "Runtime sensing",
			"activity": map[string]any{"source": "execution_ledger", "attempt_id": "run-1", "runtime_state": "running", "health": "active"}},
	}
	// Runtime health alone must not independently manufacture a start event.
	events, err := NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("runtime-only start must wait for canonical lifecycle: %v", events)
	}

	items["AID-39"]["lifecycle"] = map[string]any{"started_at": "2026-10-03T03:00:01Z"}
	events, err = NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0]["kind"] != "started" {
		t.Fatalf("canonical start events=%v", events)
	}

	// Refreshing the same canonical event must not duplicate the notification.
	events, err = NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("duplicate canonical start=%v", events)
	}

	items["AID-39"]["notification_condition"] = map[string]any{"kind": "finalize", "key": "runtime:finalize\\x00run-1", "reason_type": "completion_pending", "message": "done"}
	events, err = NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1]["kind"] != "finalize" {
		t.Fatalf("finalize events=%v", events)
	}
	items["AID-39"]["file_state"] = "done"
	items["AID-39"]["lifecycle"] = map[string]any{"started_at": "2026-10-03T03:00:01Z", "completed_at": "2026-10-03T03:10:00Z"}
	events, err = NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[2]["kind"] != "completed" {
		t.Fatalf("completed events=%v", events)
	}
}

func TestNotificationEventsRegistrationBaselinesExistingItems(t *testing.T) {
	project := t.TempDir()
	initial := map[string]map[string]any{
		"B-1": {"file_state": "backlog", "updated_at": "2026-10-03T01:00:00Z", "title": "existing"},
	}
	events, err := NotificationEvents(project, initial)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("initial baseline must not backfill registration: %v", events)
	}
	initial["B-2"] = map[string]any{"file_state": "backlog", "updated_at": "2026-10-03T02:00:00Z", "title": "new"}
	events, err = NotificationEvents(project, initial)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0]["kind"] != "registered" || events[0]["task_id"] != "B-2" {
		t.Fatalf("registration events=%v", events)
	}
}

func TestCanonicalLifecycleEmitsStartedThenCompletedOnce(t *testing.T) {
	project := t.TempDir()
	base := map[string]map[string]any{"B-442": {"file_state": "backlog", "updated_at": "2026-10-05T01:31:00Z", "title": "Task"}}
	if _, err := NotificationEvents(project, base); err != nil {
		t.Fatal(err)
	}
	running := map[string]map[string]any{"B-442": {"file_state": "doing", "updated_at": "2026-10-05T01:32:00Z", "title": "Task", "lifecycle": map[string]any{"started_at": "2026-10-05T01:32:00Z"}}}
	events, err := NotificationEvents(project, running)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0]["kind"] != "started" {
		t.Fatalf("started event missing: %+v", events)
	}
	done := map[string]map[string]any{"B-442": {"file_state": "done", "updated_at": "2026-10-05T01:35:00Z", "title": "Task", "completed_at": "2026-10-05T01:35:00Z", "lifecycle": map[string]any{"started_at": "2026-10-05T01:32:00Z", "completed_at": "2026-10-05T01:35:00Z"}}}
	events, err = NotificationEvents(project, done)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0]["kind"] != "started" || events[1]["kind"] != "completed" {
		t.Fatalf("lifecycle notification sequence=%+v", events)
	}
	again, err := NotificationEvents(project, done)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 2 {
		t.Fatalf("lifecycle notifications duplicated: %+v", again)
	}
}

func TestNotificationEventsConcurrentReconcileKeepsStartedEventAndValidJournal(t *testing.T) {
	project := t.TempDir()
	base := map[string]map[string]any{
		"B-500": {"file_state": "todo", "updated_at": "2026-10-05T10:00:00Z", "title": "Concurrent"},
	}
	if _, err := NotificationEvents(project, base); err != nil {
		t.Fatal(err)
	}

	running := map[string]map[string]any{
		"B-500": {"file_state": "doing", "updated_at": "2026-10-05T10:01:00Z", "title": "Concurrent",
			"lifecycle": map[string]any{"started_at": "2026-10-05T10:01:00Z"}},
	}
	var wg sync.WaitGroup
	errs := make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := NotificationEvents(project, running)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent reconcile failed: %v", err)
		}
	}

	events, err := ReadNotificationEvents(project)
	if err != nil {
		t.Fatalf("journal unreadable after concurrent reconcile: %v", err)
	}
	starts := 0
	for _, event := range events {
		if toString(event["task_id"]) == "B-500" && toString(event["kind"]) == "started" {
			starts++
		}
	}
	if starts != 1 {
		t.Fatalf("started event count=%d events=%+v", starts, events)
	}
}

func TestControlTowerAllOperationalKindsJournalExactlyOnce(t *testing.T) {
	cases := []struct{ health, state, kind string }{
		{"", "waiting_approval", "approval"},
		{"needs_user", "waiting_user", "intervention"},
		{"stale", "running", "stalled"},
		{"execution_interrupted", "errored", "interrupted"},
		{"awaiting_finalize", "completed", "finalize"},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			project := t.TempDir()
			base := map[string]map[string]any{"B-600": {"file_state": "doing", "updated_at": "2026-10-05T11:00:00Z", "title": "Kinds"}}
			if _, err := NotificationEvents(project, base); err != nil {
				t.Fatal(err)
			}
			row := Record{ID: "B-600", State: "doing", Location: "active"}
			_, condition := canonicalOperationalState(row, nil, map[string]any{"source": "execution_ledger", "attempt_id": "run-600", "runtime_state": tc.state, "health": tc.health, "human_action_runtime_verified": true})
			item := map[string]any{"file_state": "doing", "updated_at": "2026-10-05T11:01:00Z", "title": "Kinds", "notification_condition": condition}
			events, err := NotificationEvents(project, map[string]map[string]any{"B-600": item})
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, event := range events {
				if toString(event["kind"]) == tc.kind {
					count++
				}
			}
			want := 1
			if tc.kind == "stalled" {
				want = 0
			}
			if count != want {
				t.Fatalf("kind %s count=%d want=%d events=%+v", tc.kind, count, want, events)
			}
			again, err := NotificationEvents(project, map[string]map[string]any{"B-600": item})
			if err != nil {
				t.Fatal(err)
			}
			if len(again) != len(events) {
				t.Fatalf("kind %s duplicated: %+v", tc.kind, again)
			}
		})
	}
}

func TestRuntimeUnknownNotificationRemainsDiagnosticAfterLongDwell(t *testing.T) {
	project := t.TempDir()
	base := map[string]map[string]any{
		"B-610": {"file_state": "doing", "updated_at": "2026-10-06T00:00:00Z", "title": "Transient hook gap"},
	}
	if events, err := NotificationEvents(project, base); err != nil || len(events) != 0 {
		t.Fatalf("baseline events=%+v err=%v", events, err)
	}

	unknown := map[string]map[string]any{
		"B-610": {
			"file_state": "doing", "updated_at": "2026-10-06T00:00:01Z", "title": "Transient hook gap",
			"notification_condition": map[string]any{
				"kind": "runtime_unknown", "key": "runtime:unknown\x00run-610",
				"reason_type": "runtime_unknown", "message": "temporarily unknown",
				"attempt_id": "run-610", "runtime_state": "runtime_unknown",
			},
		},
	}
	events, err := NotificationEvents(project, unknown)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("runtime_unknown must not alert immediately: %+v", events)
	}

	path := filepath.Join(project, "_task_mecca", ".runtime", "notification_events.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var journal notificationJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		t.Fatal(err)
	}
	obs := journal.Items["B-610"]
	// One minute is enough for a previously-started runtime loss, but a task
	// that has not established canonical start yet gets a longer startup grace.
	obs.ConditionSince = time.Now().Add(-time.Minute - time.Second).Format(time.RFC3339Nano)
	obs.ConditionConsumed = false
	journal.Items["B-610"] = obs
	encoded, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	events, err = NotificationEvents(project, unknown)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if toString(event["task_id"]) == "B-610" && toString(event["kind"]) == "runtime_unknown" {
			t.Fatalf("startup runtime_unknown must not alert after only the active grace: %+v", events)
		}
	}

	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &journal); err != nil {
		t.Fatal(err)
	}
	obs = journal.Items["B-610"]
	obs.ConditionSince = time.Now().Add(-24*time.Hour - time.Second).Format(time.RFC3339Nano)
	obs.ConditionConsumed = false
	journal.Items["B-610"] = obs
	encoded, err = json.MarshalIndent(journal, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	events, err = NotificationEvents(project, unknown)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, event := range events {
		if toString(event["task_id"]) == "B-610" && toString(event["kind"]) == "runtime_unknown" {
			count++
		}
	}
	if count != 0 {
		t.Fatalf("persistent uncertainty must remain diagnostic after any dwell: %+v", events)
	}

	again, err := NotificationEvents(project, unknown)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != len(events) {
		t.Fatalf("runtime_unknown duplicated after consumption: %+v", again)
	}
}

func TestNotificationCompletedPhaseBlocksLateReplayAfterObservationLoss(t *testing.T) {
	project := t.TempDir()
	base := map[string]map[string]any{
		"B-920": {"file_state": "todo", "updated_at": "2026-10-06T00:00:00Z", "title": "Monotonic"},
	}
	if events, err := NotificationEvents(project, base); err != nil || len(events) != 0 {
		t.Fatalf("baseline events=%+v err=%v", events, err)
	}

	running := map[string]map[string]any{
		"B-920": {
			"file_state": "doing", "updated_at": "2026-10-06T00:01:00Z", "title": "Monotonic",
			"lifecycle": map[string]any{"started_at": "2026-10-06T00:01:00Z"},
		},
	}
	events, err := NotificationEvents(project, running)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || toString(events[0]["kind"]) != "started" {
		t.Fatalf("running events=%+v", events)
	}

	running["B-920"]["notification_condition"] = map[string]any{
		"kind": "finalize", "key": "runtime:finalize\x00run-920",
		"reason_type": "completion_pending", "message": "worker done",
	}
	events, err = NotificationEvents(project, running)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || toString(events[1]["kind"]) != "finalize" {
		t.Fatalf("finalize events=%+v", events)
	}

	done := map[string]map[string]any{
		"B-920": {
			"file_state": "done", "updated_at": "2026-10-06T00:03:00Z", "title": "Monotonic",
			"lifecycle": map[string]any{
				"started_at":   "2026-10-06T00:01:00Z",
				"completed_at": "2026-10-06T00:03:00Z",
			},
		},
	}
	events, err = NotificationEvents(project, done)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || toString(events[2]["kind"]) != "completed" {
		t.Fatalf("completed events=%+v", events)
	}

	// Simulate a partial journal/snapshot loss: the task observation disappears,
	// but durable phase/event history remains. Reappearance must not look new.
	path := filepath.Join(project, "_task_mecca", ".runtime", "notification_events.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var journal notificationJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		t.Fatal(err)
	}
	delete(journal.Items, "B-920")
	encoded, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	replay := map[string]map[string]any{
		"B-920": {
			"file_state": "done", "updated_at": "2026-10-06T00:04:00Z", "title": "Monotonic",
			"lifecycle": map[string]any{
				"created_at":   "2026-10-06T00:00:00Z",
				"started_at":   "2026-10-06T00:01:00Z",
				"completed_at": "2026-10-06T00:03:00Z",
			},
			"notification_condition": map[string]any{
				"kind": "runtime_unknown", "key": "runtime:unknown\x00run-920",
				"reason_type": "runtime_unknown", "message": "late stale signal",
			},
		},
	}
	again, err := NotificationEvents(project, replay)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != len(events) {
		t.Fatalf("terminal task replayed notifications: before=%+v after=%+v", events, again)
	}
	for _, event := range again {
		if toString(event["task_id"]) == "B-920" && (toString(event["kind"]) == "registered" || toString(event["kind"]) == "runtime_unknown") {
			t.Fatalf("terminal lifecycle regressed: %+v", event)
		}
	}
}

func TestNotificationFinalizeBlocksLateStartedRegression(t *testing.T) {
	project := t.TempDir()
	base := map[string]map[string]any{
		"B-921": {"file_state": "doing", "updated_at": "2026-10-06T01:00:00Z", "title": "Finalize first"},
	}
	if events, err := NotificationEvents(project, base); err != nil || len(events) != 0 {
		t.Fatalf("baseline events=%+v err=%v", events, err)
	}

	base["B-921"]["notification_condition"] = map[string]any{
		"kind": "finalize", "key": "runtime:finalize\x00run-921",
		"reason_type": "completion_pending", "message": "worker done",
	}
	events, err := NotificationEvents(project, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || toString(events[0]["kind"]) != "finalize" {
		t.Fatalf("finalize events=%+v", events)
	}

	// If lifecycle start evidence is discovered late, it belongs in lifecycle
	// history but must not become a push notification after finalize.
	base["B-921"]["lifecycle"] = map[string]any{"started_at": "2026-10-06T00:59:00Z"}
	events, err = NotificationEvents(project, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("late started regressed notification stream: %+v", events)
	}
}

func TestControlTowerHoldUserProducesCanonicalIntervention(t *testing.T) {
	row := Record{ID: "B-601", State: "hold", Location: "active"}
	reason, condition := canonicalOperationalState(row, map[string]any{"wait_kind": "user", "wait_note": "User answer to the deployment question is required", "resume_condition": "User: answer the deployment question"}, nil)
	if toString(reason["type"]) != "user_intervention" || toString(condition["kind"]) != "intervention" {
		t.Fatalf("reason=%+v condition=%+v", reason, condition)
	}
}

func TestNotificationEventsLegacyJournalMigrationDoesNotBackfillCurrentState(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "_task_mecca", ".runtime", "notification_events.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	legacy := `{
  "version": 1,
  "items": {
    "B-800": {
      "file_state": "doing",
      "updated_at": "2026-10-05T11:20:00Z",
      "attention_key": "legacy-attention",
      "runtime_key": "legacy-runtime",
      "started_at": "2026-10-05T11:00:00Z"
    }
  },
  "events": []
}`
	if err := os.WriteFile(path, []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	items := map[string]map[string]any{
		"B-800": {
			"file_state": "doing", "updated_at": "2026-10-05T11:20:00Z", "title": "Existing",
			"lifecycle":              map[string]any{"started_at": "2026-10-05T11:00:00Z"},
			"notification_condition": canonicalTaskRequestFixture("B-800", "existing owner access needed", "Owner: grant requested access", nil),
		},
	}
	if !HumanActionCondition(items["B-800"]["notification_condition"].(map[string]any)) {
		t.Fatal("existing-target migration fixture lost genuine authority")
	}
	events, err := NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("schema migration must baseline current state, got %+v", events)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var journal notificationJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		t.Fatal(err)
	}
	if journal.Version != notificationJournalVersion {
		t.Fatalf("version=%d", journal.Version)
	}
	if journal.Items["B-800"].ConditionKey != toString(items["B-800"]["notification_condition"].(map[string]any)["key"]) {
		t.Fatalf("condition key not migrated into baseline: %+v", journal.Items["B-800"])
	}
}

func TestNotificationEventsColdStartBaselinesHistoricalStartedAndCondition(t *testing.T) {
	project := t.TempDir()
	items := map[string]map[string]any{
		"B-801": {
			"file_state": "doing", "updated_at": "2026-10-04T08:00:00Z", "title": "Historical",
			"lifecycle": map[string]any{"started_at": "2026-10-04T07:30:00Z"},
			"notification_condition": map[string]any{
				"kind": "finalize", "key": "runtime:finalize:run-old",
				"reason_type": "completion_pending", "message": "historical",
			},
		},
	}
	events, err := NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("cold start must establish baseline without backfill: %+v", events)
	}
}

func TestNotificationEventsRecoveredBindingDoesNotEmitStarted(t *testing.T) {
	project := t.TempDir()
	base := map[string]map[string]any{
		"B-802": {"file_state": "doing", "updated_at": "2026-10-05T12:00:00Z", "title": "Recovered"},
	}
	if events, err := NotificationEvents(project, base); err != nil || len(events) != 0 {
		t.Fatalf("baseline events=%+v err=%v", events, err)
	}

	recovered := map[string]map[string]any{
		"B-802": {
			"file_state": "doing", "updated_at": "2026-10-05T12:00:00Z", "title": "Recovered",
			"lifecycle": map[string]any{
				"started_at":                      "2026-10-05T12:05:00Z",
				"started_notification_suppressed": true,
			},
		},
	}
	events, err := NotificationEvents(project, recovered)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("recovery binding must not emit started: %+v", events)
	}

	recovered["B-802"]["lifecycle"] = map[string]any{
		"started_at":                      "2026-10-05T12:05:00Z",
		"started_notification_suppressed": false,
	}
	again, err := NotificationEvents(project, recovered)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("same recovered start must remain consumed: %+v", again)
	}
}

func TestNotificationEventsDeletedJournalRebaselinesInsteadOfReplaying(t *testing.T) {
	project := t.TempDir()
	base := map[string]map[string]any{
		"B-803": {"file_state": "todo", "updated_at": "2026-10-05T12:00:00Z", "title": "Rebaseline"},
	}
	if _, err := NotificationEvents(project, base); err != nil {
		t.Fatal(err)
	}
	running := map[string]map[string]any{
		"B-803": {
			"file_state": "doing", "updated_at": "2026-10-05T12:01:00Z", "title": "Rebaseline",
			"lifecycle": map[string]any{"started_at": "2026-10-05T12:01:00Z"},
		},
	}
	events, err := NotificationEvents(project, running)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0]["kind"] != "started" {
		t.Fatalf("expected live start before deletion: %+v", events)
	}

	journalPath := filepath.Join(project, "_task_mecca", ".runtime", "notification_events.json")
	if err := os.Remove(journalPath); err != nil {
		t.Fatal(err)
	}
	events, err = NotificationEvents(project, running)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("journal recovery must baseline current state, got %+v", events)
	}
}

func TestNotificationEventsNewTaskAlreadyCompletedEmitsAllCanonicalStages(t *testing.T) {
	project := t.TempDir()
	base := map[string]map[string]any{
		"B-001": {"file_state": "todo", "updated_at": "2026-10-05T01:00:00Z", "title": "Baseline"},
	}
	if events, err := NotificationEvents(project, base); err != nil || len(events) != 0 {
		t.Fatalf("baseline events=%+v err=%v", events, err)
	}

	items := map[string]map[string]any{
		"B-001": {"file_state": "todo", "updated_at": "2026-10-05T01:00:00Z", "title": "Baseline"},
		"B-002": {
			"file_state": "done", "updated_at": "2026-10-05T01:03:00Z", "title": "Fast task",
			"lifecycle": map[string]any{
				"created_at":   "2026-10-05T01:00:30Z",
				"started_at":   "2026-10-05T01:01:00Z",
				"completed_at": "2026-10-05T01:02:00Z",
			},
		},
	}
	events, err := NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	got := []map[string]any{}
	for _, event := range events {
		if toString(event["task_id"]) == "B-002" {
			got = append(got, event)
		}
	}
	if len(got) != 3 {
		t.Fatalf("fast lifecycle events=%+v", got)
	}
	want := []string{"registered", "started", "completed"}
	for i, kind := range want {
		if toString(got[i]["kind"]) != kind {
			t.Fatalf("event[%d]=%+v want kind=%s", i, got[i], kind)
		}
	}
	if toString(got[0]["at"]) != "2026-10-05T01:00:30Z" {
		t.Fatalf("registration must use canonical created_at: %+v", got[0])
	}

	again, err := NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, event := range again {
		if toString(event["task_id"]) == "B-002" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("fast lifecycle duplicated or lost: %+v", again)
	}
}

func TestNotificationEventsNewTaskCurrentConditionIsNotLost(t *testing.T) {
	project := t.TempDir()
	if _, err := NotificationEvents(project, map[string]map[string]any{
		"B-001": {"file_state": "todo", "updated_at": "2026-10-05T02:00:00Z", "title": "Baseline"},
	}); err != nil {
		t.Fatal(err)
	}

	items := map[string]map[string]any{
		"B-001": {"file_state": "todo", "updated_at": "2026-10-05T02:00:00Z", "title": "Baseline"},
		"B-003": {
			"file_state": "doing", "updated_at": "2026-10-05T02:02:00Z", "title": "Approval fast path",
			"lifecycle": map[string]any{
				"created_at": "2026-10-05T02:00:30Z",
				"started_at": "2026-10-05T02:01:00Z",
			},
			"notification_condition": canonicalRuntimeConditionFixture("B-003", "run-3", "waiting_approval"),
		},
	}
	events, err := NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	kinds := []string{}
	for _, event := range events {
		if toString(event["task_id"]) == "B-003" {
			kinds = append(kinds, toString(event["kind"]))
		}
	}
	if len(kinds) != 3 || kinds[0] != "registered" || kinds[1] != "started" || kinds[2] != "approval" {
		t.Fatalf("new task current condition was lost or reordered: %v events=%+v", kinds, events)
	}
}

func TestNotificationEventsEmptyBaselinePersistsFirstRegistrationBoundary(t *testing.T) {
	project := t.TempDir()
	events, err := NotificationEvents(project, map[string]map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("empty baseline events=%+v", events)
	}

	first := map[string]map[string]any{
		"B-004": {
			"file_state": "todo", "updated_at": "2026-10-05T03:00:00Z", "title": "First real task",
			"lifecycle": map[string]any{"created_at": "2026-10-05T03:00:00Z"},
		},
	}
	events, err = NotificationEvents(project, first)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || toString(events[0]["kind"]) != "registered" || toString(events[0]["task_id"]) != "B-004" {
		t.Fatalf("first registration after observed empty baseline was lost: %+v", events)
	}
}

func TestA88Repair3ProductionJournalChild(t *testing.T) {
	project := os.Getenv("A88_REPAIR3_JOURNAL_PROJECT")
	if project == "" {
		return
	}
	worker := os.Getenv("A88_REPAIR3_JOURNAL_WORKER")
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("R3-%s-%02d", worker, i)
		if _, err := NotificationEvents(project, map[string]map[string]any{id: {"id": id, "file_state": "doing", "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}}); err != nil {
			t.Fatal("actual public writer failed", id, err)
		}
	}
}
func TestA88Repair3ProductionJournalRepeatedMultiprocess(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	for round := 0; round < 3; round++ {
		t.Run(fmt.Sprint(round), func(t *testing.T) {
			project := t.TempDir()
			if _, err := NotificationEvents(project, map[string]map[string]any{}); err != nil {
				t.Fatal(err)
			}
			sentinel, err := NotificationEvents(project, map[string]map[string]any{"OLD": {"id": "OLD", "file_state": "doing", "updated_at": "2026-11-01T00:00:00Z"}})
			if err != nil || len(sentinel) != 1 {
				t.Fatal(sentinel, err)
			}
			var wg sync.WaitGroup
			errs := make(chan error, 4)
			for worker := 0; worker < 4; worker++ {
				wg.Add(1)
				go func(worker int) {
					defer wg.Done()
					cmd := exec.Command(os.Args[0], "-test.run=^TestA88Repair3ProductionJournalChild$", "-test.count=1")
					cmd.Env = append(os.Environ(), "A88_REPAIR3_JOURNAL_PROJECT="+project, fmt.Sprintf("A88_REPAIR3_JOURNAL_WORKER=%d", worker))
					out, err := cmd.CombinedOutput()
					if err != nil {
						errs <- fmt.Errorf("child %d exit nonzero: %w %s", worker, err, out)
					} else {
						t.Logf("child %d exit0: %s", worker, out)
					}
				}(worker)
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Error(err)
			}
			events, err := ReadNotificationEvents(project)
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]int{}
			for _, event := range events {
				if event["kind"] == "registered" {
					seen[toString(event["task_id"])]++
				}
			}
			for worker := 0; worker < 4; worker++ {
				for i := 0; i < 12; i++ {
					id := fmt.Sprintf("R3-%d-%02d", worker, i)
					if seen[id] != 1 {
						t.Fatal("production history lost or duplicated", id, seen[id])
					}
				}
			}
			if len(events) != 49 || !reflect.DeepEqual(sentinel[0], events[0]) {
				t.Fatal("historical source event changed", events)
			}
			t.Log("48/48 production registrations + immutable sentinel preserved")
		})
	}
}
func TestA88Repair3ProductionJournalAtomicFaultAndCache(t *testing.T) {
	project := t.TempDir()
	if _, err := NotificationEvents(project, map[string]map[string]any{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "_task_mecca/.runtime/notification_events.json")
	before, _ := os.ReadFile(path)
	oldSync := humanStateFileSync
	defer func() { humanStateFileSync = oldSync }()
	humanStateFileSync = func(*os.File) error { return errors.New("fixture precommit sync failure") }
	items := map[string]map[string]any{"NEW": {"id": "NEW", "file_state": "doing", "updated_at": "2026-11-19T00:00:00Z"}}
	if events, err := NotificationEvents(project, items); err == nil || len(events) != 0 {
		t.Fatal("failed durable commit returned action", events, err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("precommit fault changed original journal bytes")
	}
	humanStateFileSync = oldSync
	events, err := NotificationEvents(project, items)
	if err != nil || len(events) != 1 {
		t.Fatal("failed transaction leaked cache observation", events, err)
	}
	again, err := NotificationEvents(project, items)
	if err != nil || len(again) != 1 || !reflect.DeepEqual(events, again) {
		t.Fatal("retry lost immutable identity", again, err)
	}
	// A same-size, same-inode, restored-mtime external edit must invalidate cache.
	raw, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	changed := append([]byte(nil), raw...)
	for i := range changed {
		if changed[i] == '{' {
			changed[i] = '!'
			break
		}
	}
	if err = os.WriteFile(path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if events, err := NotificationEvents(project, items); err == nil || len(events) != 0 {
		t.Fatal("external malformed bytes hidden by cache", events, err)
	}
	preserved, _ := os.ReadFile(path)
	if string(preserved) != string(changed) {
		t.Fatal("malformed original journal rewritten")
	}
}

func TestA88Repair6PostCommitPublicationMatrix(t *testing.T) {
	for _, kind := range []string{"unchanged", "corrupt", "whitespace_only", "truncated", "formatted_valid", "valid_atomic_replace", "same_inode_corrupt_restored_metadata", "same_inode_valid_restored_metadata"} {
		t.Run(kind, func(t *testing.T) {
			p := t.TempDir()
			t.Setenv("TASK_MECCA_HOME", t.TempDir())
			if _, err := NotificationEvents(p, map[string]map[string]any{}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(p, "_task_mecca/.runtime/notification_events.json")
			syncDir := humanStateDirectorySync
			defer func() { humanStateDirectorySync = syncDir }()
			injected := false
			var expectedRaw []byte
			humanStateDirectorySync = func(dir *os.File) error {
				if err := syncDir(dir); err != nil {
					return err
				}
				if injected {
					return nil
				}
				injected = true
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				info, err := os.Stat(path)
				if err != nil {
					return err
				}
				switch kind {
				case "unchanged":
					expectedRaw = raw
					return nil
				case "corrupt":
					raw = []byte("{external-corruption")
				case "whitespace_only":
					raw = []byte(strings.Repeat(" ", len(raw)))
				case "truncated":
					raw = raw[:len(raw)/2]
				case "formatted_valid":
					raw = append(append([]byte(" \n"), raw...), []byte("\n \t")...)
				case "valid_atomic_replace":
					var journal notificationJournal
					if err = json.Unmarshal(raw, &journal); err != nil {
						return err
					}
					journal.Events = []map[string]any{{"id": "external-only", "task_id": "EXTERNAL", "kind": "registered"}}
					raw, err = json.Marshal(journal)
					if err != nil {
						return err
					}
				case "same_inode_corrupt_restored_metadata":
					raw = append([]byte(nil), raw...)
					raw[0] = '!'
				case "same_inode_valid_restored_metadata":
					raw = []byte(strings.ReplaceAll(string(raw), "A-88", "B-88"))
				}
				expectedRaw = raw
				if kind == "valid_atomic_replace" {
					temp := path + ".external"
					if err = os.WriteFile(temp, raw, 0600); err != nil {
						return err
					}
					return os.Rename(temp, path)
				}
				if err = os.WriteFile(path, raw, 0600); err != nil {
					return err
				}
				if strings.HasPrefix(kind, "same_inode_") {
					after, err := os.Stat(path)
					if err != nil {
						return err
					}
					if !os.SameFile(info, after) || info.Size() != after.Size() {
						return fmt.Errorf("fixture did not preserve inode/size")
					}
					return os.Chtimes(path, info.ModTime(), info.ModTime())
				}
				return nil
			}
			condition := canonicalTaskRequestFixture("A-88", "Repository access required", "Owner: grant repository access", nil)
			if !HumanActionCondition(condition) {
				t.Fatal("genuine action fixture")
			}
			items := map[string]map[string]any{"A-88": {"id": "A-88", "file_state": "hold", "updated_at": "2026-11-19T00:00:00Z", "notification_condition": condition}}
			events, writeErr := NotificationEvents(p, items)
			humanStateDirectorySync = syncDir
			if !injected {
				t.Fatal("publication edit was not injected")
			}
			if kind == "unchanged" {
				if writeErr != nil || len(events) != 2 {
					t.Fatal("positive write failed", events, writeErr)
				}
				again, err := NotificationEvents(p, items)
				if err != nil || !reflect.DeepEqual(events, again) {
					t.Fatal("unchanged commit lost IDs/history", again, err)
				}
			} else {
				if writeErr == nil || len(events) != 0 {
					t.Fatal("unexpected bytes returned old events/action", events, writeErr)
				}
				notificationJournalMu.Lock()
				_, published := notificationJournalCache[path]
				notificationJournalMu.Unlock()
				if published {
					t.Fatal("mismatched bytes published a cache entry")
				}
			}
			raw, err := os.ReadFile(path)
			if err != nil || string(raw) != string(expectedRaw) {
				t.Fatal("external bytes overwritten", err)
			}
			var disk notificationJournal
			parseErr := json.Unmarshal(raw, &disk)
			for _, mode := range []string{"hot", "cold"} {
				if mode == "cold" {
					notificationJournalMu.Lock()
					delete(notificationJournalCache, path)
					notificationJournalMu.Unlock()
				}
				got, err := ReadNotificationEvents(p)
				if parseErr != nil {
					if err == nil || len(got) != 0 {
						t.Fatal("malformed disk hidden by cache", mode, got, err)
					}
				} else if err != nil || !reflect.DeepEqual(got, disk.Events) {
					t.Fatal("public read differs from current disk", mode, got, disk.Events, err)
				}
				after, _ := os.ReadFile(path)
				if string(after) != string(raw) {
					t.Fatal("read rewrote external bytes")
				}
			}
		})
	}
}

func TestA88Repair6EmptyBaselineRoundTripsAcrossColdRead(t *testing.T) {
	p := t.TempDir()
	if _, err := NotificationEvents(p, map[string]map[string]any{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(p, "_task_mecca/.runtime/notification_events.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var journal notificationJournal
	if err = json.Unmarshal(raw, &journal); err != nil || journal.Items == nil || journal.Phases == nil {
		t.Fatal("serialized baseline lost initialization", string(raw), err)
	}
	notificationJournalMu.Lock()
	delete(notificationJournalCache, path)
	notificationJournalMu.Unlock()
	if got, err := ReadNotificationEvents(p); err != nil || len(got) != 0 {
		t.Fatal("cold empty history changed", got, err)
	}
	first := map[string]map[string]any{"FIRST": {"id": "FIRST", "file_state": "todo", "updated_at": "2026-11-19T00:00:00Z"}}
	events, err := NotificationEvents(p, first)
	if err != nil || len(events) != 1 || events[0]["kind"] != "registered" {
		t.Fatal("cold baseline suppressed first registration", events, err)
	}
	again, err := NotificationEvents(p, first)
	if err != nil || !reflect.DeepEqual(events, again) {
		t.Fatal("first committed identity/history changed", again, err)
	}
}
