package backlog

import "time"

// WorkloadSnapshot returns only the data required by the Web workload view.
// It intentionally avoids DashboardSnapshot's tag catalog, Doctor checks,
// tree construction, and notification reconciliation.
func WorkloadSnapshot(project, root string) (map[string]any, error) {
    rows, err := CachedCatalog(project, root)
    if err != nil {
        return nil, err
    }

    readyReport := readyFromRows(project, root, rows)
    diagnostics := []map[string]string{}

    timings, timingErr := lifecycleTimings(project, root, rows)
    if timingErr != nil {
        diagnostics = append(diagnostics, map[string]string{
            "component": "lifecycle",
            "error":     timingErr.Error(),
        })
        timings = map[string]map[string]any{}
    }

    activity := runtimeActivity(project, rows, timings)
    workload := workloadFrom(rows, readyReport, timings)
    byID := preferredRows(rows)
    allItems := map[string]map[string]any{}

    for id, row := range byID {
        if row.Location != "active" || row.State != "doing" {
            continue
        }
        assignment := assignmentView(row)
        lifecycle := map[string]any{}
        if value, ok := timings[id]; ok {
            lifecycle = value
        }
        signal := map[string]any{"health": "n/a"}
        if value, ok := activity[id]; ok && value != nil {
            signal = value
        }
        allItems[id] = map[string]any{
            "id":           id,
            "title":        row.Title,
            "state":        row.State,
            "file_state":   row.State,
            "agent":        assignment["agent"],
            "scope":        assignment["change_scope"],
            "change_scope": assignment["change_scope"],
            "activity":     signal,
            "lifecycle":    lifecycle,
            "mtime":        row.Mtime,
            "updated_at":   row.Mtime,
        }
    }

    return map[string]any{
        "snapshot_at": time.Now().Format(time.RFC3339),
        "workload":    workload,
        "all_items":   allItems,
        "diagnostics": diagnostics,
    }, nil
}

// IssuesSnapshot returns only health/diagnostic information required by the
// Web issues view. A Doctor failure is surfaced as a diagnostic instead of
// making the entire page unavailable.
func IssuesSnapshot(project, root string) (map[string]any, error) {
    rows, err := CachedCatalog(project, root)
    if err != nil {
        return nil, err
    }

    diagnostics := []map[string]string{}
    health := map[string]any{}

    doctor, doctorErr := Doctor(project, root, false)
    if doctorErr != nil {
        diagnostics = append(diagnostics, map[string]string{
            "component": "doctor",
            "error":     doctorErr.Error(),
        })
    } else if checks, ok := doctor["checks"].(map[string]any); ok && checks != nil {
        health = checks
    }

    hold := HoldReview(rows)
    health["hold_review"] = hold["candidates"]
    health["runtime_metadata"] = runtimeFindings(rows)

    return map[string]any{
        "snapshot_at": time.Now().Format(time.RFC3339),
        "health":      health,
        "diagnostics": diagnostics,
    }, nil
}
