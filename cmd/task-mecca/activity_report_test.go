package main

import (
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/activity"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestActivityReportRecordsRealPhaseTransitionsWithOneCall(t *testing.T){
 project:=t.TempDir()
 now:=time.Now().UTC()
 if _,err:=runtimeobs.RecordAssignment(project,"assignment-report","B-589","/root/controller/worker-report",now);err!=nil{t.Fatal(err)}
 if err:=runtimeobs.AppendExecutionEvent(project,runtimeobs.ExecutionEvent{
  EventID:"report-hook",EventKind:"state",AttemptID:"attempt-report",
  ObservedAt:now.Format(time.RFC3339Nano),Provider:"codex",RuntimeAgentID:"worker-report",
  State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,
  ObservationQuality:runtimeobs.QualityObserved,
 });err!=nil{t.Fatal(err)}
 if _,err:=runtimeobs.BindAttempt(project,"attempt-report","B-589","/root/controller/worker-report",
  "dispatch","",map[string]string{"assignment_id":"assignment-report"},now.Add(time.Millisecond));err!=nil{t.Fatal(err)}
 args:=func(name,id string)[]string{
  return []string{"report","B-589","--project",project,"--role","worker","--agent","worker-report",
   "--attempt","attempt-report","--assignment","assignment-report","--event",name,
   "--event-id",id,"--evidence","observed phase transition","--json"}
 }
 code,resp,msg:=invokeActivity(t,args("requirements_started","phase-requirements")...)
 if code!=0||resp["recorded"]!=true{t.Fatalf("report start %d %+v %s",code,resp,msg)}
 evt:=resp["event"].(map[string]any)
 if evt["action"]!="start"||evt["type"]!="requirements"||evt["phase_event"]!="requirements_started"{
  t.Fatalf("incorrect start %+v",evt)
 }
 code,resp,msg=invokeActivity(t,args("implementation_started","phase-implementation")...)
 if code!=0||resp["event"].(map[string]any)["action"]!="switch"{t.Fatalf("switch %d %+v %s",code,resp,msg)}
 code,resp,msg=invokeActivity(t,args("requirements_started","phase-requirements")...)
 if code!=0||resp["recorded"]!=false{t.Fatalf("duplicate transition %d %+v %s",code,resp,msg)}
 code,resp,msg=invokeActivity(t,args("worker_completed","phase-worker-done")...)
 if code!=0||resp["event"].(map[string]any)["type"]!="implementation"||resp["event"].(map[string]any)["action"]!="stop"{
  t.Fatalf("phase stop %d %+v %s",code,resp,msg)
 }
 rows,err:=(activity.Ledger{Project:project}).List("B-589")
 if err!=nil||len(rows)!=3{t.Fatalf("expected 3 phase events, got %d %v",len(rows),err)}
 summary:=activity.Summarize("B-589",rows)
 if summary.OpenActivities!=0||summary.InvalidTransitions!=0{t.Fatalf("phase accounting %+v",summary)}
}

func TestActivityReportReviewOutcomeNeedsController(t *testing.T){
 store:=activity.Ledger{Project:t.TempDir()}
 now:=time.Now().UTC()
 reviewed:=activity.Event{SchemaVersion:1,EventID:"review-start",TaskID:"B-589",Action:activity.Start,
  Role:activity.RoleController,AgentID:"controller",RuntimeAttemptID:"attempt-review",Type:activity.ControllerReview,
  Confidence:activity.Inferred,Evidence:"review started",OccurredAt:now,RecordedAt:now}
 if _,err:=store.Append(reviewed);err!=nil{t.Fatal(err)}
 resolution,err:=resolveActivityReport(store,"B-589",activity.RoleController,"controller","attempt-review","review_rejected")
 if err!=nil||resolution.Action!=activity.Stop||resolution.ReviewOutcome!="rejected"{
  t.Fatalf("review decision %+v %v",resolution,err)
 }
 if _,err:=resolveActivityReport(store,"B-589",activity.RoleWorker,"controller","attempt-review","review_rejected");err==nil{
  t.Fatal("worker was allowed to report controller rejection")
 }
}
