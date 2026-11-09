package main

import (
 "bytes"
 "encoding/json"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/handoff"
 "github.com/silverkhan/TaskMecca/internal/orchestration"
)

func TestOrchestrationCLISelectionAndNames(t *testing.T) {
 root:=t.TempDir()
 for _,name:=range []string{"can_spawn_subagent","can_message_root"} {
  if _,err:=handoff.RecordCapability(root,"codex",name,handoff.CapabilitySupported,"native-smoke-test",time.Now());err!=nil {t.Fatal(err)}
 }
 var out,stderr bytes.Buffer
 call:=func(args ...string) int {
  out.Reset();stderr.Reset()
  return runOrchestrationCLI(args,&out,&stderr)
 }
 if result:=call("configure","--project",root,"--provider","codex","--root-session-id","root-exact","--mode","B","--confirmed","--timezone","Asia/Seoul","--json");result!=0 {t.Fatalf("configure: %s",stderr.String())}
 var g orchestration.Group
 if err:=json.Unmarshal(out.Bytes(),&g);err!=nil{t.Fatal(err)}
 if g.Mode!=orchestration.ModeB {t.Fatalf("%+v",g)}
 if result:=call("status","--project",root,"--provider","codex","--root-session-id","root-exact","--json");result!=0 {t.Fatalf("status: %s",stderr.String())}
 if result:=call("name","--project",root,"--group",g.GroupID,"--role","worker","--worker","Digeuda","--generation","2","--json");result!=0{t.Fatalf("name: %s",stderr.String())}
 var name struct {DisplayName string `json:"display_name"`}
 if err:=json.Unmarshal(out.Bytes(),&name);err!=nil{t.Fatal(err)}
 if name.DisplayName!=g.DisplayPrefix+"Worker_Digeuda_G2" {t.Fatalf("incorrect name: %+v",name)}
 if result:=call("configure","--project",root,"--provider","codex","--root-session-id","root-exact","--mode","A","--confirmed","--json");result==0 {t.Fatal("changed mode without change flag")}
}

func TestOrchestrationCLIRejectsUnverifiedRootMessage(t *testing.T) {
 project:=t.TempDir()
 var out,stderr bytes.Buffer
 status:=runOrchestrationCLI([]string{"configure","--project",project,"--provider","claude","--root-session-id","exact","--mode","C","--confirmed","--json"},&out,&stderr)
 if status==0 {t.Fatal("configured independent session without any native evidence")}
 out.Reset();stderr.Reset()
 status=runOrchestrationCLI([]string{"notify","--project",project,"--provider","claude","--root-session-id","exact","--task","B-100","--json"},&out,&stderr)
 if status==0 {t.Fatal("notified unconfigured Root")}
}
