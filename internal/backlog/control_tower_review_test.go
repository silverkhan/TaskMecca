package backlog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type controllerReviewFixture struct {
	project    string
	now        time.Time
	row        Record
	activity   map[string]map[string]any
	prepared   map[string]any
	claim      map[string]any
	assignment runtimeobs.Assignment
}

func (f *controllerReviewFixture) assignmentTime() time.Time {
	at, _ := time.Parse(time.RFC3339Nano, f.assignment.AssignedAt)
	return at
}

func newControllerReviewFixture(t *testing.T, withControllerHook bool) *controllerReviewFixture {
	t.Helper()
	project := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	worker := "/root/controller/kkobugi"
	assignment, err := runtimeobs.RecordAssignment(project, "assignment-review-test", "A-51", worker, now.Add(-3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: now.Add(-2 * time.Minute).Format(time.RFC3339Nano), AttemptID: "worker-attempt", Provider: "codex", SessionID: "worker-session", RuntimeAgentID: "worker-runtime", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.BindAttempt(project, "worker-attempt", "A-51", worker, "explicit", "", map[string]string{"assignment_id": assignment.AssignmentID}, now.Add(-119*time.Second)); err != nil {
		t.Fatal(err)
	}
	if withControllerHook {
		if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: now.Add(-30 * time.Second).Format(time.RFC3339Nano), AttemptID: "controller-attempt", Provider: "codex", SessionID: "root-session", RuntimeAgentID: "controller-runtime", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
			t.Fatal(err)
		}
		if _, err := runtimeobs.BindAttempt(project, "controller-attempt", "", "/root/controller", "explicit", "", nil, now.Add(-29*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	return &controllerReviewFixture{
		project: project, now: now, assignment: assignment,
		row:      Record{ID: "A-51", Path: "/project/A-51.doing.md", State: "doing", Location: "active", Fields: map[string]string{"Agent": worker}},
		activity: map[string]map[string]any{"A-51": {"health": "stale", "runtime_state": "running", "attempt_id": "worker-attempt"}},
		prepared: map[string]any{"handoff_id": "handoff-review-test", "record_kind": "prepared", "event_type": "worker_done", "task_id": "A-51", "source_agent_path": worker, "source_attempt_id": "worker-attempt", "target_agent_path": "/root/controller", "target_attempt_id": "controller-attempt", "target_runtime_agent_id": "controller-runtime", "target_session_id": "root-session", "provider": "codex", "observed_at": now.Add(-20 * time.Second).Format(time.RFC3339Nano)},
		claim:    map[string]any{"handoff_id": "handoff-review-test", "record_kind": "claimed", "claimed_by": "/root/controller", "claimed_attempt_id": "controller-attempt", "observed_at": now.Add(-10 * time.Second).Format(time.RFC3339Nano)},
	}
}

func (f *controllerReviewFixture) writeHandoff(t *testing.T, records ...map[string]any) {
	t.Helper()
	path := filepath.Join(f.project, "_task_mecca", ".runtime", "handoffs", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	writer := bufio.NewWriter(file)
	for _, record := range records {
		data, e := json.Marshal(record)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = writer.Write(append(data, '\n')); e != nil {
			t.Fatal(e)
		}
	}
	if err = writer.Flush(); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
}

func (f *controllerReviewFixture) apply(t *testing.T) map[string]any {
	t.Helper()
	suppressLiveControllerReviewSignals(f.project, []Record{f.row}, f.activity, f.now)
	return f.activity["A-51"]
}

func TestLiveExactControllerReviewSuppressesRoutineHealthOnly(t *testing.T) {
	for _, health := range []string{"stale", "worker_missing", "runtime_unknown"} {
		t.Run(health, func(t *testing.T) {
			f := newControllerReviewFixture(t, true)
			f.writeHandoff(t, f.prepared, f.claim)
			f.activity["A-51"]["health"] = health
			signal := f.apply(t)
			if signal["health"] != "controller_review" {
				t.Fatalf("routine signal not suppressed: %+v", signal)
			}
			review, ok := signal["controller_review"].(map[string]any)
			if !ok || review["handoff_id"] != "handoff-review-test" || review["assignment_id"] != f.assignment.AssignmentID || review["attempt_id"] != "controller-attempt" || review["actor"] != "/root/controller" {
				t.Fatalf("exact review evidence missing: %+v", signal)
			}
			reason, condition := canonicalOperationalState(f.row, nil, signal, f.now)
			if len(reason) != 0 || len(condition) != 0 {
				t.Fatalf("routine alert still visible: %+v %+v", reason, condition)
			}
		})
	}
}

func TestControllerReviewMismatchFailsClosed(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*controllerReviewFixture)
	}{
		{"task", func(f *controllerReviewFixture) { f.prepared["task_id"] = "A-52" }},
		{"source_attempt", func(f *controllerReviewFixture) { f.prepared["source_attempt_id"] = "other-worker-attempt" }},
		{"target_attempt", func(f *controllerReviewFixture) {
			f.prepared["target_attempt_id"] = "other-controller-attempt"
			f.claim["claimed_attempt_id"] = "other-controller-attempt"
		}},
		{"claimant", func(f *controllerReviewFixture) { f.claim["claimed_by"] = "/root/controller/other" }},
		{"target_path", func(f *controllerReviewFixture) { f.prepared["target_agent_path"] = "/root/controller/other" }},
		{"runtime_agent", func(f *controllerReviewFixture) { f.prepared["target_runtime_agent_id"] = "other-runtime" }},
		{"session", func(f *controllerReviewFixture) { f.prepared["target_session_id"] = "other-session" }},
		{"provider", func(f *controllerReviewFixture) { f.prepared["provider"] = "claude" }},
		{"worker_assignment", func(f *controllerReviewFixture) { f.prepared["source_agent_path"] = "/root/controller/other" }},
		{"latest_assignment_mismatch", func(f *controllerReviewFixture) {
			if _, err := runtimeobs.RecordAssignment(f.project, "assignment-newer", "A-51", "/root/controller/kkobugi", f.now.Add(-time.Second)); err != nil {
				t.Fatal(err)
			}
		}},
		{"ambiguous_latest_assignment", func(f *controllerReviewFixture) {
			if _, err := runtimeobs.RecordAssignment(f.project, "assignment-tie", "A-51", "/root/controller/kkobugi", f.assignmentTime()); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newControllerReviewFixture(t, true)
			tc.mutate(f)
			f.writeHandoff(t, f.prepared, f.claim)
			signal := f.apply(t)
			if signal["health"] != "stale" {
				t.Fatalf("mismatch was hidden: %+v", signal)
			}
			reason, _ := canonicalOperationalState(f.row, nil, signal, f.now)
			if reason["type"] != "runtime_stalled" {
				t.Fatalf("expected stale alert to remain: %+v", reason)
			}
		})
	}
}

func TestControllerReviewDoesNotHideFailureOrUserHold(t *testing.T) {
	t.Run("confirmed_failure", func(t *testing.T) {
		f := newControllerReviewFixture(t, true)
		f.writeHandoff(t, f.prepared, f.claim)
		if err := runtimeobs.AppendExecutionEvent(f.project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: f.now.Add(-5 * time.Second).Format(time.RFC3339Nano), AttemptID: "worker-attempt", Provider: "codex", SessionID: "worker-session", RuntimeAgentID: "worker-runtime", State: runtimeobs.StateErrored, Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
			t.Fatal(err)
		}
		f.activity["A-51"]["health"] = "execution_interrupted"
		f.activity["A-51"]["runtime_state"] = "interrupted"
		signal := f.apply(t)
		reason, _ := canonicalOperationalState(f.row, nil, signal, f.now)
		if signal["health"] != "execution_interrupted" || reason["type"] != "execution_interrupted" {
			t.Fatalf("confirmed failure hidden: %+v %+v", signal, reason)
		}
	})
	t.Run("user_hold", func(t *testing.T) {
		f := newControllerReviewFixture(t, true)
		f.row.State = "hold"
		f.row.Fields["대기유형"] = "user"
		f.row.Fields["대기"] = "사용자 답변 대기"
		f.writeHandoff(t, f.prepared, f.claim)
		f.apply(t)
		reason, condition := canonicalOperationalState(f.row, nil, f.activity["A-51"], f.now)
		if reason["type"] != "user_intervention" || condition["kind"] != "intervention" {
			t.Fatalf("user wait hidden: %+v %+v", reason, condition)
		}
	})
}

func TestControllerReviewDuplicateLateAndUnknownEvidenceRemainVisible(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*controllerReviewFixture)
	}{
		{"duplicate_claim", func(f *controllerReviewFixture) {
			duplicate := map[string]any{}
			for k, v := range f.claim {
				duplicate[k] = v
			}
			duplicate["claimed_by"] = "/root/controller/other"
			f.writeHandoff(t, f.prepared, f.claim, duplicate)
		}},
		{"claim_before_prepared", func(f *controllerReviewFixture) {
			f.claim["observed_at"] = f.now.Add(-30 * time.Second).Format(time.RFC3339Nano)
			f.writeHandoff(t, f.prepared, f.claim)
		}},
		{"future_claim", func(f *controllerReviewFixture) {
			f.claim["observed_at"] = f.now.Add(time.Minute).Format(time.RFC3339Nano)
			f.writeHandoff(t, f.prepared, f.claim)
		}},
		{"missing_controller_hook", func(f *controllerReviewFixture) { f.writeHandoff(t, f.prepared, f.claim) }},
		{"unknown_controller_state", func(f *controllerReviewFixture) {
			if err := runtimeobs.AppendExecutionEvent(f.project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: f.now.Add(-30 * time.Second).Format(time.RFC3339Nano), AttemptID: "controller-attempt", Provider: "codex", SessionID: "root-session", RuntimeAgentID: "controller-runtime", State: runtimeobs.StateRuntimeUnknown, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityInferred}); err != nil {
				t.Fatal(err)
			}
			if _, err := runtimeobs.BindAttempt(f.project, "controller-attempt", "", "/root/controller", "explicit", "", nil, f.now.Add(-29*time.Second)); err != nil {
				t.Fatal(err)
			}
			f.writeHandoff(t, f.prepared, f.claim)
		}},
		{"missing_handoff", func(f *controllerReviewFixture) {}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withHook := tc.name != "missing_controller_hook" && tc.name != "missing_handoff"
			f := newControllerReviewFixture(t, withHook)
			tc.setup(f)
			signal := f.apply(t)
			if signal["health"] != "stale" {
				t.Fatalf("incomplete/ambiguous evidence inferred review: %+v", signal)
			}
		})
	}
}

func TestControllerReviewOutOfOrderJournalUsesClaimTimesConservatively(t *testing.T) {
	f := newControllerReviewFixture(t, true)
	// Arrival order is reversed, but source timestamps prove a valid sequence.
	f.writeHandoff(t, f.claim, f.prepared)
	signal := f.apply(t)
	if signal["health"] != "controller_review" {
		t.Fatalf("valid timestamp ordering should reconcile independent of line order: %+v", signal)
	}
}
