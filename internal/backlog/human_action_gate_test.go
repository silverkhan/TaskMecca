package backlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/projectguard"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
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
	row := Record{ID: "A-88", State: "hold", Location: "active", Fields: map[string]string{"대기유형": "user", "대기": "Grant owner access", "재개조건": "Owner: grant access"}}
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
	row.Fields["대기"] = "Billing approval is required."
	row.Fields["재개조건"] = "Owner: approve billing decision."
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

func canonicalRequestConditionFixture(request, next string) map[string]any {
	_, condition := canonicalOperationalState(Record{ID: "A-55", State: "hold", Location: "active", Fields: map[string]string{"대기유형": "user", "대기": request, "재개조건": next}}, nil, nil)
	return condition
}

func TestA88Repair2PresentAssertionAndUnforgeableProof(t *testing.T) {
	negative := [][2]string{
		{"Unknown whether repository access is required; no execution signal was observed.", "Owner: grant repository access if subsequently confirmed necessary."},
		{"Repository access is not required. We only need to investigate the telemetry gap.", "Owner: grant repository access."},
		{"접근 권한이 필요한지 확인할 수 없습니다. 실행 관측 공백만 확인되었습니다.", "관리자: 저장소 접근 권한을 부여해 주세요."},
		{"Repository access is required next month.", "Owner: grant repository access."},
		{"Repository access is required.", "Owner: grant repository access next month."},
		{"Repository access is required, if deployment proceeds.", "Owner: grant repository access."},
		{"Repository access is required.", "Owner: grant repository access if necessary."},
		{"저장소 접근 권한이 필요하지 않습니다.", "관리자: 저장소 접근 권한을 부여해 주세요."},
		{"다음 달 저장소 접근 권한이 필요합니다.", "관리자: 저장소 접근 권한을 부여해 주세요."},
		{"저장소 접근 권한이 필요합니다.", "관리자: 필요하면 저장소 접근 권한을 부여해 주세요."},
		{"Repository access is required. But it is only a telemetry hypothesis.", "Owner: grant repository access."},
	}
	for _, pair := range negative {
		if requestedHumanAction(pair[0], pair[1]) != nil {
			t.Fatal("unsupported prose minted proof", pair)
		}
	}
	for _, pair := range [][2]string{{"Repository access is required to continue deployment.", "Owner: grant repository access."}, {"배포를 계속하려면 저장소 접근 권한 부여가 필요합니다.", "관리자: 저장소 접근 권한을 부여해 주세요."}, {"Billing administrator approval needed", "Owner approves the billing decision"}, {"User answer to the deployment question is required", "User: answer the deployment question"}} {
		c := canonicalRequestConditionFixture(pair[0], pair[1])
		if !HumanActionCondition(c) {
			t.Fatal("genuine request lost", pair, c)
		}
		raw, _ := json.Marshal(c)
		var forged map[string]any
		_ = json.Unmarshal(raw, &forged)
		if HumanActionCondition(forged) {
			t.Fatal("serialized private proof regained authority")
		}
		forged["human_action_basis"] = map[string]any{"source": "canonical_requested_action", "category": "grant_access", "request": pair[0], "next": pair[1]}
		if HumanActionCondition(forged) {
			t.Fatal("freeform forged proof gained authority")
		}
	}
}

func TestA88Repair2CanonicalTupleRejectsRecomputedCorruption(t *testing.T) {
	now := time.Now().UTC()
	since := now.Add(-time.Minute).Format(time.RFC3339Nano)
	key := "hold:user\x00repository access is required."
	b := humanActionBoundary{Key: key, Since: since, OriginSince: since, EventID: "canonical-event"}
	b.Checksum = humanBoundaryChecksum(b)
	j := notificationJournal{Items: map[string]notificationObservation{"A-88": {ConditionKey: key, ConditionSince: since, ConditionEventID: b.EventID}}, Events: []map[string]any{{"id": b.EventID, "task_id": "A-88", "kind": "intervention", "condition_key": key, "condition_since": since, "assignment_id": ""}}}
	if err := validateHumanActionBoundary(t.TempDir(), "A-88", b, j, HumanActionDecision{}, nil, nil, now); err != nil {
		t.Fatal("valid canonical tuple rejected", err)
	}
	for name, mutate := range map[string]func(*humanActionBoundary){"foreign_event": func(v *humanActionBoundary) { v.EventID = "foreign-transport" }, "foreign_assignment": func(v *humanActionBoundary) { v.Assignment = "foreign"; v.Key += "\x00foreign" }, "missing_since": func(v *humanActionBoundary) { v.Since = "" }, "shift_origin": func(v *humanActionBoundary) { v.OriginSince = now.Add(-2 * time.Minute).Format(time.RFC3339Nano) }, "foreign_cause": func(v *humanActionBoundary) { v.Key = "hold:user\x00different cause" }, "invented_floor": func(v *humanActionBoundary) { v.NotBefore = now.Add(-2 * time.Minute).Format(time.RFC3339Nano) }} {
		t.Run(name, func(t *testing.T) {
			v := b
			mutate(&v)
			v.Checksum = humanBoundaryChecksum(v)
			raw, _ := json.Marshal(v)
			path := filepath.Join(t.TempDir(), "boundary.json")
			_ = os.WriteFile(path, raw, 0600)
			if err := validateHumanActionBoundary(t.TempDir(), "A-88", v, j, HumanActionDecision{}, nil, nil, now); err == nil {
				t.Fatal("canonical contradiction accepted", v)
			}
			got, _ := os.ReadFile(path)
			if string(got) != string(raw) {
				t.Fatal("corrupt raw bytes changed")
			}
		})
	}
}

func TestA88Repair2DurableAtomicBoundaryFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	old := []byte("old-complete\n")
	_ = os.WriteFile(path, old, 0600)
	foreign := filepath.Join(filepath.Dir(path), ".human-action-state-foreign.tmp")
	_ = os.WriteFile(foreign, []byte("foreign"), 0600)
	fileSync, dirSync := humanStateFileSync, humanStateDirectorySync
	defer func() { humanStateFileSync = fileSync; humanStateDirectorySync = dirSync }()
	humanStateFileSync = func(*os.File) error { return errors.New("fixture file sync failure") }
	if err := writeHumanActionState(path, []byte("new-complete\n")); err == nil {
		t.Fatal("file sync failure ignored")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(old) {
		t.Fatal("precommit failure changed raw state")
	}
	humanStateFileSync = fileSync
	humanStateDirectorySync = func(*os.File) error { return errors.New("fixture directory sync failure") }
	if err := writeHumanActionState(path, []byte("new-complete\n")); err == nil {
		t.Fatal("directory sync failure ignored")
	}
	got, _ = os.ReadFile(path)
	if string(got) != "new-complete\n" {
		t.Fatal("postcommit state incomplete")
	}
	humanStateDirectorySync = dirSync
	if err := writeHumanActionState(path, []byte("durable\n")); err != nil {
		t.Fatal(err)
	}
	temps, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".human-action-state-*.tmp"))
	if len(temps) != 1 || temps[0] != foreign {
		t.Fatal("foreign temp changed or own temp leaked", temps)
	}
}

func TestA88Repair2StateLockChild(t *testing.T) {
	project := os.Getenv("A88_LOCK_CHILD_PROJECT")
	if project == "" {
		return
	}
	for i := 0; i < 12; i++ {
		release, err := projectguard.AcquireStateLock(project, "fixture_counter")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(project, "counter")
		raw, _ := os.ReadFile(path)
		n, _ := strconv.Atoi(string(raw))
		time.Sleep(time.Millisecond)
		err = os.WriteFile(path, []byte(strconv.Itoa(n+1)), 0600)
		release()
		if err != nil {
			t.Fatal(err)
		}
	}
}
func TestA88Repair2CrossProcessExclusiveStableLock(t *testing.T) {
	project := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	release, err := projectguard.AcquireStateLock(project, "fixture_counter")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "_task_mecca/.runtime/fixture_counter.lock")
	before, _ := os.Stat(path)
	release()
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestA88Repair2StateLockChild$", "-test.count=1")
			cmd.Env = append(os.Environ(), "A88_LOCK_CHILD_PROJECT="+project)
			raw, err := cmd.CombinedOutput()
			if err != nil {
				errs <- fmt.Errorf("%w: %s", err, raw)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	raw, _ := os.ReadFile(filepath.Join(project, "counter"))
	if string(raw) != "48" {
		t.Fatal("lost concurrent update", string(raw))
	}
	after, _ := os.Stat(path)
	if !os.SameFile(before, after) {
		t.Fatal("lock inode replaced")
	}
	for _, name := range []string{"../escape", "", "Upper", "a/b", strings.Repeat("a", 65)} {
		if release, err := projectguard.AcquireStateLock(project, name); err == nil {
			release()
			t.Fatal("invalid name accepted", name)
		}
	}
	_ = os.Symlink(path, filepath.Join(filepath.Dir(path), "symlink.lock"))
	if release, err := projectguard.AcquireStateLock(project, "symlink"); err == nil {
		release()
		t.Fatal("symlink accepted")
	}
	// Failed acquisition must release the shared management fence.
	management, err := projectguard.AcquireManagement()
	if err != nil {
		t.Fatal("error path leaked fence", err)
	}
	management()
	home := projectguard.Home()
	_ = os.MkdirAll(home, 0700)
	body, _ := json.Marshal(map[string]any{"paused_projects": []string{project}})
	_ = os.WriteFile(filepath.Join(home, "projects.json"), body, 0600)
	if release, err := projectguard.AcquireStateLock(project, "paused"); err == nil {
		release()
		t.Fatal("paused project acquired lock")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "paused.lock")); !os.IsNotExist(err) {
		t.Fatal("paused lock was created", err)
	}
	missing := filepath.Join(t.TempDir(), "removed")
	if release, err := projectguard.AcquireStateLock(missing, "removed"); err == nil {
		release()
		t.Fatal("removed project recreated")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("removed project was touched", err)
	}
}

func TestA88Repair2PersistedCorruptionFailsClosedWithoutRewrite(t *testing.T) {
	for _, field := range []string{"event_id", "assignment", "since", "key", "origin_since", "not_before"} {
		t.Run(field, func(t *testing.T) {
			project := t.TempDir()
			t.Setenv("TASK_MECCA_HOME", t.TempDir())
			dir := filepath.Join(project, "_task_mecca/data/backlog")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "000088.A-88.fixture.hold.md")
			if err := os.WriteFile(path, []byte("# A-88 fixture\n- 대기유형: user\n- 대기: Repository access is required.\n- 재개조건: Owner: grant repository access.\n"), 0600); err != nil {
				t.Fatal(err)
			}
			decisions, err := CurrentHumanActionDecisions(project, "")
			if err != nil || len(decisions) != 1 {
				t.Fatal("genuine baseline missing", decisions, err)
			}
			statePath := filepath.Join(project, "_task_mecca/.runtime/human_action_boundaries.json")
			raw, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			var state humanActionBoundaries
			if err = json.Unmarshal(raw, &state); err != nil {
				t.Fatal(err)
			}
			b := state.Items["A-88"]
			switch field {
			case "event_id":
				b.EventID = "foreign-sent-transport"
			case "assignment":
				b.Assignment = "foreign-assignment"
				b.Key += "\x00foreign-assignment"
			case "since":
				b.Since = ""
			case "key":
				b.Key = "hold:user\x00different cause"
			case "origin_since":
				b.OriginSince = time.Now().Add(-time.Hour).Format(time.RFC3339Nano)
			case "not_before":
				b.NotBefore = time.Now().Add(-time.Hour).Format(time.RFC3339Nano)
			}
			b.Checksum = humanBoundaryChecksum(b)
			state.Items["A-88"] = b
			raw, _ = json.Marshal(state)
			if err = os.WriteFile(statePath, raw, 0600); err != nil {
				t.Fatal(err)
			}
			decisions, err = CurrentHumanActionDecisions(project, "")
			if err == nil || len(decisions) != 0 {
				t.Fatal("persisted semantic corruption authorized decision", decisions, err)
			}
			after, _ := os.ReadFile(statePath)
			if string(after) != string(raw) {
				t.Fatal("semantic corruption silently rewritten")
			}
		})
	}
}

func canonicalTaskRequestFixture(task, request, next string, signal map[string]any) map[string]any {
	_, condition := canonicalOperationalState(Record{ID: task, State: "hold", Location: "active", Fields: map[string]string{"대기유형": "user", "대기": request, "재개조건": next}}, nil, signal)
	return condition
}

func TestA88Repair3ProofBindsCompleteFinalTuple(t *testing.T) {
	makeCondition := func() map[string]any {
		return canonicalTaskRequestFixture("A-88", "Repository access is required.", "Owner: grant repository access.", map[string]any{"agent_path": "/root/controller/pairi", "native_id": "native", "runtime_agent_id": "native", "attempt_id": "attempt", "assignment_id": "assignment", "assigned_at": "2026-11-19T01:00:00Z", "binding_at": "2026-11-19T01:00:01Z", "runtime_state": "running", "binding_state": "bound", "observation_quality": "observed"})
	}
	original := makeCondition()
	if !HumanActionCondition(original) {
		t.Fatal("genuine final tuple lost", original)
	}
	for field := range original {
		if field == "human_action_basis" {
			continue
		}
		t.Run(field, func(t *testing.T) {
			c := makeCondition()
			c[field] = "foreign-mutated-value"
			if HumanActionCondition(c) {
				t.Fatal("mutated authority field accepted", field)
			}
		})
	}
	for _, name := range []string{"copied_map", "different_condition", "extra_field", "private_basis", "removed_field", "wrong_task_writer"} {
		t.Run(name, func(t *testing.T) {
			c := makeCondition()
			switch name {
			case "copied_map":
				copy := map[string]any{}
				for k, v := range c {
					copy[k] = v
				}
				if HumanActionCondition(copy) {
					t.Fatal("same tuple copied proof accepted on different map")
				}
				return
			case "different_condition":
				other := makeCondition()
				other["human_action_basis"] = c["human_action_basis"]
				c = other
			case "extra_field":
				c["foreign_authority"] = "added"
			case "private_basis":
				p := *c["human_action_basis"].(*humanActionProof)
				p.category = "foreign"
				c["human_action_basis"] = &p
			case "removed_field":
				delete(c, "assigned_at")
			case "wrong_task_writer":
				project := t.TempDir()
				if _, err := NotificationEvents(project, map[string]map[string]any{}); err != nil {
					t.Fatal(err)
				}
				events, err := NotificationEvents(project, map[string]map[string]any{"FOREIGN": {"file_state": "hold", "notification_condition": c}})
				if err != nil {
					t.Fatal(err)
				}
				for _, event := range events {
					if event["kind"] == "intervention" {
						t.Fatal("proof transplanted to other task", event)
					}
				}
				return
			}
			if HumanActionCondition(c) {
				t.Fatal("transplanted or mutated proof accepted", name)
			}
		})
	}
}

func TestA88Repair3ParsedEntityObjectIntentInvariant(t *testing.T) {
	resources := []string{"repository", "database", "billing", "project", "service"}
	for _, request := range resources {
		for _, action := range resources {
			for _, intent := range []string{"access", "approval", "input"} {
				name := request + "_" + action + "_" + intent
				t.Run(name, func(t *testing.T) {
					reason := strings.Title(request) + " access is required."
					next := "Owner: grant " + action + " access."
					if intent == "approval" {
						reason = strings.Title(request) + " approval is required."
						next = "Owner: approve " + action + " decision."
					}
					if intent == "input" {
						reason = strings.Title(request) + " input is required."
						next = "User: provide " + action + " input."
					}
					got := requestedHumanAction(reason, next) != nil
					if got != (request == action) {
						t.Fatal("resource/entity correspondence violated", reason, next, got)
					}
					if requestedHumanAction(reason, "Owner: grant "+action+" access.") != nil && intent != "access" {
						t.Fatal("intent correspondence violated", intent)
					}
				})
			}
		}
	}
	for _, pair := range [][2]string{{"Repository access approval needed", "Owner: grant requested repository access"}, {"Billing approval is required.", "Owner: grant database access."}, {"Repository access is required.", "Owner: approve repository access."}, {"Repository read permission is required.", "Owner: grant repository write permission."}, {"Repository access is required.", "Owner: grant repository database access."}, {"저장소 접근 권한이 필요합니다.", "관리자: 데이터베이스 접근 권한을 부여해 주세요."}} {
		if requestedHumanAction(pair[0], pair[1]) != nil {
			t.Fatal("mismatched intent/object/entity accepted", pair)
		}
	}
}

func TestA88Repair3G11MatchingEquivalentEpisodes(t *testing.T) {
	project := t.TempDir()
	if _, err := NotificationEvents(project, map[string]map[string]any{}); err != nil {
		t.Fatal(err)
	}
	makeCondition := func(request, next, assignment string) map[string]any {
		return canonicalTaskRequestFixture("A-88", request, next, map[string]any{"assignment_id": assignment})
	}
	c := makeCondition("Repository access approval needed", "Owner: approve requested repository access", "assignment1")
	item := map[string]any{"file_state": "hold", "updated_at": "same", "notification_condition": c}
	items := map[string]map[string]any{"A-88": item}
	actions := func() []map[string]any {
		all, err := NotificationEvents(project, items)
		if err != nil {
			t.Fatal(err)
		}
		out := []map[string]any{}
		for _, e := range all {
			if e["kind"] == "intervention" {
				out = append(out, e)
			}
		}
		return out
	}
	first := actions()
	if len(first) != 1 {
		t.Fatal(first)
	}
	id := first[0]["id"]
	if len(actions()) != 1 {
		t.Fatal("repeated call duplicated")
	}
	item["notification_condition"] = nil
	actions()
	item["notification_condition"] = makeCondition("Repository access approval needed", "Owner: approve requested repository access", "assignment1")
	recur := actions()
	if len(recur) != 2 || recur[1]["id"] == id {
		t.Fatal("clear/recur collapsed", recur)
	}
	item["notification_condition"] = makeCondition("Billing administrator approval needed", "Owner: approve billing decision", "assignment1")
	changed := actions()
	if len(changed) != 3 {
		t.Fatal("new cause collapsed", changed)
	}
	item["notification_condition"] = makeCondition("Billing administrator approval needed", "Owner: approve billing decision", "assignment2")
	assigned := actions()
	if len(assigned) != 4 {
		t.Fatal("new assignment collapsed", assigned)
	}
	item["file_state"] = "done"
	item["notification_condition"] = nil
	actions()
	item["file_state"] = "hold"
	item["notification_condition"] = makeCondition("Billing administrator approval needed", "Owner: approve billing decision", "assignment2")
	resumed := actions()
	if len(resumed) != 5 || resumed[4]["id"] == assigned[3]["id"] {
		t.Fatal("complete/resume collapsed", resumed)
	}
}

func canonicalRuntimeConditionFixture(task, attempt, state string) map[string]any {
	_, condition := canonicalOperationalState(Record{ID: task, State: "doing", Location: "active"}, nil, map[string]any{"health": "active", "attempt_id": attempt, "runtime_state": state, "human_action_runtime_verified": true})
	return condition
}
