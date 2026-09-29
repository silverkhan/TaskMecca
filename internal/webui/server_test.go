package webui

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"
)

func TestHandlerServesDashboardAPIsAndAssets(t *testing.T) {
    root:=t.TempDir()
    ledger:=filepath.Join(root,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(ledger,0755); err!=nil { t.Fatal(err) }
    task:=`# A-1 Web Task
- Agent: -
- 변경범위: -
- 선행: -
- 연관: -
- 설명: web fixture
`
    if err:=os.WriteFile(filepath.Join(ledger,"000001.A-1.web.todo.md"),[]byte(task),0644); err!=nil { t.Fatal(err) }

    handler,err:=Handler(root,"")
    if err!=nil { t.Fatal(err) }

    cases:=[]struct{
        path string
        status int
        contentType string
    }{
        {"/api/backlog-folders",200,"application/json"},
        {"/api/snapshot",200,"application/json"},
        {"/api/tasks/A-1",200,"application/json"},
        {"/api/manual?lang=en",200,"application/json"},
        {"/",200,"text/html"},
        {"/app.js",200,"javascript"},
        {"/style.css",200,"text/css"},
        {"/tasks/A-1",200,"text/html"},
    }
    for _,tc:=range cases {
        req:=httptest.NewRequest(http.MethodGet,tc.path,nil)
        rec:=httptest.NewRecorder()
        handler.ServeHTTP(rec,req)
        if rec.Code!=tc.status { t.Fatalf("%s status=%d body=%s",tc.path,rec.Code,rec.Body.String()) }
        if tc.contentType!="" && !contains(rec.Header().Get("Content-Type"),tc.contentType) {
            t.Fatalf("%s content-type=%q",tc.path,rec.Header().Get("Content-Type"))
        }
    }

    req:=httptest.NewRequest(http.MethodGet,"/api/snapshot",nil)
    rec:=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    payload:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&payload); err!=nil { t.Fatal(err) }
    all,ok:=payload["all_items"].(map[string]any)
    if !ok { t.Fatalf("all_items=%T",payload["all_items"]) }
    if _,ok:=all["A-1"]; !ok { t.Fatalf("snapshot=%+v",payload) }
    if _,ok:=payload["backlog_selection"]; !ok { t.Fatalf("missing backlog_selection") }

    req=httptest.NewRequest(http.MethodGet,"/api/tasks/A-404",nil)
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusNotFound { t.Fatalf("missing task status=%d",rec.Code) }
}

func contains(value,needle string) bool {
    return len(value)>=len(needle) && (value==needle || index(value,needle)>=0)
}

func index(value,needle string) int {
    for i:=0;i+len(needle)<=len(value);i++ {
        if value[i:i+len(needle)]==needle { return i }
    }
    return -1
}
