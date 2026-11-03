package main

import (
    "bytes"
    "encoding/json"
    "testing"
    "time"

    "github.com/silverkhan/TaskMecca/internal/activity"
    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func invokeActivity(t *testing.T,args ...string)(int,map[string]any,string){
    t.Helper()
    var out,errOut bytes.Buffer
    status:=runActivityCLI(args,&out,&errOut)
    result:=map[string]any{}
    if out.Len()>0 {
        if err:=json.Unmarshal(out.Bytes(),&result);err!=nil{t.Fatalf("response: %v; %s",err,out.String())}
    }
    return status,result,errOut.String()
}

func TestActivityCLIStartSwitchStopAndHistory(t *testing.T){
    project:=t.TempDir()
    now:=time.Now().UTC()
    if _,err:=runtimeobs.RecordAssignment(project,"assignment-1","B-589","/root/controller/worker-test",now);err!=nil{t.Fatal(err)}
    if err:=runtimeobs.AppendExecutionEvent(project,runtimeobs.ExecutionEvent{
        EventKind:"state",ObservedAt:now.Format(time.RFC3339Nano),
        AttemptID:"run-test",Provider:"codex",RuntimeAgentID:"worker-test",
        State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,
        ObservationQuality:runtimeobs.QualityObserved,
    });err!=nil{t.Fatal(err)}
    if _,err:=runtimeobs.BindAttempt(project,"run-test","B-589","/root/controller/worker-test","dispatch","",
        map[string]string{"assignment_id":"assignment-1"},now.Add(time.Millisecond));err!=nil{t.Fatal(err)}
    base:=[]string{"--project",project,"--role","worker","--agent","worker-test","--attempt","run-test","--assignment","assignment-1","--evidence","agent observed task phase","--json"}
    args:=func(command,id,kind string)[]string{
        v:=[]string{command,"B-589","--event-id",id}
        if kind!=""{v=append(v,"--type",kind)}
        return append(v,base...)
    }
    if status,_,msg:=invokeActivity(t,args("switch","e-0","implementation")...);status==0{t.Fatalf("switch without start: %s",msg)}
    status,resp,msg:=invokeActivity(t,args("start","e-1","requirements")...)
    if status!=0||resp["recorded"]!=true{t.Fatalf("start: %d %v %s",status,resp,msg)}
    status,resp,msg=invokeActivity(t,args("start","e-1","requirements")...)
    if status!=0||resp["recorded"]!=false{t.Fatalf("idempotent start: %d %v %s",status,resp,msg)}
    if status,_,msg=invokeActivity(t,args("start","e-2","requirements")...);status==0{t.Fatalf("double start: %s",msg)}
    if status,_,msg=invokeActivity(t,args("switch","e-3","implementation")...);status!=0{t.Fatalf("switch: %s",msg)}
    if status,_,msg=invokeActivity(t,args("stop","e-4","")...);status!=0{t.Fatalf("stop: %s",msg)}
    if status,_,msg=invokeActivity(t,args("stop","e-4","")...);status!=0{t.Fatalf("idempotent stop: %s",msg)}
    status,resp,msg=invokeActivity(t,"history","B-589","--project",project,"--json")
    if status!=0{t.Fatalf("history: %s",msg)}
    events:=resp["events"].([]any)
    if len(events)!=3{t.Fatalf("expected 3 events, got %d",len(events))}
    status,resp,msg=invokeActivity(t,"status","B-589","--project",project,"--json")
    if status!=0||len(resp["open_activities"].([]any))!=0{t.Fatalf("status %d %v %s",status,resp,msg)}
    rows,err:= (activity.Ledger{Project:project}).List("B-589")
    if err!=nil||rows[2].Action!=activity.Stop{t.Fatalf("ledger %v %v",rows,err)}
}
