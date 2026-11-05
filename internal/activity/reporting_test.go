package activity

import (
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestReportingBoundaryDetectsMissingActualReportWithoutWriting(t *testing.T) {
	project := t.TempDir()
	now := time.Now().UTC()
	_, err := runtimeobs.RecordAssignment(project, "assignment-one", "B-1", "/root/controller/pairi", now)
	if err != nil {
		t.Fatal(err)
	}
	err = runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventID: "hook", EventKind: "state", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: "attempt-one", Provider: "codex", RuntimeAgentID: "worker", SessionID: "s", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved})
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtimeobs.BindAttempt(project, "attempt-one", "B-1", "/root/controller/pairi", "dispatch", "", map[string]string{"assignment_id": "assignment-one"}, now)
	if err != nil {
		t.Fatal(err)
	}
	missing := CheckReporting(project, "B-1", RoleWorker, "attempt-one", "assignment-one", "", "requirements_started", now)
	if missing.Status != "not_recorded" || !missing.ActionRequired || len(missing.Arguments) == 0 {
		t.Fatalf("real omission not actionable: %+v", missing)
	}
	rows, err := (Ledger{Project: project}).List("B-1")
	if err != nil || len(rows) != 0 {
		t.Fatal("reporting check synthesized activity")
	}
	event := Event{SchemaVersion: 1, EventID: "real-start", TaskID: "B-1", Action: Start, Role: RoleWorker, AgentID: "worker", RuntimeAttemptID: "attempt-one", AssignmentID: "assignment-one", PhaseEvent: "implementation_started", Type: Implementation, Confidence: Inferred, Evidence: "actual phase started", OccurredAt: now, RecordedAt: now}
	if _, err = (Ledger{Project: project}).Apply(event); err != nil {
		t.Fatal(err)
	}
	recorded := CheckReporting(project, "B-1", RoleWorker, "attempt-one", "assignment-one", "", "requirements_started", now)
	if recorded.Status != "recorded" || recorded.ActionRequired {
		t.Fatalf("actual report not connected: %+v", recorded)
	}
	wrong := CheckReporting(project, "B-1", RoleWorker, "absent-attempt", "assignment-one", "", "requirements_started", now)
	if wrong.Status != "unverified" || len(wrong.Arguments) > 0 {
		t.Fatalf("fake actor offered reporting attribution: %+v", wrong)
	}
}
