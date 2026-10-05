package backlog

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "path/filepath"
    "sort"
    "strings"
    "time"
)

func compactDocument(row Record) map[string]any {
    doc:=row.Document
    out:=map[string]any{}
    for _,key:=range []string{"schema","contract_kind","summary","summary_present","result","notes","requirements"} {
        if value,ok:=doc[key]; ok { out[key]=value }
    }
    return out
}

func compactFields(row Record) map[string]string {
    keys:=[]string{"설명","메모","결과","대기","재개조건","Agent","변경범위","등록자","출처"}
    out:=map[string]string{}
    for _,key:=range keys { if value:=row.Fields[key]; value!="" { out[key]=value } }
    return out
}

func webAttentionReason(row Record, review map[string]any, signal map[string]any) map[string]any {
    reason:=map[string]any{}
    if row.State=="hold" && review!=nil && toString(review["wait_kind"])=="user" {
        reason=map[string]any{
            "type":"user_intervention","severity":"danger","title":"사용자 개입 필요",
            "message":firstNonEmpty(toString(review["wait_note"]),"사용자 입력 또는 판단을 기다리고 있습니다."),
            "resume_condition":toString(review["resume_condition"]),"evidence":toString(review["wait_evidence"]),
        }
    }
    if signal!=nil {
        switch toString(signal["health"]) {
        case "awaiting_finalize":
            reason=map[string]any{"type":"completion_pending","severity":"warning","title":"완료 처리 필요","message":"워커 런타임은 작업 완료를 보고했지만 백로그는 아직 doing 상태입니다.","resume_condition":"결과와 검증을 확인한 뒤 태스크를 done으로 완료 처리하세요."}
        case "needs_user":
            reason=map[string]any{"type":"user_intervention","severity":"danger","title":"사용자 개입 필요","message":"워커 런타임이 사용자 입력 또는 조치를 기다리고 있습니다.","resume_condition":"필요한 사용자 판단 또는 입력을 제공한 뒤 작업을 재개하세요."}
        case "stale","worker_missing":
            reason=map[string]any{"type":"runtime_stalled","severity":"warning","title":"작업 정체 확인 필요","message":"진행 중 태스크의 런타임 활동이 중단되었거나 할당 워커를 찾을 수 없습니다.","resume_condition":"워커 상태와 남은 작업을 확인하고 재할당 또는 완료 처리 여부를 결정하세요."}
        case "execution_interrupted":
            reason=map[string]any{"type":"execution_interrupted","severity":"danger","title":"실행 복구 필요","message":"백로그는 doing 상태이지만 연결된 실행 attempt가 정상 완료되지 않고 종료되었습니다.","resume_condition":"실행 결과와 남은 작업을 확인한 뒤 재할당 또는 상태 정리를 수행하세요."}
        case "runtime_unknown":
            reason=map[string]any{"type":"runtime_unknown","severity":"warning","title":"실행 상태 확인 필요","message":"백로그에 연결된 실행 attempt의 현재 상태를 신뢰성 있게 확인할 수 없습니다.","resume_condition":"런타임과 hook 상태를 확인하고 필요하면 작업을 재개하세요."}
        }
    }
    return reason
}

func webSummaryItem(row Record,state string,waiting []string,review map[string]any,signal map[string]any) map[string]any {
    assignment:=assignmentView(row)
    item:=map[string]any{
        "id":row.ID,"sort_key":row.SortKey,"title":row.Title,"state":state,"file_state":row.State,
        "location":row.Location,"archive_month":row.ArchiveMonth,"path":row.Path,"mtime":row.Mtime,"updated_at":row.Mtime,
        "agent":assignment["agent"],"assignment_kind":assignment["assignment_kind"],"scope":assignment["change_scope"],
        "tags":parseTagList(row.Fields["Tags"]),"depends_on":refs(row.Fields["선행"]),"related":refs(row.Fields["연관"]),"waiting_for":waiting,
        "document":compactDocument(row),"fields":compactFields(row),"registrant":row.Fields["등록자"],"source":sourceFromFields(row.Fields),
        "activity":map[string]any{"health":"n/a"},
    }
    if signal!=nil { item["activity"]=signal }
    reason:=webAttentionReason(row,review,signal)
    if len(reason)>0 {
        item["attention_reason"]=reason
        switch toString(reason["type"]) {
        case "completion_pending": item["state"]="awaiting_finalize"
        case "user_intervention": item["state"]="needs_user"
        case "runtime_stalled","execution_interrupted","runtime_unknown": item["state"]="stalled"
        }
    }
    return item
}

func preferredRows(rows []Record) map[string]Record {
    byID:=map[string]Record{}
    for _,row:=range rows {
        existing,ok:=byID[row.ID]
        if !ok || (row.Location=="active" && existing.Location!="active") { byID[row.ID]=row }
    }
    return byID
}

func webStateMaps(project,root string,rows []Record) (map[string]bool,map[string]map[string]any,map[string]map[string]any,map[string]map[string]any,map[string]any) {
    readyReport:=readyFromRows(project,root,rows)
    readyIDs:=map[string]bool{}
    blocked:=map[string]map[string]any{}
    if values,ok:=readyReport["ready"].([]map[string]any); ok { for _,x:=range values { readyIDs[toString(x["id"])]=true } }
    if values,ok:=readyReport["blocked"].([]map[string]any); ok { for _,x:=range values { blocked[toString(x["id"])]=x } }
    hold:=HoldReview(rows)
    reviewByPath:=map[string]map[string]any{}
    for _,key:=range []string{"candidates","waiting"} {
        if values,ok:=hold[key].([]map[string]any); ok { for _,x:=range values { reviewByPath[toString(x["path"])]=x } }
    }
    activity:=runtimeActivity(project,rows,map[string]map[string]any{})
    activity=mergeRuntimeSignals(activity,runtimeLedgerSignals(project,rows,time.Now()))
    return readyIDs,blocked,reviewByPath,activity,hold
}

func webAttentionMaps(project string,rows []Record) (map[string]map[string]any,map[string]map[string]any) {
    hold:=HoldReview(rows)
    reviewByPath:=map[string]map[string]any{}
    for _,key:=range []string{"candidates","waiting"} {
        if values,ok:=hold[key].([]map[string]any); ok {
            for _,x:=range values { reviewByPath[toString(x["path"])]=x }
        }
    }
    activity:=runtimeActivity(project,rows,map[string]map[string]any{})
    activity=mergeRuntimeSignals(activity,runtimeLedgerSignals(project,rows,time.Now()))
    return reviewByPath,activity
}

func stateMatches(item map[string]any,statuses []string) bool {
    if len(statuses)==0 { return true }
    state:=toString(item["state"]); fileState:=toString(item["file_state"])
    for _,status:=range statuses {
        switch status {
        case "ready": if state=="ready" { return true }
        case "doing": if state=="doing" || fileState=="doing" { return true }
        case "hold": if state=="hold" || fileState=="hold" { return true }
        case "blocked": if state=="blocked" { return true }
        case "done": if state=="done" || fileState=="done" { return true }
        }
    }
    return false
}

func tagsMatchAll(item map[string]any,tags []string) bool {
    if len(tags)==0 { return true }
    present:=map[string]bool{}
    if values,ok:=item["tags"].([]string); ok { for _,tag:=range values { present[tag]=true } }
    for _,tag:=range tags { if !present[tag] { return false } }
    return true
}

func parseFilterList(value string) []string {
    out:=[]string{}; seen:=map[string]bool{}
    for _,part:=range strings.Split(value,",") {
        part=strings.TrimSpace(part); if part=="" || seen[part] { continue }; seen[part]=true; out=append(out,part)
    }
    return out
}

func backlogRevisionFromRows(rows []Record) string {
    ordered:=append([]Record{},rows...)
    sort.Slice(ordered,func(i,j int)bool{return ordered[i].Path<ordered[j].Path})
    h:=sha256.New()
    for _,row:=range ordered {
        _,_=h.Write([]byte(row.Path))
        _,_=h.Write([]byte{0})
        _,_=h.Write([]byte(row.State))
        _,_=h.Write([]byte{0})
        _,_=h.Write([]byte(row.RawMarkdown))
        _,_=h.Write([]byte{0})
    }
    return hex.EncodeToString(h.Sum(nil))[:20]
}

func BacklogRevision(project,root string) (map[string]any,error) {
    rows,err:=CachedCatalog(project,root)
    if err!=nil { return nil,err }
    byID:=preferredRows(rows)
    states:=make([]map[string]any,0,len(byID))
    for id,row:=range byID {
        states=append(states,map[string]any{"id":id,"title":row.Title,"file_state":row.State})
    }
    sort.Slice(states,func(i,j int)bool{return toString(states[i]["id"])<toString(states[j]["id"])})
    return map[string]any{
        "revision":backlogRevisionFromRows(rows),
        "count":len(rows),
        "states":states,
        "checked_at":time.Now().Format(time.RFC3339),
    },nil
}

func BacklogPage(project,root string,page,pageSize int,statuses,tags []string,search,sortKey string) (map[string]any,error) {
    rows,err:=CachedCatalog(project,root)
    if err!=nil { return nil,err }
    presence,err:=Presence(project,root,rows)
    if err!=nil { return nil,err }
    readyIDs,blocked,reviewByPath,activity,_:=webStateMaps(project,root,rows)
    byID:=preferredRows(rows)
    all:=make([]map[string]any,0,len(byID))
    counts:=map[string]int{"all":len(byID),"working":0,"ready":0,"blocked":0,"hold":0,"done":0,"attention":0,"needs_action":0}
    for id,row:=range byID {
        state:=row.State
        if row.State=="todo" && readyIDs[id] { state="ready" }
        waiting:=[]string{}
        if b,ok:=blocked[id]; ok { state="blocked"; if v,ok:=b["waiting_for"].([]string); ok { waiting=v } }
        item:=webSummaryItem(row,state,waiting,reviewByPath[row.Path],activity[id])
        switch row.State { case "doing": counts["working"]++; case "hold": counts["hold"]++; case "done": counts["done"]++ }
        if state=="ready" { counts["ready"]++ }; if state=="blocked" { counts["blocked"]++ }
        if reason,ok:=item["attention_reason"].(map[string]any); ok && len(reason)>0 { counts["attention"]++; counts["needs_action"]++ }
        all=append(all,item)
    }
    q:=strings.ToLower(strings.TrimSpace(search))
    filtered:=make([]map[string]any,0,len(all))
    for _,item:=range all {
        if !stateMatches(item,statuses) || !tagsMatchAll(item,tags) { continue }
        if q!="" {
            id:=toString(item["id"]); row:=byID[id]
            hay:=strings.ToLower(id+"\n"+row.Title+"\n"+row.RawMarkdown+"\n"+row.ArchiveMonth)
            if !strings.Contains(hay,q) { continue }
        }
        filtered=append(filtered,item)
    }
    cmpUpdated:=func(i,j int)bool {
        li:=toString(filtered[i]["updated_at"]); lj:=toString(filtered[j]["updated_at"])
        if li!=lj { return li<lj }; return toString(filtered[i]["sort_key"])<toString(filtered[j]["sort_key"])
    }
    switch sortKey {
    case "updated_asc":
        sort.Slice(filtered,cmpUpdated)
    case "updated_desc":
        sort.Slice(filtered,func(i,j int)bool {
            li:=toString(filtered[i]["updated_at"]); lj:=toString(filtered[j]["updated_at"])
            if li!=lj { return li>lj }
            si:=toString(filtered[i]["sort_key"]); sj:=toString(filtered[j]["sort_key"])
            if si!=sj { return si>sj }
            return toString(filtered[i]["id"])>toString(filtered[j]["id"])
        })
    case "id_asc":
        sort.Slice(filtered,func(i,j int)bool {
            si:=toString(filtered[i]["sort_key"]); sj:=toString(filtered[j]["sort_key"])
            if si!=sj { return si<sj }
            return toString(filtered[i]["id"])<toString(filtered[j]["id"])
        })
    default:
        sort.Slice(filtered,func(i,j int)bool {
            si:=toString(filtered[i]["sort_key"]); sj:=toString(filtered[j]["sort_key"])
            if si!=sj { return si>sj }
            return toString(filtered[i]["id"])>toString(filtered[j]["id"])
        })
    }
    if pageSize<1 { pageSize=20 }; if pageSize>100 { pageSize=100 }; if page<1 { page=1 }
    total:=len(filtered); pages:=(total+pageSize-1)/pageSize; if pages<1 { pages=1 }; if page>pages { page=pages }
    start:=(page-1)*pageSize; end:=start+pageSize; if end>total { end=total }
    items:=[]map[string]any{}; if start<total { items=filtered[start:end] }
    tagCatalog,tagErr:=TagCatalog(project,root,rows); if tagErr!=nil { tagCatalog=map[string]any{} }
    attention,attErr:=AttentionSnapshotFromRows(project,root,rows,true); if attErr!=nil { attention=map[string]any{"attention":[]map[string]any{},"all_items":map[string]map[string]any{},"notification_events":[]map[string]any{}} }
    if rowsAtt,ok:=attention["attention"].([]map[string]any); ok {
        counts["attention"]=len(rowsAtt)
        needs:=0; for _,row:=range rowsAtt { if toString(row["type"])!="quiet" { needs++ } }
        counts["needs_action"]=needs
    }
    repoName:=filepath.Base(project)
    if strings.TrimSpace(root)!="" { repoName=filepath.Base(repoRoot(root)) }
    return map[string]any{
        "snapshot_at":time.Now().Format(time.RFC3339),"root":root,"repo":repoName,
        "items":items,"page":page,"page_size":pageSize,"pages":pages,"total":total,
        "revision":backlogRevisionFromRows(rows),"backlog_presence":presence,
        "counts":counts,"tag_catalog":tagCatalog,"access":AccessObservation(project),
        "attention":attention["attention"],"attention_items":attention["all_items"],"notification_events":attention["notification_events"],
    },nil
}

func TaskDetail(project,root,id string) (map[string]any,error) {
    rows,err:=CachedCatalog(project,root); if err!=nil { return nil,err }
    byID:=preferredRows(rows); row,ok:=byID[strings.ToUpper(strings.TrimSpace(id))]; if !ok { return nil,fmt.Errorf("task not found: %s",id) }
    readyIDs,blocked,reviewByPath,_,_:=webStateMaps(project,root,rows)
    timings,timingErr:=lifecycleTimings(project,root,rows); if timingErr!=nil { timings=map[string]map[string]any{} }
    activity:=runtimeActivity(project,rows,timings)
    state:=row.State; waiting:=[]string{}
    if row.State=="todo" && readyIDs[row.ID] { state="ready" }
    if b,ok:=blocked[row.ID]; ok { state="blocked"; if v,ok:=b["waiting_for"].([]string); ok { waiting=v } }
    timing:=map[string]any{}; if v,ok:=timings[row.ID]; ok { timing=v }
    item:=dashboardItem(row,state,waiting,timing,nil)
    if signal,ok:=activity[row.ID]; ok { item["activity"]=signal } else { item["activity"]=map[string]any{"health":"n/a"} }
    reason:=webAttentionReason(row,reviewByPath[row.Path],activity[row.ID])
    if len(reason)>0 {
        item["attention_reason"]=reason
        switch toString(reason["type"]) { case "completion_pending": item["state"]="awaiting_finalize"; case "user_intervention": item["state"]="needs_user"; case "runtime_stalled": item["state"]="stalled" }
    }
    return item,nil
}

func AttentionSnapshotFromRows(project,root string,rows []Record,reconcile bool) (map[string]any,error) {
    reviewByPath,activity:=webAttentionMaps(project,rows)
    byID:=preferredRows(rows)
    allItems:=map[string]map[string]any{}
    attention:=[]map[string]any{}
    for id,row:=range byID {
        if row.Location!="active" || (row.State!="doing" && row.State!="hold") { continue }
        item:=webSummaryItem(row,row.State,nil,reviewByPath[row.Path],activity[id])
        reason,_:=item["attention_reason"].(map[string]any)
        if len(reason)==0 {
            if signal:=activity[id]; signal!=nil && toString(signal["health"])=="quiet" {
                rowAtt:=map[string]any{"id":id,"type":"quiet","severity":"info","title":"활동 감소"}
                for k,v:=range signal { rowAtt[k]=v }; attention=append(attention,rowAtt); allItems[id]=item
            }
            continue
        }
        rowAtt:=map[string]any{"id":id}; for k,v:=range reason { rowAtt[k]=v }; if signal:=activity[id]; signal!=nil { for k,v:=range signal { if _,exists:=rowAtt[k]; !exists { rowAtt[k]=v } } }
        attention=append(attention,rowAtt); allItems[id]=item
    }
    sort.Slice(attention,func(i,j int)bool{return toString(attention[i]["id"])<toString(attention[j]["id"])})
    lightweight:=map[string]map[string]any{}
    timings,timingErr:=lifecycleTimings(project,root,rows); if timingErr!=nil { timings=map[string]map[string]any{} }\n    for id,row:=range byID {\n        item:=map[string]any{"id":id,"title":row.Title,"file_state":row.State,"state":row.State,"updated_at":row.Mtime,"mtime":row.Mtime,"completed_at":nil}\n        if lifecycle,ok:=timings[id]; ok { item["lifecycle"]=lifecycle; item["completed_at"]=lifecycle["completed_at"] }\n        lightweight[id]=item\n    }
    var events []map[string]any
    var err error
    if reconcile { events,err=NotificationEvents(project,lightweight) } else { events,err=ReadNotificationEvents(project) }
    if err!=nil { events=[]map[string]any{} }
    for _,event:=range events {
        id:=strings.ToUpper(toString(event["task_id"])); if id=="" { continue }
        if _,exists:=allItems[id]; exists { continue }
        if row,ok:=byID[id]; ok { allItems[id]=webSummaryItem(row,row.State,nil,nil,nil) }
    }
    return map[string]any{"snapshot_at":time.Now().Format(time.RFC3339),"attention":attention,"all_items":allItems,"notification_events":events,"counts":map[string]any{"attention":len(attention)}},nil
}

func AttentionSnapshot(project,root string,reconcile bool) (map[string]any,error) {
    rows,err:=CachedCatalog(project,root); if err!=nil { return nil,err }
    return AttentionSnapshotFromRows(project,root,rows,reconcile)
}
