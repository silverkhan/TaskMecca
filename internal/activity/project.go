package activity

import ("sort";"strings")

// ProjectRollup aggregates only measured intervals. Missing/unclosed work is
// explicitly counted, not inferred to be zero work. The denominator is the
// set of tasks with at least one Activity event.
type ProjectRollup struct {
    TasksObserved int `json:"tasks_observed"`
    TasksWithMeasuredTime int `json:"tasks_with_measured_time"`
    TasksWithOpenActivities int `json:"tasks_with_open_activities"`
    EventCount int `json:"event_count"`
    AgentTimeMillis int64 `json:"agent_time_ms"`
    ObservedWallMillis int64 `json:"sum_task_observed_wall_ms"`
    ByType map[Type]int64 `json:"by_type_ms"`
    Suggestions []ClassificationSuggestion `json:"classification_suggestions"`
    TaskSummaries []Summary `json:"task_summaries"`
    MedianAgentMillis int64 `json:"median_agent_ms"`
    P90AgentMillis int64 `json:"p90_agent_ms"`
}
func percentile(sorted []int64,numerator int)int64 {
    if len(sorted)==0{return 0}
    // nearest-rank percentile, ordered 1..n
    rank:=(len(sorted)*numerator+99)/100
    if rank<1{rank=1};if rank>len(sorted){rank=len(sorted)}
    return sorted[rank-1]
}
func SummarizeProject(events []Event)ProjectRollup {
    return summarizeProject(events,true)
}

// SummarizeProjectCompact keeps only aggregate statistics for the Workload
// page. Task detail timelines remain available from the per-task endpoint.
func SummarizeProjectCompact(events []Event)ProjectRollup {
    return summarizeProject(events,false)
}
func summarizeProject(events []Event,includeTasks bool)ProjectRollup {
    grouped:=map[string][]Event{}
    for _,e:=range events{grouped[e.TaskID]=append(grouped[e.TaskID],e)}
    ids:=make([]string,0,len(grouped))
    for id:=range grouped{ids=append(ids,id)}
    sort.Strings(ids)
    result:=ProjectRollup{TasksObserved:len(ids),EventCount:len(events),ByType:map[Type]int64{},TaskSummaries:make([]Summary,0,len(ids))}
    timings:=make([]int64,0,len(ids))
    suggestionDurations:=map[string]int64{}
    for _,id:=range ids{
        s:=Summarize(id,grouped[id])
        if includeTasks {result.TaskSummaries=append(result.TaskSummaries,s)}
        result.AgentTimeMillis+=s.AgentTimeMillis
        result.ObservedWallMillis+=s.ObservedWallMillis
        if s.AgentTimeMillis>0{result.TasksWithMeasuredTime++;timings=append(timings,s.AgentTimeMillis)}
        if s.OpenActivities>0{result.TasksWithOpenActivities++}
        for kind,ms:=range s.ByType{result.ByType[kind]+=ms}
        for _,segment:=range s.Segments{
            if segment.Type!=Other{continue}
            key:=strings.ToLower(strings.TrimSpace(segment.ProposedType))
            if key==""{key="unspecified"}
            suggestionDurations[key]+=segment.DurationMillis
        }
    }
    sort.Slice(timings,func(i,j int)bool{return timings[i]<timings[j]})
    result.MedianAgentMillis=percentile(timings,50)
    result.P90AgentMillis=percentile(timings,90)
    result.Suggestions=SuggestionsWithDurations(events,suggestionDurations)
    return result
}
