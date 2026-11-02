package handoff

import (
 "errors"
 "fmt"
 "strings"
 "time"

 "github.com/silverkhan/TaskMecca/internal/backlog"
)

// RedispatchResult reports a second verified native delivery receipt.
// It never implies handoff claim or implementation start.
type RedispatchResult struct {
 HandoffID string `json:"handoff_id"`
 TargetAttemptID string `json:"target_attempt_id"`
 Recorded bool `json:"recorded"`
 Duplicate bool `json:"duplicate"`
 Claimed bool `json:"claimed"`
 RedispatchCount int `json:"redispatch_count"`
}

func Redispatch(project, handoffID, targetAttemptID, evidence string, now time.Time) (RedispatchResult, error) {
 handoffID = strings.TrimSpace(handoffID)
 targetAttemptID = strings.TrimSpace(targetAttemptID)
 evidence = strings.TrimSpace(evidence)
 if handoffID == "" || targetAttemptID == "" || evidence == "" {
  return RedispatchResult{}, errors.New("handoff ID, current --target-attempt and native transport --evidence are required")
 }
 var result RedispatchResult
 err := withLock(project,func() error {
  ledger,err:=buildLedgerUnlocked(project,now)
  if err!=nil{return err}
  row,ok:=ledger.Handoffs[handoffID]
  if !ok{return fmt.Errorf("handoff not found: %s",handoffID)}
  if row.Applied{return errors.New("applied handoff cannot be redispatched")}
  if row.Action == ActionHold{return errors.New("handoff on hold: redispatch denied")}
  if row.Steps["dispatch"].Result != "ok" {return errors.New("original dispatch=ok receipt is required before redispatch")}
  if row.Target.AttemptID != targetAttemptID {return errors.New("redispatch target attempt does not match the current handoff target")}
  if row.TaskID == "" || row.ContractSHA256 == "" {return errors.New("canonical contract evidence missing")}
  inspection,err:=backlog.Inspect(project,"",row.TaskID)
  if err!=nil{return err}
  if inspection["exists"]!=true{return errors.New("canonical task missing")}
  raw,_:=inspection["raw_markdown"].(string)
  sha,err:=ContractFingerprint(raw)
  if err!=nil{return err}
  if sha!=row.ContractSHA256{return errors.New("canonical contract changed: redispatch denied")}
  result = RedispatchResult{HandoffID:handoffID,TargetAttemptID:targetAttemptID,Claimed:row.ClaimedBy!="",RedispatchCount:row.RedispatchCount}
  for _,prior:=range row.Redispatches {
   if prior.TargetAttemptID==targetAttemptID && prior.Evidence==evidence {result.Duplicate=true;return nil}
  }
  target,action,_,_,err:=resolveTarget(project,row.Target.AgentPath,row.SourceAttemptID,targetAttemptID,now)
  if err!=nil{return err}
  if action!=ActionMessageRunning || target.AttemptID!=targetAttemptID || !sameVerifiedRuntimeIdentity(row.Target,target) {
   return errors.New("redispatch requires verified running native target with the same runtime identity")
  }
  record:=Record{HandoffID:handoffID,RecordKind:"redispatched",TargetAttemptID:targetAttemptID,ObservedAt:now.UTC().Format(time.RFC3339Nano),Evidence:map[string]string{"detail":evidence}}
  if err:=appendRecordUnlocked(project,record);err!=nil{return err}
  result.Recorded=true
  result.RedispatchCount++
  return nil
 })
 return result,err
}
