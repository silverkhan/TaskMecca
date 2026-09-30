package maintenance

import (
    "bufio"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "sort"
    "strings"
    "time"
)

const (
    repo = "silverkhan/TaskMecca"
    releaseTag = "go-main"
)

type Project struct {
    Name string `json:"name"`
    Path string `json:"path"`
    FrameworkVersion string `json:"framework_version"`
    LastSeen string `json:"last_seen"`
}

type registry struct {
    Projects []Project `json:"projects"`
}

type VersionInfo struct {
    Current string `json:"current"`
    Latest string `json:"latest,omitempty"`
    UpdateAvailable bool `json:"update_available"`
    CheckedAt string `json:"checked_at,omitempty"`
    Error string `json:"error,omitempty"`
}

type UpgradeResult struct {
    From string `json:"from"`
    To string `json:"to"`
    Executable string `json:"executable"`
    RestartRequired bool `json:"restart_required"`
    Scheduled bool `json:"scheduled,omitempty"`
}

func homeDir() string {
    if v:=strings.TrimSpace(os.Getenv("TASK_MECCA_HOME")); v!="" { return v }
    home,err:=os.UserHomeDir()
    if err!=nil || home=="" { return "." }
    return filepath.Join(home,".task-mecca")
}

func registryPath() string { return filepath.Join(homeDir(),"projects.json") }

func frameworkVersion(project string) string {
    data,err:=os.ReadFile(filepath.Join(project,"_task_mecca","VERSION"))
    if err!=nil { return "" }
    return strings.TrimSpace(string(data))
}

func RegisterProject(project string) error {
    abs,err:=filepath.Abs(project)
    if err!=nil { return err }
    if _,err=os.Stat(filepath.Join(abs,"_task_mecca")); err!=nil { return err }
    _=os.MkdirAll(homeDir(),0755)
    reg:=registry{}
    if data,readErr:=os.ReadFile(registryPath()); readErr==nil { _=json.Unmarshal(data,&reg) }
    now:=time.Now().Format(time.RFC3339)
    found:=false
    for i:=range reg.Projects {
        if filepath.Clean(reg.Projects[i].Path)==filepath.Clean(abs) {
            reg.Projects[i].Name=filepath.Base(abs)
            reg.Projects[i].FrameworkVersion=frameworkVersion(abs)
            reg.Projects[i].LastSeen=now
            found=true
            break
        }
    }
    if !found {
        reg.Projects=append(reg.Projects,Project{Name:filepath.Base(abs),Path:abs,FrameworkVersion:frameworkVersion(abs),LastSeen:now})
    }
    sort.Slice(reg.Projects,func(i,j int) bool { return strings.ToLower(reg.Projects[i].Name)<strings.ToLower(reg.Projects[j].Name) })
    data,err:=json.MarshalIndent(reg,"","  ")
    if err!=nil { return err }
    return os.WriteFile(registryPath(),append(data,'\n'),0644)
}

func ListProjects() []Project {
    reg:=registry{}
    data,err:=os.ReadFile(registryPath())
    if err!=nil { return []Project{} }
    if json.Unmarshal(data,&reg)!=nil { return []Project{} }
    out:=make([]Project,0,len(reg.Projects))
    for _,p:=range reg.Projects {
        if _,err:=os.Stat(filepath.Join(p.Path,"_task_mecca")); err!=nil { continue }
        p.FrameworkVersion=frameworkVersion(p.Path)
        out=append(out,p)
    }
    return out
}

func IsRegisteredProject(path string) bool {
    abs,err:=filepath.Abs(path); if err!=nil { return false }
    for _,p:=range ListProjects() {
        pabs,_:=filepath.Abs(p.Path)
        if filepath.Clean(pabs)==filepath.Clean(abs) { return true }
    }
    return false
}

func releaseBase() string {
    repoName:=repo
    if v:=strings.TrimSpace(os.Getenv("TASK_MECCA_REPO")); v!="" { repoName=v }
    tag:=releaseTag
    if v:=strings.TrimSpace(os.Getenv("TASK_MECCA_RELEASE_TAG")); v!="" { tag=v }
    return "https://github.com/"+repoName+"/releases/download/"+tag
}

func httpGet(url string) ([]byte,error) {
    client:=&http.Client{Timeout:15*time.Second}
    resp,err:=client.Get(url)
    if err!=nil { return nil,err }
    defer resp.Body.Close()
    if resp.StatusCode<200 || resp.StatusCode>=300 { return nil,fmt.Errorf("HTTP %d",resp.StatusCode) }
    return io.ReadAll(resp.Body)
}

func CheckLatest(current string) VersionInfo {
    info:=VersionInfo{Current:current,CheckedAt:time.Now().Format(time.RFC3339)}
    data,err:=httpGet(releaseBase()+"/VERSION.txt")
    if err!=nil { info.Error=err.Error(); return info }
    info.Latest=strings.TrimSpace(string(data))
    info.UpdateAvailable=info.Latest!="" && info.Latest!=current
    return info
}

func assetName() (string,error) {
    machine:=""
    switch runtime.GOARCH {
    case "amd64": machine="amd64"
    case "arm64": machine="arm64"
    default: return "",fmt.Errorf("unsupported architecture: %s",runtime.GOARCH)
    }
    switch runtime.GOOS {
    case "darwin": return "task-mecca-macos-"+machine,nil
    case "linux":
        if machine!="amd64" { return "",errors.New("linux arm64 release is not available") }
        return "task-mecca-linux-"+machine,nil
    case "windows":
        if machine!="amd64" { return "",errors.New("windows arm64 release is not available") }
        return "task-mecca-windows-"+machine+".exe",nil
    default: return "",fmt.Errorf("unsupported operating system: %s",runtime.GOOS)
    }
}

func checksumFor(data []byte,asset string) (string,error) {
    s:=bufio.NewScanner(strings.NewReader(string(data)))
    for s.Scan() {
        fields:=strings.Fields(s.Text())
        if len(fields)<2 { continue }
        name:=strings.TrimPrefix(fields[1],"*")
        if filepath.Base(name)==asset { return fields[0],nil }
    }
    if err:=s.Err(); err!=nil { return "",err }
    return "",fmt.Errorf("checksum entry missing for %s",asset)
}

func Upgrade(current string) (UpgradeResult,error) {
    result:=UpgradeResult{From:current}
    info:=CheckLatest(current)
    if info.Error!="" { return result,errors.New(info.Error) }
    result.To=info.Latest
    exe,err:=os.Executable()
    if err!=nil { return result,err }
    exe,err=filepath.EvalSymlinks(exe)
    if err!=nil { return result,err }
    result.Executable=exe
    if !info.UpdateAvailable { return result,nil }
    asset,err:=assetName(); if err!=nil { return result,err }
    binary,err:=httpGet(releaseBase()+"/"+asset); if err!=nil { return result,err }
    sums,err:=httpGet(releaseBase()+"/SHA256SUMS.txt"); if err!=nil { return result,err }
    expected,err:=checksumFor(sums,asset); if err!=nil { return result,err }
    digest:=sha256.Sum256(binary)
    if hex.EncodeToString(digest[:])!=expected { return result,errors.New("SHA-256 verification failed") }

    dir:=filepath.Dir(exe)
    tmp,err:=os.CreateTemp(dir,".task-mecca-upgrade-*")
    if err!=nil { return result,fmt.Errorf("cannot write next to executable %s: %w",exe,err) }
    tmpPath:=tmp.Name()
    defer func(){ _=os.Remove(tmpPath) }()
    if _,err=tmp.Write(binary); err!=nil { _=tmp.Close(); return result,err }
    if err=tmp.Close(); err!=nil { return result,err }
    if runtime.GOOS!="windows" {
        if err=os.Chmod(tmpPath,0755); err!=nil { return result,err }
        if err=os.Rename(tmpPath,exe); err!=nil { return result,err }
        result.RestartRequired=true
        return result,nil
    }

    newPath:=exe+".new"
    _=os.Remove(newPath)
    if err=os.Rename(tmpPath,newPath); err!=nil { return result,err }
    script:=exe+".upgrade.cmd"
    body:=fmt.Sprintf("@echo off\r\n:wait\r\nmove /Y \"%s\" \"%s\" >nul 2>&1\r\nif errorlevel 1 (timeout /t 1 /nobreak >nul & goto wait)\r\ndel \"%%~f0\"\r\n",newPath,exe)
    if err=os.WriteFile(script,[]byte(body),0600); err!=nil { return result,err }
    cmd:=exec.Command("cmd","/C","start","\"Task Mecca Upgrade\"","/MIN",script)
    if err=cmd.Start(); err!=nil { return result,err }
    result.RestartRequired=true
    result.Scheduled=true
    return result,nil
}
