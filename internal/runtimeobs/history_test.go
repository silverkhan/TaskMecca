package runtimeobs

import (
    "encoding/json"
    "os"
    "testing"
    "time"
)

func terminalFixture(id,provider string,start,end time.Time) []ExecutionEvent {
    return []ExecutionEvent{
        {
            EventKind:"state",ObservedAt:start.UTC().Format(time.RFC3339Nano),
            AttemptID:id,Provider:provider,RuntimeAgentID:id+"-agent",
            State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
        },
        {
            EventKind:"state",ObservedAt:end.UTC().Format(time.RFC3339Nano),
            AttemptID:id,Provider:provider,RuntimeAgentID:id+"-agent",
            State:StateCompleted,Terminal:true,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
        },
    }
}

func appendEventsForHistoryTest(t *testing.T,project string,events []ExecutionEvent) {
    t.Helper()
    for _,event:=range events {
        event.EventID=eventIDFor(event)
        if err:=AppendExecutionEvent(project,event); err!=nil { t.Fatal(err) }
    }
}

func TestVisibleWorkloadAttemptsKeepsAllActiveAndOnlyRecentTerminal(t *testing.T) {
    ledger:=Ledger{}
    for i:=0;i<3;i++ {
        ledger.Attempts=append(ledger.Attempts,Attempt{AttemptID:"active-"+string(rune('a'+i)),CurrentState:StateRunning})
    }
    for i:=0;i<9;i++ {
        ledger.Attempts=append(ledger.Attempts,Attempt{
            AttemptID:"done-"+string(rune('a'+i)),CurrentState:StateCompleted,Terminal:true,
            EndedAt:time.Date(2026,10,2,10,i,0,0,time.UTC).Format(time.RFC3339Nano),
        })
    }
    visible:=VisibleWorkloadAttempts(ledger,6)
    if len(visible)!=9 { t.Fatalf("visible=%d want=9",len(visible)) }
    active:=0; terminal:=0
    for _,attempt:=range visible {
        if attempt.Terminal { terminal++ } else { active++ }
    }
    if active!=3 || terminal!=6 { t.Fatalf("active=%d terminal=%d",active,terminal) }
}

func TestExecutionHistoryPaginationAndCompaction(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,2,12,0,0,0,time.UTC)

    for i:=0;i<8;i++ {
        start:=now.Add(-time.Duration(8-i)*time.Hour)
        appendEventsForHistoryTest(t,project,terminalFixture("run-"+string(rune('a'+i)),"codex",start,start.Add(time.Minute)))
    }
    ledger,err:=BuildLedger(project,10,now)
    if err!=nil { t.Fatal(err) }
    stats,err:=MaintainExecutionHistory(project,ledger,now)
    if err!=nil { t.Fatal(err) }
    if stats.SummariesAdded!=8 { t.Fatalf("summaries=%d",stats.SummariesAdded) }

    page,err:=QueryExecutionHistory(project,ledger,2,3,"","",now)
    if err!=nil { t.Fatal(err) }
    if page.Total!=8 || page.TotalPages!=3 || page.Page!=2 || len(page.Items)!=3 {
        t.Fatalf("page=%+v",page)
    }
}

func TestRetentionMovesOldTerminalRawToSummaryButKeepsOldActive(t *testing.T) {
    project:=t.TempDir()
    now:=time.Date(2026,10,20,12,0,0,0,time.UTC)
    old:=now.AddDate(0,0,-10)

    appendEventsForHistoryTest(t,project,terminalFixture("run-terminal","codex",old,old.Add(time.Minute)))
    active:=ExecutionEvent{
        EventKind:"state",ObservedAt:old.Add(2*time.Minute).Format(time.RFC3339Nano),
        AttemptID:"run-active",Provider:"claude",RuntimeAgentID:"active-agent",
        State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
    }
    active.EventID=eventIDFor(active)
    if err:=AppendExecutionEvent(project,active); err!=nil { t.Fatal(err) }

    ledger,err:=BuildLedger(project,10,now)
    if err!=nil { t.Fatal(err) }
    stats,err:=MaintainExecutionHistory(project,ledger,now)
    if err!=nil { t.Fatal(err) }

    if stats.RawFilesRemoved!=0 {
        t.Fatalf("shared raw day contains active attempt and must be retained: %+v",stats)
    }

    // Put a terminal-only attempt on a different old day so that file can be removed.
    older:=old.AddDate(0,0,-1)
    appendEventsForHistoryTest(t,project,terminalFixture("run-terminal-only","codex",older,older.Add(time.Minute)))
    ledger,err=BuildLedger(project,10,now)
    if err!=nil { t.Fatal(err) }
    stats,err=MaintainExecutionHistory(project,ledger,now)
    if err!=nil { t.Fatal(err) }
    if stats.RawFilesRemoved<1 { t.Fatalf("expected old terminal-only raw file removal: %+v",stats) }

    freshLedger,err:=BuildLedger(project,10,now)
    if err!=nil { t.Fatal(err) }
    activeFound:=false
    for _,attempt:=range freshLedger.Attempts {
        if attempt.AttemptID=="run-active" && !attempt.Terminal { activeFound=true }
    }
    if !activeFound { t.Fatalf("old active attempt disappeared: %+v",freshLedger.Attempts) }

    history,err:=QueryExecutionHistory(project,freshLedger,1,20,"","",now)
    if err!=nil { t.Fatal(err) }
    found:=false
    for _,attempt:=range history.Items {
        if attempt.AttemptID=="run-terminal-only" { found=true }
    }
    if !found { t.Fatalf("compacted terminal attempt missing from history: %+v",history.Items) }
}

func TestLegacyJournalRemainsReadableAlongsideDailyRaw(t *testing.T) {
    project:=t.TempDir()
    if err:=os.MkdirAll(ExecutionRootPath(project),0700); err!=nil { t.Fatal(err) }
    legacy:=ExecutionEvent{
        EventKind:"state",ObservedAt:"2026-10-01T10:00:00Z",AttemptID:"legacy",
        Provider:"codex",State:StateCompleted,Terminal:true,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
    }
    legacy.EventID=eventIDFor(legacy)
    data,err:=jsonMarshalLine(legacy)
    if err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(ExecutionLegacyJournalPath(project),data,0600); err!=nil { t.Fatal(err) }

    current:=ExecutionEvent{
        EventKind:"state",ObservedAt:"2026-10-02T10:00:00Z",AttemptID:"current",
        Provider:"codex",State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
    }
    current.EventID=eventIDFor(current)
    if err:=AppendExecutionEvent(project,current); err!=nil { t.Fatal(err) }

    ledger,err:=BuildLedger(project,10,time.Date(2026,10,2,12,0,0,0,time.UTC))
    if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=2 { t.Fatalf("attempts=%+v",ledger.Attempts) }
}

func jsonMarshalLine(event ExecutionEvent) ([]byte,error) {
    data,err:=json.Marshal(event)
    if err!=nil { return nil,err }
    return append(data,'\n'),nil
}
