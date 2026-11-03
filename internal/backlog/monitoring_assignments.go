package backlog

import "strings"

// MonitoringAssignments is a deliberately narrow read-only projection for
// operation monitoring. Unlike WorkloadSnapshot it never reconciles the
// control tower, runtime sessions, notification/retention state or hook
// observations. It preserves the active/doing and preferred-row selection
// semantics of WorkloadSnapshot's all_items collection.
func MonitoringAssignments(project, root string) (map[string]string, error) {
    rows, err := CachedCatalog(project, root)
    if err != nil { return nil, err }
    return monitoringAssignmentsFromRows(rows), nil
}

func monitoringAssignmentsFromRows(rows []Record) map[string]string {
    assignments := make(map[string]string)
    for id, row := range preferredRows(rows) {
        if row.Location != "active" || row.State != "doing" { continue }
        agent := strings.TrimSpace(row.Fields["Agent"])
        if agent == "" || agent == "-" || agent == "<nil>" { continue }
        assignments[id] = agent
    }
    return assignments
}
