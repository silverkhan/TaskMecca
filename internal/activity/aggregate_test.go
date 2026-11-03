package activity

import (
    "testing"
    "time"
)

func TestSummarizeParallelAndRounds(t *testing.T){
    base:=time.Date(2026,11,3,9,0,0,0,time.UTC)
    mk:=func(id,agent string,minute int,action Action,kind Type,round int)Event{
        e:=validEvent()
        e.EventID=id;e.AgentID=agent;e.RuntimeAttemptID="attempt-"+agent
        e.Action=action;e.Type=kind;e.Round=round
        e.OccurredAt=base.Add(time.Duration(minute)*time.Minute);e.RecordedAt=e.OccurredAt
        return e
    }
    records:=[]Event{
        mk("w-start","w1",0,Start,Requirements,0),
        mk("w-switch","w1",7,Switch,Implementation,0),
        mk("w-stop","w1",21,Stop,Implementation,0),
        mk("b-start","w2",10,Start,Implementation,1),
        mk("b-stop","w2",30,Stop,Implementation,1),
    }
    // Intentionally unsorted input: aggregation sorts by actual timestamps.
    records[0],records[4]=records[4],records[0]
    got:=Summarize("B-589",records)
    if got.AgentTimeMillis!=41*60*1000 {t.Fatalf("agent total = %d",got.AgentTimeMillis)}
    if got.ObservedWallMillis!=30*60*1000{t.Fatalf("union total = %d",got.ObservedWallMillis)}
    if got.InitialWorkerMillis!=21*60*1000||got.ReworkWorkerMillis!=20*60*1000{t.Fatalf("round totals = %+v",got)}
    if got.OpenActivities!=0||got.InvalidTransitions!=0{t.Fatalf("unexpected gaps = %+v",got)}
    if len(got.Segments)!=3{t.Fatalf("segments = %d",len(got.Segments))}
}
func TestSummarizeNeverFabricatesOpenTime(t *testing.T){
    e:=validEvent()
    got:=Summarize(e.TaskID,[]Event{e})
    if got.AgentTimeMillis!=0||got.ObservedWallMillis!=0||got.OpenActivities!=1 {
        t.Fatalf("unclosed interval must remain unknown: %+v",got)
    }
}
