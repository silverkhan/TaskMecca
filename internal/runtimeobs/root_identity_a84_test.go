package runtimeobs

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/orchestration"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestA84HostTitleOutranksNativeName(t *testing.T) {
	oldNative, oldHost := resolveCodexThreadMetadata, resolveT3RootTitle
	t.Cleanup(func() { resolveCodexThreadMetadata = oldNative; resolveT3RootTitle = oldHost })
	resolveCodexThreadMetadata = func(context.Context, string) (CodexThreadMetadata, error) {
		return CodexThreadMetadata{ID: "native", Name: "native Worker name"}, nil
	}
	resolveT3RootTitle = func(context.Context, string, string, time.Time) (t3RootTitle, error) {
		return t3RootTitle{ThreadID: "host-thread", Title: "Official host title"}, nil
	}
	got := observeRootName(t.TempDir(), "native", time.Now())
	if got.Name != "Official host title" || got.Source != "t3_title" {
		t.Fatalf("host title must win, got %+v", got)
	}
}

func TestA84ExactRootHierarchyPreservesNativeGroupsAndUsage(t *testing.T) {
	project := t.TempDir()
	now := time.Date(2026, 11, 17, 5, 0, 0, 0, time.UTC)
	// Saved immutable Root identities are independent of display labels.
	save := func(native, id string) {
		dir := filepath.Join(project, "_task_mecca", "data", "orchestration", "groups")
		if e := os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
		b, _ := json.Marshal(orchestration.Group{SchemaVersion: 1, GroupID: id, DisplayPrefix: id + "_", Mode: orchestration.ModeB, Provider: "codex", RootSessionID: native, CreatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano)})
		if e := os.WriteFile(filepath.Join(dir, id+".json"), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	save("root", "D1112_01")
	save("root2", "D1116_01")
	attempts := []Attempt{}
	metadata := map[string]rootSessionMetadata{}
	roots := []RootSession{}
	add := func(native, thread, parent, role, used string) {
		a := Attempt{AttemptID: native, Provider: "codex", SessionID: native, RuntimeAgentID: native, AgentPath: role, BindingState: BindingBound, HookActivityCount: 1, LastHookActivityAt: used}
		attempts = append(attempts, a)
		roots = append(roots, RootSession{RootSessionID: RootSessionIDForAttempt(a), Provider: "codex", ProviderSessionID: native, LastUsedAt: used, DisplayName: "misleading Worker Root title"})
		metadata["codex\x00"+native] = rootSessionMetadata{nameSnapshot: &RootNameObservation{Version: 1, Name: "host-title", Source: "t3_title", Status: "verified", HostStatus: "verified", IdentityVerified: true, T3ThreadID: thread, ParentThreadID: parent, Relationship: "subagent", HostProjectID: "project", CheckedAt: now.Format(time.RFC3339Nano)}}
		if parent == "" {
			metadata["codex\x00"+native].nameSnapshot.Relationship = ""
		}
	}
	add("root", "host-root", "", "/root", now.Add(-60*time.Hour).Format(time.RFC3339Nano))
	add("controller", "host-controller", "host-root", "/root/controller", now.Add(-time.Minute).Format(time.RFC3339Nano))
	add("worker", "host-worker", "host-controller", "/root/controller/compact_spec", now.Format(time.RFC3339Nano))
	add("unknown", "host-unknown", "host-root", "", now.Format(time.RFC3339Nano))
	add("root2", "host-root2", "", "/root", now.Add(-time.Hour).Format(time.RFC3339Nano))
	add("foreign", "host-foreign", "host-root", "/root/controller/other", now.Format(time.RFC3339Nano))
	metadata["codex\x00foreign"].nameSnapshot.HostProjectID = "foreign-project"
	for _, a := range attempts {
		snap := metadata["codex\x00"+a.SessionID].nameSnapshot
		if e := AppendExecutionEvent(project, ExecutionEvent{EventKind: "session_metadata", Provider: "codex", SessionID: a.SessionID, AttemptID: "meta-" + a.SessionID, ObservedAt: now.Format(time.RFC3339Nano), RootName: snap}); e != nil {
			t.Fatal(e)
		}
	}
	workload, e := QuerySessionWorkload(project, Ledger{Attempts: attempts}, 1, 1, 10, now)
	if e != nil {
		t.Fatal(e)
	}
	if workload.LatestSessionID != roots[0].RootSessionID && workload.LatestSessionID != roots[4].RootSessionID {
		t.Fatalf("newest worker selected: %+v", workload)
	}
	if workload.Representative == nil || workload.Representative.Role != "root" || workload.Active.Total+workload.Historical.Total != 6 {
		t.Fatalf("representative/pages: %+v", workload)
	}
	before := roots[0].LastUsedAt
	applyRootSessionRelations(project, roots, attempts, metadata, now)
	if len(roots) != 6 || roots[0].Role != "root" || roots[4].Role != "root" || roots[1].Role != "controller" || roots[2].Role != "worker" || roots[3].Role != "unknown" || roots[5].RelationStatus == "verified" {
		t.Fatalf("roles/retention: %+v", roots)
	}
	if roots[2].ParentSessionID != roots[1].RootSessionID || roots[2].RootAncestorID != roots[0].RootSessionID || roots[0].LastUsedAt != before || roots[0].DescendantLastUsedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("hierarchy/own usage: %+v", roots)
	}
	metadata["codex\x00controller"].nameSnapshot.CheckedAt = now.Add(-3 * time.Minute).Format(time.RFC3339Nano)
	applyRootSessionRelations(project, roots, attempts, metadata, now)
	if roots[2].RelationStatus == "verified" {
		t.Fatal("stale intermediate linkage accepted")
	}
}

func TestA84NameDeadlineReachesBothReaders(t *testing.T) {
	oldNative, oldHost := resolveCodexThreadMetadata, resolveT3RootTitle
	t.Cleanup(func() { resolveCodexThreadMetadata = oldNative; resolveT3RootTitle = oldHost })
	resolveT3RootTitle = func(ctx context.Context, _ string, _ string, _ time.Time) (t3RootTitle, error) {
		<-ctx.Done()
		return t3RootTitle{}, ctx.Err()
	}
	nativeCalls := 0
	resolveCodexThreadMetadata = func(context.Context, string) (CodexThreadMetadata, error) {
		nativeCalls++
		return CodexThreadMetadata{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	got := observeRootNameContext(ctx, t.TempDir(), "native", time.Now())
	if time.Since(start) > time.Second || nativeCalls != 0 || got.Failure != "name_refresh_deferred" {
		t.Fatalf("deadline/call bound: %+v calls=%d", got, nativeCalls)
	}
}

func TestA84HostLineageRejectsWrongProjectParentAndEmptyTitleIsCandidate(t *testing.T) {
	store, sql := t3Fixture(t)
	project := t.TempDir()
	sql(`ALTER TABLE orchestration_v2_projection_threads ADD COLUMN title TEXT; ALTER TABLE orchestration_v2_projection_threads ADD COLUMN project_id TEXT; ALTER TABLE orchestration_v2_projection_threads ADD COLUMN payload_json TEXT; CREATE TABLE projection_projects(project_id TEXT,workspace_root TEXT,deleted_at TEXT,archived_at TEXT); UPDATE orchestration_v2_projection_threads SET title='Host title',project_id='p',payload_json='{"lineage":{"parentThreadId":"parent","relationshipToParent":"subagent"}}'; INSERT INTO projection_projects(project_id,workspace_root,deleted_at) VALUES('p',` + sqlLiteral(project) + `,NULL); INSERT INTO orchestration_v2_projection_threads(thread_id,project_id) VALUES('parent','foreign');`)
	now := time.Date(2026, 11, 12, 1, 0, 1, 0, time.UTC)
	if _, e := store.rootTitle(project, "native-session", now); e == nil {
		t.Fatal("cross-project parent accepted")
	}
	sql(`UPDATE orchestration_v2_projection_threads SET project_id='p' WHERE thread_id='parent';`)
	if _, e := store.rootTitle(project, "native-session", now); e != nil {
		t.Fatal(e)
	}
	sql(`UPDATE orchestration_v2_projection_threads SET deleted_at='deleted' WHERE thread_id='parent';`)
	if _, e := store.rootTitle(project, "native-session", now); e == nil {
		t.Fatal("deleted parent accepted")
	}
	oldN, oldH := resolveCodexThreadMetadata, resolveT3RootTitle
	t.Cleanup(func() { resolveCodexThreadMetadata = oldN; resolveT3RootTitle = oldH })
	resolveT3RootTitle = func(context.Context, string, string, time.Time) (t3RootTitle, error) {
		return t3RootTitle{ThreadID: "thread", Title: ""}, nil
	}
	resolveCodexThreadMetadata = func(context.Context, string) (CodexThreadMetadata, error) {
		return CodexThreadMetadata{ID: "native", Name: "native candidate"}, nil
	}
	got := observeRootName(project, "native", now)
	if got.Source != "provider_name" || got.HostStatus != "empty" || got.Failure != "host_title_empty" {
		t.Fatalf("empty candidate: %+v", got)
	}
}

func TestA84NamesRefreshCapsBatchWithoutChangingAttempts(t *testing.T) {
	oldN, oldH := resolveCodexThreadMetadata, resolveT3RootTitle
	t.Cleanup(func() { resolveCodexThreadMetadata = oldN; resolveT3RootTitle = oldH })
	hostCalls, nativeCalls := 0, 0
	resolveT3RootTitle = func(context.Context, string, string, time.Time) (t3RootTitle, error) {
		hostCalls++
		return t3RootTitle{ThreadID: "fixture-host", Title: "official fixture"}, nil
	}
	resolveCodexThreadMetadata = func(_ context.Context, id string) (CodexThreadMetadata, error) {
		nativeCalls++
		return CodexThreadMetadata{ID: id, Name: "native fixture"}, nil
	}
	now := time.Now().UTC()
	ledger := Ledger{}
	for i := 0; i < 40; i++ {
		id := fmt.Sprintf("session-%02d", i)
		ledger.Attempts = append(ledger.Attempts, Attempt{AttemptID: id, Provider: "codex", SessionID: id, RuntimeAgentID: id, CurrentState: StateCompleted, Terminal: true})
	}
	before, _ := json.Marshal(ledger)
	changed, err := RefreshCodexRootNames(t.TempDir(), ledger, now)
	after, _ := json.Marshal(ledger)
	if err != nil || !changed || hostCalls != 16 || nativeCalls != 16 || string(before) != string(after) {
		t.Fatalf("batch changed=%v err=%v host=%d native=%d", changed, err, hostCalls, nativeCalls)
	}
}

func TestA84InvalidNameSnapshotCannotProveHierarchy(t *testing.T) {
	for _, v := range []RootNameObservation{
		{Version: 2, IdentityVerified: true, Source: "t3_title", Status: "verified", Name: "host", T3ThreadID: "t", HostProjectID: "p", HostStatus: "verified"},
		{Version: 1, IdentityVerified: true, Source: "t3_title", Status: "verified", Name: "host", T3ThreadID: "t", HostStatus: "verified"},
		{Version: 1, IdentityVerified: true, Source: "t3_title", Status: "verified", Name: "host", T3ThreadID: "t", HostProjectID: "p", HostStatus: "verified", ParentThreadID: "parent"},
	} {
		if v.displaySnapshot().IdentityVerified {
			t.Fatalf("invalid identity accepted: %+v", v)
		}
	}
}

func TestA84RootOwnObservationPinsCanonicalWithoutChangingStoredState(t *testing.T) {
	store, sql := t3Fixture(t)
	project := t.TempDir()
	t.Setenv("HOME", filepath.Dir(filepath.Dir(store.dir)))
	sql(`ALTER TABLE orchestration_v2_projection_threads ADD COLUMN project_id TEXT; UPDATE orchestration_v2_projection_threads SET project_id='p'; CREATE TABLE projection_projects(project_id TEXT,workspace_root TEXT,deleted_at TEXT,archived_at TEXT); INSERT INTO projection_projects VALUES('p',` + sqlLiteral(project) + `,NULL,NULL);`)
	root := RootSession{CanonicalProject: project, Role: "root", RelationStatus: "verified", T3ThreadID: "thread", Provider: "codex", ProviderSessionID: "native-session", Status: RootSessionTerminal, LastUsedAt: "2026-11-10T00:00:00Z"}
	now := time.Date(2026, 11, 12, 1, 0, 1, 0, time.UTC)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	got := ObserveRootSessionState(ctx, root, now)
	if !got.OfficialCurrent || got.State != StateRunning || root.Status != RootSessionTerminal || root.LastUsedAt != "2026-11-10T00:00:00Z" {
		t.Fatalf("own overlay %+v root %+v", got, root)
	}
	sql(`UPDATE projection_projects SET workspace_root='/other-project';`)
	got = ObserveRootSessionState(ctx, root, now)
	if got.OfficialCurrent || got.FailedStage != "official_root_observation" {
		t.Fatalf("wrong project accepted %+v", got)
	}
}

func TestA84ReviewStaleNativeCannotBorrowFreshThreadIdentity(t *testing.T) {
	project := t.TempDir()
	now := time.Now().UTC()
	dir := filepath.Join(project, "_task_mecca", "data", "orchestration", "groups")
	os.MkdirAll(dir, 0700)
	b, _ := json.Marshal(orchestration.Group{SchemaVersion: 1, GroupID: "D1112_01", DisplayPrefix: "D1112_01_", Mode: orchestration.ModeB, Provider: "codex", RootSessionID: "root", CreatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano)})
	os.WriteFile(filepath.Join(dir, "D1112_01.json"), b, 0600)
	roots := []RootSession{}
	attempts := []Attempt{}
	metadata := map[string]rootSessionMetadata{}
	for _, id := range []string{"root", "old-native", "new-native"} {
		a := Attempt{Provider: "codex", SessionID: id, RuntimeAgentID: id, BindingState: BindingBound, AgentPath: "/root/controller/worker"}
		if id == "root" {
			a.AgentPath = "/root"
		}
		attempts = append(attempts, a)
		roots = append(roots, RootSession{RootSessionID: RootSessionIDForAttempt(a), Provider: "codex", ProviderSessionID: id})
		s := &RootNameObservation{Version: 1, Source: "t3_title", Status: "verified", HostStatus: "verified", IdentityVerified: true, Name: id, HostProjectID: "project", CheckedAt: now.Format(time.RFC3339Nano), T3ThreadID: "worker-thread", ParentThreadID: "root-thread", Relationship: "subagent"}
		if id == "root" {
			s.T3ThreadID = "root-thread"
			s.ParentThreadID = ""
			s.Relationship = ""
		}
		if id == "old-native" {
			s.CheckedAt = now.Add(-3 * time.Minute).Format(time.RFC3339Nano)
		}
		metadata["codex\x00"+id] = rootSessionMetadata{nameSnapshot: s}
	}
	applyRootSessionRelations(project, roots, attempts, metadata, now)
	t.Logf("old=%+v new=%+v", roots[1], roots[2])
	if roots[1].RelationStatus == "verified" {
		t.Fatal("STALE native snapshot borrowed another native session's fresh thread identity")
	}
}

func TestA84RepairNativeRowEligibilityAndDescendantUsage(t *testing.T) {
	now := time.Date(2026, 11, 17, 6, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name               string
		change             func(*RootNameObservation)
		freshAliasVerified bool
	}{
		{"stale", func(s *RootNameObservation) { s.CheckedAt = now.Add(-3 * time.Minute).Format(time.RFC3339Nano) }, true},
		{"future", func(s *RootNameObservation) { s.CheckedAt = now.Add(time.Second).Format(time.RFC3339Nano) }, true},
		{"invalid-time", func(s *RootNameObservation) { s.CheckedAt = "not-a-time" }, true},
		{"unverified-identity", func(s *RootNameObservation) { s.IdentityVerified = false }, true},
		{"missing-project", func(s *RootNameObservation) { s.HostProjectID = "" }, true},
		{"missing-thread", func(s *RootNameObservation) { s.T3ThreadID = "" }, true},
		{"two-fresh-natives-ambiguous", func(*RootNameObservation) {}, false},
		{"fresh-foreign-project-ambiguous", func(s *RootNameObservation) { s.HostProjectID = "foreign" }, false},
	} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reverse=%v", tc.name, reverse), func(t *testing.T) {
				project := t.TempDir()
				dir := filepath.Join(project, "_task_mecca", "data", "orchestration", "groups")
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				group := orchestration.Group{SchemaVersion: 1, GroupID: "D1112_01", DisplayPrefix: "D1112_01_", Mode: orchestration.ModeB, Provider: "codex", RootSessionID: "root", CreatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano)}
				b, err := json.Marshal(group)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(dir, "D1112_01.json"), b, 0600); err != nil {
					t.Fatal(err)
				}
				roots := []RootSession{}
				attempts := []Attempt{}
				metadata := map[string]rootSessionMetadata{}
				for _, native := range []string{"root", "rejected-native", "fresh-native", "leaf"} {
					agent, thread, parent := "/root/controller", "controller-thread", "root-thread"
					if native == "root" {
						agent, thread, parent = "/root", "root-thread", ""
					}
					if native == "leaf" {
						agent, thread, parent = "/root/controller/worker", "leaf-thread", "controller-thread"
					}
					a := Attempt{AttemptID: native, Provider: "codex", SessionID: native, RuntimeAgentID: native, BindingState: BindingBound, AgentPath: agent}
					attempts = append(attempts, a)
					used := now.Add(-time.Hour).Format(time.RFC3339Nano)
					if native == "root" {
						used = now.Add(-4 * time.Hour).Format(time.RFC3339Nano)
					}
					if native == "rejected-native" {
						used = now.Format(time.RFC3339Nano)
					}
					if native == "leaf" {
						used = now.Add(-30 * time.Minute).Format(time.RFC3339Nano)
					}
					roots = append(roots, RootSession{RootSessionID: RootSessionIDForAttempt(a), Provider: "codex", ProviderSessionID: native, LastUsedAt: used})
					s := &RootNameObservation{Version: 1, Name: native, Source: "t3_title", Status: "verified", HostStatus: "verified", IdentityVerified: true, T3ThreadID: thread, ParentThreadID: parent, Relationship: "subagent", HostProjectID: "project", CheckedAt: now.Format(time.RFC3339Nano)}
					if parent == "" {
						s.Relationship = ""
					}
					if native == "rejected-native" {
						tc.change(s)
					}
					metadata["codex\x00"+native] = rootSessionMetadata{nameSnapshot: s}
				}
				if reverse {
					for left, right := 0, len(roots)-1; left < right; left, right = left+1, right-1 {
						roots[left], roots[right] = roots[right], roots[left]
					}
				}
				applyRootSessionRelations(project, roots, attempts, metadata, now)
				byNative := map[string]RootSession{}
				for _, r := range roots {
					byNative[r.ProviderSessionID] = r
				}
				rejected, fresh, leaf, root := byNative["rejected-native"], byNative["fresh-native"], byNative["leaf"], byNative["root"]
				if rejected.RelationStatus == "verified" || rejected.Role != "unknown" || rejected.GroupID != "" || rejected.RootAncestorID != "" || rejected.ParentSessionID != "" {
					t.Fatalf("rejected row borrowed identity: %+v", rejected)
				}
				if (fresh.RelationStatus == "verified") != tc.freshAliasVerified || (leaf.RelationStatus == "verified") != tc.freshAliasVerified {
					t.Fatalf("fresh/chain result fresh=%+v leaf=%+v", fresh, leaf)
				}
				wantDesc := ""
				if tc.freshAliasVerified {
					wantDesc = now.Add(-30 * time.Minute).Format(time.RFC3339Nano)
					if leaf.ParentSessionID != fresh.RootSessionID {
						t.Fatal("leaf did not use exact fresh parent row")
					}
				}
				if root.DescendantLastUsedAt != wantDesc || root.LastUsedAt != now.Add(-4*time.Hour).Format(time.RFC3339Nano) {
					t.Fatalf("rejected row changed Root usage: %+v", root)
				}
			})
		}
	}
}
