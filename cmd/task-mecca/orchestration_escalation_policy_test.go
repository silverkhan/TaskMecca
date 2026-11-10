package main

import (
 "os"
 "path/filepath"
 "strings"
 "testing"

 "github.com/silverkhan/TaskMecca/internal/handoff"
)

func TestCanonicalUserHoldOnly(t *testing.T) {
 fields:=map[string]string{"대기유형":"user","대기":"Needs a policy decision","재개조건":"User chooses A or B"}
 if !canonicalUserHold("hold",fields) {t.Fatal("valid user hold rejected")}
 cases:=[]struct{
  name,state,kind,reason,resume string
 }{
  {"working","doing","user","Needs a policy decision","User chooses A or B"},
  {"external","hold","external","Awaiting provider","Provider replies"},
  {"internal","hold","internal","Need repair","Repair succeeds"},
  {"dependency","hold","dependency","Waiting for dependency","Dependency completes"},
  {"empty reason","hold","user","-","User chooses A or B"},
  {"empty resume","hold","user","Needs a policy decision","-"},
  {"no type","hold","","Needs a policy decision","User chooses A or B"},
 }
 for _,c:=range cases {
  t.Run(c.name,func(t *testing.T){
   got:=canonicalUserHold(c.state,map[string]string{"대기유형":c.kind,"대기":c.reason,"재개조건":c.resume})
   if got {t.Fatalf("non-user hold was accepted: %+v",c)}
  })
 }
}

func TestPriorRootDeliveryPreventsDuplicateInstruction(t *testing.T) {
 cases:=[]struct{name string;row handoff.Handoff;action string;blocked bool}{
  {"new",handoff.Handoff{},"",false},
  {"dispatched",handoff.Handoff{Steps:map[string]handoff.StepState{
   "dispatch":{Result:"ok"},
  }},"root_notification_already_dispatched",true},
  {"failed",handoff.Handoff{Steps:map[string]handoff.StepState{
   "dispatch":{Result:"failed"},
  }},"root_delivery_requires_reconciliation",true},
  {"unknown",handoff.Handoff{Steps:map[string]handoff.StepState{
   "dispatch":{Result:"unknown"},
  }},"root_delivery_requires_reconciliation",true},
  {"claimed",handoff.Handoff{ClaimedBy:"/root"},"root_handoff_already_claimed",true},
  {"applied",handoff.Handoff{Applied:true},"root_handoff_already_claimed",true},
 }
 for _,c:=range cases {
  t.Run(c.name,func(t *testing.T) {
   action,reason,note,blocked:=priorRootDelivery(c.row)
   if blocked!=c.blocked || action!=c.action {t.Fatalf("outcome %s %t, want %s %t",action,blocked,c.action,c.blocked)}
   if blocked && (reason=="" || note=="") {t.Fatalf("no actionable reason: %s %s",reason,note)}
  })
 }
}

func TestRootEscalationFallsBackWhenModeNotConfigured(t *testing.T) {
 project:=t.TempDir()
 folder:=filepath.Join(project,"_task_mecca","data","backlog")
 if err:=os.MkdirAll(folder,0755);err!=nil {t.Fatal(err)}
 path:=filepath.Join(folder,"000001.B-620.user-policy.hold.md")
 write:=func(kind string){
  t.Helper()
  content:="# Policy approval\n## 작업 노트\n- 대기유형: "+kind+"\n- 대기: User must choose policy\n- 재개조건: User confirms policy\n"
  if err:=os.WriteFile(path,[]byte(content),0644);err!=nil {t.Fatal(err)}
 }
 write("user")
 d:=evaluateRootEscalationDecision(project,"B-620","","")
 if d.ExitAllowed || d.Decision!="CONTINUE" || !strings.Contains(d.NextAction,"next Root user turn") {
  t.Fatalf("legacy fallback unavailable: %+v",d)
 }
 d=evaluateRootEscalationDecision(project,"B-620","codex","not-configured")
 if d.ExitAllowed||d.Decision!="CONTINUE" {t.Fatalf("missing mode blocked legacy hold: %+v",d)}
 write("external")
 d=evaluateRootEscalationDecision(project,"B-620","","")
 if d.Decision!="EXIT_BLOCKED" {t.Fatalf("non-user hold went to Root: %+v",d)}
}
