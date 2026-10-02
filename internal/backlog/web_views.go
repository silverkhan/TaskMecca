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

    runtimeNow := time.Now()
    runtimeLedger, ledgerErr := runtimeobs.ReconcileLedger(project, 10, runtimeNow)
    if ledgerErr != nil {
        diagnostics = append(diagnostics, map[string]string{
            "component": "runtime_observability",
            "error":     ledgerErr.Error(),
        })
        runtimeLedger = runtimeobs.Ledger{
            Version:     1,
            GeneratedAt: runtimeNow.Format(time.RFC3339Nano),
            Attempts:    []runtimeobs.Attempt{},
            Findings:    []runtimeobs.LedgerFinding{},
        }
    }

    if _, retentionErr := runtimeobs.MaybeMaintainExecutionHistory(project, runtimeLedger, runtimeNow); retentionErr != nil {
        diagnostics = append(diagnostics, map[string]string{
            "component": "runtime_retention",
            "error":     retentionErr.Error(),
        })
    }
    historyTotal, historyObservations, historyErr := runtimeobs.ExecutionHistoryStats(project, runtimeLedger, runtimeNow)
    if historyErr != nil {
        diagnostics = append(diagnostics, map[string]string{
            "component": "runtime_history",
            "error":     historyErr.Error(),
        })
        historyTotal = 0
        historyObservations = map[string]runtimeobs.ProviderObservation{}
    }
    rootSessions, rootErr := runtimeobs.BuildRootSessions(project, runtimeLedger, runtimeNow)
    if rootErr != nil {
        diagnostics = append(diagnostics, map[string]string{
            "component": "runtime_root_sessions",
            "error":     rootErr.Error(),
        })
        rootSessions = runtimeobs.RootSessionCollection{Items: []runtimeobs.RootSession{}}
    }
    visibleRootSessions := runtimeobs.VisibleRootSessions(rootSessions, 3)
    visibleAttempts, visibleAttemptErr := runtimeobs.AttemptsForRootSessions(project, runtimeLedger, visibleRootSessions.Items, runtimeNow)
    if visibleAttemptErr != nil {
        diagnostics = append(diagnostics, map[string]string{
            "component": "runtime_root_attempts",
            "error":     visibleAttemptErr.Error(),
        })
        visibleAttempts = runtimeobs.VisibleWorkloadAttempts(runtimeLedger, 6)
    }
    visibleFindings := runtimeobs.FilterFindingsForAttempts(runtimeLedger.Findings, visibleAttempts)
    sessionGroups := runtimeobs.ClassifySessions(visibleAttempts, visibleFindings)
    sessionGroups.Terminal = historyTotal

    hookSetups := []map[string]any{}
    for _, provider := range []string{"codex", "claude"} {
        setup, hookErr := runtimeobs.HookStatus(project, provider)
        if hookErr != nil {
            diagnostics = append(diagnostics, map[string]string{
                "component": "runtime_hooks_" + provider,
                "error":     hookErr.Error(),
            })
            continue
        }
        providerObservation := historyObservations[provider]
        observedEvents := map[string]bool{
            "activity": providerObservation.Activity,
            "start":    providerObservation.Start,
            "stop":     providerObservation.Stop,
        }
        lastObservedAt := providerObservation.LastObservedAt
        observed := observedEvents["activity"] || observedEvents["start"] || observedEvents["stop"]
        state := "unconfigured"
        if setup.Installed {
            state = "verification_required"
            if observed {
                state = "observed"
            }
        }
        trustModel := "workspace_trust"
        if provider == "codex" {
            trustModel = "hook_trust"
        }
        hookSetups = append(hookSetups, map[string]any{
            "provider":          provider,
            "path":              setup.Path,
            "configured":        setup.Installed,
            "configured_events": setup.Events,
            "state":             state,
            "observed":          observed,
            "observed_events":   observedEvents,
            "last_observed_at":  lastObservedAt,
            "trust_model":       trustModel,
        })
    }

    activeCount := 0
    visibleTerminalCount := 0
    runtimeCounts := map[string]int{
        "total":       0,
        "running":     0,
        "current":     sessionGroups.Current,
        "needs_check": sessionGroups.NeedsCheck,
        "terminal":    historyTotal,
        "unbound":     0,
        "ambiguous":   0,
        "stale":       0,
    }
    for _, attempt := range runtimeLedger.Attempts {
        if !attempt.Terminal {
            activeCount++
            if attempt.CurrentState == runtimeobs.StateRunning || attempt.CurrentState == runtimeobs.StateStarting {
                runtimeCounts["running"]++
            }
        }
    }
    for _, attempt := range visibleAttempts {
        if attempt.Terminal { visibleTerminalCount++ }
        switch attempt.BindingState {
        case runtimeobs.BindingUnbound:
            runtimeCounts["unbound"]++
        case runtimeobs.BindingAmbiguous:
            runtimeCounts["ambiguous"]++
        }
    }
    runtimeCounts["total"] = activeCount + historyTotal
    staleAttempts := map[string]bool{}
    visibleIDs := map[string]bool{}
    for _, attempt := range visibleAttempts { visibleIDs[attempt.AttemptID] = true }
    for _, finding := range visibleFindings {
        if finding.Code == "stale" && finding.AttemptID != "" && visibleIDs[finding.AttemptID] {
            staleAttempts[finding.AttemptID] = true
        }
    }
    runtimeCounts["stale"] = len(staleAttempts)
    hiddenHistory := historyTotal - visibleTerminalCount
    if hiddenHistory < 0 { hiddenHistory = 0 }
    hiddenRootSessions := rootSessions.Total - len(visibleRootSessions.Items)
    if hiddenRootSessions < 0 { hiddenRootSessions = 0 }
    attemptRootSessions := map[string]string{}
    for _, attempt := range visibleAttempts {
        attemptRootSessions[attempt.AttemptID] = runtimeobs.RootSessionIDForAttempt(attempt)
    }

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
            "attempts": visibleAttempts,
            "findings": visibleFindings,
            "hooks":    hookSetups,
            "counts":   runtimeCounts,
            "session_groups": sessionGroups,
            "root_sessions": visibleRootSessions.Items,
            "root_session_counts": map[string]any{
                "active": rootSessions.Active,
                "needs_check": rootSessions.NeedsCheck,
                "terminal": rootSessions.Terminal,
                "cleanup_eligible": rootSessions.CleanupEligible,
                "total": rootSessions.Total,
                "hidden": hiddenRootSessions,
                "recent_terminal_limit": 3,
            },
            "attempt_root_sessions": attemptRootSessions,
            "history": map[string]any{
                "total": historyTotal,
                "shown_terminal": visibleTerminalCount,
                "hidden": hiddenHistory,
                "recent_limit": 6,
                "retention": runtimeobs.RetentionPolicy(),
            },
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
