package activity

import (
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"testing"
	"time"
)

func TestTerminalHookClosesUnknownWithoutInventingTime(t *testing.T) {
	for _, actor := range []struct {
		path string
		role Role
	}{
		{"/root/controller/w", RoleWorker},
		{"/root/controller/reviewer/raichyu", RoleReviewer},
	} {
		t.Run(string(actor.role), func(t *testing.T) {
			project := t.TempDir()
			base := time.Now().UTC().Add(-5 * time.Minute)
			assignment, err := runtimeobs.RecordAssignment(project, "assignment-terminal", "B-589", actor.path, base)
			if err != nil {
				t.Fatal(err)
			}
			start := runtimeobs.ExecutionEvent{
				EventID: "hook-start", EventKind: "state", AttemptID: "run-terminal", ObservedAt: base.Format(time.RFC3339Nano),
				RuntimeAgentID: "agent-w", Provider: "codex", State: runtimeobs.StateRunning,
				EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved,
			}
			if err := runtimeobs.AppendExecutionEvent(project, start); err != nil {
				t.Fatal(err)
			}
			_, err = runtimeobs.BindAttempt(project, start.AttemptID, assignment.TaskID, assignment.AgentPath, "dispatch", "",
				map[string]string{"assignment_id": assignment.AssignmentID}, base.Add(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			observed := validEvent()
			observed.Role = actor.role
			observed.EventID = "reported-start"
			observed.AgentID = "agent-w"
			observed.RuntimeAttemptID = "run-terminal"
			observed.AssignmentID = assignment.AssignmentID
			observed.OccurredAt = base.Add(2 * time.Minute)
			observed.RecordedAt = observed.OccurredAt
			observed.Confidence = Inferred
			observed.Evidence = "reported analysis"
			if ok, err := (Ledger{Project: project}).Apply(observed); err != nil || !ok {
				t.Fatalf("activity start %v %v", ok, err)
			}
			end := start
			end.EventID = "hook-terminal"
			end.Terminal = true
			end.State = runtimeobs.StateCompleted
			end.ObservedAt = base.Add(4 * time.Minute).Format(time.RFC3339Nano)
			if err := runtimeobs.AppendExecutionEvent(project, end); err != nil {
				t.Fatal(err)
			}
			if changed, err := RecordTerminal(project, end); err != nil || !changed {
				t.Fatalf("terminal close %v %v", changed, err)
			}
			if changed, err := RecordTerminal(project, end); err != nil || changed {
				t.Fatalf("terminal idempotency %v %v", changed, err)
			}
			rows, err := (Ledger{Project: project}).List(assignment.TaskID)
			if err != nil {
				t.Fatal(err)
			}
			summary := Summarize(assignment.TaskID, rows)
			if summary.InterruptedActivities != 1 || summary.AgentTimeMillis != 0 || summary.OpenActivities != 0 ||
				summary.UnmeasuredUpperBoundMillis != 120000 {
				t.Fatalf("terminal cannot invent active duration: %+v", summary)
			}
		})
	}
}

func TestNonTerminalHookHasZeroRecorderCost(t *testing.T) {
	changed, err := RecordTerminal("/missing", runtimeobs.ExecutionEvent{EventKind: "activity", AttemptID: "run-one"})
	if err != nil || changed {
		t.Fatalf("tool hook should never touch recorder: %v %v", changed, err)
	}
}

func TestA98TerminalNativeCompletionNeverBecomesPhaseSuccess(t *testing.T) {
	p := t.TempDir()
	base := time.Now().UTC().Add(-10 * time.Minute)
	stamp := func(m int) string { return base.Add(time.Duration(m) * time.Minute).Format(time.RFC3339Nano) }
	assignment, err := runtimeobs.RecordAssignment(p, "assignment-native-end", "B-589", "/root/controller/fixture", base)
	if err != nil {
		t.Fatal(err)
	}
	start := runtimeobs.ExecutionEvent{EventID: "start", EventKind: "state", AttemptID: "run-native-end", Provider: "codex", SessionID: "native", RuntimeAgentID: "native", TurnID: "native-run", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved, ObservedAt: stamp(0)}
	if err := runtimeobs.AppendExecutionEvent(p, start); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.BindAttempt(p, start.AttemptID, assignment.TaskID, assignment.AgentPath, "dispatch", "", map[string]string{"assignment_id": assignment.AssignmentID}, base); err != nil {
		t.Fatal(err)
	}
	e := validEvent()
	e.EventID = "phase"
	e.AgentID = "native"
	e.RuntimeAttemptID = start.AttemptID
	e.AssignmentID = assignment.AssignmentID
	e.OccurredAt = base.Add(2 * time.Minute)
	e.RecordedAt = e.OccurredAt
	e.Explanation = "명시적 원문"
	e.ExplanationOrigin = "written"
	if _, err := (Ledger{Project: p}).Apply(e); err != nil {
		t.Fatal(err)
	}
	end := start
	end.EventID = "end"
	end.Terminal = true
	end.State = runtimeobs.StateCompleted
	end.ObservedAt = stamp(5)
	end.EvidenceSource = runtimeobs.EvidenceT3
	end.NativeReceipt = &runtimeobs.T3Receipt{Origin: "t3_local_projection_v2", ThreadID: "thread", RunID: "run", NativeAttemptID: "native-run", Provider: "codex", SessionID: "native", State: runtimeobs.StateCompleted, StartedAt: stamp(0), CompletedAt: stamp(3), ObservedAt: stamp(5)}
	if err := runtimeobs.AppendExecutionEvent(p, end); err != nil {
		t.Fatal(err)
	}
	if changed, err := RecordTerminal(p, end); err != nil || !changed {
		t.Fatal(changed, err)
	}
	rows, err := (Ledger{Project: p}).List("B-589")
	if err != nil {
		t.Fatal(err)
	}
	last := rows[len(rows)-1]
	if last.Action != Abandon || last.ReviewOutcome != "" || !last.OccurredAt.Equal(base.Add(3*time.Minute)) || last.ExplanationOrigin != "written" {
		t.Fatal("native end invented success or collector work", last)
	}
	s := Summarize("B-589", rows)
	if s.AgentTimeMillis != 0 || s.OpenActivities != 0 || s.UnmeasuredUpperBoundMillis != 60000 {
		t.Fatal("collector delay became work", s)
	}
	if _, err := runtimeobs.RecordCollectionReceipt(p, terminalCollectionReceipt(last, "pending", "")); err != nil {
		t.Fatal(err)
	}
	if changed, err := RecordTerminal(p, end); err != nil || changed {
		t.Fatal("terminal retry did not reconcile immutable event", changed, err)
	}
	receipts, err := runtimeobs.ReadCollectionReceipts(p)
	if err != nil || receipts[len(receipts)-1].Status != "ack" {
		t.Fatal("terminal ACK not restored", err, receipts)
	}
}
