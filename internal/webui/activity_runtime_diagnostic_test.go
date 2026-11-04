package webui

import (
    "testing"

    "github.com/silverkhan/TaskMecca/internal/activity"
    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)
func TestActivityRuntimeDiagnosticMissingWorkerIsUnknownExecution(t *testing.T){
    attempts:=[]runtimeobs.Attempt{{AttemptID:"attempt-1",TaskID:"B-602",RuntimeAgentID:"worker-1",AgentType:"worker",BindingState:runtimeobs.BindingBound}}
    got:=summarizeActivityRuntimeEvidence("B-602",attempts,nil)
    if got.WithoutActivity!=1||got.MatchedAttempts!=0||got.Attempts[0].Diagnostic!="no_matching_activity"{t.Fatalf("unexpected diagnostic %+v",got)}
}
func TestActivityRuntimeDiagnosticExactAttemptAgentAndAmbiguity(t *testing.T){
    attempts:=[]runtimeobs.Attempt{
        {AttemptID:"a",TaskID:"B-602",RuntimeAgentID:"w",BindingState:runtimeobs.BindingBound},
        {AttemptID:"b",TaskID:"B-602",RuntimeAgentID:"w2",BindingState:runtimeobs.BindingAmbiguous},
        {AttemptID:"c",TaskID:"B-602",RuntimeAgentID:"w3",BindingState:runtimeobs.BindingBound},
        {AttemptID:"x",TaskID:"B-OTHER",RuntimeAgentID:"w4",BindingState:runtimeobs.BindingBound},
    }
    events:=[]activity.Event{
        {EventID:"e1",TaskID:"B-602",RuntimeAttemptID:"a",AgentID:"w"},
        {EventID:"e2",TaskID:"B-602",RuntimeAttemptID:"c",AgentID:"incorrect"},
        {EventID:"e3",TaskID:"B-602",RuntimeAttemptID:"absent",AgentID:"w"},
    }
    got:=summarizeActivityRuntimeEvidence("B-602",attempts,events)
    if got.MatchedAttempts!=1||got.IdentityUnknown!=1||got.IdentityMismatch!=1||got.ActivityWithoutRuntime!=2 {
        t.Fatalf("unexpected reconciliation %+v",got)
    }
    if len(got.Attempts)!=3 || got.Attempts[0].Diagnostic!="activity_recorded" || got.Attempts[1].Diagnostic!="identity_unverified" || got.Attempts[2].Diagnostic!="identity_mismatch" {
        t.Fatalf("unexpected attempt rows %+v",got.Attempts)
    }
}
func TestActivityRuntimeDiagnosticNeverInfersUnboundTask(t *testing.T){
    got:=summarizeActivityRuntimeEvidence("B-602",[]runtimeobs.Attempt{
        {AttemptID:"unbound",RuntimeAgentID:"worker",BindingState:runtimeobs.BindingUnbound},
    },nil)
    if len(got.Attempts)!=0||got.WithoutActivity!=0 {t.Fatalf("unbound task falsely attributed: %+v",got)}
}
