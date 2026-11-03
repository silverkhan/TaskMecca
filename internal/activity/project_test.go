package activity

import (
    "testing"
    "time"
)

func TestSummarizeProjectMeasuredCoverage(t *testing.T){
    base:=time.Date(2026,11,3,0,0,0,0,time.UTC)
    event:=func(id,task,agent string,at time.Time,action Action)Event{
        return Event{SchemaVersion:1,EventID:id,TaskID:task,AgentID:agent,RuntimeAttemptID:agent,Role:RoleWorker,Type:Implementation,Action:action,OccurredAt:at,RecordedAt:at,Confidence:Confirmed}
    }
    events:=[]Event{
        event("1","B-1","w1",base,Start),event("2","B-1","w1",base.Add(2*time.Minute),Stop),
        event("3","B-2","w2",base,Start), // unclosed, must not count as zero-time completion
        event("4","B-3","w3",base,Start),event("5","B-3","w3",base.Add(8*time.Minute),Stop),
    }
    p:=SummarizeProject(events)
    if p.TasksObserved!=3||p.TasksWithMeasuredTime!=2||p.TasksWithOpenActivities!=1{t.Fatalf("coverage %+v",p)}
    if p.AgentTimeMillis!=10*60*1000||p.MedianAgentMillis!=2*60*1000||p.P90AgentMillis!=8*60*1000{t.Fatalf("time %+v",p)}
}
