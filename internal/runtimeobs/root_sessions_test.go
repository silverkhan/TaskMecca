package runtimeobs

import (
    "context"
    "errors"
    "os"
    "strings"
    "testing"
    "time"
)

func TestBuildRootSessionsGroupsByProviderSessionAndPrefersProviderName(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,10,12,0,0,0,time.UTC)
    events:=[]ExecutionEvent{
        {
            EventKind:"state",ObservedAt:now.Add(-time.Minute).Format(time.RFC3339Nano),
            AttemptID:"run-a1",Provider:"codex",SessionID:"session-a",SessionName:"EMP 전략 개선",
            RuntimeAgentID:"agent-a1",AgentPath:"/root/controller/꼬부기",
            State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
        },
        {
            EventKind:"state",ObservedAt:now.Add(-90*time.Minute).Format(time.RFC3339Nano),
            AttemptID:"run-a2",Provider:"codex",SessionID:"session-a",
            RuntimeAgentID:"agent-a2",AgentPath:"/root/controller/파이리",
            State:StateCompleted,Terminal:true,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
        },
        {
            EventKind:"state",ObservedAt:now.Add(-time.Hour).Format(time.RFC3339Nano),
            AttemptID:"run-b1",Provider:"codex",SessionID:"session-b",SessionTitle:"다른 Root",
            RuntimeAgentID:"agent-b1",AgentPath:"/root/controller/꼬부기",
            State:StateCompleted,Terminal:true,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
        },
    }
    for _,event:=range events {
        if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    }
    ledger,err:=ReconcileLedger(project,10,now)
    if err!=nil { t.Fatal(err) }
    roots,err:=BuildRootSessions(project,ledger,now)
    if err!=nil { t.Fatal(err) }
    if roots.Total!=2 { t.Fatalf("roots=%+v",roots) }

    bySession:=map[string]RootSession{}
    for _,root:=range roots.Items { bySession[root.ProviderSessionID]=root }
    first:=bySession["session-a"]
    second:=bySession["session-b"]
    if first.DisplayName!="EMP 전략 개선" || first.DisplayNameSource!="provider_name" {
        t.Fatalf("first name=%+v",first)
    }
    if first.AgentCount!=2 || first.CurrentCount!=1 || first.TerminalCount!=1 || first.Status!=RootSessionActive {
        t.Fatalf("first=%+v",first)
    }
    if second.DisplayName!="다른 Root" || second.DisplayNameSource!="provider_title" {
        t.Fatalf("second=%+v",second)
    }
    if first.RootSessionID==second.RootSessionID {
        t.Fatalf("different provider sessions must not share root id: %s",first.RootSessionID)
    }
}

func TestRootSessionFallbackAndSevenDayCleanupEligibility(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,20,12,0,0,0,time.UTC)
    ended:=now.Add(-8*24*time.Hour)
    event:=ExecutionEvent{
        EventKind:"state",ObservedAt:ended.Format(time.RFC3339Nano),
        AttemptID:"run-old",Provider:"claude",SessionID:"session-old",
        RuntimeAgentID:"agent-old",State:StateCompleted,Terminal:true,
        EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
    }
    if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    ledger,err:=ReconcileLedger(project,10,now)
    if err!=nil { t.Fatal(err) }
    roots,err:=BuildRootSessions(project,ledger,now)
    if err!=nil { t.Fatal(err) }
    if len(roots.Items)!=1 { t.Fatalf("roots=%+v",roots) }
    root:=roots.Items[0]
    if root.DisplayNameSource!="task_mecca_fallback" || root.DisplayName=="" {
        t.Fatalf("fallback=%+v",root)
    }
    if root.Status!=RootSessionInactiveTerminal || !root.CleanupEligible {
        t.Fatalf("cleanup eligibility=%+v",root)
    }
}

func TestRootSessionNeedsCheckIsNeverCleanupEligible(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,20,12,0,0,0,time.UTC)
    old:=now.Add(-10*24*time.Hour)
    event:=ExecutionEvent{
        EventKind:"activity",ObservedAt:old.Format(time.RFC3339Nano),
        AttemptID:"run-unknown",Provider:"codex",SessionID:"session-unknown",
        RuntimeAgentID:"agent-unknown",EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
    }
    if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    ledger,err:=ReconcileLedger(project,10,now)
    if err!=nil { t.Fatal(err) }
    roots,err:=BuildRootSessions(project,ledger,now)
    if err!=nil { t.Fatal(err) }
    root:=roots.Items[0]
    if root.Status!=RootSessionNeedsCheck || root.CleanupEligible {
        t.Fatalf("unknown root must be protected: %+v",root)
    }
    preview,err:=PreviewRootSessionCleanup(project,ledger,root.RootSessionID,now)
    if err!=nil { t.Fatal(err) }
    if preview.Eligible || preview.Reason=="" {
        t.Fatalf("preview=%+v",preview)
    }
}

func TestCleanupRootSessionRemovesOnlySelectedRoot(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,20,12,0,0,0,time.UTC)
    old:=now.Add(-9*24*time.Hour)
    for _,event:=range []ExecutionEvent{
        {
            EventKind:"state",ObservedAt:old.Format(time.RFC3339Nano),
            AttemptID:"run-delete",Provider:"codex",SessionID:"session-delete",
            RuntimeAgentID:"agent-delete",State:StateCompleted,Terminal:true,
            EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
        },
        {
            EventKind:"state",ObservedAt:old.Add(time.Hour).Format(time.RFC3339Nano),
            AttemptID:"run-keep",Provider:"codex",SessionID:"session-keep",
            RuntimeAgentID:"agent-keep",State:StateCompleted,Terminal:true,
            EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
        },
    } {
        if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    }
    ledger,err:=ReconcileLedger(project,10,now)
    if err!=nil { t.Fatal(err) }
    if _,err:=MaintainExecutionHistory(project,ledger,now); err!=nil { t.Fatal(err) }

    roots,err:=BuildRootSessions(project,ledger,now)
    if err!=nil { t.Fatal(err) }
    deleteID:=""
    for _,root:=range roots.Items {
        if root.ProviderSessionID=="session-delete" { deleteID=root.RootSessionID }
    }
    if deleteID=="" { t.Fatalf("delete root missing: %+v",roots) }

    result,err:=CleanupRootSession(project,ledger,deleteID,now)
    if err!=nil { t.Fatal(err) }
    if result.ReclaimedBytes<=0 {
        t.Fatalf("expected reclaimed bytes: %+v",result)
    }

    fresh,err:=ReconcileLedger(project,10,now)
    if err!=nil { t.Fatal(err) }
    remaining,err:=BuildRootSessions(project,fresh,now)
    if err!=nil { t.Fatal(err) }
    if remaining.Total!=1 || remaining.Items[0].ProviderSessionID!="session-keep" {
        t.Fatalf("remaining roots=%+v",remaining)
    }

    // Cleanup must not delete the runtime directory or unrelated files.
    if _,err:=os.Stat(ExecutionRootPath(project)); err!=nil {
        t.Fatalf("execution root removed: %v",err)
    }
}


func TestClaudeSessionStartExplicitNameFeedsRootSession(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,2,7,0,0,0,time.UTC)
    payload:=strings.NewReader(`{"hook_event_name":"SessionStart","source":"startup","session_id":"claude-root-1","session_title":"Runtime Observability 개선"}`)
    meta,err:=ObserveHook(project,"claude",payload,now)
    if err!=nil { t.Fatal(err) }
    if meta.EventKind!="session_metadata" || meta.SessionName!="Runtime Observability 개선" || meta.SessionTitle!="" {
        t.Fatalf("metadata=%+v",meta)
    }
    start:=ExecutionEvent{
        EventKind:"state",ObservedAt:now.Add(time.Minute).Format(time.RFC3339Nano),
        AttemptID:"run-claude-1",Provider:"claude",SessionID:"claude-root-1",
        RuntimeAgentID:"agent-1",State:StateRunning,
        EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
    }
    if err:=AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    ledger,err:=BuildLedger(project,10,now.Add(2*time.Minute))
    if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=1 { t.Fatalf("session metadata must not create an attempt: %+v",ledger.Attempts) }
    roots,err:=BuildRootSessions(project,ledger,now.Add(2*time.Minute))
    if err!=nil { t.Fatal(err) }
    if len(roots.Items)!=1 { t.Fatalf("roots=%+v",roots.Items) }
    root:=roots.Items[0]
    if root.DisplayName!="Runtime Observability 개선" || root.DisplayNameSource!="provider_name" {
        t.Fatalf("root name=%+v",root)
    }
    if root.CreatedAtSource!="provider_metadata" || root.CreatedAt!=now.Format(time.RFC3339Nano) {
        t.Fatalf("created metadata=%+v",root)
    }
}


func TestRefreshCodexRootNamesThrottlesAndUpdatesSameRoot(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,2,12,0,0,0,time.UTC)
    event:=ExecutionEvent{EventKind:"state",ObservedAt:now.Format(time.RFC3339Nano),AttemptID:"run-refresh",Provider:"codex",SessionID:"session-refresh",RuntimeAgentID:"agent-refresh",State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved}
    if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    ledger,err:=BuildLedger(project,10,now); if err!=nil { t.Fatal(err) }
    original:=resolveCodexThreadMetadata
    calls:=0
    resolveCodexThreadMetadata=func(ctx context.Context,sessionID string)(CodexThreadMetadata,error){ calls++; return CodexThreadMetadata{Name:"새 이름"},nil }
    defer func(){ resolveCodexThreadMetadata=original }()
    codexNameCacheMu.Lock(); codexNameCache=map[string]time.Time{}; codexNameCacheMu.Unlock()

    changed,err:=RefreshCodexRootNames(project,ledger,now); if err!=nil || !changed { t.Fatalf("refresh changed=%v err=%v",changed,err) }
    if calls!=1 { t.Fatalf("calls=%d",calls) }
    if changed,err=RefreshCodexRootNames(project,ledger,now.Add(time.Second)); err!=nil || changed || calls!=1 { t.Fatalf("throttle changed=%v calls=%d err=%v",changed,calls,err) }

    roots,err:=BuildRootSessions(project,ledger,now.Add(2*time.Second)); if err!=nil { t.Fatal(err) }
    if len(roots.Items)!=1 || roots.Items[0].DisplayName!="새 이름" || roots.Items[0].RootSessionID!=rootSessionIDFor("codex","session-refresh") { t.Fatalf("roots=%+v",roots.Items) }
}

func TestRefreshCodexRootNamesUsesShortNegativeTTL(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,2,13,0,0,0,time.UTC)
    event:=ExecutionEvent{EventKind:"state",ObservedAt:now.Format(time.RFC3339Nano),AttemptID:"run-negative",Provider:"codex",SessionID:"session-negative",RuntimeAgentID:"agent-negative",State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved}
    if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    ledger,err:=BuildLedger(project,10,now); if err!=nil { t.Fatal(err) }
    original:=resolveCodexThreadMetadata
    calls:=0
    resolveCodexThreadMetadata=func(ctx context.Context,sessionID string)(CodexThreadMetadata,error){ calls++; return CodexThreadMetadata{},errors.New("temporary") }
    defer func(){ resolveCodexThreadMetadata=original }()
    codexNameCacheMu.Lock(); codexNameCache=map[string]time.Time{}; codexNameCacheMu.Unlock()

    _,_=RefreshCodexRootNames(project,ledger,now)
    _,_=RefreshCodexRootNames(project,ledger,now.Add(5*time.Second))
    if calls!=1 { t.Fatalf("negative cache calls=%d",calls) }
    _,_=RefreshCodexRootNames(project,ledger,now.Add(11*time.Second))
    if calls!=2 { t.Fatalf("retry calls=%d",calls) }
}


func TestProviderMetadataRenameOutranksStaleAttemptName(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,3,1,0,0,0,time.UTC)
    state:=ExecutionEvent{
        EventKind:"state",ObservedAt:now.Add(-time.Minute).Format(time.RFC3339Nano),
        AttemptID:"run-rename",Provider:"codex",SessionID:"session-rename",
        SessionName:"이전 이름",RuntimeAgentID:"agent-rename",
        State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
    }
    if err:=AppendExecutionEvent(project,state); err!=nil { t.Fatal(err) }
    meta:=ExecutionEvent{
        EventKind:"session_metadata",ObservedAt:now.Format(time.RFC3339Nano),
        AttemptID:"rootmeta-"+rootSessionIDFor("codex","session-rename"),
        Provider:"codex",SessionID:"session-rename",SessionName:"변경된 이름",
        Reason:"codex_app_server",EvidenceSource:EvidenceReconciled,ObservationQuality:QualityObserved,
    }
    meta.EventID=eventIDFor(meta)
    if err:=AppendExecutionEvent(project,meta); err!=nil { t.Fatal(err) }

    // Simulate later Root activity that still carries the old session name.
    state.ObservedAt=now.Add(time.Minute).Format(time.RFC3339Nano)
    state.EventID=""
    state.EventID=eventIDFor(state)
    if err:=AppendExecutionEvent(project,state); err!=nil { t.Fatal(err) }

    ledger,err:=BuildLedger(project,10,now.Add(2*time.Minute)); if err!=nil { t.Fatal(err) }
    roots,err:=BuildRootSessions(project,ledger,now.Add(2*time.Minute)); if err!=nil { t.Fatal(err) }
    if len(roots.Items)!=1 { t.Fatalf("roots=%+v",roots.Items) }
    root:=roots.Items[0]
    if root.DisplayName!="변경된 이름" || root.DisplayNameSource!="provider_name" {
        t.Fatalf("display=%q source=%q root=%+v",root.DisplayName,root.DisplayNameSource,root)
    }
    if root.RootSessionID!=rootSessionIDFor("codex","session-rename") {
        t.Fatalf("root id changed: %s",root.RootSessionID)
    }
}


func TestRefreshCodexRootNamesIncludesTerminalRoot(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,3,8,30,0,0,time.UTC)
    old:=now.Add(-7*time.Hour)
    event:=ExecutionEvent{EventKind:"state",ObservedAt:old.Format(time.RFC3339Nano),AttemptID:"run-stale-name",Provider:"codex",SessionID:"session-stale-name",RuntimeAgentID:"agent-stale-name",State:StateCompleted,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved}
    if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    ledger,err:=BuildLedger(project,10,now); if err!=nil { t.Fatal(err) }
    original:=resolveCodexThreadMetadata
    calls:=0
    resolveCodexThreadMetadata=func(ctx context.Context,sessionID string)(CodexThreadMetadata,error){ calls++; return CodexThreadMetadata{Name:"대화 없이 바뀐 이름"},nil }
    defer func(){ resolveCodexThreadMetadata=original }()
    codexNameCacheMu.Lock(); codexNameCache=map[string]time.Time{}; codexNameCacheMu.Unlock()

    changed,err:=RefreshCodexRootNames(project,ledger,now)
    if err!=nil || !changed || calls!=1 { t.Fatalf("changed=%v calls=%d err=%v",changed,calls,err) }
    roots,err:=BuildRootSessions(project,ledger,now); if err!=nil { t.Fatal(err) }
    if len(roots.Items)!=1 || roots.Items[0].DisplayName!="대화 없이 바뀐 이름" { t.Fatalf("roots=%+v",roots.Items) }
}


func TestRootSessionReactivatesWhenFreshActivityArrivesAfterTerminal(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,5,5,0,0,0,time.UTC)
    attemptID:="run-reactivate"
    sessionID:="session-reactivate"
    agentID:="agent-reactivate"
    ended:=now.Add(-time.Hour)
    terminal:=ExecutionEvent{
        EventKind:"state",ObservedAt:ended.Format(time.RFC3339Nano),
        AttemptID:attemptID,Provider:"codex",SessionID:sessionID,
        RuntimeAgentID:agentID,AgentPath:"/root/controller",
        State:StateCompleted,Terminal:true,
        EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
    }
    terminal.EventID=eventIDFor(terminal)
    if err:=AppendExecutionEvent(project,terminal); err!=nil { t.Fatal(err) }
    if _,err:=BindAttempt(project,attemptID,"AID-OLD","/root/controller","explicit","",map[string]string{"dispatch":"old"},ended.Add(-time.Minute)); err!=nil { t.Fatal(err) }

    before,err:=ReconcileLedger(project,10,now.Add(-time.Minute))
    if err!=nil { t.Fatal(err) }
    roots,err:=BuildRootSessions(project,before,now.Add(-time.Minute))
    if err!=nil { t.Fatal(err) }
    if len(roots.Items)!=1 || roots.Items[0].Status!=RootSessionTerminal {
        t.Fatalf("root must initially be previous/terminal: %+v",roots.Items)
    }
    rootID:=roots.Items[0].RootSessionID

    activity:=ExecutionEvent{
        EventKind:"activity",ObservedAt:now.Format(time.RFC3339Nano),
        AttemptID:attemptID,Provider:"codex",SessionID:sessionID,
        RuntimeAgentID:agentID,AgentPath:"/root/controller",
        EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
    }
    activity.EventID=eventIDFor(activity)
    if err:=AppendExecutionEvent(project,activity); err!=nil { t.Fatal(err) }

    after,err:=ReconcileLedger(project,10,now.Add(time.Second))
    if err!=nil { t.Fatal(err) }
    roots,err=BuildRootSessions(project,after,now.Add(time.Second))
    if err!=nil { t.Fatal(err) }
    if len(roots.Items)!=1 { t.Fatalf("roots=%+v",roots.Items) }
    root:=roots.Items[0]
    if root.RootSessionID!=rootID { t.Fatalf("reactivation changed root identity: before=%s after=%s",rootID,root.RootSessionID) }
    if root.Status!=RootSessionActive || root.CurrentCount!=1 || root.TerminalCount!=0 {
        t.Fatalf("fresh activity must reactivate previous root: %+v",root)
    }
    if root.LastActivityAt!=activity.ObservedAt {
        t.Fatalf("last activity=%s want=%s",root.LastActivityAt,activity.ObservedAt)
    }
    if len(after.Attempts)!=1 { t.Fatalf("attempts=%+v",after.Attempts) }
    reopened:=after.Attempts[0]
    if reopened.BindingState!=BindingUnbound || reopened.TaskID!="" {
        t.Fatalf("new runtime episode must not inherit prior task binding: %+v",reopened)
    }
}
