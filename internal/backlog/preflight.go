package backlog

import (
    "github.com/silverkhan/TaskMecca/internal/projectguard"
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "strings"
    "time"
)

func sandboxMarkerState(raw string) string {
    value:=strings.ToLower(strings.TrimSpace(raw))
    if value=="" { return "absent" }
    restricted:=map[string]bool{"1":true,"true":true,"yes":true,"on":true,"sandbox":true,"read-only":true,"workspace-write":true,"seatbelt":true,"landlock":true,"seccomp":true}
    full:=map[string]bool{"0":true,"false":true,"no":true,"off":true,"none":true,"full-access":true,"danger-full-access":true,"unrestricted":true}
    if restricted[value] { return "restricted" }
    if full[value] { return "full_hint" }
    return "unknown"
}

func probeDirectoryWrite(directory,label string) map[string]any {
    token:=fmt.Sprintf(".task_mecca_access_probe_%d_%d",os.Getpid(),time.Now().UnixNano())
    path:=filepath.Join(directory,token)
    if err:=os.MkdirAll(directory,0755); err!=nil {
        return map[string]any{"name":label,"ok":false,"status":"fail","path":directory,"error":fmt.Sprintf("%T: %v",err,err)}
    }
    file,err:=os.OpenFile(path,os.O_WRONLY|os.O_CREATE|os.O_EXCL,0644)
    if err==nil {
        _,writeErr:=file.WriteString("task-mecca-access-probe\n")
        closeErr:=file.Close()
        removeErr:=os.Remove(path)
        if writeErr==nil && closeErr==nil && removeErr==nil {
            return map[string]any{"name":label,"ok":true,"status":"pass","path":directory}
        }
        if writeErr!=nil { err=writeErr } else if closeErr!=nil { err=closeErr } else { err=removeErr }
    } else { _=os.Remove(path) }
    return map[string]any{"name":label,"ok":false,"status":"fail","path":directory,"error":fmt.Sprintf("%T: %v",err,err)}
}

func pathWithin(path,parent string) bool {
    absPath,err1:=filepath.Abs(path); absParent,err2:=filepath.Abs(parent)
    if err1!=nil || err2!=nil { return false }
    rel,err:=filepath.Rel(absParent,absPath)
    if err!=nil { return false }
    return rel=="." || (rel!=".." && !strings.HasPrefix(rel,".."+string(filepath.Separator)))
}

func codexConfigHints() map[string]any {
    home,_:=os.UserHomeDir()
    codexHome:=os.Getenv("CODEX_HOME")
    if codexHome=="" { codexHome=filepath.Join(home,".codex") }
    path:=filepath.Join(codexHome,"config.toml")
    result:=map[string]any{"path":path,"readable":false}
    data,err:=os.ReadFile(path)
    if err!=nil { return result }
    result["readable"]=true
    text:=string(data)
    for _,key:=range []string{"sandbox_mode","sandbox","approval_policy","approvals_reviewer"} {
        pattern:=regexp.MustCompile(`(?m)^\s*`+regexp.QuoteMeta(key)+`\s*=\s*["\x27]([^"\x27]+)["\x27]\s*$`)
        if match:=pattern.FindStringSubmatch(text); match!=nil { result[key]=match[1] }
    }
    return result
}

func AccessPreflight(project,root string) map[string]any {
    releaseGuard, guardErr := projectguard.AcquireWrite(project)
    if guardErr != nil { return map[string]any{"project":project,"dispatch_allowed":false,"error":guardErr.Error(),"checked_at":time.Now().Format(time.RFC3339)} }
    defer releaseGuard()

    now:=time.Now()
    sandboxEnv:=strings.TrimSpace(os.Getenv("CODEX_SANDBOX"))
    marker:=sandboxMarkerState(sandboxEnv)
    networkRaw:=strings.TrimSpace(os.Getenv("CODEX_SANDBOX_NETWORK_DISABLED"))
    networkDisabled:=networkRaw!="" && networkRaw!="0" && strings.ToLower(networkRaw)!="false" && strings.ToLower(networkRaw)!="no"
    base:=baseRoot(project,root)
    repo:=repoRoot(base)
    probes:=map[string]map[string]any{}
    probes["workspace_write"]=probeDirectoryWrite(filepath.Join(project,"_task_mecca",".runtime"),"workspace_write")

    gitDirText:=strings.TrimSpace(gitOutput(repo,"rev-parse","--git-dir"))
    if gitDirText!="" {
        gitDir:=gitDirText
        if !filepath.IsAbs(gitDir) { gitDir=filepath.Join(repo,gitDir) }
        probes["git_metadata_write"]=probeDirectoryWrite(gitDir,"git_metadata_write")
    } else {
        probes["git_metadata_write"]=map[string]any{"name":"git_metadata_write","ok":false,"status":"fail","error":"Git repository not detected"}
    }

    cmd:=exec.Command("git","--version")
    cmd.Dir=repo
    processErr:=cmd.Run()
    code:=0
    if processErr!=nil {
        code=1
        if exitErr,ok:=processErr.(*exec.ExitError); ok { code=exitErr.ExitCode() }
    }
    probes["subprocess"]=map[string]any{
        "name":"subprocess","ok":processErr==nil,
        "status":map[bool]string{true:"pass",false:"fail"}[processErr==nil],
        "returncode":code,
    }

    home,_:=os.UserHomeDir()
    if pathWithin(home,repo) {
        probes["external_write"]=map[string]any{"name":"external_write","ok":false,"status":"unavailable","path":home,"error":"HOME is inside the repository; no safe outside-workspace probe location"}
    } else { probes["external_write"]=probeDirectoryWrite(home,"external_write") }

    reasons:=[]string{}
    if marker=="restricted" { reasons=append(reasons,"CODEX_SANDBOX="+sandboxEnv) }
    if marker=="unknown" { reasons=append(reasons,"CODEX_SANDBOX(unrecognized)="+sandboxEnv) }
    names:=[]string{"workspace_write","git_metadata_write","subprocess","external_write"}
    allEffective:=true; hardFailure:=marker=="restricted"
    for _,name:=range names {
        probe:=probes[name]
        ok,_:=probe["ok"].(bool)
        if !ok { allEffective=false; reasons=append(reasons,name+"="+toString(probe["status"])) }
        if probe["status"]=="fail" { hardFailure=true }
    }
    unavailable:=probes["external_write"]["status"]=="unavailable"
    status:="unknown"; message:="Effective Full Access를 확정할 수 없습니다. subagent dispatch 전에 권한을 확인하세요."
    if allEffective && marker!="restricted" {
        status="full"; message="Effective Full Access가 확인되었습니다. subagent orchestration을 시작할 수 있습니다."
    } else if hardFailure {
        status="restricted"; message="Effective Full Access가 확인되지 않았습니다. subagent dispatch 전에 Codex에서 Full Access를 활성화하세요."
    } else if unavailable {
        status="unknown"; message="외부 workspace 쓰기 검증 위치를 확보하지 못해 Full Access를 확정할 수 없습니다."
    }
    network:="not_explicitly_disabled"; if networkDisabled { network="disabled" }
    var sandboxValue any=nil; if sandboxEnv!="" { sandboxValue=sandboxEnv }
    report:=map[string]any{
        "status":status,"full_access_confirmed":status=="full","orchestration_ready":status=="full",
        "dispatch_recheck_required":false,"restriction_current":marker=="restricted",
        "checked_at":now.Format("2006-01-02T15:04:05-07:00"),"source":"effective_probe",
        "sandbox_env":sandboxValue,"sandbox_marker":marker,
        "permission_contract":"effective-full-filesystem-access",
        "permission_note":"Product UI toggle is not read directly; readiness is based on current runtime markers plus harmless effective probes.",
        "network":network,
        "network_note":"Network is independent from filesystem sandbox mode; check it separately when the task requires network.",
        "probes":probes,"config_hint":codexConfigHints(),"reasons":reasons,"message":message,
    }
    cachePath:=filepath.Join(project,"_task_mecca",".runtime","access_preflight.json")
    if err:=os.MkdirAll(filepath.Dir(cachePath),0755); err==nil {
        if data,err:=json.MarshalIndent(report,"","  "); err==nil { _=os.WriteFile(cachePath,append(data, '\n'),0644) }
    }
    return report
}


func AccessObservation(project string) map[string]any {
    now:=time.Now()
    sandboxEnv:=strings.TrimSpace(os.Getenv("CODEX_SANDBOX"))
    marker:=sandboxMarkerState(sandboxEnv)
    networkRaw:=strings.TrimSpace(os.Getenv("CODEX_SANDBOX_NETWORK_DISABLED"))
    networkDisabled:=networkRaw!="" && networkRaw!="0" && strings.ToLower(networkRaw)!="false" && strings.ToLower(networkRaw)!="no"
    if marker=="restricted" {
        network:="unknown"; if networkDisabled { network="disabled" }
        return map[string]any{
            "status":"restricted","full_access_confirmed":false,"orchestration_ready":false,
            "checked_at":now.Format("2006-01-02T15:04:05-07:00"),"source":"runtime_environment",
            "sandbox_env":sandboxEnv,"sandbox_marker":marker,"network":network,
            "message":"현재 프로세스에 제한된 Codex sandbox 신호가 있습니다. subagent dispatch 전에 Full Access를 활성화하세요.",
            "dispatch_recheck_required":true,"restriction_current":true,
            "reasons":[]string{"CODEX_SANDBOX="+sandboxEnv},
        }
    }
    cachePath:=filepath.Join(project,"_task_mecca",".runtime","access_preflight.json")
    if data,err:=os.ReadFile(cachePath); err==nil {
        cached:=map[string]any{}
        if json.Unmarshal(data,&cached)==nil {
            var age any=nil
            stale:=true
            if checked:=toString(cached["checked_at"]); checked!="" {
                if dt,ok:=parseTime(checked); ok {
                    seconds:=now.Sub(dt).Seconds(); if seconds<0 { seconds=0 }
                    age=seconds; stale=seconds>900
                }
            }
            cached["source"]="cached_effective_probe"
            cached["cache_age_seconds"]=age
            cached["cache_stale"]=stale
            cached["last_observed_status"]=valueOr(cached["status"],"unknown")
            cached["dispatch_recheck_required"]=true
            cached["orchestration_ready"]=false
            if cached["status"]=="full" {
                cached["message"]="마지막 effective Full Access 검증은 성공했습니다. 대시보드에서는 관측 이력으로만 표시하며 subagent dispatch 직전에 자동으로 다시 검증합니다."
            } else if stale {
                cached["message"]="마지막 권한 검증 결과가 오래되었습니다. 경고 상태는 아니며, subagent dispatch 직전에 active preflight를 자동 재실행합니다."
            }
            if sandboxEnv!="" { cached["sandbox_env"]=sandboxEnv }
            cached["sandbox_marker"]=marker
            cached["restriction_current"]=false
            if networkDisabled { cached["network"]="disabled" }
            return cached
        }
    }
    network:="unknown"; if networkDisabled { network="disabled" }
    var sandbox any=nil; if sandboxEnv!="" { sandbox=sandboxEnv }
    return map[string]any{
        "status":"unknown","full_access_confirmed":false,"orchestration_ready":false,
        "checked_at":nil,"source":"no_effective_probe","sandbox_env":sandbox,
        "sandbox_marker":marker,"network":network,
        "message":"아직 effective Full Access 검증 이력이 없습니다. subagent dispatch 직전에 active preflight를 자동 실행합니다.",
        "dispatch_recheck_required":true,"restriction_current":false,"reasons":[]string{},
    }
}

func Preflight(project,root string,requireFull bool) (map[string]any,error) {
    backlog,err:=Presence(project,root,nil)
    if err!=nil { return nil,err }
    access:=AccessPreflight(project,root)
    orchestrationReady,_:=access["orchestration_ready"].(bool)
    backlogOK,_:=backlog["ok"].(bool)
    ok:=backlogOK && (!requireFull || orchestrationReady)
    report:=map[string]any{}
    for key,value:=range backlog { report[key]=value }
    report["ok"]=ok; report["backlog"]=backlog; report["access"]=access
    report["full_access_required"]=requireFull; report["orchestration_ready"]=orchestrationReady
    return report,nil
}
