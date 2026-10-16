package main

import (
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestRuntimeDiagnosisNoEvidenceVsNoMatchingAgent(t *testing.T){
	now:=time.Date(2026,10,16,0,0,0,0,time.UTC)
	for _,tc:=range []struct{enabled,configured bool;events int;want string}{
		{false,true,0,"observation_disabled"},
		{true,false,0,"hooks_unconfigured"},
		{true,true,0,"not_observed"},
		{true,true,4,"no_matching_runtime"},
	}{
		got:=classifyRuntimeDiagnostic(tc.enabled,tc.configured,tc.events,nil,now)
		if got!=tc.want{t.Errorf("%+v got %s",tc,got)}
	}
}

func TestRuntimeDiagnosisNoFabricatedLiveness(t *testing.T){
	now:=time.Date(2026,10,16,0,0,0,0,time.UTC)
	base:=runtimeobs.Attempt{AttemptID:"run-current",Provider:"codex",BindingState:runtimeobs.BindingBound,CurrentState:runtimeobs.StateRunning,
		LastObservedAt:now.Add(-2*time.Minute).Format(time.RFC3339Nano)}
	if got:=classifyRuntimeDiagnostic(true,true,1,[]runtimeobs.Attempt{base},now);got!="observed_nonterminal"{t.Fatal(got)}
	old:=base;old.LastObservedAt=now.Add(-2*time.Hour).Format(time.RFC3339Nano)
	if got:=classifyRuntimeDiagnostic(true,true,1,[]runtimeobs.Attempt{old},now);got!="observation_stale"{t.Fatal(got)}
	terminal:=base;terminal.Terminal=true;terminal.CurrentState=runtimeobs.StateCompleted
	if got:=classifyRuntimeDiagnostic(true,true,1,[]runtimeobs.Attempt{terminal},now);got!="terminal_history"{t.Fatal(got)}
	unbound:=base;unbound.BindingState=runtimeobs.BindingUnbound
	if got:=classifyRuntimeDiagnostic(true,true,1,[]runtimeobs.Attempt{unbound},now);got!="binding_unbound"{t.Fatal(got)}
	ambiguous:=base;ambiguous.BindingState=runtimeobs.BindingAmbiguous
	if got:=classifyRuntimeDiagnostic(true,true,1,[]runtimeobs.Attempt{ambiguous},now);got!="binding_ambiguous"{t.Fatal(got)}
	unknown:=base;unknown.CurrentState=runtimeobs.StateRuntimeUnknown
	if got:=classifyRuntimeDiagnostic(true,true,1,[]runtimeobs.Attempt{unknown},now);got!="runtime_unknown"{t.Fatal(got)}
}
