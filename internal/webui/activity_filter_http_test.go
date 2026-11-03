package webui

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "net/url"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/activity"
)
func TestActivityProjectFiltersUseClosedSegmentsAndValidateInputs(t *testing.T){
 project:=t.TempDir()
 registerWebFixture(t,project)
 at:=time.Now().UTC().Add(-5*time.Minute)
 first:=activity.Event{
  SchemaVersion:1,EventID:"filter-start",TaskID:"B-1",Action:activity.Start,
  Type:activity.Implementation,Role:activity.RoleWorker,AgentID:"worker-1",
  RuntimeAttemptID:"run-filter",Confidence:activity.Confirmed,
  OccurredAt:at,RecordedAt:at,
 }
 done:=first;done.EventID="filter-stop";done.Action=activity.Stop;done.OccurredAt=at.Add(time.Minute);done.RecordedAt=done.OccurredAt
 store:=activity.Ledger{Project:project}
 for _,e:=range []activity.Event{first,done}{if _,err:=store.Apply(e);err!=nil{t.Fatal(err)}}
 handler,err:=Handler(project,"","test")
 if err!=nil{t.Fatal(err)}
 fetch:=func(values url.Values)(int,map[string]any){
  t.Helper()
  resp:=httptest.NewRecorder()
  handler.ServeHTTP(resp,httptest.NewRequest(http.MethodGet,"/api/activity/project?"+values.Encode(),nil))
  body:=map[string]any{}
  if err:=json.Unmarshal(resp.Body.Bytes(),&body);err!=nil{t.Fatal(err)}
  return resp.Code,body
 }
 values:=url.Values{"project":{project},"role":{"worker"},"type":{"implementation"},"days":{"7"}}
 status,report:=fetch(values)
 if status!=http.StatusOK||report["agent_time_ms"]!=float64(60000)||report["filtered_activity"]!=true {
  t.Fatalf("worker filter %d %+v",status,report)
 }
 values.Set("role","controller")
 status,report=fetch(values)
 if status!=http.StatusOK||report["agent_time_ms"]!=float64(0){
  t.Fatalf("controller filter %d %+v",status,report)
 }
 values.Set("role","invalid-role")
 status,_=fetch(values)
 if status!=http.StatusBadRequest{t.Fatalf("invalid role should fail, got %d",status)}
 values.Set("role","worker");values.Set("days","5000")
 status,_=fetch(values)
 if status!=http.StatusBadRequest{t.Fatalf("invalid date range should fail, got %d",status)}
}
