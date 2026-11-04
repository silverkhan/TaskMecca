package main

import (
 "bytes"
 "encoding/json"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/activity"
)

func TestActivityDiagnosticsBoundedAndReadOnly(t *testing.T){
 project:=t.TempDir()
 store:=activity.Ledger{Project:project}
 e:=validDiagnosticEvent()
 if _,err:=store.Append(e);err!=nil{t.Fatal(err)}
 var out,errorsOut bytes.Buffer
 rc:=runActivityDiagnose([]string{"--project",project,"--task","B-589","--samples","3","--json"},&out,&errorsOut)
 if rc!=0{t.Fatalf("status %d: %s",rc,errorsOut.String())}
 var report activityDiagnosticReport
 if err:=json.Unmarshal(out.Bytes(),&report);err!=nil{t.Fatal(err)}
 if report.Events!=1||report.StoredEventFiles!=1||report.StoredBytes==0||
   report.ReadP95Millis<report.ReadP50Millis||report.Measurement!="wall_clock_elapsed_not_cpu_or_battery"{
  t.Fatalf("incorrect diagnostic report %+v",report)
 }
 if events,err:=store.List("B-589");err!=nil||len(events)!=1{t.Fatalf("diagnostics modified ledger: %d %v",len(events),err)}
 out.Reset();errorsOut.Reset()
 if rc:=runActivityDiagnose([]string{"--project",project,"--samples","100"},&out,&errorsOut);rc!=2{
  t.Fatalf("unbounded repeated reads accepted: %d",rc)
 }
}
func validDiagnosticEvent()activity.Event {
 now:=time.Date(2026,11,4,0,0,0,0,time.UTC)
 return activity.Event{SchemaVersion:1,EventID:"diagnose-event",TaskID:"B-589",
  Action:activity.Start,Role:activity.RoleWorker,AgentID:"w",RuntimeAttemptID:"a",
  Type:activity.Implementation,Confidence:activity.Confirmed,OccurredAt:now,RecordedAt:now}
}
func TestDiagnosticPercentile(t *testing.T){
 values:=[]time.Duration{5*time.Millisecond,1*time.Millisecond,4*time.Millisecond,3*time.Millisecond,2*time.Millisecond}
 if diagnosticPercentile(values,50)!=3||diagnosticPercentile(values,95)!=5{
  t.Fatalf("wrong quantile p50=%v p95=%v",diagnosticPercentile(values,50),diagnosticPercentile(values,95))
 }
}
