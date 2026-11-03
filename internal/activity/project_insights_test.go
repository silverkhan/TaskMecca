package activity

import (
 "testing"
 "time"
)
func TestProjectTopTasksUseGroundedDurationBasis(t *testing.T){
 base:=time.Date(2026,11,4,0,0,0,0,time.UTC)
 first:=validEvent()
 first.TaskID="B-1";first.EventID="b1-start";first.OccurredAt=base;first.RecordedAt=base
 done:=first;done.EventID="b1-stop";done.Action=Stop;done.OccurredAt=base.Add(3*time.Minute);done.RecordedAt=done.OccurredAt
 mk:=func(id,task,kind string,minute int)LifecycleFact{
  return LifecycleFact{EventID:id,TaskID:task,Kind:kind,OccurredAt:base.Add(time.Duration(minute)*time.Minute)}
 }
 facts:=map[string][]LifecycleFact{
  "B-1":{mk("r1","B-1","registered",0),mk("c1","B-1","completed",20)},
  "B-2":{mk("r2","B-2","registered",0),mk("c2","B-2","completed",40)},
 }
 report:=SummarizeProjectWithLifecycle([]Event{first,done},facts)
 if len(report.TopTasks)!=2{t.Fatalf("expected 2 tasks, got %+v",report.TopTasks)}
 if report.TopTasks[0].TaskID!="B-2"||report.TopTasks[0].DurationMillis!=40*60000||
   report.TopTasks[0].DurationBasis!="registered_completed"{t.Fatalf("wrong longest task: %+v",report.TopTasks)}
 if report.TopTasks[1].TaskID!="B-1"||report.TopTasks[1].AgentMillis!=3*60000{
  t.Fatalf("wrong smaller task: %+v",report.TopTasks[1])
 }
}
