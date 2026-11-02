package handoff

import (
 "strings"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestRedispatchPreservesOriginalAndIdempotent(t *testing.T) {
 project:=t.TempDir()
 writeSimpleTask(t,project,"A-1")
 now:=time.Date(2026,11,2,8,0,0,0,time.UTC)
 appendControllerAttempt(t,project,"controller-old","session-1","agent-1",runtimeobs.StateCompleted,now)
 p,err:=Prepare(PrepareRequest{Project:project,TaskID:"A-1",EventType:EventRegistrationReady,SourceAgentPath:"/root/registrar",TargetAgentPath:"/root/controller",TargetAttemptID:"controller-old",ExecutionAuthorized:true},now.Add(time.Second))
 if err!=nil{t.Fatal(err)}
 if _,err:=Mark(project,p.HandoffID,"dispatch","ok","native first delivery",now.Add(2*time.Second));err!=nil{t.Fatal(err)}
 appendControllerAttempt(t,project,"controller-new","session-1","agent-1",runtimeobs.StateRunning,now.Add(3*time.Second))
 p2,err:=Prepare(PrepareRequest{Project:project,TaskID:"A-1",EventType:EventRegistrationReady,SourceAgentPath:"/root/registrar",TargetAgentPath:"/root/controller",TargetAttemptID:"controller-new",ExecutionAuthorized:true},now.Add(4*time.Second))
 if err!=nil{t.Fatal(err)}
 if p.HandoffID!=p2.HandoffID||!p2.Duplicate {t.Fatalf("not retargeted: %+v",p2)}
 if _,err:=Redispatch(project,p.HandoffID,"controller-old","native redelivery",now.Add(5*time.Second));err==nil{t.Fatal("stale target permitted")}
 receipt,err:=Redispatch(project,p.HandoffID,"controller-new","native redelivery",now.Add(5*time.Second))
 if err!=nil||!receipt.Recorded||receipt.Claimed||receipt.RedispatchCount!=1 {t.Fatalf("%+v %v",receipt,err)}
 again,err:=Redispatch(project,p.HandoffID,"controller-new","native redelivery",now.Add(6*time.Second))
 if err!=nil||!again.Duplicate||again.RedispatchCount!=1{t.Fatalf("%+v %v",again,err)}
 row,ok,err:=Inspect(project,p.HandoffID,now.Add(7*time.Second))
 if err!=nil||!ok{t.Fatalf("%+v %v",row,err)}
 if row.Steps["dispatch"].Evidence!="native first delivery" || row.RedispatchCount!=1 || len(row.Redispatches)!=1 || row.ClaimedBy!="" {
  t.Fatalf("original delivery/claim was mutated: %+v",row)
 }
 claim,err:=Claim(project,p.HandoffID,"/root/controller","controller-new",now.Add(8*time.Second))
 if err!=nil||!claim.Claimed{t.Fatalf("claim %+v %v",claim,err)}
}

func TestRedispatchRejectsMissingFirstProof(t *testing.T) {
 project:=t.TempDir()
 writeSimpleTask(t,project,"A-1")
 now:=time.Date(2026,11,2,8,0,0,0,time.UTC)
 appendControllerAttempt(t,project,"controller-new","session-1","agent-1",runtimeobs.StateRunning,now)
 p,err:=Prepare(PrepareRequest{Project:project,TaskID:"A-1",EventType:EventRegistrationReady,SourceAgentPath:"/root/registrar",TargetAgentPath:"/root/controller",TargetAttemptID:"controller-new",ExecutionAuthorized:true},now.Add(time.Second))
 if err!=nil{t.Fatal(err)}
 _,err=Redispatch(project,p.HandoffID,"controller-new","proof",now.Add(2*time.Second))
 if err==nil||!strings.Contains(err.Error(),"original dispatch=ok"){t.Fatalf("expected first dispatch fence, got %v",err)}
}
