package backlog

import (
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"testing"
	"time"
)

func TestAssignmentObservationGraceUsesDurableTimeAndPreservesFailures(t *testing.T) {
	now := time.Date(2026, 10, 8, 5, 0, 0, 0, time.UTC)
	row := Record{ID: "A-44", State: "doing", Location: "active"}
	assigned := now.Add(-runtimeobs.AssignmentObservationGrace)
	signal := map[string]any{"health": "assignment_unobserved", "assigned_at": assigned.Format(time.RFC3339Nano), "assignment_id": "assignment-current"}
	for _, delta := range []time.Duration{-time.Nanosecond, 0, time.Minute} {
		reason, _ := canonicalOperationalState(row, nil, signal, now.Add(delta))
		if (len(reason) > 0) != (delta >= 0) {
			t.Fatalf("boundary %v: %#v", delta, reason)
		}
	}
	// Repeated projections never move the assignment's grace anchor.
	for i := 0; i < 3; i++ {
		reason, _ := canonicalOperationalState(row, nil, signal, now.Add(time.Duration(i)*time.Second))
		if reason["assigned_at"] != signal["assigned_at"] {
			t.Fatal(reason)
		}
	}
	for _, at := range []string{"", "invalid", now.Add(time.Minute).Format(time.RFC3339Nano)} {
		reason, _ := canonicalOperationalState(row, nil, map[string]any{"health": "assignment_unobserved", "assigned_at": at}, now)
		if len(reason) == 0 {
			t.Fatalf("unknown timestamp hidden: %q", at)
		}
	}
	for _, health := range []string{"binding_ambiguous", "worker_missing", "execution_interrupted", "needs_user", "runtime_unknown"} {
		reason, _ := canonicalOperationalState(row, nil, map[string]any{"health": health, "assigned_at": now.Format(time.RFC3339Nano)}, now)
		if len(reason) == 0 {
			t.Fatalf("failure hidden: %s", health)
		}
	}
	reason, _ := canonicalOperationalState(row, nil, map[string]any{"health": "active", "attempt_id": "actual-hook"}, now)
	if len(reason) > 0 {
		t.Fatal("late hook did not clear warning", reason)
	}
}
