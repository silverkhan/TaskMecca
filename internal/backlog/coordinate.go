package backlog

import (
    "path/filepath"
    "regexp"
    "sort"
    "strings"
    "time"
)

var scopeSplit = regexp.MustCompile("[,;\\n]+")

func scopeTokens(value string) []string {
    value=strings.TrimSpace(value)
    if value=="" || value=="읽기 전용" || value=="read-only" || value=="readonly" { return nil }
    seen:=map[string]bool{}
    for _,raw:=range scopeSplit.Split(value,-1) {
        token:=strings.Trim(strings.TrimSpace(raw),"`")
        token=strings.ReplaceAll(token,"\\","/")
        if token=="" || token=="-" { continue }
        wildcard:=-1
        for _,needle:=range []string{"*","?"} {
            if i:=strings.Index(token,needle); i>=0 && (wildcard<0 || i<wildcard) { wildcard=i }
        }
        if wildcard>=0 { token=strings.TrimRight(token[:wildcard],"/") }
        token=strings.TrimPrefix(token,"./")
        token=strings.TrimRight(token,"/")
        token=strings.ToLower(token)
        if token!="" { seen[token]=true }
    }
    out:=[]string{}
    for token:=range seen { out=append(out,token) }
    sort.Strings(out)
    return out
}

func scopeOverlap(left,right string) bool {
    return left==right || strings.HasPrefix(left,right+"/") || strings.HasPrefix(right,left+"/")
}

func ScopeConflicts(rows []Record) []map[string]any {
    type activeScope struct { row Record; scopes []string }
    active:=[]activeScope{}
    for _,row:=range rows {
        if row.Location!="active" || row.State!="doing" { continue }
        scopes:=scopeTokens(row.Fields["변경범위"])
        if len(scopes)>0 { active=append(active,activeScope{row,scopes}) }
    }
    conflicts:=[]map[string]any{}
    for i,left:=range active {
        for _,right:=range active[i+1:] {
            overlapsSet:=map[string]bool{}
            for _,ls:=range left.scopes {
                for _,rs:=range right.scopes {
                    if scopeOverlap(ls,rs) { overlapsSet[ls+" <> "+rs]=true }
                }
            }
            if len(overlapsSet)==0 { continue }
            overlaps:=[]string{}
            for value:=range overlapsSet { overlaps=append(overlaps,value) }
            sort.Strings(overlaps)
            conflicts=append(conflicts,map[string]any{
                "left":left.row.ID,"right":right.row.ID,"overlaps":overlaps,
            })
        }
    }
    return conflicts
}

func Coordinate(project,root string,workerCap int) (map[string]any,error) {
    rows,err:=Catalog(project,root)
    if err!=nil { return nil,err }
    readyReport,err:=Ready(project,root)
    if err!=nil { return nil,err }
    timings,err:=LifecycleTimings(project,root)
    if err!=nil { return nil,err }

    doing:=[]map[string]any{}
    for _,row:=range rows {
        if row.Location!="active" || row.State!="doing" { continue }
        lifecycle:=map[string]any{}
        if value,ok:=timings[row.ID]; ok { lifecycle=value }
        elapsed:="-"
        var elapsedSeconds any=nil
        if value,ok:=lifecycle["elapsed"].(string); ok { elapsed=value }
        if value,ok:=lifecycle["elapsed_seconds"]; ok { elapsedSeconds=value }
        doing=append(doing,map[string]any{
            "id":row.ID,
            "title":row.Title,
            "agent":row.Fields["Agent"],
            "change_scope":row.Fields["변경범위"],
            "runtime_metadata":runtimeFromFields(row.Fields),
            "elapsed":elapsed,
            "elapsed_seconds":elapsedSeconds,
            "lifecycle":lifecycle,
        })
    }
    enrich:=func(value any) []map[string]any {
        source,ok:=value.([]map[string]any)
        if !ok { return []map[string]any{} }
        out:=[]map[string]any{}
        for _,row:=range source {
            item:=map[string]any{}
            for key,v:=range row { item[key]=v }
            lifecycle:=map[string]any{}
            if id,ok:=item["id"].(string); ok {
                if value,exists:=timings[id]; exists { lifecycle=value }
            }
            item["lifecycle"]=lifecycle
            out=append(out,item)
        }
        return out
    }
    readyItems:=enrich(readyReport["ready"])
    blockedItems:=enrich(readyReport["blocked"])
    if workerCap<1 { workerCap=1 }
    activeCount:=len(doing)
    candidateSlots:=workerCap-activeCount
    if candidateSlots<0 { candidateSlots=0 }
    readyToReview:=len(readyItems)
    if readyToReview>candidateSlots { readyToReview=candidateSlots }
    holdReview:=HoldReview(rows)
    base:=root
    if base=="" { base=filepath.Join(project,"_task_mecca") }
    if absolute,absErr:=filepath.Abs(base); absErr==nil { base=absolute }

    return map[string]any{
        "snapshot_at":time.Now().Format("2006-01-02T15:04:05.999999999-07:00"),
        "source":"git_backlog",
        "root":base,
        "scheduling_needed":len(readyItems)>0,
        "controller_review_needed":holdReview["review_needed"],
        "hold_review":holdReview,
        "ready":readyItems,
        "blocked":blockedItems,
        "doing":doing,
        "workload":workloadFrom(rows,readyReport,timings),
        "scope_conflicts":ScopeConflicts(rows),
        "parallel_fill":map[string]any{
            "implementation_worker_cap":workerCap,
            "active_doing":activeCount,
            "candidate_slots":candidateSlots,
            "ready_count":len(readyItems),
            "ready_to_review_this_pass":readyToReview,
            "review_required":readyToReview>0,
            "meaning":"inspect/allocate up to this many ready tasks before waiting; final dispatch still requires live-state and scope checks",
        },
        "worker_naming":map[string]any{
            "prefix":"/root/controller/",
            "pool":append([]string{},workerPool...),
            "task_derived_names_forbidden":true,
            "allocator":"worker-name --used <live worker path/alias> ... --json",
            "new_validation":"agent <path> --new --json",
            "reuse_validation":"agent <existing-path> --json; preserve actual identity",
        },
        "contract":map[string]any{
            "fresh_snapshot":true,
            "live_agent_state_source":"runtime list_agents; not inferred from Git",
            "worker_selection_is_controller_judgment":true,
            "adaptive_worker_allocation":true,
            "parallel_fill_invariant":"do not wait after the first assignment while another independent ready task and an implementation slot remain",
            "batch_before_wait":"build and dispatch the full safe allocation batch for this pass before wait_agent",
            "continuity_wait_exception":"waiting for a busy preferred worker requires strong continuity plus concrete near-term completion evidence; vague 'soon' is insufficient",
            "prefer_continuity_then_avoid_unnecessary_wait":true,
            "controller_drain_invariant":"after DONE/BLOCKED, refresh and refill freed slots until no actionable ready and no active workers remain",
            "spawn_proxy":"if controller cannot spawn in this runtime, /root executes controller's batch spawn request without rejudging",
            "dispatch_requires_recheck":"inspect <ID> --json immediately before doing/dispatch",
            "no_work_statement_requires":"this turn's coordinate + live agent state",
            "individual_completion":"settle original acceptance independently of queue drain; worker DONE alone is not proof",
            "hold_review_is_advisory":"review this event, not automatic readiness/spawn or a command to keep draining unchanged external waits",
        },
    },nil
}
