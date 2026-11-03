package activity

import "time"

// ActivityFilter affects observed Activity segments only. Lifecycle-derived
// registration, queue and waiting totals remain separate and unfiltered.
type ActivityFilter struct {
    Role Role
    Type Type
    Round *int
    Since time.Time
    Until time.Time
}
func(f ActivityFilter) Active()bool{
    return f.Role!=""||f.Type!=""||f.Round!=nil||!f.Since.IsZero()||!f.Until.IsZero()
}
func FilterSummary(full Summary,f ActivityFilter)Summary{
    if !f.Active(){return full}
    out:=Summary{
        TaskID:full.TaskID,Segments:[]Segment{},
        ByDimension:map[Dimension]int64{},ByType:map[Type]int64{},
        InvalidTransitions:full.InvalidTransitions,
        InvalidCorrections:full.InvalidCorrections,
    }
    ranges:=[]timingInterval{}
    for _,seg:=range full.Segments{
        if f.Role!=""&&seg.Role!=f.Role{continue}
        if f.Type!=""&&seg.Type!=f.Type{continue}
        if f.Round!=nil&&seg.Round!=*f.Round{continue}
        start,end:=seg.StartedAt,seg.EndedAt
        if !f.Since.IsZero() && start.Before(f.Since){start=f.Since}
        if !f.Until.IsZero() && end.After(f.Until){end=f.Until}
        if !end.After(start){continue}
        copySeg:=seg
        copySeg.StartedAt=start;copySeg.EndedAt=end
        copySeg.DurationMillis=end.Sub(start).Milliseconds()
        out.Segments=append(out.Segments,copySeg)
        out.AgentTimeMillis+=copySeg.DurationMillis
        out.ByType[copySeg.Type]+=copySeg.DurationMillis
        out.ByDimension[Dimension{Role:copySeg.Role,Type:copySeg.Type,Round:copySeg.Round}]+=copySeg.DurationMillis
        switch {
        case copySeg.Role==RoleWorker && copySeg.Round==0:
            out.InitialWorkerMillis+=copySeg.DurationMillis
        case copySeg.Role==RoleWorker && copySeg.Round>0:
            out.ReworkWorkerMillis+=copySeg.DurationMillis
        }
        if (copySeg.Role==RoleController||copySeg.Role==RoleSubController)&&copySeg.Type==ControllerReview {
            out.ControllerFirstReviewMillis+=copySeg.DurationMillis
        }
        if (copySeg.Role==RoleController||copySeg.Role==RoleSubController)&&copySeg.Type==ControllerReReview {
            out.ControllerRereviewMillis+=copySeg.DurationMillis
        }
        ranges=append(ranges,timingInterval{start,end})
        if out.FirstObservedAt==nil||start.Before(*out.FirstObservedAt){stamp:=start;out.FirstObservedAt=&stamp}
        if out.LastObservedAt==nil||end.After(*out.LastObservedAt){stamp:=end;out.LastObservedAt=&stamp}
    }
    out.ObservedWallMillis=unionDuration(ranges)
    if out.FirstObservedAt!=nil && out.LastObservedAt!=nil{
        out.ObservedSpanMillis=out.LastObservedAt.Sub(*out.FirstObservedAt).Milliseconds()
        if out.ObservedSpanMillis>out.ObservedWallMillis{
            out.UnattributedGapMillis=out.ObservedSpanMillis-out.ObservedWallMillis
        }
    }
    return out
}
