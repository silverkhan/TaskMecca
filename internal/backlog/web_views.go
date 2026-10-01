package backlog

import (
    "time"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

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

    runtimeLedger, ledgerErr := runtimeobs.ReconcileLedger(project, 10, time.Now())
    if ledgerErr != nil {
        diagnostics = append(diagnostics, map[string]string{
            "component": "runtime_observability",
            "error":     ledgerErr.Error(),
        })
        runtimeLedger = runtimeobs.Ledger{
            Version:     1,
            GeneratedAt: time.Now().Format(time.RFC3339Nano),
            Attempts:    []runtimeobs.Attempt{},
            Findings:    []runtimeobs.LedgerFinding{},
        }
    }

    hookSetups := []runtimeobs.HookSetup{}
    for _, provider := range []string{"codex", "claude"} {
        setup, hookErr := runtimeobs.HookStatus(project, provider)
        if hookErr != nil {
            diagnostics = append(diagnostics, map[string]string{
                "component": "runtime_hooks_" + provider,
                "error":     hookErr.Error(),
            })
            continue
        }
        hookSetups = append(hookSetups, setup)
    }

    runtimeCounts := map[string]int{
        "total":     len(runtimeLedger.Attempts),
        "running":   0,
        "terminal":  0,
        "unbound":   0,
        "ambiguous": 0,
        "stale":     0,
    }
    for _, attempt := range runtimeLedger.Attempts {
        if attempt.Terminal {
            runtimeCounts["terminal"]++
        } else if attempt.CurrentState == runtimeobs.StateRunning || attempt.CurrentState == runtimeobs.StateStarting {
            runtimeCounts["running"]++
        }
        switch attempt.BindingState {
        case runtimeobs.BindingUnbound:
            runtimeCounts["unbound"]++
        case runtimeobs.BindingAmbiguous:
            runtimeCounts["ambiguous"]++
        }
    }
    staleAttempts := map[string]bool{}
    for _, finding := range runtimeLedger.Findings {
        if finding.Code == "stale" && finding.AttemptID != "" {
            staleAttempts[finding.AttemptID] = true
        }
    }
    runtimeCounts["stale"] = len(staleAttempts)

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
        "runtime_observability": map[string]any{
            "attempts": runtimeLedger.Attempts,
            "findings": runtimeLedger.Findings,
            "hooks":    hookSetups,
            "counts":   runtimeCounts,
            "generated_at": runtimeLedger.GeneratedAt,
        },
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
