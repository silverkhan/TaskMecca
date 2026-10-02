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
