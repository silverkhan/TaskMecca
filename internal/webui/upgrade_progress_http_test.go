package webui

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "sync"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/maintenance"
)

func TestUpgradeStatusHTTPAndConcurrentRequestGate(t *testing.T){
 t.Setenv("TASK_MECCA_HOME",t.TempDir())
 project:=t.TempDir()
 if err:=os.MkdirAll(filepath.Join(project,"_task_mecca","data","backlog"),0700);err!=nil{t.Fatal(err)}
 registerWebFixture(t,project)
 original:=webUpgradeRunner
 defer func(){webUpgradeRunner=original}()
 started:=make(chan struct{})
 release:=make(chan struct{})
 var once sync.Once
 webUpgradeRunner=func(current string,report func(string))(maintenance.UpgradeResult,error){
  report("downloading")
  once.Do(func(){close(started)})
  <-release
  report("verifying")
  report("installing")
  return maintenance.UpgradeResult{From:current,To:current,RestartRequired:false},nil
 }
 handler,err:=Handler(project,"","test")
 if err!=nil{t.Fatal(err)}
 stopProjectAttentionFeedsForTest(t,project)
 call:=func(method,path string,authorized bool)*httptest.ResponseRecorder {
  request:=httptest.NewRequest(method,path,nil)
  if authorized{request.Header.Set("X-Task-Mecca-Action","1")}
  rec:=httptest.NewRecorder()
  handler.ServeHTTP(rec,request)
  return rec
 }
 if code:=call(http.MethodPost,"/api/upgrade",false).Code;code!=403{t.Fatalf("unguarded upgrade=%d",code)}
 if code:=call(http.MethodPost,"/api/upgrade-status",false).Code;code!=405{t.Fatalf("bad status method=%d",code)}
 result:=make(chan *httptest.ResponseRecorder,1)
 go func(){result<-call(http.MethodPost,"/api/upgrade",true)}()
 select{case <-started:case <-time.After(4*time.Second):t.Fatal("upgrade did not start")}
 state:=call(http.MethodGet,"/api/upgrade-status",false)
 if state.Code!=200{t.Fatalf("status %d",state.Code)}
 var progress upgradeProgressState
 if err:=json.Unmarshal(state.Body.Bytes(),&progress);err!=nil{t.Fatal(err)}
 if !progress.Active||progress.Phase!="downloading"||progress.ID==""{t.Fatalf("running=%+v",progress)}
 if code:=call(http.MethodPost,"/api/upgrade",true).Code;code!=409{t.Fatalf("duplicate was not blocked: %d",code)}
 close(release)
 select{
 case done:=<-result:if done.Code!=200{t.Fatalf("first upgrade=%d: %s",done.Code,done.Body.String())}
 case <-time.After(4*time.Second):t.Fatal("upgrade failed to finish")
 }
 state=call(http.MethodGet,"/api/upgrade-status",false)
 if err:=json.Unmarshal(state.Body.Bytes(),&progress);err!=nil{t.Fatal(err)}
 if progress.Phase!="completed"||progress.Active{t.Fatalf("final=%+v",progress)}
}
