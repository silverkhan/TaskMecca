package main

import (
 "testing"
 "time"
 "github.com/silverkhan/TaskMecca/internal/activity"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestActivityCLIAutoRoundAfterExplicitRejection(t *testing.T){
 project:=t.TempDir()
 now:=time.Now().UTC()
 _,err:=runtimeobs.RecordAssignment(project,"assignment-round","B-589","/root/controller/worker-round",now)
 if err!=nil{t.Fatal(err)}
 err=runtimeobs.AppendExecutionEvent(project,runtimeobs.ExecutionEvent{
  EventID:"round-hook-start",EventKind:"state",AttemptID:"run-round",ObservedAt:now.Format(time.RFC3339Nano),
  Provider:"codex",RuntimeAgentID:"worker-round",State:runtimeobs.StateRunning,
  EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
 })
 if err!=nil{t.Fatal(err)}
 _,err=runtimeobs.BindAttempt(project,"run-round","B-589","/root/controller/worker-round","dispatch","",
  map[string]string{"assignment_id":"assignment-round"},now.Add(time.Millisecond))
 if err!=nil{t.Fatal(err)}
 rejected:=activity.Event{
  SchemaVersion:1,EventID:"review-reject",TaskID:"B-589",Action:activity.Stop,
  Role:activity.RoleController,AgentID:"c",RuntimeAttemptID:"run-c",
  Type:activity.ControllerReview,Round:0,Confidence:activity.Confirmed,
  ReviewOutcome:"rejected",OccurredAt:now.Add(-time.Second),RecordedAt:now.Add(-time.Second),
 }
 if _,err=(activity.Ledger{Project:project}).Append(rejected);err!=nil{t.Fatal(err)}
 base:=[]string{"B-589","--project",project,"--role","worker","--agent","worker-round",
  "--attempt","run-round","--assignment","assignment-round","--evidence","phase reported","--json"}
 invoke:=func(cmd,id,kind string)map[string]any{
  t.Helper();args:=append([]string{cmd},base...)
  args=append(args,"--event-id",id)
  if kind!=""{args=append(args,"--type",kind)}
  status,out,msg:=invokeActivity(t,args...)
  if status!=0{t.Fatalf("%s status=%d msg=%s",cmd,status,msg)}
  return out
 }
 start:=invoke("start","round-start","requirements")
 ev:=start["event"].(map[string]any)
 if ev["round"]!=float64(1){t.Fatalf("expected round1 from rejection: %+v",ev)}
 switched:=invoke("switch","round-switch","implementation")
 if switched["event"].(map[string]any)["round"]!=float64(1){t.Fatalf("switch lost round: %+v",switched)}
 stopped:=invoke("stop","round-stop","")
 if stopped["event"].(map[string]any)["round"]!=float64(1){t.Fatalf("stop lost round: %+v",stopped)}
 retried:=invoke("stop","round-stop","")
 if retried["recorded"]!=false{t.Fatalf("stop retry should be idempotent: %+v",retried)}
}
