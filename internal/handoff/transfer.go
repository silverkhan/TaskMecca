package handoff

import (
 "errors"
 "fmt"
 "strings"
 "time"

 "github.com/silverkhan/TaskMecca/internal/backlog"
 "github.com/silverkhan/TaskMecca/internal/contextlife"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// TransferClaim changes a claimed, unapplied Controller handoff's exact
// attempt owner after provider-observed retirement of the former owner.
// It preserves the original Handoff ID, prepared contract, approvals, prior
// claim and all step evidence in an append-only journal transition.
type TransferClaimResult struct {
 HandoffID string `json:"handoff_id"`
 Transferred bool `json:"transferred"`
 Duplicate bool `json:"duplicate"`
 FromAttempt string `json:"from_attempt"`
 ToAttempt string `json:"to_attempt"`
 TransferCount int `json:"transfer_count"`
 ContractSHA256 string `json:"contract_sha256"`
}

func TransferClaim(project,handoffID,fromAttempt,toAttempt,evidence string,now time.Time)(TransferClaimResult,error) {
 handoffID=strings.TrimSpace(handoffID)
 fromAttempt=strings.TrimSpace(fromAttempt)
 toAttempt=strings.TrimSpace(toAttempt)
 evidence=strings.TrimSpace(evidence)
 if handoffID==""||fromAttempt==""||toAttempt==""||evidence=="" {
  return TransferClaimResult{},errors.New("handoff, --from-attempt, --to-attempt and --evidence are required")
 }
 if fromAttempt==toAttempt {return TransferClaimResult{},errors.New("new Controller attempt must differ from prior claimant")}
 var result TransferClaimResult
 err:=withLock(project,func()error{
  handoffs,err:=buildLedgerUnlocked(project,now)
  if err!=nil{return err}
  row,ok:=handoffs.Handoffs[handoffID]
  if !ok{return fmt.Errorf("handoff not found: %s",handoffID)}
  if row.TransferCount>0 && row.LastTransferFrom==fromAttempt && row.ClaimedAttemptID==toAttempt {
   result=TransferClaimResult{HandoffID:handoffID,Duplicate:true,FromAttempt:fromAttempt,ToAttempt:toAttempt,TransferCount:row.TransferCount,ContractSHA256:row.ContractSHA256}
   return nil
  }
  if row.Applied{return errors.New("applied handoff cannot be transferred")}
  if row.Target.AgentPath!="/root/controller"||row.ClaimedBy!="/root/controller"||row.ClaimedAttemptID!=fromAttempt ||
    (row.Target.AttemptID!=""&&row.Target.AttemptID!=fromAttempt) {
   return errors.New("transfer requires the exact previously claimed Controller attempt and target")
  }
  if row.TaskID==""||row.ContractSHA256=="" {
   return errors.New("handoff lacks immutable task/contract evidence; manual review required")
  }
  inspection,err:=backlog.Inspect(project,"",row.TaskID)
  if err!=nil{return err}
  if inspection["exists"]!=true{return errors.New("canonical task missing")}
  raw,_:=inspection["raw_markdown"].(string)
  digest,err:=ContractFingerprint(raw)
  if err!=nil{return err}
  if digest!=row.ContractSHA256{return errors.New("canonical contract changed: transfer denied")}
  ledger,err:=runtimeobs.BuildLedger(project,20,now)
  if err!=nil{return err}
  var oldAgent,newAgent *runtimeobs.Attempt
  currentControllers:=0
  for i:=range ledger.Attempts {
   a:=&ledger.Attempts[i]
   if a.AttemptID==fromAttempt{oldAgent=a}
   if a.AttemptID==toAttempt{newAgent=a}
   if a.AgentPath=="/root/controller" && a.BindingState==runtimeobs.BindingBound && !a.Terminal &&
    a.CurrentState==runtimeobs.StateRunning {currentControllers++}
  }
  if oldAgent==nil||newAgent==nil{return errors.New("both exact Controller runtime attempts must exist")}
  if oldAgent.AgentPath!="/root/controller"||newAgent.AgentPath!="/root/controller"||
    oldAgent.BindingState!=runtimeobs.BindingBound||newAgent.BindingState!=runtimeobs.BindingBound {
   return errors.New("Controller attempts require exact bound /root/controller identity")
  }
  if !oldAgent.Terminal||oldAgent.StateObservationQuality!=runtimeobs.QualityObserved||oldAgent.EndedAt=="" {
   return errors.New("former Controller is not confirmed terminal by observed provider state")
  }
  if newAgent.Terminal||newAgent.CurrentState!=runtimeobs.StateRunning||newAgent.StateObservationQuality!=runtimeobs.QualityObserved {
   return errors.New("successor Controller is not confirmed running by observed provider state")
  }
  if currentControllers!=1 {return fmt.Errorf("expected exactly one running bound Controller; found %d",currentControllers)}
  if oldAgent.Provider==""||oldAgent.Provider!=newAgent.Provider||newAgent.SessionID==""||
    newAgent.RuntimeAgentID==""||oldAgent.SessionID==""||oldAgent.RuntimeAgentID=="" {
   return errors.New("provider/session/runtime identity missing or cross-provider transition unsupported")
  }
  // A separately committed CLM generation may already fence the old owner.
  // Pending CLM generations must not be bypassed by a generic claim; only
  // this evidence-checked, exact-hand-off transfer is authorized.
  if ownerErr:=contextlife.MayClaimController(project,toAttempt); ownerErr!=nil {
   current,readErr:=contextlife.Read(project,"controller")
   if readErr!=nil{return readErr}
   if current.OwnerAttempt!=fromAttempt {return fmt.Errorf("current CLM owner is neither predecessor nor successor: %w",ownerErr)}
  }
  target,action,fresh,_,err:=resolveTarget(project,"/root/controller",row.SourceAttemptID,toAttempt,now)
  if err!=nil{return err}
  if target.AttemptID!=toAttempt||action!=ActionMessageRunning {
   return errors.New("successor is not currently deliverable by verified native identity/capability")
  }
  record:=Record{
   HandoffID:handoffID,RecordKind:"claim_transferred",
   PreviousClaimedAttemptID:fromAttempt,ClaimedBy:"/root/controller",ClaimedAttemptID:toAttempt,
   TargetAgentPath:target.AgentPath,TargetAttemptID:target.AttemptID,
   TargetRuntimeAgentID:target.RuntimeAgentID,TargetRuntimeName:target.RuntimeName,
   TargetSessionID:target.SessionID,Provider:target.Provider,TargetState:target.State,
   Action:action,RequiresFreshPreflight:fresh,ObservedAt:now.UTC().Format(time.RFC3339Nano),
   Evidence:map[string]string{"reason":evidence,"from_attempt":fromAttempt,"to_attempt":toAttempt,"verified_contract_sha256":digest},
  }
  if err:=appendRecordUnlocked(project,record);err!=nil{return err}
  result=TransferClaimResult{HandoffID:handoffID,Transferred:true,FromAttempt:fromAttempt,ToAttempt:toAttempt,TransferCount:row.TransferCount+1,ContractSHA256:row.ContractSHA256}
  return nil
 })
 return result,err
}
