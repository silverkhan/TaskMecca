package webui

import (
    "bytes"
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"
)

const DefaultPort = 18765

type ServiceState struct {
    PID int `json:"pid"`
    InstanceID string `json:"instance_id"`
    ControlToken string `json:"control_token,omitempty"`
    Host string `json:"host"`
    Port int `json:"port"`
    URL string `json:"url"`
    LocalURL string `json:"local_url,omitempty"`
    TailscaleURL string `json:"tailscale_url,omitempty"`
    TLSEnabled bool `json:"tls_enabled"`
    TLSError string `json:"tls_error,omitempty"`
    Version string `json:"version"`
    Project string `json:"project"`
    StartedAt string `json:"started_at"`
    Running bool `json:"running"`
}

func webHomeDir() string {
    if v:=strings.TrimSpace(os.Getenv("TASK_MECCA_HOME")); v!="" { return v }
    home,err:=os.UserHomeDir()
    if err!=nil || home=="" { return "." }
    return filepath.Join(home,".task-mecca")
}

func webServiceDir() string { return filepath.Join(webHomeDir(),"web") }
func webStatePath() string { return filepath.Join(webServiceDir(),"state.json") }
func WebLogPath() string { return filepath.Join(webServiceDir(),"web.log") }

func randomHex(n int) (string,error) {
    raw:=make([]byte,n)
    if _,err:=rand.Read(raw); err!=nil { return "",err }
    return hex.EncodeToString(raw),nil
}

func NewServiceIdentity() (string,string,error) {
    id,err:=randomHex(12)
    if err!=nil { return "","",err }
    token,err:=randomHex(24)
    if err!=nil { return "","",err }
    return id,token,nil
}

func writeServiceState(state ServiceState) error {
    if err:=os.MkdirAll(webServiceDir(),0755); err!=nil { return err }
    data,err:=json.MarshalIndent(state,"","  ")
    if err!=nil { return err }
    tmp:=webStatePath()+".tmp"
    if err=os.WriteFile(tmp,append(data,'\n'),0600); err!=nil { return err }
    return os.Rename(tmp,webStatePath())
}

func removeServiceState(instanceID string,pid ...int) {
    data,err:=os.ReadFile(webStatePath())
    if err!=nil { return }
    var current ServiceState
    if json.Unmarshal(data,&current)==nil {
        if instanceID!="" && current.InstanceID!=instanceID { return }
        if len(pid)>0 && pid[0]>0 && current.PID!=pid[0] { return }
    }
    _=os.Remove(webStatePath())
}

func readServiceState() (ServiceState,error) {
    data,err:=os.ReadFile(webStatePath())
    if err!=nil { return ServiceState{},err }
    var state ServiceState
    if err=json.Unmarshal(data,&state); err!=nil { return ServiceState{},err }
    return state,nil
}

func healthState(state ServiceState) bool {
    healthURL:=state.LocalURL
    if healthURL=="" { healthURL=state.URL }
    if healthURL=="" || state.InstanceID=="" { return false }
    client:=http.Client{Timeout:700*time.Millisecond}
    resp,err:=client.Get(strings.TrimRight(healthURL,"/")+"/api/health")
    if err!=nil { return false }
    defer resp.Body.Close()
    if resp.StatusCode!=http.StatusOK { return false }
    var payload map[string]any
    if json.NewDecoder(resp.Body).Decode(&payload)!=nil { return false }
    return fmt.Sprint(payload["instance_id"])==state.InstanceID
}

func IsTailscaleHost(host string) bool {
    return isTailscaleIPv4(net.ParseIP(strings.TrimSpace(host)))
}

func NormalizeManagedHost(host string) string {
    host=strings.TrimSpace(host)
    if host=="" || strings.EqualFold(host,"auto") { return "auto" }
    if IsTailscaleHost(host) { return "auto" }
    return host
}

func ServiceStatus() ServiceState {
    state,err:=readServiceState()
    if err!=nil { return ServiceState{} }
    state.Running=healthState(state)
    if !state.Running { removeServiceState(state.InstanceID) }
    state.ControlToken=""
    return state
}

func serviceStatusWithToken() ServiceState {
    state,err:=readServiceState()
    if err!=nil { return ServiceState{} }
    state.Running=healthState(state)
    if !state.Running { removeServiceState(state.InstanceID) }
    return state
}

func StartService(config Config) (ServiceState,error) {
    if current:=ServiceStatus(); current.Running {
        if config.OpenBrowser { openBrowser(current.URL) }
        return current,nil
    }
    id,token,err:=NewServiceIdentity()
    if err!=nil { return ServiceState{},err }
    if strings.TrimSpace(config.Host)=="" { config.Host="auto" }
    if config.Port<=0 { config.Port=DefaultPort }

    if err=os.MkdirAll(webServiceDir(),0755); err!=nil { return ServiceState{},err }
    logFile,err:=os.OpenFile(WebLogPath(),os.O_CREATE|os.O_WRONLY|os.O_APPEND,0644)
    if err!=nil { return ServiceState{},err }

    exe,err:=os.Executable()
    if err!=nil { _=logFile.Close(); return ServiceState{},err }
    exe,_=filepath.EvalSymlinks(exe)
    args:=[]string{"web","--foreground","--project",config.Project,"--host",config.Host,"--port",fmt.Sprint(config.Port),"--no-open","--web-instance-id",id,"--web-control-token",token}
    if config.Root!="" { args=append(args,"--root",config.Root) }
    cmd:=exec.Command(exe,args...)
    cmd.Dir=config.Project
    cmd.Stdout=logFile
    cmd.Stderr=logFile
    if err=cmd.Start(); err!=nil { _=logFile.Close(); return ServiceState{},err }
    _=logFile.Close()
    _=cmd.Process.Release()

    deadline:=time.Now().Add(6*time.Second)
    for time.Now().Before(deadline) {
        time.Sleep(150*time.Millisecond)
        state,readErr:=readServiceState()
        if readErr==nil && state.InstanceID==id && healthState(state) {
            state.Running=true
            state.ControlToken=""
            if config.OpenBrowser { openBrowser(state.URL) }
            return state,nil
        }
    }
    tail,_:=os.ReadFile(WebLogPath())
    if len(tail)>2000 { tail=tail[len(tail)-2000:] }
    return ServiceState{},fmt.Errorf("Task Mecca Web failed to start on %s:%d\n%s",config.Host,config.Port,strings.TrimSpace(string(tail)))
}

func StopService() (ServiceState,error) {
    state:=serviceStatusWithToken()
    if !state.Running { return state,nil }
    controlURL:=state.LocalURL
    if controlURL=="" { controlURL=state.URL }
    endpoint:=strings.TrimRight(controlURL,"/")+"/api/admin/stop"
    req,err:=http.NewRequest(http.MethodPost,endpoint,bytes.NewReader(nil))
    if err!=nil { return ServiceState{},err }
    req.Header.Set("X-Task-Mecca-Control",state.ControlToken)
    client:=http.Client{Timeout:2*time.Second}
    resp,err:=client.Do(req)
    if err!=nil { return ServiceState{},err }
    _,_=io.Copy(io.Discard,resp.Body)
    _=resp.Body.Close()
    if resp.StatusCode!=http.StatusOK { return ServiceState{},fmt.Errorf("Web stop failed: HTTP %d",resp.StatusCode) }
    deadline:=time.Now().Add(5*time.Second)
    for time.Now().Before(deadline) {
        time.Sleep(100*time.Millisecond)
        if !healthState(state) { removeServiceState(state.InstanceID); state.Running=false; state.ControlToken=""; return state,nil }
    }
    return ServiceState{},errors.New("Task Mecca Web did not stop within timeout")
}

func RestartService(config Config) (ServiceState,error) {
    current:=serviceStatusWithToken()
    if current.Running {
        if config.Host=="" || config.Host=="auto" {
            config.Host=NormalizeManagedHost(current.Host)
        }
        if config.Port<=0 || config.Port==DefaultPort { config.Port=current.Port }
        if _,err:=StopService(); err!=nil { return ServiceState{},err }
    }
    return StartService(config)
}


func StreamLogs(w io.Writer, follow bool) error {
    if err:=os.MkdirAll(webServiceDir(),0755); err!=nil { return err }
    file,err:=os.OpenFile(WebLogPath(),os.O_CREATE|os.O_RDONLY,0644)
    if err!=nil { return err }
    defer file.Close()
    if !follow {
        _,err=io.Copy(w,file)
        return err
    }
    for {
        if _,err=io.Copy(w,file); err!=nil { return err }
        time.Sleep(300*time.Millisecond)
    }
}
