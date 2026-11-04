package activity

import (
 "testing"
 "time"
)

func TestProjectFirstReviewAndReworkKPIs(t *testing.T){
 base:=time.Date(2026,11,4,0,0,0,0,time.UTC)
 mk:=func(id,task,agent string,kind Type,action Action,round,minute int,role Role)Event{
  e:=validEvent()
  e.EventID=id;e.TaskID=task;e.AgentID=agent;e.RuntimeAttemptID="attempt-"+agent
  e.Type=kind;e.Action=action;e.Role=role;e.Round=round
  e.OccurredAt=base.Add(time.Duration(minute)*time.Minute);e.RecordedAt=e.OccurredAt
  return e
 }
 a1:=mk("a1","B-1","controller1",ControllerReview,Start,0,0,RoleController)
 a2:=mk("a2","B-1","controller1",ControllerReview,Stop,0,5,RoleController)
 a2.ReviewOutcome="accepted"
 b1:=mk("b1","B-2","controller2",ControllerReview,Start,0,0,RoleController)
 b2:=mk("b2","B-2","controller2",ControllerReview,Stop,0,2,RoleController)
 b2.ReviewOutcome="rejected"
 rw1:=mk("rw1","B-2","worker2",Implementation,Start,1,2,RoleWorker)
 rw2:=mk("rw2","B-2","worker2",Implementation,Stop,1,7,RoleWorker)
 re1:=mk("re1","B-2","controller2",ControllerReReview,Start,1,8,RoleController)
 re2:=mk("re2","B-2","controller2",ControllerReReview,Stop,1,9,RoleController)
 re2.ReviewOutcome="accepted"
 report:=SummarizeProjectCompact([]Event{a1,a2,b1,b2,rw1,rw2,re1,re2})
 if report.FirstReviewDecidedTasks!=2||report.FirstReviewAcceptedTasks!=1||report.FirstPassAcceptancePct==nil||*report.FirstPassAcceptancePct!=50{
  t.Fatalf("first pass report = %+v",report)
 }
 if report.ReworkedTasks!=1||report.ReworkWorkerMillis!=5*60000||report.ControllerReviewMillis!=8*60000{
  t.Fatalf("rework/review report = %+v",report)
 }
}

func TestProjectFirstReviewWithoutEvidenceHasNoRate(t *testing.T){
 report:=SummarizeProjectCompact([]Event{validEvent()})
 if report.FirstReviewDecidedTasks!=0||report.FirstPassAcceptancePct!=nil{
  t.Fatalf("missing review should be unknown not 0pct: %+v",report)
 }
}
