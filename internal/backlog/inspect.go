package backlog

import (
    "strings"
)

func Inspect(project,root,itemID string) (map[string]any,error) {
    wanted:=strings.ToUpper(itemID)
    rows,err:=Catalog(project,root)
    if err!=nil { return nil,err }
    matches:=[]Record{}
    for _,row:=range rows { if row.ID==wanted { matches=append(matches,row) } }
    if len(matches)==0 {
        return map[string]any{"exists":false,"id":wanted,"duplicate_count":0},nil
    }
    row:=matches[0]
    grouped:=groupedByID(rows)
    blockers:=claimBlockers(row.Fields,grouped)
    reviews:=HoldReview(rows)
    var holdReview any=nil
    for _,key:=range []string{"candidates","waiting"} {
        if list,ok:=reviews[key].([]map[string]any); ok {
            for _,review:=range list {
                if review["path"]==row.Path { holdReview=review; break }
            }
        }
        if holdReview!=nil { break }
    }
    continuityAgents,legacy:=continuity(rows,row)
    timings,timingErr:=LifecycleTimings(project,root)
    if timingErr!=nil { return nil,timingErr }
    lifecycle:=map[string]any{}
    if value,ok:=timings[wanted]; ok { lifecycle=value }
    assignment:=assignmentView(row)
    duplicatePaths:=[]string{}
    for _,match:=range matches { duplicatePaths=append(duplicatePaths,match.Path) }
    ready:=row.Location=="active" && row.State=="todo"
    if blocked,ok:=blockers["blocked_by"].([]string); ok && len(blocked)>0 { ready=false }
    report:=map[string]any{
        "exists":true,
        "id":wanted,
        "duplicate_count":len(matches),
        "duplicate_paths":duplicatePaths,
        "state":row.State,
        "location":row.Location,
        "title":row.Title,
        "path":row.Path,
        "ready":ready,
        "hold_review":holdReview,
        "related":refs(row.Fields["연관"]),
        "agent":assignment["agent"],
        "change_scope":assignment["change_scope"],
        "assignment_kind":assignment["assignment_kind"],
        "hold_audit":assignment["hold_audit"],
        "runtime_metadata":runtimeFromFields(row.Fields),
        "registrant":row.Fields["등록자"],
        "source":sourceFromFields(row.Fields),
        "continuity_agents":continuityAgents,
        "legacy_agent_history":legacy,
        "lifecycle":lifecycle,
        "document":row.Document,
        "raw_markdown":row.RawMarkdown,
        "fields":row.Fields,
    }
    for key,value:=range blockers { report[key]=value }
    return report,nil
}
