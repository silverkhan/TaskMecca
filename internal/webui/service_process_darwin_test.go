//go:build darwin

package webui

import (
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
)

func TestLaunchdRollbackScriptRestoresAndKickstarts(t *testing.T) {
    t.Setenv("TASK_MECCA_HOME",t.TempDir())
    script:=launchdRollbackScript("/Applications/Task Mecca/task-mecca","/Applications/Task Mecca/task-mecca.previous",18765)
    for _,needle:=range []string{"/api/health","/bin/cp ","upgrade-recovery.json","rolled_back","launchctl kickstart -k","com.taskmecca.web"} {
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
    cmd:=exec.Command("/bin/sh","-c",launchdRollbackScriptWithAttempts(exe,previous,1,1))
    _=cmd.Run() // launchctl kickstart is expected to fail in an unregistered CI LaunchAgent.
    got,err:=os.ReadFile(exe)
    if err!=nil { t.Fatal(err) }
    if string(got)!="previous" { t.Fatalf("rollback did not restore previous binary: %q",got) }
    notice:=UpgradeRecoveryStatus()
    if notice==nil || notice.Status!="rolled_back" { t.Fatalf("rollback notice missing: %#v",notice) }
}
