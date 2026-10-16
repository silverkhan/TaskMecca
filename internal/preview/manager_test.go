package preview

import (
 "context"
 "fmt"
 "net/http"
 "net/http/httptest"
 "strings"
 "sync"
 "testing"
)

type fakeNativeRuntime struct {
 mu sync.Mutex
 healthURL string
 checks int
 launches []int
 stops []string
 checkErr error
}
func (f *fakeNativeRuntime) Check(context.Context)error {
 f.mu.Lock();defer f.mu.Unlock()
 f.checks++;return f.checkErr
}
func (f *fakeNativeRuntime) Start(_ context.Context,_ string,_ string,env Environment,port int)(ProcessHandle,error) {
 f.mu.Lock();defer f.mu.Unlock()
 f.launches=append(f.launches,port)
 return ProcessHandle{ID:fmt.Sprintf("process-%d",port),PID:port,Port:port,URL:fmt.Sprintf("%s/%d/",f.healthURL,env.PRNumber)},nil
}
func (f *fakeNativeRuntime) Stop(_ context.Context,s ProcessHandle)error {
 f.mu.Lock();defer f.mu.Unlock()
 f.stops=append(f.stops,s.ID);return nil
}
func TestManagerStartsSpecificPRReusesAndStopsOnlySelectedInstance(t *testing.T) {
 sha1:=strings.Repeat("a",40);sha2:=strings.Repeat("b",40)
 health:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if strings.HasSuffix(r.URL.Path,"/api/health") {
  w.Header().Set("Content-Type","application/json")
  parts:=strings.Split(strings.Trim(r.URL.Path,"/"),"/")
  number:=0
  _,_=fmt.Sscanf(parts[0],"%d",&number)
  sha:=strings.Repeat("a",40)
  if number==102{sha=strings.Repeat("b",40)}
  fmt.Fprintf(w,`{"ok":true,"preview_mode":true,"preview_pr":%d,"preview_sha":"%s"}`,number,sha)
  return
 }
  http.NotFound(w,r)
 }))
 defer health.Close()
 builds:=map[int]string{101:sha1,102:sha2}
 api:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  switch r.URL.Path {
  case "/repos/silverkhan/TaskMecca/pulls":
   fmt.Fprintf(w,`[
    {"number":101,"title":"A","base":{"ref":"dev"},"head":{"sha":"%s","ref":"feature-a","repo":{"full_name":"silverkhan/TaskMecca"}}},
    {"number":102,"title":"B","base":{"ref":"dev"},"head":{"sha":"%s","ref":"feature-b","repo":{"full_name":"silverkhan/TaskMecca"}}}
   ]`,sha1,sha2)
  case "/repos/silverkhan/TaskMecca/actions/workflows/go-artifacts.yml/runs":
   fmt.Fprint(w,`{"workflow_runs":[
    {"id":1001,"event":"pull_request","status":"completed","conclusion":"success","pull_requests":[{"number":101}]},
    {"id":1002,"event":"pull_request","status":"completed","conclusion":"success","pull_requests":[{"number":102}]}
   ]}`)
  case "/repos/silverkhan/TaskMecca/actions/runs/1001/artifacts":
   fmt.Fprintf(w,`{"artifacts":[{"id":5001,"name":"task-mecca-preview-pr-101-%s"}]}`,sha1)
  case "/repos/silverkhan/TaskMecca/actions/runs/1002/artifacts":
   fmt.Fprintf(w,`{"artifacts":[{"id":5002,"name":"task-mecca-preview-pr-102-%s"}]}`,sha2)
  case "/repos/silverkhan/TaskMecca/actions/artifacts/5001/zip":
   _,_=w.Write(testArchive(t,101,builds[101],1001,false,false,false))
  case "/repos/silverkhan/TaskMecca/actions/artifacts/5002/zip":
   _,_=w.Write(testArchive(t,102,builds[102],1002,false,false,false))
  default:
   http.Error(w,"unknown fixture",404)
  }
 }))
 defer api.Close()
 runner:=&fakeNativeRuntime{healthURL:health.URL}
 manager:=NewManager(t.TempDir())
 manager.Client=&Client{HTTP:api.Client(),APIURL:api.URL}
 manager.Runtime=runner
 first,err:=manager.Start(context.Background(),101)
 if err!=nil{t.Fatal(err)}
 if first.Status!="running"||first.HeadSHA!=sha1||first.Port==0||first.URL==""{t.Fatalf("not started: %+v",first)}
 same,err:=manager.Start(context.Background(),101)
 if err!=nil||same.Port!=first.Port{t.Fatalf("duplicate Preview launch: %+v %v",same,err)}
 other,err:=manager.Start(context.Background(),102)
 if err!=nil||other.Port==first.Port {t.Fatalf("second instance failed: %+v %v",other,err)}
 if len(runner.launches)!=2{t.Fatalf("expected exactly two launches: %+v",runner.launches)}
 if err=manager.Stop(context.Background(),101);err!=nil{t.Fatal(err)}
 if len(runner.stops)!=1||runner.stops[0]!=first.Process.ID{t.Fatalf("wrong Preview stopped: %+v",runner.stops)}
 statuses:=manager.List()
 if len(statuses)!=2||statuses[0].Status!="stopped"||statuses[1].Status!="running"{
  t.Fatalf("unexpected state after one stop: %+v",statuses)
 }
 if err=manager.Stop(context.Background(),101);err==nil{t.Fatal("double-stop should fail")}
}
func TestManagerCannotLaunchWithoutNativeRuntime(t *testing.T){
 manager:=NewManager(t.TempDir())
 manager.Runtime=nil
 _,err:=manager.Start(context.Background(),122)
 if err==nil{t.Fatal("runtime-less Preview launched")}
 state:=manager.List()
 if len(state)!=1||state[0].Status!="error"{t.Fatalf("failed state not visible: %+v",state)}
}
