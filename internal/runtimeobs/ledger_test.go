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
    project:=t.TempDir()
	base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
    for i,session:=range []string{"s1","s2"} {
        event:=hookExecutionEvent(t,"claude","{\"session_id\":\""+session+"\",\"hook_event_name\":\"SubagentStart\",\"agent_id\":\"a1\"}",base.Add(time.Duration(i)*time.Minute))
        if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    }
    ledger,err:=BuildLedger(project,10,base.Add(2*time.Minute))
	if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=2 { t.Fatalf("attempts=%+v",ledger.Attempts) }
    if ledger.Attempts[0].AttemptID==ledger.Attempts[1].AttemptID { t.Fatal("attempt ids must differ across sessions") }
}

func TestNewerAuthoritativeBindingSupersedesOlderBinding(t *testing.T) {
    project:=t.TempDir()
	base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
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
    project:=t.TempDir()
	base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
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
    ledger,err:=BuildLedger(project,10,base.Add(2*time.Second))
	if err!=nil { t.Fatal(err) }
    got:=ledger.Attempts[0]
    if got.BindingState!=BindingAmbiguous || got.TaskID!="" || got.AgentPath!="" {
        t.Fatalf("same-time authoritative conflict must remain ambiguous: %+v",got)
    }
}

func TestMissingStopBecomesStaleNotDead(t *testing.T) {
    project:=t.TempDir()
	t.Setenv("TASK_MECCA_STALE_WARN_SECONDS","60")
    base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
    start:=hookExecutionEvent(t,"claude",`{"session_id":"s","hook_event_name":"SubagentStart","agent_id":"a"}`,base)
    if err:=AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    ledger,err:=ReconcileLedger(project,10,base.Add(2*time.Minute))
	if err!=nil { t.Fatal(err) }
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
    project:=t.TempDir()
	base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
    start:=hookExecutionEvent(t,"codex",`{"session_id":"s","hook_event_name":"SubagentStart","agent_id":"a"}`,base)
    if err:=AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    if err:=AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    first,err:=BuildLedger(project,10,base.Add(time.Second))
	if err!=nil { t.Fatal(err) }
    second,err:=BuildLedger(project,10,base.Add(time.Second))
	if err!=nil { t.Fatal(err) }
    if len(first.Attempts)!=1 || len(second.Attempts)!=1 { t.Fatalf("first=%+v second=%+v",first,second) }
    if first.Attempts[0].EvidenceCount!=1 || second.Attempts[0].EvidenceCount!=1 { t.Fatalf("duplicate event IDs must fold once") }
}

func TestBindingAtSurvivesRecentTransitionTruncation(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,6,0,0,0,0,time.UTC)
    attempt:="run-binding-at"
    start:=ExecutionEvent{
        EventKind:"state",ObservedAt:base.Format(time.RFC3339Nano),
        AttemptID:attempt,Provider:"codex",RuntimeAgentID:"worker-binding-at",
        State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
    }
    if err:=AppendExecutionEvent(project,start);err!=nil{t.Fatal(err)}
    bindingAt:=base.Add(time.Second)
    if _,err:=BindAttempt(project,attempt,"B-930","/root/controller/worker-binding-at","dispatch","",nil,bindingAt);err!=nil{t.Fatal(err)}

    // Push the binding transition out of RecentTransitions. BindingAt must
    // remain available as folded canonical state, not depend on the UI history window.
    states:=[]CanonicalState{StateWaitingApproval,StateRunning,StateWaitingUser,StateRunning,StateWaitingApproval,StateRunning}
    for i,state:=range states{
        event:=ExecutionEvent{
            EventKind:"state",ObservedAt:base.Add(time.Duration(i+2)*time.Second).Format(time.RFC3339Nano),
            AttemptID:attempt,Provider:"codex",RuntimeAgentID:"worker-binding-at",
            State:state,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
        }
        if err:=AppendExecutionEvent(project,event);err!=nil{t.Fatal(err)}
    }
    ledger,err:=BuildLedger(project,3,base.Add(20*time.Second))
	if err!=nil{t.Fatal(err)}
    if len(ledger.Attempts)!=1{t.Fatalf("attempts=%+v",ledger.Attempts)}
    got:=ledger.Attempts[0]
    if got.BindingAt!=bindingAt.Format(time.RFC3339Nano){
        t.Fatalf("binding_at=%q want=%q attempt=%+v",got.BindingAt,bindingAt.Format(time.RFC3339Nano),got)
    }
    for _,transition:=range got.RecentTransitions{
        if transition.Kind=="binding"{t.Fatalf("fixture did not truncate binding transition: %+v",got.RecentTransitions)}
    }
}

func TestRecentTransitionLimitAndWaitingTime(t *testing.T) {
    project:=t.TempDir()
	base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
	attempt:="run-test"
    states:=[]CanonicalState{StateStarting,StateRunning,StateWaitingApproval,StateRunning,StateWaitingUser,StateRunning}
    for i,state:=range states {
        event:=ExecutionEvent{EventKind:"state",ObservedAt:base.Add(time.Duration(i)*time.Second).Format(time.RFC3339Nano),AttemptID:attempt,Provider:"codex",State:state,EvidenceSource:EvidenceReconciled,ObservationQuality:QualityAuthoritative}
        event.EventID=eventIDFor(event)
        if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    }
    ledger,err:=BuildLedger(project,3,base.Add(10*time.Second))
	if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=1 || len(ledger.Attempts[0].RecentTransitions)!=3 { t.Fatalf("transitions=%+v",ledger.Attempts) }
    if ledger.Attempts[0].WaitingMillis!=2000 { t.Fatalf("waiting_ms=%d",ledger.Attempts[0].WaitingMillis) }
}

func TestStopReasonMapsInterruptAndError(t *testing.T) {
    interrupted:=hookExecutionEvent(t,"codex",`{"session_id":"s","hook_event_name":"SubagentStop","agent_id":"a","reason":"cancelled by user"}`,time.Now())
    if interrupted.State!=StateInterrupted { t.Fatalf("state=%s",interrupted.State) }
    errored:=hookExecutionEvent(t,"claude",`{"session_id":"s","hook_event_name":"SubagentStop","agent_id":"b","reason":"API error"}`,time.Now())
    if errored.State!=StateErrored { t.Fatalf("state=%s",errored.State) }
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
    ledger,err:=BuildLedger(project,10,base.Add(3*time.Second))
	if err!=nil { t.Fatal(err) }
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
        return `{"hook_event_name":"SubagentStart","session_id":"root-1","agent_id":"worker-1"}`
    }
    stopPayload:=`{"hook_event_name":"SubagentStop","session_id":"root-1","agent_id":"worker-1"}`
    first,err:=ObserveHook(project,"codex",strings.NewReader(startPayload(t0)),t0)
	if err!=nil { t.Fatal(err) }
    if _,err=BindAttempt(project,first.AttemptID,"B-OLD","/root/controller/worker","dispatch","",nil,t0.Add(time.Second)); err!=nil { t.Fatal(err) }
    if _,err=ObserveHook(project,"codex",strings.NewReader(stopPayload),t0.Add(time.Minute)); err!=nil { t.Fatal(err) }

    second,err:=ObserveHook(project,"codex",strings.NewReader(startPayload(t0.Add(2*time.Minute))),t0.Add(2*time.Minute))
	if err!=nil { t.Fatal(err) }
    if second.AttemptID==first.AttemptID { t.Fatalf("reused worker must create a new attempt episode: %s",second.AttemptID) }
    if _,err=BindAttempt(project,second.AttemptID,"B-NEW","/root/controller/worker","dispatch","",nil,t0.Add(2*time.Minute+time.Second)); err!=nil { t.Fatal(err) }

    ledger,err:=BuildLedger(project,20,t0.Add(3*time.Minute))
	if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=2 { t.Fatalf("attempts=%+v",ledger.Attempts) }
    var old,new Attempt
    for _,a:=range ledger.Attempts { if a.AttemptID==first.AttemptID { old=a }
		if a.AttemptID==second.AttemptID { new=a } }
    if !old.Terminal || old.TaskID!="B-OLD" || old.EndedAt=="" { t.Fatalf("completed episode mutated: %+v",old) }
    if new.Terminal || new.TaskID!="B-NEW" || new.CurrentState!=StateRunning { t.Fatalf("new episode invalid: %+v",new) }
    if old.StartedAt==new.StartedAt { t.Fatalf("episodes share lifecycle start: old=%s new=%s",old.StartedAt,new.StartedAt) }
}


func TestReusedWorkerActivityWithoutStartCreatesNewEpisode(t *testing.T) {
    project:=t.TempDir()
    t0:=time.Date(2026,10,5,12,0,0,0,time.UTC)
    start:=`{"hook_event_name":"SubagentStart","session_id":"root-1","agent_id":"worker-1"}`
    stop:=`{"hook_event_name":"SubagentStop","session_id":"root-1","agent_id":"worker-1"}`
    activity:=`{"hook_event_name":"PreToolUse","session_id":"root-1","agent_id":"worker-1","tool_name":"shell","tool_use_id":"new-work"}`
    first,err:=ObserveHook(project,"codex",strings.NewReader(start),t0)
	if err!=nil { t.Fatal(err) }
    if _,err=BindAttempt(project,first.AttemptID,"B-OLD","/root/controller/worker","dispatch","",nil,t0.Add(time.Second)); err!=nil { t.Fatal(err) }
    if _,err=ObserveHook(project,"codex",strings.NewReader(stop),t0.Add(time.Minute)); err!=nil { t.Fatal(err) }
    second,err:=ObserveHook(project,"codex",strings.NewReader(activity),t0.Add(2*time.Minute))
	if err!=nil { t.Fatal(err) }
    if second.AttemptID==first.AttemptID { t.Fatal("post-terminal activity must create a new immutable episode") }
    if _,err=BindAttempt(project,second.AttemptID,"B-NEW","/root/controller/worker","dispatch","",nil,t0.Add(2*time.Minute+time.Second)); err!=nil { t.Fatal(err) }
    ledger,err:=BuildLedger(project,20,t0.Add(3*time.Minute))
	if err!=nil { t.Fatal(err) }
    var old,new Attempt
    for _,a:=range ledger.Attempts { if a.AttemptID==first.AttemptID { old=a }
		if a.AttemptID==second.AttemptID { new=a } }
    if !old.Terminal || old.TaskID!="B-OLD" || old.EndedAt=="" { t.Fatalf("completed episode mutated: %+v",old) }
    if new.Terminal || new.TaskID!="B-NEW" || new.StartedAt=="" || new.CurrentState!=StateRunning { t.Fatalf("new activity episode invalid: %+v",new) }
}

func TestDispatchBeforeActivitySplitsTerminalWorkerEpisode(t *testing.T) {
    project:=t.TempDir()
    t0:=time.Date(2026,10,5,13,0,0,0,time.UTC)
    start:=`{"hook_event_name":"SubagentStart","session_id":"root-1","agent_id":"worker-1"}`
    stop:=`{"hook_event_name":"SubagentStop","session_id":"root-1","agent_id":"worker-1"}`
    activity:=`{"hook_event_name":"PreToolUse","session_id":"root-1","agent_id":"worker-1","tool_name":"shell","tool_use_id":"after-dispatch"}`
    first,err:=ObserveHook(project,"codex",strings.NewReader(start),t0)
	if err!=nil { t.Fatal(err) }
    if _,err=BindAttempt(project,first.AttemptID,"B-OLD","/root/controller/worker","dispatch","",nil,t0.Add(time.Second)); err!=nil { t.Fatal(err) }
    if _,err=ObserveHook(project,"codex",strings.NewReader(stop),t0.Add(time.Minute)); err!=nil { t.Fatal(err) }
    second,err:=BindRuntimeAgent(project,"worker-1","B-NEW","/root/controller/worker","",t0.Add(2*time.Minute))
	if err!=nil { t.Fatal(err) }
    if second.AttemptID==first.AttemptID { t.Fatal("dispatch after terminal must allocate a new episode") }
    observed,err:=ObserveHook(project,"codex",strings.NewReader(activity),t0.Add(2*time.Minute+time.Second))
	if err!=nil { t.Fatal(err) }
    if observed.AttemptID!=second.AttemptID { t.Fatalf("activity routed to %s, want %s",observed.AttemptID,second.AttemptID) }
    ledger,err:=BuildLedger(project,20,t0.Add(3*time.Minute))
	if err!=nil { t.Fatal(err) }
    var old,new Attempt
    for _,a:=range ledger.Attempts { if a.AttemptID==first.AttemptID { old=a }
		if a.AttemptID==second.AttemptID { new=a } }
    if !old.Terminal || old.TaskID!="B-OLD" { t.Fatalf("old episode mutated: %+v",old) }
    if new.TaskID!="B-NEW" || new.StartedAt=="" || new.CurrentState!=StateRunning { t.Fatalf("new episode did not recover start: %+v",new) }
}


func TestReusedWorkerDispatchWaitsForExecutionEvidence(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,5,8,23,0,0,time.UTC)
    oldID:="run-old"
    start:=ExecutionEvent{EventKind:"state",ObservedAt:now.Add(-10*time.Minute).Format(time.RFC3339Nano),AttemptID:oldID,Provider:"codex",SessionID:"root-1",RuntimeAgentID:"worker-1",State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved}
    if err:=AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    if _,err:=BindAttempt(project,oldID,"B-442","/root/controller/raichyu","dispatch","",nil,now.Add(-9*time.Minute)); err!=nil { t.Fatal(err) }
    stop:=ExecutionEvent{EventKind:"state",ObservedAt:now.Add(-time.Minute).Format(time.RFC3339Nano),AttemptID:oldID,Provider:"codex",SessionID:"root-1",RuntimeAgentID:"worker-1",State:StateCompleted,Terminal:true,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved}
    if err:=AppendExecutionEvent(project,stop); err!=nil { t.Fatal(err) }

    fresh,err:=BindRuntimeAgent(project,"worker-1","B-443","/root/controller/raichyu","",now)
    if err!=nil { t.Fatal(err) }
    if fresh.AttemptID==oldID { t.Fatal("reused worker mutated completed attempt") }
    if fresh.TaskID!="B-443" || fresh.StartedAt !="" || fresh.CurrentState!= StateRuntimeUnknown || fresh.Terminal {
        t.Fatalf("reused worker dispatch manufactured start: %+v",fresh)
    }
    ledger,err:=BuildLedger(project,20,now)
	if err!=nil { t.Fatal(err) }
    var old Attempt
    for _,attempt:=range ledger.Attempts { if attempt.AttemptID==oldID { old=attempt } }
    if !old.Terminal || old.TaskID!="B-442" { t.Fatalf("completed history changed: %+v",old) }
}

func TestDispatchBeforeFirstHookKeepsOneBoundAttempt(t *testing.T) {
	project := t.TempDir()
	assignedAt := time.Date(2026, 10, 6, 0, 21, 53, 0, time.UTC)
	bound, err := BindRuntimeAgent(project, "worker-446", "B-446", "/root/controller/worker-446", "", assignedAt)
	if err != nil {
		t.Fatal(err)
	}
	if bound.BindingState != BindingBound || bound.TaskID != "B-446" || bound.StartedAt != "" {
		t.Fatalf("dispatch must be bound but not started: %+v", bound)
	}
	hookAt := assignedAt.Add(91 * time.Second)
	spike, err := ParseHookEvent("codex", strings.NewReader(`{"session_id":"root-446","turn_id":"turn-1","hook_event_name":"SubagentStart","agent_id":"worker-446","agent_type":"worker"}`), hookAt)
	if err != nil {
		t.Fatal(err)
	}
	event, err := HookToExecutionEvent(spike)
	if err != nil {
		t.Fatal(err)
	}
	event.AttemptID = resolveAttemptID(project, spike)
	if event.AttemptID != bound.AttemptID {
		t.Fatalf("hook split assignment into a new attempt: %s != %s", event.AttemptID, bound.AttemptID)
	}
	if err := AppendExecutionEvent(project, event); err != nil {
		t.Fatal(err)
	}
	ledger, err := BuildLedger(project, 20, hookAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Attempts) != 1 {
		t.Fatalf("expected one attempt: %+v", ledger.Attempts)
	}
	attempt := ledger.Attempts[0]
	if attempt.TaskID != "B-446" || attempt.StartedAt != hookAt.Format(time.RFC3339Nano) || attempt.SessionID != "root-446" {
		t.Fatalf("late hook did not enrich assigned attempt: %+v", attempt)
	}
}

func TestLiveDispatchCannotBeSilentlyReassigned(t *testing.T) {
	project := t.TempDir()
	now := time.Date(2026, 10, 6, 1, 0, 0, 0, time.UTC)
	first, err := BindRuntimeAgent(project, "worker-reused", "B-446", "/root/controller/worker-reused", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BindRuntimeAgent(project, "worker-reused", "B-447", "/root/controller/worker-reused", "", now.Add(time.Minute)); err == nil {
		t.Fatal("live attempt was silently reassigned")
	}
	ledger, err := BuildLedger(project, 20, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Attempts) != 1 || ledger.Attempts[0].AttemptID != first.AttemptID || ledger.Attempts[0].TaskID != "B-446" {
		t.Fatalf("failed reassignment changed original binding: %+v", ledger.Attempts)
	}
}

func TestRepeatedDispatchBindingIsIdempotent(t *testing.T) {
	project := t.TempDir()
	now := time.Date(2026, 10, 6, 1, 0, 0, 0, time.UTC)
	first, err := BindRuntimeAgent(project, "worker-retry", "B-448", "/root/controller/worker-retry", "", now)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := BindRuntimeAgent(project, "worker-retry", "B-448", "/root/controller/worker-retry", "", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if repeated.AttemptID != first.AttemptID || repeated.BindingAt != first.BindingAt || repeated.EvidenceCount != first.EvidenceCount {
		t.Fatalf("retry changed binding: first=%+v repeated=%+v", first, repeated)
	}
}
