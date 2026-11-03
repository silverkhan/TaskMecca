package main

import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "io"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "time"

 "github.com/silverkhan/TaskMecca/internal/activity"
)

// runActivityCommand wraps an explicitly requested test/build process. The
// process runs without a shell and only after the start event is committed.
// It restores a prior open activity when the child exits, including failures.
// This is opt-in; it does not launch automatic full regressions.
func runActivityCommand(args []string,out,errOut io.Writer)int{
 sep:=-1
 for i,x:=range args{if x=="--"{sep=i;break}}
 if sep<0||sep==len(args)-1{
  fmt.Fprintln(errOut,"usage: activity run TASK [options] -- PROGRAM [ARGS...]");return 2
 }
 flagsInput:=append([]string{},args[:sep]...)
 command:=append([]string{},args[sep+1:]...)
 if len(flagsInput)>0&&!strings.HasPrefix(flagsInput[0],"-"){flagsInput=append(flagsInput[1:],flagsInput[0])}
 fs:=flag.NewFlagSet("activity run",flag.ContinueOnError);fs.SetOutput(errOut)
 project:=fs.String("project",".","project root")
 kind:=fs.String("type","","targeted_test|full_regression|build_check")
 role:=fs.String("role",os.Getenv("TASK_MECCA_ROLE"),"runtime role")
 agent:=fs.String("agent",os.Getenv("TASK_MECCA_AGENT_ID"),"runtime agent")
 attempt:=fs.String("attempt",os.Getenv("TASK_MECCA_ATTEMPT_ID"),"runtime attempt")
 assignment:=fs.String("assignment","","worker assignment")
 handoff:=fs.String("handoff","","controller handoff")
 eventID:=fs.String("event-id","","stable unique run ID")
 round:=fs.Int("round",-1,"automatically derived rework round")
 sha:=fs.String("git-sha","","tested revision")
 scope:=fs.String("scope","","test/build scope")
 purpose:=fs.String("purpose","validation","reason for execution")
 if err:=fs.Parse(flagsInput);err!=nil{return 2}
 if fs.NArg()!=1 || *eventID=="" || *agent=="" || *attempt=="" || !activity.Role(*role).Valid() ||
  (*kind!=string(activity.TargetedTest)&&*kind!=string(activity.FullRegression)&&*kind!=string(activity.BuildCheck)){
  fmt.Fprintln(errOut,"task, event-id, exact actor identity and a test/build type are required");return 2
 }
 if len(*eventID)>110 || strings.ContainsAny(*eventID," /\\\n\r\x00"){
  fmt.Fprintln(errOut,"unsafe or excessively long --event-id");return 2
 }
 root,err:=filepath.Abs(*project)
 if err!=nil{fmt.Fprintln(errOut,err);return 2}
 task:=fs.Arg(0)
 store:=activity.Ledger{Project:root}
 // No automatic rerun on retry: an already committed result is final
 // evidence. A dangling start requires explicit reconciliation.
 endID:=*eventID+"-end"
 if _,err:=store.Read(endID);err==nil{
  fmt.Fprintln(errOut,"the run already has a recorded outcome; not executing twice");return 3
 }else if !os.IsNotExist(err){fmt.Fprintln(errOut,err);return 1}
 if _,err:=store.Read(*eventID+"-begin");err==nil{
  fmt.Fprintln(errOut,"the run start exists without an outcome; reconcile before retrying");return 3
 }else if !os.IsNotExist(err){fmt.Fprintln(errOut,err);return 1}

 rows,err:=store.List(task)
 if err!=nil{fmt.Fprintln(errOut,err);return 1}
 var previous *activity.Event
 for i:=len(rows)-1;i>=0;i--{
  e:=&rows[i]
  if e.AgentID==*agent&&e.RuntimeAttemptID==*attempt&&string(e.Role)==*role&&e.Action!=activity.Correct{
   if e.Action==activity.Start || e.Action==activity.Switch{previous=e}
   break
  }
 }
 // Save exact argv equality without persisting secrets possibly present in
 // flags or arguments. Basename and fingerprint suffice for repeat detection.
 argvJSON,_:=json.Marshal(command)
 sum:=sha256.Sum256(argvJSON)
 commandEvidence:=filepath.Base(command[0])+"#sha256:"+hex.EncodeToString(sum[:12])
 common:=[]string{"--project",root,"--role",*role,"--agent",*agent,"--attempt",*attempt}
 if *assignment!=""{common=append(common,"--assignment",*assignment)}
 if *handoff!=""{common=append(common,"--handoff",*handoff)}
 if *round>=0{common=append(common,"--round",fmt.Sprint(*round))}
 if previous!=nil{common=append(common,"--round",fmt.Sprint(previous.Round))}
 makeEventArgs:=func(op,id,typ,evidence string,extra ...string)[]string{
  options:=[]string{op,task,"--event-id",id,"--type",typ,"--evidence",evidence}
  options=append(options,common...)
  options=append(options,extra...)
  return options
 }
 beginAction:="start";if previous!=nil{beginAction="switch"}
 started:=makeEventArgs(beginAction,*eventID+"-begin",*kind,"test/build process start")
 started=append(started,"--run-id",*eventID,"--command",commandEvidence,"--scope",*scope,
  "--git-sha",*sha,"--purpose",*purpose)
 if status:=runActivityCLI(started,io.Discard,errOut);status!=0{
  fmt.Fprintln(errOut,"test/build not executed because Activity start was rejected");return status
 }
 startTime:=time.Now()
 cmd:=exec.Command(command[0],command[1:]...)
 cmd.Stdout=out;cmd.Stderr=errOut
 cmd.Stdin=os.Stdin
 runErr:=cmd.Run()
 result:="passed"
 exitCode:=0
 if runErr!=nil{
  result="failed";exitCode=1
  var exitError *exec.ExitError
  if errors.As(runErr,&exitError){exitCode=exitError.ExitCode()}
  if exitCode<0{exitCode=1}
 }
 endAction:="stop";endType:=*kind
 endExtra:=[]string{}
 if previous!=nil {
  endAction="switch";endType=string(previous.Type)
  if previous.Type==activity.Other{
   endExtra=append(endExtra,"--explanation",previous.Explanation)
   if previous.ProposedType!=""{endExtra=append(endExtra,"--proposed-type",previous.ProposedType)}
  }
 }
 ending:=makeEventArgs(endAction,endID,endType,"test/build process exited",endExtra...)
 ending=append(ending,"--run-id",*eventID,"--command",commandEvidence,"--scope",*scope,
  "--git-sha",*sha,"--result",result)
 if status:=runActivityCLI(ending,io.Discard,errOut);status!=0{
  fmt.Fprintln(errOut,"WARNING: child process ran but Activity result failed to record")
  return status
 }
 fmt.Fprintf(errOut,"Activity run recorded: type=%s result=%s exit=%d elapsed_ms=%.1f\n",
  *kind,result,exitCode,float64(time.Since(startTime).Microseconds())/1000)
 return exitCode
}
