// Package contextlife manages read-only rotation decisions and durable,
// explicitly acknowledged session succession. It does NOT spawn native agents
// or grant canonical writer permissions to arbitrary provider sessions.
package contextlife

import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "sort"
 "strings"
 "time"

 "github.com/silverkhan/TaskMecca/internal/backlog"
 "github.com/silverkhan/TaskMecca/internal/handoffjournal"
 "github.com/silverkhan/TaskMecca/internal/projectguard"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type Observation struct {
 AttemptID string `json:"attempt_id"`
 Provider string `json:"provider"`
 UsedTokens int64 `json:"used_tokens,omitempty"`
 WindowTokens int64 `json:"window_tokens,omitempty"`
 UsagePercent *float64 `json:"usage_percent,omitempty"`
 Evidence string `json:"evidence"`
 Trust string `json:"trust"`
 ObservedAt string `json:"observed_at"`
}
type Decision struct {
 Action string `json:"action"`
 Reason string `json:"reason"`
 ContextStatus string `json:"context_status"`
 ContextPercent *float64 `json:"context_percent,omitempty"`
 RequiresNativeAction bool `json:"requires_native_action"`
}
type Checkpoint struct {
 CreatedAt string `json:"created_at"`
 Role string `json:"role"`
 OwnerAttempt string `json:"owner_attempt"`
 ActiveDoing []string `json:"active_doing"`
 PendingHandoffs []string `json:"pending_handoffs"`
 Fingerprint string `json:"fingerprint"`
}
type Rotation struct {
 ID string `json:"id"`
 Stage string `json:"stage"`
 FromAttempt string `json:"from_attempt"`
 ToAttempt string `json:"to_attempt,omitempty"`
 Checkpoint Checkpoint `json:"checkpoint"`
 PreparedAt string `json:"prepared_at"`
 AcknowledgedAt string `json:"acknowledged_at,omitempty"`
 CommittedAt string `json:"committed_at,omitempty"`
}
type State struct {
 Version int `json:"version"`
 Role string `json:"role"`
 Generation int `json:"generation"`
 OwnerAttempt string `json:"owner_attempt,omitempty"`
 Observation *Observation `json:"observation,omitempty"`
 Rotation *Rotation `json:"rotation,omitempty"`
 UpdatedAt string `json:"updated_at,omitempty"`
}
type StatusReport struct {
 State State `json:"state"`
 Decision Decision `json:"decision"`
 Current Checkpoint `json:"current"`
 Eligible bool `json:"eligible"`
 Blockers []string `json:"blockers"`
 Notice string `json:"notice"`
}
func validRole(role string) bool {
 switch role {case "controller","registrar":return true}
 return strings.HasPrefix(role,"worker:") && len(role)>7 && len(role)<96 && validName(role[7:])
}
func validName(v string) bool {
 if v=="" {return false}
 for _,r:=range v {if (r>='a'&&r<='z')||(r>='A'&&r<='Z')||(r>='0'&&r<='9')||r=='-'||r=='_' {continue};return false}
 return true
}
func rolePath(role string) string {
 if role=="controller" {return "/root/controller"}
 if role=="registrar" {return "/root/registrar"}
 return "/root/controller/"+strings.TrimPrefix(role,"worker:")
}
func filePath(project,role string) string {
 return filepath.Join(project,"_task_mecca",".runtime","context_lifecycle","roles",strings.ReplaceAll(role,":","-")+".json")
}
func newState(role string) State {return State{Version:1,Role:role,Generation:1}}
func read(project,role string)(State,error){
 if !validRole(role){return State{},fmt.Errorf("invalid role %q",role)}
 raw,err:=os.ReadFile(filePath(project,role))
 if errors.Is(err,os.ErrNotExist){return newState(role),nil}
 if err!=nil{return State{},err}
 var state State
 if err=json.Unmarshal(raw,&state);err!=nil{return State{},fmt.Errorf("invalid context lifecycle state: %w",err)}
 if state.Version!=1||state.Role!=role||state.Generation<1{return State{},fmt.Errorf("invalid persisted context lifecycle owner state")}
 return state,nil
}
func Read(project,role string)(State,error){return read(project,role)}
func write(project string,s State)error{
 path:=filePath(project,s.Role)
 if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return err}
 raw,err:=json.MarshalIndent(s,"","  ");if err!=nil{return err}
 tmp,err:=os.CreateTemp(filepath.Dir(path),".context-*.tmp");if err!=nil{return err}
 defer os.Remove(tmp.Name())
 if err=tmp.Chmod(0600);err!=nil{tmp.Close();return err}
 if _,err=tmp.Write(append(raw,'\n'));err!=nil{tmp.Close();return err}
 if err=tmp.Sync();err!=nil{tmp.Close();return err}
 if err=tmp.Close();err!=nil{return err}
 // On Windows rename cannot replace a destination. Never remove the
 // current owner in that case: return an error and preserve both records.
 if err=replaceFile(tmp.Name(),path);err!=nil{return fmt.Errorf("atomic owner state replace unavailable: %w",err)}
 return nil
}
func withState(project,role string,fn func(*State)error)(State,error){
 if !validRole(role){return State{},fmt.Errorf("invalid role")}
 unlock,err:=acquire(project)
 if err!=nil{return State{},err}
 defer unlock()
 s,err:=read(project,role);if err!=nil{return State{},err}
 if err=fn(&s);err!=nil{return s,err}
 s.UpdatedAt=time.Now().UTC().Format(time.RFC3339Nano)
 if err=write(project,s);err!=nil{return State{},err}
 if err=appendTransition(project,s);err!=nil{return s,fmt.Errorf("owner state persisted but rotation audit append failed: %w",err)}
 return s,nil
}
// The role state is authoritative. The append-only transition record is an
// audit trail; if append fails the caller must re-read state before retrying.
func appendTransition(project string,s State)error{
 path:=filepath.Join(project,"_task_mecca",".runtime","context_lifecycle","events.jsonl")
 if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return err}
 stage,id:="active",""
 if s.Rotation!=nil {stage=s.Rotation.Stage;id=s.Rotation.ID}
 record:=map[string]any{"role":s.Role,"generation":s.Generation,"owner_attempt":s.OwnerAttempt,"rotation_id":id,"stage":stage,"observed_at":s.UpdatedAt}
 raw,err:=json.Marshal(record);if err!=nil{return err}
 fd,err:=os.OpenFile(path,os.O_CREATE|os.O_WRONLY|os.O_APPEND,0600);if err!=nil{return err}
 if _,err=fd.Write(append(raw,'\n'));err!=nil{fd.Close();return err}
 if err=fd.Sync();err!=nil{fd.Close();return err}
 return fd.Close()
}
func fingerprint(parts ...string)string{
 sum:=sha256.Sum256([]byte(strings.Join(parts,"\x00")))
 return hex.EncodeToString(sum[:])
}
func checkpoint(project,role string,now time.Time)(Checkpoint,error){
 snap:=Checkpoint{CreatedAt:now.UTC().Format(time.RFC3339Nano),Role:role,ActiveDoing:[]string{},PendingHandoffs:[]string{}}
 coord,err:=backlog.Coordinate(project,"",3);if err!=nil{return snap,err}
 if rows,ok:=coord["doing"].([]map[string]any);ok {
  for _,item:=range rows {
   id,_:=item["id"].(string);agent,_:=item["agent"].(string);scope,_:=item["change_scope"].(string)
   if role=="controller" || (strings.HasPrefix(role,"worker:")&&agent==rolePath(role)){
    snap.ActiveDoing=append(snap.ActiveDoing,id+"|"+agent+"|"+scope)
   }
  }
 }
 journal,err:=handoffjournal.BuildLedger(project,now);if err!=nil{return snap,err}
 if len(journal.Findings)>0{return snap,errors.New("handoff journal findings must be resolved before rotation")}
 for id,item:=range journal.Handoffs {
  if item.Applied {continue}
  if (role=="controller"&&item.Target.AgentPath==rolePath(role)) ||
    (role=="registrar"&&item.SourceAgentPath==rolePath(role)) ||
    (strings.HasPrefix(role,"worker:")&&item.SourceAgentPath==rolePath(role)){
   snap.PendingHandoffs=append(snap.PendingHandoffs,id)
  }
 }
 sort.Strings(snap.ActiveDoing);sort.Strings(snap.PendingHandoffs)
 data:=append(append([]string{role},snap.ActiveDoing...),snap.PendingHandoffs...)
 snap.Fingerprint=fingerprint(data...)
 return snap,nil
}
func attemptFor(project,role,id string,now time.Time)(runtimeobs.Attempt,error){
 if strings.TrimSpace(id)==""{return runtimeobs.Attempt{},errors.New("exact runtime attempt required")}
 ledger,err:=runtimeobs.BuildLedger(project,5,now);if err!=nil{return runtimeobs.Attempt{},err}
 for _,a:=range ledger.Attempts {
  if a.AttemptID==id {
   if a.BindingState!=runtimeobs.BindingBound || a.AgentPath!=rolePath(role) || a.Provider=="" || a.SessionID=="" || a.RuntimeAgentID=="" {
    return a,errors.New("attempt missing exact role/provider/session/runtime binding")
   }
   return a,nil
  }
 }
 return runtimeobs.Attempt{},errors.New("exact attempt not found in runtime ledger")
}
func Decide(role string,o *Observation,age time.Duration,event string)Decision {
 d:=Decision{Action:"continue",Reason:"no verified context pressure",ContextStatus:"unknown"}
 if role=="root"{d.Action="manual_only";d.Reason="Root never auto-rotates";return d}
 if o!=nil&&o.WindowTokens>0&&o.UsagePercent!=nil {
  d.ContextPercent=o.UsagePercent
  if o.Trust=="provider_verified"{d.ContextStatus="measured"}else{d.ContextStatus="reported_unverified"}
  // External/manual reports never authorize an automatic generation change.
  if o.Trust=="provider_verified" {
   p:=*o.UsagePercent
   if p>=90 {d.Action="protect";d.Reason="context >=90 percent: stop new dispatch and checkpoint"}
   else if p>=80 {d.Action="rotate";d.Reason="context >=80 percent"}
   else if p>=65 {d.Action="prepare";d.Reason="context >=65 percent"}
  }
 }
 if d.Action=="continue" && age>=7*24*time.Hour {
  d.Action="prepare";d.Reason="seven-day session: inspect rotation at next safe boundary"
 }
 if role=="registrar" && event=="registration_completed" {
  d.Action="prepare";d.Reason="registrar completed one registration; rotate at idle boundary"
 }
 if strings.HasPrefix(role,"worker:") && event=="worker_done" && d.Action=="continue"{
  d.Action="prepare";d.Reason="worker task completed; rotate at idle boundary"
 }
 if d.Action!="continue"{d.RequiresNativeAction=true}
 return d
}
func Status(project,role,event string,now time.Time)(StatusReport,error){
 s,err:=read(project,role);if err!=nil{return StatusReport{},err}
 cp,err:=checkpoint(project,role,now);if err!=nil{return StatusReport{},err}
 cp.OwnerAttempt=s.OwnerAttempt
 age:=time.Duration(0)
 if s.OwnerAttempt!="" {
  a,err:=attemptFor(project,role,s.OwnerAttempt,now);if err==nil {
   if t,e:=time.Parse(time.RFC3339Nano,a.StartedAt);e==nil&&now.After(t) {age=now.Sub(t)}
   if age==0 {if t,e:=time.Parse(time.RFC3339Nano,a.FirstObservedAt);e==nil&&now.After(t){age=now.Sub(t)}}
  }
 }
 if s.Observation!=nil && s.OwnerAttempt!="" && s.Observation.AttemptID!=s.OwnerAttempt {s.Observation=nil}
 decision:=Decide(role,s.Observation,age,event)
 blockers:=[]string{}
 for _,id:=range cp.ActiveDoing {blockers=append(blockers,"active doing: "+id)}
 for _,id:=range cp.PendingHandoffs {blockers=append(blockers,"pending handoff: "+id)}
 if s.Rotation!=nil && s.Rotation.Stage!="committed" {blockers=append(blockers,"rotation already pending: "+s.Rotation.ID)}
 return StatusReport{State:s,Decision:decision,Current:cp,Eligible:len(blockers)==0,Blockers:blockers,
  Notice:"Read-only advisory. Native spawn, live liveness, current preflight and canonical writer fencing are NOT provided by this status."},nil
}
func Observe(project,role,attemptID string,used,window int64,evidence string,now time.Time)(State,error){
 if used<0||window<=0||used>window||strings.TrimSpace(evidence)==""{return State{},errors.New("invalid token report or missing evidence")}
 a,err:=attemptFor(project,role,attemptID,now);if err!=nil{return State{},err}
 pct:=float64(used)*100/float64(window)
 return withState(project,role,func(s *State)error{
  if s.OwnerAttempt!="" && s.OwnerAttempt!=attemptID {return errors.New("reported attempt is not current owner")}
  s.Observation=&Observation{AttemptID:attemptID,Provider:a.Provider,UsedTokens:used,WindowTokens:window,UsagePercent:&pct,Evidence:evidence,Trust:"reported_unverified",ObservedAt:now.UTC().Format(time.RFC3339Nano)}
  return nil
 })
}
func Prepare(project,role,attemptID string,now time.Time)(State,error){
 a,err:=attemptFor(project,role,attemptID,now);if err!=nil{return State{},err}
 if a.CurrentState==runtimeobs.StateRuntimeUnknown {return State{},errors.New("unknown old runtime state; no rotation without evidence")}
 return withState(project,role,func(s *State)error{
  if s.OwnerAttempt!=""&&s.OwnerAttempt!=attemptID{return errors.New("owner attempt mismatch")}
  if s.Rotation!=nil && s.Rotation.Stage!="committed" {
   if s.Rotation.FromAttempt==attemptID && s.Rotation.Stage=="prepared"{return nil}
   return errors.New("rotation already exists; explicit recover/review required")
  }
  cp,err:=checkpoint(project,role,now);if err!=nil{return err}
  if len(cp.ActiveDoing)>0||len(cp.PendingHandoffs)>0{return errors.New("rotation deferred: active work or pending handoff")}
  cp.OwnerAttempt=attemptID
  s.OwnerAttempt=attemptID
  s.Rotation=&Rotation{ID:"rotation-"+fingerprint(role,attemptID,cp.Fingerprint)[:20],Stage:"prepared",FromAttempt:attemptID,Checkpoint:cp,PreparedAt:now.UTC().Format(time.RFC3339Nano)}
  return nil
 })
}
func Acknowledge(project,role,rotationID,targetAttempt,checkpointSHA string,now time.Time)(State,error){
 successor,err:=attemptFor(project,role,targetAttempt,now);if err!=nil{return State{},err}
 if successor.Terminal || (successor.CurrentState!=runtimeobs.StateRunning&&successor.CurrentState!=runtimeobs.StateStarting){return State{},errors.New("successor must be observed starting or running")}
 return withState(project,role,func(s *State)error{
  r:=s.Rotation
  if r==nil||r.ID!=rotationID||r.Stage!="prepared"{return errors.New("rotation not prepared")}
  if targetAttempt==r.FromAttempt{return errors.New("successor must be a new exact runtime attempt")}
  if checkpointSHA!=r.Checkpoint.Fingerprint {return errors.New("checkpoint fingerprint mismatch")}
  owner,err:=attemptFor(project,role,r.FromAttempt,now);if err!=nil{return err}
  if owner.Provider!=successor.Provider{return errors.New("cross-provider rotation requires a separate verified adapter")}
  if owner.SessionID==successor.SessionID&&owner.RuntimeAgentID==successor.RuntimeAgentID{return errors.New("same native session cannot be a fresh successor")}
  r.ToAttempt=targetAttempt;r.Stage="acknowledged";r.AcknowledgedAt=now.UTC().Format(time.RFC3339Nano)
  return nil
 })
}
func Commit(project,role,rotationID string,now time.Time)(State,error){
 return withState(project,role,func(s *State)error{
  r:=s.Rotation
  if r==nil||r.ID!=rotationID{return errors.New("rotation does not exist")}
  if r.Stage=="committed"{return nil}
  if r.Stage!="acknowledged"{return errors.New("successor checkpoint ACK required")}
  if s.OwnerAttempt!=r.FromAttempt{return errors.New("owner generation changed")}
  owner,err:=attemptFor(project,role,r.FromAttempt,now);if err!=nil{return err}
  if !owner.Terminal{return errors.New("previous owner not terminal; do not create dual writers")}
  next,err:=attemptFor(project,role,r.ToAttempt,now);if err!=nil{return err}
  if next.Terminal || next.CurrentState!=runtimeobs.StateRunning{return errors.New("new owner not confirmed running")}
  cp,err:=checkpoint(project,role,now);if err!=nil{return err}
  if cp.Fingerprint!=r.Checkpoint.Fingerprint||len(cp.ActiveDoing)>0||len(cp.PendingHandoffs)>0{return errors.New("canonical/handoff changed or work active; rotation cannot commit")}
  s.Generation++;s.OwnerAttempt=r.ToAttempt;s.Observation=nil;r.Stage="committed";r.CommittedAt=now.UTC().Format(time.RFC3339Nano)
  return nil
 })
}
func Recover(project,role string,now time.Time)(StatusReport,error){
 // Recovery is intentionally read-only: if the old runtime is gone and the
 // new agent is missing, only native discovery can determine the next action.
 return Status(project,role,"controller_resume",now)
}
// AssertOwner is a reusable generation fence for future canonical CLI writers.
// It is intentionally not advertised as universal protection: direct edits
// and existing unintegrated writers do not consult this fence yet.
func AssertOwner(project,role,attempt string,generation int)error{
 s,err:=read(project,role);if err!=nil{return err}
 if s.OwnerAttempt==""{return errors.New("owner not initialized")}
 if s.OwnerAttempt!=attempt||s.Generation!=generation{return fmt.Errorf("stale controller generation or attempt")}
 return nil
}
// MayClaimController gates exact Controller handoff claims after CLM has an
// explicit owner. Legacy projects without a CLM owner keep their old behavior.
func MayClaimController(project,attempt string)error{
 s,err:=read(project,"controller");if err!=nil{return err}
 if s.OwnerAttempt!=""&&s.OwnerAttempt!=attempt{return fmt.Errorf("controller handoff claim rejected: stale rotation owner attempt")}
 return nil
}
func EnsureAllowed(project string)error {
 if !projectguard.Allowed(project){return projectguard.ErrSuppressed}
 return nil
}
