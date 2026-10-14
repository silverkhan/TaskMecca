package preview

import (
 "context"
 "crypto/rand"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "net"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "sync"
 "time"
)

// Docker is the only launch backend. Running downloaded Preview binaries on
// the host is intentionally unsupported. There is NO host-exec fallback.
type Docker struct{}

func dockerCommand(ctx context.Context, args ...string) (string,error) {
 cmd:=exec.CommandContext(ctx,"docker",args...)
 // Never let an injected DOCKER_HOST/DOCKER_CONTEXT redirect launch to an
 // arbitrary remote engine, even if the user's active local context is safe.
 for _,entry:=range os.Environ() {
  key:=strings.ToUpper(strings.SplitN(entry,"=",2)[0])
  switch key {
  case "DOCKER_HOST","DOCKER_CONTEXT","DOCKER_TLS_VERIFY","DOCKER_CERT_PATH","DOCKER_TLS_CERTDIR":
   continue
  }
  cmd.Env=append(cmd.Env,entry)
 }
 out,err:=cmd.CombinedOutput()
 if err!=nil {
  message:=strings.TrimSpace(string(out))
  if len(message)>350 {message=message[:350]}
  return "",fmt.Errorf("Docker %s failed: %s: %w",args[0],message,err)
 }
 return strings.TrimSpace(string(out)),nil
}

func (Docker) Check(ctx context.Context) error {
 // A remote Docker daemon would run the PR on an uncontrolled machine.
 raw,err:=dockerCommand(ctx,"context","inspect","--format","{{json .Endpoints.docker.Host}}")
 if err!=nil{return fmt.Errorf("Docker Desktop/Engine required for Preview: %w",err)}
 var endpoint string
 if json.Unmarshal([]byte(raw),&endpoint)!=nil || !(strings.HasPrefix(endpoint,"unix://")||strings.HasPrefix(endpoint,"npipe://")) {
  return errors.New("Preview requires a local Docker Unix socket or Windows named pipe; remote Docker is not allowed")
 }
 if _,err=dockerCommand(ctx,"info","--format","{{.ServerVersion}}");err!=nil{return err}
 if _,err=dockerCommand(ctx,"image","inspect","alpine:3.20","--format","{{.Id}}");err!=nil{
  return errors.New("Docker image alpine:3.20 is not installed; run docker pull alpine:3.20, then retry")
 }
 return nil
}
type Sandbox struct {
 ContainerID string `json:"container_id"`
 Network string `json:"network"`
 Port int `json:"port"`
 URL string `json:"url"`
}
type SandboxRuntime interface {
 Check(context.Context)error
 Start(context.Context,string,string,Environment,int)(Sandbox,error)
 Stop(context.Context,Sandbox)error
}

func previewRandomID()(string,error) {
 buf:=make([]byte,8)
 if _,err:=rand.Read(buf);err!=nil{return "",err}
 return hex.EncodeToString(buf),nil
}
func reservePort()(int,error) {
 listener,err:=net.Listen("tcp","127.0.0.1:0")
 if err!=nil{return 0,err}
 port:=listener.Addr().(*net.TCPAddr).Port
 _=listener.Close()
 if port==18765{return reservePort()}
 return port,nil
}

func (Docker) Start(ctx context.Context,binary,digest string,env Environment,port int)(Sandbox,error) {
 if port<1024||port>65535||port==18765 {return Sandbox{},errors.New("unsafe Preview port")}
 if !validSHA(env.HeadSHA)||!strings.HasPrefix(digest,env.HeadSHA[:12]) {
  return Sandbox{},errors.New("Preview binary identity mismatch")
 }
 source,err:=filepath.Abs(binary)
 if err!=nil{return Sandbox{},err}
 stat,err:=os.Lstat(source)
 if err!=nil{return Sandbox{},err}
 if !stat.Mode().IsRegular()||stat.Mode().Perm()&0111!=0{return Sandbox{},errors.New("Preview cache must be a verified non-executable file")}
 id,err:=previewRandomID()
 if err!=nil{return Sandbox{},err}
 netName:="tm-preview-"+id
 if _,err=dockerCommand(ctx,"network","create","--internal","--driver","bridge",
  "--label","task-mecca.preview=1",netName);err!=nil{return Sandbox{},err}
 started:=false
 defer func(){if !started{_,_=dockerCommand(context.Background(),"network","rm",netName)}}()
 // Docker may read bind mounts that are not inside the user's local host.
 // Using a local, verified daemon is checked in Check().
 args:=[]string{
  "run","-d","--rm","--name",netName,
  "--label","task-mecca.preview=1",
  "--label","task-mecca.preview.id="+id,
  "--network",netName,
  "--platform","linux/amd64",
  "--publish",fmt.Sprintf("127.0.0.1:%d:18765",port),
  "--read-only","--cap-drop=ALL","--security-opt=no-new-privileges",
  "--pids-limit=64","--memory=384m","--cpus=1",
  "--tmpfs","/tmp:rw,nosuid,nodev,size=128m,mode=1777",
  "--mount","type=bind,source="+source+",target=/opt/task-mecca,readonly",
  "--mount","type=bind,source="+env.Project+",target=/preview/project",
  "--env","HOME=/tmp",
  "--env","TASK_MECCA_HOME=/tmp/task-mecca",
  "--env","TASK_MECCA_PREVIEW_MODE=1",
  "--env","TASK_MECCA_PREVIEW_SANDBOX=docker",
  "--env","TASK_MECCA_TELEGRAM_TRANSPORT=disabled",
  "--env","TASK_MECCA_CHANNEL=dev",
  "--entrypoint","/opt/task-mecca",
  "alpine:3.20",
  "web","--foreground","--host","0.0.0.0","--port","18765",
  "--project","/preview/project","--no-open",
 }
 // Preserve cached bytes as mode 0600 until the sandbox is created.
 // Docker can execute a read-only mounted file if it has executable bits.
 // Stage only a verified executable in this dedicated Preview area.
 if err=os.Chmod(source,0500);err!=nil{return Sandbox{},err}
 defer os.Chmod(source,0600)
 container,err:=dockerCommand(ctx,args...)
 if err!=nil{return Sandbox{},err}
 started=true
 return Sandbox{ContainerID:container,Network:netName,Port:port,
  URL:fmt.Sprintf("http://127.0.0.1:%d/",port)},nil
}
func (Docker) Stop(ctx context.Context,s Sandbox)error{
 if s.ContainerID=="" || !strings.HasPrefix(s.Network,"tm-preview-"){return errors.New("unowned Preview sandbox")}
 // No PID-based kill: Docker owns and verifies the exact container ID.
 _,stopErr:=dockerCommand(ctx,"container","stop","--time","3",s.ContainerID)
 _,netErr:=dockerCommand(ctx,"network","rm",s.Network)
 if stopErr!=nil{return stopErr}
 return netErr
}

type Instance struct {
 PRNumber int `json:"pr_number"`
 HeadSHA string `json:"head_sha"`
 BuildRunID int64 `json:"build_run_id"`
 Status string `json:"status"`
 URL string `json:"url,omitempty"`
 Port int `json:"port,omitempty"`
 StartedAt string `json:"started_at,omitempty"`
 Error string `json:"error,omitempty"`
 Sandbox Sandbox `json:"-"`
 Environment Environment `json:"-"`
}
type Manager struct {
 mu sync.Mutex
 current map[int]Instance
 busy map[int]bool
 Client *Client
 Runtime SandboxRuntime
 Base string
 HTTP *http.Client
}
func NewManager(base string)*Manager{
 return &Manager{current:map[int]Instance{},busy:map[int]bool{},
  Client:NewClient(),Runtime:Docker{},Base:base,HTTP:&http.Client{Timeout:2*time.Second}}
}
func (m *Manager) List()[]Instance {
 m.mu.Lock()
 defer m.mu.Unlock()
 result:=make([]Instance,0,len(m.current))
 for _,v:=range m.current{result=append(result,v)}
 // Stable ordering makes duplicate UI requests deterministic.
 for i:=1;i<len(result);i++ {for j:=i;j>0&&result[j].PRNumber<result[j-1].PRNumber;j-- {
  result[j],result[j-1]=result[j-1],result[j]
 }}
 return result
}
func (m *Manager) setStatus(number int,change func(*Instance)){
 m.mu.Lock()
 defer m.mu.Unlock()
 item:=m.current[number]
 change(&item)
 m.current[number]=item
}
func (m *Manager) Start(ctx context.Context,number int)(Instance,error){
 if number<=0{return Instance{},errors.New("invalid PR number")}
 m.mu.Lock()
 if m.busy[number] {v:=m.current[number];m.mu.Unlock();return v,errors.New("Preview start or stop already in progress")}
 if v,found:=m.current[number];found&&v.Status=="running" {m.mu.Unlock();return v,nil}
 m.busy[number]=true
 m.current[number]=Instance{PRNumber:number,Status:"preparing"}
 m.mu.Unlock()
 defer func(){m.mu.Lock();delete(m.busy,number);m.mu.Unlock()}()
 fail:=func(err error)(Instance,error){
  m.setStatus(number,func(v *Instance){v.Status="error";v.Error=err.Error()})
  return Instance{},err
 }
 if m.Runtime==nil ||m.Client==nil{return fail(errors.New("Preview runner unavailable"))}
 if err:=m.Runtime.Check(ctx);err!=nil{return fail(err)}
 builds,err:=m.Client.ListBuilds(ctx,30)
 if err!=nil{return fail(err)}
 var selected Build
 for _,b:=range builds{if b.Number==number{selected=b;break}}
 if selected.Number==0 ||selected.BuildStatus!="success"||selected.ArtifactID<=0 {
  return fail(errors.New("no successful verified PR head artifact available"))
 }
 m.setStatus(number,func(v *Instance){v.HeadSHA=selected.HeadSHA;v.BuildRunID=selected.RunID;v.Status="downloading"})
 // Linux/amd64 is the portable Docker Preview target even on macOS/Windows.
 binary,err:=m.Client.DownloadVerified(ctx,selected,filepath.Join(m.Base,"artifacts"),"linux","amd64")
 if err!=nil{return fail(err)}
 env,err:=PrepareEnvironment(filepath.Join(m.Base,"instances"),number,selected.HeadSHA)
 if err!=nil{return fail(err)}
 m.setStatus(number,func(v *Instance){v.Status="starting"})
 var sandbox Sandbox
 for try:=0;try<3;try++{
  port,portErr:=reservePort()
  if portErr!=nil{err=portErr;break}
  sandbox,err=m.Runtime.Start(ctx,binary,selected.HeadSHA,env,port)
  if err==nil{break}
 }
 if err!=nil{return fail(err)}
 healthErr:=m.waitReady(ctx,sandbox)
 if healthErr!=nil{
  stopCtx,cancel:=context.WithTimeout(context.Background(),12*time.Second)
  _=m.Runtime.Stop(stopCtx,sandbox)
  cancel()
  return fail(healthErr)
 }
 now:=time.Now().UTC().Format(time.RFC3339)
 m.setStatus(number,func(v *Instance){
  v.Status="running";v.URL=sandbox.URL;v.Port=sandbox.Port
  v.StartedAt=now;v.Sandbox=sandbox;v.Environment=env
 })
 for _,v:=range m.List(){if v.PRNumber==number{return v,nil}}
 return Instance{},errors.New("Preview disappeared during launch")
}
func (m *Manager) waitReady(ctx context.Context,s Sandbox)error{
 client:=m.HTTP
 if client==nil{client=&http.Client{Timeout:2*time.Second}}
 deadline:=time.NewTimer(25*time.Second)
 defer deadline.Stop()
 ticker:=time.NewTicker(350*time.Millisecond)
 defer ticker.Stop()
 for {
  req,err:=http.NewRequestWithContext(ctx,"GET",s.URL+"api/health",nil)
  if err!=nil{return err}
  resp,err:=client.Do(req)
  if err==nil{
   var health struct {Ok bool `json:"ok"`}
   if resp.StatusCode==200{_=json.NewDecoder(resp.Body).Decode(&health)}
   _=resp.Body.Close()
   if health.Ok{return nil}
  }
  select {
  case <-ctx.Done():return ctx.Err()
  case <-deadline.C:return errors.New("Preview failed health check within 25 seconds")
  case <-ticker.C:
  }
 }
}
func (m *Manager) Stop(ctx context.Context,number int)error{
 m.mu.Lock()
 if m.busy[number]{m.mu.Unlock();return errors.New("Preview is busy")}
 item,ok:=m.current[number]
 if !ok||item.Status!="running"{m.mu.Unlock();return errors.New("Preview is not running")}
 m.busy[number]=true;item.Status="stopping";m.current[number]=item
 m.mu.Unlock()
 defer func(){m.mu.Lock();delete(m.busy,number);m.mu.Unlock()}()
 if err:=m.Runtime.Stop(ctx,item.Sandbox);err!=nil{
  m.setStatus(number,func(v *Instance){v.Status="error";v.Error=err.Error()})
  return err
 }
 m.setStatus(number,func(v *Instance){v.Status="stopped";v.URL="";v.Port=0;v.Sandbox=Sandbox{}})
 return nil
}

// Restart/recovery of unmanaged containers belongs to AID-153; the first
// manager version deliberately does not kill unknown containers or host PIDs.
