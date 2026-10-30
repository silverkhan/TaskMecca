package main

import (
 "encoding/json"
 "flag"
 "fmt"
 "io"
 "path/filepath"
 "time"

 "github.com/silverkhan/TaskMecca/internal/contextlife"
)

func runContextCLI(args []string,out,stderr io.Writer)int{
 if len(args)==0 {fmt.Fprintln(stderr,"usage: task-mecca context status|policy|event|observe|rotation|owner");return 2}
 group:=args[0];action:=""
 if group=="rotation"||group=="policy"||group=="owner" {
  if len(args)<2 {fmt.Fprintln(stderr,group+" requires subcommand");return 2}
  action=args[1];args=args[2:]
 } else {args=args[1:]}
 flags:=flag.NewFlagSet("context",flag.ContinueOnError);flags.SetOutput(stderr)
 project:=flags.String("project",".","canonical project")
 role:=flags.String("role","","controller|registrar|worker:<canonical-name>")
 event:=flags.String("event","","registration_completed|controller_start|controller_resume|worker_done|worker_blocked")
 attempt:=flags.String("attempt","","exact old owner attempt")
 successor:=flags.String("successor","","exact new runtime attempt")
 id:=flags.String("id","","rotation ID")
 digest:=flags.String("checkpoint","","checkpoint fingerprint")
 used:=flags.Int64("used",0,"reported used context tokens")
 window:=flags.Int64("window",0,"reported context window tokens")
 evidence:=flags.String("evidence","","source for reported token counts")
 generation:=flags.Int("generation",0,"exact generation for assert")
 apply:=flags.Bool("apply",false,"prepare an eligible rotation on a named orchestration event")
 asJSON:=flags.Bool("json",false,"structured output")
 if err:=flags.Parse(args);err!=nil{return 2}
 if flags.NArg()!=0||*role==""{fmt.Fprintln(stderr,"explicit --role and named options are required");return 2}
 root,err:=filepath.Abs(*project);if err!=nil{fmt.Fprintln(stderr,err);return 2}
 now:=time.Now()
 var result any
 switch group{
 case "status":
  result,err=contextlife.Status(root,*role,*event,now)
 case "policy":
  if action!="show"{fmt.Fprintln(stderr,"context policy show");return 2}
  result=map[string]any{"prepare_at_percent":65,"rotate_at_percent":80,"protect_at_percent":90,"max_session_age_days":7,"metrics_rule":"reported token values are unverified/advisory; no synthetic provider-verified measurement","native_spawn":"external provider runtime required","root_rotation":"manual only"}
 case "observe":
  result,err=contextlife.Observe(root,*role,*attempt,*used,*window,*evidence,now)
 case "event":
  var report contextlife.StatusReport
  report,err=contextlife.Status(root,*role,*event,now)
  if err==nil {
   if *apply&&(report.Decision.Action=="prepare"||report.Decision.Action=="rotate"||report.Decision.Action=="protect")&&report.Eligible {
    result,err=contextlife.Prepare(root,*role,*attempt,now)
   }else{result=report}
  }
 case "rotation":
  switch action{
  case "prepare":result,err=contextlife.Prepare(root,*role,*attempt,now)
  case "acknowledge":result,err=contextlife.Acknowledge(root,*role,*id,*successor,*digest,now)
  case "commit":result,err=contextlife.Commit(root,*role,*id,now)
  case "recover":result,err=contextlife.Recover(root,*role,now)
  default:fmt.Fprintln(stderr,"context rotation prepare|acknowledge|commit|recover");return 2
  }
 case "owner":
  if action!="assert"||*generation<1 {fmt.Fprintln(stderr,"context owner assert requires --attempt and --generation");return 2}
  err=contextlife.AssertOwner(root,*role,*attempt,*generation)
  result=map[string]any{"ok":err==nil,"role":*role,"attempt":*attempt,"generation":*generation}
 default:fmt.Fprintln(stderr,"context status|policy|event|observe|rotation|owner");return 2
 }
 if err!=nil {fmt.Fprintln(stderr,err);return 1}
 if *asJSON{return writeContextJSON(out,result,stderr)}
 b,e:=json.MarshalIndent(result,"","  ");if e!=nil{fmt.Fprintln(stderr,e);return 1};fmt.Fprintln(out,string(b));return 0
}
func writeContextJSON(out io.Writer,result any,stderr io.Writer)int {
 if err:=json.NewEncoder(out).Encode(result);err!=nil{fmt.Fprintln(stderr,err);return 1};return 0
}
