package preview

import (
 "context"
 "encoding/json"
 "errors"
 "net/http"
 "path/filepath"
 "runtime"
 "sort"
 "sync"
 "time"
)
type Instance struct {
 PRNumber int `json:"pr_number"`
 HeadSHA string `json:"head_sha"`
 BuildRunID int64 `json:"build_run_id"`
 Status string `json:"status"`
 URL string `json:"url,omitempty"`
 Port int `json:"port,omitempty"`
 StartedAt string `json:"started_at,omitempty"`
 Error string `json:"error,omitempty"`
 Process ProcessHandle `json:"-"`
 Environment Environment `json:"-"`
}
type Manager struct {
 mu sync.Mutex
 current map[int]Instance
 busy map[int]bool
 Client *Client
 Runtime ProcessRuntime
 Base string
 HTTP *http.Client
}
func NewManager(base string)*Manager{
 return &Manager{current:map[int]Instance{},busy:map[int]bool{},
  Client:NewClient(),Runtime:Native{},Base:base,HTTP:&http.Client{Timeout:2*time.Second}}
}
func (m *Manager) List()[]Instance {
 m.mu.Lock()
 defer m.mu.Unlock()
 result:=make([]Instance,0,len(m.current))
 for _,v:=range m.current{result=append(result,v)}
 sort.Slice(result,func(i,j int)bool{return result[i].PRNumber<result[j].PRNumber})
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
 // Download the binary for the current native platform. No Docker dependency.
 binary,err:=m.Client.DownloadVerified(ctx,selected,filepath.Join(m.Base,"artifacts"),runtime.GOOS,runtime.GOARCH)
 if err!=nil{return fail(err)}
 env,err:=PrepareEnvironment(filepath.Join(m.Base,"instances"),number,selected.HeadSHA)
 if err!=nil{return fail(err)}
 m.setStatus(number,func(v *Instance){v.Status="starting"})
 var process ProcessHandle
 for try:=0;try<3;try++{
  port,portErr:=reservePort()
  if portErr!=nil{err=portErr;break}
  process,err=m.Runtime.Start(ctx,binary,selected.HeadSHA,env,port)
  if err==nil{break}
 }
 if err!=nil{return fail(err)}
 healthErr:=m.waitReady(ctx,process,number,selected.HeadSHA)
 if healthErr!=nil{
  stopCtx,cancel:=context.WithTimeout(context.Background(),12*time.Second)
  _=m.Runtime.Stop(stopCtx,process)
  cancel()
  return fail(healthErr)
 }
 now:=time.Now().UTC().Format(time.RFC3339)
 m.setStatus(number,func(v *Instance){
  v.Status="running";v.URL=process.URL;v.Port=process.Port
  v.StartedAt=now;v.Process=process;v.Environment=env
 })
 if process.done!=nil {
  go func(){
   exitErr,ok:=<-process.done
   m.setStatus(number,func(v *Instance){
    if v.Process.ID!=process.ID||v.Status=="stopping"||v.Status=="stopped"{return}
    v.Status="error";v.URL="";v.Port=0
    if ok&&exitErr!=nil {v.Error="Preview process exited: "+exitErr.Error()}else{v.Error="Preview process exited"}
   })
  }()
 }
 for _,v:=range m.List(){if v.PRNumber==number{return v,nil}}
 return Instance{},errors.New("Preview disappeared during launch")
}
func (m *Manager) waitReady(ctx context.Context,s ProcessHandle,number int,sha string)error{
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
   var health struct {
    Ok bool `json:"ok"`
    PreviewMode bool `json:"preview_mode"`
    PR int `json:"preview_pr"`
    SHA string `json:"preview_sha"`
   }
   if resp.StatusCode==200{_=json.NewDecoder(resp.Body).Decode(&health)}
   _=resp.Body.Close()
   if health.Ok&&health.PreviewMode&&health.PR==number&&health.SHA==sha{return nil}
  }
  select {
  case <-ctx.Done():return ctx.Err()
  case <-deadline.C:return errors.New("Preview did not confirm the expected PR identity within 25 seconds")
  case <-ticker.C:
  }
 }
}
func (m *Manager) Stop(ctx context.Context,number int)error{
 m.mu.Lock()
 if m.busy[number]{m.mu.Unlock();return errors.New("Preview is busy")}
 item,ok:=m.current[number]
 if !ok||(item.Status!="running"&&item.Status!="error")||item.Process.ID==""{m.mu.Unlock();return errors.New("Preview is not running")}
 m.busy[number]=true;item.Status="stopping";m.current[number]=item
 m.mu.Unlock()
 defer func(){m.mu.Lock();delete(m.busy,number);m.mu.Unlock()}()
 if err:=m.Runtime.Stop(ctx,item.Process);err!=nil{
  m.setStatus(number,func(v *Instance){v.Status="error";v.Error=err.Error()})
  return err
 }
 m.setStatus(number,func(v *Instance){v.Status="stopped";v.URL="";v.Port=0;v.Process=ProcessHandle{}})
 return nil
}

// Recovery of an older parent process is AID-153. Never kill unknown OS PIDs.
