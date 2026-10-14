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

func TestResolvedSessionWarningRequiresExactLaterHook(t *testing.T) {
 now:=time.Date(2026,10,15,0,0,0,0,time.UTC)
 item:=operationIncident{Kind:"runtime_unknown",Quality:"verification_required",
  AttemptID:"run-1",TaskID:"A-5",AgentPath:"/root/controller/worker",
  DetectedAt:now.Add(-time.Minute).Format(time.RFC3339Nano)}
 good:=runtimeobs.Attempt{AttemptID:"run-1",TaskID:"A-5",AgentPath:item.AgentPath,
  CurrentState:runtimeobs.StateRunning,StateEvidenceSource:runtimeobs.EvidenceHook,
  StateObservationQuality:runtimeobs.QualityObserved,
  LastObservedAt:now.Add(-time.Second).Format(time.RFC3339Nano)}
 when,why,ok:=operationExactAttemptRecovered(item,[]runtimeobs.Attempt{good},now)
 if !ok||when!=good.LastObservedAt||!strings.Contains(why,"observed Hook"){
  t.Fatalf("recovered exact Hook running attempt: %v %q %q",ok,when,why)
 }
 completed:=good
 completed.CurrentState=runtimeobs.StateCompleted
 completed.Terminal=true
 completed.EndedAt=now.Add(-time.Second).Format(time.RFC3339Nano)
 when,why,ok=operationExactAttemptRecovered(item,[]runtimeobs.Attempt{completed},now)
 if !ok||when!=completed.EndedAt||!strings.Contains(why,"completion"){
  t.Fatalf("same-attempt completion not accepted: %v %q %q",ok,when,why)
 }
 cases:=map[string]runtimeobs.Attempt{}
 wrongID:=good;wrongID.AttemptID="run-other";cases["different attempt"]=wrongID
 wrongTask:=good;wrongTask.TaskID="A-6";cases["different task"]=wrongTask
 wrongAgent:=good;wrongAgent.AgentPath="/root/controller/different";cases["different agent"]=wrongAgent
 inferred:=good;inferred.StateObservationQuality=runtimeobs.QualityInferred;cases["not Hook verified"]=inferred
 manual:=good;manual.StateEvidenceSource=runtimeobs.EvidenceManualBinding;cases["manual binding"]=manual
 old:=good;old.LastObservedAt=now.Add(-2*time.Minute).Format(time.RFC3339Nano);cases["old signal"]=old
 stale:=good;stale.LastObservedAt=now.Add(-runtimeobs.StaleWarnDuration()).Format(time.RFC3339Nano);cases["stale Hook"]=stale
 interrupted:=good;interrupted.CurrentState=runtimeobs.StateInterrupted;cases["interrupted"]=interrupted
 for name,candidate:=range cases {
  t.Run(name,func(t *testing.T){
   if _,_,ok:=operationExactAttemptRecovered(item,[]runtimeobs.Attempt{candidate},now);ok {
    t.Fatalf("untrusted recovery accepted: %+v",candidate)
   }
  })
 }
 confirmed:=item
 confirmed.Kind="errored";confirmed.Quality="confirmed"
 if _,_,ok:=operationExactAttemptRecovered(confirmed,[]runtimeobs.Attempt{good,completed},now);ok {
  t.Fatal("confirmed terminal failure was hidden")
 }
}

func TestCanonicalDoneOnlyClosesUnambiguousBacklog(t *testing.T) {
 done:=backlog.Record{State:"done",ID:"A-5"}
 doing:=backlog.Record{State:"doing",ID:"A-5"}
 if !operationCanonicalDoneRows(map[string][]backlog.Record{"A-5":{done}},"A-5"){t.Fatal("unique done missing")}
 if operationCanonicalDoneRows(map[string][]backlog.Record{"A-5":{done,doing}},"A-5"){t.Fatal("ambiguous ID was retired")}
 if operationCanonicalDoneRows(map[string][]backlog.Record{"A-5":{doing}},"A-5"){t.Fatal("ongoing work was retired")}
 if operationCanonicalDoneRows(map[string][]backlog.Record{},"A-5"){t.Fatal("missing task was retired")}
}

func TestResolvedAssignmentWarningClosesAndStaysClosedAfterRestart(t *testing.T) {
 project:=testOperationProject(t)
 folder:=filepath.Join(project,"_task_mecca","data","backlog")
 doing:=filepath.Join(folder,"000005.A-5.worker.doing.md")
 if err:=os.WriteFile(doing,[]byte("# A-5 Worker\n- Agent: /root/controller/worker\n- 변경범위: -\n- 선행: -\n- 연관: -\n"),0600);err!=nil{t.Fatal(err)}
 now:=time.Now().UTC()
 first,err:=scanOperationProject(project,now)
 if err!=nil{t.Fatal(err)}
 var activeID string
 for _,warning:=range first.Incidents{
  if warning.TaskID=="A-5"&&warning.Kind=="runtime_unknown"&&warning.RecoveredAt==""{activeID=warning.ID}
 }
 if activeID==""{t.Fatalf("missing initial warning %+v",first.Incidents)}
 done:=filepath.Join(folder,"000005.A-5.worker.done.md")
 if err:=os.Rename(doing,done);err!=nil{t.Fatal(err)}
 second,err:=scanOperationProject(project,now.Add(time.Second))
 if err!=nil{t.Fatal(err)}
 for _,warning:=range second.Incidents {
  if warning.ID==activeID{
   if warning.RecoveredAt==""||!strings.Contains(warning.RecoveryEvidence,"canonical task is done"){
    t.Fatalf("obsolete warning still active: %+v",warning)
   }
  }
  if warning.RecoveredAt==""&&warning.Kind!="monitor_gap"{t.Fatalf("warning survived canonical completion: %+v",warning)}
 }
 after,err:=scanOperationProject(project,now.Add(2*time.Second))
 if err!=nil{t.Fatal(err)}
 if len(after.Incidents)!=len(second.Incidents){t.Fatalf("closed warning recurred: %+v",after.Incidents)}
 restart,err:=readOperationJournal(project)
 if err!=nil||len(restart.Incidents)!=len(first.Incidents)||restart.Incidents[0].RecoveredAt==""{
  t.Fatalf("restart lost resolved source: %+v %v",restart.Incidents,err)
 }
}

func TestMissingRuntimeSignalDoesNotPretendRecovery(t *testing.T){
 // An absent record or temporarily unavailable sensor is not evidence.
 now:=time.Date(2026,10,15,0,0,0,0,time.UTC)
 warning:=operationIncident{Kind:"runtime_unknown",Quality:"verification_required",
  AttemptID:"run-old",TaskID:"A-5",AgentPath:"/worker",DetectedAt:now.Add(-time.Minute).Format(time.RFC3339Nano)}
 if _,_,ok:=operationExactAttemptRecovered(warning,nil,now);ok{t.Fatal("missing attempt was treated as resolution")}
}
