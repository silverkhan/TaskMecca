package handoff

import (
 "fmt"
 "strings"
 "time"

 "github.com/silverkhan/TaskMecca/internal/backlog"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// RecoveryPlan is an advisory, read-only Decision Engine projection. Native
// resume/delivery and their evidence remain the responsibility of the caller.
type RecoveryPlan struct {
 HandoffID string `json:"handoff_id"`
 TaskID string `json:"task_id"`
 ClaimStatus string `json:"claim_status"`
 DeliveryStatus string `json:"delivery_status"`
 NextAction string `json:"next_action"`
 TargetAttemptID string `json:"target_attempt_id,omitempty"`
 CandidateAttemptID string `json:"candidate_attempt_id,omitempty"`
 Verified bool `json:"verified"`
 RequiresUserDecision bool `json:"requires_user_decision"`
 Reason string `json:"reason"`
}

func PlanRecovery(project, handoffID string, now time.Time) (RecoveryPlan,error) {
 row,ok,err:=Inspect(project,strings.TrimSpace(handoffID),now)
 if err!=nil{return RecoveryPlan{},err}
 if !ok{return RecoveryPlan{},fmt.Errorf("handoff not found: %s",handoffID)}
 result:=RecoveryPlan{HandoffID:row.HandoffID,TaskID:row.TaskID,ClaimStatus:row.ClaimStatus,DeliveryStatus:row.DeliveryStatus,TargetAttemptID:row.Target.AttemptID,NextAction:"hold",Reason:"insufficient_verified_runtime_evidence"}
 if row.Applied {result.NextAction="none";result.Reason="already_applied";return result,nil}
 if row.Action==ActionHold {result.Reason="handoff_on_hold";return result,nil}
 if row.TaskID==""||row.ContractSHA256=="" {result.Reason="missing_contract_evidence";return result,nil}
 inspection,err:=backlog.Inspect(project,"",row.TaskID)
 if err!=nil{return RecoveryPlan{},err}
 if inspection["exists"]!=true {result.Reason="task_missing";return result,nil}
 raw,_:=inspection["raw_markdown"].(string)
 sha,err:=ContractFingerprint(raw)
 if err!=nil{return RecoveryPlan{},err}
 if sha!=row.ContractSHA256 {result.Reason="contract_changed";result.RequiresUserDecision=true;return result,nil}
 if row.Target.AgentPath!="/root/controller" {result.Reason="unsupported_target_role";return result,nil}
 runtimeLedger,err:=runtimeobs.BuildLedger(project,20,now)
 if err!=nil{return RecoveryPlan{},err}
 candidates:=[]string{}
 for _,a:=range runtimeLedger.Attempts {
  if a.AttemptID==""||a.AttemptID==row.Target.AttemptID||a.AgentPath!="/root/controller" || a.BindingState!=runtimeobs.BindingBound ||
   a.Terminal||a.CurrentState!=runtimeobs.StateRunning||a.StateObservationQuality!=runtimeobs.QualityObserved {continue}
  candidate:=Target{AgentPath:a.AgentPath,Provider:a.Provider,SessionID:a.SessionID,RuntimeAgentID:a.RuntimeAgentID}
  if sameVerifiedRuntimeIdentity(row.Target,candidate) { candidates=append(candidates,a.AttemptID) }
 }
 if len(candidates)!=1 {
  if len(candidates)>1 {result.Reason="ambiguous_running_controller_attempts"} else {result.Reason="no_verified_running_successor"}
  return result,nil
 }
 id:=candidates[0]
 target,action,_,_,err:=resolveTarget(project,"/root/controller",row.SourceAttemptID,id,now)
 if err!=nil||action!=ActionMessageRunning||target.AttemptID!=id||!sameVerifiedRuntimeIdentity(row.Target,target) {
  result.Reason="successor_not_deliverable"
  return result,nil
 }
 if row.ClaimedBy!="" {
  if row.ClaimedBy!="/root/controller"||row.ClaimedAttemptID=="" {
   result.Reason="unexpected_claim_owner";return result,nil
  }
  result.NextAction="transfer_claim"
  result.Reason="claimed_owner_requires_exact_fenced_transfer"
 } else {
  result.NextAction="prepare_retarget"
  result.Reason="unclaimed_verified_successor"
 }
 result.CandidateAttemptID=id
 result.Verified=true
 return result,nil
}

