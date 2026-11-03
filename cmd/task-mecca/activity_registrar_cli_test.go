package main
import (
 "testing"
 "time"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)
func TestRegistrarRecorderUsesBoundRuntimeWithoutWorkerAssignment(t *testing.T){
 project:=t.TempDir()
 now:=time.Now().UTC()
 if err:=runtimeobs.AppendExecutionEvent(project,runtimeobs.ExecutionEvent{
  EventID:"registrar-start",EventKind:"state",ObservedAt:now.Format(time.RFC3339Nano),
  AttemptID:"run-reg",Provider:"codex",RuntimeAgentID:"reg-agent",State:runtimeobs.StateRunning,
  EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
 });err!=nil{t.Fatal(err)}
 if _,err:=runtimeobs.BindAttempt(project,"run-reg","B-589","/root/registrar","explicit","",
  map[string]string{"origin":"registration"},now.Add(time.Millisecond));err!=nil{t.Fatal(err)}
 args:=[]string{"start","B-589","--project",project,"--role","registrar","--agent","reg-agent",
  "--attempt","run-reg","--event-id","registrar-activity-start","--type","requirements",
  "--evidence","registration requirements review","--json"}
 code,response,reason:=invokeActivity(t,args...)
 if code!=0||response["recorded"]!=true{t.Fatalf("registrar start %d %+v %s",code,response,reason)}
}
