package webui

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/notify"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestA88RepairPreBacklogLegacyChronologyAndRecoveryBoundaries(t *testing.T) {
	for _, scenario := range []string{"continuous", "new_assignment", "new_cause", "clear_recur", "clear_recur_before_tracker", "complete_resume", "future_action", "before_assignment", "resolved_action", "restart"} {
		t.Run(scenario, func(t *testing.T) {
			project, rows := a88ActionFixture(t)
			now := time.Now().UTC()
			action := operationAction{ID: "immutable-pre-backlog-sent", TaskID: "A-88", Reason: "Owner access required", NextAction: "Owner: grant requested access", DetectedAt: now.Add(-30 * time.Second).Format(time.RFC3339Nano)}
			switch scenario {
			case "future_action":
				action.DetectedAt = now.Add(time.Minute).Format(time.RFC3339Nano)
			case "before_assignment":
				action.DetectedAt = now.Add(-3 * time.Minute).Format(time.RFC3339Nano)
			case "resolved_action":
				action.ResolvedAt = now.Add(-time.Second).Format(time.RFC3339Nano)
			}
			if err := writeOperationJournal(project, operationJournal{Version: 1, Project: project, Actions: []operationAction{action}}); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(project, "_task_mecca", ".runtime", "notifications")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			sent := []byte(`{"version":1,"records":{"immutable-pre-backlog-sent":{"event_id":"immutable-pre-backlog-sent","task_id":"A-88","kind":"intervention","channel":"telegram","state":"sent","attempts":1}}}`)
			ledger := filepath.Join(dir, "delivery_ledger.json")
			if err := os.WriteFile(ledger, sent, 0600); err != nil {
				t.Fatal(err)
			}
			journalBefore, _ := os.ReadFile(operationJournalPath(project))
			path := filepath.Join(project, "_task_mecca", "data", "backlog", "000001.A-88.fixture.hold.md")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "clear_recur_before_tracker" {
				_ = currentBacklogDeliveryEvents(project, "", rows)
			}
			switch scenario {
			case "new_assignment":
				if _, err := runtimeobs.RecordAssignment(project, "repair-new-assignment", "A-88", "/root/controller/fixture", time.Now().UTC()); err != nil {
					t.Fatal(err)
				}
			case "new_cause":
				changed := strings.ReplaceAll(string(original), "Owner access required", "Database permission required")
				changed = strings.ReplaceAll(changed, "Owner: grant requested access", "Owner: grant database permission")
				if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
					t.Fatal(err)
				}
			case "clear_recur", "clear_recur_before_tracker":
				if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(original), "- 대기유형: user", "- 대기유형: -")), 0600); err != nil {
					t.Fatal(err)
				}
				if scenario == "clear_recur" {
					_ = currentBacklogDeliveryEvents(project, "", rows)
				} else {
					if _, err := backlog.AttentionSnapshot(project, "", true); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(path, original, 0600); err != nil {
					t.Fatal(err)
				}
			case "complete_resume":
				done := strings.ReplaceAll(path, ".hold.md", ".done.md")
				if err := os.Rename(path, done); err != nil {
					t.Fatal(err)
				}
				_ = currentBacklogDeliveryEvents(project, "", rows)
				if err := os.Rename(done, path); err != nil {
					t.Fatal(err)
				}
			}
			snapshot, err := backlog.AttentionSnapshot(project, "", true)
			if err != nil {
				t.Fatal(err)
			}
			rows = a88OnlyActions(snapshot["notification_events"].([]map[string]any))
			got := currentBacklogDeliveryEvents(project, "", rows)
			if len(got) != 1 {
				t.Fatal("current genuine cause lost", scenario, got)
			}
			inherit := scenario == "continuous" || scenario == "restart"
			if (got[0].ID == action.ID) != inherit {
				t.Fatal("incorrect legacy episode inheritance", scenario, got)
			}
			if scenario == "restart" {
				again := currentBacklogDeliveryEvents(project, "", rows)
				if len(again) != 1 || again[0].ID != action.ID {
					t.Fatal("persisted continuity lost", again)
				}
			}
			journalAfter, _ := os.ReadFile(operationJournalPath(project))
			sentAfter, _ := os.ReadFile(ledger)
			if string(journalBefore) != string(journalAfter) || string(sent) != string(sentAfter) {
				t.Fatal("source or sent journal bytes changed", scenario)
			}
		})
	}
}

func TestA88RepairCollectionFailureDoesNotInventRecoveryOrNewIdentity(t *testing.T) {
	project, rows := a88ActionFixture(t)
	first := currentBacklogDeliveryEvents(project, "", rows)
	if len(first) != 1 {
		t.Fatal(first)
	}
	path := filepath.Join(project, "_task_mecca", "data", "backlog", "000001.A-88.fixture.hold.md")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Unknown implementation identity cannot clear an independently real cause.
	foreign := strings.ReplaceAll(string(original), "fixture-native-A-88", "unobserved-native")
	if err := os.WriteFile(path, []byte(foreign), 0600); err != nil {
		t.Fatal(err)
	}
	if got := currentBacklogDeliveryEvents(project, "", rows); len(got) != 0 {
		t.Fatal("unverified identity created new external cause", got)
	}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := backlog.AttentionSnapshot(project, "", true)
	if err != nil {
		t.Fatal(err)
	}
	currentRows := a88OnlyActions(snapshot["notification_events"].([]map[string]any))
	for i := 0; i < 2; i++ {
		restored := currentBacklogDeliveryEvents(project, "", currentRows)
		if len(restored) != 1 || restored[0].ID != first[0].ID {
			t.Fatal("collection outage fabricated recovery or replay ID", restored, first)
		}
	}
}

func a88ActionFixture(t *testing.T) (string, []map[string]any) {
	t.Helper()
	project := testOperationProject(t)
	if _, err := backlog.NotificationEvents(project, map[string]map[string]any{}); err != nil {
		t.Fatal(err)
	}
	a83TypedActionFixture(t, project, "A-88", "Owner access required", "Owner: grant requested access", runtimeobs.StateRunning)
	installA83MockCurrentExecution(t)
	snapshot, err := backlog.AttentionSnapshot(project, "", true)
	if err != nil {
		t.Fatal(err)
	}
	return project, a88OnlyActions(snapshot["notification_events"].([]map[string]any))
}

func TestA88CrossPathRealMockTransportDeduplicatesAndPreservesSent(t *testing.T) {
	project, rows := a88ActionFixture(t)
	first := currentBacklogDeliveryEvents(project, "", rows)
	if len(first) != 1 || first[0].Kind != "intervention" {
		t.Fatal("real cause lost", first)
	}
	operation := currentOperationDeliveryEvents(project, []notify.Event{{ID: "independent-monitor-id", TaskID: "A-88", Kind: "approval", Message: first[0].Message, ResumeCondition: first[0].ResumeCondition, At: first[0].At}})
	if len(operation) != 1 || first[0].ID != operation[0].ID {
		t.Fatal("paths lack shared cause ID", first, operation)
	}
	dir := filepath.Join(project, "_task_mecca", ".runtime", "notifications")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]any{"enabled": true, "token": "fixture-only", "chat_id": 7, "activated_at": time.Now().Add(-time.Hour).Format(time.RFC3339Nano), "kinds": map[string]bool{"intervention": true}}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(dir, "telegram.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	defer func() { http.DefaultTransport = original }()
	calls := 0
	http.DefaultTransport = lifecycleFakeTelegramTransport{fallback: original, telegram: func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1}}`))}, nil
	}}
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "")
	if errs := notify.Deliver(project, first); len(errs) > 0 {
		t.Fatal(errs)
	}
	before, err := notify.DeliveryRecords(project, "A-88", first[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].State != "sent" {
		t.Fatal("mock not sent", before)
	}
	if errs := notify.Deliver(project, operation); len(errs) > 0 {
		t.Fatal(errs)
	}
	after, err := notify.DeliveryRecords(project, "A-88", first[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !reflect.DeepEqual(before, after) {
		t.Fatal("cross-path duplicated or sent history changed", calls, before, after)
	}
	// Restart/current revalidation uses the same episode, not a new ID.
	again := currentBacklogDeliveryEvents(project, "", rows)
	if len(again) != 1 || again[0].ID != first[0].ID {
		t.Fatal("restart changed episode", again)
	}
}

func TestA88PendingAndBrowserClaimsRevalidateCurrentCause(t *testing.T) {
	project, rows := a88ActionFixture(t)
	webDeliveryTestCutover(t, project)
	if len(rows) != 1 {
		t.Fatal(rows)
	}
	id := notificationValue(rows[0]["id"])
	claim, err := applyWebDelivery(project, "claim", id, "", "chrome")
	if err != nil || !claim.Granted {
		t.Fatal("current action denied", claim, err)
	}
	if _, err := applyWebDelivery(project, "fail", id, claim.Token, "chrome"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "_task_mecca", "data", "backlog", "000001.A-88.fixture.hold.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(body), "Owner access required", "Different human cause")), 0600); err != nil {
		t.Fatal(err)
	}
	if got := currentBacklogDeliveryEvents(project, "", rows); len(got) != 0 {
		t.Fatal("queued obsolete cause delivered", got)
	}
	retry, err := applyWebDelivery(project, "claim", id, "", "push")
	if err != nil || retry.Granted || retry.State != "condition_resolved" {
		t.Fatal("pending push retry bypassed gate", retry, err)
	}
	// Already sent browser evidence remains immutable even after resolution.
	ledger, _, err := readWebDelivery(project)
	if err != nil {
		t.Fatal(err)
	}
	entry := ledger.Entries[id]
	entry.State = "display_requested"
	entry.SentAt = time.Now().Format(time.RFC3339Nano)
	ledger.Entries[id] = entry
	if err := saveWebDelivery(project, ledger); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(webDeliveryPath(project))
	decision, err := applyWebDelivery(project, "claim", id, "", "chrome")
	if err != nil || decision.State != "display_requested" {
		t.Fatal(decision, err)
	}
	after, _ := os.ReadFile(webDeliveryPath(project))
	if string(before) != string(after) {
		t.Fatal("sent web evidence rewritten")
	}
}

func TestA88HistoricalStalePendingNeverReachesEitherSurface(t *testing.T) {
	project := testOperationProject(t)
	gap := map[string]any{"id": "old-pending", "task_id": "B-648", "kind": "stalled", "at": time.Now().Format(time.RFC3339Nano), "message": "runtime stopped", "reason_type": "runtime_stalled"}
	folder := filepath.Join(project, "_task_mecca", ".runtime")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"version": 4, "items": map[string]any{}, "phases": map[string]int{}, "events": []map[string]any{gap}})
	if err := os.WriteFile(filepath.Join(folder, "notification_events.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	webDeliveryTestCutover(t, project)
	decision, err := applyWebDelivery(project, "claim", "old-pending", "", "push")
	if err != nil || decision.Granted {
		t.Fatal("stale push claim", decision, err)
	}
	if events := currentBacklogDeliveryEvents(project, "", []map[string]any{gap}); len(events) != 0 {
		t.Fatal("stale Telegram retry", events)
	}
	retained, err := backlog.ReadNotificationEvents(project)
	if err != nil || len(retained) != 1 || !reflect.DeepEqual(retained[0], gap) {
		t.Fatal("historical evidence changed", retained, err)
	}
}

func TestA88RuntimeActionRequiresFreshOfficialExactIdentity(t *testing.T) {
	for _, state := range []runtimeobs.CanonicalState{runtimeobs.StateWaitingApproval, runtimeobs.StateWaitingUser, runtimeobs.StateErrored, runtimeobs.StateInterrupted, runtimeobs.StateShutdown} {
		t.Run(string(state), func(t *testing.T) {
			project := testOperationProject(t)
			if _, err := backlog.NotificationEvents(project, map[string]map[string]any{}); err != nil {
				t.Fatal(err)
			}
			a83TypedActionFixture(t, project, "A-88", "Owner access required", "Owner: grant requested access", state)
			installA83MockCurrentExecution(t)
			path := filepath.Join(project, "_task_mecca", "data", "backlog", "000001.A-88.fixture.hold.md")
			raw, _ := os.ReadFile(path)
			raw = []byte(strings.ReplaceAll(string(raw), "- 대기유형: user", "- 대기유형: -"))
			if err := os.WriteFile(strings.ReplaceAll(path, ".hold.md", ".doing.md"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			} // fixture only
			snapshot, err := backlog.AttentionSnapshot(project, "", true)
			if err != nil {
				t.Fatal(err)
			}
			rows := a88OnlyActions(snapshot["notification_events"].([]map[string]any))
			positive := currentBacklogDeliveryEvents(project, "", rows)
			if len(positive) != 1 {
				t.Fatal("genuine runtime action lost", state, rows, positive)
			}
			base := operationExecutionLoader
			for _, bad := range []string{"unfresh", "wrong_attempt", "wrong_assignment", "wrong_native", "future", "resumed", "completed", "canonical_completed_during_official_read"} {
				operationExecutionLoader = func(project string, ledger runtimeobs.Ledger, now time.Time, refresh bool) map[string]runtimeobs.CurrentExecutionSnapshot {
					result := base(project, ledger, now, refresh)
					s := result["A-88"]
					switch bad {
					case "unfresh":
						s.OfficialCurrent = false
					case "wrong_attempt":
						s.AttemptID = "other"
					case "wrong_assignment":
						s.Identity.AssignmentID = "other"
					case "wrong_native":
						s.Identity.NativeID = "reviewer"
					case "future":
						s.ObservedAt = now.Add(time.Hour).Format(time.RFC3339Nano)
					case "resumed":
						s.State = runtimeobs.StateRunning
					case "completed":
						s.State = runtimeobs.StateCompleted
					case "canonical_completed_during_official_read":
						doing := strings.ReplaceAll(path, ".hold.md", ".doing.md")
						if err := os.Rename(doing, strings.ReplaceAll(path, ".hold.md", ".done.md")); err != nil {
							t.Fatal(err)
						}
					}
					result["A-88"] = s
					return result
				}
				if got := currentBacklogDeliveryEvents(project, "", rows); len(got) != 0 {
					t.Fatal("invalid official observation sent", bad, got)
				}
			}
			operationExecutionLoader = base
		})
	}
}

func a88OnlyActions(rows []map[string]any) []map[string]any {
	actions := []map[string]any{}
	for _, row := range rows {
		if backlog.HumanActionKind(notificationValue(row["kind"])) {
			actions = append(actions, row)
		}
	}
	return actions
}

func TestA88ExistingMonitorSentCauseAliasesWithoutChangingItsJournal(t *testing.T) {
	project, rows := a88ActionFixture(t)
	now := time.Now().UTC()
	action := operationAction{ID: "legacy-monitor-sent", TaskID: "A-88", Reason: "Owner access required", NextAction: "Owner: grant requested access", DetectedAt: now.Format(time.RFC3339Nano)}
	journal := operationJournal{Project: project, Actions: []operationAction{action}}
	if err := writeOperationJournal(project, journal); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(project, "_task_mecca", ".runtime", "notifications")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"version":1,"records":{"legacy-monitor-sent":{"event_id":"legacy-monitor-sent","task_id":"A-88","kind":"intervention","channel":"telegram","state":"sent","attempts":1}}}`)
	ledger := filepath.Join(dir, "delivery_ledger.json")
	if err := os.WriteFile(ledger, original, 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(operationJournalPath(project))
	outbound := currentBacklogDeliveryEvents(project, "", rows)
	if len(outbound) != 1 || outbound[0].ID != action.ID {
		decisions, _ := currentNotificationDecisions(project, "")
		records, _ := notify.DeliveryRecords(project, "A-88", action.ID)
		t.Fatal("upgrade gave sent cause a new delivery ID", outbound, decisions["A-88"].Condition, decisions["A-88"].Since, action, records)
	}
	after, _ := os.ReadFile(operationJournalPath(project))
	sent, _ := os.ReadFile(ledger)
	if string(before) != string(after) || string(original) != string(sent) {
		t.Fatal("migration rewrote immutable action or sent journal")
	}
	// A later assignment must not inherit the preceding recipient delivery.
	if _, err := runtimeobs.RecordAssignment(project, "new-owner-assignment", "A-88", "/root/controller/fixture", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	decisions, err := currentNotificationDecisions(project, "")
	if err != nil {
		t.Fatal(err)
	}
	if decisions["A-88"].EventID == action.ID {
		t.Fatal("new assignment reused prior sent episode")
	}
}
