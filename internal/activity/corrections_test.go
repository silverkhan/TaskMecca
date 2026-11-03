package activity

import (
    "errors"
    "testing"
    "time"
)

func TestCorrectionPreservesOriginalAndChangesTimeAttribution(t *testing.T){
    project:=t.TempDir()
    store:=Ledger{Project:project}
    started:=validEvent()
    started.EventID="analysis-start"
    started.Type=Requirements
    started.OccurredAt=time.Date(2026,11,4,0,0,0,0,time.UTC)
    started.RecordedAt=started.OccurredAt
    changed:=started
    changed.Action=Switch
    changed.EventID="implementation-switch"
    changed.Type=Implementation
    changed.OccurredAt=started.OccurredAt.Add(5*time.Minute)
    changed.RecordedAt=changed.OccurredAt
    stopped:=changed
    stopped.Action=Stop
    stopped.EventID="implementation-stop"
    stopped.OccurredAt=changed.OccurredAt.Add(5*time.Minute)
    stopped.RecordedAt=stopped.OccurredAt
    for _,e:=range []Event{started,changed,stopped}{
        if ok,err:=store.Apply(e);err!=nil||!ok{t.Fatalf("append %v %v",ok,err)}
    }
    correction:=started
    correction.Action=Correct
    correction.EventID="analysis-corrected"
    correction.SupersedesID=started.EventID
    correction.Type=Research
    correction.Source="agent_correction"
    correction.Confidence=Inferred
    correction.Evidence="requirements phase was actually code research"
    correction.RecordedAt=stopped.RecordedAt.Add(time.Second)
    if ok,err:=store.Apply(correction);err!=nil||!ok{t.Fatalf("correct %v %v",ok,err)}
    if ok,err:=store.Apply(correction);err!=nil||ok{t.Fatalf("retry %v %v",ok,err)}
    stored,err:=store.Read(started.EventID)
    if err!=nil||stored.Type!=Requirements{t.Fatalf("original overwritten %+v %v",stored,err)}
    rows,err:=store.List(started.TaskID)
    if err!=nil||len(rows)!=4{t.Fatalf("raw audit history %d %v",len(rows),err)}
    summary:=Summarize(started.TaskID,rows)
    if summary.InvalidTransitions!=0||summary.InvalidCorrections!=0||
       summary.ByType[Research]!=5*60000||summary.ByType[Requirements]!=0||
       summary.AgentTimeMillis!=10*60000 {
       t.Fatalf("corrected summary %+v",summary)
    }
    duplicate:=correction;duplicate.EventID="duplicate-correction"
    if _,err:=store.Apply(duplicate);!errors.Is(err,ErrEventConflict){
        t.Fatalf("multiple correction to same target: %v",err)
    }
    mismatch:=correction;mismatch.EventID="incorrect-ownership";mismatch.SupersedesID=changed.EventID
    mismatch.AgentID="other-agent"
    if _,err:=store.Apply(mismatch);!errors.Is(err,ErrInvalidEvent){
        t.Fatalf("cross-agent correction accepted: %v",err)
    }
}
func TestCorrectionCannotBreakPhaseChronology(t *testing.T){
    store:=Ledger{Project:t.TempDir()}
    at:=time.Date(2026,11,4,0,0,0,0,time.UTC)
    first:=validEvent();first.EventID="first";first.OccurredAt=at;first.RecordedAt=at
    last:=first;last.EventID="last";last.Action=Stop;last.OccurredAt=at.Add(time.Minute);last.RecordedAt=last.OccurredAt
    for _,e:=range []Event{first,last}{if _,err:=store.Apply(e);err!=nil{t.Fatal(err)}}
    corrected:=first;corrected.Action=Correct;corrected.SupersedesID=first.EventID
    corrected.EventID="late-start";corrected.RecordedAt=at.Add(2*time.Minute)
    corrected.OccurredAt=at.Add(3*time.Minute)
    if _,err:=store.Apply(corrected);!errors.Is(err,ErrInvalidEvent){t.Fatalf("invalid chronology accepted: %v",err)}
}
