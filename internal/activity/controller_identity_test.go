package activity

import (
  "errors"
  "testing"
  "github.com/silverkhan/TaskMecca/internal/handoffjournal"
  "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)
func TestControllerActivityRequiresExactClaim(t *testing.T){
  e:=validEvent();e.Role=RoleController;e.Type=ControllerReview;e.AgentID="controller-agent";e.RuntimeAttemptID="controller-run";e.HandoffID="handoff-one"
  a:=runtimeobs.Attempt{AttemptID:e.RuntimeAttemptID,RuntimeAgentID:e.AgentID,AgentPath:"/root/controller",BindingState:runtimeobs.BindingBound,CurrentState:runtimeobs.StateRunning}
  claim:=handoffjournal.Handoff{HandoffID:e.HandoffID,TaskID:e.TaskID,ClaimedBy:"/root/controller",ClaimedAttemptID:e.RuntimeAttemptID,
    Target:handoffjournal.Target{AgentPath:"/root/controller",AttemptID:e.RuntimeAttemptID,RuntimeAgentID:e.AgentID}}
  assert:=func(evt Event,attempt runtimeobs.Attempt,handoff handoffjournal.Handoff,want bool){
    t.Helper();err:=ValidateControllerAttribution(evt,[]runtimeobs.Attempt{attempt},map[string]handoffjournal.Handoff{evt.HandoffID:handoff})
    if want&&err!=nil{t.Fatalf("expected verified claim: %v",err)}
    if !want&&!errors.Is(err,ErrUnverifiedActor){t.Fatalf("expected rejection: %v",err)}
  }
  assert(e,a,claim,true)
  old:=claim;old.ClaimedAttemptID="old-controller-run";assert(e,a,old,false)
  wrong:=e;wrong.TaskID="B-590";assert(wrong,a,claim,false)
  ended:=a;ended.Terminal=true;assert(e,ended,claim,false)
  done:=claim;done.Applied=true;assert(e,a,done,false)
}
