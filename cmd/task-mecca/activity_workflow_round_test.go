package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
)

func TestActivityReportSharesGroundedRoundWithHistoricalHandoff(t *testing.T) {
	project := t.TempDir()
	now := time.Now().UTC()
	store := activity.Ledger{Project: project}
	start := activity.Event{SchemaVersion: 1, EventID: "start", TaskID: "B-1", Action: activity.Start, Role: activity.RoleWorker, AgentID: "w", RuntimeAttemptID: "a", Type: activity.Implementation, Confidence: activity.Confirmed, OccurredAt: now.Add(-10 * time.Minute), RecordedAt: now.Add(-10 * time.Minute)}
	if _, err := store.Append(start); err != nil {
		t.Fatal(err)
	}
	path := handoffjournal.JournalPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(f)
	for _, r := range []handoffjournal.Record{{HandoffID: "h", RecordKind: "prepared", TaskID: "B-1", EventType: handoffjournal.EventWorkerDone, ObservedAt: now.Add(-5 * time.Minute).Format(time.RFC3339Nano)}, {HandoffID: "h", RecordKind: "step_failed", Step: "acceptance", Result: "failed", ObservedAt: now.Add(-4 * time.Minute).Format(time.RFC3339Nano)}} {
		if err = encoder.Encode(r); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()
	resolved, err := resolveActivityReport(store, "B-1", activity.RoleWorker, "w", "a", "implementation_started")
	if err != nil || resolved.Round != 1 {
		t.Fatalf("active worker failed to enter grounded rework: %+v %v", resolved, err)
	}
	review := start
	review.EventID = "review"
	review.Role = activity.RoleController
	review.AgentID = "c"
	review.RuntimeAttemptID = "c"
	review.Action = activity.Stop
	review.Type = activity.ControllerReview
	review.HandoffID = "h"
	review.ReviewOutcome = "needs_changes"
	review.OccurredAt = now.Add(-4 * time.Minute)
	if _, err = store.Append(review); err != nil {
		t.Fatal(err)
	}
	resolved, err = resolveActivityReport(store, "B-1", activity.RoleWorker, "w", "new-native-attempt", "requirements_started")
	if err != nil || resolved.Round != 1 {
		t.Fatalf("duplicate source or native replacement invented a round: %+v %v", resolved, err)
	}
}
