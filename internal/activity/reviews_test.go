package activity

import "testing"

func TestReviewOutcomesDriveReworkRounds(t *testing.T){
 e:=validEvent();e.Type=ControllerReview;e.Role=RoleController;e.Action=Stop;e.ReviewOutcome="rejected"
 next:=e;next.EventID="review-2";next.Type=ControllerReReview;next.ReviewOutcome="accepted"
 events:=[]Event{e,e,next}
 segments:=[]Segment{{Role:RoleWorker,Round:0,DurationMillis:120000},{Role:RoleController,Round:0,Type:ControllerReview,DurationMillis:30000},
 {Role:RoleWorker,Round:1,DurationMillis:60000},{Role:RoleController,Round:1,Type:ControllerReReview,DurationMillis:15000}}
 r:=SummarizeReviews(events,segments)
 if r.Rejections!=1||r.Accepted!=1||r.SuggestedNextRound!=1||len(r.RoundBreakdown)!=2{
    t.Fatalf("review counts %+v",r)
 }
 if r.RoundBreakdown[0].WorkerMillis!=120000||r.RoundBreakdown[1].WorkerMillis!=60000||r.RoundBreakdown[1].ControllerReviewMillis!=15000 {
    t.Fatalf("round breakdown %+v",r.RoundBreakdown)
 }
}
