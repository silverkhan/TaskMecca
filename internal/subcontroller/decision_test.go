package subcontroller

import "testing"

func TestDelegationAtTwoPending(t *testing.T) {
 s:=Snapshot{BacklogID:"B-563",ActorRole:"main_controller",Event:"worker_completed",Authorized:true,RuntimeKnown:true,PendingReview:2,MaxSubcontrollers:2}
 d,err:=Decide(s);if err!=nil||d.Action!="propose_delegation"||d.CanonicalWriteAllowed {t.Fatalf("decision %#v %v",d,err)}
 s.PendingReview=1;d,_=Decide(s);if d.Action!="handle_directly" {t.Fatal(d)}
}

func TestSubCannotAccept(t *testing.T) {
 s:=Snapshot{BacklogID:"B-563",ActorRole:"sub_controller",Event:"review_completed",Authorized:true,RuntimeKnown:true,EvidenceReady:true}
 d,err:=Decide(s);if err!=nil||d.Action!="recommend_acceptance"||d.CanonicalWriteAllowed||!d.NeedsMainApproval {t.Fatalf("%#v %v",d,err)}
 s.ContractChanged=true;d,_=Decide(s);if d.Action!="escalate" {t.Fatal(d)}
 s.ContractChanged=false;s.IndependentReviewRequired=true;d,_=Decide(s);if d.Action!="request_evidence" {t.Fatal(d)}
}
func TestFailClosed(t *testing.T) {
 s:=Snapshot{BacklogID:"B-563",ActorRole:"main_controller",Event:"worker_completed",PendingReview:3,MaxSubcontrollers:3}
 d,_:=Decide(s);if d.Action!="defer"{t.Fatal(d)}
 s.Authorized=true;d,_=Decide(s);if d.Action!="escalate"{t.Fatal(d)}
}
