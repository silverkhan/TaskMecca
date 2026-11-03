package activity

import (
    "errors"
    "fmt"
    "strings"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

var ErrUnverifiedActor = errors.New("activity actor identity is not verified")

// ValidateAttribution cross-checks a claimed activity against the canonical
// runtime and dispatch records. This is attribution verification, NOT caller
// authentication: CLI flags alone cannot prove which OS process invoked it.
// A write API must also enforce a trusted caller boundary before publishing.
func ValidateAttribution(e Event, attempts []runtimeobs.Attempt, assignments []runtimeobs.Assignment) error {
    deny:=func(reason string)error{return fmt.Errorf("%w: %s",ErrUnverifiedActor,reason)}
    if strings.TrimSpace(e.AssignmentID)==""{return deny("missing assignment")}
    matches:=0
    for _,a:=range assignments {
        if a.AssignmentID!=e.AssignmentID {continue}
        matches++
        if a.TaskID!=e.TaskID{return deny("assignment task mismatch")}
    }
    if matches!=1{return deny("assignment is missing or ambiguous")}
    var found *runtimeobs.Attempt
    for i:=range attempts{
        a:=&attempts[i]
        if a.AttemptID!=e.RuntimeAttemptID{continue}
        if found!=nil{return deny("duplicate runtime attempt")}
        found=a
    }
    if found==nil{return deny("runtime attempt missing")}
    if found.BindingState!=runtimeobs.BindingBound{return deny("attempt is not bound")}
    if found.TaskID!=e.TaskID{return deny("runtime task mismatch")}
    if found.RuntimeAgentID!=e.AgentID{return deny("runtime agent mismatch")}
    if found.BindingEvidence["assignment_id"]!=e.AssignmentID{return deny("runtime assignment mismatch")}
    if found.Terminal{return deny("terminal attempt cannot start new activity")}
    if found.CurrentState!=runtimeobs.StateRunning && found.CurrentState!=runtimeobs.StateStarting {
        return deny("runtime execution is not observed as active")
    }
    if found.Provider!="" && e.Provider!="" && found.Provider!=e.Provider{return deny("provider mismatch")}
    if found.SessionID!="" && e.SessionID!="" && found.SessionID!=e.SessionID{return deny("session mismatch")}
    expectedRole:=""
    switch {
    case found.AgentPath=="/root/registrar":expectedRole=string(RoleRegistrar)
    case found.AgentPath=="/root/controller":expectedRole=string(RoleController)
    case strings.HasPrefix(found.AgentPath,"/root/controller/sub-"):expectedRole=string(RoleSubController)
    case strings.HasPrefix(found.AgentPath,"/root/controller/"):expectedRole=string(RoleWorker)
    }
    if expectedRole==""||string(e.Role)!=expectedRole{return deny("role does not match bound agent path")}
    for _,a:=range assignments {
        if a.AssignmentID==e.AssignmentID && a.AgentPath!=found.AgentPath{return deny("assignment agent mismatch")}
    }
    return nil
}
