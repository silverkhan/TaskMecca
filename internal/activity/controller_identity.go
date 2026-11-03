package activity

import (
    "fmt"

    "github.com/silverkhan/TaskMecca/internal/handoffjournal"
    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// ValidateControllerAttribution checks a live, exact Controller claim for the
// requested backlog. A role label alone never authorizes a record.
// This validates attribution, not OS-level caller authentication.
func ValidateControllerAttribution(e Event, attempts []runtimeobs.Attempt, handoffs map[string]handoffjournal.Handoff) error {
    reject:=func(msg string)error{return fmt.Errorf("%w: %s",ErrUnverifiedActor,msg)}
    if e.Role!=RoleController{return reject("controller role required")}
    if e.HandoffID==""{return reject("handoff ID required")}
    handoff,ok:=handoffs[e.HandoffID]
    if !ok{return reject("handoff missing")}
    if handoff.TaskID!=e.TaskID{return reject("handoff task mismatch")}
    if handoff.ClaimedBy!="/root/controller"||handoff.ClaimedAttemptID!=e.RuntimeAttemptID{
        return reject("handoff claim not owned by this Controller attempt")
    }
    if handoff.Applied && e.Action!=Stop{return reject("handoff already finalized")}
    var actual *runtimeobs.Attempt
    for i:=range attempts{
        attempt:=&attempts[i]
        if attempt.AttemptID!=e.RuntimeAttemptID{continue}
        if actual!=nil{return reject("ambiguous Controller attempt")}
        actual=attempt
    }
    if actual==nil{return reject("Controller runtime attempt missing")}
    if actual.AgentPath!="/root/controller"||actual.RuntimeAgentID!=e.AgentID{return reject("Controller identity mismatch")}
    if actual.BindingState!=runtimeobs.BindingBound{return reject("Controller role binding missing")}
    if actual.Terminal{return reject("Controller attempt terminal")}
    if actual.CurrentState!=runtimeobs.StateRunning&&actual.CurrentState!=runtimeobs.StateStarting{return reject("Controller not observed running")}
    if handoff.Target.AgentPath!="/root/controller" || (handoff.Target.AttemptID!=""&&handoff.Target.AttemptID!=e.RuntimeAttemptID){
        return reject("handoff recipient mismatch")
    }
    if handoff.Target.RuntimeAgentID!=""&&handoff.Target.RuntimeAgentID!=e.AgentID{return reject("handoff runtime agent mismatch")}
    if actual.TaskID!=""&&actual.TaskID!=e.TaskID{return reject("runtime task mismatch")}
    return nil
}
