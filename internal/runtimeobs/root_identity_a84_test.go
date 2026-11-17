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
