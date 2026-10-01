package runtimeobs

import (
    "io"
    "strings"
    "time"
)

func ObserveHook(project,provider string,input io.Reader,now time.Time) (ExecutionEvent,error) {
    spike,err:=ParseHookEvent(provider,input,now)
    if err!=nil { return ExecutionEvent{},err }
    event,err:=HookToExecutionEvent(spike)
    if err!=nil { return ExecutionEvent{},err }

    // Some provider/tool event variants may omit session_id. Reuse a unique
    // live runtime-agent match if one exists; never guess when multiple
    // candidates exist.
    if event.SessionID=="" && event.RuntimeAgentID!="" {
        if ledger,readErr:=BuildLedger(project,5,now); readErr==nil {
            matches:=[]Attempt{}
            for _,attempt:=range ledger.Attempts {
                if attempt.Provider==event.Provider && attempt.RuntimeAgentID==event.RuntimeAgentID && !attempt.Terminal {
                    matches=append(matches,attempt)
                }
            }
            if len(matches)==1 { event.AttemptID=matches[0].AttemptID; event.EventID=eventIDFor(event) }
        }
    }

    if err:=AppendExecutionEvent(project,event); err!=nil { return ExecutionEvent{},err }
    return event,nil
}

func FilterLedger(ledger Ledger,provider string) Ledger {
    provider=strings.ToLower(strings.TrimSpace(provider))
    if provider=="" { return ledger }
    filtered:=ledger
    filtered.Attempts=nil
    filtered.Findings=nil
    allowed:=map[string]bool{}
    for _,attempt:=range ledger.Attempts {
        if attempt.Provider==provider { filtered.Attempts=append(filtered.Attempts,attempt); allowed[attempt.AttemptID]=true }
    }
    for _,finding:=range ledger.Findings {
        if finding.AttemptID=="" || allowed[finding.AttemptID] { filtered.Findings=append(filtered.Findings,finding) }
    }
    return filtered
}
