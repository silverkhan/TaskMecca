package preview

import (
 "context"
 "os"
 "os/exec"
 "path/filepath"
 "runtime"
 "strings"
 "testing"
 "time"
)

func TestNativeRunnerRequiresNoDockerAndOnlyStopsOwnedProcess(t *testing.T){
 if runtime.GOOS=="windows"{t.Skip("Windows native launch is separately covered by platform build and Dev acceptance")}
 native:=Native{}
 if err:=native.Check(context.Background());err!=nil{t.Fatalf("native runtime unavailable without Docker: %v",err)}
 root:=t.TempDir()
 sha:=strings.Repeat("a",40)
 env,err:=PrepareEnvironment(filepath.Join(root,"preview"),300,sha)
 if err!=nil{t.Fatal(err)}
 // Build a tiny independent Go binary that mimics the child lifecycle.
 helperPath:=filepath.Join(root,"fixture.go")
 helperSource:=`package main
import ("os";"path/filepath";"time")
func main(){
 _=os.WriteFile(filepath.Join(os.Getenv("HOME"),"child-status.txt"),[]byte(os.Getenv("TASK_MECCA_PREVIEW_MODE")+"|"+os.Getenv("TASK_MECCA_PREVIEW_PR")+"|"+os.Getenv("TASK_MECCA_PREVIEW_SHA")+"|"+os.Getenv("TASK_MECCA_TELEGRAM_TRANSPORT")),0600)
 for{time.Sleep(time.Second)}
}
`
 if err:=os.WriteFile(helperPath,[]byte(helperSource),0600);err!=nil{t.Fatal(err)}
 cache:=filepath.Join(root,"verified-cache")
 builder:=exec.Command("go","build","-o",cache,helperPath)
 if output,err:=builder.CombinedOutput();err!=nil{t.Fatalf("helper Go build: %v %s",err,output)}
 if err:=os.Chmod(cache,0600);err!=nil{t.Fatal(err)}
 port,err:=reservePort()
 if err!=nil{t.Fatal(err)}
 ctx,cancel:=context.WithTimeout(context.Background(),12*time.Second)
 defer cancel()
 process,err:=native.Start(ctx,cache,sha,env,port)
 if err!=nil{t.Fatal(err)}
 stopped:=false
 defer func(){if !stopped{_ = native.Stop(context.Background(),process)}}()
 if process.PID<=0||process.ID==""||process.Port!=port{t.Fatalf("invalid process ownership: %+v",process)}
 marker:=filepath.Join(env.Home,"child-status.txt")
 var contents []byte
 for deadline:=time.Now().Add(4*time.Second);time.Now().Before(deadline);time.Sleep(50*time.Millisecond){
  contents,err=os.ReadFile(marker)
  if err==nil{break}
 }
 if err!=nil{t.Fatalf("native child did not start: %v",err)}
 if string(contents)!="1|300|"+sha+"|disabled"{
  t.Fatalf("Preview environment missing identity or transport protection: %s",contents)
 }
 orig,err:=os.Stat(cache)
 if err!=nil{t.Fatal(err)}
 if orig.Mode().Perm()&0111!=0{t.Fatal("native launch modified shared cache into executable")}
 if err=native.Stop(context.Background(),ProcessHandle{ID:"fake",PID:process.PID});err==nil{
  t.Fatal("unowned PID was accepted")
 }
 if err=native.Stop(context.Background(),process);err!=nil{t.Fatalf("native stop: %v",err)}
 stopped=true
 select{
 case <-process.done:
 case <-time.After(2*time.Second):t.Fatal("native child survived stop")
 }
}
