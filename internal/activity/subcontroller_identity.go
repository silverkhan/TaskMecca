package activity

import (
 "fmt"
 "strings"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// ValidateSubControllerAttribution verifies that an observed attempt really
// belongs to the reserved /root/controller/sub-* namespace. This does NOT
// grant delegation/approval or canonical backlog write authority: the phase
// remains an agent-reported observation and Main retains final control.
func ValidateSubControllerAttribution(e Event,attempts []runtimeobs.Attempt)error{
 reject:=func(reason string)error{return fmt.Errorf("%w: %s",ErrUnverifiedActor,reason)}
 if e.Role!=RoleSubController{return reject("sub_controller role required")}
 if e.AssignmentID!=""||e.HandoffID!=""{return reject("sub-controller cannot claim worker/controller ownership proof")}
 var found *runtimeobs.Attempt
 for i:=range attempts{
  a:=&attempts[i]
  if a.AttemptID!=e.RuntimeAttemptID{continue}
  if found!=nil{return reject("ambiguous sub-controller attempt")}
  found=a
 }
 if found==nil{return reject("sub-controller attempt missing")}
 if !strings.HasPrefix(found.AgentPath,"/root/controller/sub-") ||
   len(found.AgentPath)<=len("/root/controller/sub-") ||
   strings.Contains(found.AgentPath[len("/root/controller/sub-"):],"/"){
  return reject("attempt is not in reserved sub-controller path")
 }
 if found.RuntimeAgentID!=e.AgentID||found.TaskID!=e.TaskID||
  found.BindingState!=runtimeobs.BindingBound{return reject("sub-controller identity/task binding mismatch")}
 if found.Terminal || (found.CurrentState!=runtimeobs.StateRunning&&found.CurrentState!=runtimeobs.StateStarting){
  return reject("sub-controller attempt not observed active")
 }
 if e.Provider!=""&&found.Provider!=""&&e.Provider!=found.Provider{return reject("provider mismatch")}
 if e.SessionID!=""&&found.SessionID!=""&&e.SessionID!=found.SessionID{return reject("session mismatch")}
 return nil
}
