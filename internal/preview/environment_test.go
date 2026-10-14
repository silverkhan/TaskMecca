package preview

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestPrepareEnvironmentCreatesSyntheticProjectOnly(t *testing.T){
    base:=filepath.Join(t.TempDir(),"previews")
    sha:=strings.Repeat("a",40)
    a,err:=PrepareEnvironment(base,207,sha)
    if err!=nil{t.Fatal(err)}
    b,err:=PrepareEnvironment(base,208,sha)
    if err!=nil{t.Fatal(err)}
    if a.Root==b.Root||a.Home==b.Home||a.Project==b.Project{t.Fatal("Preview instances share storage")}
    if !strings.HasPrefix(a.Project,base+string(os.PathSeparator)){
        t.Fatalf("Preview escaped isolated base: %q",a.Project)
    }
    path:=filepath.Join(a.Project,"_task_mecca","data","backlog")
    entries,err:=os.ReadDir(path)
    if err!=nil||len(entries)!=2{t.Fatalf("synthetic backlog invalid: %d %v",len(entries),err)}
    for _,entry:=range entries{
        data,err:=os.ReadFile(filepath.Join(path,entry.Name()))
        if err!=nil{t.Fatal(err)}
        if !strings.Contains(string(data),"Preview sample"){t.Fatal("Preview fixture unexpectedly references real work")}
    }
    if _,err:=os.Stat(filepath.Join(a.Root,"preview-instance.json"));err!=nil{t.Fatal(err)}
}

func TestChildEnvironmentDropsAllCredentialsAndParentPaths(t *testing.T){
    a,err:=PrepareEnvironment(t.TempDir(),211,strings.Repeat("b",40))
    if err!=nil{t.Fatal(err)}
    unsafe:=[]string{
        "PATH=/usr/bin", "LANG=ko_KR.UTF-8", "HOME=/real/user", "USERPROFILE=C:\\Users\\production",
        "TASK_MECCA_HOME=/real/user/.task-mecca","TASK_MECCA_CHANNEL=stable",
        "TASK_MECCA_GITHUB_TOKEN=secret", "GITHUB_TOKEN=secret", "GH_TOKEN=secret",
        "TELEGRAM_BOT_TOKEN=secret","AWS_SECRET_ACCESS_KEY=secret",
        "SSH_AUTH_SOCK=/real/ssh","HTTPS_PROXY=http://real-proxy",
        "XDG_CONFIG_HOME=/real/config","APPDATA=C:\\Users\\production\\AppData",
    }
    child,err:=ChildEnvironment(unsafe,a)
    if err!=nil{t.Fatal(err)}
    result:=strings.Join(child,"\n")
    for _,forbidden:=range []string{"secret","/real/user","/real/ssh","production","real-proxy"}{
        if strings.Contains(result,forbidden){t.Fatalf("leaked parent environment %q: %s",forbidden,result)}
    }
    required:=[]string{
        "TASK_MECCA_PREVIEW_MODE=1",
        "TASK_MECCA_TELEGRAM_TRANSPORT=disabled",
        "TASK_MECCA_CHANNEL=dev",
        "TASK_MECCA_HOME="+filepath.Join(a.Home,".task-mecca"),
        "HOME="+a.Home,"USERPROFILE="+a.Home,"LANG=ko_KR.UTF-8",
    }
    for _,value:=range required{if !strings.Contains(result,value){t.Fatalf("missing %q in %q",value,result)}}
}

func TestPrepareEnvironmentFailsClosed(t *testing.T){
    base:=t.TempDir()
    for _,input:=range []struct{pr int;sha string}{
        {0,strings.Repeat("a",40)},{21,"not-a-sha"},{11,strings.Repeat("f",39)},
    }{
        _,err:=PrepareEnvironment(base,input.pr,input.sha)
        if err==nil{t.Fatalf("invalid Preview accepted: %+v",input)}
    }
    _,err:=ChildEnvironment(nil,Environment{PRNumber:1,HeadSHA:"wrong",Home:base,Project:base})
    if err==nil{t.Fatal("invalid Preview identity accepted")}
}
