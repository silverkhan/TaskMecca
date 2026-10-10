package webui

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/notify"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type completionFixture struct {
	project, taskPath, assignmentID, attemptID, incidentID string
	base                                                   time.Time
}

func newCompletionFixture(t *testing.T) completionFixture {
	return newCompletionFixtureIn(t, testOperationProject(t))
}

func newCompletionFixtureIn(t *testing.T, project string) completionFixture {
	t.Helper()
	f := completionFixture{project: project, assignmentID: "assignment-completion", base: time.Now().UTC().Add(-time.Hour)}
	a, err := runtimeobs.RecordAssignment(f.project, f.assignmentID, "A-5", "/root/controller/worker", f.base)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := runtimeobs.BindRuntimeAgentForAssignment(f.project, a.AssignmentID, "worker", "", f.base.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	f.attemptID = attempt.AttemptID
	journal, err := scanOperationProject(f.project, f.base.Add(2*time.Second))
	if err != nil || len(journal.Incidents) != 1 {
		t.Fatalf("initial incident: %+v %v", journal, err)
	}
	f.incidentID = journal.Incidents[0].ID
	f.taskPath = filepath.Join(f.project, "_task_mecca", "data", "backlog", "archive", "2026-10", "000005.A-5.completed.done.md")
	if err := os.MkdirAll(filepath.Dir(f.taskPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.taskPath, []byte("# A-5 Completed\n- Agent: /root/controller/worker\n## 결과\nVerified result\n## 검증\nRegression verified\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = backlog.RecordLifecycleTransition(f.project, backlog.LifecycleTransition{EventID: "completed-fixture", TaskID: "A-5", Kind: "completed", Actor: "/root/controller", EvidenceSource: "controller_verified", EvidenceRef: "verified regression and integration", AssignmentID: f.assignmentID, AttemptID: f.attemptID, OccurredAt: f.base.Add(10 * time.Second).Format(time.RFC3339Nano)}, f.base.Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestA23BuildIsolatedUIFixture(t *testing.T) {
	project := os.Getenv("TASK_MECCA_A23_UI_FIXTURE_ROOT")
	if project == "" {
		t.Skip("explicit isolated UI fixture only")
	}
	if !strings.HasPrefix(project, "/private/tmp/a23-raichyu-") {
		t.Fatal("unexpected UI fixture path")
	}
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca", "data", "backlog"), 0755); err != nil {
		t.Fatal(err)
	}
	f := newCompletionFixtureIn(t, project)
	if _, err := ReconcileCompletedOperation(project, f.incidentID, f.base.Add(11*time.Second), false); err != nil {
		t.Fatal(err)
	}
	started := testOperationHook(t, project, `{"session_id":"isolated-a23","turn_id":"fixture-error","hook_event_name":"SubagentStart","agent_id":"error-worker"}`, f.base.Add(12*time.Second))
	if _, err := runtimeobs.BindAttempt(project, started.AttemptID, "A-6", "/root/controller/error-worker", "explicit", "", nil, f.base.Add(13*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: f.base.Add(14 * time.Second).Format(time.RFC3339Nano), AttemptID: started.AttemptID, State: runtimeobs.StateErrored, Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.RecordAssignment(project, "assignment-unverified", "A-7", "/root/controller/unverified", f.base.Add(15*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.BindRuntimeAgentForAssignment(project, "assignment-unverified", "unverified-worker", "", f.base.Add(16*time.Second)); err != nil {
		t.Fatal(err)
	}
	journal, err := scanOperationProject(project, f.base.Add(17*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("A23_ISOLATED_UI_FIXTURE project=%s resolved=%s incidents=%d", project, f.incidentID, len(journal.Incidents))
}

func TestCompletedUnknownResolutionPreservesRuntimeAndHistoryWithoutDelivery(t *testing.T) {
	f := newCompletionFixture(t)
	now := f.base.Add(11 * time.Second)
	before, _ := os.ReadFile(operationJournalPath(f.project))
	ledgerBefore, _ := runtimeobs.BuildLedger(f.project, 3, now)
	oldDelivery := operationDeliver
	operationDeliver = func(string, []notify.Event) []error {
		t.Fatal("reconciliation attempted notification delivery")
		return nil
	}
	defer func() { operationDeliver = oldDelivery }()
	preview, err := ReconcileCompletedOperation(f.project, f.incidentID, now, true)
	if err != nil || preview["changed"] != true || preview["notifications_sent"] != false {
		t.Fatalf("preview=%v err=%v", preview, err)
	}
	afterPreview, _ := os.ReadFile(operationJournalPath(f.project))
	if !bytes.Equal(before, afterPreview) {
		t.Fatal("dry-run modified journal")
	}
	if _, err := ReconcileCompletedOperation(f.project, f.incidentID, now, false); err != nil {
		t.Fatal(err)
	}
	journal, _ := readOperationJournal(f.project)
	item := journal.Incidents[0]
	if item.RecoveredAt == "" || item.Resolution == nil || item.Kind != "runtime_unknown" || item.Evidence != "runtime identity or hook execution unverified" || item.EndedAt != "" {
		t.Fatalf("original evidence lost or fabricated: %+v", item)
	}
	if item.Resolution.EventID != "completed-fixture" || item.Resolution.AssignmentID != f.assignmentID || item.Resolution.TaskPath != f.taskPath {
		t.Fatalf("resolution=%+v", item.Resolution)
	}
	again, err := ReconcileCompletedOperation(f.project, f.incidentID, now.Add(time.Second), false)
	if err != nil || again["changed"] != false {
		t.Fatalf("idempotence: %v %v", again, err)
	}
	for i := 0; i < 2; i++ {
		restarted, err := scanOperationProject(f.project, now.Add(time.Duration(i+2)*time.Second))
		if err != nil || len(restarted.Incidents) != 1 || restarted.Incidents[0].ID != f.incidentID {
			t.Fatalf("stale recurrence after restart: %+v %v", restarted, err)
		}
	}
	ledgerAfter, _ := runtimeobs.BuildLedger(f.project, 3, now)
	a, _ := json.Marshal(ledgerBefore.Attempts)
	b, _ := json.Marshal(ledgerAfter.Attempts)
	if !bytes.Equal(a, b) || ledgerAfter.Attempts[0].Terminal || ledgerAfter.Attempts[0].CurrentState != runtimeobs.StateRuntimeUnknown {
		t.Fatal("completion changed runtime facts")
	}
}

func TestCompletedUnknownRequiresUniqueCanonicalMapping(t *testing.T) {
	for _, name := range []string{"missing_task", "duplicate_task", "not_done", "placeholder_result", "missing_completion", "missing_assignment", "duplicate_assignment", "new_assignment", "unverified_completion", "wrong_attempt", "late_hook", "errored", "interrupted", "shutdown"} {
		t.Run(name, func(t *testing.T) {
			f := newCompletionFixture(t)
			switch name {
			case "missing_task":
				if err := os.Remove(f.taskPath); err != nil {
					t.Fatal(err)
				}
			case "duplicate_task":
				data, _ := os.ReadFile(f.taskPath)
				if err := os.WriteFile(filepath.Join(f.project, "_task_mecca", "data", "backlog", "000005.A-5.duplicate.done.md"), data, 0644); err != nil {
					t.Fatal(err)
				}
			case "not_done":
				if err := os.Rename(f.taskPath, strings.Replace(f.taskPath, ".done.md", ".doing.md", 1)); err != nil {
					t.Fatal(err)
				}
			case "placeholder_result":
				data, _ := os.ReadFile(f.taskPath)
				if err := os.WriteFile(f.taskPath, bytes.ReplaceAll(data, []byte("Verified result"), []byte("-")), 0644); err != nil {
					t.Fatal(err)
				}
			case "missing_completion":
				if err := os.Remove(filepath.Join(f.project, "_task_mecca", "data", "lifecycle", "events", "completed-fixture", "event.json")); err != nil {
					t.Fatal(err)
				}
			case "missing_assignment":
				if err := os.Remove(filepath.Join(f.project, "_task_mecca", "data", "assignments", "events.jsonl")); err != nil {
					t.Fatal(err)
				}
			case "duplicate_assignment":
				path := filepath.Join(f.project, "_task_mecca", "data", "assignments", "events.jsonl")
				data, _ := os.ReadFile(path)
				if err := os.WriteFile(path, append(data, data...), 0600); err != nil {
					t.Fatal(err)
				}
			case "new_assignment":
				if _, err := runtimeobs.RecordAssignment(f.project, "assignment-new", "A-5", "/root/controller/worker", f.base.Add(12*time.Second)); err != nil {
					t.Fatal(err)
				}
			case "unverified_completion", "wrong_attempt":
				path := filepath.Join(f.project, "_task_mecca", "data", "lifecycle", "events", "completed-fixture", "event.json")
				data, _ := os.ReadFile(path)
				old, new := []byte("controller_verified"), []byte("worker_report")
				if name == "wrong_attempt" {
					old, new = []byte(f.attemptID), []byte("run-other")
				}
				if err := os.WriteFile(path, bytes.ReplaceAll(data, old, new), 0600); err != nil {
					t.Fatal(err)
				}
			case "late_hook":
				if err := runtimeobs.AppendExecutionEvent(f.project, runtimeobs.ExecutionEvent{EventKind: "activity", ObservedAt: f.base.Add(12 * time.Second).Format(time.RFC3339Nano), AttemptID: f.attemptID, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
					t.Fatal(err)
				}
			default:
				if err := runtimeobs.AppendExecutionEvent(f.project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: f.base.Add(3 * time.Second).Format(time.RFC3339Nano), AttemptID: f.attemptID, State: runtimeobs.CanonicalState(name), Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(operationJournalPath(f.project))
			if _, err := ReconcileCompletedOperation(f.project, f.incidentID, f.base.Add(15*time.Second), false); err == nil {
				t.Fatal("unsafe mapping accepted")
			}
			after, _ := os.ReadFile(operationJournalPath(f.project))
			if !bytes.Equal(before, after) {
				t.Fatal("rejected reconciliation wrote journal")
			}
		})
	}
}

func TestCompletedObservationLateHookCreatesNewWarningNotFakeCompletion(t *testing.T) {
	f := newCompletionFixture(t)
	if _, err := scanOperationProject(f.project, f.base.Add(11*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := runtimeobs.AppendExecutionEvent(f.project, runtimeobs.ExecutionEvent{EventKind: "activity", ObservedAt: f.base.Add(12 * time.Second).Format(time.RFC3339Nano), AttemptID: f.attemptID, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
		t.Fatal(err)
	}
	journal, err := scanOperationProject(f.project, f.base.Add(13*time.Second))
	if err != nil || len(journal.Incidents) != 2 || journal.Incidents[0].Resolution == nil || journal.Incidents[1].RecoveredAt != "" || journal.Incidents[1].Resolution != nil {
		t.Fatalf("late signal hidden: %+v %v", journal, err)
	}
}

func TestMissingRuntimeMappingDoesNotSilentlyRecoverUnknown(t *testing.T) {
	project := testOperationProject(t)
	base := time.Now().UTC()
	incident := operationIncident{ID: "missing", Key: "attempt:missing", Project: project, AttemptID: "missing", TaskID: "A-5", AgentPath: "/root/controller/worker", Kind: "runtime_unknown", Quality: "verification_required", Evidence: "runtime identity or hook execution unverified", DetectedAt: base.Format(time.RFC3339Nano), LastObservedAt: base.Format(time.RFC3339Nano)}
	if err := writeOperationJournal(project, operationJournal{Version: 1, Project: project, Incidents: []operationIncident{incident}}); err != nil {
		t.Fatal(err)
	}
	journal, err := scanOperationProject(project, base.Add(time.Second))
	if err != nil || journal.Incidents[0].RecoveredAt != "" {
		t.Fatalf("missing mapping concealed: %+v %v", journal, err)
	}
}

func TestCompletedTaskStillProjectsActualRuntimeError(t *testing.T) {
	f := newCompletionFixture(t)
	if err := runtimeobs.AppendExecutionEvent(f.project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: f.base.Add(3 * time.Second).Format(time.RFC3339Nano), AttemptID: f.attemptID, State: runtimeobs.StateErrored, Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
		t.Fatal(err)
	}
	journal, err := scanOperationProject(f.project, f.base.Add(11*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, incident := range journal.Incidents {
		if incident.Kind == "errored" && incident.RecoveredAt == "" && incident.Resolution == nil {
			found = true
		}
	}
	if !found {
		t.Fatalf("done concealed confirmed error: %+v", journal.Incidents)
	}
}

func TestResolutionRejectsUnrelatedProjectTaskOrAgent(t *testing.T) {
	f := newCompletionFixture(t)
	ledger, err := runtimeobs.BuildLedger(f.project, 3, f.base.Add(11*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	proof := operationCompletionEvidence(f.project, ledger, f.base.Add(11*time.Second))
	journal, _ := readOperationJournal(f.project)
	for _, scope := range []string{"project", "task", "agent", "attempt"} {
		incident := journal.Incidents[0]
		switch scope {
		case "project":
			incident.Project = t.TempDir()
		case "task":
			incident.TaskID = "A-6"
		case "agent":
			incident.AgentPath = "/root/controller/other"
		case "attempt":
			incident.AttemptID = "run-other"
		}
		if operationCompletedUnknown(incident, proof) != nil {
			t.Fatalf("unrelated %s scope resolved", scope)
		}
	}
}
