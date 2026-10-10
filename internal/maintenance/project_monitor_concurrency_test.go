package maintenance

import (
 "os"
 "path/filepath"
 "sync"
 "testing"
 "time"
)

func monitoringFixture(t *testing.T) (string,string) {
 t.Helper()
 dir:=t.TempDir()
 t.Setenv("TASK_MECCA_HOME",filepath.Join(dir,"home"))
 a:=filepath.Join(dir,"one")
 b:=filepath.Join(dir,"two")
 for _,p:=range []string{a,b}{
  if err:=os.MkdirAll(filepath.Join(p,"_task_mecca"),0755);err!=nil{t.Fatal(err)}
  if err:=RegisterProject(p);err!=nil{t.Fatal(err)}
 }
 return a,b
}
func waitMonitoring(t *testing.T,signal <-chan struct{},what string) {
 t.Helper()
 select{case <-signal: case <-time.After(3*time.Second):t.Fatal("timed out waiting for "+what)}
}
func TestMonitoringDifferentProjectsRunConcurrentlyWithoutLosingGuard(t *testing.T) {
 first,second:=monitoringFixture(t)
 enteredFirst:=make(chan struct{})
 releaseFirst:=make(chan struct{})
 finishedFirst:=make(chan bool,1)
 go func(){finishedFirst<-WithProjectMonitoring(first,func(){close(enteredFirst);<-releaseFirst})}()
 waitMonitoring(t,enteredFirst,"first project")
 enteredSecond:=make(chan struct{})
 finishedSecond:=make(chan bool,1)
 go func(){finishedSecond<-WithProjectMonitoring(second,func(){close(enteredSecond)})}()
 // The old global mutex forced this to wait for releaseFirst.
 waitMonitoring(t,enteredSecond,"independent second project")
 if !<-finishedSecond {t.Fatal("second project was suppressed")}
 close(releaseFirst)
 if !<-finishedFirst {t.Fatal("first project was suppressed")}
 projectMonitoringLocks.Lock()
 n:=len(projectMonitoringLocks.entries)
 projectMonitoringLocks.Unlock()
 if n!=0 {t.Fatalf("monitoring lock map leaked %d entries",n)}
}
func TestMonitoringCanonicalProjectAliasesStaySerialized(t *testing.T) {
 first,_:=monitoringFixture(t)
 alias:=filepath.Join(first,"..",filepath.Base(first))
 entered:=make(chan struct{})
 release:=make(chan struct{})
 done:=make(chan bool,1)
 go func(){done<-WithProjectMonitoring(first,func(){close(entered);<-release})}()
 waitMonitoring(t,entered,"canonical first project")
 aliasStarted:=make(chan struct{})
 aliasEntered:=make(chan struct{})
 aliasFinished:=make(chan bool,1)
 go func(){close(aliasStarted);aliasFinished<-WithProjectMonitoring(alias,func(){close(aliasEntered)})}()
 waitMonitoring(t,aliasStarted,"aliased waiter started")
 select{case <-aliasEntered:t.Fatal("same canonical project performed concurrent operations");case <-time.After(60*time.Millisecond):}
 close(release)
 if !<-done{t.Fatal("canonical operation was suppressed")}
 waitMonitoring(t,aliasEntered,"alias after canonical operation")
 if !<-aliasFinished{t.Fatal("alias was suppressed")}
}
func TestMonitoringPauseWaitsForInFlightWorkAndPreventsRestart(t *testing.T) {
 first,_:=monitoringFixture(t)
 running:=make(chan struct{})
 release:=make(chan struct{})
 workFinished:=make(chan bool,1)
 go func(){workFinished<-WithProjectMonitoring(first,func(){close(running);<-release})}()
 waitMonitoring(t,running,"active project before pause")
 pauseStarted:=make(chan struct{})
 pauseFinished:=make(chan error,1)
 go func(){close(pauseStarted);pauseFinished<-SetProjectMonitoring(first,false)}()
 waitMonitoring(t,pauseStarted,"pause request")
 select{case err:=<-pauseFinished:t.Fatalf("pause finished before active work drained: %v",err);case <-time.After(60*time.Millisecond):}
 close(release)
 if !<-workFinished{t.Fatal("running work did not finish")}
 select{case err:=<-pauseFinished:if err!=nil{t.Fatal(err)};case <-time.After(3*time.Second):t.Fatal("pause did not finish")}
 resumed:=false
 if WithProjectMonitoring(first,func(){resumed=true})||resumed{t.Fatal("paused project unexpectedly ran work")}
}

func TestMonitoringManyConcurrentWaitersUseOneLockIdentity(t *testing.T) {
 first,_:=monitoringFixture(t)
 entered:=make(chan struct{})
 release:=make(chan struct{})
 var wg sync.WaitGroup
 var concurrent int
 // Hold the canonical operation while contenders build up.
 wg.Add(1)
 go func(){defer wg.Done();WithProjectMonitoring(first,func(){close(entered);<-release})}()
 waitMonitoring(t,entered,"first lock")
 for i:=0;i<12;i++{
  wg.Add(1)
  go func(){defer wg.Done();WithProjectMonitoring(first,func(){
   // This assert is intentionally protected by the canonical monitor lock.
   concurrent++
  })}()
 }
 close(release)
 wg.Wait()
 if concurrent!=12{t.Fatalf("same project operations lost: %d",concurrent)}
 projectMonitoringLocks.Lock()
 n:=len(projectMonitoringLocks.entries)
 projectMonitoringLocks.Unlock()
 if n!=0{t.Fatalf("per-project lock registry has %d dangling references",n)}
}
