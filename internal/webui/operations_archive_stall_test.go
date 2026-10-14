package webui

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/backlog"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestArchiveRetiresExactUnboundDurableAssignmentOnly(t *testing.T){
 now:=time.Date(2026,10,15,5,0,0,0,time.UTC)
 assigned:=now.Add(-time.Hour).Format(time.RFC3339Nano)
 archived:=now.Add(-time.Minute).Format(time.RFC3339Nano)
 assignment:=runtimeobs.Assignment{AssignmentID:"assignment-a",TaskID:"B-464",AgentPath:"/root/controller/kkobugi",AssignedAt:assigned}
 done:=backlog.Record{ID:"B-464",State:"done",Location:"archive",ArchiveMonth:"2026-10",Mtime:archived}
 accept:=func(rows map[string][]backlog.Record,assignments []runtimeobs.Assignment,id,stageAt string)bool{
  return operationArchivedAssignmentRetired(rows,assignments,"B-464",id,stageAt,now)
 }
 rows:=map[string][]backlog.Record{"B-464":{done}}
 if !accept(rows,[]runtimeobs.Assignment{assignment},"assignment-a",assigned){t.Fatal("archived done exact old assignment still pending")}
 alternatives:=[]struct{name string; row backlog.Record}{
  {"active_done",backlog.Record{ID:"B-464",State:"done",Location:"active",Mtime:archived}},
  {"archive_doing",backlog.Record{ID:"B-464",State:"doing",Location:"archive",ArchiveMonth:"2026-10",Mtime:archived}},
  {"legacy_done",backlog.Record{ID:"B-464",State:"done",Location:"legacy",Mtime:archived}},
  {"invalid_time",backlog.Record{ID:"B-464",State:"done",Location:"archive",ArchiveMonth:"2026-10",Mtime:"not-a-clock"}},
 }
 for _,tc:=range alternatives{t.Run(tc.name,func(t *testing.T){
  if accept(map[string][]backlog.Record{"B-464":{tc.row}},[]runtimeobs.Assignment{assignment},"assignment-a",assigned){
   t.Fatal("unsafe task state retired durable stage")
  }
 })}
 if accept(map[string][]backlog.Record{"B-464":{done,done}},[]runtimeobs.Assignment{assignment},"assignment-a",assigned){t.Fatal("duplicate canonical ID retired stage")}
 if accept(rows,[]runtimeobs.Assignment{assignment},"different-assignment",assigned){t.Fatal("mismatched source retired")}
 if accept(rows,[]runtimeobs.Assignment{assignment},"assignment-a",now.Add(-2*time.Hour).Format(time.RFC3339Nano)){t.Fatal("wrong assignment clock retired")}
 newer:=assignment
 newer.AssignmentID="assignment-new"
 newer.AssignedAt=now.Add(time.Minute).Format(time.RFC3339Nano)
 if accept(rows,[]runtimeobs.Assignment{assignment,newer},"assignment-a",assigned){t.Fatal("post-archive new assignment was hidden")}
 if accept(rows,[]runtimeobs.Assignment{assignment,assignment},"assignment-a",assigned){t.Fatal("ambiguous assignment identity was hidden")}
}

func TestArchivedDoneBacklogRetiresPriorStalledStageAcrossScans(t *testing.T){
 project:=testOperationProject(t)
 folder:=filepath.Join(project,"_task_mecca","data","backlog")
 base:=time.Now().UTC().Add(-2*time.Hour)
 if _,err:=runtimeobs.RecordAssignment(project,"assignment-464","B-464","/root/controller/kkobugi",base);err!=nil{t.Fatal(err)}
 doing:=filepath.Join(folder,"000464.B-464.regime.doing.md")
 if err:=os.WriteFile(doing,[]byte("# B-464 regime\n- Agent: /root/controller/kkobugi\n"),0600);err!=nil{t.Fatal(err)}
 now:=time.Now().UTC()
 first,err:=scanOperationProject(project,now)
 if err!=nil{t.Fatal(err)}
 var oldID string
 for _,warning:=range first.Incidents{
  if warning.TaskID=="B-464" && warning.Kind=="handoff_stalled" && warning.Evidence=="latest durable assignment" && warning.RecoveredAt=="" {
   oldID=warning.ID
  }
 }
 if oldID==""{t.Fatalf("missing stalled durable assignment warning: %+v",first.Incidents)}
 archive:=filepath.Join(folder,"archive","2026-10")
 if err:=os.MkdirAll(archive,0755);err!=nil{t.Fatal(err)}
 done:=filepath.Join(archive,"000464.B-464.regime.done.md")
 if err:=os.WriteFile(doing,[]byte("# B-464 regime\n- Agent: /root/controller/kkobugi\n- 결과: completed\n- 검증: verified\n"),0600);err!=nil{t.Fatal(err)}
 if err:=os.Rename(doing,done);err!=nil{t.Fatal(err)}
 second,err:=scanOperationProject(project,now.Add(2*time.Second))
 if err!=nil{t.Fatal(err)}
 if len(second.Stages)!=0{t.Fatalf("archived assignment remains current stage: %+v",second.Stages)}
 found:=false
 for _,warning:=range second.Incidents{
  if warning.ID==oldID{
   found=true
   if warning.RecoveredAt==""||!strings.Contains(warning.RecoveryEvidence,"canonical archived done"){
    t.Fatalf("archived done warning remains active or lacks reason: %+v",warning)
   }
  }
  if warning.RecoveredAt=="" {t.Fatalf("historical warning still active: %+v",warning)}
 }
 if !found{t.Fatal("incident was discarded instead of retained as history")}
 for _,when:=range []time.Time{now.Add(3*time.Second),now.Add(4*time.Second)}{
  next,err:=scanOperationProject(project,when)
  if err!=nil{t.Fatal(err)}
  if len(next.Incidents)!=len(second.Incidents)||next.NextSequence!=second.NextSequence||len(next.Stages)!=0{
   t.Fatalf("archived warning was re-created: %+v",next)
  }
 }
 loaded,err:=readOperationJournal(project)
 if err!=nil||len(loaded.Incidents)!=len(second.Incidents)||loaded.Incidents[0].RecoveredAt==""{
  t.Fatalf("restored journal lost resolution: %+v %v",loaded,err)
 }
 data,err:=os.ReadFile(done)
 if err!=nil||!strings.Contains(string(data),"B-464"){t.Fatalf("monitor changed archival file: %v",err)}
}

func TestArchivedDoneDoesNotConcealActualFailure(t *testing.T){
 project:=testOperationProject(t)
 folder:=filepath.Join(project,"_task_mecca","data","backlog")
 archive:=filepath.Join(folder,"archive","2026-10")
 if err:=os.MkdirAll(archive,0755);err!=nil{t.Fatal(err)}
 done:=filepath.Join(archive,"000464.B-464.regime.done.md")
 if err:=os.WriteFile(done,[]byte("# B-464 regime\\n- 결과: done\\n"),0600);err!=nil{t.Fatal(err)}
 now:=time.Now().UTC()
 hook:=testOperationHook(t,project,`{"session_id":"failed-session","hook_event_name":"SubagentStart","agent_id":"failed-worker"}`,now.Add(-time.Minute))
 if _,err:=runtimeobs.BindAttempt(project,hook.AttemptID,"B-464","/root/controller/worker","explicit","",nil,now.Add(-59*time.Second));err!=nil{t.Fatal(err)}
 if err:=runtimeobs.AppendExecutionEvent(project,runtimeobs.ExecutionEvent{
  EventKind:"state",ObservedAt:now.Add(-30*time.Second).Format(time.RFC3339Nano),
  AttemptID:hook.AttemptID,Provider:"codex",RuntimeAgentID:"failed-worker",
  State:runtimeobs.StateErrored,Terminal:true,EvidenceSource:runtimeobs.EvidenceHook,
  ObservationQuality:runtimeobs.QualityObserved,
 });err!=nil{t.Fatal(err)}
 after,err:=scanOperationProject(project,now)
 if err!=nil{t.Fatal(err)}
 found:=false
 for _,warning:=range after.Incidents{
  if warning.AttemptID==hook.AttemptID && warning.Kind=="errored" && warning.Quality=="confirmed" && warning.RecoveredAt==""{found=true}
 }
 if !found{t.Fatalf("observed errored Hook failure concealed by canonical archive: %+v",after.Incidents)}
}
