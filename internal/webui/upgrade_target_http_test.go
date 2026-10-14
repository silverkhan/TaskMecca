package webui

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "github.com/silverkhan/TaskMecca/internal/maintenance"
)

func TestSelectedWebUpgradeSkipsDuplicateLatestDiscoveryButKeepsLegacy(t *testing.T){
 t.Setenv("TASK_MECCA_HOME",t.TempDir())
 project:=t.TempDir()
 if err:=os.MkdirAll(filepath.Join(project,"_task_mecca","data","backlog"),0700);err!=nil{t.Fatal(err)}
 registerWebFixture(t,project)
 originalLegacy,originalTarget:=webUpgradeRunner,webUpgradeTargetRunner
 defer func(){webUpgradeRunner=originalLegacy;webUpgradeTargetRunner=originalTarget}()
 legacyCalls,targetCalls:=0,0
 webUpgradeRunner=func(current string,report func(string))(maintenance.UpgradeResult,error){
  legacyCalls++;report("checking")
  return maintenance.UpgradeResult{From:current,To:current},nil
 }
 webUpgradeTargetRunner=func(current,target string,report func(string))(maintenance.UpgradeResult,error){
  targetCalls++;report("checking");report("downloading");report("verifying")
  if target!="0.3.1-dev.30"{t.Errorf("expected pinned target, got %q",target)}
  return maintenance.UpgradeResult{From:current,To:current},nil
 }
 handler,err:=Handler(project,"","0.3.1-dev.29")
 if err!=nil{t.Fatal(err)}
 stopProjectAttentionFeedsForTest(t,project)
 call:=func(body string,header bool)*httptest.ResponseRecorder{
  req:=httptest.NewRequest(http.MethodPost,"/api/upgrade",strings.NewReader(body))
  if header {req.Header.Set("X-Task-Mecca-Action","1")}
  w:=httptest.NewRecorder()
  handler.ServeHTTP(w,req)
  return w
 }
 if res:=call(`{"expected_version":"0.3.1-dev.30"}`,false);res.Code!=403 {t.Fatalf("unauthorized version upgrade returned %d",res.Code)}
 if res:=call(`{"expected_version":"0.3.1-dev.30"}`,true);res.Code!=200{t.Fatalf("pinned request failed: %d %s",res.Code,res.Body.String())}
 if targetCalls!=1||legacyCalls!=0{t.Fatalf("selected update rediscovered latest: target=%d legacy=%d",targetCalls,legacyCalls)}
 if res:=call("",true);res.Code!=200{t.Fatalf("legacy request failed: %d",res.Code)}
 if targetCalls!=1||legacyCalls!=1{t.Fatalf("legacy compat broken: target=%d legacy=%d",targetCalls,legacyCalls)}
 for _,body:=range []string{`{"expected_version":"v0.3.1-dev.30","bogus":1}`,`{"expected_version":`,`{"expected_version":"`+strings.Repeat("9",600)+`"}`} {
  if res:=call(body,true);res.Code!=400{t.Fatalf("malformed request returned %d: %q",res.Code,body[:min(len(body),50)])}
 }
 if targetCalls!=1||legacyCalls!=1{t.Fatal("invalid payload attempted installation")}
 var status upgradeProgressState
 req:=httptest.NewRequest(http.MethodGet,"/api/upgrade-status",nil)
 rr:=httptest.NewRecorder();handler.ServeHTTP(rr,req)
 if err:=json.Unmarshal(rr.Body.Bytes(),&status);err!=nil{t.Fatal(err)}
 if status.Phase!="completed"{t.Fatalf("last update phase %+v",status)}
}
