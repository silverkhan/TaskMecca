package main

import (
 "testing"
 "time"
 "github.com/silverkhan/TaskMecca/internal/activity"
)

func TestActivityStatusTerminalDoesNotAppearOpen(t *testing.T){
 project:=t.TempDir()
 t0:=time.Now().UTC().Add(-time.Minute)
 started:=activity.Event{
  SchemaVersion:1,EventID:"status-start",TaskID:"B-589",Action:activity.Start,
  Role:activity.RoleWorker,AgentID:"worker-a",RuntimeAttemptID:"attempt-a",
  Type:activity.Requirements,Confidence:activity.Inferred,
  Evidence:"worker reported start",OccurredAt:t0,RecordedAt:t0,
 }
 if _,err:=(activity.Ledger{Project:project}).Append(started);err!=nil{t.Fatal(err)}
 ended:=started
 ended.EventID="status-abandon"
 ended.Action=activity.Abandon
 ended.Confidence=activity.Unknown
 ended.OccurredAt=t0.Add(time.Second)
 ended.RecordedAt=ended.OccurredAt
 if _,err:=(activity.Ledger{Project:project}).Append(ended);err!=nil{t.Fatal(err)}
 status,payload,reason:=invokeActivity(t,"status","B-589","--project",project,"--json")
 if status!=0{t.Fatalf("status failed: %s",reason)}
 rows:=payload["open_activities"].([]any)
 if len(rows)!=0{t.Fatalf("terminal activity counted as open: %+v",rows)}
}
