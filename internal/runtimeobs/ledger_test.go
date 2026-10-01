package runtimeobs

import (
    "strings"
    "testing"
    "time"
)

func hookExecutionEvent(t *testing.T,provider,payload string,at time.Time) ExecutionEvent {
    t.Helper()
    spike,err:=ParseHookEvent(provider,strings.NewReader(payload),at)
    if err!=nil { t.Fatal(err) }
    event,err:=HookToExecutionEvent(spike)
    if err!=nil { t.Fatal(err) }
    return event
}

func TestExecutionLedgerFoldsStartActivityStop(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
    events:=[]ExecutionEvent{
        hookExecutionEvent(t,"codex",`{"session_id":"s1","turn_id":"t1","hook_event_name":"SubagentStart","agent_id":"a1","agent_type":"worker"}`,base),
        hookExecutionEvent(t,"codex",`{"session_id":"s1","turn_id":"t1","hook_event_name":"PreToolUse","agent_id":"a1","agent_type":"worker","tool_name":"Bash","tool_use_id":"tool-1"}`,base.Add(10*time.Second)),
        hookExecutionEvent(t,"codex",`{"session_id":"s1","turn_id":"t1","hook_event_name":"PostToolUse","agent_id":"a1","agent_type":"worker","tool_name":"Bash","tool_use_id":"tool-1"}`,base.Add(20*time.Second)),
        hookExecutionEvent(t,"codex",`{"session_id":"s1","turn_id":"t1","hook_event_name":"SubagentStop","agent_id":"a1","agent_type":"worker"}`,base.Add(30*time.Second)),
    }
    for _,event:=range events { if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) } }
    ledger,err:=BuildLedger(project,10,base.Add(time.Minute))
    if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=1 { t.Fatalf("attempts=%+v",ledger.Attempts) }
    got:=ledger.Attempts[0]
    if got.CurrentState!=StateCompleted || !got.Terminal { t.Fatalf("state=%+v",got) }
    if got.ActivityCount!=2 { t.Fatalf("activity=%d",got.ActivityCount) }
    if got.ElapsedMillis!=30000 { t.Fatalf("elapsed=%d",got.ElapsedMillis) }
    if got.ActiveTimeAvailable || got.ObservedActiveMillis!=nil { t.Fatalf("hook-only active time must remain unavailable: %+v",got) }
    if got.BindingState!=BindingUnbound { t.Fatalf("binding=%s",got.BindingState) }
}

func TestSameAgentIDAcrossSessionsProducesDistinctAttempts(t *testing.T) {
    project:=t.TempDir(); base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
    for i,session:=range []string{"s1","s2"} {
        event:=hookExecutionEvent(t,"claude","{\"session_id\":\""+session+"\",\"hook_event_name\":\"SubagentStart\",\"agent_id\":\"a1\"}",base.Add(time.Duration(i)*time.Minute))
        if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    }
    ledger,err:=BuildLedger(project,10,base.Add(2*time.Minute)); if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=2 { t.Fatalf("attempts=%+v",ledger.Attempts) }
    if ledger.Attempts[0].AttemptID==ledger.Attempts[1].AttemptID { t.Fatal("attempt ids must differ across sessions") }
}

func TestBindAttemptAndConflictBecomesAmbiguous(t *testing.T) {
    project:=t.TempDir(); base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
    start:=hookExecutionEvent(t,"codex",`{"session_id":"s","hook_event_name":"SubagentStart","agent_id":"a"}`,base)
    if err:=AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    bound,err:=BindAttempt(project,start.AttemptID,"AID-41","/root/controller/pairi","explicit","",map[string]string{"source":"test"},base.Add(time.Second))
    if err!=nil { t.Fatal(err) }
    if bound.BindingState!=BindingBound || bound.TaskID!="AID-41" || bound.AgentPath!="/root/controller/pairi" { t.Fatalf("bound=%+v",bound) }
    conflicted,err:=BindAttempt(project,start.AttemptID,"AID-42","/root/controller/pairi","explicit","",nil,base.Add(2*time.Second))
    if err!=nil { t.Fatal(err) }
    if conflicted.BindingState!=BindingAmbiguous { t.Fatalf("binding=%+v",conflicted) }
}

func TestMissingStopBecomesStaleNotDead(t *testing.T) {
    project:=t.TempDir(); t.Setenv("TASK_MECCA_STALE_WARN_SECONDS","60")
    base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
    start:=hookExecutionEvent(t,"claude",`{"session_id":"s","hook_event_name":"SubagentStart","agent_id":"a"}`,base)
    if err:=AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    ledger,err:=ReconcileLedger(project,10,base.Add(2*time.Minute)); if err!=nil { t.Fatal(err) }
    got:=ledger.Attempts[0]
    if got.CurrentState!=StateRunning || got.Terminal { t.Fatalf("must not infer death: %+v",got) }
    stale:=false
    for _,finding:=range ledger.Findings {
        if finding.AttemptID==got.AttemptID && finding.Code=="stale" { stale=true }
        if finding.AttemptID==got.AttemptID && (finding.Code=="interrupted" || finding.Code=="dead") { t.Fatalf("unexpected death inference: %+v",finding) }
    }
    if !stale { t.Fatalf("findings=%+v",ledger.Findings) }
}

func TestLedgerRebuildDeduplicatesAndSurvivesRestart(t *testing.T) {
    project:=t.TempDir(); base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
    start:=hookExecutionEvent(t,"codex",`{"session_id":"s","hook_event_name":"SubagentStart","agent_id":"a"}`,base)
    if err:=AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    if err:=AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    first,err:=BuildLedger(project,10,base.Add(time.Second)); if err!=nil { t.Fatal(err) }
    second,err:=BuildLedger(project,10,base.Add(time.Second)); if err!=nil { t.Fatal(err) }
    if len(first.Attempts)!=1 || len(second.Attempts)!=1 { t.Fatalf("first=%+v second=%+v",first,second) }
    if first.Attempts[0].EvidenceCount!=1 || second.Attempts[0].EvidenceCount!=1 { t.Fatalf("duplicate event IDs must fold once") }
}

func TestRecentTransitionLimitAndWaitingTime(t *testing.T) {
    project:=t.TempDir(); base:=time.Date(2026,10,1,10,0,0,0,time.UTC); attempt:="run-test"
    states:=[]CanonicalState{StateStarting,StateRunning,StateWaitingApproval,StateRunning,StateWaitingUser,StateRunning}
    for i,state:=range states {
        event:=ExecutionEvent{EventKind:"state",ObservedAt:base.Add(time.Duration(i)*time.Second).Format(time.RFC3339Nano),AttemptID:attempt,Provider:"codex",State:state,EvidenceSource:EvidenceReconciled,ObservationQuality:QualityAuthoritative}
        event.EventID=eventIDFor(event)
        if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    }
    ledger,err:=BuildLedger(project,3,base.Add(10*time.Second)); if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=1 || len(ledger.Attempts[0].RecentTransitions)!=3 { t.Fatalf("transitions=%+v",ledger.Attempts) }
    if ledger.Attempts[0].WaitingMillis!=2000 { t.Fatalf("waiting_ms=%d",ledger.Attempts[0].WaitingMillis) }
}

func TestStopReasonMapsInterruptAndError(t *testing.T) {
    interrupted:=hookExecutionEvent(t,"codex",`{"session_id":"s","hook_event_name":"SubagentStop","agent_id":"a","reason":"cancelled by user"}`,time.Now())
    if interrupted.State!=StateInterrupted { t.Fatalf("state=%s",interrupted.State) }
    errored:=hookExecutionEvent(t,"claude",`{"session_id":"s","hook_event_name":"SubagentStop","agent_id":"b","reason":"API error"}`,time.Now())
    if errored.State!=StateErrored { t.Fatalf("state=%s",errored.State) }
}
