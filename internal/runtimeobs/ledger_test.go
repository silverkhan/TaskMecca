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

func TestNewerAuthoritativeBindingSupersedesOlderBinding(t *testing.T) {
    project:=t.TempDir(); base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
    start:=hookExecutionEvent(t,"codex",`{"session_id":"s","hook_event_name":"SubagentStart","agent_id":"a"}`,base)
    if err:=AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    first,err:=BindAttempt(project,start.AttemptID,"AID-41","/root/controller/pairi","explicit","",map[string]string{"source":"test"},base.Add(time.Second))
    if err!=nil { t.Fatal(err) }
    if first.BindingState!=BindingBound || first.TaskID!="AID-41" || first.AgentPath!="/root/controller/pairi" { t.Fatalf("first=%+v",first) }
    corrected,err:=BindAttempt(project,start.AttemptID,"AID-42","/root/controller/kkobugi","explicit","",map[string]string{"source":"correction"},base.Add(2*time.Second))
    if err!=nil { t.Fatal(err) }
    if corrected.BindingState!=BindingBound || corrected.TaskID!="AID-42" || corrected.AgentPath!="/root/controller/kkobugi" {
        t.Fatalf("new authoritative binding must supersede the old one: %+v",corrected)
    }
}

func TestSameTimestampAuthoritativeConflictRemainsAmbiguous(t *testing.T) {
    project:=t.TempDir(); base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
    start:=hookExecutionEvent(t,"codex",`{"session_id":"s","hook_event_name":"SubagentStart","agent_id":"a"}`,base)
    if err:=AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    at:=base.Add(time.Second)
    first:=ExecutionEvent{EventKind:"binding",ObservedAt:at.Format(time.RFC3339Nano),AttemptID:start.AttemptID,TaskID:"AID-41",AgentPath:"/root/controller/pairi",BindingSource:"explicit",EvidenceSource:EvidenceManualBinding,ObservationQuality:QualityAuthoritative}
    first.EventID=eventIDFor(first)
    second:=first
    second.TaskID="AID-42"
    second.AgentPath="/root/controller/kkobugi"
    second.EventID=eventIDFor(second)
    for _,event:=range []ExecutionEvent{first,second} {
        if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    }
    ledger,err:=BuildLedger(project,10,base.Add(2*time.Second)); if err!=nil { t.Fatal(err) }
    got:=ledger.Attempts[0]
    if got.BindingState!=BindingAmbiguous || got.TaskID!="" || got.AgentPath!="" {
        t.Fatalf("same-time authoritative conflict must remain ambiguous: %+v",got)
    }
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


func TestCompletedRuntimeAgentCanResumeSameAttempt(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,2,0,0,0,0,time.UTC)
    sequence:=[]ExecutionEvent{
        hookExecutionEvent(t,"codex",`{"session_id":"s1","turn_id":"t1","hook_event_name":"SubagentStart","agent_id":"controller-1"}`,base),
        hookExecutionEvent(t,"codex",`{"session_id":"s1","turn_id":"t1","hook_event_name":"SubagentStop","agent_id":"controller-1"}`,base.Add(time.Second)),
        hookExecutionEvent(t,"codex",`{"session_id":"s1","turn_id":"t2","hook_event_name":"SubagentStart","agent_id":"controller-1"}`,base.Add(2*time.Second)),
    }
    for _,event:=range sequence {
        if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    }
    ledger,err:=BuildLedger(project,10,base.Add(3*time.Second))
    if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=1 { t.Fatalf("attempts=%+v",ledger.Attempts) }
    got:=ledger.Attempts[0]
    if got.CurrentState!=StateRunning || got.Terminal {
        t.Fatalf("resumed agent must be running, got=%+v",got)
    }
    if got.TurnID!="t2" { t.Fatalf("turn_id=%q",got.TurnID) }
    if got.EndedAt!="" { t.Fatalf("ended_at must be cleared after resume: %+v",got) }
    if got.StartedAt!=base.Add(2*time.Second).Format(time.RFC3339Nano) {
        t.Fatalf("started_at=%q",got.StartedAt)
    }

    stop:=hookExecutionEvent(t,"codex",`{"session_id":"s1","turn_id":"t2","hook_event_name":"SubagentStop","agent_id":"controller-1"}`,base.Add(4*time.Second))
    if err:=AppendExecutionEvent(project,stop); err!=nil { t.Fatal(err) }
    ledger,err=BuildLedger(project,10,base.Add(5*time.Second))
    if err!=nil { t.Fatal(err) }
    got=ledger.Attempts[0]
    if got.CurrentState!=StateCompleted || !got.Terminal {
        t.Fatalf("second stop must complete resumed agent: %+v",got)
    }
}


func TestResumeWithoutTurnIDIsNotDeduplicated(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,2,1,0,0,0,time.UTC)
    startOne:=hookExecutionEvent(t,"codex",`{"session_id":"s1","hook_event_name":"SubagentStart","agent_id":"controller-1"}`,base)
    stopOne:=hookExecutionEvent(t,"codex",`{"session_id":"s1","hook_event_name":"SubagentStop","agent_id":"controller-1"}`,base.Add(time.Second))
    startTwo:=hookExecutionEvent(t,"codex",`{"session_id":"s1","hook_event_name":"SubagentStart","agent_id":"controller-1"}`,base.Add(2*time.Second))
    if startOne.RawSHA256!=startTwo.RawSHA256 { t.Fatal("fixture must use identical raw start payload") }
    if startOne.EventID==startTwo.EventID { t.Fatal("repeated start without turn_id must remain a distinct lifecycle event") }
    for _,event:=range []ExecutionEvent{startOne,stopOne,startTwo} {
        if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    }
    ledger,err:=BuildLedger(project,10,base.Add(3*time.Second)); if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=1 { t.Fatalf("attempts=%+v",ledger.Attempts) }
    got:=ledger.Attempts[0]
    if got.CurrentState!=StateRunning || got.Terminal { t.Fatalf("resume was lost: %+v",got) }
}

func TestDuplicateTerminalStateDoesNotExtendEndedAt(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,2,1,0,0,0,time.UTC)
    start:=ExecutionEvent{EventKind:"state",ObservedAt:base.Format(time.RFC3339Nano),AttemptID:"run-a",Provider:"codex",State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved}
    stopOne:=ExecutionEvent{EventKind:"state",ObservedAt:base.Add(time.Second).Format(time.RFC3339Nano),AttemptID:"run-a",Provider:"codex",State:StateCompleted,Terminal:true,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved}
    stopTwo:=stopOne
    stopTwo.ObservedAt=base.Add(2*time.Second).Format(time.RFC3339Nano)
    for _,event:=range []ExecutionEvent{start,stopOne,stopTwo} {
        event.EventID=eventIDFor(event)
        if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    }
    ledger,err:=BuildLedger(project,10,base.Add(3*time.Second)); if err!=nil { t.Fatal(err) }
    got:=ledger.Attempts[0]
    if got.EndedAt!=stopOne.ObservedAt { t.Fatalf("ended_at=%s want=%s",got.EndedAt,stopOne.ObservedAt) }
}

func TestBindRuntimeAgentRequiresUniqueLiveAttempt(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,3,2,0,0,0,time.UTC)
    start:=ExecutionEvent{EventKind:"state",ObservedAt:base.Format(time.RFC3339Nano),AttemptID:"run-dispatch",Provider:"codex",SessionID:"root-1",RuntimeAgentID:"agent-42",State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved}
    if err:=AppendExecutionEvent(project,start);err!=nil{t.Fatal(err)}
    bound,err:=BindRuntimeAgent(project,"agent-42","AID-39","/root/controller/pairi","",base.Add(time.Second))
    if err!=nil{t.Fatal(err)}
    if bound.BindingState!=BindingBound||bound.TaskID!="AID-39"||bound.BindingSource!="dispatch"{t.Fatalf("bound=%+v",bound)}
    if bound.BindingEvidence["correlation"]!="explicit_dispatch_result"{t.Fatalf("evidence=%v",bound.BindingEvidence)}
}

func TestBindRuntimeAgentRefusesAmbiguousLiveAttempts(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,3,2,0,0,0,time.UTC)
    for _,id:=range []string{"run-a","run-b"} {
        event:=ExecutionEvent{EventKind:"state",ObservedAt:base.Format(time.RFC3339Nano),AttemptID:id,Provider:"codex",SessionID:id,RuntimeAgentID:"agent-shared",State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved}
        if err:=AppendExecutionEvent(project,event);err!=nil{t.Fatal(err)}
    }
    if _,err:=BindRuntimeAgent(project,"agent-shared","AID-39","/root/controller/pairi","",base.Add(time.Second));err==nil{t.Fatal("expected ambiguous binding error")}
}


func TestAuthoritativeBindingAndActivityRecoverMissingStartHook(t *testing.T) {
    project:=t.TempDir()
    activityAt:=time.Date(2026,10,5,7,42,0,0,time.UTC)
    attemptID:="run-missing-start"
    activity:=ExecutionEvent{EventKind:"activity",ObservedAt:activityAt.Format(time.RFC3339Nano),AttemptID:attemptID,Provider:"codex",SessionID:"session-missing-start",RuntimeAgentID:"agent-worker",EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved}
    activity.EventID=eventIDFor(activity)
    if err:=AppendExecutionEvent(project,activity); err!=nil { t.Fatal(err) }
    boundAt:=activityAt.Add(time.Minute)
    if _,err:=BindAttempt(project,attemptID,"B-441","/root/controller/worker","dispatch","",map[string]string{"runtime_agent_id":"agent-worker"},boundAt); err!=nil { t.Fatal(err) }
    ledger,err:=ReconcileLedger(project,20,boundAt.Add(time.Second))
    if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=1 { t.Fatalf("attempts=%+v",ledger.Attempts) }
    got:=ledger.Attempts[0]
    if got.BindingState!=BindingBound || got.TaskID!="B-441" { t.Fatalf("binding=%+v",got) }
    if got.StartedAt!=boundAt.Format(time.RFC3339Nano) || got.CurrentState!=StateRunning {
        t.Fatalf("missing start hook was not recovered from dispatch+activity: %+v",got)
    }
}


func TestReusedWorkerStartCreatesImmutableExecutionEpisode(t *testing.T) {
    project:=t.TempDir()
    t0:=time.Date(2026,10,5,8,0,0,0,time.UTC)
    startPayload:=func(at time.Time) string {
        return fmt.Sprintf(`{"hook_event_name":"SubagentStart","session_id":"root-1","agent_id":"worker-1"}`)
    }
    stopPayload:=`{"hook_event_name":"SubagentStop","session_id":"root-1","agent_id":"worker-1"}`
    first,err:=ObserveHook(project,"codex",strings.NewReader(startPayload(t0)),t0); if err!=nil { t.Fatal(err) }
    if _,err=BindAttempt(project,first.AttemptID,"B-OLD","/root/controller/worker","dispatch","",nil,t0.Add(time.Second)); err!=nil { t.Fatal(err) }
    if _,err=ObserveHook(project,"codex",strings.NewReader(stopPayload),t0.Add(time.Minute)); err!=nil { t.Fatal(err) }

    second,err:=ObserveHook(project,"codex",strings.NewReader(startPayload(t0.Add(2*time.Minute))),t0.Add(2*time.Minute)); if err!=nil { t.Fatal(err) }
    if second.AttemptID==first.AttemptID { t.Fatalf("reused worker must create a new attempt episode: %s",second.AttemptID) }
    if _,err=BindAttempt(project,second.AttemptID,"B-NEW","/root/controller/worker","dispatch","",nil,t0.Add(2*time.Minute+time.Second)); err!=nil { t.Fatal(err) }

    ledger,err:=BuildLedger(project,20,t0.Add(3*time.Minute)); if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=2 { t.Fatalf("attempts=%+v",ledger.Attempts) }
    var old,new Attempt
    for _,a:=range ledger.Attempts { if a.AttemptID==first.AttemptID { old=a }; if a.AttemptID==second.AttemptID { new=a } }
    if !old.Terminal || old.TaskID!="B-OLD" || old.EndedAt=="" { t.Fatalf("completed episode mutated: %+v",old) }
    if new.Terminal || new.TaskID!="B-NEW" || new.CurrentState!=StateRunning { t.Fatalf("new episode invalid: %+v",new) }
    if old.StartedAt==new.StartedAt { t.Fatalf("episodes share lifecycle start: old=%s new=%s",old.StartedAt,new.StartedAt) }
}
