package runtimeobs

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func currentEvidenceFixture(now time.Time) (CurrentExecutionIdentity, Attempt) {
	id := CurrentExecutionIdentity{Project: "/fixture/project", TaskID: "A-83", AgentPath: "/root/controller/worker", NativeID: "native", AssignmentID: "assigned", Provider: "codex"}
	r := &T3Receipt{ThreadID: "thread", RunID: "current-run", NativeAttemptID: "native-attempt", SessionID: "native", Provider: "codex", State: StateRunning, StartedAt: now.Add(-time.Hour).Format(time.RFC3339Nano), ObservedAt: now.Format(time.RFC3339Nano), Origin: "t3_local_projection_v2"}
	a := Attempt{AttemptID: "attempt", Provider: "codex", TaskID: id.TaskID, AgentPath: id.AgentPath, SessionID: "native", RuntimeAgentID: "native", TurnID: r.NativeAttemptID, BindingState: BindingBound, BindingEvidence: map[string]string{"assignment_id": id.AssignmentID}, CurrentState: StateRunning, StateEvidenceSource: EvidenceT3, StateObservationQuality: QualityObserved, StartedAt: r.StartedAt, LastObservedAt: r.ObservedAt, NativeReceipt: r}
	return id, a
}

func TestCurrentExecutionExactIdentityAndFreshness(t *testing.T) {
	now := time.Date(2026, 11, 17, 5, 0, 0, 0, time.UTC)
	id, a := currentEvidenceFixture(now)
	s := EvaluateCurrentExecution(id, []Attempt{a}, now)
	if !s.Matched || !s.Fresh || s.OfficialCurrent || s.ProgressAt != "" {
		t.Fatalf("running became progress or certificate: %+v", s)
	}
	for name, change := range map[string]func(*Attempt){
		"reviewer":       func(a *Attempt) { a.AgentPath = "/root/controller/reviewer" },
		"old assignment": func(a *Attempt) { a.BindingEvidence = map[string]string{"assignment_id": "old"} },
		"other native":   func(a *Attempt) { a.SessionID = "other" },
		"other task":     func(a *Attempt) { a.TaskID = "A-84" },
		"other provider": func(a *Attempt) { a.Provider = "claude" },
		"ambiguous":      func(a *Attempt) { a.BindingState = BindingAmbiguous },
	} {
		t.Run(name, func(t *testing.T) {
			bad := a
			change(&bad)
			s := EvaluateCurrentExecution(id, []Attempt{bad}, now)
			if s.Matched || s.Fresh {
				t.Fatalf("substitute selected: %+v", s)
			}
		})
	}
	if s := EvaluateCurrentExecution(id, []Attempt{a, a}, now); s.Matched {
		t.Fatal("ambiguous episodes selected")
	}
	if s := EvaluateCurrentExecution(id, []Attempt{a}, now.Add(3*time.Minute)); s.Fresh || !s.Matched || s.FailedStage != "observation_freshness" {
		t.Fatalf("stale not diagnostic %+v", s)
	}
}

func TestCurrentExecutionBoundedReadOnlyRefresh(t *testing.T) {
	now := time.Date(2026, 11, 17, 5, 0, 0, 0, time.UTC)
	id, a := currentEvidenceFixture(now)
	s := EvaluateCurrentExecution(id, []Attempt{a}, now.Add(3*time.Minute))
	before := *s.Receipt
	calls := 0
	read := func(sel T3Selector, at time.Time) (T3Receipt, error) {
		calls++
		if sel.RunID != "current-run" || sel.NativeAttemptID != a.TurnID {
			t.Fatal("unpinned selector")
		}
		r := *a.NativeReceipt
		r.ObservedAt = at.Format(time.RFC3339Nano)
		return r, nil
	}
	got := refreshCurrentExecution(s, now.Add(3*time.Minute), read)
	if calls != 1 || !got.Fresh || !got.OfficialCurrent || got.ProgressAt != "" || !reflect.DeepEqual(before, *s.Receipt) {
		t.Fatalf("refresh mutated source/progress: %+v", got)
	}
	for _, failure := range []string{"unsupported", "new run", "new episode", "wrong native", "future"} {
		t.Run(failure, func(t *testing.T) {
			calls = 0
			got := refreshCurrentExecution(s, now, func(sel T3Selector, at time.Time) (T3Receipt, error) {
				calls++
				r := *a.NativeReceipt
				switch failure {
				case "unsupported":
					return r, errors.New("unsupported")
				case "new run":
					r.RunID = "resumed"
				case "new episode":
					r.NativeAttemptID = "retry"
				case "wrong native":
					r.SessionID = "other"
				case "future":
					r.ObservedAt = at.Add(time.Second).Format(time.RFC3339Nano)
				}
				return r, nil
			})
			if calls != 1 || got.OfficialCurrent || got.Fresh || got.FailedStage == "" || got.ActionOwner == "" {
				t.Fatalf("bad observation accepted: %+v", got)
			}
		})
	}
}

func TestCurrentExecutionRunningDoesNotEraseWaitOrInventProgress(t *testing.T) {
	now := time.Now().UTC()
	id, a := currentEvidenceFixture(now)
	a.CurrentState = StateWaitingApproval
	a.NativeReceipt.State = a.CurrentState
	a.RecentTransitions = []Transition{{State: a.CurrentState, Reason: "permission approval required", ObservationQuality: QualityObserved}}
	s := EvaluateCurrentExecution(id, []Attempt{a}, now)
	got := refreshCurrentExecution(s, now, func(T3Selector, time.Time) (T3Receipt, error) {
		r := *a.NativeReceipt
		r.State = StateRunning
		return r, nil
	})
	if !got.OfficialCurrent || got.WaitReason != s.WaitReason || got.WaitReason == "" || got.ProgressAt != "" {
		t.Fatalf("running erased blocker: %+v", got)
	}
}
