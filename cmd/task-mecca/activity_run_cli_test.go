package main

import (
 "bytes"
 "os"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/activity"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestActivityRunHelperProcess(t *testing.T){
 if os.Getenv("TASK_MECCA_ACTIVITY_RUN_HELPER")=="1" {os.Exit(0)}
 if os.Getenv("TASK_MECCA_ACTIVITY_RUN_HELPER")=="fail" {os.Exit(7)}
}
func TestActivityRunWrapsCommandAndAvoidsDuplicateExecution(t *testing.T){
 project:=t.TempDir()
 now:=time.Now().UTC()
 if _,err:=runtimeobs.RecordAssignment(project,"assignment-run","B-589","/root/controller/worker-run",now);err!=nil{t.Fatal(err)}
 if err:=runtimeobs.AppendExecutionEvent(project,runtimeobs.ExecutionEvent{
   EventID:"hook-run",EventKind:"state",AttemptID:"attempt-run",ObservedAt:now.Format(time.RFC3339Nano),
   Provider:"codex",RuntimeAgentID:"agent-run",State:runtimeobs.StateRunning,
   EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
 });err!=nil{t.Fatal(err)}
 if _,err:=runtimeobs.BindAttempt(project,"attempt-run","B-589","/root/controller/worker-run","dispatch","",
   map[string]string{"assignment_id":"assignment-run"},now.Add(time.Millisecond));err!=nil{t.Fatal(err)}
 t.Setenv("TASK_MECCA_ACTIVITY_RUN_HELPER","1")
 args:=[]string{"B-589","--project",project,"--role","worker","--agent","agent-run",
   "--attempt","attempt-run","--assignment","assignment-run","--event-id","run-one",
   "--type","targeted_test","--git-sha","123abc","--scope","activity",
   "--",""+os.Args[0],"-test.run=TestActivityRunHelperProcess"}
 var stdout,stderr bytes.Buffer
 if status:=runActivityCommand(args,&stdout,&stderr);status!=0{t.Fatalf("run exit=%d stderr=%s",status,stderr.String())}
 rows,err:=(activity.Ledger{Project:project}).List("B-589")
 if err!=nil{t.Fatal(err)}
 if len(rows)!=2{t.Fatalf("expected start/stop, got %d",len(rows))}
 sum:=activity.Summarize("B-589",rows)
 report:=activity.SummarizeTests(sum.Segments)
 if report.FullRegressionCount!=0||len(report.Runs)!=1||report.Runs[0].RunEvidence.Result!="passed"{
   t.Fatalf("incorrect test-run evidence: %+v",report)
 }
 stderr.Reset()
 if status:=runActivityCommand(args,&stdout,&stderr);status==0{t.Fatal("duplicate execution unexpectedly permitted")}
 if rows,err:=(activity.Ledger{Project:project}).List("B-589");err!=nil||len(rows)!=2{
  t.Fatalf("duplicate run mutated ledger: %d %v",len(rows),err)
 }
}

func TestActivityRunRestoresPriorPhaseAfterFailure(t *testing.T){
 project:=t.TempDir()
 now:=time.Now().UTC()
 if _,err:=runtimeobs.RecordAssignment(project,"assignment-restore","B-589","/root/controller/worker-restore",now);err!=nil{t.Fatal(err)}
 if err:=runtimeobs.AppendExecutionEvent(project,runtimeobs.ExecutionEvent{
  EventID:"hook-restore",EventKind:"state",AttemptID:"attempt-restore",ObservedAt:now.Format(time.RFC3339Nano),
  RuntimeAgentID:"agent-restore",Provider:"codex",State:runtimeobs.StateRunning,
  EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
 });err!=nil{t.Fatal(err)}
 if _,err:=runtimeobs.BindAttempt(project,"attempt-restore","B-589","/root/controller/worker-restore","dispatch","",
  map[string]string{"assignment_id":"assignment-restore"},now.Add(time.Millisecond));err!=nil{t.Fatal(err)}
 var ignored,stderr bytes.Buffer
 status:=runActivityCLI([]string{"start","B-589","--project",project,"--role","worker",
  "--agent","agent-restore","--attempt","attempt-restore","--assignment","assignment-restore",
  "--type","implementation","--event-id","implementation-start","--evidence","implementing"},&ignored,&stderr)
 if status!=0{t.Fatalf("implementation begin %d %s",status,stderr.String())}
 t.Setenv("TASK_MECCA_ACTIVITY_RUN_HELPER","fail")
 runArgs:=[]string{"B-589","--project",project,"--role","worker","--agent","agent-restore",
  "--attempt","attempt-restore","--assignment","assignment-restore","--event-id","run-failed",
  "--type","full_regression","--git-sha","abc123","--scope","all-go",
  "--",os.Args[0],"-test.run=TestActivityRunHelperProcess"}
 stderr.Reset()
 if status:=runActivityCommand(runArgs,&ignored,&stderr);status!=7{t.Fatalf("expected child exit 7, got %d %s",status,stderr.String())}
 rows,err:=(activity.Ledger{Project:project}).List("B-589")
 if err!=nil{t.Fatal(err)}
 sum:=activity.Summarize("B-589",rows)
 if sum.OpenActivities!=1{t.Fatalf("implementation should resume, open=%d",sum.OpenActivities)}
 var last activity.Event
 for _,e:=range rows{if e.Action==activity.Switch{last=e}}
 if last.Type!=activity.Implementation{t.Fatalf("implementation not restored, last switch %+v",last)}
 report:=activity.SummarizeTests(sum.Segments)
 if report.FullRegressionCount!=1||report.Runs[0].RunEvidence.Result!="failed"{
  t.Fatalf("failed test evidence not recorded: %+v",report)
 }
}
