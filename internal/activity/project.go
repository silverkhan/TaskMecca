package activity

import ("sort";"strings")

// TaskTime gives a bounded, grounded drill-down shortlist for the Workload
// dashboard. Completed lifecycle elapsed and Activity-only observed wall-time
// must never be presented as the same measurement.
type TaskTime struct {
    TaskID string `json:"task_id"`
    DurationMillis int64 `json:"duration_ms"`
    DurationBasis string `json:"duration_basis"`
    AgentMillis int64 `json:"agent_ms"`
    ReworkMillis int64 `json:"rework_ms"`
    ReviewMillis int64 `json:"review_ms"`
    UnattributedMillis *int64 `json:"unattributed_ms,omitempty"`
}

// ProjectRollup aggregates only measured intervals. Missing/unclosed work is
// explicitly counted, not inferred to be zero work. The denominator is the
// set of tasks with at least one Activity event.
type ProjectRollup struct {
    TasksObserved int `json:"tasks_observed"`
    TasksWithMeasuredTime int `json:"tasks_with_measured_time"`
    TasksWithOpenActivities int `json:"tasks_with_open_activities"`
    EventCount int `json:"event_count"`
    LifecycleCompletedTasks int `json:"lifecycle_completed_tasks"`
    LifecycleOpenWaitTasks int `json:"lifecycle_open_wait_tasks"`
    LifecycleTotalElapsedMillis int64 `json:"lifecycle_total_elapsed_ms"`
    LifecycleCoveredMillis int64 `json:"lifecycle_covered_ms"`
    LifecycleWaitMillis int64 `json:"lifecycle_wait_ms"`
    LifecycleUnattributedMillis int64 `json:"lifecycle_unattributed_ms"`
    LifecycleOverlapMillis int64 `json:"lifecycle_overlap_ms"`
    AgentTimeMillis int64 `json:"agent_time_ms"`
    ObservedWallMillis int64 `json:"sum_task_observed_wall_ms"`
    ByType map[Type]int64 `json:"by_type_ms"`
    Suggestions []ClassificationSuggestion `json:"classification_suggestions"`
    TaskSummaries []Summary `json:"task_summaries"`
    MedianAgentMillis int64 `json:"median_agent_ms"`
    P90AgentMillis int64 `json:"p90_agent_ms"`
    FilteredActivity bool `json:"filtered_activity"`
    TopTasks []TaskTime `json:"top_tasks"`
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
// SummarizeProjectWithLifecycle reuses already aggregated Activity intervals
// when calculating reported queue/wait/unknown times. No duplicate pass over
// each backlog's Activity events is required.
func SummarizeProjectWithLifecycle(events []Event, facts map[string][]LifecycleFact)ProjectRollup {
    return summarizeProjectWithFilter(events,false,facts,ActivityFilter{})
}
func SummarizeProjectWithFilter(events []Event, facts map[string][]LifecycleFact,filter ActivityFilter)ProjectRollup{
    return summarizeProjectWithFilter(events,false,facts,filter)
}
func summarizeProject(events []Event,includeTasks bool)ProjectRollup {
    return summarizeProjectWithFilter(events,includeTasks,nil,ActivityFilter{})
}
func summarizeProjectWithLifecycle(events []Event,includeTasks bool,facts map[string][]LifecycleFact)ProjectRollup {
    return summarizeProjectWithFilter(events,includeTasks,facts,ActivityFilter{})
}
func summarizeProjectWithFilter(events []Event,includeTasks bool,facts map[string][]LifecycleFact,filter ActivityFilter)ProjectRollup {
    grouped:=map[string][]Event{}
    for _,e:=range events{grouped[e.TaskID]=append(grouped[e.TaskID],e)}
    allIDs:=map[string]bool{}
    for id:=range grouped{allIDs[id]=true}
    for id:=range facts{allIDs[id]=true}
    ids:=make([]string,0,len(allIDs))
    for id:=range allIDs{ids=append(ids,id)}
    sort.Strings(ids)
    result:=ProjectRollup{TasksObserved:len(grouped),EventCount:len(events),ByType:map[Type]int64{},TaskSummaries:make([]Summary,0,len(grouped))}
    result.FilteredActivity=filter.Active()
    topCandidates:=make([]TaskTime,0,len(ids))
    timings:=make([]int64,0,len(ids))
    suggestionDurations:=map[string]int64{}
    for _,id:=range ids{
        full:=Summarize(id,grouped[id])
        s:=FilterSummary(full,filter)
        if includeTasks {result.TaskSummaries=append(result.TaskSummaries,s)}
        result.AgentTimeMillis+=s.AgentTimeMillis
        result.ObservedWallMillis+=s.ObservedWallMillis
        if s.AgentTimeMillis>0{result.TasksWithMeasuredTime++;timings=append(timings,s.AgentTimeMillis)}
        if s.OpenActivities>0{result.TasksWithOpenActivities++}
        for kind,ms:=range s.ByType{result.ByType[kind]+=ms}
        candidate:=TaskTime{
            TaskID:id,DurationMillis:s.ObservedWallMillis,DurationBasis:"activity_observed_wall",
            AgentMillis:s.AgentTimeMillis,ReworkMillis:s.ReworkWorkerMillis,
            ReviewMillis:s.ControllerFirstReviewMillis+s.ControllerRereviewMillis,
        }
        if len(facts[id])>0{
            coverage:=SummarizeLifecycle(id,facts[id],full.Segments)
            if coverage.OpenWaitCount>0{result.LifecycleOpenWaitTasks++}
            if coverage.TotalElapsedMillis!=nil{
                if !filter.Active(){
                    candidate.DurationMillis=*coverage.TotalElapsedMillis
                    candidate.DurationBasis="registered_completed"
                    candidate.UnattributedMillis=coverage.UnattributedElapsedMillis
                }else{
                    candidate.DurationBasis="filtered_activity_wall"
                }
                result.LifecycleCompletedTasks++
                result.LifecycleTotalElapsedMillis+=*coverage.TotalElapsedMillis
                result.LifecycleCoveredMillis+=coverage.CoveredElapsedMillis
                result.LifecycleOverlapMillis+=coverage.OverlapMillis
                // WaitMillis is the union of explicit intervals; if some
                // lifecycle facts are outside the completed window, clamp the
                // advertised aggregate to the completed elapsed duration.
                waitMillis:=coverage.WaitMillis
                if waitMillis>*coverage.TotalElapsedMillis{waitMillis=*coverage.TotalElapsedMillis}
                result.LifecycleWaitMillis+=waitMillis
                if coverage.UnattributedElapsedMillis!=nil{result.LifecycleUnattributedMillis+=*coverage.UnattributedElapsedMillis}
            }
        }
        if candidate.DurationMillis>0{topCandidates=append(topCandidates,candidate)}
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
    result.Suggestions=SuggestionsWithDurations(events,suggestionDurations) // proposals remain project-wide, not role/time-filtered.
    sort.Slice(topCandidates,func(i,j int)bool {
        if topCandidates[i].DurationMillis==topCandidates[j].DurationMillis{return topCandidates[i].TaskID<topCandidates[j].TaskID}
        return topCandidates[i].DurationMillis>topCandidates[j].DurationMillis
    })
    if len(topCandidates)>10{topCandidates=topCandidates[:10]}
    result.TopTasks=topCandidates
    return result
}
