package activity
import "testing"
func TestSuggestReviewRoundsAndUnknown(t *testing.T){
 first:=Suggest(RoleController,"review_started",0)
 again:=Suggest(RoleController,"review_started",2)
 if first.Type!=ControllerReview || again.Type!=ControllerReReview || first.Authoritative {t.Fatalf("review hints: %+v %+v",first,again)}
 unknown:=Suggest(RoleWorker,"unexpected_dependency",0)
 if unknown.Type!=Other || !unknown.RequiresExplanation {t.Fatalf("unknown hint: %+v",unknown)}
 mislabeled:=Suggest(RoleWorker,"review_started",1)
 if mislabeled.Type!=Other {t.Fatalf("workers cannot self-classify as reviewer: %+v",mislabeled)}
}
