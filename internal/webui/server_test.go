package webui

import (
    "encoding/json"
    "net"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "testing"
    "time"

    "github.com/silverkhan/TaskMecca/internal/maintenance"
    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestHandlerServesDashboardAPIsAndAssets(t *testing.T) {
    oldIndexProvider:=stableReleaseNotesIndexProvider
    oldDetailProvider:=stableReleaseNoteProvider
    oldChannelOptionsProvider:=releaseChannelOptionsProvider
    oldChannelSwitchProvider:=releaseChannelSwitchProvider
    defer func(){
        stableReleaseNotesIndexProvider=oldIndexProvider
        stableReleaseNoteProvider=oldDetailProvider
        releaseChannelOptionsProvider=oldChannelOptionsProvider
        releaseChannelSwitchProvider=oldChannelSwitchProvider
    }()
    stableReleaseNotesIndexProvider=func()([]byte,error){
        return []byte(`{"releases":[{"version":"0.2.49"},{"version":"0.2.48"},{"version":"0.2.47"}]}`),nil
    }
    stableReleaseNoteProvider=func(version string)([]byte,error){
        return []byte(`{"version":"`+version+`","date":"2026-10-01","summary":{"ko":"원격","en":"Remote"},"sections":[],"migration":{"required":false}}`),nil
    }
    releaseChannelOptionsProvider=func(current string) maintenance.ReleaseChannelOptions {
        return maintenance.ReleaseChannelOptions{
            CurrentVersion:current,CurrentChannel:"stable",
            Stable:maintenance.ReleaseChannelTarget{Channel:"stable",Version:"0.2.49"},
            Dev:maintenance.ReleaseChannelTarget{Channel:"dev",Version:"0.2.50-dev.4"},
        }
    }
    releaseChannelSwitchProvider=func(current,target string)(maintenance.UpgradeResult,error){
        return maintenance.UpgradeResult{From:current,To:"0.2.50-dev.4",Channel:target},nil
    }

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

    runtimeEvent:=runtimeobs.ExecutionEvent{
        EventKind:"state",
        ObservedAt:time.Date(2026,10,1,12,0,0,0,time.UTC).Format(time.RFC3339Nano),
        AttemptID:"run-web-test",
        Provider:"codex",
        RuntimeAgentID:"agent-web",
        State:runtimeobs.StateRunning,
        EvidenceSource:runtimeobs.EvidenceHook,
        ObservationQuality:runtimeobs.QualityObserved,
    }
    if err:=runtimeobs.AppendExecutionEvent(root,runtimeEvent); err!=nil { t.Fatal(err) }

    handler,err:=Handler(root,"","0.2.4")
    if err!=nil { t.Fatal(err) }

    cases:=[]struct{
        path string
        status int
        contentType string
    }{
        {"/api/backlog-folders",200,"application/json"},
        {"/api/revision",200,"application/json"},
        {"/api/version",200,"application/json"},
        {"/api/release-notes?limit=2",200,"application/json"},
        {"/api/release-notes/0.2.49",200,"application/json"},
        {"/api/channel-options",200,"application/json"},
        {"/api/snapshot",200,"application/json"},
        {"/api/attention",200,"application/json"},
        {"/api/workload",200,"application/json"},
        {"/api/issues",200,"application/json"},
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

    req:=httptest.NewRequest(http.MethodGet,"/",nil)
    rec:=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if !contains(rec.Body.String(),"releaseUnreadPrompt") {
        t.Fatal("release unread prompt container missing from Web shell")
    }

    req=httptest.NewRequest(http.MethodGet,"/app.js",nil)
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    appJS:=rec.Body.String()
    for _,needle:=range []string{"renderReleaseUnreadPrompt","showAvailableUpdateNotes","globalUpdateChangesBtn"} {
        if !contains(appJS,needle) { t.Fatalf("app.js missing release note UX marker %q",needle) }
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

    for _,path:=range []string{"/api/attention","/api/workload","/api/issues"} {
        req=httptest.NewRequest(http.MethodGet,path,nil)
        rec=httptest.NewRecorder()
        handler.ServeHTTP(rec,req)
        if rec.Code!=http.StatusOK { t.Fatalf("%s status=%d body=%s",path,rec.Code,rec.Body.String()) }
        viewPayload:=map[string]any{}
        if err:=json.Unmarshal(rec.Body.Bytes(),&viewPayload); err!=nil { t.Fatal(err) }
        if _,ok:=viewPayload["backlog_selection"]; !ok { t.Fatalf("%s missing backlog_selection",path) }
        if viewPayload["project_path"]!=root { t.Fatalf("%s project_path=%v",path,viewPayload["project_path"]) }
    }

    req=httptest.NewRequest(http.MethodGet,"/api/workload",nil)
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    workloadPayload:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&workloadPayload); err!=nil { t.Fatal(err) }
    runtimeView,ok:=workloadPayload["runtime_observability"].(map[string]any)
    if !ok { t.Fatalf("runtime_observability=%T payload=%+v",workloadPayload["runtime_observability"],workloadPayload) }
    attempts,ok:=runtimeView["attempts"].([]any)
    if !ok || len(attempts)!=1 { t.Fatalf("runtime attempts=%T %+v",runtimeView["attempts"],runtimeView["attempts"]) }
    firstAttempt,ok:=attempts[0].(map[string]any)
    if !ok || firstAttempt["runtime_agent_id"]!="agent-web" { t.Fatalf("attempt=%+v",attempts[0]) }

    req=httptest.NewRequest(http.MethodPost,"/api/runtime/hooks",strings.NewReader(`{"provider":"codex"}`))
    req.Header.Set("Content-Type","application/json")
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusForbidden { t.Fatalf("runtime hooks without action header status=%d",rec.Code) }

    req=httptest.NewRequest(http.MethodPost,"/api/runtime/hooks",strings.NewReader(`{"provider":"codex"}`))
    req.Header.Set("Content-Type","application/json")
    req.Header.Set("X-Task-Mecca-Action","1")
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("runtime hooks status=%d body=%s",rec.Code,rec.Body.String()) }
    if _,err:=os.Stat(filepath.Join(root,".codex","hooks.json")); err!=nil { t.Fatalf("codex hook config not written: %v",err) }

    req=httptest.NewRequest(http.MethodGet,"/api/manual?lang=ko",nil)
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("manual status=%d body=%s",rec.Code,rec.Body.String()) }
    manual:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&manual); err!=nil { t.Fatal(err) }
    rootPrompt,ok:=manual["root_prompt"].(string)
    if !ok || !contains(rootPrompt,"이 세션에서는 Task Mecca의 Root로 동작해 주세요.") {
        t.Fatalf("root_prompt missing from manual payload: %+v",manual)
    }
    if manual["root_prompt_path"]!="_task_mecca/ROOT_PROMPT.md" {
        t.Fatalf("root_prompt_path=%v",manual["root_prompt_path"])
    }

    req=httptest.NewRequest(http.MethodGet,"/api/release-notes?limit=2",nil)
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("release notes status=%d body=%s",rec.Code,rec.Body.String()) }
    releaseIndex:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&releaseIndex); err!=nil { t.Fatal(err) }
    items,ok:=releaseIndex["items"].([]any)
    if !ok || len(items)!=2 { t.Fatalf("release notes items=%T %+v",releaseIndex["items"],releaseIndex["items"]) }
    if releaseIndex["has_more"]!=true { t.Fatalf("expected paginated release notes: %+v",releaseIndex) }

    req=httptest.NewRequest(http.MethodGet,"/api/release-notes/0.2.49",nil)
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("release note detail status=%d body=%s",rec.Code,rec.Body.String()) }
    releaseDetail:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&releaseDetail); err!=nil { t.Fatal(err) }
    if releaseDetail["version"]!="0.2.49" { t.Fatalf("release note version=%v",releaseDetail["version"]) }

    if safeReleaseNoteVersion("../VERSION") { t.Fatal("unsafe release note version accepted") }
    if !safeReleaseNoteVersion("0.2.49-dev.1") { t.Fatal("valid prerelease version rejected") }

    req=httptest.NewRequest(http.MethodPost,"/api/channel-switch",strings.NewReader(`{"channel":"dev"}`))
    req.Header.Set("Content-Type","application/json")
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusForbidden { t.Fatalf("channel switch without action header status=%d",rec.Code) }

    req=httptest.NewRequest(http.MethodPost,"/api/channel-switch",strings.NewReader(`{"channel":"dev"}`))
    req.Header.Set("Content-Type","application/json")
    req.Header.Set("X-Task-Mecca-Action","1")
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("channel switch status=%d body=%s",rec.Code,rec.Body.String()) }
    switched:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&switched); err!=nil { t.Fatal(err) }
    if switched["channel"]!="dev" { t.Fatalf("channel switch payload=%+v",switched) }

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


func TestRunAutoPublishesLocalStateBeforeDirectTLSReady(t *testing.T) {
    if runtime.GOOS=="darwin" {
        t.Skip("direct Tailscale TLS path is used on Windows/Linux")
    }

    home:=t.TempDir()
    t.Setenv("TASK_MECCA_HOME",home)
    project:=t.TempDir()
    ledger:=filepath.Join(project,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(ledger,0755); err!=nil { t.Fatal(err) }

    probe,err:=net.Listen("tcp","127.0.0.1:0")
    if err!=nil { t.Fatal(err) }
    port:=probe.Addr().(*net.TCPAddr).Port
    _=probe.Close()

    originalIP:=autoTailscaleIPv4Provider
    originalTLS:=directTailscaleTLSProvider
    tlsStarted:=make(chan struct{})
    releaseTLS:=make(chan struct{})
    autoTailscaleIPv4Provider=func() string { return "100.64.0.1" }
    directTailscaleTLSProvider=func() TLSInfo {
        close(tlsStarted)
        <-releaseTLS
        return TLSInfo{Error:"test delayed TLS"}
    }
    defer func() {
        autoTailscaleIPv4Provider=originalIP
        directTailscaleTLSProvider=originalTLS
    }()

    done:=make(chan error,1)
    go func() {
        done<-Run(Config{
            Project:project,Host:"auto",Port:port,OpenBrowser:false,Version:"test",
            InstanceID:"delayed-tls-instance",ControlToken:"delayed-tls-token",
        })
    }()

    select {
    case <-tlsStarted:
    case <-time.After(2*time.Second):
        close(releaseTLS)
        t.Fatal("direct TLS initialization did not start")
    }

    deadline:=time.Now().Add(2*time.Second)
    var state ServiceState
    for time.Now().Before(deadline) {
        state=ServiceStatus()
        if state.Running && state.LocalURL!="" { break }
        time.Sleep(25*time.Millisecond)
    }
    if !state.Running {
        close(releaseTLS)
        t.Fatal("local Web was not published while TLS initialization was blocked")
    }
    if state.TailscaleMode!="initializing" {
        close(releaseTLS)
        t.Fatalf("TailscaleMode=%q, want initializing",state.TailscaleMode)
    }

    close(releaseTLS)
    deadline=time.Now().Add(2*time.Second)
    for time.Now().Before(deadline) {
        state=ServiceStatus()
        if state.Running && state.TLSError=="test delayed TLS" { break }
        time.Sleep(25*time.Millisecond)
    }
    if state.TLSError!="test delayed TLS" {
        t.Fatalf("TLSError=%q, want delayed TLS failure",state.TLSError)
    }
    if !state.Running {
        t.Fatal("local Web stopped after direct TLS failure")
    }

    if _,err:=StopService(); err!=nil { t.Fatal(err) }
    select {
    case err:=<-done:
        if err!=nil { t.Fatal(err) }
    case <-time.After(3*time.Second):
        t.Fatal("Web did not stop after test")
    }
}
