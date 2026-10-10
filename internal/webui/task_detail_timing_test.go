package webui

import (
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestTaskDetailExposesTimingWithoutLosingBody(t *testing.T) {
 project:=t.TempDir()
 registerWebFixture(t,project)
 folder:=filepath.Join(project,"_task_mecca","data","backlog")
 if err:=os.MkdirAll(folder,0755);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(folder,"000001.A-1.detail.done.md"),[]byte("# A-1 Full detail\n## 작업 정의\nComplete body\n"),0644);err!=nil{t.Fatal(err)}
 handler,err:=Handler(project,"","test")
 if err!=nil{t.Fatal(err)}
 response:=httptest.NewRecorder()
 handler.ServeHTTP(response,httptest.NewRequest(http.MethodGet,"/api/tasks/A-1",nil))
 if response.Code!=200{t.Fatalf("status=%d body=%s",response.Code,response.Body.String())}
 timing:=response.Header().Get("Server-Timing")
 for _,phase:=range []string{"selection;dur=","catalog;dur=","readiness;dur=","control;dur=","projection;dur=","detail;dur="}{
  if !strings.Contains(timing,phase){t.Fatalf("missing %s in detail timing: %q",phase,timing)}
 }
 if !strings.Contains(response.Body.String(),"Full detail")||!strings.Contains(response.Body.String(),"raw_markdown"){
  t.Fatalf("full body not returned: %s",response.Body.String())
 }
}
