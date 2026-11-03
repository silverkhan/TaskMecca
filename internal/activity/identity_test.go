package activity

import (
 "errors"
 "testing"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)
func TestValidateAttribution(t *testing.T){
 e:=validEvent();e.AgentID="native-agent";e.RuntimeAttemptID="run-1";e.AssignmentID="assignment-1"
 a:=runtimeobs.Attempt{AttemptID:"run-1",RuntimeAgentID:"native-agent",TaskID:"B-589",AgentPath:"/root/controller/worker-a",BindingState:runtimeobs.BindingBound,CurrentState:runtimeobs.StateRunning,BindingEvidence:map[string]string{"assignment_id":"assignment-1"}}
 assignment:=runtimeobs.Assignment{AssignmentID:"assignment-1",TaskID:"B-589",AgentPath:"/root/controller/worker-a"}
 if err:=ValidateAttribution(e,[]runtimeobs.Attempt{a},[]runtimeobs.Assignment{assignment});err!=nil{t.Fatal(err)}
 cases:=map[string]func(*Event,*runtimeobs.Attempt,*runtimeobs.Assignment){
  "stale attempt":func(e *Event,a *runtimeobs.Attempt,_ *runtimeobs.Assignment){a.Terminal=true},
  "mismatch task":func(e *Event,a *runtimeobs.Attempt,_ *runtimeobs.Assignment){a.TaskID="B-590"},
  "spoofed agent":func(e *Event,a *runtimeobs.Attempt,_ *runtimeobs.Assignment){e.AgentID="other"},
  "unbound":func(e *Event,a *runtimeobs.Attempt,_ *runtimeobs.Assignment){a.BindingState=runtimeobs.BindingUnbound},
  "wrong assignment":func(e *Event,a *runtimeobs.Attempt,_ *runtimeobs.Assignment){a.BindingEvidence["assignment_id"]="assignment-old"},
  "spoofed role":func(e *Event,a *runtimeobs.Attempt,_ *runtimeobs.Assignment){e.Role=RoleController},
  "wrong actor path":func(e *Event,a *runtimeobs.Attempt,b *runtimeobs.Assignment){b.AgentPath="/root/controller/other"},
 }
 for name,mutate:=range cases{t.Run(name,func(t *testing.T){
  v:=e;at:=a;at.BindingEvidence=map[string]string{"assignment_id":"assignment-1"};as:=assignment
  mutate(&v,&at,&as)
  err:=ValidateAttribution(v,[]runtimeobs.Attempt{at},[]runtimeobs.Assignment{as})
  if !errors.Is(err,ErrUnverifiedActor){t.Fatalf("expected rejection, got %v",err)}
 })}
}
