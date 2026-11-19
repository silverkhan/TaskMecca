package backlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestA88B648SameAttemptTestCrossesStaleNotificationTime(t *testing.T) {
	t.Setenv("TASK_MECCA_STALE_WARN_SECONDS", "1800") // fixture threshold; historical environment remains unknown
	project, row, _ := ownerFixture(t, "/root/controller/pairi", "native")
	now := time.Now().UTC()
	ownerAttempt(t, project, "/root/controller/pairi", "native", "b648-run", now.Add(-40*time.Minute), runtimeobs.StateRunning)
	event := activity.Event{SchemaVersion: 1, EventID: "b648-test-start", TaskID: row.ID, Action: activity.Switch, Role: activity.RoleWorker, AgentID: "native", RuntimeAttemptID: "b648-run", AssignmentID: "b648-run-assignment", Type: activity.TargetedTest, Confidence: activity.Inferred, Evidence: "fixture test start eight seconds before notification", Source: "agent_report", PhaseEvent: "targeted_test_started", OccurredAt: now.Add(-8 * time.Second), RecordedAt: now.Add(-8 * time.Second)}
	directory := filepath.Join(project, "_task_mecca", "data", "activity", "events")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(event)
	if err := os.WriteFile(filepath.Join(directory, "b648-test-start.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	signal := runtimeLedgerSignals(project, []Record{row}, now)[row.ID]
	if signal["health"] != "stale" {
		t.Fatal("fixture did not reproduce ledger stale, or report invented active", signal)
	}
	if signal["recent_activity_report"] == nil {
		t.Fatal("exact simultaneous Activity lost", signal)
	}
	_, condition := canonicalOperationalState(row, nil, signal, now)
	if len(condition) != 0 {
		t.Fatal("B648 test was called stopped/missing", condition)
	}
}

func TestA88FutureExecutionAndReassignmentCannotAuthorizeOldOutcome(t *testing.T) {
	for _, name := range []string{"future_execution", "new_assignment"} {
		t.Run(name, func(t *testing.T) {
			project, row, _ := ownerFixture(t, "/root/controller/pairi", "native")
			now := time.Now().UTC()
			at := now.Add(-time.Minute)
			if name == "future_execution" {
				at = now.Add(time.Minute)
			}
			ownerAttempt(t, project, "/root/controller/pairi", "native", "old", at, runtimeobs.StateErrored)
			if name == "new_assignment" {
				if _, err := runtimeobs.RecordAssignment(project, "new", row.ID, "/root/controller/pairi", now); err != nil {
					t.Fatal(err)
				}
			}
			signal := runtimeLedgerSignals(project, []Record{row}, now)[row.ID]
			_, condition := canonicalOperationalState(row, nil, signal, now)
			if len(condition) != 0 {
				t.Fatal("noncurrent outcome emitted", signal, condition)
			}
		})
	}
}

func TestRuntimeLedgerSignalsProjectBoundTerminalAttempt(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,3,1,0,0,0,time.UTC)
    start:=runtimeobs.ExecutionEvent{
        EventKind:"state",ObservedAt:base.Format(time.RFC3339Nano),AttemptID:"run-test",
        Provider:"codex",SessionID:"root-1",RuntimeAgentID:"worker-1",State:runtimeobs.StateRunning,
        EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
    }
    if err:=runtimeobs.AppendExecutionEvent(project,start);err!=nil{t.Fatal(err)}
    if _,err:=runtimeobs.BindAttempt(project,"run-test","AID-39","/root/controller/pairi","explicit","",map[string]string{"dispatch":"test"},base.Add(time.Second));err!=nil{t.Fatal(err)}
    end:=start
    end.EventID=""
	end.ObservedAt=base.Add(2*time.Minute).Format(time.RFC3339Nano)
	end.State=runtimeobs.StateCompleted
	end.Terminal=true
    if err:=runtimeobs.AppendExecutionEvent(project,end);err!=nil{t.Fatal(err)}

    rows:=[]Record{{ID:"AID-39",State:"doing",Location:"active",Fields:map[string]string{"Agent":"/root/controller/pairi"}}}
    got:=runtimeLedgerSignals(project,rows,base.Add(3*time.Minute))
    if got["AID-39"]["health"]!="progress_collection_gap"{t.Fatalf("signal=%v",got["AID-39"])}
    progress,ok:=got["AID-39"]["progress_evidence"].(map[string]any)
    if !ok || progress["phase"]!="assigned_prepared" || progress["technical_crash"]!="unproven" || progress["remaining_stage"]!="worker_START_and_current_phase" {t.Fatalf("finished turn fabricated product result: %v",got["AID-39"])}
    if got["AID-39"]["attempt_id"]!="run-test"{t.Fatalf("attempt=%v",got["AID-39"])}
}

func TestRuntimeLedgerSignalsIgnoreUnboundAttempt(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,3,1,0,0,0,time.UTC)
    event:=runtimeobs.ExecutionEvent{EventKind:"state",ObservedAt:base.Format(time.RFC3339Nano),AttemptID:"run-unbound",Provider:"codex",RuntimeAgentID:"worker-1",State:runtimeobs.StateCompleted,Terminal:true,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved}
    if err:=runtimeobs.AppendExecutionEvent(project,event);err!=nil{t.Fatal(err)}
    rows:=[]Record{{ID:"AID-39",State:"doing",Location:"active",Fields:map[string]string{"Agent":"/root/controller/pairi"}}}
    if got:=runtimeLedgerSignals(project,rows,base.Add(time.Minute));len(got)!=0{t.Fatalf("unbound attempt leaked into backlog signal: %v",got)}
}

func TestControlTowerSupportsLedgerRecoveryStates(t *testing.T) {
    row:=Record{ID:"AID-39",State:"doing"}
    interrupted,_:=canonicalOperationalState(row,nil,map[string]any{"health":"execution_interrupted"})
    if interrupted["type"]!="execution_interrupted"{t.Fatalf("reason=%v",interrupted)}
    unknown,_:=canonicalOperationalState(row,nil,map[string]any{"health":"runtime_unknown"})
    if unknown["type"]!="runtime_unknown"{t.Fatalf("reason=%v",unknown)}
}

func TestAssignedAttemptWithoutHookShowsStartGap(t *testing.T) {
	project := t.TempDir()
	now := time.Date(2026, 10, 6, 0, 21, 53, 0, time.UTC)
	bound, err := runtimeobs.BindRuntimeAgent(project, "worker-446", "B-446", "/root/controller/worker-446", "", now)
	if err != nil {
		t.Fatal(err)
	}
	rows := []Record{{ID: "B-446", State: "doing", Location: "active", Fields: map[string]string{"Agent": "/root/controller/worker-446"}}}
	signal := runtimeLedgerSignals(project, rows, now)["B-446"]
	if signal["health"] != "awaiting_start" || signal["attempt_id"] != bound.AttemptID || signal["binding_at"] == "" {
		t.Fatalf("missing assignment gap evidence: %+v", signal)
	}
	reason, condition := canonicalOperationalState(rows[0], nil, signal)
	if reason["type"] != "awaiting_start" || reason["attempt_id"] != bound.AttemptID || len(condition) != 0 {
		t.Fatalf("gap should be visible without premature push: reason=%+v condition=%+v", reason, condition)
	}
}

func TestAssignmentAndAmbiguousCandidatesRemainVisible(t *testing.T) {
	project := t.TempDir()
	now := time.Date(2026, 10, 6, 0, 21, 53, 0, time.UTC)
	assignment, err := runtimeobs.RecordAssignment(project, "dispatch-b446", "B-446", "/root/controller/shared", now)
	if err != nil {
		t.Fatal(err)
	}
	rows := []Record{{ID: "B-446", State: "doing", Location: "active", Fields: map[string]string{"Agent": "/root/controller/shared"}}}
	signal := runtimeLedgerSignals(project, rows, now)["B-446"]
	if signal["health"] != "assignment_unobserved" || signal["assignment_id"] != assignment.AssignmentID {
		t.Fatalf("missing pre-start assignment: %+v", signal)
	}
	for _, id := range []string{"run-one", "run-two"} {
		if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{
			EventKind: "state", ObservedAt: now.Add(time.Second).Format(time.RFC3339Nano), AttemptID: id,
			Provider: "codex", SessionID: id, RuntimeAgentID: "shared", State: runtimeobs.StateRunning,
			EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved,
		}); err != nil {
			t.Fatal(err)
		}
	}
	signal = runtimeLedgerSignals(project, rows, now.Add(2*time.Second))["B-446"]
	reason, _ := canonicalOperationalState(rows[0], nil, signal)
	if signal["health"] != "binding_ambiguous" || signal["candidate_count"] != 2 || reason["type"] != "binding_ambiguous" {
		t.Fatalf("ambiguous candidates were hidden or guessed: signal=%+v reason=%+v", signal, reason)
	}
}

func TestConflictingBindingEvidenceIsVisibleWithoutGuessing(t *testing.T) {
    project:=t.TempDir()
    now:=time.Now().UTC()
    if _,err:=runtimeobs.RecordAssignment(project,"dispatch-conflict","B-9","/root/controller/shared",now);err!=nil { t.Fatal(err) }
    attemptID:="run-conflict"
    if err:=runtimeobs.AppendExecutionEvent(project,runtimeobs.ExecutionEvent{
        EventKind:"state",ObservedAt:now.Format(time.RFC3339Nano),AttemptID:attemptID,
        Provider:"codex",SessionID:"root-conflict",RuntimeAgentID:"shared",State:runtimeobs.StateRunning,
        EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
    });err!=nil { t.Fatal(err) }
    bindingAt:=now.Add(time.Second).Format(time.RFC3339Nano)
    for _,id:=range []string{"B-1","B-2"} {
        if err:=runtimeobs.AppendExecutionEvent(project,runtimeobs.ExecutionEvent{
            EventKind:"binding",ObservedAt:bindingAt,AttemptID:attemptID,TaskID:id,
            AgentPath:"/root/controller/shared",BindingSource:"explicit",
            EvidenceSource:runtimeobs.EvidenceManualBinding,ObservationQuality:runtimeobs.QualityAuthoritative,
        });err!=nil { t.Fatal(err) }
    }
    rows:=[]Record{{ID:"B-9",State:"doing",Location:"active",Fields:map[string]string{"Agent":"/root/controller/shared"}}}
    signal:=runtimeLedgerSignals(project,rows,now.Add(2*time.Second))["B-9"]
    if signal["health"]!="binding_ambiguous" || signal["candidate_count"]!=1 {
        t.Fatalf("conflicting attempt evidence was hidden: %+v",signal)
    }
}
