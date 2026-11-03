package activity

import (
 "errors"
 "testing"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)
func TestSubControllerExactIdentityAndNoWorkerConfusion(t *testing.T){
 e:=validEvent();e.Role=RoleSubController;e.AgentID="sub-native";e.RuntimeAttemptID="run-sub"
 a:=runtimeobs.Attempt{AttemptID:e.RuntimeAttemptID,TaskID:e.TaskID,
  AgentPath:"/root/controller/sub-reviewer",RuntimeAgentID:e.AgentID,
  BindingState:runtimeobs.BindingBound,CurrentState:runtimeobs.StateRunning}
 if err:=ValidateSubControllerAttribution(e,[]runtimeobs.Attempt{a});err!=nil{t.Fatal(err)}
 a.AgentPath="/root/controller/worker"
 if err:=ValidateSubControllerAttribution(e,[]runtimeobs.Attempt{a});!errors.Is(err,ErrUnverifiedActor){t.Fatalf("worker masquerade accepted: %v",err)}
 a.AgentPath="/root/controller/sub-reviewer";a.Terminal=true
 if err:=ValidateSubControllerAttribution(e,[]runtimeobs.Attempt{a});!errors.Is(err,ErrUnverifiedActor){t.Fatalf("stale sub-controller accepted: %v",err)}
}
