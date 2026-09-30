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
    return handler(project,root,version,"","",nil,nil)
}

func handler(project,root,version,instanceID,controlToken string,restartCh chan<- maintenance.UpgradeResult,stopCh chan<- struct{}) (http.Handler,error) {
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
        writeJSON(w,map[string]any{"ok":true,"version":version,"instance_id":instanceID,"pid":os.Getpid()},200)
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
        stableKey:=func(payload map[string]any) string {
            parts:=[]string{}
            if rows,ok:=payload["attention"].([]map[string]any); ok {
                for _,row:=range rows {
                    parts=append(parts,strings.Join([]string{
                        fmt.Sprint(row["id"]),fmt.Sprint(row["type"]),fmt.Sprint(row["health"]),
                        fmt.Sprint(row["runtime_state"]),fmt.Sprint(row["last_activity_at"]),
                        fmt.Sprint(row["title"]),fmt.Sprint(row["message"]),fmt.Sprint(row["resume_condition"]),
                    },"|"))
                }
            }
            if events,ok:=payload["notification_events"].([]map[string]any); ok {
                for _,event:=range events { parts=append(parts,"event:"+fmt.Sprint(event["id"])) }
            }
            sort.Strings(parts)
            return strings.Join(parts,"\n")
        }
        last:=""
        send:=func() bool {
            payload,snapshotErr:=backlog.AttentionSnapshot(activeProject,selected,true)
            if snapshotErr!=nil {
                data,_:=json.Marshal(map[string]any{"error":snapshotErr.Error()})
                _,_=fmt.Fprintf(w,"event: error\ndata: %s\n\n",data)
                flusher.Flush()
                return false
            }
            current:=stableKey(payload)
            if current==last { return true }
            last=current
            data,_:=json.Marshal(payload)
            _,_=fmt.Fprintf(w,"event: attention\ndata: %s\n\n",data)
            flusher.Flush()
            return true
        }
        if !send() { return }
        ticker:=time.NewTicker(3*time.Second)
        keepalive:=time.NewTicker(15*time.Second)
        defer ticker.Stop(); defer keepalive.Stop()
        for {
            select {
            case <-r.Context().Done():
                return
            case <-ticker.C:
                if !send() { return }
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
    var remoteListener net.Listener
    localURL:=""
    tailscaleURL:=""
    candidateTailscaleURL:=""
    tailscaleDNS:=""
    tailscaleIP:=""
    tailscaleManaged:=false
    tailscaleMode:=""
    tlsError:=""
    bindHost:=""
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
        } else {
            tlsInfo:=ensureTailscaleTLS()
            if tlsInfo.Enabled {
                raw,listenErr:=listenTailscaleIP(tlsInfo.IP,port)
                if listenErr!=nil {
                    tlsError=fmt.Sprintf("Tailscale HTTPS cannot bind %s:%d: %v",tlsInfo.IP,port,listenErr)
                } else {
                    tlsConfig,tlsErr:=tailscaleTLSConfig(tlsInfo)
                    if tlsErr!=nil {
                        _=raw.Close()
                        tlsError=tlsErr.Error()
                    } else {
                        remoteListener=tls.NewListener(raw,tlsConfig)
                        candidateTailscaleURL=fmt.Sprintf("https://%s:%d/",tlsInfo.DNSName,port)
                        tailscaleMode="direct"
                    }
                }
            } else if tlsInfo.Error!="" {
                tlsError=tlsInfo.Error
            }
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
        if remoteListener!=nil { _=remoteListener.Close() }
        return err
    }

    server:=&http.Server{Handler:handler,ReadHeaderTimeout:5*time.Second}
    errCh:=make(chan error,2)
    go func(){ errCh<-server.Serve(primaryListener) }()
    listeners:=1
    if remoteListener!=nil {
        listeners++
        go func(){ errCh<-server.Serve(remoteListener) }()
        tailscaleURL=candidateTailscaleURL
    }

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
        if remoteListener!=nil { _=remoteListener.Close() }
        return err
    }
    defer removeServiceState(config.InstanceID,os.Getpid())

    fmt.Println("Task Mecca Web UI")
    if localURL!="" { fmt.Println("local:     "+localURL) }
    if tailscaleURL!="" {
        fmt.Println("tailscale: "+tailscaleURL)
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

    go func(){
        select {
        case result:=<-restartCh:
            time.Sleep(250*time.Millisecond)
            if restartErr:=scheduleWebRestart(result,config,port); restartErr!=nil {
                fmt.Fprintln(os.Stderr,"Task Mecca Web restart failed:",restartErr)
                return
            }
        case <-stopCh:
            time.Sleep(100*time.Millisecond)
        }
        shutdownCtx,cancel:=stdcontext.WithTimeout(stdcontext.Background(),2*time.Second)
        defer cancel()
        _=server.Shutdown(shutdownCtx)
    }()


    for i:=0;i<listeners;i++ {
        serveErr:=<-errCh
        if serveErr!=nil && serveErr!=http.ErrServerClosed {
            shutdownCtx,cancel:=stdcontext.WithTimeout(stdcontext.Background(),2*time.Second)
            _=server.Shutdown(shutdownCtx)
            cancel()
            return serveErr
        }
    }
    return nil
}

func LogEnabled() bool { return os.Getenv("TASK_MECCA_WEB_LOG")=="1" }
