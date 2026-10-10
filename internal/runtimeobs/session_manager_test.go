package runtimeobs

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestClassifySessionsSeparatesCurrentNeedsCheckAndTerminal(t *testing.T) {
    attempts:=[]Attempt{
        {AttemptID:"run-current",CurrentState:StateRunning},
        {AttemptID:"run-stale",CurrentState:StateRunning},
        {AttemptID:"run-unknown",CurrentState:StateRuntimeUnknown},
        {AttemptID:"run-done",CurrentState:StateCompleted,Terminal:true},
    }
    findings:=[]LedgerFinding{{Code:"stale",AttemptID:"run-stale",Severity:"warning"}}
    groups:=ClassifySessions(attempts,findings)
    if groups.Current!=1 || groups.NeedsCheck!=2 || groups.Terminal!=1 {
        t.Fatalf("groups=%+v",groups)
    }
    if groups.CurrentIDs[0]!="run-current" { t.Fatalf("current=%v",groups.CurrentIDs) }
}

func TestRuntimeStorageCleanupPreservesNonTerminalAndRemovesSafeOldRaw(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,20,12,0,0,0,time.UTC)
    old:=now.AddDate(0,0,-10)

    terminalEvents:=terminalFixture("run-old-terminal","codex",old,old.Add(time.Minute))
    appendEventsForHistoryTest(t,project,terminalEvents)

    active:=ExecutionEvent{
        EventKind:"state",ObservedAt:now.Add(-time.Minute).Format(time.RFC3339Nano),
        AttemptID:"run-active",Provider:"codex",RuntimeAgentID:"active-agent",
        State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
    }
    active.EventID=eventIDFor(active)
    if err:=AppendExecutionEvent(project,active); err!=nil { t.Fatal(err) }

    ledger,err:=BuildLedger(project,10,now)
    if err!=nil { t.Fatal(err) }
    var terminal Attempt
    for _,attempt:=range ledger.Attempts {
        if attempt.AttemptID=="run-old-terminal" { terminal=attempt }
    }
    if terminal.AttemptID=="" { t.Fatalf("terminal attempt missing: %+v",ledger.Attempts) }
    if err:=appendHistoryAttempt(project,terminal,now); err!=nil { t.Fatal(err) }

    report,err:=RuntimeStorageReport(project,ledger,now)
    if err!=nil { t.Fatal(err) }
    if report.Cleanup.RawFiles<1 || report.Cleanup.ReclaimableBytes<=0 {
        t.Fatalf("cleanup preview=%+v report=%+v",report.Cleanup,report)
    }
    if report.ProtectedRaw.Files<1 {
        t.Fatalf("active raw must be protected: %+v",report.ProtectedRaw)
    }

    result,err:=CleanupRuntimeStorage(project,ledger,now)
    if err!=nil { t.Fatal(err) }
    if result.RawFilesRemoved<1 || result.ReclaimedBytes<=0 {
        t.Fatalf("cleanup result=%+v",result)
    }

    afterLedger,err:=BuildLedger(project,10,now)
    if err!=nil { t.Fatal(err) }
    activeFound:=false
    for _,attempt:=range afterLedger.Attempts {
        if attempt.AttemptID=="run-active" && !attempt.Terminal { activeFound=true }
    }
    if !activeFound { t.Fatalf("active attempt evidence was removed: %+v",afterLedger.Attempts) }

    history,err:=QueryExecutionHistory(project,afterLedger,1,20,"","",now)
    if err!=nil { t.Fatal(err) }
    terminalFound:=false
    for _,attempt:=range history.Items {
        if attempt.AttemptID=="run-old-terminal" { terminalFound=true }
    }
    if !terminalFound { t.Fatalf("terminal summary missing after cleanup: %+v",history.Items) }
}

func TestRuntimeStorageCleanupProtectsOldRuntimeUnknown(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,20,12,0,0,0,time.UTC)
    old:=now.AddDate(0,0,-10)
    unknown:=ExecutionEvent{
        EventKind:"activity",ObservedAt:old.Format(time.RFC3339Nano),
        AttemptID:"run-unknown",Provider:"claude",RuntimeAgentID:"unknown-agent",
        EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
    }
    unknown.EventID=eventIDFor(unknown)
    if err:=AppendExecutionEvent(project,unknown); err!=nil { t.Fatal(err) }
    ledger,err:=ReconcileLedger(project,10,now)
    if err!=nil { t.Fatal(err) }

    report,err:=RuntimeStorageReport(project,ledger,now)
    if err!=nil { t.Fatal(err) }
    if report.Cleanup.RawFiles!=0 {
        t.Fatalf("runtime_unknown raw must not be cleanable: %+v",report.Cleanup)
    }
    if report.ProtectedRaw.Files!=1 {
        t.Fatalf("unknown raw should be protected: %+v",report.ProtectedRaw)
    }
}

func TestHistoryRetentionRecordsEnforcesPhysicalAttemptLimit(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,20,12,0,0,0,time.UTC)
    dir:=ExecutionHistoryDir(project)
    if err:=os.MkdirAll(dir,0700); err!=nil { t.Fatal(err) }
    path:=filepath.Join(dir,"2026-10.jsonl")
    payload:=[]byte{}
    for i:=0;i<historyMaxAttempts+2;i++ {
        ended:=now.Add(-time.Duration(i)*time.Minute)
        record:=HistoryRecord{
            Version:1,ArchivedAt:now.Format(time.RFC3339Nano),
            Attempt:Attempt{
                AttemptID:fmt.Sprintf("run-%04d",i),Provider:"codex",
                CurrentState:StateCompleted,Terminal:true,EndedAt:ended.Format(time.RFC3339Nano),
            },
        }
        line,err:=json.Marshal(record); if err!=nil { t.Fatal(err) }
        payload=append(payload,line...)
        payload=append(payload,'\n')
    }
    if err:=os.WriteFile(path,payload,0600); err!=nil { t.Fatal(err) }

    retained,removed,err:=historyRetentionRecords(project,Ledger{},now)
    if err!=nil { t.Fatal(err) }
    if len(retained)!=historyMaxAttempts || removed!=2 {
        t.Fatalf("retained=%d removed=%d",len(retained),removed)
    }
}
