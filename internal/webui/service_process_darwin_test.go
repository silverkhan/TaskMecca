//go:build darwin

package webui

import (
    "os"
    "os/exec"
    "net"
    "net/http"
    "path/filepath"
    "strings"
    "testing"
)

func TestLaunchdRollbackScriptRestoresAndKickstarts(t *testing.T) {
    t.Setenv("TASK_MECCA_HOME",t.TempDir())
    script:=launchdRollbackScript("/Applications/Task Mecca/task-mecca","/Applications/Task Mecca/task-mecca.previous","new-instance",18765)
    for _,needle:=range []string{"/api/health","instance_id","new-instance","/bin/cp ","upgrade-recovery.json","rollback-","rolled_back","launchctl kickstart -k","com.taskmecca.web"} {
        if !strings.Contains(script,needle) { t.Fatalf("macOS rollback helper missing %q",needle) }
    }
}

func TestLaunchdRollbackFailureInjectionRestoresBinaryAndNotice(t *testing.T) {
    home:=t.TempDir()
    t.Setenv("TASK_MECCA_HOME",home)
    dir:=t.TempDir()
    exe:=filepath.Join(dir,"task-mecca")
    previous:=exe+".previous"
    if err:=os.WriteFile(exe,[]byte("broken"),0755); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(previous,[]byte("previous"),0755); err!=nil { t.Fatal(err) }
    // Port 1 is intentionally unavailable: inject a failed health check.
    cmd:=exec.Command("/bin/sh","-c",launchdRollbackScriptWithAttempts(exe,previous,"new-instance",1,1))
    _=cmd.Run() // launchctl kickstart is expected to fail in an unregistered CI LaunchAgent.
    got,err:=os.ReadFile(exe)
    if err!=nil { t.Fatal(err) }
    if string(got)!="previous" { t.Fatalf("rollback did not restore previous binary: %q",got) }
    notice:=UpgradeRecoveryStatus()
    if notice==nil || notice.Status!="rolled_back" { t.Fatalf("rollback notice missing: %#v",notice) }
}


func TestLaunchdRollbackDoesNotAcceptOldHealthyInstance(t *testing.T) {
    t.Setenv("TASK_MECCA_HOME",t.TempDir())
    dir:=t.TempDir()
    exe:=filepath.Join(dir,"task-mecca")
    previous:=exe+".previous"
    if err:=os.WriteFile(exe,[]byte("upgraded"),0755); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(previous,[]byte("previous"),0755); err!=nil { t.Fatal(err) }

    listener,err:=net.Listen("tcp","127.0.0.1:0")
    if err!=nil { t.Fatal(err) }
    port:=listener.Addr().(*net.TCPAddr).Port
    server:=&http.Server{Handler:http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        w.Header().Set("Content-Type","application/json")
        _,_=w.Write([]byte(`{"ok":true,"instance_id":"old-instance"}`))
    })}
    go server.Serve(listener)
    defer server.Close()

    // This is the race that previously escaped rollback: the old Web still
    // answers 200 while the watchdog is waiting for launchd to start the new one.
    cmd:=exec.Command("/bin/sh","-c",launchdRollbackScriptWithAttempts(exe,previous,"new-instance",port,1))
    _=cmd.Run()
    got,err:=os.ReadFile(exe)
    if err!=nil { t.Fatal(err) }
    if string(got)!="previous" { t.Fatalf("old healthy instance was mistaken for upgraded Web: %q",got) }
}
