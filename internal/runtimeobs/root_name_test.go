package runtimeobs

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestA75RootTitleRequiresOwnedExactMapping(t *testing.T) {
	store, sql := t3Fixture(t)
	project := t.TempDir()
	sql(`ALTER TABLE orchestration_v2_projection_threads ADD COLUMN title TEXT;
ALTER TABLE orchestration_v2_projection_threads ADD COLUMN project_id TEXT;
CREATE TABLE projection_projects(project_id TEXT,workspace_root TEXT,deleted_at TEXT);
UPDATE orchestration_v2_projection_threads SET title='🌱 1110_EMPFUND',project_id='project';
INSERT INTO projection_projects VALUES('project',` + sqlLiteral(project) + `,NULL);`)
	now := time.Date(2026, 11, 12, 1, 0, 1, 0, time.UTC)
	got, err := store.rootTitle(project, "native-session", now)
	if err != nil || got.Title != "🌱 1110_EMPFUND" || got.ThreadID != "thread" {
		t.Fatalf("%+v %v", got, err)
	}
	sql(`UPDATE orchestration_v2_projection_provider_threads SET thread_id='foreign';`)
	if _, err := store.rootTitle(project, "native-session", now); err == nil {
		t.Fatal("foreign T3 owner accepted")
	}
	sql(`UPDATE orchestration_v2_projection_provider_threads SET thread_id='thread';`)
	for _, bad := range []struct{ project, native string }{{t.TempDir(), "native-session"}, {project, "other-native"}} {
		if _, err := store.rootTitle(bad.project, bad.native, now); err == nil {
			t.Fatal("wrong mapping accepted")
		}
	}
	sql(`UPDATE orchestration_v2_projection_threads SET title='새 제목';`)
	got, err = store.rootTitle(project, "native-session", now)
	if err != nil || got.Title != "새 제목" {
		t.Fatalf("rename: %+v %v", got, err)
	}
	sql(`UPDATE orchestration_v2_projection_threads SET title='';`)
	got, err = store.rootTitle(project, "native-session", now)
	if err != nil || got.Title != "" {
		t.Fatalf("empty: %+v %v", got, err)
	}
	sql(`INSERT INTO orchestration_v2_projection_threads VALUES('other-thread',NULL,NULL,'ambiguous','project');
INSERT INTO orchestration_v2_projection_run_attempts(thread_id,provider,payload_json) VALUES('other-thread','codex','{"nativeThreadId":"native-session"}');`)
	if _, err = store.rootTitle(project, "native-session", now); err == nil {
		t.Fatal("ambiguous mapping accepted")
	}
	sql(`DELETE FROM orchestration_v2_projection_threads WHERE thread_id='other-thread'; UPDATE orchestration_v2_projection_threads SET deleted_at='deleted';`)
	if _, err = store.rootTitle(project, "native-session", now); err == nil {
		t.Fatal("deleted thread accepted")
	}
}

func TestA75NameRefreshTransitionsDoNotRetainPromptOrNames(t *testing.T) {
	project := t.TempDir()
	now := time.Date(2026, 11, 12, 2, 0, 0, 0, time.UTC)
	e := ExecutionEvent{EventKind: "state", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: "attempt", Provider: "codex", SessionID: "native", RuntimeAgentID: "native", SessionName: "legacy", SessionTitle: "first prompt", State: StateRunning, EvidenceSource: EvidenceHook, ObservationQuality: QualityObserved}
	if err := AppendExecutionEvent(project, e); err != nil {
		t.Fatal(err)
	}
	ledger, err := BuildLedger(project, 10, now)
	if err != nil {
		t.Fatal(err)
	}
	readNative, readT3 := resolveCodexThreadMetadata, resolveT3RootTitle
	t.Cleanup(func() { resolveCodexThreadMetadata = readNative; resolveT3RootTitle = readT3 })
	name, title := "official", "T3 title"
	var nativeErr, t3Err error
	resolveCodexThreadMetadata = func(context.Context, string) (CodexThreadMetadata, error) {
		return CodexThreadMetadata{ID: "native", Name: name}, nativeErr
	}
	resolveT3RootTitle = func(string, string, time.Time) (t3RootTitle, error) {
		return t3RootTitle{ThreadID: "thread", Title: title}, t3Err
	}
	check := func(want, source, status string) {
		t.Helper()
		now = now.Add(31 * time.Second)
		baseline, err := BuildLedger(project, 10, now)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := RefreshCodexRootNames(project, ledger, now); err != nil {
			t.Fatal(err)
		}
		fresh, err := BuildLedger(project, 10, now)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(baseline.Attempts, fresh.Attempts) {
			t.Fatal("display refresh mutated execution attempts")
		}
		roots, err := BuildRootSessions(project, ledger, now)
		if err != nil {
			t.Fatal(err)
		}
		r := roots.Items[0]
		if want == "" {
			want = rootFallbackName(r.CreatedAt)
		}
		if r.DisplayName != want || r.DisplayNameSource != source || r.DisplayNameStatus != status || r.DisplayNameCheckedAt != now.Format(time.RFC3339Nano) {
			t.Fatalf("root %+v", r)
		}
		if r.RootSessionID != rootSessionIDFor("codex", "native") || r.AttemptIDs[0] != "attempt" || r.CurrentCount != 1 {
			t.Fatalf("identity/liveness changed: %+v", r)
		}
	}
	check("official", "provider_name", "verified")
	name = ""
	check("T3 title", "t3_title", "verified")
	title = "변경된 T3 제목"
	check(title, "t3_title", "verified")
	title = ""
	check("", "task_mecca_fallback", "empty")
	t3Err = errors.New("unsupported store")
	check("", "task_mecca_fallback", "unavailable")
	t3Err = nil
	title = "T3 title"
	check(title, "t3_title", "verified")
	nativeErr = errors.New("official read failed")
	check("", "task_mecca_fallback", "unavailable")
	nativeErr = nil
	name = "new official name"
	check(name, "provider_name", "verified")
}

func TestA75SnapshotVersionFailsClosed(t *testing.T) {
	for _, snapshot := range []RootNameObservation{
		{Version: 2, Name: "stale", Source: "provider_name", Status: "verified"},
		{Version: 1, Name: "unverified", Source: "t3_title", Status: "verified"},
		{Version: 1, Name: "old", Source: "provider_name", Status: "unavailable"},
	} {
		got := snapshot.displaySnapshot()
		if got.Name != "" || got.Source != "task_mecca_fallback" || got.Status != "unavailable" {
			t.Fatalf("%+v", got)
		}
	}
}

func TestA75VersionTransitionClearsOlderVerifiedSnapshot(t *testing.T) {
	project := t.TempDir()
	now := time.Date(2026, 11, 12, 3, 0, 0, 0, time.UTC)
	e := ExecutionEvent{EventKind: "state", ObservedAt: now.Format(time.RFC3339Nano), Provider: "codex", SessionID: "native", RuntimeAgentID: "native", AttemptID: "attempt", State: StateRunning, SessionName: "old hook name", SessionTitle: "first prompt", EvidenceSource: EvidenceHook}
	if err := AppendExecutionEvent(project, e); err != nil {
		t.Fatal(err)
	}
	ledger, err := BuildLedger(project, 10, now)
	if err != nil {
		t.Fatal(err)
	}
	for i, snapshot := range []RootNameObservation{
		{Version: 1, Name: "verified native", Source: "provider_name", Status: "verified"},
		{Version: 2, Name: "unsupported new format", Source: "provider_name", Status: "verified"},
		{Version: 1, Name: "restored T3", Source: "t3_title", Status: "verified", T3ThreadID: "thread"},
	} {
		at := now.Add(time.Duration(i+1) * time.Minute)
		snapshot.CheckedAt = at.Format(time.RFC3339Nano)
		meta := ExecutionEvent{EventKind: "session_metadata", ObservedAt: snapshot.CheckedAt, Provider: "codex", SessionID: "native", AttemptID: "rootmeta", RootName: &snapshot}
		if err := AppendExecutionEvent(project, meta); err != nil {
			t.Fatal(err)
		}
		roots, err := BuildRootSessions(project, ledger, at)
		if err != nil {
			t.Fatal(err)
		}
		r := roots.Items[0]
		if i == 1 {
			if r.DisplayNameSource != "task_mecca_fallback" || r.DisplayNameStatus != "unavailable" {
				t.Fatalf("unsupported version retained old name: %+v", r)
			}
		} else if r.DisplayName != snapshot.Name || r.DisplayNameSource != snapshot.Source {
			t.Fatalf("source transition: %+v", r)
		}
	}
}

func TestA75UnsupportedT3StoreFailsClosed(t *testing.T) {
	s, _ := t3Fixture(t)
	if _, err := s.rootTitle(t.TempDir(), "native-session", time.Now()); err == nil {
		t.Fatal("unsupported schema accepted")
	}
}
