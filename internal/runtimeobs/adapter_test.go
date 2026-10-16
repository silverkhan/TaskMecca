package runtimeobs

import (
    "strings"
    "testing"
    "time"
)

func TestObserveHookReusesUniqueLiveAttemptWhenSessionMissing(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,1,10,0,0,0,time.UTC)

    _,err:=ObserveHook(project,"claude",strings.NewReader(`{"session_id":"session-1","hook_event_name":"SubagentStart","agent_id":"agent-1"}`),base)
    if err!=nil { t.Fatal(err) }

    activity,err:=ObserveHook(project,"claude",strings.NewReader(`{"hook_event_name":"PreToolUse","agent_id":"agent-1","tool_name":"Bash","tool_use_id":"tool-1"}`),base.Add(time.Second))
    if err!=nil { t.Fatal(err) }

    ledger,err:=BuildLedger(project,10,base.Add(2*time.Second))
    if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=1 { t.Fatalf("attempts=%+v",ledger.Attempts) }
    if activity.AttemptID!=ledger.Attempts[0].AttemptID { t.Fatalf("activity=%s attempt=%s",activity.AttemptID,ledger.Attempts[0].AttemptID) }
    if ledger.Attempts[0].ActivityCount!=1 { t.Fatalf("activity_count=%d",ledger.Attempts[0].ActivityCount) }
}

func TestObserveHookDoesNotGuessAcrossAmbiguousLiveAttempts(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
    for i,session:=range []string{"session-1","session-2"} {
        _,err:=ObserveHook(project,"claude",strings.NewReader("{\"session_id\":\""+session+"\",\"hook_event_name\":\"SubagentStart\",\"agent_id\":\"agent-1\"}"),base.Add(time.Duration(i)*time.Second))
        if err!=nil { t.Fatal(err) }
    }
    activity,err:=ObserveHook(project,"claude",strings.NewReader(`{"hook_event_name":"PreToolUse","agent_id":"agent-1","tool_name":"Bash","tool_use_id":"tool-x"}`),base.Add(3*time.Second))
    if err!=nil { t.Fatal(err) }

    ledger,err:=BuildLedger(project,10,base.Add(4*time.Second))
    if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=3 { t.Fatalf("ambiguous missing-session activity must remain separate: %+v",ledger.Attempts) }
    if activity.SessionID!="" { t.Fatalf("unexpected session attribution: %+v",activity) }
}

func TestDelayedExactWorkerStartSurvivesUnrelatedControllerActivity(t *testing.T) {
    project := t.TempDir()
    base := time.Date(2026,10,12,10,0,0,0,time.UTC)
    assignment,err:=RecordAssignment(project,"delayed-worker-assignment","A-50","/root/controller/worker",base)
    if err!=nil { t.Fatal(err) }
    controller,err:=ObserveHook(project,"codex",strings.NewReader(`{"session_id":"controller-session","hook_event_name":"SubagentStart","agent_id":"controller-1"}`),base)
    if err!=nil { t.Fatal(err) }
    worker,err:=ObserveHook(project,"codex",strings.NewReader(`{"session_id":"worker-session","hook_event_name":"SubagentStart","agent_id":"worker-1"}`),base.Add(4*time.Minute))
    if err!=nil { t.Fatal(err) }
    bound,err:=BindRuntimeAgentForAssignment(project,assignment.AssignmentID,"worker-1","",base.Add(4*time.Minute))
    if err!=nil { t.Fatal(err) }
    controllerActivity,err:=ObserveHook(project,"codex",strings.NewReader(`{"session_id":"controller-session","hook_event_name":"PreToolUse","agent_id":"controller-1","tool_name":"Read","tool_use_id":"read-1"}`),base.Add(5*time.Minute))
    if err!=nil { t.Fatal(err) }
    ledger,err:=BuildLedger(project,10,base.Add(6*time.Minute))
    if err!=nil { t.Fatal(err) }
    if worker.AttemptID==controller.AttemptID || controllerActivity.AttemptID!=controller.AttemptID || len(ledger.Attempts)!=2 || bound.AttemptID!=worker.AttemptID || bound.TaskID!="A-50" || bound.BindingEvidence["assignment_id"]!=assignment.AssignmentID {
        t.Fatalf("identity/assignment contamination: worker=%+v controller=%+v activity=%+v bound=%+v",worker,controller,controllerActivity,bound)
    }
    for _,attempt:=range ledger.Attempts { if attempt.AttemptID==worker.AttemptID && (attempt.SessionID!="worker-session" || attempt.ActivityCount!=0 || attempt.BindingEvidence["assignment_id"]!=assignment.AssignmentID) { t.Fatalf("Worker identity/activity changed: %+v",attempt) } }
}
