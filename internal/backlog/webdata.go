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

func summaryPreviewText(value any) string {
 text:=toString(value); runes:=[]rune(text)
 if len(runes)>600 { return string(runes[:600])+"…" }; return text
}
func compactListDocument(row Record) map[string]any {
 doc:=row.Document; out:=map[string]any{}
 for _,key:=range []string{"schema","contract_kind","summary","summary_present"} { if value,ok:=doc[key]; ok { out[key]=value } }
 for _,key:=range []string{"result","notes"} { if value,ok:=doc[key]; ok { out[key]=summaryPreviewText(value) } }
 if req,ok:=doc["requirements"].(map[string]any); ok { out["requirements"]=map[string]any{"goal":summaryPreviewText(req["goal"])} }
 return out
}

func compactListFields(row Record) map[string]string {
    keys:=[]string{"설명","메모","결과","대기","재개조건","Agent","변경범위","등록자","출처"}
    out:=map[string]string{}
    for _,key:=range keys { if value:=row.Fields[key]; value!="" { out[key]=summaryPreviewText(value) } }
    return out
}

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

func webSummaryItem(row Record,state string,waiting []string,reason map[string]any,signal map[string]any) map[string]any {
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
    if len(reason)>0 && toString(reason["audience"]) != "controller" {
        item["attention_reason"]=reason
        item["state"]=effectiveStateFromControl(state,reason)
    }
	applyCompletionReviewItem(item, signal)
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

func webWorkflowStateMaps(project,root string,rows []Record) (map[string]bool,map[string]map[string]any) {
    readyReport:=readyFromRows(project,root,rows)
    readyIDs:=map[string]bool{}
    blocked:=map[string]map[string]any{}
    if values,ok:=readyReport["ready"].([]map[string]any); ok { for _,x:=range values { readyIDs[toString(x["id"])]=true } }
    if values,ok:=readyReport["blocked"].([]map[string]any); ok { for _,x:=range values { blocked[toString(x["id"])]=x } }
    return readyIDs,blocked
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
        "checked_at":time.Now().Format(time.RFC3339Nano),
    },nil
}

// backlogPageRow keeps the minimum information needed to filter and sort.
// The expensive browser-facing document/assignment projection is built only
// after selecting the requested page.
type backlogPageRow struct {
    row Record
    state string
    waiting []string
    reason map[string]any
    signal map[string]any
}

func BacklogPage(project,root string,page,pageSize int,statuses,tags []string,search,sortKey string,projection ...string) (map[string]any,error) {
    rows,err:=CachedCatalog(project,root)
    if err!=nil { return nil,err }
    presence,err:=Presence(project,root,rows)
    if err!=nil { return nil,err }
    readyIDs,blocked:=webWorkflowStateMaps(project,root,rows)
    control:=reconcileControlTower(project,root,rows)
    byID:=preferredRows(rows)

    // Counts and scope still cover every matching backlog item, independently
    // of the chosen status filter and pagination. Controller sensing and
    // notification-event reconciliation remain complete and authoritative.
    counts:=map[string]int{"all":0,"working":0,"ready":0,"blocked":0,"hold":0,"done":0,"attention":0,"needs_action":0}
    q:=strings.ToLower(strings.TrimSpace(search))
    filtered:=make([]backlogPageRow,0,len(byID))
    scopeIDs:=map[string]bool{}
    for id,row:=range byID {
        if len(tags)>0 && !tagsMatchAll(map[string]any{"tags":parseTagList(row.Fields["Tags"])},tags) { continue }
        if q!="" {
            hay:=strings.ToLower(id+"\n"+row.Title+"\n"+row.RawMarkdown+"\n"+row.ArchiveMonth)
            if !strings.Contains(hay,q) { continue }
        }
        scopeIDs[id]=true
        counts["all"]++
        switch row.State {
        case "doing": counts["working"]++
        case "hold": counts["hold"]++
        case "done": counts["done"]++
        }
        state:=row.State
        if row.State=="todo" && readyIDs[id] { state="ready" }
        var waiting []string
        if b,ok:=blocked[id]; ok {
            state="blocked"
            if values,ok:=b["waiting_for"].([]string); ok { waiting=values }
        }
        if state=="ready" { counts["ready"]++ }
        if state=="blocked" { counts["blocked"]++ }
        reason,signal:=control.Attention[id],control.Activity[id]
        // Reuse the same state transformation as webSummaryItem, including
        // controller completion-review semantics, without allocating its
        // document, assignment, fields and other bulky payloads.
        stateView:=map[string]any{"state":state,"file_state":row.State}
        if len(reason)>0 && toString(reason["audience"])!="controller" {
            stateView["state"]=effectiveStateFromControl(state,reason)
        }
        applyCompletionReviewItem(stateView,signal)
        if stateMatches(stateView,statuses) {
            filtered=append(filtered,backlogPageRow{
                row:row,state:state,waiting:waiting,reason:reason,signal:signal,
            })
        }
    }
    cmpUpdated:=func(i,j int)bool {
        li,lj:=filtered[i].row.Mtime,filtered[j].row.Mtime
        if li!=lj { return li<lj }
        return filtered[i].row.SortKey<filtered[j].row.SortKey
    }
    switch sortKey {
    case "updated_asc":
        sort.Slice(filtered,cmpUpdated)
    case "updated_desc":
        sort.Slice(filtered,func(i,j int)bool {
            li,lj:=filtered[i].row.Mtime,filtered[j].row.Mtime
            if li!=lj { return li>lj }
            si,sj:=filtered[i].row.SortKey,filtered[j].row.SortKey
            if si!=sj { return si>sj }
            return filtered[i].row.ID>filtered[j].row.ID
        })
    case "id_asc":
        sort.Slice(filtered,func(i,j int)bool {
            si,sj:=filtered[i].row.SortKey,filtered[j].row.SortKey
            if si!=sj { return si<sj }
            return filtered[i].row.ID<filtered[j].row.ID
        })
    default:
        sort.Slice(filtered,func(i,j int)bool {
            si,sj:=filtered[i].row.SortKey,filtered[j].row.SortKey
            if si!=sj { return si>sj }
            return filtered[i].row.ID>filtered[j].row.ID
        })
    }
    if pageSize<1 { pageSize=20 }; if pageSize>100 { pageSize=100 }; if page<1 { page=1 }
    total:=len(filtered); pages:=(total+pageSize-1)/pageSize; if pages<1 { pages=1 }; if page>pages { page=pages }
    start:=(page-1)*pageSize; end:=start+pageSize; if end>total { end=total }
    items:=[]map[string]any{}
    isSummary:=len(projection)>0 && projection[0]=="summary"
    if !isSummary && start<total {
        items=make([]map[string]any,0,end-start)
        for _,selected:=range filtered[start:end] {
            item:=webSummaryItem(selected.row,selected.state,selected.waiting,selected.reason,selected.signal)
            item["document"]=compactListDocument(selected.row)
            item["fields"]=compactListFields(selected.row)
            items=append(items,item)
        }
    }
    tagCatalog,tagErr:=TagCatalog(project,root,rows); if tagErr!=nil { tagCatalog=map[string]any{} }
    attention,attErr:=AttentionSnapshotFromRows(project,root,rows,true,control); if attErr!=nil { attention=map[string]any{"attention":[]map[string]any{},"all_items":map[string]map[string]any{},"notification_events":[]map[string]any{}} }
    if rowsAtt,ok:=attention["attention"].([]map[string]any); ok {
        needs:=0; for _,row:=range rowsAtt { if !scopeIDs[toString(row["id"])] { continue }; counts["attention"]++; if toString(row["type"])!="quiet" { needs++ } }
        counts["needs_action"]=needs
    }
    if len(projection)>0 && projection[0]=="summary" { items=[]map[string]any{} }
    repoName:=filepath.Base(project)
    if strings.TrimSpace(root)!="" { repoName=filepath.Base(repoRoot(root)) }
    return map[string]any{
        "snapshot_at":time.Now().Format(time.RFC3339Nano),"root":root,"repo":repoName,
        "items":items,"page":page,"page_size":pageSize,"pages":pages,"total":total,
        "revision":backlogRevisionFromRows(rows),"backlog_presence":presence,
        "counts":counts,"tag_catalog":tagCatalog,"access":AccessObservation(project),
        "attention":attention["attention"],"attention_items":pagedNotificationItems(attention["all_items"]),"notification_events":attention["notification_events"],
    },nil
}

// TaskDetailTimings identifies which server-side phase is expensive. The
// timings are request-local: no mutable global counters or background writes.
type TaskDetailTimings struct {
 CatalogMS float64
 ReadinessMS float64
 ControlMS float64
 ProjectionMS float64
}

func TaskDetail(project,root,id string) (map[string]any,error) {
 item,_,err:=TaskDetailWithTimings(project,root,id)
 return item,err
}

func TaskDetailWithTimings(project,root,id string) (map[string]any,TaskDetailTimings,error) {
    var profile TaskDetailTimings
    catalogStart:=time.Now()
    rows,err:=CachedCatalog(project,root)
    profile.CatalogMS=time.Since(catalogStart).Seconds()*1000
    if err!=nil { return nil,profile,err }
    byID:=preferredRows(rows); row,ok:=byID[strings.ToUpper(strings.TrimSpace(id))]
    if !ok { return nil,profile,fmt.Errorf("task not found: %s",id) }
    state:=row.State; waiting:=[]string{}
    if row.State=="todo" {
        readinessStart:=time.Now()
        readyIDs,blocked:=webWorkflowStateMaps(project,root,rows)
        if readyIDs[row.ID] { state="ready" }
        if b,ok:=blocked[row.ID]; ok { state="blocked"; if v,ok:=b["waiting_for"].([]string); ok { waiting=v } }
        profile.ReadinessMS=time.Since(readinessStart).Seconds()*1000
    }
    controlStart:=time.Now()
    control:=reconcileControlTower(project,root,rows)
    profile.ControlMS=time.Since(controlStart).Seconds()*1000
    projectionStart:=time.Now()
    timing:=map[string]any{}
    if v,ok:=control.Timings[row.ID]; ok { timing=v }
    item:=dashboardItem(row,state,waiting,timing,nil)
    if signal,ok:=control.Activity[row.ID]; ok { item["activity"]=signal } else { item["activity"]=map[string]any{"health":"n/a"} }
    reason:=control.Attention[row.ID]
    if len(reason)>0 {
        item["attention_reason"]=reason
        item["state"]=effectiveStateFromControl(state,reason)
    }
    profile.ProjectionMS=time.Since(projectionStart).Seconds()*1000
    return item,profile,nil
}

func AttentionSnapshotFromRows(project,root string,rows []Record,reconcile bool,shared ...controlTowerSnapshot) (map[string]any,error) {
    var control controlTowerSnapshot
 if len(shared)>0 { control=shared[0] } else { control=reconcileControlTower(project,root,rows) }
    byID:=preferredRows(rows)
    allItems:=map[string]map[string]any{}
    attention:=[]map[string]any{}
	controllerReviews := []map[string]any{}
	canonical:=map[string]map[string]any{}

    for id,row:=range byID {
        reason:=control.Attention[id]
        signal:=control.Activity[id]
        if row.Location=="active" && (row.State=="doing" || row.State=="hold") {
            item:=webSummaryItem(row,row.State,nil,reason,signal)
            if review, ok := item["completion_review"].(map[string]any); ok {
				entry := map[string]any{"id": id}
				for k, v := range review {
					entry[k] = v
				}
				controllerReviews = append(controllerReviews, entry)
				allItems[id] = item
			}
			if lifecycle,ok:=control.Timings[id]; ok { item["lifecycle"]=lifecycle }
            if len(reason)>0 && toString(reason["audience"]) != "controller" {
                rowAtt:=map[string]any{"id":id}
                for k,v:=range reason { rowAtt[k]=v }
                if signal!=nil { for k,v:=range signal { if _,exists:=rowAtt[k]; !exists { rowAtt[k]=v } } }
                attention=append(attention,rowAtt); allItems[id]=item
            } else if signal!=nil && toString(signal["health"])=="quiet" {
                rowAtt:=map[string]any{"id":id,"type":"quiet","severity":"info","title":"활동 감소"}
                for k,v:=range signal { rowAtt[k]=v }
                attention=append(attention,rowAtt); allItems[id]=item
            }
        }

        item:=map[string]any{"id":id,"title":row.Title,"file_state":row.State,"state":effectiveStateFromControl(row.State,reason),"updated_at":row.Mtime,"mtime":row.Mtime,"completed_at":nil}
        if lifecycle,ok:=control.Timings[id]; ok {
            item["lifecycle"]=lifecycle
            item["completed_at"]=lifecycle["completed_at"]
        }
        if condition:=control.NotificationCondition[id]; len(condition)>0 { item["notification_condition"]=condition }
        canonical[id]=item
    }
    sort.Slice(attention,func(i,j int)bool{return toString(attention[i]["id"])<toString(attention[j]["id"])})

    var events []map[string]any
    var err error
    if reconcile { events,err=NotificationEvents(project,canonical) } else { events,err=ReadNotificationEvents(project) }
    if err!=nil { events=[]map[string]any{} }
	filtered := []map[string]any{}
	for _, event := range events {
		kind := toString(event["kind"])
		reason := toString(event["reason_type"])
		id := strings.ToUpper(toString(event["task_id"]))
		row, exists := byID[id]
		if kind == "finalize" || reason == "completion_pending" || reason == "controller_completion_review" {
			continue
		}
		if kind == "completed" && (!exists || row.State != "done") {
			continue
		}
		filtered = append(filtered, event)
	}
	events = filtered
	for _,event:=range events {
        id:=strings.ToUpper(toString(event["task_id"])); if id=="" { continue }
        if _,exists:=allItems[id]; exists { continue }
        if row,ok:=byID[id]; ok { item:=webSummaryItem(row,row.State,nil,control.Attention[id],control.Activity[id]); if lifecycle,ok:=control.Timings[id]; ok { item["lifecycle"]=lifecycle }; allItems[id]=item }
    }
    return map[string]any{"snapshot_at":time.Now().Format(time.RFC3339Nano),"attention":attention, "content_revision":backlogRevisionFromRows(rows),"controller_reviews": controllerReviews,"all_items":allItems,"notification_events":events,"counts":map[string]any{"attention":len(attention)}},nil
}

func AttentionSnapshot(project,root string,reconcile bool) (map[string]any,error) {
    rows,err:=CachedCatalog(project,root); if err!=nil { return nil,err }
    return AttentionSnapshotFromRows(project,root,rows,reconcile)
}

// List notifications need state identity and actionable reasons, not every
// completed document's requirements/notes. Full detail and attention APIs remain
// available independently.
func pagedNotificationItems(value any) map[string]map[string]any {
 out:=map[string]map[string]any{}
 items,_:=value.(map[string]map[string]any)
 for id,item:=range items {
  small:=map[string]any{}
  for _,key:=range []string{"id","title","state","file_state","mtime","updated_at","completed_at","attention_reason","activity","completion_review"} { if v,ok:=item[key];ok { small[key]=v } }
  if _,ok:=item["attention_reason"];ok { if fields,ok:=item["fields"].(map[string]string);ok {small["fields"]=compactListFields(Record{Fields:fields})};if doc,ok:=item["document"].(map[string]any);ok {small["document"]=compactListDocument(Record{Document:doc})} }
  out[id]=small
 }
 return out
}
