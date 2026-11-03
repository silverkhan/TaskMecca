package activity

import (
 "sort"
 "strings"
)

type ClassificationSuggestion struct {
    ProposedType string `json:"proposed_type"`
    EventCount int `json:"event_count"`
    ObservedMillis int64 `json:"observed_ms"`
    Examples []string `json:"examples"`
}

// SummarizeSuggestions preserves suggestions as proposals rather than adding
// their free-text names to the official Activity type vocabulary.
func SummarizeSuggestions(events []Event)[]ClassificationSuggestion{
    durations:=map[string]int64{}
    byTask:=map[string][]Event{}
    for _,e:=range events{byTask[e.TaskID]=append(byTask[e.TaskID],e)}
    for task,rows:=range byTask{
        for _,seg:=range Summarize(task,rows).Segments {
            if seg.Type!=Other{continue}
            name:=strings.ToLower(strings.TrimSpace(seg.ProposedType))
            if name==""{name="unspecified"}
            durations[name]+=seg.DurationMillis
        }
    }
    return SuggestionsWithDurations(events,durations)
}

// SuggestionsWithDurations reuses durations calculated during the project
// rollup instead of recomputing every task's sorted Activity timeline.
func SuggestionsWithDurations(events []Event,durations map[string]int64)[]ClassificationSuggestion{
    summaries:=map[string]*ClassificationSuggestion{}
    for _,e:=range events{
        if e.Type!=Other || e.Action==Correct || strings.TrimSpace(e.Explanation)==""{continue}
        name:=strings.ToLower(strings.TrimSpace(e.ProposedType))
        if name==""{name="unspecified"}
        summary:=summaries[name]
        if summary==nil{summary=&ClassificationSuggestion{ProposedType:name,Examples:[]string{}};summaries[name]=summary}
        if e.Action==Start || e.Action==Switch {
            summary.EventCount++
            if len(summary.Examples)<3 {summary.Examples=append(summary.Examples,e.Explanation)}
        }
    }
    for name,ms:=range durations{
        if group:=summaries[name];group!=nil{group.ObservedMillis=ms}
    }
    result:=make([]ClassificationSuggestion,0,len(summaries))
    for _,s:=range summaries{result=append(result,*s)}
    sort.Slice(result,func(i,j int)bool{
        if result[i].ObservedMillis!=result[j].ObservedMillis{return result[i].ObservedMillis>result[j].ObservedMillis}
        if result[i].EventCount!=result[j].EventCount{return result[i].EventCount>result[j].EventCount}
        return result[i].ProposedType<result[j].ProposedType
    })
    return result
}
