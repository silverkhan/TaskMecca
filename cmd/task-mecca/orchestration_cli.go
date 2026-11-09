package main

import (
 "encoding/json"
 "flag"
 "fmt"
 "io"
 "path/filepath"
 "strings"
 "time"

 "github.com/silverkhan/TaskMecca/internal/handoff"
 "github.com/silverkhan/TaskMecca/internal/orchestration"
)

func runtimeOrchestrationCapabilities(project,provider string) (orchestration.Capabilities,error) {
 caps,err:=handoff.LoadCapabilities(project,provider)
 if err!=nil{return orchestration.Capabilities{},err}
 return orchestration.Capabilities{
  SpawnSubagent:string(caps.CanSpawnSubagent),CreateSession:string(caps.CanCreateSession),
  MessageSession:string(caps.CanCrossSessionMessage),MessageRoot:string(caps.CanMessageRoot),
  ResumeSession:string(caps.CanResumeSession),WakeRoot:string(caps.CanWakeRoot),
 },nil
}

// Read-only decisions and durable Root selection. Native provider transport is external.
func runOrchestrationCLI(args []string,stdout,stderr io.Writer) int {
 if len(args)==0 {fmt.Fprintln(stderr,"usage: task-mecca orchestration modes|decide|configure|status|name");return 2}
 action:=strings.ToLower(strings.TrimSpace(args[0]))
 flags:=flag.NewFlagSet("orchestration "+action,flag.ContinueOnError)
 flags.SetOutput(stderr)
 project:=flags.String("project",".","canonical project directory")
 provider:=flags.String("provider","","provider of exact Root session and recorded capability evidence")
 rootSessionID:=flags.String("root-session-id","","provider-native Root session identity")
 mode:=flags.String("mode","","A|B|C|D")
 event:=flags.String("event","","orchestration event")
 task:=flags.String("task","","task identifier")
 capsArg:=flags.String("capabilities","","explicit capabilities JSON for diagnostic decisions only")
 durable:=flags.Bool("durable-recorded",false,"confirmed existing durable user hold")
 confirm:=flags.Bool("confirmed",false,"user explicitly selected the mode")
 change:=flags.Bool("change-mode",false,"user explicitly changed an existing mode")
 decisionEvidence:=flags.String("decision-evidence","","user decision reference for mode change")
 group:=flags.String("group","","group ID, e.g. D1109_01")
 role:=flags.String("role","","root|registrar|controller|worker")
 worker:=flags.String("worker","","canonical Worker display name")
 generation:=flags.Int("generation",1,"Worker generation")
 timezone:=flags.String("timezone","","IANA timezone for display date, default system local")
 asJSON:=flags.Bool("json",false,"structured JSON output")
 if err:=flags.Parse(args[1:]);err!=nil{return 2}
 if flags.NArg()!=0 {fmt.Fprintln(stderr,"unexpected positional arguments");return 2}
 abs,err:=filepath.Abs(*project)
 if err!=nil {fmt.Fprintln(stderr,err);return 2}
 write:=func(value any)int{if err:=json.NewEncoder(stdout).Encode(value);err!=nil{fmt.Fprintln(stderr,err);return 1};return 0}
 capabilities:=orchestration.Capabilities{}
 if *capsArg!="" && *provider!="" {fmt.Fprintln(stderr,"--capabilities cannot be combined with provider evidence");return 2}
 if *capsArg!="" {
  if err:=json.Unmarshal([]byte(*capsArg),&capabilities);err!=nil{fmt.Fprintln(stderr,"invalid capabilities:",err);return 2}
 } else if *provider!="" {
  capabilities,err=runtimeOrchestrationCapabilities(abs,*provider)
  if err!=nil{fmt.Fprintln(stderr,err);return 1}
 }
 selected:=orchestration.Mode(strings.ToUpper(strings.TrimSpace(*mode)))
 switch action {
 case "modes":
  type availability struct {
   orchestration.ModeInfo
   Available bool `json:"available"`
   Reasons []string `json:"reasons"`
  }
  out:=[]availability{}
  for _,m:=range orchestration.Modes {
   row:=availability{ModeInfo:m,Reasons:[]string{}}
   if *provider!="" {
    d,err:=orchestration.Decide(orchestration.Request{Mode:m.Code,Event:"root_initialized",Capabilities:capabilities})
    if err!=nil{fmt.Fprintln(stderr,err);return 1}
    row.Available=d.Allowed;row.Reasons=d.Reasons
   } else {row.Reasons=[]string{"provider_capabilities_not_checked"}}
   out=append(out,row)
  }
  if *asJSON{return write(out)}
  for _,m:=range out {
   status:="not checked"
   if *provider!="" {if m.Available{status="available"}else{status="unavailable: "+strings.Join(m.Reasons,",")}}
   fmt.Fprintf(stdout,"%s — %s\n",m.Label,status)
  }
  return 0
 case "decide":
  if *event==""{fmt.Fprintln(stderr,"--event required");return 2}
  d,err:=orchestration.Decide(orchestration.Request{Mode:selected,Event:*event,TaskID:*task,Capabilities:capabilities,DurableRecorded:*durable})
  if err!=nil{fmt.Fprintln(stderr,err);return 2}
  if *asJSON{return write(d)}
  fmt.Fprintf(stdout,"%s: %s (allowed=%t)\n",d.Mode,d.Action,d.Allowed)
  for _,reason:=range d.Reasons{fmt.Fprintln(stdout,"- "+reason)}
  return 0
 case "configure":
  if *provider==""||*rootSessionID==""||!*confirm {fmt.Fprintln(stderr,"configure requires --provider, --root-session-id, --mode, and --confirmed");return 2}
  now:=time.Now()
  if *timezone!="" {loc,err:=time.LoadLocation(*timezone);if err!=nil{fmt.Fprintln(stderr,err);return 2};now=now.In(loc)}
  g,err:=orchestration.Configure(abs,*provider,*rootSessionID,selected,capabilities,*change,*decisionEvidence,now)
  if err!=nil{fmt.Fprintln(stderr,err);return 1}
  if *asJSON{return write(g)}
  fmt.Fprintf(stdout,"Root group %s; mode %s; prefix %s\n",g.GroupID,g.Mode,g.DisplayPrefix)
  return 0
 case "status":
  if *provider==""||*rootSessionID==""{fmt.Fprintln(stderr,"status requires --provider and --root-session-id");return 2}
  g,exists,err:=orchestration.FindGroup(abs,*provider,*rootSessionID)
  if err!=nil{fmt.Fprintln(stderr,err);return 1}
  if !exists{fmt.Fprintln(stderr,"Root group not configured");return 1}
  d,err:=orchestration.Decide(orchestration.Request{Mode:g.Mode,Event:"root_initialized",Capabilities:capabilities})
  if err!=nil{fmt.Fprintln(stderr,err);return 1}
  if *asJSON{return write(struct{
   Group orchestration.Group `json:"group"`
   CurrentCapabilityDecision orchestration.Decision `json:"current_capability_decision"`
  }{g,d})}
  fmt.Fprintf(stdout,"Group %s, mode %s, currently available=%t\n",g.GroupID,g.Mode,d.Allowed)
  return 0
 case "name":
  g,err:=orchestration.FindGroupByID(abs,*group)
  if err!=nil{fmt.Fprintln(stderr,err);return 1}
  name,err:=orchestration.SessionName(g,*role,*worker,*generation)
  if err!=nil{fmt.Fprintln(stderr,err);return 2}
  if *asJSON{return write(map[string]any{"group":g.GroupID,"display_name":name,"identity_authoritative":false})}
  fmt.Fprintln(stdout,name);return 0
 default:
  fmt.Fprintln(stderr,"usage: task-mecca orchestration modes|decide|configure|status|name")
  return 2
 }
}
