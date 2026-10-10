package webui

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/maintenance"
)

func detailContentFixture(t *testing.T) (string, http.Handler) {
 t.Helper()
 project:=t.TempDir()
 registerWebFixture(t,project)
 folder:=filepath.Join(project,"_task_mecca","data","backlog")
 if err:=os.MkdirAll(folder,0755);err!=nil{t.Fatal(err)}
 source:="# B-1 Table-first\n\n| Name | Value |\n| --- | --- |\n| Alpha | 1 |\n"
 if err:=os.WriteFile(filepath.Join(folder,"000001.B-1.table.done.md"),[]byte(source),0644);err!=nil{t.Fatal(err)}
 handler,err:=Handler(project,"","test")
 if err!=nil{t.Fatal(err)}
 return project,handler
}

func TestReadOnlyContentProjectionDoesNotWaitForSameProjectMonitoring(t *testing.T) {
 project,handler:=detailContentFixture(t)
 entered:=make(chan struct{})
 release:=make(chan struct{})
 finished:=make(chan bool,1)
 go func(){finished<-maintenance.WithProjectMonitoring(project,func(){close(entered);<-release})}()
 select{case <-entered:case <-time.After(3*time.Second):t.Fatal("monitor lock not acquired")}
 // The full detail request is intentionally serialized behind the sensor.
 fullDone:=make(chan *httptest.ResponseRecorder,1)
 go func(){
  rec:=httptest.NewRecorder()
  handler.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/api/tasks/B-1?projection=full",nil))
  fullDone<-rec
 }()
 // The text must not be held hostage by the sensor's lock.
 fastDone:=make(chan *httptest.ResponseRecorder,1)
 go func(){
  rec:=httptest.NewRecorder()
  handler.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/api/tasks/B-1?projection=content",nil))
  fastDone<-rec
 }()
 select {
 case rec:=<-fastDone:
  if rec.Code!=200 {t.Fatalf("fast status=%d body=%s",rec.Code,rec.Body.String())}
  var body map[string]any
  if err:=json.Unmarshal(rec.Body.Bytes(),&body);err!=nil{t.Fatal(err)}
  if body["content_only"]!=true {t.Fatalf("missing content flag: %#v",body["content_only"])}
  if !strings.Contains(body["raw_markdown"].(string),"| Alpha | 1 |") {t.Fatal("Markdown table missing")}
  if _,ok:=body["lifecycle"].(map[string]any);!ok {t.Fatal("content projection did not retain the expected detail contract")}
  timing:=rec.Header().Get("Server-Timing")
  if !strings.Contains(timing,"content;dur=")||strings.Contains(timing,"control;dur="){t.Fatalf("invalid content timing: %s",timing)}
 case <-time.After(2*time.Second):
  close(release)
  t.Fatal("read-only task content was blocked by full project monitoring")
 }
 select {case <-fullDone:close(release);t.Fatal("full detail unexpectedly bypassed monitoring lock");default:}
 close(release)
 if !<-finished{t.Fatal("monitoring work was suppressed")}
 select {case rec:=<-fullDone:if rec.Code!=200{t.Fatalf("full status=%d body=%s",rec.Code,rec.Body.String())};case <-time.After(10*time.Second):t.Fatal("full detail did not finish")}
}

func TestReadOnlyContentRejectsPausedProjectAndReflectsCurrentMarkdown(t *testing.T) {
 project,handler:=detailContentFixture(t)
 url:="/api/tasks/B-1?projection=content"
 first:=httptest.NewRecorder()
 handler.ServeHTTP(first,httptest.NewRequest(http.MethodGet,url,nil))
 if first.Code!=200{t.Fatalf("first status=%d",first.Code)}
 folder:=filepath.Join(project,"_task_mecca","data","backlog")
 if err:=os.WriteFile(filepath.Join(folder,"000001.B-1.table.done.md"),[]byte("# B-1 Updated\n| New | 2 |\n"),0644);err!=nil{t.Fatal(err)}
 second:=httptest.NewRecorder()
 handler.ServeHTTP(second,httptest.NewRequest(http.MethodGet,url,nil))
 if second.Code!=200||!strings.Contains(second.Body.String(),"B-1 Updated"){t.Fatalf("stale content: %d %s",second.Code,second.Body.String())}
 if err:=maintenance.SetProjectMonitoring(project,false);err!=nil{t.Fatal(err)}
 paused:=httptest.NewRecorder()
 handler.ServeHTTP(paused,httptest.NewRequest(http.MethodGet,url,nil))
 if paused.Code!=http.StatusConflict{t.Fatalf("paused status=%d body=%s",paused.Code,paused.Body.String())}
 if err:=maintenance.SetProjectMonitoring(project,true);err!=nil{t.Fatal(err)}
 resumed:=httptest.NewRecorder()
 handler.ServeHTTP(resumed,httptest.NewRequest(http.MethodGet,url,nil))
 if resumed.Code!=200{t.Fatalf("resumed status=%d body=%s",resumed.Code,resumed.Body.String())}
}
