package runtimeobs

import (
    "errors"
    "io"
    "strings"
    "time"
)

func ObserveHook(project,provider string,input io.Reader,now time.Time) (ExecutionEvent,error) {
    spike,err:=ParseHookEvent(provider,input,now)
    if err!=nil { return ExecutionEvent{},err }
    // SessionStart is session-level metadata, not an execution attempt. Keep it
    // in the same append-only journal so Root Session aggregation can consume it,
    // while BuildLedger deliberately excludes it from attempt accounting.
    if strings.EqualFold(spike.HookEventName,"SessionStart") {
        if strings.TrimSpace(spike.SessionID)=="" { return ExecutionEvent{},errors.New("SessionStart hook does not include session_id") }
        event:=ExecutionEvent{
            EventKind:"session_metadata",ObservedAt:spike.ObservedAt,
            Provider:strings.ToLower(strings.TrimSpace(spike.Provider)),SessionID:spike.SessionID,
            SessionName:spike.SessionName,SessionTitle:spike.SessionTitle,
            AttemptID:"rootmeta-"+rootSessionIDFor(spike.Provider,spike.SessionID),
            HookEventName:spike.HookEventName,EvidenceSource:EvidenceHook,
            ObservationQuality:QualityObserved,RawSHA256:spike.RawSHA256,
        }
        event.EventID=eventIDFor(event)
        if err:=AppendExecutionEvent(project,event); err!=nil { return ExecutionEvent{},err }
        return event,nil
    }

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
