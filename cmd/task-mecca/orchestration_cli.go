package main

import (
 "encoding/json"
 "flag"
 "fmt"
 "io"
 "strings"

 "github.com/silverkhan/TaskMecca/internal/orchestration"
)

// Decision-only orchestration CLI. Runtime transports and durable writers
// remain in their existing owning components.
func runOrchestrationCLI(args []string,stdout,stderr io.Writer) int {
 if len(args)==0 {fmt.Fprintln(stderr,"usage: task-mecca orchestration modes|decide");return 2}
 switch args[0] {
 case "modes":
  if len(args)!=1 {fmt.Fprintln(stderr,"orchestration modes takes no options");return 2}
  if err:=json.NewEncoder(stdout).Encode(orchestration.Modes);err!=nil {fmt.Fprintln(stderr,err);return 1};return 0
 case "decide":
  flags:=flag.NewFlagSet("orchestration decide",flag.ContinueOnError)
  flags.SetOutput(stderr)
  mode:=flags.String("mode","","A|B|C|D")
  event:=flags.String("event","","root_initialized|user_decision_required|registration_completed|worker_done|session_resume")
  task:=flags.String("task","","task identifier")
  caps:=flags.String("capabilities","{}","capability JSON (supported|unsupported|unknown)")
  recorded:=flags.Bool("durable-recorded",false,"confirmed existing durable user hold")
  asJSON:=flags.Bool("json",false,"JSON result")
  if err:=flags.Parse(args[1:]);err!=nil{return 2}
  if flags.NArg()!=0 {fmt.Fprintln(stderr,"unexpected positional arguments");return 2}
  var capabilities orchestration.Capabilities
  if err:=json.Unmarshal([]byte(*caps),&capabilities);err!=nil {fmt.Fprintln(stderr,"invalid capabilities JSON:",err);return 2}
  request:=orchestration.Request{
   Mode:orchestration.Mode(strings.ToUpper(strings.TrimSpace(*mode))),
   Event:*event,TaskID:*task,Capabilities:capabilities,DurableRecorded:*recorded,
  }
  decision,err:=orchestration.Decide(request)
  if err!=nil {fmt.Fprintln(stderr,err);return 2}
  if *asJSON {
   if err:=json.NewEncoder(stdout).Encode(decision);err!=nil {fmt.Fprintln(stderr,err);return 1}
  } else {
   fmt.Fprintf(stdout,"%s: %s (allowed=%t)\n",decision.Mode,decision.Action,decision.Allowed)
   for _,reason:=range decision.Reasons {fmt.Fprintln(stdout,"- "+reason)}
  }
  return 0
 default:
  fmt.Fprintln(stderr,"usage: task-mecca orchestration modes|decide");return 2
 }
}
