package backlog

import (
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type runtimeProviderUsage struct {
    InUse bool
    CurrentEvidence bool
    Sources []string
}

func runtimeProviderUsageFrom(rows []Record, roots runtimeobs.RootSessionCollection, now time.Time) map[string]runtimeProviderUsage {
    out:=map[string]runtimeProviderUsage{"codex":{}, "claude":{}}
    add:=func(provider,source string,current bool) {
        provider=strings.ToLower(strings.TrimSpace(provider))
        usage,ok:=out[provider]
        if !ok { return }
        usage.InUse=true
        usage.CurrentEvidence=usage.CurrentEvidence || current
        found:=false
        for _,existing:=range usage.Sources { if existing==source { found=true; break } }
        if !found { usage.Sources=append(usage.Sources,source) }
        out[provider]=usage
    }
    for _,row:=range rows {
        if row.Location!="active" || row.State!="doing" { continue }
        metadata:=runtimeFromFields(row.Fields)
        provider,_:=metadata["runtime_provider"].(string)
        add(provider,"doing_task_runtime_provider",false)
    }
    cutoff:=now.Add(-30*time.Minute)
    for _,root:=range roots.Items {
        current:=root.Status=="active"
        if !current && root.LastActivityAt!="" {
            if at,err:=time.Parse(time.RFC3339Nano,root.LastActivityAt); err==nil && at.After(cutoff) { current=true }
        }
        if current { add(root.Provider,"recent_root_session",true) }
    }
    return out
}

func runtimeHookRows(project string, rows []Record, roots runtimeobs.RootSessionCollection, observations map[string]runtimeobs.ProviderObservation, now time.Time) ([]map[string]any,[]map[string]string) {
    usage:=runtimeProviderUsageFrom(rows,roots,now)
    hookSetups:=[]map[string]any{}
    diagnostics:=[]map[string]string{}
    for _,provider:=range []string{"codex","claude"} {
        setup,hookErr:=runtimeobs.HookStatus(project,provider)
        if hookErr!=nil {
            diagnostics=append(diagnostics,map[string]string{"component":"runtime_hooks_"+provider,"error":hookErr.Error()})
            continue
        }
        providerObservation:=observations[provider]
        observedEvents:=map[string]bool{"activity":providerObservation.Activity,"start":providerObservation.Start,"stop":providerObservation.Stop}
        observed:=observedEvents["activity"] || observedEvents["start"] || observedEvents["stop"]
        providerUsage:=usage[provider]
        state:="unconfigured"
        if setup.Installed {
            state="verification_required"
            if providerUsage.CurrentEvidence || (!providerUsage.InUse && observed) { state="observed" }
        }
        trustModel:="workspace_trust"
        if provider=="codex" { trustModel="hook_trust" }
        hookSetups=append(hookSetups,map[string]any{
            "provider":provider,"path":setup.Path,"configured":setup.Installed,
            "configured_events":setup.Events,"state":state,"observed":observed,
            "observed_events":observedEvents,"last_observed_at":providerObservation.LastObservedAt,
            "trust_model":trustModel,"in_use":providerUsage.InUse,
            "usage_evidence":providerUsage.Sources,"current_evidence":providerUsage.CurrentEvidence,
            "needs_attention":providerUsage.InUse && state!="observed","applies_from":"new_root_session",
        })
    }
    return hookSetups,diagnostics
}

func RuntimeHookOverview(project,root string) (map[string]any,error) {
    rows,err:=CachedCatalog(project,root)
    if err!=nil { return nil,err }
    now:=time.Now()
    ledger,ledgerErr:=runtimeobs.ReconcileLedger(project,10,now)
    if ledgerErr!=nil {
        ledger=runtimeobs.Ledger{Version:1,GeneratedAt:now.Format(time.RFC3339Nano),Attempts:[]runtimeobs.Attempt{},Findings:[]runtimeobs.LedgerFinding{}}
    }
    _,observations,historyErr:=runtimeobs.ExecutionHistoryStats(project,ledger,now)
    if historyErr!=nil { observations=map[string]runtimeobs.ProviderObservation{} }
    roots,rootErr:=runtimeobs.BuildRootSessions(project,ledger,now)
    if rootErr!=nil { roots=runtimeobs.RootSessionCollection{Items:[]runtimeobs.RootSession{}} }
    hooks,diagnostics:=runtimeHookRows(project,rows,roots,observations,now)
    if ledgerErr!=nil { diagnostics=append(diagnostics,map[string]string{"component":"runtime_observability","error":ledgerErr.Error()}) }
    if historyErr!=nil { diagnostics=append(diagnostics,map[string]string{"component":"runtime_history","error":historyErr.Error()}) }
    if rootErr!=nil { diagnostics=append(diagnostics,map[string]string{"component":"runtime_root_sessions","error":rootErr.Error()}) }
    return map[string]any{"hooks":hooks,"diagnostics":diagnostics},nil
}

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
