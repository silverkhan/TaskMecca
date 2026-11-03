package activity

import (
 "fmt"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// ValidateRegistrarAttribution checks the exact bound Registrar attempt.
// Registrar prepares the registration BEFORE a worker assignment and often
// before a registration_ready handoff exists, so neither is required here.
func ValidateRegistrarAttribution(e Event,attempts []runtimeobs.Attempt)error{
 reject:=func(reason string)error{return fmt.Errorf("%w: %s",ErrUnverifiedActor,reason)}
 if e.Role!=RoleRegistrar{return reject("registrar role required")}
 if e.AssignmentID!=""||e.HandoffID!=""{return reject("registrar cannot use worker/controller ownership proof")}
 var found *runtimeobs.Attempt
 for i:=range attempts{
  a:=&attempts[i]
  if a.AttemptID!=e.RuntimeAttemptID{continue}
  if found!=nil{return reject("ambiguous registrar attempt")}
  found=a
 }
 if found==nil{return reject("no registrar attempt")}
 if found.AgentPath!="/root/registrar"||found.RuntimeAgentID!=e.AgentID{return reject("registrar agent mismatch")}
 if found.BindingState!=runtimeobs.BindingBound||found.TaskID!=e.TaskID{return reject("registrar task binding missing")}
 if found.Terminal || (found.CurrentState!=runtimeobs.StateRunning&&found.CurrentState!=runtimeobs.StateStarting){
  return reject("registrar attempt is not observed active")
 }
 if e.Provider!=""&&found.Provider!=""&&e.Provider!=found.Provider{return reject("provider mismatch")}
 if e.SessionID!=""&&found.SessionID!=""&&e.SessionID!=found.SessionID{return reject("session mismatch")}
 return nil
}
