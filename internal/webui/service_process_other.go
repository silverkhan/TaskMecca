//go:build !darwin

package webui

import (
    "os"
    "os/exec"
    "runtime"
    "strconv"
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

func detachedWebRestartWithRollback(exe string,args []string,project,previous string,port int) error {
    if runtime.GOOS=="windows" {
        helper,err:=os.CreateTemp("","task-mecca-web-restart-*.cmd")
        if err!=nil { return err }
        helperPath:=helper.Name()
        health:="http://127.0.0.1:"+strconv.Itoa(port)+"/api/health"
        body:="@echo off\r\ntimeout /t 1 /nobreak >nul\r\nstart \"\" /D \""+project+"\" \""+exe+"\""
        for _,arg:=range args { body+=" \""+arg+"\"" }
        body+="\r\n"
        if previous!="" {
            body+="for /L %%i in (1,1,30) do (\r\n  timeout /t 1 /nobreak >nul\r\n  powershell -NoProfile -Command \"try { if ((Invoke-WebRequest -UseBasicParsing -TimeoutSec 1 '"+health+"').StatusCode -eq 200) { exit 0 } } catch {}; exit 1\" && goto healthy\r\n)\r\n"
            body+="copy /Y \""+previous+"\" \""+exe+"\" >nul\r\nstart \"\" /D \""+project+"\" \""+exe+"\""
            for _,arg:=range args { body+=" \""+arg+"\"" }
            body+="\r\n:healthy\r\n"
        }
        body+="del \"%~f0\"\r\n"
        if _,err=helper.WriteString(body); err!=nil { _=helper.Close(); return err }
        if err=helper.Close(); err!=nil { return err }
        cmd:=exec.Command("cmd.exe","/D","/C","start","","/MIN",helperPath)
        cmd.Dir=project
        if err=cmd.Start(); err!=nil { return err }
        return cmd.Process.Release()
    }

    script:="sleep 1; \"$@\" & child=$!; "
    if previous!="" {
        health:="http://127.0.0.1:"+strconv.Itoa(port)+"/api/health"
        script+="i=0; while [ $i -lt 30 ]; do sleep 1; if curl -fsS --max-time 1 '"+health+"' >/dev/null 2>&1; then exit 0; fi; i=$((i+1)); done; kill $child >/dev/null 2>&1 || true; cp '"+previous+"' '"+exe+"'; chmod +x '"+exe+"'; exec \"$@\""
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
