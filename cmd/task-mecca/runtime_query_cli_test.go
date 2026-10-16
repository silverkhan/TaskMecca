package main

import (
	"bytes"
	"testing"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestRuntimeQueryFiltersBeforeLimitAndSortsNewestFirst(t *testing.T){
	all:=[]runtimeobs.Attempt{
		{AttemptID:"run-old",RuntimeAgentID:"agent-1",StartedAt:"2026-10-01T00:00:00Z",CurrentState:runtimeobs.StateCompleted,Terminal:true},
		{AttemptID:"run-other",RuntimeAgentID:"agent-2",StartedAt:"2026-10-07T00:00:00Z",CurrentState:runtimeobs.StateRunning},
		{AttemptID:"run-new",RuntimeAgentID:"agent-1",StartedAt:"2026-10-06T00:00:00Z",CurrentState:runtimeobs.StateRunning},
		{AttemptID:"run-mid",RuntimeAgentID:"agent-1",StartedAt:"2026-10-04T00:00:00Z",CurrentState:runtimeobs.StateCompleted,Terminal:true},
	}
	out:=filterRuntimeQuery(all,runtimeQueryOptions{AgentID:"agent-1",State:"any",Limit:2})
	if len(out)!=2||out[0].AttemptID!="run-new"||out[1].AttemptID!="run-mid"{t.Fatalf("expected newest 2 of matching agent: %+v",out)}
}

func TestRuntimeQueryRoleAndStateAreEvidenceBased(t *testing.T){
	attempts:=[]runtimeobs.Attempt{
		{AttemptID:"r",AgentPath:"/root",CurrentState:runtimeobs.StateRunning},
		{AttemptID:"c",AgentPath:"/root/controller",CurrentState:runtimeobs.StateRunning},
		{AttemptID:"w",AgentPath:"/root/controller/kkobugi",CurrentState:runtimeobs.StateRunning},
		{AttemptID:"u",AgentPath:"/root/controller/kkobugi",CurrentState:runtimeobs.StateRuntimeUnknown},
		{AttemptID:"b",AgentPath:"/root/controller/kkobugi",CurrentState:runtimeobs.StateRunning,BindingState:runtimeobs.BindingAmbiguous},
		{AttemptID:"t",AgentPath:"/root/controller/kkobugi",CurrentState:runtimeobs.StateCompleted,Terminal:true},
	}
	workers:=filterRuntimeQuery(attempts,runtimeQueryOptions{Role:"worker",State:"any"})
	if len(workers)!=4{t.Fatalf("unexpected worker classification: %+v",workers)}
	terminals:=filterRuntimeQuery(attempts,runtimeQueryOptions{Role:"worker",State:"terminal"})
	if len(terminals)!=1||terminals[0].AttemptID!="t"{t.Fatalf("unexpected terminals: %+v",terminals)}
	unknown:=filterRuntimeQuery(attempts,runtimeQueryOptions{Role:"worker",State:"unknown"})
	if len(unknown)!=2{t.Fatalf("missing unknown and ambiguous: %+v",unknown)}
}

func TestRuntimeQueryFieldsRejectUnknown(t *testing.T){
	fields,err:=validateRuntimeQueryFields("attempt_id,binding_state,current_state,runtime_agent_id,session_id,turn_id")
	if err!=nil||len(fields)!=6{t.Fatalf("fields=%v err=%v",fields,err)}
	if _,err:=validateRuntimeQueryFields("attempt_id,not_a_field");err==nil{t.Fatal("unknown field accepted")}
}

func TestRuntimeQueryHelpIsDiscoverable(t *testing.T){
	var out,stderr bytes.Buffer
	if code:=runRuntimeQueryCLI([]string{"--help"},&out,&stderr);code!=0{t.Fatalf("exit=%d stderr=%s",code,stderr.String())}
	if !bytes.Contains(out.Bytes(),[]byte("--agent-id"))||!bytes.Contains(out.Bytes(),[]byte("--fields")){t.Fatal("help missing agent-oriented filters")}
}
