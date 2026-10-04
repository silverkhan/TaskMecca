//go:build !darwin

package webui

import (
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strconv"
    "strings"
)

func startManagedWebProcess(exe string,args []string,project string) error {
    logFile,err:=os.OpenFile(WebLogPath(),os.O_CREATE|os.O_WRONLY|os.O_APPEND,0644)
    if err!=nil { return err }

    cmd:=exec.Command(exe,args...)
    cmd.Dir=project
    cmd.Stdout=logFile
    cmd.Stderr=logFile
    if err=cmd.Start(); err!=nil {
        _=logFile.Close()
        return err
    }
    _=logFile.Close()
    return cmd.Process.Release()
}

func managedWebProcessActive() bool { return false }

func stopManagedWebProcess() error { return nil }

func prepareManagedWebRestart() (bool,error) { return false,nil }

func detachedWebRestart(exe string,args []string,project string) error {
    return detachedWebRestartWithRollback(exe,args,project,"",DefaultPort)
}

func launchdRollbackWatchdog(exe,previous string,port int) error { return nil }

func psQuote(value string) string { return "'" + strings.ReplaceAll(value,"'","''") + "'" }

func windowsRollbackScript(exe string,args []string,project,previous string,port int) string {
    quotedArgs:=make([]string,0,len(args))
    for _,arg:=range args { quotedArgs=append(quotedArgs,psQuote(arg)) }
    argList:="@("+strings.Join(quotedArgs,",")+")"
    health:=psQuote("http://127.0.0.1:"+strconv.Itoa(port)+"/api/health")
    var b strings.Builder
    b.WriteString("$ErrorActionPreference='Stop'\r\n")
    b.WriteString("$exe="+psQuote(exe)+"\r\n$project="+psQuote(project)+"\r\n$args="+argList+"\r\n")
    if previous!="" {
        notice:=psQuote(upgradeRecoveryPath())
        noticeDir:=psQuote(filepath.Dir(upgradeRecoveryPath()))
        prev:=psQuote(previous)
        b.WriteString("function Restore-Previous([string]$reason){ New-Item -ItemType Directory -Force -Path "+noticeDir+" | Out-Null; Copy-Item -Force "+prev+" $exe; $n=@{id=[guid]::NewGuid().ToString();status='rolled_back';at=(Get-Date).ToUniversalTime().ToString('o');message=$reason}|ConvertTo-Json -Compress; [IO.File]::WriteAllText("+notice+",$n); Start-Process -FilePath $exe -ArgumentList $args -WorkingDirectory $project }\r\n")
    }
    b.WriteString("Start-Sleep -Seconds 1\r\n")
    if previous!="" {
        marker:=psQuote(exe+".upgrade.state")
        b.WriteString("$handoff=''; for($i=0;$i -lt 35;$i++){ if(Test-Path "+marker+"){ $handoff=(Get-Content -Raw "+marker+").Trim(); Remove-Item -Force "+marker+" -ErrorAction SilentlyContinue; break }; Start-Sleep -Seconds 1 }\r\n")
        b.WriteString("if($handoff -eq 'rolled_back'){ Restore-Previous 'Upgrade replacement failed; previous binary restored.'; exit 0 }\r\n")
        b.WriteString("if($handoff -eq 'failed' -or $handoff -eq ''){ Restore-Previous 'Upgrade replacement handoff failed; previous binary restored.'; exit 0 }\r\n")
        b.WriteString("try { $p=Start-Process -FilePath $exe -ArgumentList $args -WorkingDirectory $project -PassThru } catch { Restore-Previous 'Upgraded Web could not start; previous binary restored.'; exit 0 }\r\n")
        b.WriteString("$healthy=$false\r\nfor($i=0;$i -lt 30;$i++){ Start-Sleep -Seconds 1; try { $r=Invoke-WebRequest -UseBasicParsing -TimeoutSec 1 "+health+"; if($r.StatusCode -eq 200){$healthy=$true;break} } catch {} }\r\n")
        b.WriteString("if(-not $healthy){ try { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue } catch {}; try { $p.WaitForExit() } catch {}; Restore-Previous 'Upgraded Web failed health check; previous binary restored.' }\r\n")
    } else {
        b.WriteString("$p=Start-Process -FilePath $exe -ArgumentList $args -WorkingDirectory $project -PassThru\r\n")
    }
    return b.String()
}

func detachedWebRestartWithRollback(exe string,args []string,project,previous string,port int) error {
    if runtime.GOOS=="windows" {
        helper,err:=os.CreateTemp("","task-mecca-web-restart-*.ps1")
        if err!=nil { return err }
        helperPath:=helper.Name()
        if _,err=helper.WriteString(windowsRollbackScript(exe,args,project,previous,port)); err!=nil { _=helper.Close(); return err }
        if err=helper.Close(); err!=nil { return err }
        cmd:=exec.Command("powershell.exe","-NoProfile","-ExecutionPolicy","Bypass","-File",helperPath)
        cmd.Dir=project
        if err=cmd.Start(); err!=nil { return err }
        return cmd.Process.Release()
    }

    script:="sleep 1; \"$@\" & child=$!; "
    if previous!="" {
        health:="http://127.0.0.1:"+strconv.Itoa(port)+"/api/health"
        notice:=upgradeRecoveryPath()
        script+="i=0; while [ $i -lt 30 ]; do sleep 1; if curl -fsS --max-time 1 '"+health+"' >/dev/null 2>&1; then exit 0; fi; i=$((i+1)); done; kill $child >/dev/null 2>&1 || true; cp '"+previous+"' '"+exe+"'; chmod +x '"+exe+"'; mkdir -p '"+filepath.Dir(notice)+"'; printf '%s\\n' '{\"id\":\"rollback\",\"status\":\"rolled_back\",\"at\":\"'$(date -u +%Y-%m-%dT%H:%M:%SZ)'\",\"message\":\"Upgraded Web failed health check; previous binary restored.\"}' > '"+notice+"'; exec \"$@\""
    } else {
        script+="wait $child"
    }
    shellArgs:=append([]string{"-c",script,"task-mecca-web-restart",exe},args...)
    cmd:=exec.Command("sh",shellArgs...)
    cmd.Dir=project
    cmd.Stdout=os.Stdout
    cmd.Stderr=os.Stderr
    return cmd.Start()
}
