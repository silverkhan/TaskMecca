package preview

import (
 "context"
 "crypto/rand"
 "encoding/hex"
 "errors"
 "fmt"
 "io"
 "net"
 "os"
 "os/exec"
 "path/filepath"
 "runtime"
 "strconv"
 "time"
)

// Native uses an ordinary Go child process. This is NOT an OS sandbox:
// trusted PR code runs with the same filesystem privileges as its parent.
type Native struct{}
type ProcessHandle struct {
 ID string `json:"instance_id"`
 PID int `json:"pid"`
 BinaryPath string `json:"-"`
 Port int `json:"port"`
 URL string `json:"url"`
 process *os.Process
 done <-chan error
}
type ProcessRuntime interface {
 Check(context.Context)error
 Start(context.Context,string,string,Environment,int)(ProcessHandle,error)
 Stop(context.Context,ProcessHandle)error
}
func (Native) Check(ctx context.Context)error{
 if err:=ctx.Err();err!=nil{return err}
 _,err:=assetForPlatform(runtime.GOOS,runtime.GOARCH)
 return err
}
func randomInstanceID()(string,error){
 entropy:=make([]byte,16)
 if _,err:=rand.Read(entropy);err!=nil{return "",err}
 return hex.EncodeToString(entropy),nil
}
func reservePort()(int,error){
 l,err:=net.Listen("tcp","127.0.0.1:0")
 if err!=nil{return 0,err}
 defer l.Close()
 port:=l.Addr().(*net.TCPAddr).Port
 if port==18765{return 0,errors.New("Preview cannot bind to the Dev port")}
 return port,nil
}
// Copy only to a unique instance directory, never chmod the shared artifact
// cache. Preserve a private, non-operational project directory as cwd.
func stageNativeBinary(source string,env Environment)(string,error){
 if !validSHA(env.HeadSHA)||env.Root=="" {return "",errors.New("invalid Preview identity")}
 info,err:=os.Lstat(source)
 if err!=nil{return "",err}
 if !info.Mode().IsRegular()||info.Mode().Perm()&0111!=0 {
  return "",errors.New("Preview cache must be a regular non-executable file")
 }
 input,err:=os.Open(source)
 if err!=nil{return "",err}
 defer input.Close()
 name:="task-mecca-preview"
 if runtime.GOOS=="windows"{name+=".exe"}
 dest:=filepath.Join(env.Root,name)
 output,err:=os.OpenFile(dest,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600)
 if err!=nil{return "",err}
 size,err:=io.Copy(output,io.LimitReader(input,maxBinaryBytes+1))
 if closeErr:=output.Close();err==nil{err=closeErr}
 if err!=nil||size>maxBinaryBytes{_ = os.Remove(dest);return "",errors.New("failed to stage a bounded Preview executable")}
 if err=os.Chmod(dest,0700);err!=nil{_ = os.Remove(dest);return "",err}
 return dest,nil
}
func (Native) Start(ctx context.Context,binary,sha string,env Environment,port int)(ProcessHandle,error){
 if err:=ctx.Err();err!=nil{return ProcessHandle{},err}
 if port<=1024||port>65535||port==18765||!validSHA(sha)||sha!=env.HeadSHA||env.PRNumber<=0 {
  return ProcessHandle{},errors.New("invalid Preview launch parameters")
 }
 if env.Project==""||env.Home==""||env.Root=="" {return ProcessHandle{},errors.New("Preview environment not prepared")}
 if stat,err:=os.Stat(env.Project);err!=nil||!stat.IsDir(){return ProcessHandle{},errors.New("Preview test project unavailable")}
 stage,err:=stageNativeBinary(binary,env)
 if err!=nil{return ProcessHandle{},err}
 childEnv,err:=ChildEnvironment(os.Environ(),env)
 if err!=nil{return ProcessHandle{},err}
 parentURL,err:=previewParentOrigin(env.ParentURL)
 if err!=nil{return ProcessHandle{},err}
 childEnv=append(childEnv,"TASK_MECCA_PREVIEW_PR="+strconv.Itoa(env.PRNumber),
   "TASK_MECCA_PREVIEW_SHA="+sha,"TASK_MECCA_PREVIEW_PARENT_URL="+parentURL)
 id,err:=randomInstanceID()
 if err!=nil{return ProcessHandle{},err}
 logs,err:=os.OpenFile(filepath.Join(env.Logs,"web.log"),os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600)
 if err!=nil{return ProcessHandle{},err}
 // CommandContext would kill the child as soon as the HTTP request completes.
 cmd:=exec.Command(stage,"web","--foreground","--host","127.0.0.1",
  "--port",strconv.Itoa(port),"--project",env.Project,"--no-open")
 cmd.Dir=env.Project
 cmd.Env=childEnv
 cmd.Stdout,cmd.Stderr=logs,logs
 if err=cmd.Start();err!=nil{_ = logs.Close();return ProcessHandle{},fmt.Errorf("Preview process failed: %w",err)}
 done:=make(chan error,1)
 go func(){done<-cmd.Wait();close(done);_ = logs.Close()}()
 return ProcessHandle{ID:id,PID:cmd.Process.Pid,BinaryPath:stage,Port:port,
  URL:fmt.Sprintf("http://127.0.0.1:%d/",port),process:cmd.Process,done:done},nil
}
func (Native) Stop(ctx context.Context,h ProcessHandle)error{
 if h.ID==""||h.PID<=0||h.process==nil||h.process.Pid!=h.PID||h.done==nil {
  return errors.New("Preview process ownership unavailable: refusing PID-based stop")
 }
 select {case <-h.done:return nil;default:}
 if runtime.GOOS!="windows"{_ = h.process.Signal(os.Interrupt)}
 timer:=time.NewTimer(3*time.Second)
 defer timer.Stop()
 select {case <-h.done:return nil;case <-ctx.Done():case <-timer.C:}
 if err:=h.process.Kill();err!=nil&&!errors.Is(err,os.ErrProcessDone){return err}
 select {case <-h.done:return nil;case <-time.After(3*time.Second):return errors.New("Preview process did not terminate")}
}
