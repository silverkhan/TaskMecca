package activity

import (
    "sort"
)

// CorrectionFinding preserves unexpected correction chains as audit problems
// instead of silently rewriting old facts.
type CorrectionFinding struct {
    EventID string `json:"event_id"`
    Reason string `json:"reason"`
}

// EffectiveEvents projects validated corrections without mutating the durable
// immutable ledger. The original and all corrections stay queryable via List.
// A correction inherits its target's start/switch/stop action and ownership.
func EffectiveEvents(events []Event)([]Event,[]CorrectionFinding){
    hasCorrections:=false
    for _,e:=range events{if e.Action==Correct{hasCorrections=true;break}}
    if !hasCorrections{return events,nil}
    byID:=make(map[string]Event,len(events))
    corrections:=make([]Event,0)
    findings:=make([]CorrectionFinding,0)
    for _,e:=range events{
        if e.Action==Correct{corrections=append(corrections,e)}else{byID[e.EventID]=e}
    }
    sort.Slice(corrections,func(i,j int)bool {
        if corrections[i].RecordedAt.Equal(corrections[j].RecordedAt){return corrections[i].EventID<corrections[j].EventID}
        return corrections[i].RecordedAt.Before(corrections[j].RecordedAt)
    })
    for _,e:=range corrections{
        target,ok:=byID[e.SupersedesID]
        if !ok {
            findings=append(findings,CorrectionFinding{EventID:e.EventID,Reason:"superseded event missing or already replaced"})
            continue
        }
        if target.TaskID!=e.TaskID||target.Role!=e.Role||target.AgentID!=e.AgentID||
            target.RuntimeAttemptID!=e.RuntimeAttemptID||target.AssignmentID!=e.AssignmentID||
            target.HandoffID!=e.HandoffID ||e.EventID==e.SupersedesID{
            findings=append(findings,CorrectionFinding{EventID:e.EventID,Reason:"attempt or ownership mismatch"})
            continue
        }
        effective:=e
        effective.Action=target.Action
        effective.SupersedesID=""
        if err:=effective.Validate();err!=nil{
            findings=append(findings,CorrectionFinding{EventID:e.EventID,Reason:"invalid effective correction"})
            continue
        }
        delete(byID,e.SupersedesID)
        byID[e.EventID]=effective
    }
    out:=make([]Event,0,len(byID))
    for _,e:=range byID{out=append(out,e)}
    sort.Slice(out,func(i,j int)bool{
        if out[i].OccurredAt.Equal(out[j].OccurredAt){return out[i].EventID<out[j].EventID}
        return out[i].OccurredAt.Before(out[j].OccurredAt)
    })
    return out,findings
}
