package webui

import (
    stdcontext "context"
    "crypto/tls"
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
    "strconv"
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/goassets"
    "github.com/silverkhan/TaskMecca/internal/backlog"
    "github.com/silverkhan/TaskMecca/internal/install"
    "github.com/silverkhan/TaskMecca/internal/maintenance"
    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

const embeddedRoot = "template/_task_mecca/framework"

type Config struct {
    Project string
    Root string
    Host string
    Port int
    OpenBrowser bool
    Version string
    InstanceID string
    ControlToken string
    StartedAt string
}

type context struct {
    scanRoot string
    selected string
    explicit bool
    candidates []backlog.Candidate
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
    candidates,err:=cachedBacklogDiscover(project,scan)
    if err!=nil { return context{},err }
    explicit:=""
    if root!="" && looksLikeBacklog(root) {
        explicit,err=filepath.Abs(root)
        if err!=nil { return context{},err }
    }
    selected:=explicit
    if selected=="" && len(candidates)>0 { selected=candidates[0].Path }
    return context{scanRoot:scan,selected:selected,explicit:explicit!="",candidates:candidates},nil
}

func resolveBacklog(project string, ctx context, query url.Values) (string,[]backlog.Candidate,error) {
    candidates:=ctx.candidates
    if candidates==nil {
        var err error
        candidates,err=backlog.Discover(project,ctx.scanRoot)
        if err!=nil { return "",nil,err }
    }
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
    return "",candidates,nil
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

var stableReleaseNotesIndexProvider = maintenance.StableReleaseNotesIndex
var stableReleaseNoteProvider = maintenance.StableReleaseNote
var releaseChannelOptionsProvider = maintenance.GetReleaseChannelOptions
var releaseChannelSwitchProvider = maintenance.SwitchChannel

func currentReleaseNote() (map[string]any,error) {
    data,err:=fs.ReadFile(goassets.Template,embeddedRoot+"/release-notes/current.json")
    if err!=nil { return nil,err }
    var payload map[string]any
    if err=json.Unmarshal(data,&payload); err!=nil { return nil,err }
    return payload,nil
}

func releaseNotesIndex() ([]map[string]any,error) {
    data,err:=stableReleaseNotesIndexProvider()
    if err==nil {
        var payload struct{ Releases []map[string]any `json:"releases"` }
        if decodeErr:=json.Unmarshal(data,&payload); decodeErr==nil { return payload.Releases,nil }
        err=json.Unmarshal(data,&payload)
    }
    current,currentErr:=currentReleaseNote()
    if currentErr==nil { return []map[string]any{current},nil }
    if err!=nil { return nil,err }
    return nil,currentErr
}

func releaseNoteDetail(version string) (map[string]any,error) {
    current,currentErr:=currentReleaseNote()
    if currentErr==nil && strings.TrimSpace(fmt.Sprint(current["version"]))==version { return current,nil }
    data,err:=stableReleaseNoteProvider(version)
    if err!=nil {
        if currentErr!=nil { return nil,fmt.Errorf("local release note: %v; remote release note: %w",currentErr,err) }
        return nil,err
    }
    var payload map[string]any
    if err=json.Unmarshal(data,&payload); err!=nil { return nil,err }
    return payload,nil
}

func safeReleaseNoteVersion(value string) bool {
    value=strings.TrimSpace(value)
    if value=="" { return false }
    for _,r:=range value {
        if (r>='0'&&r<='9')||(r>='A'&&r<='Z')||(r>='a'&&r<='z')||r=='.'||r=='-' { continue }
        return false
    }
    return !strings.Contains(value,"..")
}

func Handler(project,root,version string) (http.Handler,error) {
    return handler(project,root,version,"","",nil,nil)
}

func handler(project,root,version,instanceID,controlToken string,restartCh chan<- maintenance.UpgradeResult,stopCh chan<- struct{}) (http.Handler,error) {
    initialCtx,initialErr:=webContext(project,root)
    if initialErr!=nil { return nil,initialErr }
    if selected,_,selectErr:=resolveBacklog(project,initialCtx,url.Values{}); selectErr==nil {
        ensureAttentionFeed(project,selected)
    }
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
        writeJSON(w,map[string]any{"ok":true,"version":version,"instance_id":instanceID,"pid":os.Getpid()},200)
    })

    mux.HandleFunc("/api/version",func(w http.ResponseWriter,r *http.Request) {
        info:=maintenance.CachedVersionInfo(version)
        if r.URL.Query().Get("refresh")=="1" { info=maintenance.RefreshVersionInfo(version) }
        activeProject:=projectFor(r)
        projectInfo:=map[string]any{"path":activeProject,"framework_version":"","migration_available":false}
        for _,p:=range maintenance.ListProjects() {
            if filepath.Clean(p.Path)!=filepath.Clean(activeProject) { continue }
            projectInfo["framework_version"]=p.FrameworkVersion
            projectInfo["migration_available"]=p.FrameworkVersion!="" && p.FrameworkVersion!=version
            break
        }
        writeJSON(w,map[string]any{"cli":info,"project":projectInfo},200)
    })

    mux.HandleFunc("/api/release-notes",func(w http.ResponseWriter,r *http.Request) {
        releases,readErr:=releaseNotesIndex()
        if readErr!=nil { writeJSON(w,map[string]any{"error":readErr.Error()},500); return }
        offset:=0
        if raw:=r.URL.Query().Get("offset"); raw!="" { if value,e:=strconv.Atoi(raw); e==nil && value>=0 { offset=value } }
        limit:=20
        if raw:=r.URL.Query().Get("limit"); raw!="" { if value,e:=strconv.Atoi(raw); e==nil && value>0 { limit=value } }
        if limit>50 { limit=50 }
        if offset>len(releases) { offset=len(releases) }
        end:=offset+limit
        if end>len(releases) { end=len(releases) }
        items:=releases[offset:end]
        writeJSON(w,map[string]any{"items":items,"offset":offset,"limit":limit,"total":len(releases),"has_more":end<len(releases)},200)
    })

    mux.HandleFunc("/api/release-notes/",func(w http.ResponseWriter,r *http.Request) {
        version:=strings.TrimSpace(strings.TrimPrefix(r.URL.Path,"/api/release-notes/"))
        if !safeReleaseNoteVersion(version) { writeJSON(w,map[string]any{"error":"invalid release note version"},400); return }
        payload,readErr:=releaseNoteDetail(version)
        if readErr!=nil { writeJSON(w,map[string]any{"error":readErr.Error(),"version":version},404); return }
        writeJSON(w,payload,200)
    })

    mux.HandleFunc("/api/channel-options",func(w http.ResponseWriter,r *http.Request) {
        if r.Method!="GET" { writeJSON(w,map[string]any{"error":"GET required"},405); return }
        writeJSON(w,releaseChannelOptionsProvider(version),200)
    })

    mux.HandleFunc("/api/channel-switch",func(w http.ResponseWriter,r *http.Request) {
        if r.Method!="POST" { writeJSON(w,map[string]any{"error":"POST required"},405); return }
        if r.Header.Get("X-Task-Mecca-Action")!="1" { writeJSON(w,map[string]any{"error":"maintenance action header required"},403); return }
        var body struct{ Channel string `json:"channel"` }
        if err:=json.NewDecoder(r.Body).Decode(&body); err!=nil { writeJSON(w,map[string]any{"error":"invalid JSON"},400); return }
        result,switchErr:=releaseChannelSwitchProvider(version,body.Channel)
        if switchErr!=nil { writeJSON(w,map[string]any{"error":switchErr.Error()},409); return }
        writeJSON(w,result,200)
        if result.RestartRequired && restartCh!=nil {
            select { case restartCh<-result: default: }
        }
    })

    mux.HandleFunc("/api/revision",func(w http.ResponseWriter,r *http.Request) {
        activeProject:=projectFor(r)
        activeCtx,ctxErr:=webContext(activeProject,"")
        if ctxErr!=nil { writeJSON(w,map[string]any{"error":ctxErr.Error()},500); return }
        selected,_,err:=resolveBacklog(activeProject,activeCtx,r.URL.Query())
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        revision,err:=backlog.BacklogRevision(activeProject,selected)
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        revision["project_path"]=activeProject
        revision["backlog"]=selected
        writeJSON(w,revision,200)
    })


    mux.HandleFunc("/api/admin/stop",func(w http.ResponseWriter,r *http.Request) {
        if r.Method!="POST" { writeJSON(w,map[string]any{"error":"POST required"},405); return }
        if stopCh==nil || controlToken=="" || r.Header.Get("X-Task-Mecca-Control")!=controlToken {
            writeJSON(w,map[string]any{"error":"forbidden"},403); return
        }
        writeJSON(w,map[string]any{"ok":true},200)
        select { case stopCh<-struct{}{}: default: }
    })

    mux.HandleFunc("/api/backlog-folders",func(w http.ResponseWriter,r *http.Request) {
        activeProject:=projectFor(r)
        activeCtx,ctxErr:=webContext(activeProject,"")
        if ctxErr!=nil { writeJSON(w,map[string]any{"error":ctxErr.Error()},500); return }
        selected,candidates,err:=resolveBacklog(activeProject,activeCtx,r.URL.Query())
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        writeJSON(w,selectionPayload(activeCtx,selected,candidates),200)
    })

    mux.HandleFunc("/api/backlog/tasks",func(w http.ResponseWriter,r *http.Request) {
        activeProject:=projectFor(r)
        activeCtx,ctxErr:=webContext(activeProject,"")
        if ctxErr!=nil { writeJSON(w,map[string]any{"error":ctxErr.Error()},500); return }
        selected,candidates,err:=resolveBacklog(activeProject,activeCtx,r.URL.Query())
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        page:=1; if raw:=r.URL.Query().Get("page"); raw!="" { if value,e:=strconv.Atoi(raw); e==nil { page=value } }
        pageSize:=20; if raw:=r.URL.Query().Get("page_size"); raw!="" { if value,e:=strconv.Atoi(raw); e==nil { pageSize=value } }
        split:=func(value string) []string {
            out:=[]string{}; seen:=map[string]bool{}
            for _,part:=range strings.Split(value,",") {
                part=strings.TrimSpace(part)
                if part=="" || seen[part] { continue }
                seen[part]=true; out=append(out,part)
            }
            return out
        }
        statuses:=split(r.URL.Query().Get("status"))
        tags:=split(r.URL.Query().Get("tags"))
        result,err:=backlog.BacklogPage(activeProject,selected,page,pageSize,statuses,tags,r.URL.Query().Get("q"),r.URL.Query().Get("sort"))
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        result["backlog_selection"]=selectionPayload(activeCtx,selected,candidates)
        result["project_path"]=activeProject
        writeJSON(w,result,200)
    })

    mux.HandleFunc("/api/events",func(w http.ResponseWriter,r *http.Request) {
        activeProject:=projectFor(r)
        activeCtx,ctxErr:=webContext(activeProject,"")
        if ctxErr!=nil { http.Error(w,ctxErr.Error(),500); return }
        selected,_,err:=resolveBacklog(activeProject,activeCtx,r.URL.Query())
        if err!=nil { http.Error(w,err.Error(),500); return }
        flusher,ok:=w.(http.Flusher)
        if !ok { http.Error(w,"streaming unsupported",500); return }
        w.Header().Set("Content-Type","text/event-stream; charset=utf-8")
        w.Header().Set("Cache-Control","no-cache")
        w.Header().Set("Connection","keep-alive")
        w.Header().Set("X-Accel-Buffering","no")
        feed:=ensureAttentionFeed(activeProject,selected)
        ch,initial,cancel:=feed.subscribe()
        defer cancel()
        if len(initial)>0 {
            _,_=fmt.Fprintf(w,"event: attention\ndata: %s\n\n",initial)
            flusher.Flush()
        }
        keepalive:=time.NewTicker(15*time.Second)
        defer keepalive.Stop()
        for {
            select {
            case <-r.Context().Done():
                return
            case data,ok:=<-ch:
                if !ok { return }
                _,_=fmt.Fprintf(w,"event: attention\ndata: %s\n\n",data)
                flusher.Flush()
            case <-keepalive.C:
                _,_=fmt.Fprint(w,": keepalive\n\n")
                flusher.Flush()
            }
        }
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

    mux.HandleFunc("/api/attention",func(w http.ResponseWriter,r *http.Request) {
        activeProject:=projectFor(r)
        activeCtx,ctxErr:=webContext(activeProject,"")
        if ctxErr!=nil { writeJSON(w,map[string]any{"error":ctxErr.Error()},500); return }
        selected,candidates,err:=resolveBacklog(activeProject,activeCtx,r.URL.Query())
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        snapshot,err:=backlog.AttentionSnapshot(activeProject,selected,true)
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        snapshot["backlog_selection"]=selectionPayload(activeCtx,selected,candidates)
        snapshot["project_path"]=activeProject
        writeJSON(w,snapshot,200)
    })

    mux.HandleFunc("/api/workload",func(w http.ResponseWriter,r *http.Request) {
        activeProject:=projectFor(r)
        activeCtx,ctxErr:=webContext(activeProject,"")
        if ctxErr!=nil { writeJSON(w,map[string]any{"error":ctxErr.Error()},500); return }
        selected,candidates,err:=resolveBacklog(activeProject,activeCtx,r.URL.Query())
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        snapshot,err:=backlog.WorkloadSnapshot(activeProject,selected)
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        snapshot["backlog_selection"]=selectionPayload(activeCtx,selected,candidates)
        snapshot["project_path"]=activeProject
        writeJSON(w,snapshot,200)
    })

    mux.HandleFunc("/api/runtime/storage",func(w http.ResponseWriter,r *http.Request) {
        activeProject:=projectFor(r)
        now:=time.Now()
        ledger,err:=runtimeobs.ReconcileLedger(activeProject,10,now)
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }

        if r.Method=="GET" {
            report,reportErr:=runtimeobs.RuntimeStorageReport(activeProject,ledger,now)
            if reportErr!=nil { writeJSON(w,map[string]any{"error":reportErr.Error()},500); return }
            writeJSON(w,report,200)
            return
        }

        if r.Method!="POST" {
            writeJSON(w,map[string]any{"error":"GET or POST required"},405)
            return
        }
        if r.Header.Get("X-Task-Mecca-Action")!="1" {
            writeJSON(w,map[string]any{"error":"maintenance action header required"},403)
            return
        }
        var body struct{ Action string `json:"action"` }
        if err:=json.NewDecoder(r.Body).Decode(&body); err!=nil {
            writeJSON(w,map[string]any{"error":"invalid JSON"},400)
            return
        }
        if strings.ToLower(strings.TrimSpace(body.Action))!="cleanup" {
            writeJSON(w,map[string]any{"error":"action must be cleanup"},400)
            return
        }
        result,cleanupErr:=runtimeobs.CleanupRuntimeStorage(activeProject,ledger,now)
        if cleanupErr!=nil { writeJSON(w,map[string]any{"error":cleanupErr.Error()},500); return }
        refreshed,refreshErr:=runtimeobs.RuntimeStorageReport(activeProject,ledger,time.Now())
        if refreshErr!=nil { writeJSON(w,map[string]any{"error":refreshErr.Error(),"result":result},500); return }
        writeJSON(w,map[string]any{"ok":true,"result":result,"storage":refreshed},200)
    })

    mux.HandleFunc("/api/runtime/history",func(w http.ResponseWriter,r *http.Request) {
        if r.Method!="GET" { writeJSON(w,map[string]any{"error":"GET required"},405); return }
        activeProject:=projectFor(r)
        page,_:=strconv.Atoi(r.URL.Query().Get("page"))
        pageSize,_:=strconv.Atoi(r.URL.Query().Get("page_size"))
        provider:=strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
        stateFilter:=strings.ToLower(strings.TrimSpace(r.URL.Query().Get("state")))
        if provider!="" && provider!="codex" && provider!="claude" {
            writeJSON(w,map[string]any{"error":"provider must be codex or claude"},400)
            return
        }
        now:=time.Now()
        ledger,err:=runtimeobs.ReconcileLedger(activeProject,10,now)
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        _,_ = runtimeobs.MaybeMaintainExecutionHistory(activeProject,ledger,now)
        history,err:=runtimeobs.QueryExecutionHistory(activeProject,ledger,page,pageSize,provider,stateFilter,now)
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        writeJSON(w,history,200)
    })

    mux.HandleFunc("/api/runtime/hooks",func(w http.ResponseWriter,r *http.Request) {
        if r.Method!="POST" { writeJSON(w,map[string]any{"error":"POST required"},405); return }
        if r.Header.Get("X-Task-Mecca-Action")!="1" { writeJSON(w,map[string]any{"error":"maintenance action header required"},403); return }
        activeProject:=projectFor(r)
        var body struct{
            Provider string `json:"provider"`
            Action string `json:"action"`
        }
        if err:=json.NewDecoder(r.Body).Decode(&body); err!=nil { writeJSON(w,map[string]any{"error":"invalid JSON"},400); return }
        provider:=strings.ToLower(strings.TrimSpace(body.Provider))
        if provider=="" { provider="all" }
        action:=strings.ToLower(strings.TrimSpace(body.Action))
        if action=="" { action="enable" }
        if provider!="all" && provider!="codex" && provider!="claude" {
            writeJSON(w,map[string]any{"error":"provider must be codex, claude, or all"},400)
            return
        }
        if action!="enable" && action!="disable" {
            writeJSON(w,map[string]any{"error":"action must be enable or disable"},400)
            return
        }
        providers:=[]string{"codex","claude"}
        if provider!="all" { providers=[]string{provider} }
        results:=[]runtimeobs.HookSetup{}
        for _,name:=range providers {
            var setup runtimeobs.HookSetup
            var err error
            if action=="disable" {
                setup,err=runtimeobs.DisableHooks(activeProject,name)
            } else {
                setup,err=runtimeobs.EnsureHooks(activeProject,name)
            }
            if err!=nil { writeJSON(w,map[string]any{"error":err.Error(),"provider":name},500); return }
            results=append(results,setup)
        }
        writeJSON(w,map[string]any{"ok":true,"action":action,"hooks":results},200)
    })

    mux.HandleFunc("/api/issues",func(w http.ResponseWriter,r *http.Request) {
        activeProject:=projectFor(r)
        activeCtx,ctxErr:=webContext(activeProject,"")
        if ctxErr!=nil { writeJSON(w,map[string]any{"error":ctxErr.Error()},500); return }
        selected,candidates,err:=resolveBacklog(activeProject,activeCtx,r.URL.Query())
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        snapshot,err:=backlog.IssuesSnapshot(activeProject,selected)
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
        rootPrompt:=""
        if data,err:=fs.ReadFile(goassets.Template,"template/_task_mecca/ROOT_PROMPT.md"); err==nil { rootPrompt=string(data) }
        writeJSON(w,map[string]any{
            "readme":read(readme,"README.md"),
            "session_guide":read(session,"SESSION_GUIDE.md"),
            "root_prompt":rootPrompt,
            "root_prompt_path":"_task_mecca/ROOT_PROMPT.md",
            "language":lang,
        },200)
    })

    mux.HandleFunc("/api/hub",func(w http.ResponseWriter,r *http.Request) {
        projects:=maintenance.ListProjects()
        rows:=make([]map[string]any,0,len(projects))
        for _,p:=range projects {
            counts:=map[string]any{}
            if ctx,err:=webContext(p.Path,""); err==nil {
                if quick,err:=backlog.QuickCounts(p.Path,ctx.selected); err==nil {
                    counts=quick
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
        result,err:=install.MigrateWithResult(target,version)
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},409); return }
        _,_ = backlog.EnsureTagRegistry(target)
        _=maintenance.RegisterProject(target)
        writeJSON(w,map[string]any{
            "ok":true,"project":target,"framework_version":version,
            "from_version":result.FromVersion,"to_version":result.ToVersion,
            "instruction_refresh_required":result.InstructionRefreshRequired,
            "changed_instructions":result.ChangedInstructions,
            "legacy_bootstrap":result.LegacyBootstrap,
            "backup_path":result.BackupPath,
        },200)
    })

    mux.HandleFunc("/api/tasks/",func(w http.ResponseWriter,r *http.Request) {
        activeProject:=projectFor(r)
        activeCtx,ctxErr:=webContext(activeProject,"")
        if ctxErr!=nil { writeJSON(w,map[string]any{"error":ctxErr.Error()},500); return }
        selected,_,err:=resolveBacklog(activeProject,activeCtx,r.URL.Query())
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error()},500); return }
        id:=strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(r.URL.Path,"/api/tasks/")))
        item,err:=backlog.TaskDetail(activeProject,selected,id)
        if err!=nil { writeJSON(w,map[string]any{"error":err.Error(),"id":id},404); return }
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
    mux.HandleFunc("/sw.js",func(w http.ResponseWriter,r *http.Request){ sendEmbedded(w,"sw.js") })
    mux.HandleFunc("/vendor/",func(w http.ResponseWriter,r *http.Request){
        name:=strings.TrimPrefix(r.URL.Path,"/")
        clean:=filepath.ToSlash(filepath.Clean(name))
        if !strings.HasPrefix(clean,"vendor/") || strings.Contains(clean,"..") { http.NotFound(w,r); return }
        sendEmbedded(w,clean)
    })
    mux.HandleFunc("/",func(w http.ResponseWriter,r *http.Request){ sendEmbedded(w,"index.html") })

    return mux,nil
}

func isTailscaleIPv4(ip net.IP) bool {
    v4:=ip.To4()
    if v4==nil { return false }
    // Tailscale IPv4 addresses are allocated from 100.64.0.0/10.
    return v4[0]==100 && v4[1]>=64 && v4[1]<=127
}

func tailscaleIPv4() string {
    interfaces,err:=net.Interfaces()
    if err!=nil { return "" }
    for _,iface:=range interfaces {
        if iface.Flags&net.FlagUp==0 || iface.Flags&net.FlagLoopback!=0 { continue }
        addrs,err:=iface.Addrs()
        if err!=nil { continue }
        for _,addr:=range addrs {
            raw:=addr.String()
            if slash:=strings.IndexByte(raw,'/'); slash>=0 { raw=raw[:slash] }
            ip:=net.ParseIP(raw)
            if isTailscaleIPv4(ip) { return ip.String() }
        }
    }
    return ""
}

func resolveWebHost(requested string) (string,string) {
    requested=strings.TrimSpace(requested)
    if requested=="" || strings.EqualFold(requested,"auto") {
        if ts:=tailscaleIPv4(); ts!="" { return ts,"tailscale" }
        return "127.0.0.1","localhost"
    }
    if strings.EqualFold(requested,"localhost") { return "127.0.0.1","localhost" }
    return requested,"explicit"
}

func findListener(host string,port int) (net.Listener,int,error) {
    if port<1 { port=DefaultPort }
    listener,err:=net.Listen("tcp",fmt.Sprintf("%s:%d",host,port))
    if err!=nil {
        return nil,0,fmt.Errorf("Task Mecca Web cannot bind %s:%d: %w; another process may already use this port. Use --port <port> to choose another port",host,port,err)
    }
    return listener,port,nil
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
    args:=[]string{"web","--foreground","--project",config.Project,"--host",config.Host,"--port",fmt.Sprint(port),"--no-open","--web-instance-id",config.InstanceID,"--web-control-token",config.ControlToken}
    if config.Root!="" { args=append(args,"--root",config.Root) }

    handled,err:=prepareManagedWebRestart()
    if err!=nil { return err }
    if handled {
        // launchd owns the macOS background service with KeepAlive=true.
        // The caller will gracefully shut this process down; launchd then
        // starts the upgraded executable independently of the terminal session.
        return nil
    }
    return detachedWebRestart(exe,args,config.Project)
}

var autoTailscaleIPv4Provider = tailscaleIPv4
var directTailscaleTLSProvider = ensureTailscaleTLS

func publishDirectTailscaleFailure(instanceID,message string) {
    message=strings.TrimSpace(message)
    if message=="" { message="Tailscale HTTPS is unavailable" }
    if err:=updateServiceState(instanceID,func(state *ServiceState) {
        state.TailscaleURL=""
        state.TLSEnabled=false
        state.TLSError=message
        state.TailscaleManaged=false
        state.TailscaleMode=""
    }); err==nil {
        fmt.Println("tailscale HTTPS: unavailable")
        fmt.Println("  "+message)
        fmt.Println("  Local Web remains available; inspect: task-mecca web logs")
    }
}

func serveDirectTailscaleHTTPS(server *http.Server,port int,instanceID string,done <-chan struct{}) {
    tlsInfo:=directTailscaleTLSProvider()
    select {
    case <-done:
        return
    default:
    }

    if !tlsInfo.Enabled {
        publishDirectTailscaleFailure(instanceID,tlsInfo.Error)
        return
    }

    raw,listenErr:=listenTailscaleIP(tlsInfo.IP,port)
    if listenErr!=nil {
        publishDirectTailscaleFailure(instanceID,fmt.Sprintf("Tailscale HTTPS cannot bind %s:%d: %v",tlsInfo.IP,port,listenErr))
        return
    }

    tlsConfig,tlsErr:=tailscaleTLSConfig(tlsInfo)
    if tlsErr!=nil {
        _=raw.Close()
        publishDirectTailscaleFailure(instanceID,tlsErr.Error())
        return
    }

    remoteListener:=tls.NewListener(raw,tlsConfig)
    select {
    case <-done:
        _=remoteListener.Close()
        return
    default:
    }

    tailscaleURL:=fmt.Sprintf("https://%s:%d/",tlsInfo.DNSName,port)
    if err:=updateServiceState(instanceID,func(state *ServiceState) {
        state.TailscaleURL=tailscaleURL
        state.TLSEnabled=true
        state.TLSError=""
        state.TailscaleManaged=false
        state.TailscaleMode="direct"
    }); err!=nil {
        _=remoteListener.Close()
        return
    }
    fmt.Println("tailscale: "+tailscaleURL)

    serveErr:=server.Serve(remoteListener)
    if serveErr!=nil && serveErr!=http.ErrServerClosed {
        select {
        case <-done:
            return
        default:
            publishDirectTailscaleFailure(instanceID,"Tailscale HTTPS listener stopped: "+serveErr.Error())
        }
    }
}

func Run(config Config) error {
    if config.Port<=0 { config.Port=DefaultPort }
    if config.InstanceID=="" || config.ControlToken=="" {
        id,token,idErr:=NewServiceIdentity()
        if idErr!=nil { return idErr }
        config.InstanceID=id
        config.ControlToken=token
    }
    if config.StartedAt=="" { config.StartedAt=time.Now().Format(time.RFC3339) }

    restartCh:=make(chan maintenance.UpgradeResult,1)
    stopCh:=make(chan struct{},1)
    handler,err:=handler(config.Project,config.Root,config.Version,config.InstanceID,config.ControlToken,restartCh,stopCh)
    if err!=nil { return err }

    requestedHost:=strings.TrimSpace(config.Host)
    autoMode:=requestedHost=="" || strings.EqualFold(requestedHost,"auto")
    var primaryListener net.Listener
    localURL:=""
    tailscaleURL:=""
    candidateTailscaleURL:=""
    tailscaleDNS:=""
    tailscaleIP:=""
    tailscaleManaged:=false
    tailscaleMode:=""
    tlsError:=""
    bindHost:=""
    directTLSPending:=false
    port:=config.Port

    if autoMode {
        bindHost="127.0.0.1"
        primaryListener,port,err=findListener(bindHost,config.Port)
        if err!=nil { return err }
        localURL=fmt.Sprintf("http://127.0.0.1:%d/",port)

        if runtime.GOOS=="darwin" {
            dns,ip,statusErr:=tailscaleStatusInfo()
            if statusErr!=nil {
                tlsError=statusErr.Error()
            } else {
                serveURL,managed,serveErr:=ensureTailscaleServe(dns,port)
                if serveErr!=nil {
                    tlsError=serveErr.Error()
                } else {
                    candidateTailscaleURL=serveURL
                    tailscaleDNS=dns
                    tailscaleIP=ip
                    tailscaleManaged=managed
                    tailscaleMode="serve"
                }
            }
        } else if autoTailscaleIPv4Provider()!="" {
            // Windows/Linux HTTPS setup can involve a slow first certificate
            // provision. Publish the local service first and finish HTTPS in
            // the background so startup success never depends on that latency.
            directTLSPending=true
            tailscaleMode="initializing"
        }
    } else {
        host,hostMode:=resolveWebHost(requestedHost)
        bindHost=host
        primaryListener,port,err=findListener(host,config.Port)
        if err!=nil { return err }
        localURL=fmt.Sprintf("http://%s:%d/",host,port)
        if hostMode=="tailscale" {
            tlsError="explicit Tailscale IP binding uses HTTP; use --host auto for managed HTTPS"
        }
    }

    ctx,err:=webContext(config.Project,config.Root)
    if err!=nil {
        _=primaryListener.Close()
        return err
    }

    server:=&http.Server{Handler:handler,ReadHeaderTimeout:5*time.Second}
    primaryErrCh:=make(chan error,1)
    go func(){ primaryErrCh<-server.Serve(primaryListener) }()

    if runtime.GOOS=="darwin" && candidateTailscaleURL!="" && tailscaleDNS!="" && tailscaleIP!="" {
        verifyErr:=verifyTailscaleEndpoint(tailscaleDNS,tailscaleIP,port,config.InstanceID,2*time.Second)
        if verifyErr!=nil {
            firstErr:=verifyErr
            _,applyErr:=applyTailscaleServe(port)
            if applyErr==nil {
                verifyErr=verifyTailscaleEndpoint(tailscaleDNS,tailscaleIP,port,config.InstanceID,2*time.Second)
            } else {
                verifyErr=fmt.Errorf("Serve reapply failed after health check error (%v): %w",firstErr,applyErr)
            }

            if verifyErr!=nil && tailscaleManaged {
                beforeResetErr:=verifyErr
                if resetErr:=refreshOwnedTailscaleServe(tailscaleDNS,port); resetErr==nil {
                    verifyErr=verifyTailscaleEndpoint(tailscaleDNS,tailscaleIP,port,config.InstanceID,2*time.Second)
                } else {
                    verifyErr=fmt.Errorf("Serve hard reset failed after health check error (%v): %w",beforeResetErr,resetErr)
                }
            }
        }
        if verifyErr==nil {
            tailscaleURL=candidateTailscaleURL
            tlsError=""
        } else {
            tailscaleURL=""
            tlsError=verifyErr.Error()
        }
    }

    preferredURL:=localURL
    if preferredURL=="" { preferredURL=tailscaleURL }
    if err=writeServiceState(ServiceState{
        PID:os.Getpid(),InstanceID:config.InstanceID,ControlToken:config.ControlToken,
        Host:requestedHost,Port:port,URL:preferredURL,LocalURL:localURL,TailscaleURL:tailscaleURL,
        TLSEnabled:tailscaleURL!="",TLSError:tlsError,
        TailscaleManaged:tailscaleManaged,TailscaleMode:tailscaleMode,
        Version:config.Version,Project:config.Project,StartedAt:config.StartedAt,Running:true,
    }); err!=nil {
        _=primaryListener.Close()
        return err
    }
    defer removeServiceState(config.InstanceID,os.Getpid())

    fmt.Println("Task Mecca Web UI")
    if localURL!="" { fmt.Println("local:     "+localURL) }
    if tailscaleURL!="" {
        fmt.Println("tailscale: "+tailscaleURL)
    } else if directTLSPending {
        fmt.Println("tailscale HTTPS: initializing in background")
    } else if autoMode && tailscaleIPv4()!="" {
        fmt.Println("tailscale HTTPS: unavailable")
        if tlsError!="" { fmt.Println("  "+tlsError) }
        if runtime.GOOS=="darwin" {
            fmt.Println("  Task Mecca could not configure Tailscale Serve; inspect: task-mecca web logs")
        } else {
            fmt.Println("  Direct HTTPS was not verified; inspect: task-mecca web logs")
        }
    }
    if !autoMode { fmt.Println("network: explicit bind · "+bindHost) }

    if ctx.selected!="" {
        mode:="(auto)"; if ctx.explicit { mode="(explicit)" }
        fmt.Printf("backlog: %s %s\n",ctx.selected,mode)
    } else {
        fmt.Println("backlog: not initialized (Registrar creates data/backlog on first registration)")
    }
    fmt.Println("maintenance actions require explicit UI confirmation · Ctrl+C to stop")
    access:=backlog.AccessObservation(config.Project)
    if access["restriction_current"]==true {
        fmt.Println("WARNING: current runtime restriction detected · subagent dispatch will remain blocked until a fresh preflight succeeds")
    } else if access["checked_at"]!=nil {
        fmt.Printf("access: last observed %s · fresh active preflight runs automatically before dispatch\n",strings.ToUpper(fmt.Sprint(access["status"])))
    } else {
        fmt.Println("access: not yet observed · fresh active preflight runs automatically before dispatch")
    }

    if config.OpenBrowser && localURL!="" {
        go func(){ time.Sleep(200*time.Millisecond); openBrowser(localURL) }()
    }

    lifecycleDone:=make(chan struct{})
    if directTLSPending {
        go serveDirectTailscaleHTTPS(server,port,config.InstanceID,lifecycleDone)
    }

    shutdown:=func() error {
        close(lifecycleDone)
        shutdownCtx,cancel:=stdcontext.WithTimeout(stdcontext.Background(),2*time.Second)
        shutdownErr:=server.Shutdown(shutdownCtx)
        cancel()
        serveErr:=<-primaryErrCh
        if shutdownErr!=nil { return shutdownErr }
        if serveErr!=nil && serveErr!=http.ErrServerClosed { return serveErr }
        return nil
    }

    for {
        select {
        case result:=<-restartCh:
            time.Sleep(250*time.Millisecond)
            if restartErr:=scheduleWebRestart(result,config,port); restartErr!=nil {
                fmt.Fprintln(os.Stderr,"Task Mecca Web restart failed:",restartErr)
                continue
            }
            return shutdown()
        case <-stopCh:
            time.Sleep(100*time.Millisecond)
            return shutdown()
        case serveErr:=<-primaryErrCh:
            close(lifecycleDone)
            if serveErr!=nil && serveErr!=http.ErrServerClosed { return serveErr }
            return nil
        }
    }
}

func LogEnabled() bool { return os.Getenv("TASK_MECCA_WEB_LOG")=="1" }
