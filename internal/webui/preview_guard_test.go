package webui

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/silverkhan/TaskMecca/internal/preview"
)

func TestPreviewGuardRejectsMutationsAndRiskyReadRoutes(t *testing.T){
    t.Setenv("TASK_MECCA_PREVIEW_MODE","1")
    reached:=0
    guarded:=previewGuard(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        reached++
        w.WriteHeader(http.StatusOK)
    }))
    denied:=[]struct{method,path string}{
        {"POST","/api/hub/projects"},{"POST","/api/channel-switch"},
        {"POST","/api/notifications/telegram"},{"POST","/api/notifications/push"},
        {"POST","/api/runtime/action"},{"DELETE","/api/project/remove"},
        {"GET","/api/notifications/history"},{"GET","/api/runtime/storage"},
        {"GET","/api/terminal/ws"},{"GET","/terminal"},
        {"GET","/api/preview/builds"},{"GET","/sw.js"},
        {"GET","/manifest.webmanifest"},{"GET","/api/web/logs"},
    }
    for _,tc:=range denied{
        w:=httptest.NewRecorder()
        guarded.ServeHTTP(w,httptest.NewRequest(tc.method,tc.path,nil))
        if w.Code!=http.StatusForbidden {
            t.Fatalf("Preview route %s %s status %d",tc.method,tc.path,w.Code)
        }
    }
    if reached!=0{t.Fatalf("denied routes reached handler %d times",reached)}
    for _,path:=range []string{"/","/app.js","/style.css","/api/health","/api/snapshot","/api/backlog/tasks"}{
        w:=httptest.NewRecorder()
        guarded.ServeHTTP(w,httptest.NewRequest("GET",path,nil))
        if w.Code!=http.StatusOK{t.Fatalf("expected safe view %s, got %d",path,w.Code)}
    }
    if reached!=6{t.Fatalf("unexpected allowed requests %d",reached)}
}

func TestPreviewWebRequiresExplicitLoopback(t *testing.T){
    t.Setenv("TASK_MECCA_PREVIEW_MODE","1")
    for _,host:=range []string{"auto","0.0.0.0","","100.100.218.126"}{
        if err:=Run(Config{Host:host,Port:0});err==nil||!strings.Contains(err.Error(),"loopback") {
            t.Fatalf("unsafe Preview host %q accepted: %v",host,err)
        }
    }
}

func TestPreviewGuardDoesNotAffectNormalDev(t *testing.T){
    t.Setenv("TASK_MECCA_PREVIEW_MODE","")
    g:=previewGuard(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(201)}))
    w:=httptest.NewRecorder()
    g.ServeHTTP(w,httptest.NewRequest("POST","/api/hub/projects",nil))
    if w.Code!=201{t.Fatalf("ordinary Dev API was blocked: %d",w.Code)}
}

func TestPreviewEnvironmentUsesTelegramTransportKillSwitch(t *testing.T){
    env,err:=preview.PrepareEnvironment(t.TempDir(),208,strings.Repeat("a",40))
    if err!=nil{t.Fatal(err)}
    child,err:=preview.ChildEnvironment([]string{"TASK_MECCA_TELEGRAM_TRANSPORT=enabled","GITHUB_TOKEN=secret"},env)
    if err!=nil{t.Fatal(err)}
    all:=strings.Join(child,"\n")
    if !strings.Contains(all,"TASK_MECCA_TELEGRAM_TRANSPORT=disabled")||strings.Contains(all,"secret"){
        t.Fatalf("Preview child transport not safe: %s",all)
    }
}
