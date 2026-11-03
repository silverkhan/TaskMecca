package activity

import "sort"

type RoundBreakdown struct {
    Round int `json:"round"`
    WorkerMillis int64 `json:"worker_ms"`
    ControllerReviewMillis int64 `json:"controller_review_ms"`
    TestMillis int64 `json:"test_ms"`
}
type ReviewReport struct {
    Rejections int `json:"rejections"`
    Accepted int `json:"accepted"`
    NeedsChanges int `json:"needs_changes"`
    SuggestedNextRound int `json:"suggested_next_round"`
    RoundBreakdown []RoundBreakdown `json:"round_breakdown"`
}

// SummarizeReviews counts only explicit review outcomes. A comment or failed
// test does not count as a Controller rejection. Historical duplicate IDs do
// not increment the rework round.
func SummarizeReviews(events []Event,segments []Segment)ReviewReport {
    events,_=EffectiveEvents(events)
    result:=ReviewReport{RoundBreakdown:[]RoundBreakdown{}}
    seen:=map[string]bool{}
    for _,e:=range events {
        if e.Action!=Stop || (e.Type!=ControllerReview && e.Type!=ControllerReReview)||seen[e.EventID]{continue}
        seen[e.EventID]=true
        switch e.ReviewOutcome {
        case "rejected":result.Rejections++
        case "accepted":result.Accepted++
        case "needs_changes":result.NeedsChanges++
        }
    }
    result.SuggestedNextRound=result.Rejections+result.NeedsChanges
    groups:=map[int]*RoundBreakdown{}
    for _,s:=range segments {
        group:=groups[s.Round]
        if group==nil{group=&RoundBreakdown{Round:s.Round};groups[s.Round]=group}
        if s.Role==RoleWorker {group.WorkerMillis+=s.DurationMillis}
        if s.Type==ControllerReview || s.Type==ControllerReReview {group.ControllerReviewMillis+=s.DurationMillis}
        if s.Type==TargetedTest || s.Type==FullRegression || s.Type==BuildCheck {group.TestMillis+=s.DurationMillis}
    }
    for _,g:=range groups{result.RoundBreakdown=append(result.RoundBreakdown,*g)}
    sort.Slice(result.RoundBreakdown,func(i,j int)bool{return result.RoundBreakdown[i].Round<result.RoundBreakdown[j].Round})
    return result
}
