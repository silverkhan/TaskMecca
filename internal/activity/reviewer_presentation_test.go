package activity

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestReviewerNamesLargeArchiveRetainsRelevantRecentEvidence(t *testing.T) {
	project := t.TempDir()
	dir := runtimeobs.ExecutionRawDir(project)
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	old := filepath.Join(dir, "2020-01-01.jsonl")
	f, e := os.Create(old)
	if e != nil {
		t.Fatal(e)
	}
	// Real complete JSONL rows exceed the old global cap; not a sparse file.
	padding := strings.Repeat("x", 900)
	row, _ := json.Marshal(map[string]any{"event_kind": "session_metadata", "provider": "codex", "session_id": "unrelated-archive", "session_title": padding, "observed_at": "2020-01-01T00:00:00Z"})
	row = append(row, '\n')
	for written := 0; written < 35<<20; written += len(row) {
		if _, e = f.Write(row); e != nil {
			t.Fatal(e)
		}
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC().Add(-time.Minute)
	name := runtimeobs.RootNameObservation{Version: 1, Name: "D1112_01_Worker_fixture_G14", Source: "provider_name", Status: "verified", CheckedAt: now.Format(time.RFC3339Nano), T3ThreadID: "thread-fixture", HostStatus: "verified", HostProjectID: "project-fixture", IdentityVerified: true}
	event := runtimeobs.ExecutionEvent{EventKind: "session_metadata", Provider: "codex", SessionID: "native-recent", ObservedAt: name.CheckedAt, RootName: &name}
	b, _ := json.Marshal(event)
	recent := filepath.Join(dir, now.Format("2006-01-02")+".jsonl")
	os.WriteFile(recent, append(b, '\n'), 0600)
	names, stats := readPersistedNameEvidence(project, map[string]bool{"codex\x00native-recent": true, "codex\x00missing": true}, 8<<20, 64)
	if names["codex\x00native-recent"].Name != name.Name || names["codex\x00missing"].Name != "" || stats.BytesRead > 8<<20 || !stats.BudgetExhausted {
		t.Fatal(names, stats)
	}
	a := runtimeobs.Attempt{AttemptID: "run-recent", TaskID: "A-88", SessionID: "native-recent", RuntimeAgentID: "native-recent", Provider: "codex", AgentPath: "/root/controller/pikachyu", BindingState: runtimeobs.BindingBound, NativeReceipt: &runtimeobs.T3Receipt{SessionID: "native-recent", ThreadID: "thread-fixture"}}
	p := NewReviewerPresentation(project, "A-88", []runtimeobs.Attempt{a}, nil, nil, nil).ByAttempt[a.AttemptID]
	if !p.NameVerified || p.OfficialName != name.Name || p.ActorName != "pikachyu" || p.ActorNameSource != "bound_runtime_agent_path" || p.ReviewRound != nil || p.SessionGeneration != nil {
		t.Fatal(p)
	}
}

func TestReviewerNamesMalformedAffectedKeyAndOpaqueBarrier(t *testing.T) {
	project := t.TempDir()
	dir := runtimeobs.ExecutionRawDir(project)
	os.MkdirAll(dir, 0700)
	path := filepath.Join(dir, "2026-11-19.jsonl")
	line := func(native, name, at string) []byte {
		b, _ := json.Marshal(runtimeobs.ExecutionEvent{EventKind: "session_metadata", Provider: "codex", SessionID: native, ObservedAt: at, RootName: &runtimeobs.RootNameObservation{Name: name, CheckedAt: at}})
		return append(b, '\n')
	}
	data := line("bad", "old-bad", "2026-11-19T00:00:00Z")
	data = append(data, line("good", "good-proof", "2026-11-19T01:00:00Z")...)
	malformed := []byte("{\"event_kind\":\"session_metadata\",\"provider\":\"codex\",\"session_id\":\"bad\",\"observed_at\":\"2026-11-19T02:00:00Z\",\"root_name\":{\"version\":\"malformed\"}}\n")
	data = append(data, malformed...)
	os.WriteFile(path, data, 0600)
	names, _ := readPersistedNameEvidence(project, map[string]bool{"codex\x00bad": true, "codex\x00good": true}, 8<<20, 64)
	if names["codex\x00good"].Name != "good-proof" || names["codex\x00bad"].Name != "" {
		t.Fatal("affected key erased unrelated proof or revived stale name", names)
	}
	// A completely opaque older record cannot invalidate a newer parsed proof;
	// unresolved keys beyond that opaque boundary remain unknown.
	data = append([]byte("{unparseable\n"), line("good", "new-proof", "2026-11-19T03:00:00Z")...)
	os.WriteFile(path, data, 0600)
	names, _ = readPersistedNameEvidence(project, nil, 8<<20, 64)
	if names["codex\x00good"].Name != "new-proof" || names["codex\x00bad"].Name != "" {
		t.Fatal(names)
	}
}

func TestReviewerTerminalNameObservationIsMetadataNotLivenessOrBackfill(t *testing.T) {
	project := t.TempDir()
	dir := runtimeobs.ExecutionRawDir(project)
	os.MkdirAll(dir, 0700)
	ended := time.Now().UTC().Add(-3 * time.Minute)
	a := runtimeobs.Attempt{AttemptID: "run-terminal", TaskID: "A-88", SessionID: "native-terminal", RuntimeAgentID: "native-terminal", Provider: "codex", AgentPath: "/root/controller/reviewer/raichyu", BindingState: runtimeobs.BindingBound, CurrentState: runtimeobs.StateCompleted, Terminal: true, EndedAt: ended.Format(time.RFC3339Nano), NativeReceipt: &runtimeobs.T3Receipt{ThreadID: "thread-terminal", SessionID: "native-terminal"}}
	original := a
	for _, offset := range []time.Duration{-time.Minute, time.Minute} {
		at := ended.Add(offset).Format(time.RFC3339Nano)
		name := runtimeobs.RootNameObservation{Version: 1, Name: "D1112_01_Reviewer_raichyu_G3", Source: "provider_name", Status: "verified", CheckedAt: at, T3ThreadID: "thread-terminal", HostStatus: "verified", HostProjectID: "project-fixture", IdentityVerified: true}
		b, _ := json.Marshal(runtimeobs.ExecutionEvent{EventKind: "session_metadata", Provider: "codex", SessionID: a.SessionID, ObservedAt: at, RootName: &name})
		os.WriteFile(filepath.Join(dir, "2026-11-19.jsonl"), append(b, '\n'), 0600)
		got := NewReviewerPresentation(project, "A-88", []runtimeobs.Attempt{a}, nil, nil, nil).ByAttempt[a.AttemptID]
		if !got.NameVerified || got.NameObservedAt != at || got.ActualState != "unknown" || got.PurposeVerified || got.SessionGeneration != nil || got.ReviewRound != nil || !reflect.DeepEqual(original, a) {
			t.Fatal("metadata altered execution/history semantics", got, a)
		}
	}
	a.AgentPath = "/root/controller/reviewer/../guess"
	got := NewReviewerPresentation(project, "A-88", []runtimeobs.Attempt{a}, nil, nil, nil).ByAttempt[a.AttemptID]
	if got.ActorName != "" || got.ActorNameSource != "" {
		t.Fatal("invalid role path became alias", got)
	}
}
