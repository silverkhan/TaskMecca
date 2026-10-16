package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type diagnosticCheck struct {
	Source string `json:"source"`
	Status string `json:"status"`
	Evidence string `json:"evidence,omitempty"`
}

type runtimeDiagnostic struct {
	Status string `json:"status"`
	Diagnosis string `json:"diagnosis"`
	AsOf string `json:"as_of"`
	Provider string `json:"provider"`
	AgentID string `json:"agent_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Role string `json:"role,omitempty"`
	Hooks []runtimeobs.HookSetup `json:"hook_configuration"`
	Checks []diagnosticCheck `json:"checks"`
	Attempts []runtimeobs.Attempt `json:"matched_attempts"`
	Findings []runtimeobs.LedgerFinding `json:"findings"`
	Recovery []string `json:"recovery_steps"`
	NativeOperationAttempted bool `json:"native_operation_attempted"`
	ReconciliationPerformed bool `json:"reconciliation_performed"`
}

func runRuntimeDiagnoseCLI(args []string,out,stderr io.Writer)int{
	f:=flag.NewFlagSet("runtime diagnose",flag.ContinueOnError)
	f.SetOutput(stderr)
	project:=f.String("project",".","project root")
	provider:=f.String("provider","codex","codex or claude")
	agent:=f.String("agent-id","","exact native runtime agent ID")
	session:=f.String("session-id","","exact provider session ID")
	role:=f.String("role","","root, controller, worker, or registrar")
	limit:=f.Int("limit",4,"recent matched attempts")
	asJSON:=f.Bool("json",false,"machine-readable evidence")
	help:=f.Bool("help",false,"show examples")
	if err:=f.Parse(args);err!=nil{return 2}
	if *help{
		fmt.Fprintln(out,"task-mecca runtime diagnose --project DIR --provider codex|claude [--agent-id UUID|--session-id SESSION|--role controller] [--limit 4] --json")
		fmt.Fprintln(out,"Read-only observation diagnosis; does not generate events, bind old attempts or invoke native Agent transport.")
		return 0
	}
	if f.NArg()!=0||(*provider!="codex"&&*provider!="claude")||*limit<=0||*limit>50{
		fmt.Fprintln(stderr,"invalid options: --provider codex|claude, --limit 1..50 and no positional args");return 2
	}
	if *role!=""&&*role!="root"&&*role!="registrar"&&*role!="controller"&&*role!="worker"{
		fmt.Fprintln(stderr,"--role requires root|registrar|controller|worker");return 2
	}
	resolved,err:=runtimeobs.ResolveProject(*project)
	if err!=nil{fmt.Fprintln(stderr,err);return 1}
	ledger,err:=runtimeobs.BuildLedger(resolved,10,time.Now())
	if err!=nil{fmt.Fprintln(stderr,err);return 1}
	report:=runtimeDiagnostic{
		Status:"checked",Diagnosis:"not_observed",AsOf:ledger.GeneratedAt,Provider:*provider,
		AgentID:*agent,SessionID:*session,Role:*role,
		Hooks:[]runtimeobs.HookSetup{},Checks:[]diagnosticCheck{},Attempts:[]runtimeobs.Attempt{},
		Findings:ledger.Findings,Recovery:[]string{},
		NativeOperationAttempted:false,ReconciliationPerformed:false,
	}
	add:=func(source,status,evidence string){report.Checks=append(report.Checks,diagnosticCheck{Source:source,Status:status,Evidence:evidence})}
	// User-global and project-local settings are separate scopes. Neither
	// alone proves that a provider hook actually delivered an execution event.
	global,globalErr:=runtimeobs.GlobalHookStatus(*provider)
	local,localErr:=runtimeobs.HookStatus(resolved,*provider)
	configured:=false
	if globalErr==nil {
		report.Hooks=append(report.Hooks,global)
		if global.Installed {configured=true}
		add("global_hook_configuration",boolDiagnosticState(global.Installed),global.Path)
	}else{add("global_hook_configuration","unknown",globalErr.Error())}
	if localErr==nil {
		report.Hooks=append(report.Hooks,local)
		if local.Installed {configured=true}
		add("project_hook_configuration",boolDiagnosticState(local.Installed),local.Path)
	}else{add("project_hook_configuration","unknown",localErr.Error())}
	control,controlErr:=runtimeobs.ObservationStatus(*provider)
	enabled:=true
	if controlErr!=nil{
		add("observation_control","unknown",controlErr.Error())
	}else{
		enabled=control.Enabled
		add("observation_control",boolDiagnosticState(enabled),control.ChangedAt)
	}
	spike,spikeErr:=runtimeobs.Report(resolved,5)
	providerEvents:=0
	if spikeErr!=nil{add("hook_event_journal","unknown",spikeErr.Error())}else{
		for _,summary:=range spike.Providers{if summary.Provider==*provider{providerEvents=summary.Events}}
		add("hook_event_journal",observationDiagnosticState(providerEvents),fmt.Sprintf("provider_events=%d; source=%s",providerEvents,spike.JournalPath))
	}
	for _,a:=range ledger.Attempts {
		if a.Provider!=*provider {continue}
		if *agent!=""&&a.RuntimeAgentID!=*agent{continue}
		if *session!=""&&a.SessionID!=*session{continue}
		if *role!=""&&runtimeQueryRole(a.AgentPath)!=*role{continue}
		report.Attempts=append(report.Attempts,a)
	}
	if len(report.Attempts)>*limit{report.Attempts=report.Attempts[:*limit]}
	add("canonical_runtime_ledger",observationDiagnosticState(len(report.Attempts)),fmt.Sprintf("matched=%d; journal=%s",len(report.Attempts),ledger.JournalPath))
	report.Diagnosis=classifyRuntimeDiagnostic(enabled,configured,providerEvents,report.Attempts,time.Now())
	report.Recovery=runtimeDiagnosisSteps(report.Diagnosis,*provider)
	if *asJSON {
		if err:=json.NewEncoder(out).Encode(report);err!=nil{fmt.Fprintln(stderr,err);return 1}
		return 0
	}
	fmt.Fprintf(out,"%s: %s; matched_attempts=%d; provider_hook_events=%d\n",report.Provider,report.Diagnosis,len(report.Attempts),providerEvents)
	for _,check:=range report.Checks{fmt.Fprintf(out,"  %s: %s (%s)\n",check.Source,check.Status,check.Evidence)}
	for _,step:=range report.Recovery{fmt.Fprintf(out,"  next: %s\n",step)}
	return 0
}
func boolDiagnosticState(v bool)string{if v{return "configured"};return "not_configured"}
func observationDiagnosticState(n int)string{if n>0{return "observed"};return "not_observed"}
func classifyRuntimeDiagnostic(enabled,configured bool,events int,attempts []runtimeobs.Attempt,now time.Time)string{
	if !enabled{return "observation_disabled"}
	if len(attempts)==0{
		if events>0{return "no_matching_runtime"}
		if !configured{return "hooks_unconfigured"}
		return "not_observed"
	}
	latest:=attempts[0]
	if latest.BindingState==runtimeobs.BindingAmbiguous{return "binding_ambiguous"}
	if latest.BindingState==runtimeobs.BindingUnbound{return "binding_unbound"}
	if latest.Terminal{return "terminal_history"}
	if latest.CurrentState==runtimeobs.StateRuntimeUnknown{return "runtime_unknown"}
	last:=latest.LastObservedAt
	if last==""{last=latest.FirstObservedAt}
	if moment,err:=time.Parse(time.RFC3339Nano,last);err==nil && now.Sub(moment)>runtimeobs.StaleWarnDuration(){return "observation_stale"}
	return "observed_nonterminal"
}
func runtimeDiagnosisSteps(kind,provider string)[]string{
	switch kind{
	case "observation_disabled":return []string{"Check runtime observation control and explicit provider Hook approval; do not synthesize events."}
	case "hooks_unconfigured":return []string{"Review task-mecca runtime hooks status "+provider+" --json and provider application Hook settings.","Enable approved Hooks, then start a fresh session/attempt and check real observation."}
	case "not_observed":return []string{"Hooks are configured but no matching actual execution evidence was observed; verify provider Hook execution and permissions.","Use a new real provider session/turn; do not reuse completed attempts or inject synthetic events."}
	case "no_matching_runtime":return []string{"Provider Hook events exist, but the requested agent/session/role was not matched; verify exact IDs.","Use runtime query with explicit --agent-id or --session-id; check binding separately."}
	case "binding_unbound","binding_ambiguous":return []string{"Review real runtime assignment and provider evidence; bind only an exact verified current attempt.","Do not infer identity from agent name, timing or a completed past attempt."}
	case "terminal_history":return []string{"Only a terminal attempt is currently matched; start a new real provider run if needed.","Do not treat prior completed attempt as a live target."}
	case "observation_stale":return []string{"Recent Hook activity was not observed; inspect provider status and observation gap.","Avoid declaring the process terminated without authoritative terminal evidence."}
	case "runtime_unknown":return []string{"Available evidence cannot establish current execution; check provider state and current Hook delivery.","Do not create synthetic lifecycle transitions."}
	default:return []string{"Observed non-terminal evidence does not by itself prove current liveness; verify real native provider state before dispatch."}
	}
}

var _ = strings.TrimSpace
