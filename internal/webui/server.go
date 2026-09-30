package webui

import (
    "context"
    "encoding/json"
    "fmt"
    "io/fs"
    "mime"
    "net"
    "net/http"
    "net/url"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/goassets"
    "github.com/silverkhan/TaskMecca/internal/backlog"
    "github.com/silverkhan/TaskMecca/internal/install"
    "github.com/silverkhan/TaskMecca/internal/maintenance"
)

const embeddedRoot = "template/_task_mecca/framework"

type Config struct {
    Project string
    Root string
    Port int
    OpenBrowser bool
    Version string
}

type context struct {
    scanRoot string
    selected string
    explicit bool
}

func looksLikeBacklog(path string) bool {
    return strings.HasPrefix(strings.ToLower(filepath.Base(filepath.Clean(path))),"backlog")
}

func scanRoot(project,root string) string {
    taskMecca:=filepath.Join(project,"_task_mecca")
    base:=root
    if base=="" { base=taskMecca }
    if absolute,err:=filepath.Abs(base); err==nil { base=absolute }
    framework:=filepath.Join(taskMecca,"framework")
    if filepath.Clean(base)==filepath.Clean(taskMecca) || filepath.Clean(base)==filepath.Clean(framework) {
        return taskMecca
    }
    if looksLikeBacklog(base) { return filepath.Dir(base) }
    return base
}

func webContext(project,root string) (context,error) {
    scan:=scanRoot(project,root)
    candidates,err:=backlog.Discover(project,scan)
    if err!=nil { return context{},err }
    explicit:=""
    if root!="" && looksLikeBacklog(root) {
        explicit,err=filepath.Abs(root)
        if err!=nil { return context{},err }
    }
    selected:=explicit
    if selected=="" && len(candidates)>0 { selected=candidates[0].Path }
    if selected=="" { selected=filepath.Join(project,"_task_mecca","data","backlog") }
    return context{scanRoot:scan,selected:selected,explicit:explicit!=""},nil
}

func resolveBacklog(project string, ctx context, query url.Values) (string,[]backlog.Candidate,error) {
    candidates,err:=backlog.Discover(project,ctx.scanRoot)
    if err!=nil { return "",nil,err }
    allowed:=map[string]bool{}
    for _,candidate:=range candidates {
        absolute,_:=filepath.Abs(candidate.Path)
        allowed[filepath.Clean(absolute)]=true
    }
    requested:=query.Get("backlog")
    if requested!="" {
        if absolute,err:=filepath.Abs(requested); err==nil && allowed[filepath.Clean(absolute)] {
            return absolute,candidates,nil
        }
    }
    if ctx.selected!="" {
        absolute,_:=filepath.Abs(ctx.selected)
        if len(candidates)==0 || allowed[filepath.Clean(absolute)] { return absolute,candidates,nil }
    }
    if len(candidates)>0 { return candidates[0].Path,candidates,nil }
    return filepath.Join(project,"_task_mecca","data","backlog"),candidates,nil
}

func selectionPayload(ctx context, selected string, candidates []backlog.Candidate) map[string]any {
    clean:=[]map[string]any{}
    selectedAbs,_:=filepath.Abs(selected)
    for _,candidate:=range candidates {
        candidateAbs,_:=filepath.Abs(candidate.Path)
        clean=append(clean,map[string]any{
            "path":candidate.Path,"name":candidate.Name,"canonical":candidate.Canonical,
            "under_data":candidate.UnderData,"backlog_named":candidate.BacklogNamed,
            "record_count":candidate.RecordCount,"latest_modified":candidate.LatestModified,
            "selected":filepath.Clean(candidateAbs)==filepath.Clean(selectedAbs),
        })
    }
    return map[string]any{
        "scan_root":ctx.scanRoot,"selected":selected,
        "auto_selected":!ctx.explicit,"candidates":clean,
    }
}

func Handler(project,root,version string) (http.Handler,error) {
    return handler(project,root,version,nil)
}

func handler(project,root,version string,restartCh chan<- maintenance.UpgradeResult) (http.Handler,error) {
    if _,err:=webContext(project,root); err!=nil { return nil,err }
    mux:=http.NewServeMux()

    projectFor:=func(r *http.Request) string {
        requested:=strings.TrimSpace(r.URL.Query().Get("project"))
        if requested=="" { return project }
        if maintenance.IsRegisteredProject(requested) { if abs,err:=filepath.Abs(requested); err==nil { return abs } }
        return project
    }

    writeJSON:=func(w http.ResponseWriter,payload any,status int) {
        body,err:=json.Marshal(payload)
        if err!=nil { http.Error(w,err.Error(),500); return }
        w.Header().Set("Content-Type","application/json; charset=utf-8")
        w.Header().Set("Cache-Control","no-store")
        w.Header().Set("X-Content-Type-Options","nosniff")
        w.WriteHeader(status)
        _,_=w.Write(body)
    }

    mux.HandleFunc("/api/health",func(w http.ResponseWriter,r *http.Request) {
        writeJSON(w,map[string]any{"ok":true,"version":version},200)
    })

    mux.HandleFunc("/api/backlog-folders",func(w http.ResponseWriter,r *http.Request) {
        activeProject:=projectFor(r)
        activeCtx,ctxErr:=webContext(activeProject,"")
        if ctxErr!=nil { writeJSON(w,map[string]any{"error":ctxErr.Error()},500); return }
        selected,candidates,err:=resolveBacklog(activeProject,activeCtx,r.URL.Query())
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        writeJSON(w,selectionPayload(activeCtx,selected,candidates),200)
    })

    mux.HandleFunc("/api/snapshot",func(w http.ResponseWriter,r *http.Request) {
        activeProject:=projectFor(r)
        activeCtx,ctxErr:=webContext(activeProject,"")
        if ctxErr!=nil { writeJSON(w,map[string]any{"error":ctxErr.Error()},500); return }
        selected,candidates,err:=resolveBacklog(activeProject,activeCtx,r.URL.Query())
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        snapshot,err:=backlog.DashboardSnapshot(activeProject,selected,5)
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        snapshot["backlog_selection"]=selectionPayload(activeCtx,selected,candidates)
        snapshot["project_path"]=activeProject
        writeJSON(w,snapshot,200)
    })

    mux.HandleFunc("/api/manual",func(w http.ResponseWriter,r *http.Request) {
        lang:=strings.ToLower(r.URL.Query().Get("lang"))
        if lang=="" { lang="ko" }
        readme:="README.md"; session:="SESSION_GUIDE.md"
        if lang!="ko" { readme="README."+lang+".md"; session="SESSION_GUIDE."+lang+".md" }
        read:=func(name,fallback string) string {
            if data,err:=fs.ReadFile(goassets.Template,embeddedRoot+"/"+name); err==nil { return string(data) }
            if data,err:=fs.ReadFile(goassets.Template,embeddedRoot+"/"+fallback); err==nil { return string(data) }
            return ""
        }
        writeJSON(w,map[string]any{
            "readme":read(readme,"README.md"),
            "session_guide":read(session,"SESSION_GUIDE.md"),
            "language":lang,
        },200)
    })

    mux.HandleFunc("/api/hub",func(w http.ResponseWriter,r *http.Request) {
        projects:=maintenance.ListProjects()
        rows:=make([]map[string]any,0,len(projects))
        for _,p:=range projects {
            counts:=map[string]any{}
            if ctx,err:=webContext(p.Path,""); err==nil {
                if snap,err:=backlog.DashboardSnapshot(p.Path,ctx.selected,5); err==nil {
                    if c,ok:=snap["counts"].(map[string]any); ok { counts=c }
                }
            }
            rows=append(rows,map[string]any{"name":p.Name,"path":p.Path,"framework_version":p.FrameworkVersion,"last_seen":p.LastSeen,"counts":counts,"migration_available":p.FrameworkVersion!="" && p.FrameworkVersion!=version})
        }
        writeJSON(w,map[string]any{"cli":maintenance.CachedVersionInfo(version),"projects":rows,"current_project":project},200)
    })

    mux.HandleFunc("/api/upgrade",func(w http.ResponseWriter,r *http.Request) {
        if r.Method!="POST" { writeJSON(w,map[string]any{"error":"POST required"},405); return }
        if r.Header.Get("X-Task-Mecca-Action")!="1" { writeJSON(w,map[string]any{"error":"maintenance action header required"},403); return }
        result,err:=maintenance.Upgrade(version)
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        writeJSON(w,result,200)
        if result.RestartRequired && restartCh!=nil {
            select { case restartCh<-result: default: }
        }
    })

    mux.HandleFunc("/api/migrate",func(w http.ResponseWriter,r *http.Request) {
        if r.Method!="POST" { writeJSON(w,map[string]any{"error":"POST required"},405); return }
        if r.Header.Get("X-Task-Mecca-Action")!="1" { writeJSON(w,map[string]any{"error":"maintenance action header required"},403); return }
        var body struct{ Project string `json:"project"` }
        if err:=json.NewDecoder(r.Body).Decode(&body); err!=nil { writeJSON(w,map[string]any{"error":"invalid JSON"},400); return }
        target:=strings.TrimSpace(body.Project)
        if !maintenance.IsRegisteredProject(target) { writeJSON(w,map[string]any{"error":"project is not registered"},403); return }
        if err:=install.Migrate(target,version); err!=nil { writeJSON(w,map[string]any{"error":err.Error()},409); return }
        _=maintenance.RegisterProject(target)
        writeJSON(w,map[string]any{"ok":true,"project":target,"framework_version":version},200)
    })

    mux.HandleFunc("/api/tasks/",func(w http.ResponseWriter,r *http.Request) {
        activeProject:=projectFor(r)
        activeCtx,ctxErr:=webContext(activeProject,"")
        if ctxErr!=nil { writeJSON(w,map[string]any{"error":ctxErr.Error()},500); return }
        selected,_,err:=resolveBacklog(activeProject,activeCtx,r.URL.Query())
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        id:=strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(r.URL.Path,"/api/tasks/")))
        snapshot,err:=backlog.DashboardSnapshot(activeProject,selected,5)
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        all,ok:=snapshot["all_items"].(map[string]map[string]any)
        if !ok { writeJSON(w,map[string]any{"error":"task not found","id":id},404); return }
        item,ok:=all[id]
        if !ok { writeJSON(w,map[string]any{"error":"task not found","id":id},404); return }
        writeJSON(w,item,200)
    })

    sendEmbedded:=func(w http.ResponseWriter,name string) {
        data,err:=fs.ReadFile(goassets.Template,embeddedRoot+"/web/"+name)
        if err!=nil { http.Error(w,"not found",http.StatusNotFound); return }
        kind:=mime.TypeByExtension(filepath.Ext(name))
        if kind=="" { kind="application/octet-stream" }
        if strings.HasPrefix(kind,"text/") || strings.Contains(kind,"javascript") { kind=strings.Split(kind,";")[0]+"; charset=utf-8" }
        w.Header().Set("Content-Type",kind)
        w.Header().Set("Cache-Control","no-cache")
        w.Header().Set("X-Content-Type-Options","nosniff")
        w.Header().Set("Content-Security-Policy","default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' https://cdnjs.cloudflare.com https://cdn.jsdelivr.net; img-src 'self' data:; connect-src 'self' https://cdnjs.cloudflare.com https://cdn.jsdelivr.net")
        _,_=w.Write(data)
    }

    mux.HandleFunc("/style.css",func(w http.ResponseWriter,r *http.Request){ sendEmbedded(w,"style.css") })
    mux.HandleFunc("/app.js",func(w http.ResponseWriter,r *http.Request){ sendEmbedded(w,"app.js") })
    mux.HandleFunc("/vendor/",func(w http.ResponseWriter,r *http.Request){
        name:=strings.TrimPrefix(r.URL.Path,"/")
        clean:=filepath.ToSlash(filepath.Clean(name))
        if !strings.HasPrefix(clean,"vendor/") || strings.Contains(clean,"..") { http.NotFound(w,r); return }
        sendEmbedded(w,clean)
    })
    mux.HandleFunc("/",func(w http.ResponseWriter,r *http.Request){ sendEmbedded(w,"index.html") })

    return mux,nil
}

func findListener(host string,preferred int) (net.Listener,int,error) {
    if preferred<1 { preferred=1 }
    for port:=preferred;port<preferred+30;port++ {
        listener,err:=net.Listen("tcp",fmt.Sprintf("%s:%d",host,port))
        if err==nil { return listener,port,nil }
    }
    return nil,0,fmt.Errorf("사용 가능한 local Web UI port를 찾지 못했다: %d-%d",preferred,preferred+29)
}

func openBrowser(url string) {
    var cmd *exec.Cmd
    switch runtime.GOOS {
    case "darwin": cmd=exec.Command("open",url)
    case "windows": cmd=exec.Command("rundll32","url.dll,FileProtocolHandler",url)
    default: cmd=exec.Command("xdg-open",url)
    }
    _=cmd.Start()
}

func scheduleWebRestart(result maintenance.UpgradeResult,config Config,port int) error {
    exe:=result.Executable
    if exe=="" {
        var err error
        exe,err=os.Executable()
        if err!=nil { return err }
        exe,_=filepath.EvalSymlinks(exe)
    }
    args:=[]string{"web","--project",config.Project,"--port",fmt.Sprint(port),"--no-open"}
    if config.Root!="" { args=append(args,"--root",config.Root) }

    if runtime.GOOS=="windows" {
        helper,err:=os.CreateTemp("","task-mecca-web-restart-*.cmd")
        if err!=nil { return err }
        helperPath:=helper.Name()
        rootArg:=""
        if config.Root!="" { rootArg=" --root \\""+config.Root+"\\\"" }
        body:=fmt.Sprintf("@echo off\\r\\n:wait\\r\\nif exist \\"%s.new\\" (timeout /t 1 /nobreak >nul & goto wait)\\r\\ntimeout /t 1 /nobreak >nul\\r\\nstart \\"\\" /D \\"%s\\" \\"%s\\" web --project \\"%s\\" --port %d --no-open%s\\r\\ndel \\"%%~f0\\"\\r\\n",
            exe,config.Project,exe,config.Project,port,rootArg)
        if _,err=helper.WriteString(body); err!=nil { _=helper.Close(); return err }
        if err=helper.Close(); err!=nil { return err }
        cmd:=exec.Command("cmd","/C","start","\"Task Mecca Web Restart\"","/MIN",helperPath)
        cmd.Dir=config.Project
        return cmd.Start()
    }

    shellArgs:=append([]string{"-c","sleep 1; exec \"$@\"" ,"task-mecca-web-restart",exe},args...)
    cmd:=exec.Command("sh",shellArgs...)
    cmd.Dir=config.Project
    cmd.Stdout=os.Stdout
    cmd.Stderr=os.Stderr
    return cmd.Start()
}

func Run(config Config) error {
    restartCh:=make(chan maintenance.UpgradeResult,1)
    handler,err:=handler(config.Project,config.Root,config.Version,restartCh)
    if err!=nil { return err }
    listener,port,err:=findListener("127.0.0.1",config.Port)
    if err!=nil { return err }
    ctx,err:=webContext(config.Project,config.Root)
    if err!=nil { _=listener.Close(); return err }
    url:=fmt.Sprintf("http://127.0.0.1:%d/",port)
    fmt.Println("Task Mecca Web UI: "+url)
    if ctx.selected!="" {
        mode:="(auto)"; if ctx.explicit { mode="(explicit)" }
        fmt.Printf("backlog: %s %s\n",ctx.selected,mode)
    } else {
        fmt.Println("backlog: not initialized (Registrar creates data/backlog on first registration)")
    }
    fmt.Println("localhost only · maintenance actions require explicit UI confirmation · Ctrl+C to stop")
    access:=backlog.AccessObservation(config.Project)
    if access["restriction_current"]==true {
        fmt.Println("WARNING: current runtime restriction detected · subagent dispatch will remain blocked until a fresh preflight succeeds")
    } else if access["checked_at"]!=nil {
        fmt.Printf("access: last observed %s · fresh active preflight runs automatically before dispatch\n",strings.ToUpper(fmt.Sprint(access["status"])))
    } else {
        fmt.Println("access: not yet observed · fresh active preflight runs automatically before dispatch")
    }
    if config.OpenBrowser { go func(){ time.Sleep(200*time.Millisecond); openBrowser(url) }() }
    server:=&http.Server{Handler:handler,ReadHeaderTimeout:5*time.Second}
    go func(){
        result:=<-restartCh
        time.Sleep(250*time.Millisecond)
        if restartErr:=scheduleWebRestart(result,config,port); restartErr!=nil {
            fmt.Fprintln(os.Stderr,"Task Mecca Web restart failed:",restartErr)
            return
        }
        ctx,cancel:=context.WithTimeout(context.Background(),2*time.Second)
        defer cancel()
        _=server.Shutdown(ctx)
    }()
    err=server.Serve(listener)
    if err==http.ErrServerClosed { return nil }
    return err
}

func LogEnabled() bool { return os.Getenv("TASK_MECCA_WEB_LOG")=="1" }
