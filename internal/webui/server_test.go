package webui

import (
    "encoding/json"
    "net"
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

    handler,err:=Handler(root,"","0.2.4")
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
        {"/sw.js",200,"javascript"},
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


func TestTailscaleIPv4Range(t *testing.T) {
    cases:=[]struct{
        raw string
        want bool
    }{
        {"100.64.0.1",true},
        {"100.100.20.30",true},
        {"100.127.255.254",true},
        {"100.63.255.255",false},
        {"100.128.0.1",false},
        {"127.0.0.1",false},
        {"192.168.0.10",false},
    }
    for _,tc:=range cases {
        if got:=isTailscaleIPv4(net.ParseIP(tc.raw)); got!=tc.want {
            t.Fatalf("isTailscaleIPv4(%s)=%v want %v",tc.raw,got,tc.want)
        }
    }
}

func TestResolveWebHostExplicitAndLocalhost(t *testing.T) {
    host,mode:=resolveWebHost("localhost")
    if host!="127.0.0.1" || mode!="localhost" {
        t.Fatalf("localhost resolved to %q mode=%q",host,mode)
    }
    host,mode=resolveWebHost("0.0.0.0")
    if host!="0.0.0.0" || mode!="explicit" {
        t.Fatalf("explicit host resolved to %q mode=%q",host,mode)
    }
}


func TestFindListenerDoesNotIncrementOccupiedPort(t *testing.T) {
    occupied,err:=net.Listen("tcp","127.0.0.1:0")
    if err!=nil { t.Fatal(err) }
    defer occupied.Close()
    port:=occupied.Addr().(*net.TCPAddr).Port
    listener,actual,err:=findListener("127.0.0.1",port)
    if listener!=nil { listener.Close(); t.Fatal("listener unexpectedly succeeded on occupied port") }
    if err==nil { t.Fatal("expected occupied-port error") }
    if actual!=0 { t.Fatalf("unexpected fallback port: %d",actual) }
}

func TestManagedStopEndpointRequiresControlToken(t *testing.T) {
    root:=t.TempDir()
    ledger:=filepath.Join(root,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(ledger,0755); err!=nil { t.Fatal(err) }
    stopCh:=make(chan struct{},1)
    handler,err:=handler(root,"","0.2.17","instance-1","secret-token",nil,stopCh)
    if err!=nil { t.Fatal(err) }

    req:=httptest.NewRequest(http.MethodPost,"/api/admin/stop",nil)
    rec:=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusForbidden { t.Fatalf("missing token status=%d",rec.Code) }

    req=httptest.NewRequest(http.MethodPost,"/api/admin/stop",nil)
    req.Header.Set("X-Task-Mecca-Control","secret-token")
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("valid token status=%d body=%s",rec.Code,rec.Body.String()) }
    select {
    case <-stopCh:
    default:
        t.Fatal("stop signal was not delivered")
    }
}

func TestDefaultWebPortIsDedicated(t *testing.T) {
    if DefaultPort!=18765 { t.Fatalf("DefaultPort=%d",DefaultPort) }
}


func TestNormalizeManagedHostPromotesLegacyTailscaleIPToAuto(t *testing.T) {
    if got:=NormalizeManagedHost("100.100.218.126"); got!="auto" {
        t.Fatalf("NormalizeManagedHost tailscale=%q",got)
    }
    if got:=NormalizeManagedHost("127.0.0.1"); got!="127.0.0.1" {
        t.Fatalf("NormalizeManagedHost localhost=%q",got)
    }
    if got:=NormalizeManagedHost("auto"); got!="auto" {
        t.Fatalf("NormalizeManagedHost auto=%q",got)
    }
}
