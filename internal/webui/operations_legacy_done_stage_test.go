package webui

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestArchivedDoneTaskRetiresLegacyUnboundAssignmentStall(t *testing.T){
 project:=testOperationProject(t)
 base:=time.Now().UTC().Add(-30*time.Minute)
 a,err:=runtimeobs.RecordAssignment(project,"legacy-b464-assignment","B-464","/root/controller/kkobugi",base)
 if err!=nil{t.Fatal(err)}
 folder:=filepath.Join(project,"_task_mecca","data","backlog")
 doing:=filepath.Join(folder,"000464.B-464.regime.doing.md")
 if err:=os.WriteFile(doing,[]byte("# B-464 vol regime SJM\n- Agent: /root/controller/kkobugi\n"),0600);err!=nil{t.Fatal(err)}
 scan:=func(at time.Time)operationJournal{
  t.Helper()
  journal,err:=scanOperationProject(project,at)
  if err!=nil{t.Fatal(err)}
  return journal
 }
 now:=time.Now().UTC()
 first:=scan(now)
 var incidentID string
 for _,item:=range first.Incidents{
  if item.TaskID=="B-464" && item.Kind=="handoff_stalled" && item.Evidence=="latest durable assignment" && item.RecoveredAt==""{
   if item.AssignmentID!=a.AssignmentID||item.AttemptID!=""{t.Fatalf("unexpected warning: %+v",item)}
   incidentID=item.ID
  }
 }
 if incidentID==""{t.Fatalf("missing legacy B-464 stall: %+v",first.Incidents)}
 archive:=filepath.Join(folder,"archive","2026-10","000464.B-464.regime.done.md")
 if err:=os.MkdirAll(filepath.Dir(archive),0700);err!=nil{t.Fatal(err)}
 if err:=os.Rename(doing,archive);err!=nil{t.Fatal(err)}
 second:=scan(now.Add(time.Second))
 var retired int
 for _,item:=range second.Incidents{
  if item.ID==incidentID{
   retired++
   if item.RecoveredAt==""||!strings.Contains(item.RecoveryEvidence,"unbound durable assignment stage retired"){
    t.Fatalf("completed B-464 still has active warning: %+v",item)
   }
  }
  if item.TaskID=="B-464" && item.RecoveredAt==""{t.Fatalf("completed task still has active incident: %+v",item)}
 }
 if retired!=1{t.Fatalf("historical incident ID disappeared: %+v",second.Incidents)}
 for _,stage:=range second.Stages{if stage.TaskID=="B-464"{t.Fatalf("obsolete current stage: %+v",stage)}}
 third:=scan(now.Add(2*time.Second))
 if len(third.Incidents)!=len(second.Incidents){t.Fatalf("rescan reissued alert: %+v",third.Incidents)}
 persisted,err:=readOperationJournal(project)
 if err!=nil{t.Fatal(err)}
 if len(persisted.Incidents)!=len(second.Incidents){t.Fatal("journal lost incident history")}
}

func TestRetirementRequiresUnboundLatestAssignmentAndUniqueDone(t *testing.T) {
 done:=func(_ string)bool{return true}
 stage:=operationStage{
  TaskID:"B-464",AssignmentID:"assignment-1",
  Stage:"assignment_pending",Evidence:"latest durable assignment",
 }
 if !retiredUnlinkedAssignmentStage(stage,done){t.Fatal("unique done assignment not retired")}
 variants:=map[string]operationStage{}
 linked:=stage;linked.AttemptID="run-1";variants["linked runtime"]=linked
 handoff:=stage;handoff.HandoffID="handoff-1";variants["handoff"]=handoff
 failed:=stage;failed.Failed=true;variants["confirmed failed"]=failed
 review:=stage;review.ControllerReview=true;variants["review"]=review
 wrongStage:=stage;wrongStage.Stage="handoff_pending";variants["handoff pending"]=wrongStage
 wrongEvidence:=stage;wrongEvidence.Evidence="durable worker_done handoff";variants["source changed"]=wrongEvidence
 for name,item:=range variants{
  t.Run(name,func(t *testing.T){
   if retiredUnlinkedAssignmentStage(item,done){t.Fatalf("unsafe retirement: %+v",item)}
  })
 }
 if retiredUnlinkedAssignmentStage(stage,func(string)bool{return false}){t.Fatal("unavailable/duplicate canonical state retired warning")}
}

func TestDuplicateCompletedTaskIDsDoNotRetireAssignmentStall(t *testing.T){
 project:=testOperationProject(t)
 base:=time.Now().UTC().Add(-30*time.Minute)
 if _,err:=runtimeobs.RecordAssignment(project,"legacy-duplicate","B-464","/root/controller/worker",base);err!=nil{t.Fatal(err)}
 folder:=filepath.Join(project,"_task_mecca","data","backlog")
 for _,name:=range []string{"000464.B-464.first.done.md","000465.B-464.second.done.md"}{
  if err:=os.WriteFile(filepath.Join(folder,name),[]byte("# B-464\n- Agent: /root/controller/worker\n"),0600);err!=nil{t.Fatal(err)}
 }
 journal,err:=scanOperationProject(project,time.Now().UTC())
 if err!=nil{t.Fatal(err)}
 found:=false
 for _,item:=range journal.Incidents{
  if item.TaskID=="B-464" && item.Kind=="handoff_stalled" && item.RecoveredAt=="" {found=true}
 }
 if !found{t.Fatalf("duplicate canonical ID incorrectly retired stage: %+v",journal.Incidents)}
}
