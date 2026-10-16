package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type runtimeQueryOptions struct {
	Provider string
	AgentID string
	AgentPath string
	Role string
	SessionID string
	TaskID string
	State string
	Fields []string
	Limit int
}

type runtimeQueryResponse struct {
	OK bool `json:"ok"`
	Status string `json:"status"`
	GeneratedAt string `json:"generated_at"`
	Count int `json:"count"`
	TotalMatches int `json:"total_matches"`
	Attempts []map[string]any `json:"attempts"`
	Findings []runtimeobs.LedgerFinding `json:"findings"`
	ObservationScope string `json:"observation_scope"`
}

func runtimeQueryUsage(out io.Writer) {
	fmt.Fprintln(out,"Usage: task-mecca runtime query [--project DIR] [--agent-id UUID] [--agent-path /root/controller]")
	fmt.Fprintln(out,"       [--role root|registrar|controller|worker] [--session-id ID] [--task-id ID]")
	fmt.Fprintln(out,"       [--provider codex|claude] [--state any|open|terminal|unknown]")
	fmt.Fprintln(out,"       [--limit N] [--fields field1,field2] [--json]")
	fmt.Fprintln(out,"Read-only query, filtered first and sorted newest-start-first. Open != proven live.")
	fmt.Fprintln(out,"Example: task-mecca runtime query --agent-id UUID --limit 2 --fields attempt_id,binding_state,current_state,runtime_agent_id,session_id,turn_id --json")
}

func runRuntimeQueryCLI(args []string,out,stderr io.Writer) int {
	flags:=flag.NewFlagSet("runtime query",flag.ContinueOnError)
	flags.SetOutput(stderr)
	project:=flags.String("project",".","Task Mecca project root")
	agentID:=flags.String("agent-id","","exact runtime_agent_id")
	agentPath:=flags.String("agent-path","","exact /root agent path")
	role:=flags.String("role","","root, registrar, controller, worker")
	sessionID:=flags.String("session-id","","exact session ID")
	taskID:=flags.String("task-id","","exact backlog ID")
	provider:=flags.String("provider","","codex or claude")
	state:=flags.String("state","any","any, open, terminal, unknown")
	limit:=flags.Int("limit",10,"maximum returned matches")
	fieldText:=flags.String("fields","","comma-separated JSON field names; default all")
	asJSON:=flags.Bool("json",false,"machine-readable JSON output")
	help:=flags.Bool("help",false,"show usage")
	if err:=flags.Parse(args);err!=nil{return 2}
	if *help {runtimeQueryUsage(out);return 0}
	if flags.NArg()!=0 {fmt.Fprintln(stderr,"runtime query takes named options only");runtimeQueryUsage(stderr);return 2}
	if *limit<=0||*limit>500 {fmt.Fprintln(stderr,"--limit must be between 1 and 500");return 2}
	if *state!="any"&&*state!="open"&&*state!="terminal"&&*state!="unknown" {fmt.Fprintln(stderr,"--state must be any, open, terminal or unknown");return 2}
	if *role!=""&&*role!="root"&&*role!="registrar"&&*role!="controller"&&*role!="worker" {fmt.Fprintln(stderr,"--role must be root, registrar, controller or worker");return 2}
	if *provider!=""&&*provider!="codex"&&*provider!="claude" {fmt.Fprintln(stderr,"--provider must be codex or claude");return 2}
	fields,err:=validateRuntimeQueryFields(*fieldText)
	if err!=nil {fmt.Fprintln(stderr,err);return 2}
	resolved,err:=runtimeobs.ResolveProject(*project)
	if err!=nil {fmt.Fprintln(stderr,err);return 1}
	// BuildLedger's 'recentLimit' trims recent transitions inside an attempt,
	// not the attempt list. Apply the query filters before output pagination.
	ledger,err:=runtimeobs.BuildLedger(resolved,10,time.Now())
	if err!=nil {fmt.Fprintln(stderr,err);return 1}
	opts:=runtimeQueryOptions{Provider:*provider,AgentID:*agentID,AgentPath:*agentPath,Role:*role,SessionID:*sessionID,TaskID:*taskID,State:*state,Limit:*limit,Fields:fields}
	matches:=filterRuntimeQuery(ledger.Attempts,opts)
	response:=runtimeQueryResponse{
		OK:true,Status:"found",GeneratedAt:ledger.GeneratedAt,
		Count:len(matches),Attempts:[]map[string]any{},TotalMatches:0,
		Findings:[]runtimeobs.LedgerFinding{},
		ObservationScope:"runtime evidence only; non-terminal state does not prove process liveness",
	}
	for _,attempt:=range ledger.Attempts {if matchesRuntimeQuery(attempt,opts){response.TotalMatches++}}
	if len(matches)==0{response.Status="not_found"}
	ids:=map[string]bool{}
	for _,attempt:=range matches {
		ids[attempt.AttemptID]=true
		payload,err:=json.Marshal(attempt)
		if err!=nil {fmt.Fprintln(stderr,err);return 1}
		var data map[string]any
		if err:=json.Unmarshal(payload,&data);err!=nil {fmt.Fprintln(stderr,err);return 1}
		if len(fields)>0{
			filtered:=make(map[string]any,len(fields))
			for _,field:=range fields {filtered[field]=data[field]}
			data=filtered
		}
		response.Attempts=append(response.Attempts,data)
	}
	for _,finding:=range ledger.Findings {
		if finding.AttemptID=="" || ids[finding.AttemptID] {response.Findings=append(response.Findings,finding)}
	}
	if *asJSON {
		if err:=json.NewEncoder(out).Encode(response);err!=nil {fmt.Fprintln(stderr,err);return 1}
		return 0
	}
	fmt.Fprintf(out,"Runtime query: %d of %d matched attempt(s); %s\n",response.Count,response.TotalMatches,response.ObservationScope)
	for _,attempt:=range response.Attempts{
		encoded,_:=json.Marshal(attempt)
		fmt.Fprintln(out,string(encoded))
	}
	if response.Status=="not_found"{fmt.Fprintln(out,"(no matching observation; does not prove there was no execution)")}
	return 0
}

func runtimeQueryRole(path string)string {
	path=strings.TrimSuffix(strings.TrimSpace(path),"/")
	switch {
	case path=="/root":return "root"
	case path=="/root/registrar":return "registrar"
	case path=="/root/controller":return "controller"
	case strings.HasPrefix(path,"/root/controller/"):return "worker"
	default:return ""
	}
}

func matchesRuntimeQuery(a runtimeobs.Attempt,o runtimeQueryOptions)bool {
	if o.Provider!=""&&o.Provider!=a.Provider{return false}
	if o.AgentID!=""&&o.AgentID!=a.RuntimeAgentID{return false}
	if o.AgentPath!=""&&o.AgentPath!=a.AgentPath{return false}
	if o.Role!=""&&o.Role!=runtimeQueryRole(a.AgentPath){return false}
	if o.SessionID!=""&&o.SessionID!=a.SessionID{return false}
	if o.TaskID!=""&&!strings.EqualFold(o.TaskID,a.TaskID){return false}
	switch o.State{
	case "open":return !a.Terminal && a.CurrentState!=runtimeobs.StateRuntimeUnknown
	case "terminal":return a.Terminal
	case "unknown":return a.CurrentState==runtimeobs.StateRuntimeUnknown||a.BindingState==runtimeobs.BindingAmbiguous
	default:return true
	}
}

func runtimeQueryStarted(a runtimeobs.Attempt)time.Time {
	for _,raw:=range []string{a.StartedAt,a.FirstObservedAt} {
		if t,err:=time.Parse(time.RFC3339Nano,raw);err==nil{return t}
	}
	return time.Time{}
}

func filterRuntimeQuery(attempts []runtimeobs.Attempt,o runtimeQueryOptions)[]runtimeobs.Attempt {
	found:=make([]runtimeobs.Attempt,0)
	for _,a:=range attempts{if matchesRuntimeQuery(a,o){found=append(found,a)}}
	sort.SliceStable(found,func(i,j int)bool {
		a,b:=runtimeQueryStarted(found[i]),runtimeQueryStarted(found[j])
		if !a.Equal(b){return a.After(b)}
		return found[i].AttemptID<found[j].AttemptID
	})
	if o.Limit>0&&len(found)>o.Limit{return found[:o.Limit]}
	return found
}

func validateRuntimeQueryFields(raw string)([]string,error){
	if strings.TrimSpace(raw)=="" {return nil,nil}
	allowed:=map[string]bool{}
	value:=reflect.TypeOf(runtimeobs.Attempt{})
	for i:=0;i<value.NumField();i++{
		field:=strings.Split(value.Field(i).Tag.Get("json"),",")[0]
		if field!=""&&field!="-" {allowed[field]=true}
	}
	fields:=[]string{}
	seen:=map[string]bool{}
	for _,part:=range strings.Split(raw,",") {
		field:=strings.TrimSpace(part)
		if !allowed[field] {return nil,fmt.Errorf("unknown --fields entry %q; use JSON keys from runtime list --json",field)}
		if !seen[field] {fields=append(fields,field);seen[field]=true}
	}
	return fields,nil
}

