package activity

import (
 "errors"
 "testing"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)
func TestRegistrarAttribution(t *testing.T){
 e:=validEvent()
 e.Role=RoleRegistrar
 e.AgentID="registrar-native"
 e.RuntimeAttemptID="run-registrar"
 a:=runtimeobs.Attempt{AttemptID:e.RuntimeAttemptID,TaskID:e.TaskID,AgentPath:"/root/registrar",
  RuntimeAgentID:e.AgentID,BindingState:runtimeobs.BindingBound,CurrentState:runtimeobs.StateRunning}
 if err:=ValidateRegistrarAttribution(e,[]runtimeobs.Attempt{a});err!=nil{t.Fatal(err)}
 a.AgentPath="/root/controller/worker-a"
 if err:=ValidateRegistrarAttribution(e,[]runtimeobs.Attempt{a});!errors.Is(err,ErrUnverifiedActor){t.Fatalf("wrong agent path accepted: %v",err)}
 a.AgentPath="/root/registrar";a.Terminal=true
 if err:=ValidateRegistrarAttribution(e,[]runtimeobs.Attempt{a});!errors.Is(err,ErrUnverifiedActor){t.Fatalf("terminal registrar accepted: %v",err)}
}
