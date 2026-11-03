package activity

import (
 "testing"
 "time"
)
func TestProjectLifecycleRollupIncludesUnmeasuredBacklogs(t *testing.T){
 origin:=time.Date(2026,11,4,0,0,0,0,time.UTC)
 fact:=func(task,id,kind string,minute int)LifecycleFact{
  return LifecycleFact{TaskID:task,EventID:id,Kind:kind,OccurredAt:origin.Add(time.Duration(minute)*time.Minute)}
 }
 facts:=map[string][]LifecycleFact{
  "B-1":{fact("B-1","r1","registered",0),fact("B-1","a1","assigned",5),fact("B-1","s1","started",10),fact("B-1","c1","completed",20)},
  "B-2":{fact("B-2","r2","registered",0),fact("B-2","w2","waiting",3),fact("B-2","resume2","resumed",8)},
 }
 report:=SummarizeProjectWithLifecycle(nil,facts)
 if report.TasksObserved!=0 || report.LifecycleCompletedTasks!=1 || report.LifecycleOpenWaitTasks!=0 {
  t.Fatalf("unmeasured lifecycle rollup %+v",report)
 }
 if report.LifecycleWaitMillis!=600000 || report.LifecycleTotalElapsedMillis!=1200000 || report.LifecycleUnattributedMillis!=600000 {
  t.Fatalf("lifecycle sums %+v",report)
 }
}
