package backlog

import (
    "testing"
    "time"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

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
    end.EventID="";end.ObservedAt=base.Add(2*time.Minute).Format(time.RFC3339Nano);end.State=runtimeobs.StateCompleted;end.Terminal=true
    if err:=runtimeobs.AppendExecutionEvent(project,end);err!=nil{t.Fatal(err)}

    rows:=[]Record{{ID:"AID-39",State:"doing",Location:"active"}}
    got:=runtimeLedgerSignals(project,rows,base.Add(3*time.Minute))
    if got["AID-39"]["health"]!="awaiting_finalize"{t.Fatalf("signal=%v",got["AID-39"])}
    if got["AID-39"]["attempt_id"]!="run-test"{t.Fatalf("attempt=%v",got["AID-39"])}
}

func TestRuntimeLedgerSignalsIgnoreUnboundAttempt(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,3,1,0,0,0,time.UTC)
    event:=runtimeobs.ExecutionEvent{EventKind:"state",ObservedAt:base.Format(time.RFC3339Nano),AttemptID:"run-unbound",Provider:"codex",RuntimeAgentID:"worker-1",State:runtimeobs.StateCompleted,Terminal:true,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved}
    if err:=runtimeobs.AppendExecutionEvent(project,event);err!=nil{t.Fatal(err)}
    rows:=[]Record{{ID:"AID-39",State:"doing",Location:"active"}}
    if got:=runtimeLedgerSignals(project,rows,base.Add(time.Minute));len(got)!=0{t.Fatalf("unbound attempt leaked into backlog signal: %v",got)}
}

func TestControlTowerSupportsLedgerRecoveryStates(t *testing.T) {
    row:=Record{ID:"AID-39",State:"doing"}
    interrupted,_:=canonicalOperationalState(row,nil,map[string]any{"health":"execution_interrupted"})
    if interrupted["type"]!="execution_interrupted"{t.Fatalf("reason=%v",interrupted)}
    unknown,_:=canonicalOperationalState(row,nil,map[string]any{"health":"runtime_unknown"})
    if unknown["type"]!="runtime_unknown"{t.Fatalf("reason=%v",unknown)}
}
