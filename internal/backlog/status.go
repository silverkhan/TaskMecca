package backlog

import (
    "sort"
    "time"
)

func Status(project,root string,includeDone bool) (map[string]any,error) {
    rows,err:=Catalog(project,root)
    if err!=nil { return nil,err }
    readyReport,err:=Ready(project,root)
    if err!=nil { return nil,err }
    timings,err:=LifecycleTimings(project,root)
    if err!=nil { return nil,err }

    readyByID:=map[string]map[string]any{}
    blockedByID:=map[string]map[string]any{}
    if values,ok:=readyReport["ready"].([]map[string]any); ok { for _,row:=range values { readyByID[row["id"].(string)]=row } }
    if values,ok:=readyReport["blocked"].([]map[string]any); ok { for _,row:=range values { blockedByID[row["id"].(string)]=row } }

    active:=[]map[string]any{}
    done:=[]map[string]any{}
    for _,row:=range rows {
        assignment:=assignmentView(row)
        lifecycle:=map[string]any{}
        if value,ok:=timings[row.ID]; ok { lifecycle=value }
        state:=row.State
        if _,ok:=readyByID[row.ID]; ok { state="ready" } else if _,ok:=blockedByID[row.ID]; ok { state="blocked" }
        waiting:=[]string{}
        if blocked,ok:=blockedByID[row.ID]; ok { if value,ok:=blocked["waiting_for"].([]string); ok { waiting=value } }
        itemTime:="-"
        if row.State=="doing" { if value,ok:=lifecycle["elapsed"].(string); ok { itemTime=value } }
        if row.State=="done" { if value,ok:=lifecycle["duration"].(string); ok { itemTime=value } }
        item:=map[string]any{
            "id":row.ID,"title":row.Title,"state":state,"file_state":row.State,
            "agent":assignment["agent"],"change_scope":assignment["change_scope"],
            "assignment_kind":assignment["assignment_kind"],"hold_audit":assignment["hold_audit"],
            "runtime_metadata":runtimeFromFields(row.Fields),
            "wait_note":row.Fields["대기"],"depends_on":refs(row.Fields["선행"]),"waiting_for":waiting,
            "path":row.Path,"mtime":row.Mtime,"time":itemTime,
            "created_at":lifecycle["created_at"],"started_at":lifecycle["started_at"],
            "claimed_at":lifecycle["claimed_at"],"completed_at":lifecycle["completed_at"],
            "queue_time":valueOr(lifecycle["queue"],"-"),"work_time":valueOr(lifecycle["work"],"-"),
            "lead_time":valueOr(lifecycle["lead"],"-"),"lifecycle":lifecycle,
        }
        if row.Location=="active" { active=append(active,item) } else if includeDone && row.State=="done" { done=append(done,item) }
    }
    sort.Slice(done,func(i,j int)bool {
        left:=toString(done[i]["completed_at"]); right:=toString(done[j]["completed_at"])
        if left!=right { return left>right }
        return toString(done[i]["id"])>toString(done[j]["id"])
    })

    audit,err:=Audit(project,root)
    if err!=nil { return nil,err }
    deps:=DependencyReport(rows)
    hold:=HoldReview(rows)
    health:=map[string]any{
        "audit":audit,"duplicate_ids":duplicateIDs(rows),"missing_dependencies":deps["missing"],
        "dependency_cycles":deps["cycles"],"agent_paths":agentProblems(rows),
        "scope_conflicts":ScopeConflicts(rows),"hold_review":hold["candidates"],
        "runtime_metadata":runtimeFindings(rows),
    }
    warningCount:=0
    for _,value:=range health { warningCount+=collectionLen(value) }
    doingCount:=0; holdCount:=0; doneTotal:=0
    for _,item:=range active {
        if item["file_state"]=="doing" { doingCount++ }
        if item["file_state"]=="hold" { holdCount++ }
    }
    for _,row:=range rows { if row.State=="done" { doneTotal++ } }
    workload:=workloadFrom(rows,readyReport,timings)
    return map[string]any{
        "snapshot_at":time.Now().Format("2006-01-02T15:04:05-07:00"),
        "root":baseRoot(project,root),"active":active,"done":done,"health":health,
        "hold_review":hold,"execution_summary":workload["execution_summary"],
        "counts":map[string]any{
            "doing":doingCount,"ready":len(readyByID),"blocked":len(blockedByID),
            "hold":holdCount,"done_shown":len(done),"done_total":doneTotal,"warnings":warningCount,
        },
    },nil
}

func valueOr(value any,fallback any) any { if value==nil { return fallback }; return value }

func collectionLen(value any) int {
    switch v:=value.(type) {
    case []any: return len(v)
    case []string: return len(v)
    case [][]string: return len(v)
    case []map[string]any: return len(v)
    case []map[string]string: return len(v)
    default: return 0
    }
}
