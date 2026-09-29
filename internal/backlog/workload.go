package backlog

import (
    "sort"
)

func runtimeAggregate(values map[string]map[string]any) map[string]any {
    providerSet:=map[string]bool{}
    statusSet:=map[string]bool{}
    for _,value:=range values {
        provider:="unknown"
        status:="unknown"
        if raw,ok:=value["runtime_provider"].(string); ok && raw!="" { provider=raw }
        if raw,ok:=value["dispatch_status"].(string); ok && raw!="" { status=raw }
        providerSet[provider]=true
        statusSet[status]=true
    }
    providers:=[]string{}
    statuses:=[]string{}
    for value:=range providerSet { providers=append(providers,value) }
    for value:=range statusSet { statuses=append(statuses,value) }
    sort.Strings(providers)
    sort.Strings(statuses)
    return map[string]any{"providers":providers,"statuses":statuses,"conflict":false}
}

func Workload(project,root string) (map[string]any,error) {
    rows,err:=Catalog(project,root)
    if err!=nil { return nil,err }
    readyReport,err:=Ready(project,root)
    if err!=nil { return nil,err }
    timings,err:=LifecycleTimings(project,root)
    if err!=nil { return nil,err }
    return workloadFrom(rows,readyReport,timings),nil
}

func workloadFrom(rows []Record, readyReport map[string]any, timings map[string]map[string]any) map[string]any {
    byID:=map[string]Record{}
    for _,row:=range rows { byID[row.ID]=row }
    grouped:=map[string]map[string]any{}
    bucketFor:=func(agent string) map[string]any {
        if bucket,ok:=grouped[agent]; ok { return bucket }
        bucket:=map[string]any{
            "agent":agent,
            "doing":[]string{},
            "blocking":[]string{},
            "ready_candidates":[]map[string]any{},
            "hold_history":[]any{},
            "change_scopes":map[string]any{},
        }
        grouped[agent]=bucket
        return bucket
    }

    unassigned:=[]string{}
    releasedHolds:=[]map[string]any{}
    for _,row:=range rows {
        if row.Location!="active" || (row.State!="doing" && row.State!="hold") { continue }
        assignment:=assignmentView(row)
        if row.State=="hold" {
            releasedHolds=append(releasedHolds,map[string]any{
                "id":row.ID,
                "title":row.Title,
                "path":row.Path,
                "current_agent":"",
                "current_change_scope":"",
                "audit":assignment["hold_audit"],
                "runtime_metadata":runtimeFromFields(row.Fields),
            })
            continue
        }
        agent,_:=assignment["agent"].(string)
        if agent=="" { unassigned=append(unassigned,row.ID); continue }
        bucket:=bucketFor(agent)
        bucket["doing"]=append(bucket["doing"].([]string),row.ID)
        scopes:=bucket["change_scopes"].(map[string]any)
        scopes[row.ID]=assignment["change_scope"]
    }

    if blockedRows,ok:=readyReport["blocked"].([]map[string]any); ok {
        for _,blocked:=range blockedRows {
            taskID,_:=blocked["id"].(string)
            waiting,_:=blocked["waiting_for"].([]string)
            for _,depID:=range waiting {
                dep,ok:=byID[depID]
                if !ok || dep.State!="doing" { continue }
                agent:=dep.Fields["Agent"]
                if agent=="" { continue }
                bucket:=bucketFor(agent)
                current:=bucket["blocking"].([]string)
                found:=false
                for _,id:=range current { if id==taskID { found=true; break } }
                if !found { bucket["blocking"]=append(current,taskID) }
            }
        }
    }

    if readyRows,ok:=readyReport["ready"].([]map[string]any); ok {
        for _,candidate:=range readyRows {
            itemID,_:=candidate["id"].(string)
            source,ok:=byID[itemID]
            if !ok { continue }
            continuityRows,_:=continuity(rows,source)
            if len(continuityRows)==0 { continue }
            top,_:=continuityRows[0]["count"].(int)
            for _,entry:=range continuityRows {
                count,_:=entry["count"].(int)
                if count!=top { break }
                agent,_:=entry["agent"].(string)
                if agent=="" { continue }
                bucket:=bucketFor(agent)
                evidence,_:=entry["evidence_ids"].([]string)
                bucket["ready_candidates"]=append(bucket["ready_candidates"].([]map[string]any),map[string]any{
                    "id":itemID,"evidence_ids":evidence,"count":count,
                })
            }
        }
    }

    agents:=[]map[string]any{}
    for _,bucket:=range grouped {
        doing:=bucket["doing"].([]string)
        blocking:=bucket["blocking"].([]string)
        candidates:=bucket["ready_candidates"].([]map[string]any)
        holdHistory:=bucket["hold_history"].([]any)
        bucket["doing_count"]=len(doing)
        bucket["blocking_count"]=len(blocking)
        bucket["ready_candidate_count"]=len(candidates)
        bucket["hold_history_count"]=len(holdHistory)
        details:=[]map[string]any{}
        runtimeValues:=map[string]map[string]any{}
        for _,itemID:=range doing {
            lifecycle:=map[string]any{}
            if value,ok:=timings[itemID]; ok { lifecycle=value }
            metadata:=runtimeFromFields(byID[itemID].Fields)
            runtimeValues[itemID]=metadata
            elapsed:="-"
            var elapsedSeconds any=nil
            if value,ok:=lifecycle["elapsed"].(string); ok { elapsed=value }
            if value,ok:=lifecycle["elapsed_seconds"]; ok { elapsedSeconds=value }
            details=append(details,map[string]any{
                "id":itemID,
                "elapsed":elapsed,
                "elapsed_seconds":elapsedSeconds,
                "lifecycle":lifecycle,
                "runtime_metadata":metadata,
            })
        }
        for _,candidate:=range candidates {
            lifecycle:=map[string]any{}
            if value,ok:=timings[candidate["id"].(string)]; ok { lifecycle=value }
            candidate["lifecycle"]=lifecycle
        }
        bucket["doing_details"]=details
        bucket["runtime_metadata"]=runtimeAggregate(runtimeValues)
        agents=append(agents,bucket)
    }
    sort.Slice(agents,func(i,j int)bool {
        idi:=agents[i]["doing_count"].(int); idj:=agents[j]["doing_count"].(int)
        if idi!=idj { return idi>idj }
        bi:=agents[i]["blocking_count"].(int); bj:=agents[j]["blocking_count"].(int)
        if bi!=bj { return bi>bj }
        return agents[i]["agent"].(string)<agents[j]["agent"].(string)
    })
    sort.Strings(unassigned)
    sort.Slice(releasedHolds,func(i,j int)bool{return releasedHolds[i]["id"].(string)<releasedHolds[j]["id"].(string)})

    return map[string]any{
        "agents":agents,
        "execution_summary":map[string]any{"model_effort_tracking":"removed","basis":"RuntimeProvider/Dispatch상태 only"},
        "unassigned_doing":unassigned,
        "released_holds":releasedHolds,
        "ready_candidate_basis":"explicit depends/related completed Agent history only; not live state or assignment",
    }
}
