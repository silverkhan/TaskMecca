package backlog

import (
 "bytes"
 "encoding/json"
 "os"
 "path/filepath"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestLifecycleHistoricalWorkerSeparatedFromCurrentAgentAndRecorder(t *testing.T) {
 project:=t.TempDir()
 now:=time.Date(2026,10,30,0,24,58,0,time.UTC)
 root:=filepath.Join(project,"_task_mecca","data","backlog")
 if err:=os.MkdirAll(root,0755);err!=nil{t.Fatal(err)}
 path:=filepath.Join(root,"000574.B-574.multi-owner.doing.md")
 original:=[]byte("# B-574 Historical assignments\n- Agent: /root/controller/nayong_1030_3m\n- 변경범위: internal\n")
 if err:=os.WriteFile(path,original,0644);err!=nil{t.Fatal(err)}
 for _,data:=range []struct{id,agent string}{
  {"assignment-digeuda","/root/controller/digeuda"},
  {"assignment-nayong","/root/controller/nayong_1030_3m"},
 }{
  if _,err:=runtimeobs.RecordAssignment(project,data.id,"B-574",data.agent,now);err!=nil{t.Fatal(err)}
 }
 events:=[]LifecycleTransition{
  {EventID:"b574-register",TaskID:"B-574",Kind:"registered",Actor:"/root/registrar",EvidenceSource:"registrar_report"},
  {EventID:"b574-digeuda-assign",TaskID:"B-574",Kind:"assigned",Actor:"/root/controller",EvidenceSource:"controller_report",AssignmentID:"assignment-digeuda"},
  {EventID:"b574-digeuda-start",TaskID:"B-574",Kind:"started",Actor:"/root/controller/digeuda",EvidenceSource:"worker_report",AssignmentID:"assignment-digeuda",AttemptID:"run-digeuda"},
  {EventID:"b574-nayong-assign",TaskID:"B-574",Kind:"assigned",Actor:"/root/controller",EvidenceSource:"controller_report",AssignmentID:"assignment-nayong"},
 }
 for i,e:=range events {
  e.OccurredAt=now.Add(time.Duration(i)*time.Minute).UTC().Format(time.RFC3339Nano)
  if _,err:=RecordLifecycleTransition(project,e,now.Add(time.Duration(i)*time.Minute));err!=nil{t.Fatal(err)}
 }
 // Reproduce a legacy Controller-attributed start already written by an old
 // implementation. It must remain readable but must no longer be accepted
 // as a NEW event by RecordLifecycleTransition.
 old:=LifecycleTransition{EventID:"b574-nayong-legacy-start",TaskID:"B-574",Kind:"started",Actor:"/root/controller",EvidenceSource:"worker_report",AssignmentID:"assignment-nayong",AttemptID:"run-nayong",OccurredAt:now.Add(4*time.Minute).UTC().Format(time.RFC3339Nano)}
 payload,err:=json.Marshal(old);if err!=nil{t.Fatal(err)}
 historyPath:=lifecycleTransitionPath(project,old.EventID)
 if err:=os.MkdirAll(filepath.Dir(historyPath),0755);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(historyPath,payload,0644);err!=nil{t.Fatal(err)}
 if _,err:=RecordLifecycleTransition(project,LifecycleTransition{EventID:"invalid-new-start",TaskID:"B-574",Kind:"started",Actor:"/root/controller",EvidenceSource:"worker_report"},now);err==nil{t.Fatal("new Controller-as-Worker start must be refused")}
 if same,err:=RecordLifecycleTransition(project,old,now.Add(5*time.Minute));err!=nil||same.EventID!=old.EventID{t.Fatalf("legacy exact retry must remain idempotent: %v %+v",err,same)}
 scan,err:=ReadLifecycleTransitions(project)
 if err!=nil||len(scan.Events)!=5||len(scan.Findings)!=0{t.Fatalf("old record must remain readable: %+v %v",scan,err)}
 timings,err:=LifecycleTimings(project,"")
 if err!=nil{t.Fatal(err)}
 history:=timings["B-574"]["events"].([]map[string]any)
 if len(history)!=5{t.Fatalf("event count=%d: %+v",len(history),history)}
 for i,want:=range []struct{agent,actor,id,attempt string}{
  {"","/root/registrar","",""},
  {"/root/controller/digeuda","/root/controller","assignment-digeuda",""},
  {"/root/controller/digeuda","/root/controller/digeuda","assignment-digeuda","run-digeuda"},
  {"/root/controller/nayong_1030_3m","/root/controller","assignment-nayong",""},
  {"/root/controller/nayong_1030_3m","/root/controller","assignment-nayong","run-nayong"},
 }{
  if history[i]["assigned_worker"]!=want.agent||history[i]["actor"]!=want.actor||history[i]["assignment_id"]!=want.id||history[i]["attempt_id"]!=want.attempt {
   t.Fatalf("historical identity lost at event %d: %+v",i,history[i])
  }
 }
 inspect,err:=Inspect(project,"","B-574")
 if err!=nil{t.Fatal(err)}
 if inspect["agent"]!="/root/controller/nayong_1030_3m"{t.Fatalf("current assignee wrongly replaced: %+v",inspect["agent"])}
 after,err:=os.ReadFile(path);if err!=nil||!bytes.Equal(after,original){t.Fatal("Lifecycle projection changed Markdown")}
 persisted,err:=os.ReadFile(historyPath);if err!=nil||!bytes.Equal(persisted,payload){t.Fatal("Lifecycle projection altered immutable history")}
}
