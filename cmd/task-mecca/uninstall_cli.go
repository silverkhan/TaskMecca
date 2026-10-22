package main

import (
    "bufio"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
    "github.com/silverkhan/TaskMecca/internal/webui"
)

type uninstallHook struct {
    Provider string `json:"provider"`
    Config string `json:"config"`
    OwnedCommands int `json:"owned_commands"`
}
type uninstallPlan struct {
    Executable string `json:"executable"`
    ExecutableSHA256 string `json:"executable_sha256"`
    WebRunning bool `json:"web_running"`
    ServiceRegistration string `json:"service_registration,omitempty"`
    Hooks []uninstallHook `json:"hooks"`
    PurgeSettings bool `json:"purge_settings"`
    SettingFiles []string `json:"setting_files,omitempty"`
    Preserved []string `json:"preserved"`
}
type uninstallOptions struct {
    DryRun bool
    Approve bool
    PurgeSettings bool
    JSON bool
}

// validateInstalledExecutable refuses to delete an unrelated executable,
// arbitrary directory, or symlink. A caller must run the installed CLI itself.
func validateInstalledExecutable(path string) (string, error) {
    abs, err := filepath.Abs(path)
    if err != nil { return "", err }
    basename := strings.ToLower(filepath.Base(abs))
    if basename!="task-mecca" && basename!="task-mecca.exe" {
        return "", fmt.Errorf("refusing to uninstall an unknown executable %q", abs)
    }
    info, err := os.Lstat(abs)
    if err != nil { return "", err }
    if !info.Mode().IsRegular() {
        return "", fmt.Errorf("refusing to uninstall a non-regular executable or symlink: %s", abs)
    }
    return abs,nil
}

func buildUninstallPlan(executable,home,management string,purge bool) (uninstallPlan,error) {
    executable,err:=validateInstalledExecutable(executable)
    if err!=nil { return uninstallPlan{},err }
    binaryData,err:=os.ReadFile(executable)
    if err!=nil{return uninstallPlan{},err}
    binaryHash:=sha256.Sum256(binaryData)
    result:=uninstallPlan{
        Executable:executable,ExecutableSHA256:hex.EncodeToString(binaryHash[:]),PurgeSettings:purge,
        Hooks:[]uninstallHook{},
        Preserved:[]string{"All project directories, _task_mecca, backlog and lifecycle data",
            "Project-local agent configuration/Hooks (not modified automatically)",
            "Global Task Mecca data and project registry (unless --purge-settings)"},
    }
    svcState,err:=webui.InspectServiceStatus()
    if err!=nil{return result,err}
    result.WebRunning=svcState.Running
    svc,exists,err:=webui.ManagedWebRegistration()
    if err!=nil { return result,err }
    if exists { result.ServiceRegistration=svc }
    for _,provider:=range []string{"codex","claude"} {
        count,err:=runtimeobs.OwnedHookCount(home,provider)
        if err!=nil { return result,fmt.Errorf("%s global Hook inspection failed: %w",provider,err) }
        cfg,err:=runtimeobs.HookConfigPath(home,provider)
        if err!=nil { return result,err }
        result.Hooks=append(result.Hooks,uninstallHook{Provider:provider,Config:cfg,OwnedCommands:count})
    }
    if purge {
        result.Preserved[2]="Global Task Mecca logs and unrecognized files are preserved (even with --purge-settings)"
        for _,name:=range []string{"projects.json","channel"} {
            path:=filepath.Join(management,name)
            info,err:=os.Lstat(path)
            if errors.Is(err,os.ErrNotExist){continue}
            if err!=nil{return result,err}
            if !info.Mode().IsRegular(){return result,fmt.Errorf("not a regular managed setting: %s",path)}
            result.SettingFiles=append(result.SettingFiles,path)
        }
    }
    return result,nil
}

func backupOwnedHook(path string) (string,error) {
    before,err:=os.ReadFile(path)
    if err!=nil{return "",err}
    // Keep backups adjacent to the user config. Never replace an existing
    // backup, never change other provider settings, and always use mode 0600.
    checksum:=sha256.Sum256(before)
    suffix:=time.Now().UTC().Format("20060102T150405.000000000")+"-"+hex.EncodeToString(checksum[:4])
    backup:=path+".task-mecca-uninstall-"+suffix+".bak"
    file,err:=os.OpenFile(backup,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600)
    if err!=nil{return "",err}
    if _,err=file.Write(before);err!=nil {file.Close();_ =os.Remove(backup);return "",err}
    if err=file.Sync();err!=nil {file.Close();_ =os.Remove(backup);return "",err}
    if err=file.Close();err!=nil {_ =os.Remove(backup);return "",err}
    current,err:=os.ReadFile(path)
    if err!=nil || sha256.Sum256(current)!=checksum {
        return backup,fmt.Errorf("Hook config changed during uninstall preflight; preserved backup at %s",backup)
    }
    return backup,nil
}

func performUninstall(plan uninstallPlan,output io.Writer) error {
    if plan.WebRunning { fmt.Fprintln(output,"Stopping Task Mecca Web service...") }
    if _,err:=webui.StopService();err!=nil {
        return fmt.Errorf("Web service could not be stopped; executable preserved: %w",err)
    }
    if err:=webui.RemoveManagedWebRegistration();err!=nil {
        return fmt.Errorf("could not remove owned Web autostart registration; executable preserved: %w",err)
    }

    home,err:=os.UserHomeDir()
    if err!=nil {return err}
    // Only user-global Task Mecca-owned Hook commands are changed. Never
    // enumerate or mutate arbitrary project-local agent settings on uninstall.
    for _,hook:=range plan.Hooks {
        currentCount,err:=runtimeobs.OwnedHookCount(home,hook.Provider)
        if err!=nil {return fmt.Errorf("%s Hook configuration unreadable; binary preserved: %w",hook.Provider,err)}
        if currentCount==0 { continue }
        backup,err:=backupOwnedHook(hook.Config)
        if err!=nil {return fmt.Errorf("%s Hook backup failed; executable preserved: %w",hook.Provider,err)}
        fmt.Fprintf(output,"Hook backup: %s\n",backup)
        if _,err:=runtimeobs.DisableGlobalHooks(hook.Provider);err!=nil {
            return fmt.Errorf("%s owned Hook removal failed (backup: %s); executable preserved: %w",hook.Provider,backup,err)
        }
        remaining,err:=runtimeobs.OwnedHookCount(home,hook.Provider)
        if err!=nil || remaining!=0 {
            return fmt.Errorf("%s owned Hook verification failed (backup: %s); executable preserved: %v, count %d",hook.Provider,backup,err,remaining)
        }
    }

    for _,file:=range plan.SettingFiles {
        info,err:=os.Lstat(file)
        if errors.Is(err,os.ErrNotExist){continue}
        if err!=nil{return err}
        if !info.Mode().IsRegular(){return fmt.Errorf("settings path no longer a regular file; executable preserved: %s",file)}
        if err:=os.Remove(file);err!=nil{return fmt.Errorf("could not purge managed setting %s; executable preserved: %w",file,err)}
    }
    present,err:=os.ReadFile(plan.Executable)
    if err!=nil{return fmt.Errorf("executable changed since preflight; retained: %w",err)}
    digest:=sha256.Sum256(present)
    if hex.EncodeToString(digest[:])!=plan.ExecutableSHA256 {
        return fmt.Errorf("installed executable changed since uninstall preflight; refusing to delete it")
    }
    outcome,err:=removeInstalledBinary(plan.Executable)
    if err!=nil{return fmt.Errorf("installed executable removal failed, manual cleanup required: %w",err)}
    fmt.Fprintf(output,"Executable: %s (%s)\n",plan.Executable,outcome)
    fmt.Fprintln(output,"Project directories, _task_mecca, backlog and lifecycle records preserved.")
    if runtime.GOOS=="windows" {
        fmt.Fprintln(output,"Windows executable removal runs after this CLI exits; verify the file is gone.")
    }
    return nil
}

func runUninstallCLI(args []string,in io.Reader,out,errorsOut io.Writer) int {
    flags:=flag.NewFlagSet("uninstall",flag.ContinueOnError)
    flags.SetOutput(errorsOut)
    dryRun:=flags.Bool("dry-run",false,"preview without changing any file")
    approve:=flags.Bool("approve",false,"confirm uninstall without an interactive prompt")
    purge:=flags.Bool("purge-settings",false,"also remove known global registry/channel settings; never project files")
    jsonResult:=flags.Bool("json",false,"output the read-only plan as JSON")
    if err:=flags.Parse(args);err!=nil||flags.NArg()!=0 {return 2}
    if *jsonResult && !*dryRun {
        fmt.Fprintln(errorsOut,"--json is supported only with --dry-run")
        return 2
    }
    exe,err:=os.Executable()
    if err!=nil {fmt.Fprintln(errorsOut,err);return 2}
    home,err:=os.UserHomeDir()
    if err!=nil {fmt.Fprintln(errorsOut,err);return 2}
    management:=os.Getenv("TASK_MECCA_HOME")
    if strings.TrimSpace(management)==""{management=filepath.Join(home,".task-mecca")}
    plan,err:=buildUninstallPlan(exe,home,management,*purge)
    if err!=nil {fmt.Fprintln(errorsOut,err);return 2}
    if *jsonResult {
        if err:=json.NewEncoder(out).Encode(plan);err!=nil {fmt.Fprintln(errorsOut,err);return 2}
        return 0
    }
    fmt.Fprintln(out,"Task Mecca uninstall plan:")
    fmt.Fprintln(out,"  Executable:",plan.Executable)
    fmt.Fprintln(out,"  Web service: stop if running")
    if plan.ServiceRegistration!="" {fmt.Fprintln(out,"  Remove managed service registration:",plan.ServiceRegistration)}
    for _,h:=range plan.Hooks{
        if h.OwnedCommands>0 {fmt.Fprintf(out,"  Remove %d %s Hook command(s) from %s (backup first)\n",h.OwnedCommands,h.Provider,h.Config)}
    }
    for _,path:=range plan.SettingFiles {fmt.Fprintln(out,"  Purge known global setting:",path)}
    fmt.Fprintln(out,"  PRESERVE all project directories and their _task_mecca data.")
    fmt.Fprintln(out,"  PRESERVE project-local agent settings; only global owned Hooks are removed.")
    if *dryRun {fmt.Fprintln(out,"Dry run: no files or services were changed.");return 0}
    if !*approve {
        fmt.Fprint(out,"Type 'uninstall' to confirm (anything else cancels): ")
        text,err:=bufio.NewReader(in).ReadString('\n')
        if err!=nil && !errors.Is(err,io.EOF) {fmt.Fprintln(errorsOut,err);return 2}
        if strings.TrimSpace(text)!="uninstall" {fmt.Fprintln(out,"Cancelled; nothing changed.");return 1}
    }
    if err:=performUninstall(plan,out);err!=nil {fmt.Fprintln(errorsOut,err);return 2}
    fmt.Fprintln(out,"Task Mecca program uninstall complete.")
    return 0
}
