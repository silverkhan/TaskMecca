package backlog

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestA88RepairConcreteCauseRequiresRequestAndCorrespondingAction(t *testing.T) {
	row := Record{ID: "A-88", State: "hold", Location: "active", Fields: map[string]string{"대기유형": "user"}}
	next := "Check Worker status and decide whether to reassign or complete."
	for _, request := range []string{"No signal from Worker for 30 minutes.", "Runtime observation is unknown; worker may be missing.", "실행 관측 공백이 30분 지속되었습니다.", "The collector has not received any events for two hours."} {
		row.Fields["대기"], row.Fields["재개조건"] = request, next
		_, condition := canonicalOperationalState(row, nil, map[string]any{"runtime_state": "running", "human_action_runtime_verified": true})
		if len(condition) != 0 {
			t.Fatalf("label or running state manufactured cause: %+v", condition)
		}
		disguised := map[string]any{"kind": "intervention", "reason_type": "user_intervention", "human_action_verified": true, "message": request, "resume_condition": next, "human_action_basis": map[string]any{"source": "canonical_requested_action", "category": "grant_access"}}
		if HumanActionCondition(disguised) {
			t.Fatal("copied proof accepted generic telemetry request", disguised)
		}
		_, controller := canonicalOperationalState(Record{ID: "A-88", State: "doing", Location: "active"}, nil, map[string]any{"completion_review": map[string]any{"state": "needs_user", "diagnostic": request}})
		if len(controller) != 0 {
			t.Fatal("controller needs_user label manufactured cause", controller)
		}
	}
	// A topic word plus a proposed action is still not evidence of a blocker.
	row.Fields["대기"], row.Fields["재개조건"] = "Repository access observation is unknown; Worker may be missing.", "Owner grants repository access"
	_, unknownTopic := canonicalOperationalState(row, nil, nil)
	if len(unknownTopic) != 0 {
		t.Fatal("unknown access observation manufactured an access requirement", unknownTopic)
	}
	for _, pair := range [][2]string{
		{"Repository administrator access required", "Owner grants repository access"},
		{"Billing administrator approval needed", "Owner approves the billing decision"},
		{"User answer to the deployment question is required", "User: answer the deployment question"},
		{"사용자 답변 대기", "사용자: 요청된 승인 여부를 답변해 주세요."},
	} {
		row.Fields["대기"], row.Fields["재개조건"] = pair[0], pair[1]
		_, condition := canonicalOperationalState(row, nil, nil)
		if !HumanActionCondition(condition) || condition["human_action_basis"] == nil {
			t.Fatal("concrete request/action lost", pair, condition)
		}
	}
}

func TestA88RepairControllerWaitNeedsCorrelatedProjectionEvidence(t *testing.T) {
	now := time.Now().UTC()
	markdown := "# A-88\n## 작업 정의\n### 목표\nPreserve real Controller waits.\n### 수용 기준\n- [ ] Exact current wait evidence.\n"
	review := map[string]any{"state": "needs_user", "reason": "exact_controller_claim_running", "observation_quality": "observed", "since": now.Add(-time.Minute).Format(time.RFC3339Nano), "handoff_id": "exact-claimed-handoff", "contract_sha256": completionContract(markdown), "diagnostic": "Controller가 명시적으로 사용자 판단 또는 승인을 기다리고 있습니다."}
	row := Record{ID: "A-88", State: "doing", Location: "active", RawMarkdown: markdown}
	_, condition := canonicalOperationalState(row, nil, map[string]any{"completion_review": review}, now)
	if !HumanActionCondition(condition) {
		t.Fatal("observed exact Controller wait lost", condition)
	}
	for _, bad := range []string{"quality", "handoff", "future", "label_only", "foreign_contract"} {
		copy := map[string]any{}
		for k, v := range review {
			copy[k] = v
		}
		switch bad {
		case "foreign_contract":
			copy["contract_sha256"] = "foreign-contract"
		case "quality":
			copy["observation_quality"] = "insufficient"
		case "handoff":
			copy["handoff_id"] = ""
		case "future":
			copy["since"] = now.Add(time.Hour).Format(time.RFC3339Nano)
		case "label_only":
			copy["reason"] = "handoff_on_user_hold"
		}
		_, condition := canonicalOperationalState(row, nil, map[string]any{"completion_review": copy}, now)
		if len(condition) != 0 {
			t.Fatal("unverified Controller wait escaped", bad, condition)
		}
	}
}

func TestA88ObservationGapNeverRequestsHumanAction(t *testing.T) {
	row := Record{ID: "B-648", State: "doing", Location: "active"}
	for _, health := range []string{"stale", "worker_missing", "runtime_unknown"} {
		t.Run(health, func(t *testing.T) {
			reason, condition := canonicalOperationalState(row, nil, map[string]any{"health": health, "runtime_state": "running", "attempt_id": "run-570cc63aa229cbaf"})
			if len(condition) != 0 {
				t.Fatalf("observation gap became external human action: %+v", condition)
			}
			if reason["type"] == "runtime_stalled" {
				t.Fatalf("gap asserted a stopped/missing Worker: %+v", reason)
			}
		})
	}
}

func TestA88ExactActivityAttributionAdversaries(t *testing.T) {
	now := time.Date(2026, 11, 18, 9, 30, 1, 0, time.UTC)
	a := runtimeobs.Attempt{AttemptID: "run-570cc63aa229cbaf", TaskID: "B-648", AgentPath: "/root/controller/pikachyu", SessionID: "native", RuntimeAgentID: "native", BindingState: runtimeobs.BindingBound, BindingAt: now.Add(-30 * time.Minute).Format(time.RFC3339Nano), StateEvidenceSource: runtimeobs.EvidenceHook, StateObservationQuality: runtimeobs.QualityObserved, BindingEvidence: map[string]string{"assignment_id": "assignment"}}
	event := activity.Event{EventID: "test-start", TaskID: a.TaskID, AgentID: a.SessionID, RuntimeAttemptID: a.AttemptID, AssignmentID: "assignment", Role: activity.RoleWorker, Type: activity.TargetedTest, Action: activity.Switch, OccurredAt: now.Add(-8 * time.Second), RecordedAt: now.Add(-8 * time.Second)}
	for _, tc := range []struct {
		name   string
		mutate func(*activity.Event)
		want   bool
	}{
		{"B648_same_test", func(*activity.Event) {}, true},
		{"reviewer", func(e *activity.Event) { e.AgentID = "reviewer" }, false},
		{"other_task", func(e *activity.Event) { e.TaskID = "A-88" }, false},
		{"previous_attempt", func(e *activity.Event) { e.RuntimeAttemptID = "old" }, false},
		{"previous_assignment", func(e *activity.Event) { e.AssignmentID = "old" }, false},
		{"controller", func(e *activity.Event) { e.Role = activity.RoleController }, false},
		{"foreign_session", func(e *activity.Event) { e.SessionID = "other" }, false},
		{"future_occurrence", func(e *activity.Event) { e.OccurredAt = now.Add(time.Second) }, false},
		{"future_record", func(e *activity.Event) { e.RecordedAt = now.Add(time.Second) }, false},
		{"old_report", func(e *activity.Event) { e.OccurredAt = now.Add(-3 * time.Minute) }, false},
		{"pre_dispatch", func(e *activity.Event) { e.OccurredAt = now.Add(-time.Hour) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := event
			tc.mutate(&e)
			got := currentActivityEvidence(a, []activity.Event{e}, now)
			if (got != nil) != tc.want {
				t.Fatalf("report=%+v", got)
			}
		})
	}
	a.BindingState = runtimeobs.BindingAmbiguous
	if got := currentActivityEvidence(a, []activity.Event{event}, now); got != nil {
		t.Fatalf("ambiguous binding used: %+v", got)
	}
}

func TestA88GenuineStatesRequireCurrentRuntimeAndGapCannotBeRelabeled(t *testing.T) {
	row := Record{ID: "A-88", State: "doing", Location: "active"}
	for _, tc := range []struct{ health, state, kind string }{
		{"needs_user", "waiting_user", "intervention"},
		{"needs_user", "waiting_approval", "approval"},
		{"execution_interrupted", "errored", "interrupted"},
		{"execution_interrupted", "interrupted", "interrupted"},
		{"progress_collection_gap", "completed", ""},
		{"active", "running", ""},
		{"implementation_owner_unverified", "runtime_unknown", ""},
	} {
		for _, verified := range []bool{false, true} {
			t.Run(tc.state+"/"+map[bool]string{true: "verified", false: "unverified"}[verified], func(t *testing.T) {
				_, condition := canonicalOperationalState(row, nil, map[string]any{"health": tc.health, "runtime_state": tc.state, "human_action_runtime_verified": verified, "assignment_id": "assignment", "attempt_id": "attempt"})
				want := tc.kind
				if !verified {
					want = ""
				}
				if toString(condition["kind"]) != want {
					t.Fatalf("condition=%+v want %s", condition, want)
				}
			})
		}
	}
	disguised := map[string]any{"kind": "intervention", "reason_type": "runtime_unknown", "human_action_verified": true, "message": "no signal", "resume_condition": "check worker"}
	if HumanActionCondition(disguised) {
		t.Fatal("renamed observation gap authorized")
	}
	disguised["reason_type"] = "user_intervention"
	if HumanActionCondition(disguised) {
		t.Fatal("generic no-signal was renamed to a user reason")
	}
}

func TestA88CauseEpisodesAndHistorySurviveRestart(t *testing.T) {
	project := t.TempDir()
	old := map[string]any{"id": "original-sent", "task_id": "B-648", "kind": "stalled", "message": "original evidence"}
	dir := filepath.Join(project, "_task_mecca", ".runtime")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(notificationJournal{Version: notificationJournalVersion, Items: map[string]notificationObservation{"A-88": {FileState: "hold", ConditionConsumed: true}}, Phases: map[string]int{"A-88": 10}, Events: []map[string]any{old}})
	if err := os.WriteFile(filepath.Join(dir, "notification_events.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	row := Record{ID: "A-88", State: "hold", Location: "active", Fields: map[string]string{"대기유형": "user", "대기": "Grant owner access", "재개조건": "Owner: approve access"}}
	condition := func(assignment string) map[string]any {
		_, c := canonicalOperationalState(row, nil, map[string]any{"assignment_id": assignment})
		return c
	}
	items := map[string]map[string]any{"A-88": {"file_state": "hold", "notification_condition": condition("first")}}
	get := func() []map[string]any {
		events, err := NotificationEvents(project, items)
		if err != nil {
			t.Fatal(err)
		}
		return events
	}
	events := get()
	first := events[len(events)-1]["id"]
	if len(events) != 2 {
		t.Fatalf("first genuine cause not emitted: %+v", events)
	}
	notificationJournalMu.Lock()
	delete(notificationJournalCache, filepath.Join(dir, "notification_events.json"))
	notificationJournalMu.Unlock()
	if len(get()) != 2 {
		t.Fatal("restart duplicated unresolved cause")
	}
	items["A-88"]["notification_condition"] = nil
	get()
	items["A-88"]["notification_condition"] = condition("first")
	events = get()
	if len(events) != 3 || events[2]["id"] == first {
		t.Fatal("recovery/recur collapsed", events)
	}
	items["A-88"]["notification_condition"] = condition("second")
	events = get()
	if len(events) != 4 {
		t.Fatal("new assignment collapsed", events)
	}
	row.Fields["대기"] = "Provide a different owner decision"
	items["A-88"]["notification_condition"] = condition("second")
	events = get()
	if len(events) != 5 {
		t.Fatal("new cause collapsed", events)
	}
	if !reflect.DeepEqual(events[0], old) {
		t.Fatal("historical event changed", events[0])
	}
}

func TestA88LegacySameCauseRekeyDoesNotReplayAndHistoryIsNotTrimmed(t *testing.T) {
	project := t.TempDir()
	now := time.Now().UTC()
	_, condition := canonicalOperationalState(Record{ID: "A-88", State: "doing", Location: "active"}, nil, map[string]any{"health": "needs_user", "runtime_state": "waiting_approval", "attempt_id": "same", "assignment_id": "assignment", "human_action_runtime_verified": true})
	events := []map[string]any{}
	for i := 0; i < 260; i++ {
		events = append(events, map[string]any{"id": time.Unix(int64(i), 0).Format(time.RFC3339), "task_id": "OLD", "kind": "completed"})
	}
	old := map[string]any{"id": "old-sent", "task_id": "A-88", "kind": "approval", "attempt_id": "same", "message": condition["message"], "resume_condition": condition["resume_condition"], "reason_type": "approval_required"}
	events = append(events, old)
	dir := filepath.Join(project, "_task_mecca", ".runtime")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	journal := notificationJournal{Version: notificationJournalVersion, Phases: map[string]int{"A-88": 20}, Items: map[string]notificationObservation{"A-88": {FileState: "doing", UpdatedAt: "unchanged", ConditionKey: "runtime:approval\x00same", ConditionSince: now.Add(-time.Minute).Format(time.RFC3339Nano), ConditionConsumed: true}}, Events: events}
	body, _ := json.Marshal(journal)
	path := filepath.Join(dir, "notification_events.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := NotificationEvents(project, map[string]map[string]any{"A-88": {"file_state": "doing", "updated_at": "unchanged", "notification_condition": condition}})
	if err != nil || len(got) != len(events) || !reflect.DeepEqual(got[len(got)-1], old) {
		t.Fatal("migration replayed or trimmed history", len(got), err)
	}
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &journal); err != nil {
		t.Fatal(err)
	}
	if journal.Items["A-88"].ConditionEventID != "old-sent" || journal.Items["A-88"].ConditionKey != condition["key"] {
		t.Fatal("legacy current episode lost", journal.Items["A-88"])
	}
}

func TestA88CompletedThenResumedCauseStartsNewEpisode(t *testing.T) {
	project := t.TempDir()
	row := Record{ID: "A-88", State: "hold", Location: "active", Fields: map[string]string{"대기유형": "external", "대기": "External owner decision required", "재개조건": "Owner: provide decision"}}
	_, condition := canonicalOperationalState(row, nil, nil)
	items := map[string]map[string]any{"A-88": {"file_state": "hold"}}
	if _, err := NotificationEvents(project, items); err != nil {
		t.Fatal(err)
	}
	items["A-88"]["notification_condition"] = condition
	first, err := NotificationEvents(project, items)
	if err != nil || len(first) != 1 {
		t.Fatal(first, err)
	}
	oldID := first[0]["id"]
	items["A-88"]["file_state"] = "done"
	items["A-88"]["notification_condition"] = nil
	if _, err := NotificationEvents(project, items); err != nil {
		t.Fatal(err)
	}
	items["A-88"]["file_state"] = "hold"
	items["A-88"]["notification_condition"] = condition
	resumed, err := NotificationEvents(project, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed) != 3 || resumed[2]["kind"] != "intervention" || resumed[2]["id"] == oldID {
		t.Fatal("resume reused completed cause", resumed)
	}
}

func TestA88JournalRejectsGapConditionsWithoutRewritingHistory(t *testing.T) {
	project := t.TempDir()
	if _, err := NotificationEvents(project, map[string]map[string]any{}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"stalled", "runtime_unknown"} {
		events, err := NotificationEvents(project, map[string]map[string]any{"B-648": {"file_state": "doing", "updated_at": time.Now().Format(time.RFC3339Nano), "notification_condition": map[string]any{"kind": kind, "key": "gap:" + kind, "reason_type": "runtime_stalled"}}})
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			if event["kind"] == kind {
				t.Fatalf("gap emitted: %+v", event)
			}
		}
	}
}
