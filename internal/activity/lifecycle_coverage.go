package activity

import (
    "sort"
    "strings"
    "time"
)

// LifecycleFact is a durable, independently reported backlog transition.
// It is intentionally not an Activity Event: a Worker report of "waiting"
// is not an observation of CPU activity or proof of Task Mecca overhead.
type LifecycleFact struct {
    EventID string `json:"event_id"`
    TaskID string `json:"task_id"`
    Kind string `json:"kind"`
    OccurredAt time.Time `json:"occurred_at"`
    EvidenceSource string `json:"evidence_source,omitempty"`
}

type WaitInterval struct {
    Kind string `json:"kind"`
    StartedAt time.Time `json:"started_at"`
    EndedAt time.Time `json:"ended_at"`
    DurationMillis int64 `json:"duration_ms"`
    FromEventID string `json:"from_event_id"`
    ToEventID string `json:"to_event_id"`
}

type LifecycleCoverage struct {
    TaskID string `json:"task_id"`
    Source string `json:"source"`
    RegisteredAt *time.Time `json:"registered_at,omitempty"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
    TotalElapsedMillis *int64 `json:"total_elapsed_ms,omitempty"`
    RegistrationQueueMillis int64 `json:"registration_queue_ms"`
    DispatchWaitMillis int64 `json:"dispatch_wait_ms"`
    ExplicitWaitMillis int64 `json:"explicit_wait_ms"`
    WaitMillis int64 `json:"wait_ms"`
    CoveredElapsedMillis int64 `json:"covered_elapsed_ms"`
    UnattributedElapsedMillis *int64 `json:"unattributed_elapsed_ms,omitempty"`
    OverlapMillis int64 `json:"wait_activity_overlap_ms"`
    InvalidFacts int `json:"invalid_facts"`
    OpenWaitCount int `json:"open_wait_count"`
    Intervals []WaitInterval `json:"wait_intervals"`
}

type timingInterval struct{start,end time.Time}
func unionDuration(items []timingInterval)int64 {
    if len(items)==0{return 0}
    sort.Slice(items,func(i,j int)bool {
        if items[i].start.Equal(items[j].start){return items[i].end.Before(items[j].end)}
        return items[i].start.Before(items[j].start)
    })
    begin,end:=items[0].start,items[0].end
    var total int64
    for _,v:=range items[1:] {
        if v.start.After(end) {
            total+=end.Sub(begin).Milliseconds()
            begin,end=v.start,v.end
        } else if v.end.After(end){end=v.end}
    }
    return total+end.Sub(begin).Milliseconds()
}
func clipInterval(start,end,lo,hi time.Time)(timingInterval,bool){
    if start.Before(lo){start=lo}
    if end.After(hi){end=hi}
    if !end.After(start){return timingInterval{},false}
    return timingInterval{start,end},true
}

// SummarizeLifecycle distinguishes explicit lifecycle waits from agent work.
// Only a registered→completed boundary yields a closed end-to-end duration.
// Any time not covered by Activity intervals or documented lifecycle waits
// remains unattributed, never silently reassigned to an agent.
func SummarizeLifecycle(taskID string,facts []LifecycleFact,segments []Segment)LifecycleCoverage {
    out:=LifecycleCoverage{TaskID:taskID,Source:"reported_lifecycle",Intervals:[]WaitInterval{}}
    sorted:=make([]LifecycleFact,0,len(facts))
    seen:=map[string]bool{}
    for _,e:=range facts {
        if !strings.EqualFold(e.TaskID,taskID){continue}
        if e.EventID==""||e.OccurredAt.IsZero(){out.InvalidFacts++;continue}
        if seen[e.EventID]{continue}
        seen[e.EventID]=true
        sorted=append(sorted,e)
    }
    sort.Slice(sorted,func(i,j int)bool{
        if sorted[i].OccurredAt.Equal(sorted[j].OccurredAt){return sorted[i].EventID<sorted[j].EventID}
        return sorted[i].OccurredAt.Before(sorted[j].OccurredAt)
    })
    var last LifecycleFact
    pending:=""
    finishWait:=func(e LifecycleFact){
        if pending==""{return}
        if !e.OccurredAt.After(last.OccurredAt){out.InvalidFacts++;pending="";return}
        ms:=e.OccurredAt.Sub(last.OccurredAt).Milliseconds()
        out.Intervals=append(out.Intervals,WaitInterval{
            Kind:pending,StartedAt:last.OccurredAt,EndedAt:e.OccurredAt,
            DurationMillis:ms,FromEventID:last.EventID,ToEventID:e.EventID,
        })
        switch pending {
        case "registration_queue":out.RegistrationQueueMillis+=ms
        case "dispatch_wait":out.DispatchWaitMillis+=ms
        case "explicit_wait":out.ExplicitWaitMillis+=ms
        }
        pending=""
    }
    for _,e:=range sorted {
        switch e.Kind {
        case "registered":
            if out.RegisteredAt==nil{copyTime:=e.OccurredAt;out.RegisteredAt=&copyTime}
            pending="registration_queue";last=e
        case "assigned":
            if pending=="registration_queue"{finishWait(e)}
            pending="dispatch_wait";last=e
        case "started":
            if pending=="dispatch_wait"{finishWait(e)}
            pending=""
        case "waiting":
            pending="explicit_wait";last=e
        case "resumed":
            if pending=="explicit_wait"{finishWait(e)}
            pending=""
        case "completed":
            if out.RegisteredAt!=nil && !e.OccurredAt.Before(*out.RegisteredAt) {
                completed:=e.OccurredAt
                if out.CompletedAt==nil || completed.After(*out.CompletedAt){out.CompletedAt=&completed}
            }
            pending=""
        }
    }
    if pending!=""{out.OpenWaitCount++}
    // Count only unioned wait ranges; overlapping/replayed lifecycle reports
    // cannot produce a synthetic duration above actual elapsed time.
    waits:=make([]timingInterval,0,len(out.Intervals))
    for _,v:=range out.Intervals{waits=append(waits,timingInterval{v.StartedAt,v.EndedAt})}
    out.WaitMillis=unionDuration(waits)
    if out.RegisteredAt==nil||out.CompletedAt==nil{return out}
    lo,hi:=*out.RegisteredAt,*out.CompletedAt
    elapsed:=hi.Sub(lo).Milliseconds()
    out.TotalElapsedMillis=&elapsed
    waitClipped:=make([]timingInterval,0,len(waits))
    for _,v:=range waits{if clipped,ok:=clipInterval(v.start,v.end,lo,hi);ok{waitClipped=append(waitClipped,clipped)}}
    active:=make([]timingInterval,0,len(segments))
    for _,s:=range segments {
        if !strings.EqualFold(s.TaskID,taskID){continue}
        if clipped,ok:=clipInterval(s.StartedAt,s.EndedAt,lo,hi);ok{active=append(active,clipped)}
    }
    waitsMS:=unionDuration(waitClipped)
    activeMS:=unionDuration(active)
    combined:=append(append([]timingInterval{},waitClipped...),active...)
    covered:=unionDuration(combined)
    if covered>elapsed{covered=elapsed}
    out.CoveredElapsedMillis=covered
    out.OverlapMillis=waitsMS+activeMS-covered
    unclassified:=elapsed-covered
    out.UnattributedElapsedMillis=&unclassified
    return out
}
