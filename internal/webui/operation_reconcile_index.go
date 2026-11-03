package webui

import (
    "time"
)

// Historic evidence is immutable during a scan; indexing by the exact fields
// used by both suppression predicates avoids scanning every old incident once
// for every runtime attempt. Retain the original predicates as the semantic
// authority (including stale-window and changed-source behavior).
type resolvedSourceKey struct {
    project string
    attemptID string
    assignmentID string
    taskID string
    agentPath string
    kind string
    evidence string
}

func sourceKey(item operationIncident) resolvedSourceKey {
    return resolvedSourceKey{
        project: item.Project, attemptID: item.AttemptID,
        assignmentID: item.AssignmentID, taskID: item.TaskID,
        agentPath: item.AgentPath, kind: item.Kind, evidence: item.Evidence,
    }
}

type resolvedSourceIndex struct {
    incidents map[resolvedSourceKey][]operationIncident
}

func indexResolvedSources(journal operationJournal) resolvedSourceIndex {
    out := resolvedSourceIndex{incidents: make(map[resolvedSourceKey][]operationIncident)}
    for _, item := range journal.Incidents {
        if item.RecoveredAt == "" { continue }
        // Neither suppression predicate can match unrelated event kinds.
        if item.Kind != "runtime_unknown" && item.Kind != "no_signal" { continue }
        out.incidents[sourceKey(item)] = append(out.incidents[sourceKey(item)], item)
    }
    return out
}

func (index resolvedSourceIndex) alreadyResolved(signal operationIncident, now time.Time) bool {
    return operationSourceAlreadyResolved(signal, operationJournal{
        Incidents: index.incidents[sourceKey(signal)],
    }, now)
}

func (index resolvedSourceIndex) latestAssignmentResolved(signal operationIncident) bool {
    return operationLatestAssignmentSourceAlreadyResolved(signal, operationJournal{
        Incidents: index.incidents[sourceKey(signal)],
    })
}
