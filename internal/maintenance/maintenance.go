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
    "strconv"
    "strings"
    "sync"
    "time"
)

const (
    repo = "silverkhan/TaskMecca"
    stableChannel = "stable"
    devChannel = "dev"
    stableReleaseTag = "release-stable"
    devReleaseTag = "release-dev"
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
    Channel string `json:"channel"`
    UpdateAvailable bool `json:"update_available"`
    CheckedAt string `json:"checked_at,omitempty"`
    Error string `json:"error,omitempty"`
}

var versionCheckMu sync.Mutex
var versionCheckRunning bool

type FrameworkSyncCandidate struct {
    Name string `json:"name"`
    Path string `json:"path"`
    FromVersion string `json:"from_version"`
    ToVersion string `json:"to_version"`
}

type UpgradeResult struct {
    From string `json:"from"`
    To string `json:"to"`
    Channel string `json:"channel,omitempty"`
    FrameworkSync []FrameworkSyncCandidate `json:"framework_sync,omitempty"`
    Executable string `json:"executable"`
    RestartRequired bool `json:"restart_required"`
    Scheduled bool `json:"scheduled,omitempty"`
}

type ReleaseChannelTarget struct {
    Channel string `json:"channel"`
    Version string `json:"version,omitempty"`
    FrameworkSync []FrameworkSyncCandidate `json:"framework_sync,omitempty"`
    Error string `json:"error,omitempty"`
}

type ReleaseChannelOptions struct {
    CurrentVersion string `json:"current_version"`
    CurrentChannel string `json:"current_channel"`
    Stable ReleaseChannelTarget `json:"stable"`
    Dev ReleaseChannelTarget `json:"dev"`
    EnvironmentOverride bool `json:"environment_override,omitempty"`
}

func homeDir() string {
    if v:=strings.TrimSpace(os.Getenv("TASK_MECCA_HOME")); v!="" { return v }
    home,err:=os.UserHomeDir()
    if err!=nil || home=="" { return "." }
    return filepath.Join(home,".task-mecca")
}

func registryPath() string { return filepath.Join(homeDir(),"projects.json") }
func channelPath() string { return filepath.Join(homeDir(),"channel") }

func normalizeChannel(value string) (string,error) {
    value=strings.ToLower(strings.TrimSpace(value))
    switch value {
    case "",stableChannel:
        return stableChannel,nil
    case devChannel:
        return devChannel,nil
    default:
        return "",fmt.Errorf("unknown update channel %q (use stable or dev)",value)
    }
}

func CurrentChannel() string {
    if value:=strings.TrimSpace(os.Getenv("TASK_MECCA_CHANNEL")); value!="" {
        if channel,err:=normalizeChannel(value); err==nil { return channel }
    }
    if data,err:=os.ReadFile(channelPath()); err==nil {
        if channel,channelErr:=normalizeChannel(string(data)); channelErr==nil { return channel }
    }
    return stableChannel
}

func SetChannel(value string) error {
    channel,err:=normalizeChannel(value)
    if err!=nil { return err }
    if err=os.MkdirAll(homeDir(),0755); err!=nil { return err }
    return os.WriteFile(channelPath(),[]byte(channel+"\n"),0644)
}


func normalizeVersion(value string) string {
    value=strings.TrimSpace(value)
    for {
        previous:=value
        value=strings.TrimSpace(strings.TrimSuffix(value,`\n`))
        value=strings.TrimSpace(strings.TrimSuffix(value,`\r`))
        if value==previous { return value }
    }
}

func frameworkVersion(project string) string {
    data,err:=os.ReadFile(filepath.Join(project,"_task_mecca","VERSION"))
    if err!=nil { return "" }
    return normalizeVersion(string(data))
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

func releaseLocation() (string,string) {
    repoName:=repo
    if v:=strings.TrimSpace(os.Getenv("TASK_MECCA_REPO")); v!="" { repoName=v }
    tag:=stableReleaseTag
    if CurrentChannel()==devChannel { tag=devReleaseTag }
    if v:=strings.TrimSpace(os.Getenv("TASK_MECCA_RELEASE_TAG")); v!="" { tag=v }
    return repoName,tag
}

func releaseBase() string {
    repoName,tag:=releaseLocation()
    return "https://github.com/"+repoName+"/releases/download/"+tag
}

func versionFallbackURL() string {
    repoName,tag:=releaseLocation()
    return "https://raw.githubusercontent.com/"+repoName+"/"+tag+"/goassets/template/_task_mecca/VERSION"
}

func releaseLocationForChannel(value string) (string,string,error) {
    channel,err:=normalizeChannel(value)
    if err!=nil { return "","",err }
    repoName:=repo
    if v:=strings.TrimSpace(os.Getenv("TASK_MECCA_REPO")); v!="" { repoName=v }
    tag:=stableReleaseTag
    if channel==devChannel { tag=devReleaseTag }
    return repoName,tag,nil
}

func releaseBaseForChannel(value string) (string,error) {
    repoName,tag,err:=releaseLocationForChannel(value)
    if err!=nil { return "",err }
    return "https://github.com/"+repoName+"/releases/download/"+tag,nil
}

func versionFallbackURLForChannel(value string) (string,error) {
    repoName,tag,err:=releaseLocationForChannel(value)
    if err!=nil { return "",err }
    return "https://raw.githubusercontent.com/"+repoName+"/"+tag+"/goassets/template/_task_mecca/VERSION",nil
}

func stableReleaseRawURL(path string) string {
    repoName:=repo
    if v:=strings.TrimSpace(os.Getenv("TASK_MECCA_REPO")); v!="" { repoName=v }
    return "https://raw.githubusercontent.com/"+repoName+"/"+stableReleaseTag+"/"+strings.TrimPrefix(path,"/")
}

func httpGet(url string) ([]byte,error) {
    client:=&http.Client{Timeout:5*time.Second}
    var lastErr error
    for attempt:=0; attempt<3; attempt++ {
        req,err:=http.NewRequest(http.MethodGet,url,nil)
        if err!=nil { return nil,err }
        req.Header.Set("User-Agent","task-mecca")
        resp,err:=client.Do(req)
        if err==nil {
            data,readErr:=io.ReadAll(resp.Body)
            _=resp.Body.Close()
            if readErr==nil && resp.StatusCode>=200 && resp.StatusCode<300 { return data,nil }
            if readErr!=nil {
                lastErr=readErr
            } else {
                lastErr=fmt.Errorf("HTTP %d",resp.StatusCode)
                if resp.StatusCode!=404 && resp.StatusCode!=429 && resp.StatusCode<500 { return nil,lastErr }
            }
        } else {
            lastErr=err
        }
        if attempt<2 { time.Sleep(time.Duration(attempt+1)*500*time.Millisecond) }
    }
    if lastErr==nil { lastErr=errors.New("request failed") }
    return nil,lastErr
}

func latestVersionForChannel(value string) (string,error) {
    base,err:=releaseBaseForChannel(value)
    if err!=nil { return "",err }
    data,err:=httpGet(base+"/VERSION.txt")
    if err==nil { return normalizeVersion(string(data)),nil }
    releaseErr:=err
    fallback,fallbackErr:=versionFallbackURLForChannel(value)
    if fallbackErr!=nil { return "",fallbackErr }
    data,err=httpGet(fallback)
    if err!=nil { return "",fmt.Errorf("release version check failed: %v; fallback failed: %v",releaseErr,err) }
    return normalizeVersion(string(data)),nil
}

func GetReleaseChannelOptions(current string) ReleaseChannelOptions {
    out:=ReleaseChannelOptions{
        CurrentVersion:normalizeVersion(current),
        CurrentChannel:CurrentChannel(),
        Stable:ReleaseChannelTarget{Channel:stableChannel},
        Dev:ReleaseChannelTarget{Channel:devChannel},
        EnvironmentOverride:strings.TrimSpace(os.Getenv("TASK_MECCA_CHANNEL"))!="",
    }
    if version,err:=latestVersionForChannel(stableChannel); err!=nil {
        out.Stable.Error=err.Error()
    } else {
        out.Stable.Version=version
        out.Stable.FrameworkSync=frameworkSyncCandidates(current,version,stableChannel)
    }
    if version,err:=latestVersionForChannel(devChannel); err!=nil {
        out.Dev.Error=err.Error()
    } else {
        out.Dev.Version=version
        out.Dev.FrameworkSync=frameworkSyncCandidates(current,version,devChannel)
    }
    return out
}

func frameworkSyncCandidates(currentVersion,targetVersion,targetChannel string) []FrameworkSyncCandidate {
    currentVersion=normalizeVersion(currentVersion)
    targetVersion=normalizeVersion(targetVersion)
    candidates:=[]FrameworkSyncCandidate{}
    for _,project:=range ListProjects() {
        framework:=normalizeVersion(project.FrameworkVersion)
        if framework=="" || framework==targetVersion { continue }
        include:=false
        if targetChannel==stableChannel {
            include=strings.Contains(strings.ToLower(framework),"-dev.")
        } else if targetChannel==devChannel {
            include=framework==currentVersion || !strings.Contains(strings.ToLower(framework),"-dev.")
        }
        if !include { continue }
        candidates=append(candidates,FrameworkSyncCandidate{
            Name:project.Name,Path:project.Path,FromVersion:framework,ToVersion:targetVersion,
        })
    }
    return candidates
}

func StableReleaseNotesIndex() ([]byte,error) {
    return httpGet(stableReleaseRawURL("release-notes/index.json"))
}

func StableReleaseNote(version string) ([]byte,error) {
    version=normalizeVersion(version)
    if version=="" || strings.Contains(version,"..") || strings.ContainsAny(version,"/\\") {
        return nil,errors.New("invalid release note version")
    }
    return httpGet(stableReleaseRawURL("release-notes/"+version+".json"))
}

func versionCachePath() string { return filepath.Join(homeDir(),"update-check-"+CurrentChannel()+".json") }

func CachedVersionInfo(current string) VersionInfo {
    current=normalizeVersion(current)
    info:=VersionInfo{Current:current,Channel:CurrentChannel()}
    if data,err:=os.ReadFile(versionCachePath()); err==nil {
        var cached VersionInfo
        if json.Unmarshal(data,&cached)==nil {
            cached.Current=current
            cached.Channel=CurrentChannel()
            cached.Latest=normalizeVersion(cached.Latest)
            if at,err:=time.Parse(time.RFC3339,cached.CheckedAt); err==nil && time.Since(at)<5*time.Minute {
                cached.UpdateAvailable=newerVersion(cached.Latest,current)
                return cached
            }
            info.Latest=cached.Latest
            info.CheckedAt=cached.CheckedAt
            info.UpdateAvailable=newerVersion(info.Latest,current)
        }
    }
    versionCheckMu.Lock()
    if !versionCheckRunning {
        versionCheckRunning=true
        go func(){
            _=RefreshVersionInfo(current)
            versionCheckMu.Lock(); versionCheckRunning=false; versionCheckMu.Unlock()
        }()
    }
    versionCheckMu.Unlock()
    return info
}

func compareVersions(left,right string) int {
    type semver struct {
        core [3]int
        prerelease []string
        valid bool
    }
    parse:=func(value string) semver {
        value=strings.TrimSpace(strings.TrimPrefix(normalizeVersion(value),"v"))
        value=strings.SplitN(value,"+",2)[0]
        parts:=strings.SplitN(value,"-",2)
        coreParts:=strings.Split(parts[0],".")
        if len(coreParts)!=3 { return semver{} }
        parsed:=semver{valid:true}
        for i:=0;i<3;i++ {
            n,err:=strconv.Atoi(coreParts[i])
            if err!=nil || n<0 { return semver{} }
            parsed.core[i]=n
        }
        if len(parts)==2 {
            if parts[1]=="" { return semver{} }
            parsed.prerelease=strings.Split(parts[1],".")
            for _,item:=range parsed.prerelease { if item=="" { return semver{} } }
        }
        return parsed
    }
    compareIdentifier:=func(a,b string) int {
        ai,aErr:=strconv.Atoi(a)
        bi,bErr:=strconv.Atoi(b)
        aNumeric:=aErr==nil
        bNumeric:=bErr==nil
        if aNumeric && bNumeric {
            if ai<bi { return -1 }
            if ai>bi { return 1 }
            return 0
        }
        if aNumeric && !bNumeric { return -1 }
        if !aNumeric && bNumeric { return 1 }
        return strings.Compare(a,b)
    }

    a,b:=parse(left),parse(right)
    if !a.valid || !b.valid { return strings.Compare(normalizeVersion(left),normalizeVersion(right)) }
    for i:=0;i<3;i++ {
        if a.core[i]<b.core[i] { return -1 }
        if a.core[i]>b.core[i] { return 1 }
    }
    if len(a.prerelease)==0 && len(b.prerelease)==0 { return 0 }
    if len(a.prerelease)==0 { return 1 }
    if len(b.prerelease)==0 { return -1 }
    limit:=len(a.prerelease)
    if len(b.prerelease)<limit { limit=len(b.prerelease) }
    for i:=0;i<limit;i++ {
        if cmp:=compareIdentifier(a.prerelease[i],b.prerelease[i]); cmp!=0 { return cmp }
    }
    if len(a.prerelease)<len(b.prerelease) { return -1 }
    if len(a.prerelease)>len(b.prerelease) { return 1 }
    return 0
}

func newerVersion(latest,current string) bool {
    latest=normalizeVersion(latest)
    current=normalizeVersion(current)
    if latest=="" { return false }
    return compareVersions(latest,current)>0
}

func RefreshVersionInfo(current string) VersionInfo {
    current=normalizeVersion(current)
    fresh:=CheckLatest(current)
    _=os.MkdirAll(homeDir(),0755)
    if data,err:=json.MarshalIndent(fresh,"","  "); err==nil {
        _=os.WriteFile(versionCachePath(),append(data,'\n'),0644)
    }
    return fresh
}

func ReadCachedVersionInfo(current string) VersionInfo {
    current=normalizeVersion(current)
    info:=VersionInfo{Current:current,Channel:CurrentChannel()}
    if data,err:=os.ReadFile(versionCachePath()); err==nil {
        var cached VersionInfo
        if json.Unmarshal(data,&cached)==nil {
            cached.Current=current
            cached.Channel=CurrentChannel()
            cached.Latest=normalizeVersion(cached.Latest)
            cached.UpdateAvailable=newerVersion(cached.Latest,current)
            return cached
        }
    }
    return info
}

func CheckLatest(current string) VersionInfo {
    current=normalizeVersion(current)
    info:=VersionInfo{Current:current,Channel:CurrentChannel(),CheckedAt:time.Now().Format(time.RFC3339)}
    data,err:=httpGet(releaseBase()+"/VERSION.txt")
    if err!=nil {
        releaseErr:=err
        data,err=httpGet(versionFallbackURL())
        if err!=nil {
            info.Error=fmt.Sprintf("release version check failed: %v; fallback failed: %v",releaseErr,err)
            return info
        }
    }
    info.Latest=normalizeVersion(string(data))
    info.UpdateAvailable=newerVersion(info.Latest,current)
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

func fetchReleaseBinary(base string) ([]byte,error) {
    asset,err:=assetName()
    if err!=nil { return nil,err }
    binary,err:=httpGet(base+"/"+asset)
    if err!=nil { return nil,err }
    sums,err:=httpGet(base+"/SHA256SUMS.txt")
    if err!=nil { return nil,err }
    expected,err:=checksumFor(sums,asset)
    if err!=nil { return nil,err }
    digest:=sha256.Sum256(binary)
    if hex.EncodeToString(digest[:])!=expected { return nil,errors.New("SHA-256 verification failed") }
    return binary,nil
}

func installBinary(current,to string,binary []byte) (UpgradeResult,error) {
    result:=UpgradeResult{From:normalizeVersion(current),To:normalizeVersion(to)}
    exe,err:=os.Executable()
    if err!=nil { return result,err }
    exe,err=filepath.EvalSymlinks(exe)
    if err!=nil { return result,err }
    result.Executable=exe

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

func Upgrade(current string) (UpgradeResult,error) {
    current=normalizeVersion(current)
    result:=UpgradeResult{From:current,Channel:CurrentChannel()}
    info:=CheckLatest(current)
    if info.Error!="" { return result,errors.New(info.Error) }
    if !info.UpdateAvailable {
        result.To=current
        return result,nil
    }
    binary,err:=fetchReleaseBinary(releaseBase())
    if err!=nil { return result,err }
    result,err=installBinary(current,info.Latest,binary)
    result.Channel=CurrentChannel()
    return result,err
}

func SwitchChannel(current,target string) (UpgradeResult,error) {
    current=normalizeVersion(current)
    result:=UpgradeResult{From:current}
    if strings.TrimSpace(os.Getenv("TASK_MECCA_CHANNEL"))!="" {
        return result,errors.New("release channel is controlled by TASK_MECCA_CHANNEL; remove the environment override before switching from Web")
    }
    channel,err:=normalizeChannel(target)
    if err!=nil { return result,err }
    targetVersion,err:=latestVersionForChannel(channel)
    if err!=nil { return result,err }
    result.To=targetVersion
    result.Channel=channel
    result.FrameworkSync=frameworkSyncCandidates(current,targetVersion,channel)

    previous:=CurrentChannel()
    if previous==channel && targetVersion==current { return result,nil }

    if targetVersion==current {
        if err=SetChannel(channel); err!=nil { return result,err }
        return result,nil
    }

    base,err:=releaseBaseForChannel(channel)
    if err!=nil { return result,err }
    binary,err:=fetchReleaseBinary(base)
    if err!=nil { return result,err }

    if err=SetChannel(channel); err!=nil { return result,err }
    installed,installErr:=installBinary(current,targetVersion,binary)
    installed.Channel=channel
    installed.FrameworkSync=result.FrameworkSync
    if installErr!=nil {
        _=SetChannel(previous)
        return installed,installErr
    }
    return installed,nil
}
