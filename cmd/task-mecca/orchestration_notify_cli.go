package main

import (
 "encoding/json"
 "flag"
 "fmt"
 "io"
 "path/filepath"
 "strings"
 "time"

 "github.com/silverkhan/TaskMecca/internal/backlog"
 "github.com/silverkhan/TaskMecca/internal/handoff"
 "github.com/silverkhan/TaskMecca/internal/orchestration"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type nativeMessageDirective struct {
 ActorRole string `json:"actor_role"`
 ActorAttemptID string `json:"actor_attempt_id"`
 TargetRole string `json:"target_role"`
 TargetProvider string `json:"target_provider"`
 TargetSessionID string `json:"target_session_id"`
 TargetAttemptID string `json:"target_attempt_id"`
 TargetRuntimeAgentID string `json:"target_runtime_agent_id,omitempty"`
 TransportAction string `json:"transport_action"`
 Message string `json:"message"`
 AfterActualDeliveryCommand []string `json:"after_actual_delivery_command"`
 OnTransportFailure string `json:"on_transport_failure"`
}

func rootCommunicationDirective(
 project,provider,rootSessionID,controllerAttemptID,task,reason,resume string,
 plan rootNotificationPlan,
) *nativeMessageDirective {
 if plan.HandoffID=="" || plan.NativeAction=="" || !plan.Decision.Allowed {
  return nil
 }
 // This payload is direction for the *currently executing agent*, not an
 // instruction to the CLI to send or to mark dispatch success.
 payload:=fmt.Sprintf(
  "Task Mecca USER_DECISION_REQUIRED\nTask: %s\nHandoff: %s\n"+
   "Blocking reason: %s\nResume condition: %s\n"+
   "Please inspect the canonical task and ask the user for the required decision. "+
   "Do not interpret this message as authorization or automatically resume the task.",
  task,plan.HandoffID,reason,resume,
 )
 return &nativeMessageDirective{
  ActorRole:"/root/controller",ActorAttemptID:controllerAttemptID,
  TargetRole:"/root",TargetProvider:provider,
  TargetSessionID:rootSessionID,TargetAttemptID:plan.Target.AttemptID,
  TargetRuntimeAgentID:plan.Target.RuntimeAgentID,
  TransportAction:plan.NativeAction,Message:payload,
  AfterActualDeliveryCommand:[]string{
   "task-mecca","handoff","mark",plan.HandoffID,
   "--project",project,"--step","dispatch","--result","ok",
   "--evidence","<ACTUAL_NATIVE_TRANSPORT_PROOF>","--json",
  },
  OnTransportFailure:"Do not report delivery; record failed/unknown transport evidence through handoff mark and preserve the canonical hold(user) for a subsequent Root turn.",
 }
}

// Only an explicit canonical user hold may request Root communication.
// External, internal and dependency holds continue through their existing
// owners; a filename ending in .hold.md alone does not authorize escalation.
func canonicalUserHold(state any, fields map[string]string) bool {
 meaningful:=func(value string) bool {
  v:=strings.TrimSpace(value)
  return v!="" && v!="-"
 }
 return state=="hold" &&
  strings.EqualFold(strings.TrimSpace(fields["대기유형"]),"user") &&
  meaningful(fields["대기"]) && meaningful(fields["재개조건"])
}

// Suppress duplicate Root messages across Controller resumes. A previous
// unknown/failed dispatch must be reconciled rather than blindly replayed.
func priorRootDelivery(row handoff.Handoff)(action,reason,note string,alreadyHandled bool){
 if row.Applied || row.ClaimedBy!="" {
  return "root_handoff_already_claimed","handoff_claimed_or_applied",
   "Root handoff is already claimed/applied; do not send again.",true
 }
 if dispatch,ok:=row.Steps["dispatch"];ok {
  if dispatch.Result=="ok" {
   return "root_notification_already_dispatched","dispatch_already_confirmed",
    "Actual transport was previously recorded; do not send again.",true
  }
  return "root_delivery_requires_reconciliation","previous_dispatch_"+dispatch.Result,
   "Prior send failed/unknown; inspect handoff and native state before any redispatch.",true
 }
 return "","","",false
}

type rootNotificationPlan struct {
 Mode orchestration.Mode `json:"mode"`
 TaskID string `json:"task_id"`
 Decision orchestration.Decision `json:"decision"`
 HandoffID string `json:"handoff_id,omitempty"`
 NativeAction string `json:"native_action,omitempty"`
 Target handoff.Target `json:"target,omitempty"`
 RequiresFreshPreflight bool `json:"requires_fresh_preflight,omitempty"`
 Delivered bool `json:"delivered"`
 Note string `json:"note"`
 Directive *nativeMessageDirective `json:"directive,omitempty"`
}

func runOrchestrationNotifyCLI(args []string,stdout,stderr io.Writer) int {
 flags:=flag.NewFlagSet("orchestration notify",flag.ContinueOnError)
 flags.SetOutput(stderr)
 project:=flags.String("project",".","canonical project")
 provider:=flags.String("provider","","exact runtime provider")
 rootID:=flags.String("root-session-id","","provider-native Root session ID")
 task:=flags.String("task","","canonical task ID in hold(user)")
 source:=flags.String("controller-attempt","","exact source Controller attempt")
 target:=flags.String("root-attempt","","exact recipient Root attempt")
 asJSON:=flags.Bool("json",false,"structured JSON")
 if err:=flags.Parse(args);err!=nil{return 2}
 if flags.NArg()!=0||*provider==""||*rootID==""||*task==""{
  fmt.Fprintln(stderr,"notify requires --provider, --root-session-id, --task; native send additionally needs --controller-attempt and --root-attempt")
  return 2
 }
 projectAbs,err:=filepath.Abs(*project)
 if err!=nil{fmt.Fprintln(stderr,err);return 2}
 group,ok,err:=orchestration.FindGroup(projectAbs,*provider,*rootID)
 if err!=nil{fmt.Fprintln(stderr,err);return 1}
 if !ok{fmt.Fprintln(stderr,"Root orchestration mode is not configured");return 1}
 snap,err:=backlog.Inspect(projectAbs,"",*task)
 if err!=nil{fmt.Fprintln(stderr,err);return 1}
 if snap["exists"]!=true||snap["duplicate_count"]!=1{
  fmt.Fprintln(stderr,"an exact canonical backlog task is required")
  return 1
 }
 fields,_:=snap["fields"].(map[string]string)
 reason:=strings.TrimSpace(fields["대기"])
 resume:=strings.TrimSpace(fields["재개조건"])
 durable:=canonicalUserHold(snap["state"],fields)
 caps,err:=runtimeOrchestrationCapabilities(projectAbs,*provider)
 if err!=nil{fmt.Fprintln(stderr,err);return 1}
 decision,err:=orchestration.Decide(orchestration.Request{
  Mode:group.Mode,Event:"user_decision_required",TaskID:*task,
  Capabilities:caps,DurableRecorded:durable,
 })
 if err!=nil{fmt.Fprintln(stderr,err);return 2}
 plan:=rootNotificationPlan{Mode:group.Mode,TaskID:*task,Decision:decision,Delivered:false,Note:"Policy only; no native send has occurred"}
 if decision.Action=="send_to_root" {
  if *source==""||*target=="" {
   plan.Decision.Allowed=false
   plan.Decision.Action="exact_runtime_attempts_required"
   plan.Decision.Reasons=append(plan.Decision.Reasons,"controller_and_root_attempt_required")
  }else{
   ledger,err:=runtimeobs.BuildLedger(projectAbs,20,time.Now())
   if err!=nil{fmt.Fprintln(stderr,err);return 1}
   rootBound,controllerBound:=false,false
   for _,attempt:=range ledger.Attempts{
    if attempt.BindingState!=runtimeobs.BindingBound {continue}
    if attempt.AttemptID==*target && attempt.AgentPath=="/root" &&
       attempt.SessionID==*rootID && strings.EqualFold(attempt.Provider,*provider){
     rootBound=true
    }
    if attempt.AttemptID==*source && attempt.AgentPath=="/root/controller" &&
       strings.EqualFold(attempt.Provider,*provider){controllerBound=true}
   }
   if !rootBound||!controllerBound {
    plan.Decision.Allowed=false;plan.Decision.Action="exact_identity_unverified"
    plan.Decision.Reasons=append(plan.Decision.Reasons,"bound_root_or_controller_attempt_missing")
   }else{
    prepared,err:=handoff.Prepare(handoff.PrepareRequest{
     Project:projectAbs,TaskID:*task,EventType:handoff.EventUserDecisionRequired,
     SourceAgentPath:"/root/controller",TargetAgentPath:"/root",
     SourceAttemptID:*source,TargetAttemptID:*target,
    },time.Now())
    if err!=nil{fmt.Fprintln(stderr,err);return 1}
    plan.HandoffID=prepared.HandoffID
    plan.Target=prepared.Target
    plan.RequiresFreshPreflight=prepared.RequiresFreshPreflight
    // A prepared handoff is idempotent by task/event/source attempt.
    // Never direct a second message after dispatch success, claim, or
    // unresolved failure. Transport recovery must use the existing journal.
    existing,exists,inspectErr:=handoff.Inspect(projectAbs,prepared.HandoffID,time.Now())
    if inspectErr!=nil {fmt.Fprintln(stderr,inspectErr);return 1}
    if !exists {fmt.Fprintln(stderr,"prepared handoff is missing from journal");return 1}
    if action,priorReason,note,blocked:=priorRootDelivery(existing);blocked {
     plan.Decision.Allowed=false;plan.Decision.Action=action
     plan.Decision.Reasons=append(plan.Decision.Reasons,priorReason)
     plan.Note=note
    }else{
    switch prepared.Action {
    case handoff.ActionMessageRunning:plan.NativeAction="message_running"
    case handoff.ActionResumeCompleted:
     if strings.EqualFold(caps.WakeRoot,"supported") {
      plan.NativeAction="resume_completed_after_preflight"
     } else {
      plan.Decision.Allowed=false;plan.Decision.Action="await_next_root_turn"
      plan.Decision.Reasons=append(plan.Decision.Reasons,"root_wake_not_verified")
     }
    default:
     plan.Decision.Allowed=false;plan.Decision.Action="await_next_root_turn"
     plan.Decision.Reasons=append(plan.Decision.Reasons,"handoff_target_not_messageable:"+prepared.Reason)
    }
    if plan.NativeAction!=""{
     plan.Note="Prepared handoff evidence; follow directive with an available provider-native message/resume tool, then mark with actual evidence. Not sent by CLI."
     plan.Directive=rootCommunicationDirective(projectAbs,*provider,*rootID,*source,*task,reason,resume,plan)
    }
    }
   }
  }
 }
 if *asJSON {
  if err:=json.NewEncoder(stdout).Encode(plan);err!=nil{fmt.Fprintln(stderr,err);return 1}
 } else {
  fmt.Fprintf(stdout,"%s: %s; handoff=%s; native=%s\n%s\n",group.Mode,plan.Decision.Action,plan.HandoffID,plan.NativeAction,plan.Note)
 }
 if !durable{return 1}
 // An already-issued or claimed notification is an idempotent no-op;
 // a new send that cannot be prepared still needs recovery.
 if decision.Action=="send_to_root" && plan.NativeAction=="" {
  if plan.Decision.Action=="root_notification_already_dispatched" ||
   plan.Decision.Action=="root_handoff_already_claimed" {return 0}
  return 1
 }
 return 0
}

