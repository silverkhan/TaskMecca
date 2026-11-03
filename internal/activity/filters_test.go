package activity

import (
 "errors"
 "testing"
 "time"
)

func TestFilteredProjectRolesRoundsAndClipping(t *testing.T){
 at:=time.Date(2026,11,4,0,0,0,0,time.UTC)
 event:=func(id,task,agent string,role Role,typ Type,round int,minute int,action Action)Event{
  e:=validEvent();e.EventID=id;e.TaskID=task;e.AgentID=agent;e.RuntimeAttemptID="run-"+agent
  e.Role=role;e.Type=typ;e.Round=round;e.Action=action
  e.OccurredAt=at.Add(time.Duration(minute)*time.Minute)
  e.RecordedAt=e.OccurredAt
  return e
 }
 events:=[]Event{
  event("a","B-1","worker1",RoleWorker,Implementation,0,0,Start),
  event("b","B-1","worker1",RoleWorker,Implementation,0,10,Stop),
  event("c","B-1","worker2",RoleWorker,Implementation,1,5,Start),
  event("d","B-1","worker2",RoleWorker,Implementation,1,15,Stop),
  event("e","B-2","controller",RoleController,ControllerReview,0,0,Start),
  event("f","B-2","controller",RoleController,ControllerReview,0,4,Stop),
 }
 filter:=SegmentFilter{Role:RoleWorker,Type:Implementation}
 got,err:=SummarizeFilteredProject(events,filter)
 if err!=nil{t.Fatal(err)}
 if got.AgentTimeMillis!=20*60000||got.SumTaskObservedWallMillis!=15*60000||got.MatchedTasks!=1||got.MatchedSegments!=2{
  t.Fatalf("unexpected filtered aggregation %+v",got)
 }
 round:=1
 start:=at.Add(6*time.Minute)
 end:=at.Add(12*time.Minute)
 only:=SegmentFilter{Role:RoleWorker,Round:&round,From:&start,Until:&end}
 cut,err:=SummarizeFilteredProject(events,only)
 if err!=nil{t.Fatal(err)}
 if cut.AgentTimeMillis!=6*60000||cut.MatchedSegments!=1{
  t.Fatalf("wrong clipped interval: %+v",cut)
 }
}
func TestInvalidActivityFilters(t *testing.T){
 if _,err:=SummarizeFilteredProject(nil,SegmentFilter{Role:"root"});!errors.Is(err,ErrInvalidSegmentFilter){t.Fatal(err)}
 n:=-1
 if _,err:=SummarizeFilteredProject(nil,SegmentFilter{Round:&n});!errors.Is(err,ErrInvalidSegmentFilter){t.Fatal(err)}
}
