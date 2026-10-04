//go:build darwin

package webui

import (
    "bytes"
    "encoding/xml"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "strconv"
    "time"
)

const launchdWebLabel = "com.taskmecca.web"

func launchdUserDomain() string {
    return fmt.Sprintf("gui/%d",os.Getuid())
}

func launchdWebTarget() string {
    return launchdUserDomain()+"/"+launchdWebLabel
}

func launchdWebPlistPath() string {
    home,err:=os.UserHomeDir()
    if err!=nil || home=="" {
        return filepath.Join(webServiceDir(),launchdWebLabel+".plist")
    }
    return filepath.Join(home,"Library","LaunchAgents",launchdWebLabel+".plist")
}

func xmlText(value string) string {
    var b bytes.Buffer
    _=xml.EscapeText(&b,[]byte(value))
    return b.String()
}

func renderLaunchdPlist(exe string,args []string,project string) string {
    var b strings.Builder
    b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
    b.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
    b.WriteString("<plist version=\"1.0\">\n<dict>\n")
    b.WriteString("  <key>Label</key><string>"+launchdWebLabel+"</string>\n")
    b.WriteString("  <key>ProgramArguments</key>\n  <array>\n")
    b.WriteString("    <string>"+xmlText(exe)+"</string>\n")
    for _,arg:=range args {
        b.WriteString("    <string>"+xmlText(arg)+"</string>\n")
    }
    b.WriteString("  </array>\n")
    b.WriteString("  <key>WorkingDirectory</key><string>"+xmlText(project)+"</string>\n")
    b.WriteString("  <key>RunAtLoad</key><true/>\n")
    b.WriteString("  <key>KeepAlive</key><true/>\n")
    b.WriteString("  <key>ProcessType</key><string>Background</string>\n")
    b.WriteString("  <key>ThrottleInterval</key><integer>2</integer>\n")
    b.WriteString("  <key>StandardOutPath</key><string>"+xmlText(WebLogPath())+"</string>\n")
    b.WriteString("  <key>StandardErrorPath</key><string>"+xmlText(WebLogPath())+"</string>\n")
    if home:=strings.TrimSpace(os.Getenv("TASK_MECCA_HOME")); home!="" {
        b.WriteString("  <key>EnvironmentVariables</key>\n  <dict>\n")
        b.WriteString("    <key>TASK_MECCA_HOME</key><string>"+xmlText(home)+"</string>\n")
        b.WriteString("  </dict>\n")
    }
    b.WriteString("</dict>\n</plist>\n")
    return b.String()
}

func launchdWebLoaded() bool {
    cmd:=exec.Command("/bin/launchctl","print",launchdWebTarget())
    return cmd.Run()==nil
}

func stopManagedWebProcess() error {
    if !launchdWebLoaded() { return nil }
    cmd:=exec.Command("/bin/launchctl","bootout",launchdWebTarget())
    output,err:=cmd.CombinedOutput()
    if err!=nil {
        msg:=strings.TrimSpace(string(output))
        if msg!="" {
            return fmt.Errorf("launchd bootout failed: %s",msg)
        }
        return fmt.Errorf("launchd bootout failed: %w",err)
    }
    return nil
}

func startManagedWebProcess(exe string,args []string,project string) error {
    if err:=os.MkdirAll(webServiceDir(),0755); err!=nil { return err }
    plistPath:=launchdWebPlistPath()
    if err:=os.MkdirAll(filepath.Dir(plistPath),0755); err!=nil { return err }

    if launchdWebLoaded() {
        if err:=stopManagedWebProcess(); err!=nil { return err }
        time.Sleep(100*time.Millisecond)
    }

    tmp:=plistPath+".tmp"
    if err:=os.WriteFile(tmp,[]byte(renderLaunchdPlist(exe,args,project)),0644); err!=nil { return err }
    if err:=os.Rename(tmp,plistPath); err!=nil { return err }

    cmd:=exec.Command("/bin/launchctl","bootstrap",launchdUserDomain(),plistPath)
    output,err:=cmd.CombinedOutput()
    if err!=nil {
        msg:=strings.TrimSpace(string(output))
        if msg!="" {
            return fmt.Errorf("launchd bootstrap failed: %s",msg)
        }
        return fmt.Errorf("launchd bootstrap failed: %w",err)
    }
    return nil
}

func managedWebProcessActive() bool {
    return launchdWebLoaded()
}

func prepareManagedWebRestart() (bool,error) {
    if !launchdWebLoaded() { return false,nil }
    // The LaunchAgent uses KeepAlive=true. Once the current web process
    // exits after a graceful shutdown, launchd starts the upgraded binary.
    return true,nil
}

func launchdRollbackScript(exe,previous,targetInstance string,port int) string {
    return launchdRollbackScriptWithAttempts(exe,previous,targetInstance,port,30)
}

func shQuote(value string) string { return "'" + strings.ReplaceAll(value,"'","'\\''") + "'" }

func launchdRollbackScriptWithAttempts(exe,previous,targetInstance string,port,attempts int) string {
    health:="http://127.0.0.1:"+strconv.Itoa(port)+"/api/health"
    notice:=upgradeRecoveryPath()
    // The watchdog starts before the current server shuts down. A plain 200
    // can therefore be the OLD process. Accept health only when the response
    // carries the instance ID passed to the restarted Web process.
    probe:="/usr/bin/curl -fsS --max-time 1 "+shQuote(health)+" 2>/dev/null | /usr/bin/grep -F "+shQuote("\\\"instance_id\\\":\\\""+targetInstance+"\\\"")+" >/dev/null 2>&1"
    rollbackID:="rollback-$(/bin/date -u +%Y%m%dT%H%M%SZ)-$"
    return "i=0; while [ $i -lt "+strconv.Itoa(attempts)+" ]; do sleep 1; if "+probe+"; then exit 0; fi; i=$((i+1)); done; " +
        "/bin/cp "+shQuote(previous)+" "+shQuote(exe)+"; /bin/chmod +x "+shQuote(exe)+"; /bin/mkdir -p "+shQuote(filepath.Dir(notice))+"; " +
        "/usr/bin/printf '%s\\n' '{\"id\":\"'"+rollbackID+"'\",\"status\":\"rolled_back\",\"at\":\"'$(/bin/date -u +%Y-%m-%dT%H:%M:%SZ)'\",\"message\":\"Upgraded Web failed health check; previous binary restored.\"}' > "+shQuote(notice)+"; " +
        "/bin/launchctl kickstart -k "+shQuote(launchdWebTarget())
}

func launchdRollbackWatchdog(exe,previous,targetInstance string,port int) error {
    if previous=="" { return nil }
    cmd:=exec.Command("/bin/sh","-c",launchdRollbackScript(exe,previous,targetInstance,port))
    cmd.Stdout=os.Stdout
    cmd.Stderr=os.Stderr
    if err:=cmd.Start(); err!=nil { return err }
    return cmd.Process.Release()
}

func detachedWebRestartWithRollback(exe string,args []string,project,previous string,port int) error {
    return detachedWebRestart(exe,args,project)
}

func detachedWebRestart(exe string,args []string,project string) error {
    shellArgs:=append([]string{"-c","sleep 1; exec \"$@\"" ,"task-mecca-web-restart",exe},args...)
    cmd:=exec.Command("/bin/sh",shellArgs...)
    cmd.Dir=project
    cmd.Stdout=os.Stdout
    cmd.Stderr=os.Stderr
    return cmd.Start()
}
