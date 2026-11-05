package main

import (
    "fmt"
 "time"
 "github.com/silverkhan/TaskMecca/internal/handoffjournal"

    "github.com/silverkhan/TaskMecca/internal/activity"
)

type activityReportResolution struct {
    Action activity.Action
    Type activity.Type
    Round int
    ReviewOutcome string
}

// Resolve a reported phase event using the same standard taxonomy as the
// Decision Engine's activity advice. This observes an event that the agent
// explicitly reports; it does not claim that a suggestion started any work.
func resolveActivityReport(store activity.Ledger,task string,role activity.Role,agent,attempt,eventName string)(activityReportResolution,error){
    rows,err:=store.List(task)
    if err!=nil{return activityReportResolution{},err}
    handoffs,err:=handoffjournal.BuildLedger(store.Project,time.Now().UTC());if err!=nil{return activityReportResolution{},err}
    suggested:=activity.SuggestedRoundFromEvidence(task,rows,handoffs.Handoffs)
    var latest *activity.Event
    for i:=len(rows)-1;i>=0;i--{
        row:=&rows[i]
        if row.Role==role && row.AgentID==agent && row.RuntimeAttemptID==attempt && row.Action!=activity.Correct{
            latest=row
            break
        }
    }
    active:=latest!=nil && latest.Action!=activity.Stop && latest.Action!=activity.Abandon
    round:=0
    if active {
        round=latest.Round
    }else{
        round=suggested
    }
    outcome:=""
    terminal:=false
    switch eventName {
    case "worker_completed","worker_blocked","registration_completed","review_completed","phase_completed":
        terminal=true
    case "review_rejected":
        terminal=true;outcome="rejected"
    case "review_accepted":
        terminal=true;outcome="accepted"
    case "review_needs_changes":
        terminal=true;outcome="needs_changes"
    }
    if terminal {
        if !active{return activityReportResolution{},fmt.Errorf("phase event %q requires an open activity",eventName)}
        if outcome!=""{
            if role!=activity.RoleController || (latest.Type!=activity.ControllerReview&&latest.Type!=activity.ControllerReReview){
                return activityReportResolution{},fmt.Errorf("review outcome requires the active Main Controller review")
            }
        }
        return activityReportResolution{Action:activity.Stop,Type:latest.Type,Round:round,ReviewOutcome:outcome},nil
    }
    if nextRound:=suggested;nextRound>round {round=nextRound}
    suggestion:=activity.Suggest(role,eventName,round)
    next:=activity.Start
    if active{next=activity.Switch}
    return activityReportResolution{Action:next,Type:suggestion.Type,Round:round},nil
}
