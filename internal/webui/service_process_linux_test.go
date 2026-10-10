//go:build linux

package webui

import (
    "os"
    "os/exec"
    "path/filepath"
    "testing"
)

func TestLinuxRollbackFailureInjectionRestoresBinaryAndNotice(t *testing.T) {
    home:=t.TempDir()
    t.Setenv("TASK_MECCA_HOME",home)
    dir:=t.TempDir()
    exe:=filepath.Join(dir,"task-mecca")
    previous:=exe+".previous"
    if err:=os.WriteFile(exe,[]byte("#!/bin/sh\nexit 0\n"),0755); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(previous,[]byte("#!/bin/sh\n# previous\nexit 0\n"),0755); err!=nil { t.Fatal(err) }
    script:=unixRollbackScript(exe,previous,1,1)
    cmd:=exec.Command("sh","-c",script,"task-mecca-web-restart",exe)
    if out,err:=cmd.CombinedOutput(); err!=nil { t.Fatalf("failure injection helper failed: %v\n%s",err,out) }
    got,err:=os.ReadFile(exe)
    if err!=nil { t.Fatal(err) }
    if string(got)!="#!/bin/sh\n# previous\nexit 0\n" { t.Fatalf("rollback did not restore previous binary: %q",got) }
    notice:=UpgradeRecoveryStatus()
    if notice==nil || notice.Status!="rolled_back" { t.Fatalf("rollback notice missing: %#v",notice) }
}
