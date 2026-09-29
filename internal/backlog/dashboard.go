package backlog

import (
    "encoding/json"
    "os"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "time"
)

func dashboardItem(row Record, stateName string, waiting []string, timing map[string]any, candidates []map[string]any) map[string]any {
    assignment:=assignmentView(row)
    topAgents:=[]string{}
    for _,candidate:=range candidates {
        if agent,ok:=candidate["agent"].(string); ok && agent!="" { topAgents=append(topAgents,agent) }
    }
    updatedAt:=row.Mtime
    if events,ok:=timing["events"].([]map[string]any); ok {
        for _,event:=range events {
            at:=toString(event["at"])
            if newerISO(at,updatedAt) { updatedAt=at }
        }
    }
    itemTime:="-"
    if row.State=="doing" { itemTime=toString(valueOr(timing["elapsed"],"-")) }
    if row.State=="done" { itemTime=toString(valueOr(timing["duration"],"-")) }
    return map[string]any{
        "id":row.ID,"sort_key":row.SortKey,"title":row.Title,"state":stateName,
        "file_state":row.State,"location":row.Location,"archive_month":row.ArchiveMonth,
        "path":row.Path,"mtime":row.Mtime,"updated_at":updatedAt,
        "agent":assignment["agent"],"assignment_kind":assignment["assignment_kind"],
        "hold_audit":assignment["hold_audit"],"candidate_agents":topAgents,
        "scope":assignment["change_scope"],"runtime_metadata":runtimeFromFields(row.Fields),
        "document":row.Document,"raw_markdown":row.RawMarkdown,"registrant":row.Fields["등록자"],
        "wait_note":row.Fields["대기"],"depends_on":refs(row.Fields["선행"]),
        "related":refs(row.Fields["연관"]),"waiting_for":waiting,"time":itemTime,
        "created_at":timing["created_at"],"started_at":timing["started_at"],
        "claimed_at":timing["claimed_at"],"completed_at":timing["completed_at"],
        "queue_time":valueOr(timing["queue"],"-"),"work_time":valueOr(timing["work"],"-"),
        "lead_time":valueOr(timing["lead"],"-"),"active_seconds":timing["active_seconds"],
        "wait_seconds":timing["wait_seconds"],"queue_seconds":timing["queue_seconds"],
        "lead_seconds":timing["lead_seconds"],"lifecycle":timing,"fields":row.Fields,
    }
}

func newerISO(left,right string) bool {
    if left=="" { return false }
    if right=="" { return true }
    lt,lok:=parseTime(left); rt,rok:=parseTime(right)
    if !lok { return false }
    if !rok { return true }
    return lt.After(rt)
}

func runtimeActivity(project string, rows []Record, timings map[string]map[string]any) map[string]map[string]any {
    runtimeDir:=filepath.Join(project,"_task_mecca",".runtime","agents")
    stat,err:=os.Stat(runtimeDir)
    registryAvailable:=err==nil && stat.IsDir()
    heartbeats:=[]map[string]any{}
    if registryAvailable {
        entries,_:=os.ReadDir(runtimeDir)
        for _,entry:=range entries {
            if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()),".json") { continue }
            data,readErr:=os.ReadFile(filepath.Join(runtimeDir,entry.Name()))
            if readErr!=nil { continue }
            value:=map[string]any{}
            if json.Unmarshal(data,&value)==nil { heartbeats=append(heartbeats,value) }
        }
    }
    byAgent:=map[string]map[string]any{}
    byTask:=map[string]map[string]any{}
    for _,hb:=range heartbeats {
        if agent:=toString(hb["agent"]); agent!="" { byAgent[agent]=hb }
        if task:=strings.ToUpper(toString(hb["task_id"])); task!="" { byTask[task]=hb }
    }
    warn:=1800
    critical:=3600
    if raw:=os.Getenv("TASK_MECCA_STALE_WARN_SECONDS"); raw!="" { if n,e:=strconv.Atoi(raw); e==nil { warn=n } }
    if raw:=os.Getenv("TASK_MECCA_STALE_CRITICAL_SECONDS"); raw!="" { if n,e:=strconv.Atoi(raw); e==nil { critical=n } }
    now:=time.Now()
    out:=map[string]map[string]any{}
    for _,row:=range rows {
        if row.Location!="active" || row.State!="doing" { continue }
        agent:=row.Fields["Agent"]
        hb:=byTask[row.ID]
        if hb==nil { hb=byAgent[agent] }
        source:="file"
        lastAt:=row.Mtime
        runtimeState:="unknown"
        if hb!=nil {
            candidate:=toString(hb["heartbeat_at"])
            if candidate=="" { candidate=toString(hb["updated_at"]) }
            if _,ok:=parseTime(candidate); ok { lastAt=candidate; source="runtime heartbeat" }
            runtimeState=toString(hb["state"]); if runtimeState=="" { runtimeState="running" }
        } else if timing:=timings[row.ID]; timing!=nil && timing["claimed_at"]!=nil {
            gitAt:=toString(timing["claimed_at"])
            if newerISO(gitAt,lastAt) { lastAt=gitAt; source="git lifecycle" }
        }
        var inactivity any=nil
        if dt,ok:=parseTime(lastAt); ok {
            seconds:=now.Sub(dt).Seconds(); if seconds<0 { seconds=0 }; inactivity=seconds
        }
        health:="runtime_unknown"
        if registryAvailable && agent!="" && hb==nil {
            health="worker_missing"
        } else if inactivity!=nil {
            seconds:=inactivity.(float64)
            if seconds>=float64(critical) { health="stale" } else if seconds>=float64(warn) { health="quiet" } else if hb!=nil { health="healthy" }
        }
        var lastValue any=nil; if lastAt!="" { lastValue=lastAt }
        lastSource:="none"; if lastAt!="" { lastSource=source }
        out[row.ID]=map[string]any{
            "health":health,"runtime_registry_available":registryAvailable,
            "runtime_state":runtimeState,"last_activity_at":lastValue,
            "last_activity_source":lastSource,"inactivity_seconds":inactivity,
            "warn_after_seconds":warn,"critical_after_seconds":critical,"agent":agent,
        }
    }
    return out
}

func DashboardSnapshot(project,root string,recentDoneLimit int) (map[string]any,error) {
    rows,err:=Catalog(project,root)
    if err!=nil { return nil,err }
    presence,err:=Presence(project,root,rows)
    if err!=nil { return nil,err }
    readyReport,err:=Ready(project,root)
    if err!=nil { return nil,err }
    hold:=HoldReview(rows)
    timings,err:=LifecycleTimings(project,root)
    if err!=nil { return nil,err }
    activity:=runtimeActivity(project,rows,timings)

    readyIDs:=map[string]bool{}
    blocked:=map[string]map[string]any{}
    if values,ok:=readyReport["ready"].([]map[string]any); ok { for _,x:=range values { readyIDs[toString(x["id"])]=true } }
    if values,ok:=readyReport["blocked"].([]map[string]any); ok { for _,x:=range values { blocked[toString(x["id"])]=x } }
    byID:=map[string]Record{}
    for _,row:=range rows { if _,ok:=byID[row.ID]; !ok { byID[row.ID]=row } }

    continuityMap:=map[string][]map[string]any{}
    for id:=range readyIDs {
        source,ok:=byID[id]; if !ok { continue }
        values,_:=continuity(rows,source)
        if len(values)==0 { continue }
        top:=values[0]["count"].(int)
        picked:=[]map[string]any{}
        for _,entry:=range values { if entry["count"].(int)==top { picked=append(picked,entry) } }
        continuityMap[id]=picked
    }
    reviewByPath:=map[string]map[string]any{}
    for _,key:=range []string{"candidates","waiting"} {
        if values,ok:=hold[key].([]map[string]any); ok { for _,x:=range values { reviewByPath[toString(x["path"])]=x } }
    }

    allItems:=map[string]map[string]any{}
    for id,row:=range byID {
        state:=row.State
        if row.State=="todo" && readyIDs[id] { state="ready" }
        if row.State=="todo" {
            if _,ok:=blocked[id]; ok { state="blocked" }
        }
        waiting:=[]string{}
        if value,ok:=blocked[id]; ok { if v,ok:=value["waiting_for"].([]string); ok { waiting=v } }
        timing:=map[string]any{}; if value,ok:=timings[id]; ok { timing=value }
        item:=dashboardItem(row,state,waiting,timing,continuityMap[id])
        item["hold_review"]=reviewByPath[row.Path]
        if signal,ok:=activity[id]; ok { item["activity"]=signal } else { item["activity"]=map[string]any{"health":"n/a"} }
        allItems[id]=item
    }

    visible:=map[string]bool{}
    stack:=[]string{}
    for id,item:=range allItems {
        fileState:=toString(item["file_state"])
        if item["location"]=="active" && (fileState=="todo"||fileState=="doing"||fileState=="hold") { visible[id]=true; stack=append(stack,id) }
    }
    for len(stack)>0 {
        id:=stack[len(stack)-1]; stack=stack[:len(stack)-1]
        item:=allItems[id]
        for _,dep:=range item["depends_on"].([]string) {
            if _,ok:=allItems[dep]; ok && !visible[dep] { visible[dep]=true; stack=append(stack,dep) }
        }
    }

    completed:=[]map[string]any{}
    for _,item:=range allItems { if item["file_state"]=="done" { completed=append(completed,item) } }
    sort.Slice(completed,func(i,j int)bool {
        li:=toString(completed[i]["completed_at"]); if li=="" { li=toString(completed[i]["mtime"]) }
        lj:=toString(completed[j]["completed_at"]); if lj=="" { lj=toString(completed[j]["mtime"]) }
        if li!=lj { return li>lj }
        return toString(completed[i]["sort_key"])>toString(completed[j]["sort_key"])
    })
    for _,item:=range completed {
        at:=item["completed_at"]; if at==nil || toString(at)=="" { at=item["mtime"] }
        item["completion_sort_at"]=at
    }
    if recentDoneLimit<0 { recentDoneLimit=0 }
    for i:=0;i<len(completed)&&i<recentDoneLimit;i++ { visible[toString(completed[i]["id"])]=true }

    parent:=map[string]string{}
    children:=map[string][]string{}
    for id:=range visible { children[id]=[]string{} }
    for id:=range visible {
        deps:=allItems[id]["depends_on"].([]string)
        first:=""
        for _,dep:=range deps { if visible[dep] && dep!=id { first=dep; break } }
        if first!="" { parent[id]=first; children[first]=append(children[first],id) }
    }
    for _,values:=range children {
        sort.Slice(values,func(i,j int)bool {
            a:=allItems[values[i]]; b:=allItems[values[j]]
            if a["sort_key"]!=b["sort_key"] { return toString(a["sort_key"])<toString(b["sort_key"]) }
            return values[i]<values[j]
        })
    }

    treeRows:=[]map[string]any{}
    visited:=map[string]bool{}
    var appendTree func(string,int,[]string)
    appendTree=func(id string,depth int,ancestors []string) {
        if visited[id] { return }; visited[id]=true
        source:=allItems[id]; item:=map[string]any{}
        for k,v:=range source { item[k]=v }
        item["depth"]=depth; item["ancestors"]=append([]string{},ancestors...); item["has_children"]=len(children[id])>0
        treeRows=append(treeRows,item)
        for _,child:=range children[id] { appendTree(child,depth+1,append(append([]string{},ancestors...),id)) }
    }
    roots:=[]string{}
    for id:=range visible { if _,ok:=parent[id]; !ok { roots=append(roots,id) } }
    sort.Slice(roots,func(i,j int)bool {
        a:=allItems[roots[i]]; b:=allItems[roots[j]]
        if a["sort_key"]!=b["sort_key"] { return toString(a["sort_key"])<toString(b["sort_key"]) }
        return roots[i]<roots[j]
    })
    for _,id:=range roots { appendTree(id,0,nil) }
    rest:=[]string{}
    for id:=range visible { if !visited[id] { rest=append(rest,id) } }
    sort.Strings(rest)
    for _,id:=range rest { appendTree(id,0,nil) }

    doctor,err:=Doctor(project,root,false)
    if err!=nil { return nil,err }
    health,_:=doctor["checks"].(map[string]any)
    if health==nil { health=map[string]any{} }
    health["hold_review"]=hold["candidates"]
    health["runtime_metadata"]=runtimeFindings(rows)
    unrecognized,err:=unrecognizedFiles(project,root)
    if err!=nil { return nil,err }
    workload:=workloadFrom(rows,readyReport,timings)
    selected,err:=Select(project,root)
    if err!=nil { return nil,err }
    if selected=="" { selected=filepath.Join(project,"_task_mecca","data","backlog") }
    access:=AccessObservation(project)
    attention:=[]map[string]any{}
    for id,signal:=range activity {
        healthValue:=toString(signal["health"])
        if healthValue=="quiet"||healthValue=="stale"||healthValue=="worker_missing" {
            row:=map[string]any{"id":id}; for k,v:=range signal { row[k]=v }; attention=append(attention,row)
        }
    }
    sort.Slice(attention,func(i,j int)bool{return toString(attention[i]["id"])<toString(attention[j]["id"])})
    activeIDs:=[]string{}
    for id,item:=range allItems {
        state:=toString(item["file_state"])
        if item["location"]=="active" && (state=="todo"||state=="doing"||state=="hold") { activeIDs=append(activeIDs,id) }
    }
    sort.Strings(activeIDs)
    issueCount:=0; for _,value:=range health { issueCount+=collectionLen(value) }

    return map[string]any{
        "snapshot_at":time.Now().Format("2006-01-02T15:04:05-07:00"),
        "root":selected,"repo":filepath.Base(repoRoot(selected)),
        "backlog_presence":presence,"tree_rows":treeRows,"all_items":allItems,
        "active_ids":activeIDs,"unrecognized_files":unrecognized,"done_items":completed,
        "health":health,"workload":workload,"task_timings":timings,"activity":activity,
        "hold_review":hold,"access":access,"attention":attention,
        "counts":map[string]any{
            "working":countItemsByFileState(allItems,"doing"),"ready":len(readyIDs),
            "blocked":len(blocked),"hold":countItemsByFileState(allItems,"hold"),
            "hold_review":collectionLen(hold["candidates"]),"done":len(completed),
            "issues":issueCount,"attention":len(attention),
        },
    },nil
}

func countItemsByFileState(items map[string]map[string]any,state string) int {
    total:=0
    for _,item:=range items { if item["file_state"]==state { total++ } }
    return total
}
