//go:build darwin

package webui

import (
    "github.com/silverkhan/TaskMecca/internal/notify"
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
    command := "/bin/launchctl kickstart -k "+shQuote(launchdWebTarget())
    if !strings.HasSuffix(script,command) || strings.Count(script,command)!=1 {
        t.Fatal("production rollback must retain exactly one fixed LaunchAgent kickstart")
    }
}
// These scripts are executed, unlike the production-renderer string contract
// above. Never execute that renderer in tests: its absolute launchctl command
// reaches the real fixed LaunchAgent even with an isolated TASK_MECCA_HOME.
func fixtureLaunchdRollbackScript(t *testing.T,exe,previous,targetInstance string,port int) (string,string) {
    t.Helper()
    dir:=t.TempDir()
    stub:=filepath.Join(dir,"kickstart-stub")
    log:=filepath.Join(dir,"kickstart-arguments.log")
    body:="#!/bin/sh\n/usr/bin/printf '%s\\n' \"$@\" >> "+shQuote(log)+"\n"
    if err:=os.WriteFile(stub,[]byte(body),0700); err!=nil { t.Fatal(err) }
    command:=shQuote(stub)+" kickstart -k "+shQuote(launchdWebTarget())
    script:=launchdRollbackScriptWithKickstart(exe,previous,targetInstance,port,1,command)
    if strings.Contains(script,"/bin/launchctl") || !strings.HasSuffix(script,command) {
        t.Fatal("execution fixture contains operational launchctl instead of stub")
    }
    return script,log
}
func assertFixtureKickstart(t *testing.T,log string) {
    t.Helper()
    got,err:=os.ReadFile(log)
    if err!=nil { t.Fatal(err) }
    want:="kickstart\n-k\n"+launchdWebTarget()+"\n"
    if string(got)!=want { t.Fatalf("stub kickstart arguments: got %q want %q",got,want) }
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
    script,log:=fixtureLaunchdRollbackScript(t,exe,previous,"new-instance",1)
    cmd:=exec.Command("/bin/sh","-c",script)
    if output,err:=cmd.CombinedOutput(); err!=nil { t.Fatalf("fixture rollback: %v %s",err,output) }
    assertFixtureKickstart(t,log)
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
    script,log:=fixtureLaunchdRollbackScript(t,exe,previous,"new-instance",port)
    cmd:=exec.Command("/bin/sh","-c",script)
    if output,err:=cmd.CombinedOutput(); err!=nil { t.Fatalf("fixture old-instance rollback: %v %s",err,output) }
    assertFixtureKickstart(t,log)
    got,err:=os.ReadFile(exe)
    if err!=nil { t.Fatal(err) }
    if string(got)!="previous" { t.Fatalf("old healthy instance was mistaken for upgraded Web: %q",got) }
    if notice:=UpgradeRecoveryStatus(); notice==nil || notice.Status!="rolled_back" {
        t.Fatalf("old-instance rollback notice missing: %#v",notice)
    }
}

func TestLaunchdMaintenanceActualChildReceivesGeneratedEnvironment(t *testing.T) {
	if os.Getenv("TASK_MECCA_A23_ENV_CHILD") == "1" {
		if !notify.TelegramTransportDisabled() {
			t.Fatal("child transport safety missing")
		}
		return
	}
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "disabled")
	plist := renderLaunchdPlist(os.Args[0], []string{"-test.run=TestLaunchdMaintenanceActualChildReceivesGeneratedEnvironment"}, t.TempDir())
	extract := exec.Command("/usr/bin/plutil", "-extract", "EnvironmentVariables.TASK_MECCA_TELEGRAM_TRANSPORT", "raw", "-o", "-", "-")
	extract.Stdin = strings.NewReader(plist)
	value, err := extract.Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "")
	child := exec.Command(os.Args[0], "-test.run=TestLaunchdMaintenanceActualChildReceivesGeneratedEnvironment")
	child.Env = append(os.Environ(), "TASK_MECCA_A23_ENV_CHILD=1", "TASK_MECCA_TELEGRAM_TRANSPORT="+strings.TrimSpace(string(value)))
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("generated launchd environment child: %s %v", output, err)
	}
}

func TestLaunchdMaintenanceEnvironmentIsExplicitEvenWithoutCustomHome(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", "")
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "disabled")
	plist := renderLaunchdPlist("/fixture/task-mecca", []string{"web", "--foreground"}, "/fixture/project")
	if !strings.Contains(plist, "<key>TASK_MECCA_TELEGRAM_TRANSPORT</key><string>disabled</string>") {
		t.Fatal("managed child loses transport safety mode")
	}
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "")
	normal := renderLaunchdPlist("/fixture/task-mecca", []string{"web", "--foreground"}, "/fixture/project")
	if strings.Contains(normal, "TASK_MECCA_TELEGRAM_TRANSPORT") {
		t.Fatal("normal service unexpectedly disabled")
	}
}
