package webui

import (
    stdcontext "context"
    "encoding/json"
    "errors"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"

    "github.com/silverkhan/TaskMecca/internal/preview"
)

func TestPreviewBuildListServerDevOnlyAndNeverExecutes(t *testing.T) {
    project:=t.TempDir()
    backlog:=filepath.Join(project,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlog,0755);err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(filepath.Join(backlog,"000001.B-1.preview.todo.md"),[]byte("# B-1 Preview test\n"),0644);err!=nil{t.Fatal(err)}
    registerWebFixture(t,project)
    original:=previewBuildsProvider
    defer func(){previewBuildsProvider=original}()
    called:=0
    previewBuildsProvider=func(_ stdcontext.Context,limit int)([]preview.Build,error){
        called++
        if limit!=7 {t.Fatalf("unexpected limit %d",limit)}
        return []preview.Build{{Number:218,Title:"Preview",HeadSHA:"0123456789012345678901234567890123456789",BuildStatus:"success",ExecutionAllowed:false}},nil
    }
    h,err:=Handler(project,"","0.3.1-dev.1")
    if err!=nil{t.Fatal(err)}

    t.Setenv("TASK_MECCA_CHANNEL","stable")
    w:=httptest.NewRecorder()
    h.ServeHTTP(w,httptest.NewRequest("GET","/api/preview/builds?limit=7",nil))
    if w.Code!=http.StatusForbidden||called!=0 {
        t.Fatalf("stable leaked Preview API status=%d body=%s called=%d",w.Code,w.Body.String(),called)
    }

    t.Setenv("TASK_MECCA_CHANNEL","dev")
    w=httptest.NewRecorder()
    h.ServeHTTP(w,httptest.NewRequest("POST","/api/preview/builds?limit=7",nil))
    if w.Code!=http.StatusMethodNotAllowed||called!=0 {t.Fatalf("POST accepted %d",w.Code)}
    w=httptest.NewRecorder()
    h.ServeHTTP(w,httptest.NewRequest("GET","/api/preview/builds?limit=0",nil))
    if w.Code!=http.StatusBadRequest||called!=0 {t.Fatalf("invalid limit accepted %d",w.Code)}

    w=httptest.NewRecorder()
    h.ServeHTTP(w,httptest.NewRequest("GET","/api/preview/builds?limit=7",nil))
    if w.Code!=http.StatusOK||called!=1 {t.Fatalf("dev list status=%d body=%s",w.Code,w.Body.String())}
    var response struct {
        Builds []preview.Build `json:"builds"`
        ExecutionAvailable bool `json:"execution_available"`
    }
    if err=json.Unmarshal(w.Body.Bytes(),&response);err!=nil{t.Fatal(err)}
    if len(response.Builds)!=1||response.Builds[0].ExecutionAllowed||response.ExecutionAvailable {
        t.Fatalf("preview execution incorrectly exposed: %+v",response)
    }
    previewBuildsProvider=func(stdcontext.Context,int)([]preview.Build,error){return nil,errors.New("fixture GitHub timeout")}
    w=httptest.NewRecorder()
    h.ServeHTTP(w,httptest.NewRequest("GET","/api/preview/builds",nil))
    if w.Code!=http.StatusBadGateway{t.Fatalf("discovery failure status=%d",w.Code)}
}

func TestPreviewBuildLookupFailureDoesNotReturnEmptySuccess(t *testing.T){
 project:=t.TempDir();old:=previewBuildsProvider;defer func(){previewBuildsProvider=old}()
 previewBuildsProvider=func(stdcontext.Context,int)([]preview.Build,error){return nil,&preview.APIError{Code:"github_rate_limited",Status:403}}
 t.Setenv("TASK_MECCA_CHANNEL","dev")
 h,err:=Handler(project,"","0.3.6-dev.test");if err!=nil{t.Fatal(err)}
 w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/api/preview/builds",nil))
 var body map[string]any;if err:=json.Unmarshal(w.Body.Bytes(),&body);err!=nil{t.Fatal(err)}
 if w.Code!=502||body["error_code"]!="github_rate_limited"{t.Fatal("missing lookup failure classification")}
 if _,exists:=body["builds"];exists{t.Fatal("lookup failure represented as empty builds")}
}
