package backlog

import (
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// runtimeLedgerSignals projects only explicitly/reliably bound execution attempts
// into backlog health. Unbound or ambiguous attempts are never guessed onto tasks.
func runtimeLedgerSignals(project string,rows []Record,now time.Time) map[string]map[string]any {
    out:=map[string]map[string]any{}
    ledger,err:=runtimeobs.ReconcileLedger(project,20,now)
    if err!=nil { return out }

    doing:=map[string]bool{}
    for _,row:=range rows {
        if row.Location=="active" && row.State=="doing" { doing[strings.ToUpper(strings.TrimSpace(row.ID))]=true }
    }
    stale:=map[string]bool{}
    for _,finding:=range ledger.Findings {
        if finding.Code=="stale" && finding.AttemptID!="" { stale[finding.AttemptID]=true }
    }

    latest:=map[string]runtimeobs.Attempt{}
    for _,attempt:=range ledger.Attempts {
        taskID:=strings.ToUpper(strings.TrimSpace(attempt.TaskID))
        if !doing[taskID] || attempt.BindingState!=runtimeobs.BindingBound { continue }
        previous,ok:=latest[taskID]
        if !ok || attemptSignalTime(attempt).After(attemptSignalTime(previous)) { latest[taskID]=attempt }
    }
    for taskID,attempt:=range latest {
        health:="active"
        switch attempt.CurrentState {
        case runtimeobs.StateWaitingUser,runtimeobs.StateWaitingApproval:
            health="needs_user"
        case runtimeobs.StateCompleted:
            health="awaiting_finalize"
        case runtimeobs.StateErrored,runtimeobs.StateInterrupted,runtimeobs.StateShutdown:
            health="execution_interrupted"
        case runtimeobs.StateRuntimeUnknown:
            health="runtime_unknown"
        default:
            if stale[attempt.AttemptID] { health="stale" }
        }
        signal:=map[string]any{
            "health":health,"source":"execution_ledger","attempt_id":attempt.AttemptID,
            "runtime_state":attempt.CurrentState,"last_activity_at":attempt.LastActivityAt,
            "binding_state":attempt.BindingState,"binding_source":attempt.BindingSource,
            "evidence_source":attempt.StateEvidenceSource,"observation_quality":attempt.StateObservationQuality,
        }
        out[taskID]=signal
    }
    return out
}

func attemptSignalTime(a runtimeobs.Attempt) time.Time {
    for _,raw:=range []string{a.LastObservedAt,a.EndedAt,a.LastActivityAt,a.StartedAt,a.FirstObservedAt} {
        if at,err:=time.Parse(time.RFC3339Nano,raw); err==nil { return at }
    }
    return time.Time{}
}

func mergeRuntimeSignals(base,ledger map[string]map[string]any) map[string]map[string]any {
    out:=map[string]map[string]any{}
    for id,signal:=range base { out[id]=signal }
    for id,signal:=range ledger {
        // A bound Execution Ledger attempt has stronger task correlation than the
        // legacy registry signal. Keep legacy data only when no bound signal exists.
        out[id]=signal
    }
    return out
}
