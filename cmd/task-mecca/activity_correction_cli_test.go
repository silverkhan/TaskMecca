package main

import (
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/activity"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestActivityCorrectionCLIRetainsOriginalAndVerifiesActor(t *testing.T){
 project:=t.TempDir()
 now:=time.Now().UTC()
 if _,err:=runtimeobs.RecordAssignment(project,"assign-correct","B-589","/root/controller/worker-correct",now);err!=nil{t.Fatal(err)}
 if err:=runtimeobs.AppendExecutionEvent(project,runtimeobs.ExecutionEvent{
  EventID:"correction-hook",EventKind:"state",AttemptID:"attempt-correct",
  ObservedAt:now.Format(time.RFC3339Nano),Provider:"codex",RuntimeAgentID:"worker-correct",
  State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,
  ObservationQuality:runtimeobs.QualityObserved,
 });err!=nil{t.Fatal(err)}
 if _,err:=runtimeobs.BindAttempt(project,"attempt-correct","B-589","/root/controller/worker-correct",
  "dispatch","",map[string]string{"assignment_id":"assign-correct"},now.Add(time.Millisecond));err!=nil{t.Fatal(err)}
 startArgs:=[]string{"start","B-589","--project",project,"--role","worker","--agent","worker-correct",
  "--attempt","attempt-correct","--assignment","assign-correct",
  "--type","requirements","--event-id","before-correction","--evidence","observed requirements","--json"}
 if code,_,msg:=invokeActivity(t,startArgs...);code!=0{t.Fatalf("start %d %s",code,msg)}
 correctArgs:=[]string{"correct","B-589","--project",project,"--role","worker","--agent","worker-correct",
  "--attempt","attempt-correct","--assignment","assign-correct",
  "--type","research","--supersedes-id","before-correction","--event-id","correct-research",
  "--evidence","actually researched repository before implementation","--json"}
 code,resp,msg:=invokeActivity(t,correctArgs...)
 if code!=0||resp["recorded"]!=true{t.Fatalf("correction %d %+v %s",code,resp,msg)}
 rows,err:=(activity.Ledger{Project:project}).List("B-589")
 if err!=nil||len(rows)!=2{t.Fatalf("history after correction %d %v",len(rows),err)}
 projected,findings:=activity.EffectiveEvents(rows)
 if len(findings)!=0||len(projected)!=1||projected[0].Type!=activity.Research{
  t.Fatalf("correction projection %+v findings %+v",projected,findings)
 }
 original,err:=(activity.Ledger{Project:project}).Read("before-correction")
 if err!=nil||original.Type!=activity.Requirements{t.Fatalf("original changed %+v %v",original,err)}
}
