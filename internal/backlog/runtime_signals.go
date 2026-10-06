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
		if attempt.StartedAt == "" && !attempt.Terminal {
			health = "awaiting_start"
		}
		signal:=map[string]any{
            "health":health,"source":"execution_ledger","attempt_id":attempt.AttemptID,
            "runtime_state":attempt.CurrentState,"last_activity_at":attempt.LastActivityAt,
            "binding_state":attempt.BindingState,"binding_source":attempt.BindingSource,
			"binding_at": attempt.BindingAt, "started_at": attempt.StartedAt, "runtime_agent_id": attempt.RuntimeAgentID,
			"assignment_id":   attempt.BindingEvidence["assignment_id"],
			"evidence_source":attempt.StateEvidenceSource,"observation_quality":attempt.StateObservationQuality,
        }
        out[taskID]=signal
	}
	assignments, _ := runtimeobs.ListAssignments(project)
	latestAssignment := map[string]runtimeobs.Assignment{}
	for _, assignment := range assignments {
		key := assignment.TaskID + "\x00" + assignment.AgentPath
		if previous, ok := latestAssignment[key]; !ok || assignment.AssignedAt > previous.AssignedAt {
			latestAssignment[key] = assignment
		}
	}
	owners := map[string]int{}
	for _, row := range rows {
		if row.Location == "active" && row.State == "doing" {
			owners[strings.TrimSpace(row.Fields["Agent"])]++
		}
	}
	for _, row := range rows {
		id := strings.ToUpper(strings.TrimSpace(row.ID))
		if row.Location != "active" || row.State != "doing" || out[id] != nil {
			continue
		}
		agent := strings.TrimSpace(row.Fields["Agent"])
		if agent == "" {
			continue
		}
		assignment, hasAssignment := latestAssignment[id+"\x00"+agent]
		candidates := []string{}
		ambiguousEvidence := false
		workerName := agent
		if slash := strings.LastIndex(workerName, "/"); slash >= 0 {
			workerName = workerName[slash+1:]
		}
		for _, attempt := range ledger.Attempts {
			if attempt.Terminal || (attempt.BindingState != runtimeobs.BindingUnbound && attempt.BindingState != runtimeobs.BindingAmbiguous) {
				continue
			}
			if attempt.RuntimeAgentID == agent || attempt.RuntimeAgentID == workerName {
				candidates = append(candidates, attempt.AttemptID)
				if attempt.BindingState == runtimeobs.BindingAmbiguous { ambiguousEvidence = true }
			}
		}
		if !hasAssignment && len(candidates) == 0 {
			continue
		}
		health := "assignment_unobserved"
		if len(candidates) > 0 {
			health = "binding_pending"
		}
		if ambiguousEvidence || len(candidates) > 1 || (len(candidates) > 0 && owners[agent] > 1) {
			health = "binding_ambiguous"
		}
		out[id] = map[string]any{"health": health, "source": "execution_ledger", "assignment_id": assignment.AssignmentID,
			"assigned_at": assignment.AssignedAt, "candidate_attempt_ids": candidates, "candidate_count": len(candidates), "agent_path": agent}
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
