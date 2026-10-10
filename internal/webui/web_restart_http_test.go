package webui

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "testing"

 "github.com/silverkhan/TaskMecca/internal/maintenance"
)

func TestWebRestartEndpointReusesManagedRestartQueue(t *testing.T) {
 t.Setenv("TASK_MECCA_HOME", t.TempDir())
 root:=t.TempDir()
 if err:=os.MkdirAll(filepath.Join(root,"_task_mecca","data","backlog"),0700);err!=nil{t.Fatal(err)}
 registerWebFixture(t,root)
 restart:=make(chan maintenance.UpgradeResult,1)
 app,err:=handler(root,"","dev-test","stable-service-id","control",restart,nil)
 if err!=nil{t.Fatal(err)}
 stopProjectAttentionFeedsForTest(t,root)
 request:=func(method,origin,remote,host string,header bool)*httptest.ResponseRecorder {
  req:=httptest.NewRequest(method,"http://"+host+"/api/admin/restart",nil)
  req.RemoteAddr=remote
  req.Host=host
  if origin!=""{req.Header.Set("Origin",origin)}
  if header{req.Header.Set("X-Task-Mecca-Action","1")}
  rec:=httptest.NewRecorder()
  app.ServeHTTP(rec,req)
  return rec
 }
 host:="127.0.0.1:18765"
 if got:=request("GET","","127.0.0.1:54321",host,true).Code;got!=405{t.Fatalf("method %d",got)}
 if got:=request("POST","","127.0.0.1:54321",host,false).Code;got!=403{t.Fatalf("missing header %d",got)}
 if got:=request("POST","https://evil.example","127.0.0.1:54321",host,true).Code;got!=403{t.Fatalf("foreign origin %d",got)}
 if got:=request("POST","","192.168.0.3:54321",host,true).Code;got!=403{t.Fatalf("LAN cannot restart Web %d",got)}
 rec:=request("POST","http://127.0.0.1:18765","127.0.0.1:54321",host,true)
 if rec.Code!=202{t.Fatalf("restart request %d: %s",rec.Code,rec.Body)}
 var accepted map[string]any
 if err:=json.Unmarshal(rec.Body.Bytes(),&accepted);err!=nil{t.Fatal(err)}
 if accepted["accepted"]!=true||accepted["boot_id"]==""{t.Fatalf("acceptance response: %+v",accepted)}
 select {
 case result:=<-restart:
  if !result.RestartRequired||result.Executable!=""||result.PreviousExecutable!=""{t.Fatalf("restart should not replace a binary: %+v",result)}
 default:t.Fatal("restart was not enqueued")
 }
}

func TestWebHealthBootIDChangesEvenWhenServiceInstanceIDIsReused(t *testing.T) {
 t.Setenv("TASK_MECCA_HOME",t.TempDir())
 root:=t.TempDir()
 if err:=os.MkdirAll(filepath.Join(root,"_task_mecca","data","backlog"),0700);err!=nil{t.Fatal(err)}
 registerWebFixture(t,root)
 read:=func()map[string]any{
  h,err:=handler(root,"","dev-test","same-managed-identity","control",nil,nil)
  if err!=nil{t.Fatal(err)}
  req:=httptest.NewRequest(http.MethodGet,"http://127.0.0.1:18765/api/health",nil)
  rec:=httptest.NewRecorder()
  h.ServeHTTP(rec,req)
  if rec.Code!=200{t.Fatalf("health %d",rec.Code)}
  var result map[string]any
  if err:=json.Unmarshal(rec.Body.Bytes(),&result);err!=nil{t.Fatal(err)}
  return result
 }
 first,second:=read(),read()
 stopProjectAttentionFeedsForTest(t,root)
 if first["instance_id"]!="same-managed-identity"||second["instance_id"]!=first["instance_id"]{t.Fatalf("service identity changed: %+v %+v",first,second)}
 if first["boot_id"]==""||second["boot_id"]==""||first["boot_id"]==second["boot_id"]{t.Fatalf("boot identity must change per process: %+v %+v",first,second)}
}
