package runtimeobs

import (
	"fmt"
	"testing"
	"time"
)

func TestSessionUsageProvenanceAndBoundary(t *testing.T) {
	now := time.Date(2026, 11, 16, 3, 0, 0, 0, time.UTC)
	stamp := func(d time.Duration) string { return now.Add(d).Format(time.RFC3339Nano) }
	for _, tc := range []struct {
		name        string
		a           Attempt
		want, group string
	}{
		{"observer", Attempt{LastActivityAt: stamp(0), LastObservedAt: stamp(0), StartedAt: stamp(0), EndedAt: stamp(0)}, "", "historical"},
		{"hook", Attempt{HookActivityCount: 1, LastHookActivityAt: stamp(-time.Hour)}, stamp(-time.Hour), "active"},
		{"boundary", Attempt{HookActivityCount: 1, LastHookActivityAt: stamp(-48 * time.Hour)}, stamp(-48 * time.Hour), "active"},
		{"older", Attempt{HookActivityCount: 1, LastHookActivityAt: stamp(-48*time.Hour - time.Nanosecond)}, stamp(-48*time.Hour - time.Nanosecond), "historical"},
		{"future", Attempt{HookActivityCount: 1, LastHookActivityAt: stamp(time.Nanosecond)}, "", "historical"},
		{"unproven_hook", Attempt{LastHookActivityAt: stamp(0)}, "", "historical"},
		{"unproven_native", Attempt{NativeStartedAt: stamp(0), NativeCompletedAt: stamp(0)}, "", "historical"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.a.AttemptID = "a"
			roots := []RootSession{{RootSessionID: RootSessionIDForAttempt(tc.a)}}
			applySessionUsage(roots, []Attempt{tc.a}, now)
			if roots[0].LastUsedAt != tc.want || roots[0].UsageGroup != tc.group {
				t.Fatalf("%+v", roots[0])
			}
		})
	}
	a := Attempt{Provider: "codex", SessionID: "native", RuntimeAgentID: "native", TurnID: "attempt", AttemptID: "a", CurrentState: StateCompleted, StateEvidenceSource: EvidenceT3, StateObservationQuality: QualityObserved, LastObservedAt: stamp(0), NativeReceipt: &T3Receipt{Provider: "codex", SessionID: "native", ThreadID: "thread", RunID: "run", NativeAttemptID: "attempt", State: StateCompleted, Origin: "t3_local_projection_v2", StartedAt: stamp(-72 * time.Hour), CompletedAt: stamp(-50 * time.Hour), ObservedAt: stamp(0)}}
	at, source := attemptUsage(a, now)
	if at != stamp(-50*time.Hour) || source != "native_execution_boundary" {
		t.Fatalf("%s %s", at, source)
	}
	a.NativeReceipt.SessionID = "other"
	if at, _ := attemptUsage(a, now); at != "" {
		t.Fatal("cross-session receipt accepted")
	}
}

func TestSessionUsageRejectsUnverifiedNativeReceipt(t *testing.T) {
	now := time.Date(2026, 11, 16, 3, 0, 0, 0, time.UTC)
	stamp := now.Format(time.RFC3339Nano)
	base := Attempt{Provider: "codex", SessionID: "native", RuntimeAgentID: "native", TurnID: "episode", CurrentState: StateRunning, StateEvidenceSource: EvidenceT3, StateObservationQuality: QualityObserved, LastObservedAt: stamp, NativeReceipt: &T3Receipt{Origin: "t3_local_projection_v2", Provider: "codex", SessionID: "native", ThreadID: "thread", RunID: "run", NativeAttemptID: "episode", State: StateRunning, ObservedAt: stamp, StartedAt: now.Add(-time.Hour).Format(time.RFC3339Nano)}}
	if at, _ := attemptUsage(base, now); at == "" {
		t.Fatal("verified boundary missing")
	}
	for _, tc := range []struct {
		name   string
		change func(*Attempt)
	}{
		{"unknown quality", func(a *Attempt) { a.StateObservationQuality = ObservationQuality("unknown") }},
		{"unverified source", func(a *Attempt) { a.StateEvidenceSource = EvidenceManualBinding }},
		{"foreign runtime", func(a *Attempt) { a.RuntimeAgentID = "other" }},
		{"foreign episode", func(a *Attempt) { a.TurnID = "other" }},
		{"state conflict", func(a *Attempt) { a.NativeReceipt.State = StateCompleted }},
		{"future observation", func(a *Attempt) {
			a.NativeReceipt.ObservedAt = now.Add(time.Hour).Format(time.RFC3339Nano)
			a.LastObservedAt = a.NativeReceipt.ObservedAt
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := base
			r := *base.NativeReceipt
			a.NativeReceipt = &r
			tc.change(&a)
			if at, _ := attemptUsage(a, now); at != "" {
				t.Fatalf("unverified receipt promoted to actual use: %s", at)
			}
		})
	}
}

func TestSessionUsagePagesDetailReuseAndTies(t *testing.T) {
	project := t.TempDir()
	now := time.Date(2026, 11, 16, 3, 0, 0, 0, time.UTC)
	ledger := Ledger{Attempts: []Attempt{}, Findings: []LedgerFinding{}}
	for i := 0; i < 125; i++ {
		a := Attempt{AttemptID: fmt.Sprintf("a-%03d", i), Provider: "codex", SessionID: fmt.Sprintf("session-%03d", i), CurrentState: StateCompleted, Terminal: true, HookActivityCount: 1, LastHookActivityAt: now.Add(-49 * time.Hour).Format(time.RFC3339Nano), LastObservedAt: now.Format(time.RFC3339Nano)}
		if i < 25 {
			a.LastHookActivityAt = now.Add(-time.Hour).Format(time.RFC3339Nano)
		}
		ledger.Attempts = append(ledger.Attempts, a)
	}
	page, err := QuerySessionWorkload(project, ledger, 1, 1, 10, now)
	if err != nil {
		t.Fatal(err)
	}
	if page.Active.Total != 25 || page.Historical.Total != 100 || page.Active.TotalPages != 3 || page.Historical.TotalPages != 10 {
		t.Fatalf("%+v", page)
	}
	seen := map[string]bool{}
	for i := 1; i <= 10; i++ {
		p, err := QuerySessionWorkload(project, ledger, i, i, 10, now)
		if err != nil {
			t.Fatal(err)
		}
		if i <= 3 {
			for _, r := range p.Active.Items {
				if seen[r.RootSessionID] {
					t.Fatal("duplicate")
				}
				seen[r.RootSessionID] = true
			}
		}
		for _, r := range p.Historical.Items {
			if seen[r.RootSessionID] {
				t.Fatal("duplicate")
			}
			seen[r.RootSessionID] = true
		}
	}
	if len(seen) != 125 {
		t.Fatal(len(seen))
	}
	for i := 1; i < 25; i++ {
		if page.LatestSessionID > RootSessionIDForAttempt(ledger.Attempts[i]) {
			t.Fatal("tie order")
		}
	}
	oldID := RootSessionIDForAttempt(ledger.Attempts[124])
	ledger.Attempts[124].LastHookActivityAt = now.Format(time.RFC3339Nano)
	reused, err := QuerySessionWorkload(project, ledger, 1, 1, 10, now)
	if err != nil {
		t.Fatal(err)
	}
	if reused.LatestSessionID != oldID || reused.Active.Total != 26 || reused.Historical.Total != 99 {
		t.Fatal("reuse counts/default disagree")
	}
	detail, err := QuerySessionWorkloadDetail(project, ledger, oldID, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Project != project || detail.RootSession.UsageGroup != "active" || detail.RootSession.LastUsedAt != now.Format(time.RFC3339Nano) || detail.Total != 1 {
		t.Fatalf("%+v", detail)
	}
	other := t.TempDir()
	empty, err := QuerySessionWorkload(other, Ledger{}, 1, 1, 10, now)
	if err != nil || empty.Active.Total+empty.Historical.Total != 0 {
		t.Fatalf("scope: %+v %v", empty, err)
	}
	for i := 0; i < 41; i++ {
		a := ledger.Attempts[124]
		a.AttemptID = fmt.Sprintf("reuse-%d", i)
		ledger.Attempts = append(ledger.Attempts, a)
	}
	detail, err = QuerySessionWorkloadDetail(project, ledger, oldID, 3, now)
	if err != nil || detail.Total != 42 || detail.TotalPages != 3 || len(detail.Attempts) != 2 {
		t.Fatalf("detail pagination: %+v %v", detail, err)
	}
}

func TestSessionUsageFoldIgnoresFreshMetadataAndObserver(t *testing.T) {
	project := t.TempDir()
	now := time.Date(2026, 11, 16, 3, 0, 0, 0, time.UTC)
	old := now.Add(-72 * time.Hour).Format(time.RFC3339Nano)
	for _, e := range []ExecutionEvent{
		{EventID: "real-use", EventKind: "activity", AttemptID: "a", Provider: "codex", SessionID: "s", ObservedAt: old, EvidenceSource: EvidenceHook, ObservationQuality: QualityObserved},
		{EventID: "observer", EventKind: "state", AttemptID: "a", Provider: "codex", SessionID: "s", ObservedAt: now.Format(time.RFC3339Nano), State: StateRunning, EvidenceSource: EvidenceT3, ObservationQuality: QualityObserved},
		{EventID: "name", EventKind: "session_metadata", AttemptID: "meta", Provider: "codex", SessionID: "s", SessionName: "Fresh name", ObservedAt: now.Format(time.RFC3339Nano), EvidenceSource: EvidenceHook, ObservationQuality: QualityObserved},
	} {
		if err := AppendExecutionEvent(project, e); err != nil {
			t.Fatal(err)
		}
	}
	ledger, err := ReconcileLedger(project, 10, now)
	if err != nil {
		t.Fatal(err)
	}
	page, err := QuerySessionWorkload(project, ledger, 1, 1, 10, now)
	if err != nil {
		t.Fatal(err)
	}
	if page.Active.Total != 0 || page.Historical.Total != 1 || page.Historical.Items[0].LastUsedAt != old || page.Historical.Items[0].DisplayName != "Fresh name" {
		t.Fatalf("%+v", page)
	}
	if err := AppendExecutionEvent(project, ExecutionEvent{EventID: "reuse", EventKind: "activity", AttemptID: "a", Provider: "codex", SessionID: "s", ObservedAt: now.Format(time.RFC3339Nano), EvidenceSource: EvidenceHook, ObservationQuality: QualityObserved}); err != nil {
		t.Fatal(err)
	}
	ledger, err = ReconcileLedger(project, 10, now)
	if err != nil {
		t.Fatal(err)
	}
	page, err = QuerySessionWorkload(project, ledger, 1, 1, 10, now)
	if err != nil {
		t.Fatal(err)
	}
	if page.Active.Total != 1 || page.Historical.Total != 0 || page.Active.Items[0].RootSessionID != page.LatestSessionID {
		t.Fatalf("%+v", page)
	}
}

func TestSessionUsageLargeInventory(t *testing.T) {
	project := t.TempDir()
	now := time.Date(2026, 11, 16, 3, 0, 0, 0, time.UTC)
	ledger := Ledger{Attempts: make([]Attempt, 20000)}
	for i := range ledger.Attempts {
		ledger.Attempts[i] = Attempt{AttemptID: fmt.Sprint(i), Provider: "codex", SessionID: fmt.Sprint(i), Terminal: true, CurrentState: StateCompleted}
	}
	first, err := QuerySessionWorkload(project, ledger, 1, 1, 10, now)
	if err != nil {
		t.Fatal(err)
	}
	last, err := QuerySessionWorkload(project, ledger, 1, 2000, 10, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.Active.Total != 0 || first.Historical.Total != 20000 || first.UnknownUsage != 20000 || last.Historical.Page != 2000 || len(last.Historical.Items) != 10 {
		t.Fatalf("large inventory: %+v", last)
	}
	if first.Historical.Items[0].RootSessionID == last.Historical.Items[0].RootSessionID {
		t.Fatal("last page repeated first")
	}
}
