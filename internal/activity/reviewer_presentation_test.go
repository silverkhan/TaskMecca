package activity

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReviewerPresentationRequiresExactNativeNameEvidence(t *testing.T) {
	project := t.TempDir()
	at := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	a := runtimeobs.Attempt{AttemptID: "run-fixture", TaskID: "A-88", SessionID: "native-fixture", RuntimeAgentID: "native-fixture", Provider: "codex", AgentPath: "/root/controller/pikachyu", BindingState: runtimeobs.BindingBound, CurrentState: runtimeobs.StateRunning, SessionName: "alias G999 guess", NativeReceipt: &runtimeobs.T3Receipt{ThreadID: "opaque-thread", SessionID: "native-fixture"}}
	policies := map[string]*rolepolicy.Policy{}
	initial := NewReviewerPresentation(project, "A-88", []runtimeobs.Attempt{a}, nil, policies, nil).ByAttempt[a.AttemptID]
	if initial.NameVerified || initial.OfficialName != "" || initial.PurposeVerified || initial.ActualState != "unknown" || initial.ReviewRound != nil || initial.SessionGeneration != nil {
		t.Fatal(initial)
	}
	name := runtimeobs.RootNameObservation{Version: 1, Name: "D1112_01_Worker_pikachyu_G14", Source: "provider_name", Status: "verified", CheckedAt: at, T3ThreadID: "opaque-thread", HostStatus: "verified", HostProjectID: "fixture-project", IdentityVerified: true}
	path := filepath.Join(runtimeobs.ExecutionRawDir(project), "fixture-daily.jsonl")
	os.MkdirAll(filepath.Dir(path), 0700)
	write := func(n runtimeobs.RootNameObservation, native string) {
		b, _ := json.Marshal(runtimeobs.ExecutionEvent{EventKind: "session_metadata", Provider: "codex", SessionID: native, RootName: &n})
		os.WriteFile(path, append(b, '\n'), 0600)
	}
	write(name, "other-native")
	wrong := NewReviewerPresentation(project, "A-88", []runtimeobs.Attempt{a}, nil, policies, nil).ByAttempt[a.AttemptID]
	if wrong.NameVerified {
		t.Fatal("cross native joined")
	}
	write(name, a.SessionID)
	got := NewReviewerPresentation(project, "A-88", []runtimeobs.Attempt{a}, nil, policies, nil).ByAttempt[a.AttemptID]
	if !got.NameVerified || got.OfficialName != name.Name || got.ReviewRound != nil || got.SessionGeneration != nil || got.PurposeVerified {
		t.Fatal(got)
	}
	name.Status = "unavailable"
	write(name, a.SessionID)
	if NewReviewerPresentation(project, "A-88", []runtimeobs.Attempt{a}, nil, policies, nil).ByAttempt[a.AttemptID].NameVerified {
		t.Fatal("unavailable evidence retained old name")
	}
}
func TestReviewerNecessityIsSeparateFromActualActivity(t *testing.T) {
	project := t.TempDir()
	p := &rolepolicy.Policy{ReviewRequirement: &rolepolicy.ReviewRequirement{Risk: rolepolicy.ReviewRisk{Impact: "high", Complexity: "complex", Reversibility: "easy", Verifiability: "established", ExplicitRequired: true}, Scope: []string{"x"}, Rationale: "explicit"}}
	a := runtimeobs.Attempt{AttemptID: "run-fixture", TaskID: "A-1", SessionID: "native", RuntimeAgentID: "native", Provider: "codex", BindingState: runtimeobs.BindingBound, CurrentState: runtimeobs.StateRunning}
	got := NewReviewerPresentation(project, "A-1", []runtimeobs.Attempt{a}, nil, map[string]*rolepolicy.Policy{"A-1": p}, nil)
	if got.Task.Necessity != "required" || got.Task.ActualState != "unknown" {
		t.Fatal(got)
	}
	verified := map[string]ReviewerPresentationEntry{a.AttemptID: {AttemptID: a.AttemptID, NativeID: a.SessionID, TaskID: a.TaskID, Purpose: "review", PurposeVerified: true, ActualState: "unknown", ContextStatus: "unknown"}}
	events := []Event{{TaskID: a.TaskID, RuntimeAttemptID: "another-attempt", AgentID: a.RuntimeAgentID, Type: ControllerReview, Action: Start}}
	got = NewReviewerPresentation(project, "A-1", []runtimeobs.Attempt{a}, events, map[string]*rolepolicy.Policy{"A-1": p}, verified)
	if got.Task.ActualState != "unknown" {
		t.Fatal("cross attempt review")
	}
	events[0].RuntimeAttemptID = a.AttemptID
	got = NewReviewerPresentation(project, "A-1", []runtimeobs.Attempt{a}, events, map[string]*rolepolicy.Policy{"A-1": p}, verified)
	if got.Task.ActualState != "reviewing" {
		t.Fatal(got)
	}
}
