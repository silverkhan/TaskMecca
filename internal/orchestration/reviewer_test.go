package orchestration

import "testing"

func TestA90ReviewerDispatchCannotRequestImplementation(t *testing.T) {
	e, r, now := dispatchFixture()
	r.Role = "reviewer"
	r.Purpose = "implementation"
	p := PlanDispatch(e, r, ModeB, now)
	for _, reason := range p.Reasons {
		if reason == "reviewer_cannot_dispatch_product_implementation" {
			return
		}
	}
	t.Fatal("Reviewer implementation dispatch did not fail at role boundary", p)
}

func TestA90FormalReviewerNameIsNotWorkerGenerationOrAuthority(t *testing.T) {
	g := Group{GroupID: "D1112_01", DisplayPrefix: "D1112_01_"}
	n, err := SessionName(g, "reviewer", "raichyu", 5)
	if err != nil || n != "D1112_01_Reviewer_raichyu_G5" || !IsManagedSessionName(n) {
		t.Fatal(n, err)
	}
	d, err := SessionNamingDirective(g, NativeNameTarget{Role: "reviewer", Worker: "raichyu", Generation: 5, Provider: "codex", Execution: "newSession"})
	if err != nil || d.IdentityAuthoritative || d.Verified || d.DisplayName != n {
		t.Fatal(d, err)
	}
	if _, err = SessionName(g, "reviewer", "../worker", 5); err == nil {
		t.Fatal("invalid alias")
	}
	if _, err = SessionName(g, "reviewer", "raichyu", 0); err == nil {
		t.Fatal("invalid generation")
	}
	if !IsManagedSessionName("D1112_01_Worker_raichyu_G4") {
		t.Fatal("legacy name lost")
	}
}
