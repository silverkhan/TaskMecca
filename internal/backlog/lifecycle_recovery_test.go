package backlog

import (
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestA66RecoveryPreservesOnlyRegisteredAndWaiting(t *testing.T) {
	project := t.TempDir()
	dir := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "000066.A-66.recovery.todo.md"), []byte("# A-66 Recovery\n- Agent: -\n"), 0644); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Minute)
	for i, kind := range []string{"registered", "waiting"} {
		e := LifecycleTransition{TaskID: "A-66", EventID: "a66-" + kind, Kind: kind, Actor: "/root/controller", EvidenceSource: "controller_report", OccurredAt: at.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano)}
		for j := 0; j < 2; j++ {
			if _, err := RecordLifecycleTransition(project, e, at); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Observing Controller execution does not assign/start any Worker or task.
	receipt := runtimeobs.T3Receipt{ThreadID: "controller", RunID: "current-run", NativeAttemptID: "current-turn", SessionID: "native", Provider: "codex", State: runtimeobs.StateRunning, ObservedAt: at.Add(2 * time.Second).Format(time.RFC3339Nano), Origin: "t3_local_projection_v2"}
	if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", AttemptID: "controller", RuntimeAgentID: "native", Provider: "codex", SessionID: "native", TurnID: "current-turn", ObservedAt: receipt.ObservedAt, State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceT3, ObservationQuality: runtimeobs.QualityObserved, NativeReceipt: &receipt}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.BindAttempt(project, "controller", "", "/root/controller", "dispatch", "", nil, at.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		rows, err := LifecycleTimings(project, "")
		if err != nil {
			t.Fatal(err)
		}
		row := rows["A-66"]
		events := row["events"].([]map[string]any)
		if len(events) != 2 || events[0]["kind"] != "registered" || events[1]["kind"] != "waiting" || row["started_at"] != nil || row["completed_at"] != nil {
			t.Fatalf("fabricated A66 history: %+v", row)
		}
	}
}
