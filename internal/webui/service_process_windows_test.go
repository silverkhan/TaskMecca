//go:build windows

package webui

import (
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
)

func TestWindowsRollbackScriptStopsFailedProcessBeforeRestore(t *testing.T) {
    script:=windowsRollbackScript(`C:\Program Files\Task Mecca\task-mecca.exe`,[]string{"web","--port","18765"},`C:\Work\My Project`,`C:\Program Files\Task Mecca\task-mecca.exe.previous`,18765)
    for _,needle:=range []string{"Start-Process -FilePath $exe","-PassThru","/api/health","Stop-Process -Id $p.Id -Force","$p.WaitForExit()","Copy-Item -Force","Restore-Previous",".upgrade.state","$handoff -eq 'rolled_back'","catch { Restore-Previous 'Upgraded Web could not start","Start-Process -FilePath $exe"} {
        if !contains(script,needle) { t.Fatalf("Windows rollback helper missing %q",needle) }
    }
}

func TestPowerShellQuoteHandlesApostrophe(t *testing.T) {
    got:=psQuote(`C:\Users\O'Brien\Task Mecca`)
    if got!=`'C:\Users\O''Brien\Task Mecca'` { t.Fatalf("psQuote=%q",got) }
}



func TestWindowsRollbackFailureInjectionRestoresBinaryAndNotice(t *testing.T) {
    home:=t.TempDir()
    t.Setenv("TASK_MECCA_HOME",home)
    dir:=t.TempDir()
    exe:=filepath.Join(dir,"task-mecca.cmd")
    previous:=exe+".previous"
    if err:=os.WriteFile(exe,[]byte("@echo off\r\nexit /b 0\r\n"),0644); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(previous,[]byte("@echo off\r\nexit /b 0\r\nREM previous\r\n"),0644); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(exe+".upgrade.state",[]byte("upgraded\r\n"),0644); err!=nil { t.Fatal(err) }
    script:=windowsRollbackScriptWithAttempts(exe,nil,dir,previous,1,1)
    ps:=filepath.Join(dir,"inject.ps1")
    if err:=os.WriteFile(ps,[]byte(script),0644); err!=nil { t.Fatal(err) }
    if out,err:=exec.Command("powershell.exe","-NoProfile","-ExecutionPolicy","Bypass","-File",ps).CombinedOutput(); err!=nil {
        t.Fatalf("failure injection helper failed: %v\n%s",err,out)
    }
    got,err:=os.ReadFile(exe)
    if err!=nil { t.Fatal(err) }
    if !strings.Contains(string(got),"REM previous") { t.Fatalf("rollback did not restore previous binary: %q",got) }
    notice:=UpgradeRecoveryStatus()
    if notice==nil || notice.Status!="rolled_back" { t.Fatalf("rollback notice missing: %#v",notice) }
}
