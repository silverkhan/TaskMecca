package subcontroller

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"testing"
)

func TestA78CallerJSONCannotProveIndependentReview(t *testing.T) {
	var s Snapshot
	json.Unmarshal([]byte(`{"backlog_id":"A-78","actor_role":"main_controller","event":"review_completed","execution_authorized":true,"runtime_known":true,"evidence_ready":true,"independent_review_required":true,"independent_review_ready":true,"VerifiedRolePolicy":{"allowed":true,"review":"independent"}}`), &s)
	d, e := Decide(s)
	if e != nil || d.Action != "request_evidence" {
		t.Fatal(d, e)
	}
	s.VerifiedRolePolicy = &rolepolicy.View{Allowed: true, Review: "self"}
	d, _ = Decide(s)
	if d.Action != "request_evidence" {
		t.Fatal(d)
	}
	s.VerifiedRolePolicy = &rolepolicy.View{Allowed: true, Review: "independent"}
	d, _ = Decide(s)
	if d.Action != "decide_acceptance" || d.CanonicalWriteAllowed {
		t.Fatal(d)
	}
}
