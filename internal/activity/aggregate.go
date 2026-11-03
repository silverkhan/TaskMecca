package activity

import (
    "sort"
    "time"
)

// Segment is an observed wall-clock interval attributed to one actor.
// Open activity intervals are not projected to now: that would manufacture time.
type Segment struct {
    TaskID string `json:"task_id"`
    AgentID string `json:"agent_id"`
    AttemptID string `json:"attempt_id"`
    Role Role `json:"role"`
    Type Type `json:"type"`
    Round int `json:"round"`
    StartedAt time.Time `json:"started_at"`
    EndedAt time.Time `json:"ended_at"`
    DurationMillis int64 `json:"duration_ms"`
}
type Dimension struct {
    Role Role `json:"role"`
    Type Type `json:"type"`
    Round int `json:"round"`
}
type Summary struct {
    TaskID string `json:"task_id"`
    Segments []Segment `json:"segments"`
    ByDimension map[Dimension]int64 `json:"-"`
    ByType map[Type]int64 `json:"by_type_ms"`
    AgentTimeMillis int64 `json:"agent_time_ms"`
    ObservedWallMillis int64 `json:"observed_wall_ms"`
    FirstObservedAt *time.Time `json:"first_observed_at,omitempty"`
    LastObservedAt *time.Time `json:"last_observed_at,omitempty"`
    ObservedSpanMillis int64 `json:"observed_span_ms"`
    UnattributedGapMillis int64 `json:"unattributed_gap_ms"`
    OpenActivities int `json:"open_activities"`
    InvalidTransitions int `json:"invalid_transitions"`
    InitialWorkerMillis int64 `json:"initial_worker_ms"`
    ReworkWorkerMillis int64 `json:"rework_worker_ms"`
    ControllerFirstReviewMillis int64 `json:"controller_first_review_ms"`
    ControllerRereviewMillis int64 `json:"controller_rereview_ms"`
    UnknownMillis *int64 `json:"unknown_ms,omitempty"`
}
func actorKey(e Event)string{return string(e.Role)+"\x00"+e.AgentID+"\x00"+e.RuntimeAttemptID}
type activeSegment struct{event Event}

// Summarize counts only intervals with observable start AND end boundaries.
// Correction events are excluded until an authoritative supersession projection
// supplies the corrected view. This avoids silently rewriting historical time.
func Summarize(task string, events []Event) Summary {
    rows:=make([]Event,0,len(events))
    for _,event:=range events {
        if event.TaskID==task && event.Action!=Correct {rows=append(rows,event)}
    }
    sort.SliceStable(rows,func(i,j int)bool {
        if rows[i].OccurredAt.Equal(rows[j].OccurredAt){
            if rows[i].RecordedAt.Equal(rows[j].RecordedAt){return rows[i].EventID<rows[j].EventID}
            return rows[i].RecordedAt.Before(rows[j].RecordedAt)
        }
        return rows[i].OccurredAt.Before(rows[j].OccurredAt)
    })
    out:=Summary{TaskID:task,Segments:[]Segment{},ByDimension:map[Dimension]int64{},ByType:map[Type]int64{}}
    active:=map[string]activeSegment{}
    for _,e:=range rows{
        if out.FirstObservedAt==nil{when:=e.OccurredAt;out.FirstObservedAt=&when}
        when:=e.OccurredAt;out.LastObservedAt=&when
        k:=actorKey(e)
        current,exists:=active[k]
        switch e.Action {
        case Start:
            if exists {out.InvalidTransitions++;continue}
            active[k]=activeSegment{event:e}
        case Switch,Stop:
            if !exists || e.OccurredAt.Before(current.event.OccurredAt) {
                out.InvalidTransitions++
                continue
            }
            duration:=e.OccurredAt.Sub(current.event.OccurredAt).Milliseconds()
            segment:=Segment{TaskID:task,AgentID:current.event.AgentID,AttemptID:current.event.RuntimeAttemptID,Role:current.event.Role,Type:current.event.Type,Round:current.event.Round,StartedAt:current.event.OccurredAt,EndedAt:e.OccurredAt,DurationMillis:duration}
            out.Segments=append(out.Segments,segment)
            out.ByType[segment.Type]+=duration
            out.ByDimension[Dimension{Role:segment.Role,Type:segment.Type,Round:segment.Round}]+=duration
            out.AgentTimeMillis+=duration
            if segment.Role==RoleWorker && segment.Round==0 {out.InitialWorkerMillis+=duration}
            if segment.Role==RoleWorker && segment.Round>0 {out.ReworkWorkerMillis+=duration}
            if (segment.Role==RoleController||segment.Role==RoleSubController) && segment.Type==ControllerReview {out.ControllerFirstReviewMillis+=duration}
            if (segment.Role==RoleController||segment.Role==RoleSubController) && segment.Type==ControllerReReview {out.ControllerRereviewMillis+=duration}
            if e.Action==Stop {delete(active,k)}else{active[k]=activeSegment{event:e}}
        }
    }
    out.OpenActivities=len(active)
    intervals:=append([]Segment(nil),out.Segments...)
    sort.Slice(intervals,func(i,j int)bool{return intervals[i].StartedAt.Before(intervals[j].StartedAt)})
    if len(intervals)>0{
        left,right:=intervals[0].StartedAt,intervals[0].EndedAt
        for _,s:=range intervals[1:]{
            if s.StartedAt.After(right){
                out.ObservedWallMillis+=right.Sub(left).Milliseconds()
                left,right=s.StartedAt,s.EndedAt
            }else if s.EndedAt.After(right){right=s.EndedAt}
        }
        out.ObservedWallMillis+=right.Sub(left).Milliseconds()
    }
    if out.FirstObservedAt!=nil && out.LastObservedAt!=nil {
        out.ObservedSpanMillis=out.LastObservedAt.Sub(*out.FirstObservedAt).Milliseconds()
        if out.ObservedSpanMillis>out.ObservedWallMillis {
            out.UnattributedGapMillis=out.ObservedSpanMillis-out.ObservedWallMillis
        }
    }
    return out
}
