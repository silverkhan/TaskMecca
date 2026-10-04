package webui

import (
    "fmt"
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

    "github.com/silverkhan/TaskMecca/goassets"
    "github.com/silverkhan/TaskMecca/internal/maintenance"
    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestSignatureThemeBranding(t *testing.T) {
    indexHTML,err:=goassets.Template.ReadFile(embeddedRoot+"/web/index.html")
    if err!=nil { t.Fatal(err) }
    html:=string(indexHTML)
    for _,needle:=range []string{"/logo.svg",`<option value="mecca">Mecca</option>`,`<option value="slate">Slate</option>`} {
        if !contains(html,needle) { t.Fatalf("index missing signature theme marker %q",needle) }
    }

    appAsset,err:=goassets.Template.ReadFile(embeddedRoot+"/web/app.js")
    if err!=nil { t.Fatal(err) }
    appJS:=string(appAsset)
    for _,needle:=range []string{"task-mecca-palette-version","localStorage.setItem('task-mecca-theme','dark')","localStorage.setItem('task-mecca-palette','mecca')"} {
        if !contains(appJS,needle) { t.Fatalf("app.js missing signature migration marker %q",needle) }
    }

    cssAsset,err:=goassets.Template.ReadFile(embeddedRoot+"/web/style.css")
    if err!=nil { t.Fatal(err) }
    css:=string(cssAsset)
    if !contains(css,`data-palette="mecca"`) { t.Fatal("style.css missing Mecca signature palette") }
    if contains(css,`data-palette="nocturne"`) { t.Fatal("style.css still exposes legacy Nocturne palette") }

    logo,err:=goassets.Template.ReadFile(embeddedRoot+"/web/logo.svg")
    if err!=nil { t.Fatal(err) }
    if !contains(string(logo),"<svg") { t.Fatal("logo.svg is not a valid SVG asset") }
}

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
        {"/logo.svg",200,"image/svg+xml"},
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
    for _,needle:=range []string{"renderReleaseUnreadPrompt","showAvailableUpdateNotes","globalVersionUpdateBtn","channelSwitchMark"} {
        if !contains(appJS,needle) { t.Fatalf("app.js missing release note UX marker %q",needle) }
    }
    if contains(appJS,"tf(confirmKey") {
        t.Fatal("runtime Hook action uses undefined tf() formatter")
    }
    if !contains(appJS,"t(confirmKey,{provider:providerLabel})") {
        t.Fatal("runtime Hook action confirmation must use the standard t() formatter")
    }
    for _,needle:=range []string{
        "runtime-attempt-disclosure",
        "runtime-attempt-summary",
        "runtime-attempt-glance",
        "openByDefault=attentionState||attentionFinding",
        "runtimeAttemptDisclosure",
        "runtimeTransitionDisclosure",
        "data-runtime-attempt-key",
        "data-runtime-history-key",
        "state.runtimeAttemptDisclosure[details.dataset.runtimeAttemptKey]=details.open",
        "state.runtimeTransitionDisclosure[details.dataset.runtimeHistoryKey]=details.open",
        "runtimeStorageOpen",
        "runtimeStoragePanel",
        "performRuntimeCleanup",
        "runtimeNeedsCheck",
        "runtimeCurrentValid",
        "runtimeRootSessions",
        "runtimeRootCard",
        "runtimeRootListPanel",
        "performRootSessionCleanup",
        "runtimeRootDisclosure",
        "data-runtime-root-key",
    } {
        if !contains(appJS,needle) { t.Fatalf("app.js missing compact runtime card state marker %q",needle) }
    }
    if contains(appJS,"transitions.length<=4?'open':''") {
        t.Fatal("runtime transition history must not auto-expand routine sessions")
    }

    req=httptest.NewRequest(http.MethodGet,"/api/snapshot",nil)
    rec=httptest.NewRecorder()
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

    req=httptest.NewRequest(http.MethodGet,"/api/runtime/hooks",nil)
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("runtime hooks GET status=%d body=%s",rec.Code,rec.Body.String()) }
    hookStatusPayload:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&hookStatusPayload); err!=nil { t.Fatal(err) }
    initialHookRows,ok:=hookStatusPayload["hooks"].([]any)
    if !ok || len(initialHookRows)!=2 { t.Fatalf("runtime hooks GET rows=%T %+v",hookStatusPayload["hooks"],hookStatusPayload["hooks"]) }

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

    req=httptest.NewRequest(http.MethodGet,"/api/workload",nil)
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    configuredPayload:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&configuredPayload); err!=nil { t.Fatal(err) }
    configuredRuntime,ok:=configuredPayload["runtime_observability"].(map[string]any)
    if !ok { t.Fatalf("configured runtime=%T",configuredPayload["runtime_observability"]) }
    hookRows,ok:=configuredRuntime["hooks"].([]any)
    if !ok || len(hookRows)!=2 { t.Fatalf("hook rows=%T %+v",configuredRuntime["hooks"],configuredRuntime["hooks"]) }
    codexObserved:=false
    for _,raw:=range hookRows {
        row,ok:=raw.(map[string]any); if !ok { continue }
        if row["provider"]=="codex" {
            if row["configured"]!=true { t.Fatalf("codex not configured: %+v",row) }
            if row["state"]!="observed" { t.Fatalf("codex state=%v row=%+v",row["state"],row) }
            observedEvents,ok:=row["observed_events"].(map[string]any)
            if !ok || observedEvents["start"]!=true { t.Fatalf("codex observed events=%+v",row["observed_events"]) }
            codexObserved=true
        }
    }
    if !codexObserved { t.Fatalf("codex hook status missing: %+v",hookRows) }

    req=httptest.NewRequest(http.MethodPost,"/api/runtime/hooks",strings.NewReader(`{"provider":"codex","action":"disable"}`))
    req.Header.Set("Content-Type","application/json")
    req.Header.Set("X-Task-Mecca-Action","1")
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("runtime hooks disable status=%d body=%s",rec.Code,rec.Body.String()) }
    disabledStatus,err:=runtimeobs.HookStatus(root,"codex")
    if err!=nil { t.Fatal(err) }
    if disabledStatus.Installed { t.Fatalf("codex hooks still installed after disable: %+v",disabledStatus) }

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


func TestWorkloadHookStatusSeparatesConfiguredFromObserved(t *testing.T) {
    root:=t.TempDir()
    ledger:=filepath.Join(root,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(ledger,0755); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(filepath.Join(ledger,"000001.A-1.todo.md"),[]byte("# A-1 Test\n- Agent: -\n- 변경범위: -\n- 선행: -\n- 연관: -\n"),0644); err!=nil { t.Fatal(err) }
    if _,err:=runtimeobs.EnsureHooks(root,"codex"); err!=nil { t.Fatal(err) }

    handler,err:=Handler(root,"","test")
    if err!=nil { t.Fatal(err) }
    req:=httptest.NewRequest(http.MethodGet,"/api/workload",nil)
    rec:=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("status=%d body=%s",rec.Code,rec.Body.String()) }

    payload:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&payload); err!=nil { t.Fatal(err) }
    runtimeView,ok:=payload["runtime_observability"].(map[string]any)
    if !ok { t.Fatalf("runtime_observability=%T",payload["runtime_observability"]) }
    hooks,ok:=runtimeView["hooks"].([]any)
    if !ok { t.Fatalf("hooks=%T",runtimeView["hooks"]) }
    found:=false
    for _,raw:=range hooks {
        row,ok:=raw.(map[string]any); if !ok || row["provider"]!="codex" { continue }
        found=true
        if row["configured"]!=true { t.Fatalf("configured=%v row=%+v",row["configured"],row) }
        if row["state"]!="verification_required" { t.Fatalf("state=%v row=%+v",row["state"],row) }
        if row["observed"]!=false { t.Fatalf("observed=%v row=%+v",row["observed"],row) }
    }
    if !found { t.Fatalf("codex hook row missing: %+v",hooks) }
}

func TestRuntimeWorkloadCapsTerminalCardsAndHistoryPaginates(t *testing.T) {
    root:=t.TempDir()
    backlogDir:=filepath.Join(root,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlogDir,0755); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(filepath.Join(backlogDir,"000001.A-1.todo.md"),[]byte("# A-1 Test\n- Agent: -\n- 변경범위: -\n- 선행: -\n- 연관: -\n"),0644); err!=nil { t.Fatal(err) }

    now:=time.Now().UTC()
    for i:=0;i<8;i++ {
        id:=fmt.Sprintf("run-history-%02d",i)
        startAt:=now.Add(-time.Duration(20-i)*time.Minute)
        start:=runtimeobs.ExecutionEvent{
            EventKind:"state",ObservedAt:startAt.Format(time.RFC3339Nano),AttemptID:id,
            Provider:"codex",SessionID:"session-history",RuntimeAgentID:id+"-agent",State:runtimeobs.StateRunning,
            EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
        }
        stop:=start
        stop.ObservedAt=startAt.Add(time.Minute).Format(time.RFC3339Nano)
        stop.State=runtimeobs.StateCompleted
        stop.Terminal=true
        for _,event:=range []runtimeobs.ExecutionEvent{start,stop} {
            if err:=runtimeobs.AppendExecutionEvent(root,event); err!=nil { t.Fatal(err) }
        }
    }
    active:=runtimeobs.ExecutionEvent{
        EventKind:"state",ObservedAt:now.Add(-time.Minute).Format(time.RFC3339Nano),
        AttemptID:"run-active",Provider:"codex",SessionID:"session-history",RuntimeAgentID:"agent-active",
        State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
    }
    if err:=runtimeobs.AppendExecutionEvent(root,active); err!=nil { t.Fatal(err) }

    handler,err:=Handler(root,"","test")
    if err!=nil { t.Fatal(err) }

    req:=httptest.NewRequest(http.MethodGet,"/api/workload",nil)
    rec:=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("workload status=%d body=%s",rec.Code,rec.Body.String()) }
    payload:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&payload); err!=nil { t.Fatal(err) }
    runtimeView,ok:=payload["runtime_observability"].(map[string]any)
    if !ok { t.Fatalf("runtime_observability=%T",payload["runtime_observability"]) }
    attempts,ok:=runtimeView["attempts"].([]any)
    if !ok || len(attempts)!=7 { t.Fatalf("visible attempts=%T len=%d payload=%+v",runtimeView["attempts"],len(attempts),runtimeView["attempts"]) }
    activeCount:=0
    terminalCount:=0
    for _,raw:=range attempts {
        row,ok:=raw.(map[string]any); if !ok { continue }
        if row["terminal"]==true { terminalCount++ } else { activeCount++ }
    }
    if activeCount!=1 || terminalCount!=6 { t.Fatalf("active=%d terminal=%d",activeCount,terminalCount) }
    historyMeta,ok:=runtimeView["history"].(map[string]any)
    if !ok { t.Fatalf("history=%T",runtimeView["history"]) }
    if historyMeta["total"]!=float64(8) || historyMeta["hidden"]!=float64(2) {
        t.Fatalf("history metadata=%+v",historyMeta)
    }

    req=httptest.NewRequest(http.MethodGet,"/api/runtime/history?page=2&page_size=3",nil)
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("history status=%d body=%s",rec.Code,rec.Body.String()) }
    page:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&page); err!=nil { t.Fatal(err) }
    items,ok:=page["items"].([]any)
    if !ok || len(items)!=3 { t.Fatalf("history items=%T %+v",page["items"],page["items"]) }
    if page["total"]!=float64(8) || page["total_pages"]!=float64(3) || page["page"]!=float64(2) {
        t.Fatalf("history page=%+v",page)
    }
}

func TestRuntimeSessionManagerClassifiesAndExposesStorageAPI(t *testing.T) {
    root:=t.TempDir()
    backlogDir:=filepath.Join(root,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlogDir,0755); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(filepath.Join(backlogDir,"000001.A-1.todo.md"),[]byte("# A-1 Test\n- Agent: -\n- 변경범위: -\n- 선행: -\n- 연관: -\n"),0644); err!=nil { t.Fatal(err) }

    now:=time.Now().UTC()
    events:=[]runtimeobs.ExecutionEvent{
        {
            EventKind:"state",ObservedAt:now.Add(-time.Minute).Format(time.RFC3339Nano),
            AttemptID:"run-current",Provider:"codex",RuntimeAgentID:"agent-current",
            State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
        },
        {
            EventKind:"state",ObservedAt:now.Add(-2*time.Hour).Format(time.RFC3339Nano),
            AttemptID:"run-stale",Provider:"codex",RuntimeAgentID:"agent-stale",
            State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
        },
        {
            EventKind:"state",ObservedAt:now.Add(-20*time.Minute).Format(time.RFC3339Nano),
            AttemptID:"run-done",Provider:"codex",RuntimeAgentID:"agent-done",
            State:runtimeobs.StateCompleted,Terminal:true,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
        },
    }
    for _,event:=range events {
        if err:=runtimeobs.AppendExecutionEvent(root,event); err!=nil { t.Fatal(err) }
    }

    handler,err:=Handler(root,"","test")
    if err!=nil { t.Fatal(err) }

    req:=httptest.NewRequest(http.MethodGet,"/api/workload",nil)
    rec:=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("workload status=%d body=%s",rec.Code,rec.Body.String()) }
    payload:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&payload); err!=nil { t.Fatal(err) }
    runtimeView,ok:=payload["runtime_observability"].(map[string]any)
    if !ok { t.Fatalf("runtime_observability=%T",payload["runtime_observability"]) }
    groups,ok:=runtimeView["session_groups"].(map[string]any)
    if !ok { t.Fatalf("session_groups=%T %+v",runtimeView["session_groups"],runtimeView["session_groups"]) }
    if groups["current"]!=float64(1) || groups["needs_check"]!=float64(1) || groups["terminal"]!=float64(1) {
        t.Fatalf("session groups=%+v",groups)
    }

    req=httptest.NewRequest(http.MethodGet,"/api/runtime/storage",nil)
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("storage status=%d body=%s",rec.Code,rec.Body.String()) }
    storage:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&storage); err!=nil { t.Fatal(err) }
    if storage["total_bytes"].(float64)<=0 { t.Fatalf("storage=%+v",storage) }
    if _,ok:=storage["cleanup"].(map[string]any); !ok { t.Fatalf("cleanup preview missing: %+v",storage) }

    req=httptest.NewRequest(http.MethodPost,"/api/runtime/storage",strings.NewReader(`{"action":"cleanup"}`))
    req.Header.Set("Content-Type","application/json")
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusForbidden { t.Fatalf("cleanup without action header status=%d",rec.Code) }

    req=httptest.NewRequest(http.MethodPost,"/api/runtime/storage",strings.NewReader(`{"action":"cleanup"}`))
    req.Header.Set("Content-Type","application/json")
    req.Header.Set("X-Task-Mecca-Action","1")
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("cleanup status=%d body=%s",rec.Code,rec.Body.String()) }
}

func TestRuntimeRootSessionAPIAndCleanup(t *testing.T) {
    root:=t.TempDir()
    backlogDir:=filepath.Join(root,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlogDir,0755); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(filepath.Join(backlogDir,"000001.A-1.todo.md"),[]byte("# A-1 Test\n- Agent: -\n- 변경범위: -\n- 선행: -\n- 연관: -\n"),0644); err!=nil { t.Fatal(err) }

    now:=time.Now().UTC()
    old:=now.Add(-8*24*time.Hour)
    recent:=now.Add(-time.Hour)
    events:=[]runtimeobs.ExecutionEvent{
        {
            EventKind:"state",ObservedAt:old.Format(time.RFC3339Nano),AttemptID:"run-old-root",
            Provider:"codex",SessionID:"session-old",SessionName:"오래된 Root",
            RuntimeAgentID:"agent-old",State:runtimeobs.StateCompleted,Terminal:true,
            EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
        },
        {
            EventKind:"state",ObservedAt:recent.Format(time.RFC3339Nano),AttemptID:"run-recent-root",
            Provider:"codex",SessionID:"session-recent",SessionName:"최근 Root",
            RuntimeAgentID:"agent-recent",State:runtimeobs.StateCompleted,Terminal:true,
            EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
        },
    }
    for _,event:=range events {
        if err:=runtimeobs.AppendExecutionEvent(root,event); err!=nil { t.Fatal(err) }
    }
    ledger,err:=runtimeobs.ReconcileLedger(root,10,now)
    if err!=nil { t.Fatal(err) }
    if _,err:=runtimeobs.MaintainExecutionHistory(root,ledger,now); err!=nil { t.Fatal(err) }

    handler,err:=Handler(root,"","test")
    if err!=nil { t.Fatal(err) }

    req:=httptest.NewRequest(http.MethodGet,"/api/workload",nil)
    rec:=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("workload status=%d body=%s",rec.Code,rec.Body.String()) }
    workload:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&workload); err!=nil { t.Fatal(err) }
    runtimeView,ok:=workload["runtime_observability"].(map[string]any)
    if !ok { t.Fatalf("runtime=%T",workload["runtime_observability"]) }
    rootRows,ok:=runtimeView["root_sessions"].([]any)
    if !ok || len(rootRows)!=2 { t.Fatalf("root sessions=%T %+v",runtimeView["root_sessions"],runtimeView["root_sessions"]) }
    counts,ok:=runtimeView["root_session_counts"].(map[string]any)
    if !ok || counts["terminal"]!=float64(2) || counts["cleanup_eligible"]!=float64(1) {
        t.Fatalf("root counts=%+v",counts)
    }

    req=httptest.NewRequest(http.MethodGet,"/api/runtime/root-sessions?status=previous&include_storage=1&page=1&page_size=10",nil)
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("root list status=%d body=%s",rec.Code,rec.Body.String()) }
    page:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&page); err!=nil { t.Fatal(err) }
    items,ok:=page["items"].([]any)
    if !ok || len(items)!=2 { t.Fatalf("root items=%T %+v",page["items"],page["items"]) }
    cleanupID:=""
    for _,raw:=range items {
        row,ok:=raw.(map[string]any); if !ok { continue }
        if row["cleanup_eligible"]==true {
            cleanupID,_=row["root_session_id"].(string)
            if row["display_name"]!="오래된 Root" { t.Fatalf("cleanup row=%+v",row) }
            if row["storage_bytes"].(float64)<=0 { t.Fatalf("storage bytes missing: %+v",row) }
        }
    }
    if cleanupID=="" { t.Fatalf("cleanup root id missing: %+v",items) }

    body:=fmt.Sprintf(`{"action":"cleanup","root_session_id":%q}`,cleanupID)
    req=httptest.NewRequest(http.MethodPost,"/api/runtime/root-sessions",strings.NewReader(body))
    req.Header.Set("Content-Type","application/json")
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusForbidden { t.Fatalf("root cleanup without action header=%d",rec.Code) }

    req=httptest.NewRequest(http.MethodPost,"/api/runtime/root-sessions",strings.NewReader(body))
    req.Header.Set("Content-Type","application/json")
    req.Header.Set("X-Task-Mecca-Action","1")
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("root cleanup status=%d body=%s",rec.Code,rec.Body.String()) }

    req=httptest.NewRequest(http.MethodGet,"/api/runtime/root-sessions?status=previous&include_storage=1",nil)
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    remaining:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&remaining); err!=nil { t.Fatal(err) }
    if remaining["total"]!=float64(1) { t.Fatalf("remaining=%+v",remaining) }
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


func TestHandlerServesUninitializedBacklogAsNormalProject(t *testing.T) {
    root:=t.TempDir()
    if err:=os.MkdirAll(filepath.Join(root,"_task_mecca"),0755); err!=nil { t.Fatal(err) }

    handler,err:=Handler(root,"","test")
    if err!=nil { t.Fatal(err) }

    for _,path:=range []string{"/api/backlog/tasks","/api/snapshot","/api/attention","/api/workload","/api/issues"} {
        req:=httptest.NewRequest(http.MethodGet,path,nil)
        rec:=httptest.NewRecorder()
        handler.ServeHTTP(rec,req)
        if rec.Code!=http.StatusOK {
            t.Fatalf("%s status=%d body=%s",path,rec.Code,rec.Body.String())
        }
        payload:=map[string]any{}
        if err:=json.Unmarshal(rec.Body.Bytes(),&payload); err!=nil { t.Fatalf("%s: %v",path,err) }
        selection,ok:=payload["backlog_selection"].(map[string]any)
        if !ok { t.Fatalf("%s backlog_selection=%T",path,payload["backlog_selection"]) }
        if selected,ok:=selection["selected"].(string); !ok || selected!="" {
            t.Fatalf("%s selected=%#v",path,selection["selected"])
        }
    }

    req:=httptest.NewRequest(http.MethodGet,"/api/backlog/tasks",nil)
    rec:=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    payload:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&payload); err!=nil { t.Fatal(err) }
    presence,ok:=payload["backlog_presence"].(map[string]any)
    if !ok || presence["status"]!="uninitialized" {
        t.Fatalf("backlog_presence=%T %+v",payload["backlog_presence"],payload["backlog_presence"])
    }

    req=httptest.NewRequest(http.MethodGet,"/app.js",nil)
    rec=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    appJS:=rec.Body.String()
    for _,needle:=range []string{
        "backlogUninitializedTitle:'아직 등록된 작업이 없습니다.'",
        "backlogUninitializedBody:'첫 작업을 등록하면 백로그가 자동으로 생성됩니다.'",
        "backlogUninitializedTitle:'No tasks have been registered yet.'",
        "backlogUninitializedBody:'The backlog will be created automatically when the first task is registered.'",
        "presence.status === 'uninitialized'",
    } {
        if !contains(appJS,needle) { t.Fatalf("app.js missing uninitialized backlog UX marker %q",needle) }
    }
}


func TestWindowsRollbackScriptStopsFailedProcessBeforeRestore(t *testing.T) {
    script:=windowsRollbackScript(`C:\Program Files\Task Mecca\task-mecca.exe`,[]string{"web","--port","18765"},`C:\Work\My Project`,`C:\Program Files\Task Mecca\task-mecca.exe.previous`,18765)
    for _,needle:=range []string{"Start-Process -FilePath $exe","-PassThru","/api/health","Stop-Process -Id $p.Id -Force","$p.WaitForExit()","Copy-Item -Force","Start-Process -FilePath $exe"} {
        if !contains(script,needle) { t.Fatalf("Windows rollback helper missing %q",needle) }
    }
}

func TestPowerShellQuoteHandlesApostrophe(t *testing.T) {
    got:=psQuote(`C:\Users\O'Brien\Task Mecca`)
    if got!=`'C:\Users\O''Brien\Task Mecca'` { t.Fatalf("psQuote=%q",got) }
}


func TestUpgradeRecoveryStatusReadsPersistentNotice(t *testing.T) {
    home:=t.TempDir()
    t.Setenv("TASK_MECCA_HOME",home)
    dir:=filepath.Join(home,"web")
    if err:=os.MkdirAll(dir,0755); err!=nil { t.Fatal(err) }
    raw:=[]byte(`{"id":"rollback-1","status":"rolled_back","attempted_version":"0.2.99","at":"2026-10-04T14:00:00Z"}`)
    if err:=os.WriteFile(filepath.Join(dir,"upgrade-recovery.json"),raw,0600); err!=nil { t.Fatal(err) }
    notice:=UpgradeRecoveryStatus()
    if notice==nil || notice.ID!="rollback-1" || notice.Status!="rolled_back" || notice.AttemptedVersion!="0.2.99" {
        t.Fatalf("unexpected recovery notice: %#v",notice)
    }
}
