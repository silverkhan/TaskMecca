package preview

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "strings"
)

// Environment is a non-operational project and user home for preview fixtures.
// It isolates normal Task Mecca files; it is NOT an OS sandbox for malicious Go binaries.
type Environment struct {
    PRNumber int `json:"pr_number"`
    HeadSHA string `json:"head_sha"`
    Root string `json:"root"`
    Home string `json:"home"`
    Project string `json:"project"`
    Logs string `json:"logs"`
    ParentURL string `json:"-"`
}

func PrepareEnvironment(base string,pr int,sha string)(Environment,error){
    if base=="" || pr<1 || !validSHA(sha) {return Environment{},errors.New("invalid Preview root, PR number or SHA")}
    if err:=os.MkdirAll(base,0700);err!=nil{return Environment{},err}
    root,err:=os.MkdirTemp(base,"pr-"+strconv.Itoa(pr)+"-"+sha[:10]+"-")
    if err!=nil{return Environment{},err}
    cleanup:=true
    defer func(){if cleanup{_ = os.RemoveAll(root)}}()

    result:=Environment{
        PRNumber:pr,HeadSHA:sha,Root:root,
        Home:filepath.Join(root,"home"),
        Project:filepath.Join(root,"sample-project"),
        Logs:filepath.Join(root,"logs"),
    }
    folders:=[]string{
        result.Home,filepath.Join(result.Home,".task-mecca"),filepath.Join(result.Home,".config"),
        filepath.Join(result.Home,".cache"),filepath.Join(result.Home,"AppData"),
        filepath.Join(result.Home,"LocalAppData"),result.Logs,
        filepath.Join(result.Project,"_task_mecca","data","backlog"),
    }
    for _,path:=range folders {
        if err:=os.MkdirAll(path,0700);err!=nil{return Environment{},err}
    }
    backlog:=filepath.Join(result.Project,"_task_mecca","data","backlog")
    tasks:=map[string]string{
        "000001.B-1.preview-guide.todo.md": "# B-1 Preview sample: registered\n- Agent: -\n- 설명: This is synthetic Preview test data. No operating project is mounted.\n",
        "000002.B-2.preview-done.done.md": "# B-2 Preview sample: completed\n- Agent: -\n- 결과: Synthetic completed task for visual validation.\n",
    }
    for name,body:=range tasks {
        if err:=os.WriteFile(filepath.Join(backlog,name),[]byte(body),0600);err!=nil{return Environment{},err}
    }
    marker,err:=json.MarshalIndent(map[string]any{
        "schema_version":1,"pr_number":pr,"head_sha":sha,
        "warning":"synthetic Preview fixture, not a trusted OS sandbox",
    },"","  ")
    if err!=nil{return Environment{},err}
    if err=os.WriteFile(filepath.Join(root,"preview-instance.json"),append(marker,'\n'),0600);err!=nil{return Environment{},err}
    cleanup=false
    return result,nil
}

// ChildEnvironment intentionally carries no GitHub/Telegram/SSH/Cloud credentials,
// parent Task Mecca configuration, proxy configuration or inherited OS HOME.
// It cannot protect secrets that a malicious binary can read through the host OS.
func ChildEnvironment(parent []string,env Environment) ([]string,error) {
    if env.Home==""||env.Project==""||!validSHA(env.HeadSHA)||env.PRNumber<1 {
        return nil,errors.New("Preview environment is not initialized")
    }
    safeKeys:=map[string]bool{
        "PATH":true,"LANG":true,"LC_ALL":true,"LC_CTYPE":true,"TZ":true,
        "SYSTEMROOT":true,"WINDIR":true,"COMSPEC":true,"PATHEXT":true,
    }
    passed:=map[string]string{}
    for _,entry:=range parent{
        pair:=strings.SplitN(entry,"=",2)
        if len(pair)!=2{continue}
        key:=strings.ToUpper(pair[0])
        if safeKeys[key]{passed[key]=pair[1]}
    }
    // Force a separate OS and Task Mecca configuration home. On Windows the
    // config and cache paths must be isolated independently of USERPROFILE.
    passed["HOME"]=env.Home
    passed["USERPROFILE"]=env.Home
    passed["APPDATA"]=filepath.Join(env.Home,"AppData")
    passed["LOCALAPPDATA"]=filepath.Join(env.Home,"LocalAppData")
    passed["XDG_CONFIG_HOME"]=filepath.Join(env.Home,".config")
    passed["XDG_CACHE_HOME"]=filepath.Join(env.Home,".cache")
    passed["TASK_MECCA_HOME"]=filepath.Join(env.Home,".task-mecca")
    passed["TASK_MECCA_PREVIEW_MODE"]="1"
    passed["TASK_MECCA_TELEGRAM_TRANSPORT"]="disabled"
    passed["TASK_MECCA_CHANNEL"]="dev"
    keys:=make([]string,0,len(passed))
    for k:=range passed{keys=append(keys,k)}
    // deterministic environment for tests and reproducible launches
    sortStrings(keys)
    out:=make([]string,0,len(keys))
    for _,key:=range keys{
        if strings.ContainsAny(key,"=\x00")||strings.ContainsRune(passed[key],'\x00'){
            return nil,fmt.Errorf("invalid Preview environment key %q",key)
        }
        out=append(out,key+"="+passed[key])
    }
    return out,nil
}

func sortStrings(values []string){
    for i:=1;i<len(values);i++{
        for j:=i;j>0&&values[j]<values[j-1];j--{values[j],values[j-1]=values[j-1],values[j]}
    }
}
