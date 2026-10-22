package main

import (
    "bytes"
    "encoding/json"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "testing"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// Keep all tests in a fake installation. Never use the running test binary as
// the deletion target.
func uninstallFixture(t *testing.T) (string,string,string,string) {
    t.Helper()
    base:=t.TempDir()
    home:=filepath.Join(base,"user-home")
    management:=filepath.Join(home,".task-mecca")
    t.Setenv("HOME",home)
    t.Setenv("USERPROFILE",home)
    t.Setenv("TASK_MECCA_HOME",management)
    if err:=os.MkdirAll(management,0700);err!=nil{t.Fatal(err)}
    exe:=filepath.Join(base,"task-mecca")
    if runtime.GOOS=="windows" {exe+=".exe"}
    if err:=os.WriteFile(exe,[]byte("fake binary for uninstall test"),0700);err!=nil{t.Fatal(err)}
    project:=filepath.Join(base,"important project")
    protected:=filepath.Join(project,"_task_mecca","data","backlog","B-777.md")
    if err:=os.MkdirAll(filepath.Dir(protected),0700);err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(protected,[]byte("valuable user data"),0600);err!=nil{t.Fatal(err)}
    return exe,home,management,protected
}

func TestUninstallPlanNeverChangesProjectOrGlobalData(t *testing.T) {
    exe,home,management,protected:=uninstallFixture(t)
    registry:=filepath.Join(management,"projects.json")
    if err:=os.WriteFile(registry,[]byte("{\"projects\":[]}\n"),0600);err!=nil{t.Fatal(err)}
    hook:=filepath.Join(home,".claude","settings.json")
    if err:=os.MkdirAll(filepath.Dir(hook),0700);err!=nil{t.Fatal(err)}
    config:=[]byte(`{"permissions":{"allow":["Bash(git status)"]},"hooks":{"SubagentStart":[{"hooks":[{"type":"command","command":"echo user-hook"},{"type":"command","command":"task-mecca runtime observe claude"}]}]}}`)
    if err:=os.WriteFile(hook,config,0600);err!=nil{t.Fatal(err)}
    state:=filepath.Join(management,"web","state.json")
    if err:=os.MkdirAll(filepath.Dir(state),0700);err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(state,[]byte(`{"pid":42,"port":18765}`),0600);err!=nil{t.Fatal(err)}

    plan,err:=buildUninstallPlan(exe,home,management,false)
    if err!=nil{t.Fatal(err)}
    if len(plan.Hooks)!=2 || plan.Hooks[1].OwnedCommands!=1 || plan.PurgeSettings {
        t.Fatalf("unexpected read-only plan: %#v",plan)
    }
    encoded,err:=json.Marshal(plan)
    if err!=nil || !bytes.Contains(encoded,[]byte("owned_commands")) {t.Fatalf("plan JSON: %s %v",encoded,err)}
    for _,tc:=range []struct{path string;want []byte}{{hook,config},{registry,[]byte("{\"projects\":[]}\n")},{state,[]byte(`{"pid":42,"port":18765}`)},{protected,[]byte("valuable user data")}} {
        got,err:=os.ReadFile(tc.path)
        if err!=nil || !bytes.Equal(got,tc.want) {t.Fatalf("dry-run modified %s: %q %v",tc.path,got,err)}
    }
}

func TestUninstallRejectsUnrelatedFilesAndSymlinks(t *testing.T) {
    exe,_,_,_:=uninstallFixture(t)
    ordinary:=filepath.Join(filepath.Dir(exe),"notes.txt")
    if err:=os.WriteFile(ordinary,[]byte("important"),0600);err!=nil{t.Fatal(err)}
    if _,err:=validateInstalledExecutable(ordinary);err==nil {t.Fatal("accepted unrelated filename")}
    link:=filepath.Join(filepath.Dir(exe),"other","task-mecca")
    if err:=os.MkdirAll(filepath.Dir(link),0700);err!=nil {t.Fatal(err)}
    if runtime.GOOS=="windows" {link+=".exe"}
    if err:=os.Symlink(exe,link);err==nil{
        if _,err:=validateInstalledExecutable(link);err==nil{t.Fatal("accepted symlink executable")}
    }
}

func TestUninstallFailsClosedOnCorruptHookConfiguration(t *testing.T) {
    exe,home,management,protected:=uninstallFixture(t)
    cfg:=filepath.Join(home,".codex","hooks.json")
    if err:=os.MkdirAll(filepath.Dir(cfg),0700);err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(cfg,[]byte("{broken JSON"),0600);err!=nil{t.Fatal(err)}
    if _,err:=buildUninstallPlan(exe,home,management,false);err==nil {
        t.Fatal("corrupt provider settings were silently ignored")
    }
    if _,err:=os.Stat(exe);err!=nil {t.Fatalf("binary should be retained: %v",err)}
    if _,err:=os.Stat(protected);err!=nil {t.Fatalf("project should be retained: %v",err)}
}

func TestUninstallRemoveOnlyOwnedGlobalHooksAndPreserveProject(t *testing.T) {
    if runtime.GOOS=="windows" {t.Skip("Windows removal is deferred to child after CLI exit")}
    exe,home,management,protected:=uninstallFixture(t)
    registry:=filepath.Join(management,"projects.json")
    if err:=os.WriteFile(registry,[]byte("{\"projects\":[]}\n"),0600);err!=nil{t.Fatal(err)}
    for _,p:=range []string{"codex","claude"} {
        config,err:=runtimeobs.HookConfigPath(home,p)
        if err!=nil{t.Fatal(err)}
        if err:=os.MkdirAll(filepath.Dir(config),0700);err!=nil{t.Fatal(err)}
        if err:=os.WriteFile(config,[]byte(`{"permissions":{"allow":["Other"]},"hooks":{"SubagentStart":[{"hooks":[{"type":"command","command":"echo keep-me"}]}]}}`),0600);err!=nil{t.Fatal(err)}
        if _,err:=runtimeobs.EnsureGlobalHooks(p);err!=nil{t.Fatal(err)}
    }
    plan,err:=buildUninstallPlan(exe,home,management,false)
    if err!=nil{t.Fatal(err)}
    var output bytes.Buffer
    if err:=performUninstall(plan,&output);err!=nil{t.Fatal(err)}
    if _,err:=os.Stat(exe);!os.IsNotExist(err){t.Fatalf("binary still exists: %v",err)}
    if _,err:=os.Stat(registry);err!=nil{t.Fatalf("default uninstall purged registry: %v",err)}
    if data,err:=os.ReadFile(protected);err!=nil||string(data)!="valuable user data"{t.Fatalf("data was modified: %q %v",data,err)}
    for _,p:=range []string{"codex","claude"} {
        count,err:=runtimeobs.OwnedHookCount(home,p)
        if err!=nil || count!=0{t.Fatalf("%s Hook still installed: %d %v",p,count,err)}
        config,_:=runtimeobs.HookConfigPath(home,p)
        data,err:=os.ReadFile(config)
        if err!=nil||!strings.Contains(string(data),"echo keep-me")||!strings.Contains(string(data),"Other"){t.Fatalf("other provider settings lost: %s %v",data,err)}
        backup,err:=filepath.Glob(config+".task-mecca-uninstall-*.bak")
        if err!=nil||len(backup)!=1{t.Fatalf("Hook backup missing: %v %v",backup,err)}
        info,err:=os.Stat(backup[0])
        if err!=nil||info.Mode().Perm()!=0600{t.Fatalf("Hook backup insecure or missing: %v %v",info,err)}
        original,err:=os.ReadFile(backup[0])
        if err!=nil||!strings.Contains(string(original),"task-mecca runtime observe "+p){t.Fatalf("backup omitted removed Hook: %s %v",original,err)}
    }
}

func TestUninstallPurgeSettingsOnlyKnownGlobalFiles(t *testing.T) {
    if runtime.GOOS=="windows" {t.Skip("Windows removal is deferred to child after CLI exit")}
    exe,home,management,protected:=uninstallFixture(t)
    for _,f:=range []string{"projects.json","channel","never-delete.txt"} {
        if err:=os.WriteFile(filepath.Join(management,f),[]byte(f),0600);err!=nil{t.Fatal(err)}
    }
    plan,err:=buildUninstallPlan(exe,home,management,true)
    if err!=nil{t.Fatal(err)}
    if len(plan.SettingFiles)!=2{t.Fatalf("expected precisely two known settings, got %#v",plan.SettingFiles)}
    if err:=performUninstall(plan,&bytes.Buffer{});err!=nil{t.Fatal(err)}
    for _,f:=range []string{"projects.json","channel"} {
        if _,err:=os.Stat(filepath.Join(management,f));!os.IsNotExist(err){t.Fatalf("%s not removed: %v",f,err)}
    }
    if _,err:=os.Stat(filepath.Join(management,"never-delete.txt"));err!=nil{t.Fatal(err)}
    if _,err:=os.Stat(protected);err!=nil{t.Fatal(err)}
}
