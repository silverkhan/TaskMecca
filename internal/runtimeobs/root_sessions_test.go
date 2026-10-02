package runtimeobs

import (
    "os"
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
