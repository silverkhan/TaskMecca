package backlog

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func reviewFixture() (Record, runtimeobs.Attempt, runtimeobs.Attempt, *completionHandoff, time.Time) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	row := Record{ID: "A-31", State: "doing", Location: "active", Fields: map[string]string{"Agent": "/root/controller/kkobugi"}, RawMarkdown: "# A-31 Review\n- Agent: /root/controller/kkobugi\n## 작업 정의\n- 목표: 검토\n## 결과\n- 대기\n"}
	worker := runtimeobs.Attempt{AttemptID: "worker-1", TaskID: row.ID, AgentPath: row.Fields["Agent"], BindingState: runtimeobs.BindingBound, StartedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), EndedAt: now.Format(time.RFC3339Nano), CurrentState: runtimeobs.StateCompleted, Terminal: true, StateEvidenceSource: runtimeobs.EvidenceHook, StateObservationQuality: runtimeobs.QualityObserved}
	controller := runtimeobs.Attempt{AttemptID: "controller-1", AgentPath: "/root/controller", RuntimeAgentID: "controller-native", SessionID: "session-1", Provider: "codex", CurrentState: runtimeobs.StateRunning, BindingState: runtimeobs.BindingBound, StateEvidenceSource: runtimeobs.EvidenceHook, StateObservationQuality: runtimeobs.QualityObserved}
	h := &completionHandoff{prepared: completionHandoffRecord{HandoffID: "handoff-1", Kind: "prepared", Event: "worker_done", TaskID: row.ID, Contract: completionContract(row.RawMarkdown), SourceAgent: worker.AgentPath, SourceAttempt: worker.AttemptID, TargetAgent: controller.AgentPath, TargetAttempt: controller.AttemptID, TargetRuntimeID: controller.RuntimeAgentID, TargetSession: controller.SessionID, Provider: controller.Provider, At: now.Add(time.Second).Format(time.RFC3339Nano)}}
	return row, worker, controller, h, now
}

func TestCompletionReviewPhaseBoundariesDoNotMoveWithPolling(t *testing.T) {
	row, worker, controller, h, base := reviewFixture()
	stages := []struct {
		name      string
		configure func()
		since     time.Time
		grace     time.Duration
		want      string
	}{
		{"worker completed", func() {}, base, 5 * time.Minute, "controller_review_pending"},
		{"handoff pending", func() {}, base.Add(time.Second), 5 * time.Minute, "controller_review_pending"},
		{"claimed", func() {
			h.claim = completionHandoffRecord{ClaimedBy: controller.AgentPath, ClaimedAttempt: controller.AttemptID, At: base.Add(2 * time.Second).Format(time.RFC3339Nano)}
		}, base.Add(2 * time.Second), 15 * time.Minute, "controller_review"},
		{"acceptance recorded", func() { h.acceptance = completionHandoffRecord{At: base.Add(3 * time.Second).Format(time.RFC3339Nano)} }, base.Add(3 * time.Second), 10 * time.Minute, "controller_finalizing"},
	}
	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			stage.configure()
			handoffs := map[string]*completionHandoff{"handoff-1": h}
			if stage.name == "worker completed" {
				handoffs = nil
			}
			for _, elapsed := range []time.Duration{time.Second, stage.grace - time.Nanosecond, stage.grace, stage.grace + time.Minute} {
				now := stage.since.Add(elapsed)
				review := completionReview(row, &worker, worker.EndedAt, handoffs, nil, []runtimeobs.Attempt{controller}, now)
				want := stage.want
				if elapsed >= stage.grace {
					want = "controller_recovery"
				}
				if review["state"] != want {
					t.Fatalf("elapsed %v review=%v want=%s", elapsed, review, want)
				}
				if review["since"] != stage.since.Format(time.RFC3339Nano) || review["grace_until"] != stage.since.Add(stage.grace).Format(time.RFC3339Nano) {
					t.Fatalf("phase anchor moved: %v", review)
				}
			}
		})
	}
}

func TestCompletionReviewRequiresExactContractAndExecutions(t *testing.T) {
	cases := []struct {
		name   string
		change func(*completionHandoff, runtimeobs.Attempt) runtimeobs.Attempt
		want   string
	}{
		{"wrong task", func(h *completionHandoff, c runtimeobs.Attempt) runtimeobs.Attempt {
			h.prepared.TaskID = "A-32"
			return c
		}, "controller_review_pending"},
		{"changed contract", func(h *completionHandoff, c runtimeobs.Attempt) runtimeobs.Attempt {
			h.prepared.Contract = "wrong"
			return c
		}, "controller_review_pending"},
		{"wrong worker attempt", func(h *completionHandoff, c runtimeobs.Attempt) runtimeobs.Attempt {
			h.prepared.SourceAttempt = "old-worker"
			return c
		}, "controller_review_pending"},
		{"wrong worker", func(h *completionHandoff, c runtimeobs.Attempt) runtimeobs.Attempt {
			h.prepared.SourceAgent = "/root/controller/pairi"
			return c
		}, "controller_review_pending"},
		{"wrong controller claim", func(h *completionHandoff, c runtimeobs.Attempt) runtimeobs.Attempt {
			h.claim.ClaimedAttempt = "unrelated-running"
			return c
		}, "controller_recovery"},
		{"wrong controller native", func(h *completionHandoff, c runtimeobs.Attempt) runtimeobs.Attempt {
			c.RuntimeAgentID = "unrelated"
			return c
		}, "controller_recovery"},
		{"wrong controller session", func(h *completionHandoff, c runtimeobs.Attempt) runtimeobs.Attempt {
			c.SessionID = "other-session"
			return c
		}, "controller_recovery"},
		{"wrong controller provider", func(h *completionHandoff, c runtimeobs.Attempt) runtimeobs.Attempt { c.Provider = "claude"; return c }, "controller_recovery"},
		{"controller ended", func(h *completionHandoff, c runtimeobs.Attempt) runtimeobs.Attempt {
			c.CurrentState = runtimeobs.StateCompleted
			c.Terminal = true
			return c
		}, "controller_recovery"},
		{"controller error", func(h *completionHandoff, c runtimeobs.Attempt) runtimeobs.Attempt {
			c.CurrentState = runtimeobs.StateErrored
			return c
		}, "controller_recovery"},
		{"explicit controller user wait", func(h *completionHandoff, c runtimeobs.Attempt) runtimeobs.Attempt {
			c.CurrentState = runtimeobs.StateWaitingUser
			return c
		}, "needs_user"},
		{"handoff failed", func(h *completionHandoff, c runtimeobs.Attempt) runtimeobs.Attempt { h.failed = true; return c }, "controller_recovery"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row, worker, controller, h, base := reviewFixture()
			h.claim = completionHandoffRecord{ClaimedBy: controller.AgentPath, ClaimedAttempt: controller.AttemptID, At: base.Add(2 * time.Second).Format(time.RFC3339Nano)}
			controller = tc.change(h, controller)
			got := completionReview(row, &worker, worker.EndedAt, map[string]*completionHandoff{"handoff-1": h}, nil, []runtimeobs.Attempt{controller}, base.Add(time.Minute))
			if got["state"] != tc.want {
				t.Fatalf("got %v want %s", got, tc.want)
			}
		})
	}
}

func TestCompletionReviewMissingOrUnreadableEvidenceNeverRequestsUser(t *testing.T) {
	row, worker, controller, _, now := reviewFixture()
	for _, err := range []error{os.ErrNotExist, errors.New("invalid JSON")} {
		got := completionReview(row, &worker, worker.EndedAt, nil, err, []runtimeobs.Attempt{controller}, now.Add(time.Minute))
		want := "controller_review_pending"
		if !os.IsNotExist(err) {
			want = "controller_recovery"
		}
		if got["state"] != want || got["audience"] != "controller" {
			t.Fatalf("got=%v", got)
		}
	}
	for _, at := range []string{"", now.Add(time.Hour).Format(time.RFC3339Nano)} {
		got := completionReview(row, nil, at, nil, nil, nil, now)
		if got["state"] != "controller_recovery" {
			t.Fatalf("bad source timestamp: %v", got)
		}
	}
}

func writeReviewJournal(t *testing.T, project string, records ...completionHandoffRecord) {
	t.Helper()
	file := filepath.Join(project, "_task_mecca", ".runtime", "handoffs", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte{}
	for _, record := range records {
		raw, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, raw...)
		data = append(data, '\n')
	}
	if err := os.WriteFile(file, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCompletionJournalCompatibilityAndFixedFirstAnchors(t *testing.T) {
	project := t.TempDir()
	_, _, controller, h, now := reviewFixture()
	id := h.prepared.HandoffID
	claim := completionHandoffRecord{HandoffID: id, Kind: "claimed", ClaimedBy: controller.AgentPath, ClaimedAttempt: controller.AttemptID, At: now.Add(2 * time.Second).Format(time.RFC3339Nano)}
	duplicate := claim
	duplicate.At = now.Add(time.Hour).Format(time.RFC3339Nano)
	fail := completionHandoffRecord{HandoffID: id, Kind: "step_failed", Step: "dispatch", Result: "failed", At: claim.At}
	repaired := fail
	repaired.Kind = "step_succeeded"
	repaired.Result = "ok"
	writeReviewJournal(t, project, h.prepared, claim, duplicate, fail, repaired)
	got, err := readCompletionHandoffs(project)
	if err != nil {
		t.Fatal(err)
	}
	if got[id].prepared.At != h.prepared.At || got[id].claim.At != claim.At || !got[id].failed {
		t.Fatalf("fold=%+v", got[id])
	}
	file := filepath.Join(project, "_task_mecca", ".runtime", "handoffs", "events.jsonl")
	if err := os.WriteFile(file, []byte("invalid\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readCompletionHandoffs(project); err == nil {
		t.Fatal("malformed journal accepted")
	}
}

func TestCompletionReviewProjectionKeepsUserAndControllerAudiencesSeparate(t *testing.T) {
	row, _, _, _, _ := reviewFixture()
	for _, health := range []string{"controller_review_pending", "controller_review", "controller_finalizing", "controller_recovery"} {
		signal := map[string]any{"health": health, "completion_review": map[string]any{"state": health, "audience": "controller"}}
		item := webSummaryItem(row, row.State, nil, nil, signal)
		if item["state"] != health || item["file_state"] != "doing" || item["attention_reason"] != nil {
			t.Fatalf("review leaks into user attention: %v", item)
		}
		if continuityHealthConsumesWorkerSlot(health) {
			t.Fatalf("completed worker consumes slot: %s", health)
		}
	}
	signal := map[string]any{"health": "needs_user"}
	reason, _ := canonicalOperationalState(row, nil, signal)
	item := webSummaryItem(row, row.State, nil, reason, signal)
	if item["state"] != "needs_user" || item["attention_reason"] == nil {
		t.Fatalf("explicit user wait lost: %v", item)
	}
	row.State = "done"
	item = webSummaryItem(row, row.State, nil, nil, map[string]any{"health": "controller_review"})
	if item["state"] != "done" {
		t.Fatalf("file done overridden: %v", item)
	}
}

func appendReviewAttempt(t *testing.T, project string, a runtimeobs.Attempt, ended time.Time) {
	t.Helper()
	events := []runtimeobs.ExecutionEvent{
		{EventKind: "state", AttemptID: a.AttemptID, Provider: "codex", RuntimeAgentID: a.RuntimeAgentID, SessionID: a.SessionID, ObservedAt: a.StartedAt, State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved},
		{EventKind: "binding", AttemptID: a.AttemptID, ObservedAt: a.StartedAt, TaskID: a.TaskID, AgentPath: a.AgentPath, EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative},
	}
	if a.CurrentState != runtimeobs.StateRunning {
		events = append(events, runtimeobs.ExecutionEvent{EventKind: "state", AttemptID: a.AttemptID, Provider: "codex", RuntimeAgentID: a.RuntimeAgentID, SessionID: a.SessionID, ObservedAt: ended.Format(time.RFC3339Nano), State: a.CurrentState, Terminal: a.Terminal, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved})
	}
	for _, event := range events {
		if err := runtimeobs.AppendExecutionEvent(project, event); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCompletionReviewActualEventsAcrossWebAndRecoveryAPIs(t *testing.T) {
	modes := []struct {
		name, state     string
		workerRunning   bool
		workerError     bool
		controllerState runtimeobs.CanonicalState
		fail, accept    bool
	}{
		{name: "DONE before journal", state: "controller_review_pending", controllerState: runtimeobs.StateRunning},
		{name: "DONE handoff claim before Worker terminal", state: "controller_review", workerRunning: true, controllerState: runtimeobs.StateRunning},
		{name: "terminal after DONE handoff claim", state: "controller_review", controllerState: runtimeobs.StateRunning},
		{name: "failed handoff", state: "controller_recovery", controllerState: runtimeobs.StateRunning, fail: true},
		{name: "Worker error after DONE report", state: "controller_recovery", controllerState: runtimeobs.StateRunning, workerError: true},
		{name: "stopped Controller", state: "controller_recovery", controllerState: runtimeobs.StateCompleted},
		{name: "explicit Controller user wait", state: "needs_user", controllerState: runtimeobs.StateWaitingUser},
		{name: "Controller acceptance before file done", state: "controller_finalizing", controllerState: runtimeobs.StateRunning, accept: true},
	}
	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			project := t.TempDir()
			row, worker, controller, h, _ := reviewFixture()
			now := time.Now().UTC().Add(-30 * time.Second)
			worker.StartedAt = now.Add(-time.Minute).Format(time.RFC3339Nano)
			worker.EndedAt = now.Format(time.RFC3339Nano)
			controller.StartedAt = worker.StartedAt
			controller.CurrentState = mode.controllerState
			controller.Terminal = mode.controllerState == runtimeobs.StateCompleted
			if mode.workerRunning {
				worker.CurrentState = runtimeobs.StateRunning
				worker.Terminal = false
			}
			if mode.workerError {
				worker.CurrentState = runtimeobs.StateErrored
				worker.Terminal = true
			}
			folder := filepath.Join(project, "_task_mecca", "data", "backlog")
			file := writeWebTask(t, folder, "000031.A-31.review.doing.md", row.RawMarkdown)
			appendReviewAttempt(t, project, worker, now)
			appendReviewAttempt(t, project, controller, now)
			if mode.name != "DONE before journal" {
				h.prepared.At = now.Add(-2 * time.Second).Format(time.RFC3339Nano)
				claim := completionHandoffRecord{HandoffID: h.prepared.HandoffID, Kind: "claimed", ClaimedBy: controller.AgentPath, ClaimedAttempt: controller.AttemptID, At: now.Add(-time.Second).Format(time.RFC3339Nano)}
				records := []completionHandoffRecord{h.prepared, claim}
				if mode.fail {
					records = append(records, completionHandoffRecord{HandoffID: h.prepared.HandoffID, Kind: "step_failed", Step: "dispatch", Result: "failed", At: now.Format(time.RFC3339Nano)})
				}
				if mode.accept {
					records = append(records, completionHandoffRecord{HandoffID: h.prepared.HandoffID, Kind: "step_succeeded", Step: "acceptance", Result: "ok", At: now.Format(time.RFC3339Nano)})
				}
				writeReviewJournal(t, project, records...)
			}
			snapshot, err := AttentionSnapshot(project, folder, true)
			if err != nil {
				t.Fatal(err)
			}
			items := snapshot["all_items"].(map[string]map[string]any)
			item := items[row.ID]
			if item == nil || item["state"] != mode.state || item["file_state"] != "doing" {
				t.Fatalf("attention item=%v expected %s", item, mode.state)
			}
			attention := snapshot["attention"].([]map[string]any)
			if mode.state == "needs_user" {
				if len(attention) != 1 || attention[0]["type"] != "user_intervention" {
					t.Fatalf("real user wait lost: %v", attention)
				}
			} else if len(attention) != 0 {
				t.Fatalf("Controller work leaked into user attention/feed: %v", attention)
			}
			if len(snapshot["notification_events"].([]map[string]any)) != 0 {
				t.Fatal("premature completion notification")
			}
			detail, err := TaskDetail(project, folder, row.ID)
			if err != nil {
				t.Fatal(err)
			}
			if detail["state"] != mode.state || detail["file_state"] != "doing" {
				t.Fatalf("detail=%v", detail)
			}
			list, err := BacklogPage(project, folder, 1, 20, []string{"doing"}, nil, "", "id_desc")
			if err != nil {
				t.Fatal(err)
			}
			rows := list["items"].([]map[string]any)
			if len(rows) != 1 || rows[0]["state"] != mode.state {
				t.Fatalf("list=%v", rows)
			}
			dashboard, err := DashboardSnapshot(project, folder, 5)
			if err != nil {
				t.Fatal(err)
			}
			if mode.state != "needs_user" && len(dashboard["attention"].([]map[string]any)) != 0 {
				t.Fatal("dashboard leaks Controller attention")
			}
			if dashboard["all_items"].(map[string]map[string]any)[row.ID]["state"] != mode.state {
				t.Fatal("dashboard lost review state")
			}
			coordinate, err := Coordinate(project, folder, 3)
			if err != nil {
				t.Fatal(err)
			}
			gaps := coordinate["continuity_gaps"].([]map[string]any)
			if mode.state == "controller_recovery" {
				if len(gaps) != 1 || gaps[0]["code"] != "controller_completion_recovery" {
					t.Fatalf("recovery=%v", gaps)
				}
			} else if mode.state != "needs_user" && len(gaps) != 0 {
				t.Fatalf("normal Controller review falsely treated as gap: %v", gaps)
			}
			if err := os.Rename(file, filepath.Join(folder, "000031.A-31.review.done.md")); err != nil {
				t.Fatal(err)
			}
			done, err := AttentionSnapshot(project, folder, true)
			if err != nil {
				t.Fatal(err)
			}
			if events := done["notification_events"].([]map[string]any); len(events) != 1 || events[0]["kind"] != "completed" {
				t.Fatalf("final completion=%v", events)
			}
		})
	}
}
