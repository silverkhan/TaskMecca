package webui

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "net/url"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/backlog"
)
func TestActivityHTTPIncludesReportedLifecycleCoverage(t *testing.T){
 project:=t.TempDir()
 registerWebFixture(t,project)
 stopProjectAttentionFeedsForTest(t,project)
 origin:=time.Date(2026,11,4,0,0,0,0,time.UTC)
 for i,e:=range []struct{kind,actor string;minute int}{
  {"registered","/root/registrar",0},
  {"assigned","/root/controller",3},
  {"started","/root/controller/worker",10},
  {"completed","/root/controller",30},
 }{
  _,err:=backlog.RecordLifecycleTransition(project,backlog.LifecycleTransition{
   EventID:"activity-lifecycle-"+e.kind,TaskID:"B-1",Kind:e.kind,
   Actor:e.actor,EvidenceSource:"worker_report",
   OccurredAt:origin.Add(time.Duration(e.minute)*time.Minute).Format(time.RFC3339Nano),
  },origin.Add(time.Duration(i)*time.Second))
  if err!=nil{t.Fatal(err)}
 }
 handler,err:=Handler(project,"","test")
 if err!=nil{t.Fatal(err)}
 path:="/api/activity?"+url.Values{"project":{project},"task":{"B-1"}}.Encode()
 response:=httptest.NewRecorder()
 handler.ServeHTTP(response,httptest.NewRequest(http.MethodGet,path,nil))
 if response.Code!=http.StatusOK{t.Fatalf("detail status %d: %s",response.Code,response.Body.String())}
 var data struct{
  Coverage struct{
   TotalElapsedMillis *int64 `json:"total_elapsed_ms"`
   RegistrationQueueMillis int64 `json:"registration_queue_ms"`
   DispatchWaitMillis int64 `json:"dispatch_wait_ms"`
   UnattributedElapsedMillis *int64 `json:"unattributed_elapsed_ms"`
  } `json:"lifecycle_coverage"`
 }
 if err:=json.Unmarshal(response.Body.Bytes(),&data);err!=nil{t.Fatal(err)}
 c:=data.Coverage
 if c.TotalElapsedMillis==nil||*c.TotalElapsedMillis!=30*60000||
   c.RegistrationQueueMillis!=3*60000||c.DispatchWaitMillis!=7*60000||
   c.UnattributedElapsedMillis==nil||*c.UnattributedElapsedMillis!=20*60000{
    t.Fatalf("Activity HTTP lifecycle coverage %+v",c)
 }
 projectResponse:=httptest.NewRecorder()
 handler.ServeHTTP(projectResponse,httptest.NewRequest(http.MethodGet,"/api/activity/project?"+url.Values{"project":{project}}.Encode(),nil))
 if projectResponse.Code!=http.StatusOK{t.Fatalf("project status %d: %s",projectResponse.Code,projectResponse.Body.String())}
 var projectRollup struct{ Completed int `json:"lifecycle_completed_tasks"` }
 if err:=json.Unmarshal(projectResponse.Body.Bytes(),&projectRollup);err!=nil{t.Fatal(err)}
 if projectRollup.Completed!=1{t.Fatalf("project lifecycle count %+v",projectRollup)}
}
