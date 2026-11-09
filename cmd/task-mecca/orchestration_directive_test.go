package main

import (
 "strings"
 "testing"

 "github.com/silverkhan/TaskMecca/internal/handoff"
 "github.com/silverkhan/TaskMecca/internal/orchestration"
)

func TestRootMessageDirectionIsExecutableByAgentNotCLI(t *testing.T) {
 plan:=rootNotificationPlan{
  Mode:orchestration.ModeB, TaskID:"B-620",
  Decision:orchestration.Decision{Allowed:true,Action:"send_to_root"},
  HandoffID:"handoff-test-123",
  NativeAction:"message_running",
  Target:handoff.Target{
   AgentPath:"/root",Provider:"codex",
   SessionID:"root-native-session",AttemptID:"run-root-1",
   RuntimeAgentID:"root-native-agent",
  },
 }
 d:=rootCommunicationDirective("/project","codex","root-native-session","run-controller-2",
  "B-620","Schema conflict","Choose the approved schema",plan)
 if d==nil {t.Fatal("expected explicit native directive")}
 if d.ActorRole!="/root/controller"||d.TargetRole!="/root" || d.TargetAttemptID!="run-root-1" ||
  d.TargetSessionID!="root-native-session"||d.TargetRuntimeAgentID!="root-native-agent" {
  t.Fatalf("directive lost exact identities: %+v",d)
 }
 for _,s:=range []string{"B-620","handoff-test-123","Schema conflict","Choose the approved schema"} {
  if !strings.Contains(d.Message,s){t.Fatalf("message missing %q: %s",s,d.Message)}
 }
 if d.TransportAction!="message_running"{t.Fatalf("wrong native action: %s",d.TransportAction)}
 command:=strings.Join(d.AfterActualDeliveryCommand," ")
 if !strings.Contains(command,"handoff mark handoff-test-123") ||
  !strings.Contains(command,"--evidence <ACTUAL_NATIVE_TRANSPORT_PROOF>"){
  t.Fatalf("missing follow-up evidence direction: %s",command)
 }
 plan.Decision.Allowed=false
 if got:=rootCommunicationDirective("/project","codex","root-native-session","run-controller-2",
  "B-620","Schema conflict","Choose the approved schema",plan);got!=nil {
  t.Fatal("blocked decision must not yield a native messaging direction")
 }
 plan.Decision.Allowed=true
 plan.HandoffID=""
 if got:=rootCommunicationDirective("/project","codex","root-native-session","run-controller-2",
  "B-620","Schema conflict","Choose the approved schema",plan);got!=nil {
  t.Fatal("missing handoff evidence must not yield a native messaging direction")
 }
}
