package main

import (
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/internal/activity"
)

// Activity mutations are explicit and evidence-backed. An untrusted agent may
// identify its own attempt, but must not infer a different actor's execution.
func runActivityCLI(args []string,out,errOut io.Writer) int {
    if len(args)==0 {fmt.Fprintln(errOut,"activity requires start, switch, stop, history or status");return 2}
    action:=args[0]
    fs:=flag.NewFlagSet("activity "+action,flag.ContinueOnError)
    fs.SetOutput(errOut)
    project:=fs.String("project",".","canonical project")
    kind:=fs.String("type","","activity type")
    role:=fs.String("role",os.Getenv("TASK_MECCA_ROLE"),"actor role")
    agent:=fs.String("agent",os.Getenv("TASK_MECCA_AGENT_ID"),"exact actor ID")
    attempt:=fs.String("attempt",os.Getenv("TASK_MECCA_ATTEMPT_ID"),"exact attempt ID")
    assignment:=fs.String("assignment","","assignment ID")
    eventID:=fs.String("event-id","","stable event ID for retries")
    evidence:=fs.String("evidence","","source evidence")
    explain:=fs.String("explanation","","required for other activity")
    proposed:=fs.String("proposed-type","","suggested type for other activity")
    round:=fs.Int("round",0,"zero for initial execution; positive for rework")
    jsonOutput:=fs.Bool("json",false,"JSON response")
    if err:=fs.Parse(args[1:]);err!=nil{return 2}
    if fs.NArg()>1 {fmt.Fprintln(errOut,"expected at most one task ID");return 2}
    root,err:=filepath.Abs(*project);if err!=nil{fmt.Fprintln(errOut,err);return 2}
    store:=activity.Ledger{Project:root}
    emit:=func(v any){if *jsonOutput{_ =json.NewEncoder(out).Encode(v)}else{fmt.Fprintln(out,v)}}
    if action=="history" || action=="status" {
        task:="";if fs.NArg()==1{task=fs.Arg(0)}
        if task==""{fmt.Fprintln(errOut,"task ID required");return 2}
        rows,readErr:=store.List(task);if readErr!=nil{fmt.Fprintln(errOut,readErr);return 1}
        if action=="history" {emit(map[string]any{"task_id":task,"events":rows});return 0}
        latest:=map[string]activity.Event{}
        for _,row:=range rows {key:=row.Role.String()+"|"+row.AgentID+"|"+row.RuntimeAttemptID;latest[key]=row}
        active:=[]activity.Event{}
        for _,row:=range latest{if row.Action!=activity.Stop{active=append(active,row)}}
        emit(map[string]any{"task_id":task,"open_activities":active,"event_count":len(rows)})
        return 0
    }
    if action!="start" && action!="switch" && action!="stop"{fmt.Fprintln(errOut,"unsupported activity action");return 2}
    if fs.NArg()!=1{fmt.Fprintln(errOut,"task ID required");return 2}
    if strings.TrimSpace(*eventID)==""{fmt.Fprintln(errOut,"--event-id required for idempotent retries");return 2}
    if strings.TrimSpace(*role)==""||strings.TrimSpace(*agent)==""||strings.TrimSpace(*attempt)==""{
        fmt.Fprintln(errOut,"--role, --agent, --attempt (or exact runtime env) required");return 2
    }
    task:=fs.Arg(0)
    if action=="stop" && *kind=="" {
        rows,readErr:=store.List(task);if readErr!=nil{fmt.Fprintln(errOut,readErr);return 1}
        for i:=len(rows)-1;i>=0;i--{
            v:=rows[i]
            if string(v.Role)==*role&&v.AgentID==*agent&&v.RuntimeAttemptID==*attempt{
                if v.Action==activity.Stop{fmt.Fprintln(errOut,"no open activity for attempt");return 2}
                *kind=string(v.Type);*round=v.Round;break
            }
        }
        if *kind==""{fmt.Fprintln(errOut,"no matching open activity to stop");return 2}
    }
    now:=time.Now().UTC()
    e:=activity.Event{SchemaVersion:1,EventID:*eventID,TaskID:task,Action:activity.Action(action),
        Role:activity.Role(*role),AgentID:*agent,RuntimeAttemptID:*attempt,AssignmentID:*assignment,
        Round:*round,Type:activity.Type(*kind),Confidence:activity.Confirmed,
        Evidence:*evidence,Explanation:*explain,ProposedType:*proposed,OccurredAt:now,RecordedAt:now}
    if err:=e.Validate();err!=nil{fmt.Fprintln(errOut,err);return 2}
    inserted,writeErr:=store.Append(e)
    if writeErr!=nil{
        if errors.Is(writeErr,activity.ErrEventConflict){fmt.Fprintln(errOut,"event-id reused with a different event; original record preserved");return 3}
        fmt.Fprintln(errOut,writeErr);return 1
    }
    emit(map[string]any{"ok":true,"recorded":inserted,"event":e})
    return 0
}
