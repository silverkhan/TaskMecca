package contextlife

import (
	"testing"
	"time"
)

func TestReviewerRolesUnknownThresholdsAndIdleEvents(t *testing.T) {
	if !validRole("reviewer:raichyu") || validRole("reviewer:") || validRole("reviewer:../other") || rolePath("reviewer:raichyu") != "/root/controller/reviewer/raichyu" {
		t.Fatal("reviewer mapping")
	}
	unknown := Decide("reviewer:raichyu", nil, 0, "")
	if unknown.ContextStatus != "unknown" || unknown.ContextPercent != nil {
		t.Fatal(unknown)
	}
	for pct, want := range map[float64]string{64: "continue", 65: "prepare", 80: "rotate", 90: "protect"} {
		p := pct
		got := Decide("reviewer:raichyu", &Observation{WindowTokens: 100, UsagePercent: &p, Trust: "provider_verified"}, 0, "")
		if got.Action != want {
			t.Fatal(pct, got)
		}
	}
	for _, event := range []string{"review_completed", "review_iteration_completed"} {
		got := Decide("reviewer:raichyu", nil, 0, event)
		if got.Action != "prepare" || !got.RequiresNativeAction || got.ContextStatus != "unknown" {
			t.Fatal(got)
		}
	}
}
func TestReviewerRotationPreservesFailuresAndValidatesFreshSuccessor(t *testing.T) {
	project := seedProject(t)
	at := time.Now().UTC().Add(-time.Hour)
	role := "reviewer:raichyu"
	seedAttempt(t, project, "run-review-old", "native-old", "agent-old", role, at)
	s, e := Prepare(project, role, "run-review-old", at.Add(time.Minute))
	if e != nil {
		t.Fatal(e)
	}
	r := s.Rotation
	seedAttempt(t, project, "run-review-same", "native-old", "agent-new", role, at.Add(2*time.Minute))
	if _, e = Acknowledge(project, role, r.ID, "run-review-same", r.Checkpoint.Fingerprint, at.Add(3*time.Minute)); e == nil {
		t.Fatal("same native session accepted as successor")
	}
	failed, e := Read(project, role)
	if e != nil || failed.Rotation.Stage != "prepared" || failed.OwnerAttempt != "run-review-old" {
		t.Fatal("failure rewrote old evidence", failed, e)
	}
	seedAttempt(t, project, "run-review-next", "native-next", "agent-next", role, at.Add(4*time.Minute))
	if _, e = Acknowledge(project, role, r.ID, "run-review-next", "wrong-source-fingerprint", at.Add(5*time.Minute)); e == nil {
		t.Fatal("wrong checkpoint accepted")
	}
	if _, e = Acknowledge(project, role, r.ID, "run-review-next", r.Checkpoint.Fingerprint, at.Add(6*time.Minute)); e != nil {
		t.Fatal(e)
	}
	if _, e = Commit(project, role, r.ID, at.Add(7*time.Minute)); e == nil {
		t.Fatal("live old reviewer committed")
	}
	terminate(t, project, "run-review-old", at.Add(8*time.Minute))
	committed, e := Commit(project, role, r.ID, at.Add(9*time.Minute))
	if e != nil || committed.Generation != 2 || committed.OwnerAttempt != "run-review-next" || committed.Observation != nil {
		t.Fatal(committed, e)
	}
	status, e := Status(project, role, "", at.Add(10*time.Minute))
	if e != nil || status.Decision.ContextStatus != "unknown" || status.Decision.ContextPercent != nil {
		t.Fatal(status, e)
	}
}
