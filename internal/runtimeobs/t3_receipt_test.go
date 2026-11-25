package runtimeobs

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestT3CurrentExecutionContextCancelsAndReapsChild(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"statev2.sqlite", "server-owner.sqlite"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	bin := t.TempDir()
	pidPath := filepath.Join(bin, "pid")
	// exec ensures the actual SQL replacement is the owned child, so its PID
	// must be reaped when CommandContext returns, not an abandoned grandchild.
	script := "#!/bin/sh\necho $$ > '" + pidPath + "'\nexec /bin/sleep 10\n"
	if err := os.WriteFile(filepath.Join(bin, "sqlite3"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	base := t3Store{dir: dir}
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := base.withContext(ctx).query("SELECT 1")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2*time.Second || base.ctx != nil {
		t.Fatalf("deadline/copy failed: %v elapsed %v", err, time.Since(start))
	}
	pid, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	if exec.Command("/bin/kill", "-0", strings.TrimSpace(string(pid))).Run() == nil {
		t.Fatal("cancelled SQL child still exists")
	}
	if base.withContext(nil).ctx != nil {
		t.Fatal("legacy nil context changed")
	}
}

func t3Fixture(t *testing.T) (t3Store, func(string)) {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 unavailable")
	}
	dir := filepath.Join(t.TempDir(), ".t3", "userdata")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	store := t3Store{dir: dir}
	sql := func(q string) {
		t.Helper()
		out, err := exec.Command("sqlite3", filepath.Join(dir, "statev2.sqlite"), q).CombinedOutput()
		if err != nil {
			t.Fatalf("fixture: %s %v", out, err)
		}
	}
	sql(`CREATE TABLE orchestration_v2_projection_threads(thread_id TEXT,deleted_at TEXT,archived_at TEXT);
 CREATE TABLE orchestration_v2_projection_runs(thread_id TEXT,run_id TEXT,provider TEXT,status TEXT,completed_at TEXT,ordinal INTEGER,provider_thread_id TEXT);
 CREATE TABLE orchestration_v2_projection_run_attempts(thread_id TEXT,run_id TEXT,attempt_id TEXT,status TEXT,payload_json TEXT,attempt_ordinal INTEGER,provider_thread_id TEXT,provider TEXT DEFAULT 'codex');
 CREATE TABLE orchestration_v2_projection_provider_threads(provider_thread_id TEXT,thread_id TEXT,driver TEXT,status TEXT);
 INSERT INTO orchestration_v2_projection_threads VALUES('thread',NULL,NULL);
 INSERT INTO orchestration_v2_projection_provider_threads VALUES('provider-thread','thread','codex','active');
 INSERT INTO orchestration_v2_projection_runs VALUES('thread','run-1','codex','running',NULL,1,'provider-thread');
 INSERT INTO orchestration_v2_projection_run_attempts(thread_id,run_id,attempt_id,status,payload_json,attempt_ordinal,provider_thread_id) VALUES('thread','run-1','native-attempt-1','running','{"nativeThreadId":"native-session","startedAt":"2026-11-12T01:00:00Z"}',1,'provider-thread');`)
	owner := exec.Command("sqlite3", filepath.Join(dir, "server-owner.sqlite"))
	in, err := owner.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := owner.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = owner.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Close(); _ = owner.Wait() })
	fmt.Fprintln(in, "BEGIN EXCLUSIVE; SELECT 'ready';")
	scan := bufio.NewScanner(out)
	if !scan.Scan() || scan.Text() != "ready" {
		t.Fatal("owner fixture lock")
	}
	return store, sql
}
func testT3Selector() T3Selector {
	return T3Selector{ThreadID: "thread", SessionID: "native-session", RuntimeAgentID: "native-session"}
}

func TestA102TerminalHostStatusMatrix(t *testing.T) {
	now := time.Date(2026, 11, 12, 1, 2, 0, 0, time.UTC)
	for status, state := range map[string]CanonicalState{"completed": StateCompleted, "cancelled": StateInterrupted, "interrupted": StateInterrupted, "errored": StateErrored, "failed": StateErrored} {
		for _, mode := range []string{"current", "historical", "missing_native_end", "missing_run_end", "run_status_conflict", "run_end_before_native", "native_end_before_start", "future_native_end", "future_run_end", "unknown"} {
			t.Run(status+"/"+mode, func(t *testing.T) {
				store, sql := t3Fixture(t)
				sql(fmt.Sprintf(`UPDATE orchestration_v2_projection_runs SET status='%s',completed_at='2026-11-12T01:01:01.010Z'; UPDATE orchestration_v2_projection_run_attempts SET status='%s',payload_json=json_set(payload_json,'$.completedAt','2026-11-12T01:01:01Z');`, status, status))
				sel := testT3Selector()
				historical := mode == "historical"
				switch mode {
				case "historical":
					sel.RunID = "run-1"
					sel.NativeAttemptID = "native-attempt-1"
					sql(`INSERT INTO orchestration_v2_projection_runs VALUES('thread','run-2','codex','running',NULL,2,'provider-thread'); INSERT INTO orchestration_v2_projection_run_attempts(thread_id,run_id,attempt_id,status,payload_json,attempt_ordinal,provider_thread_id) VALUES('thread','run-2','native-attempt-2','running','{"nativeThreadId":"native-session","startedAt":"2026-11-12T01:01:02Z"}',1,'provider-thread');`)
				case "missing_native_end":
					sql(`UPDATE orchestration_v2_projection_run_attempts SET payload_json=json_remove(payload_json,'$.completedAt');`)
				case "missing_run_end":
					sql(`UPDATE orchestration_v2_projection_runs SET completed_at=NULL;`)
				case "run_status_conflict":
					sql(`UPDATE orchestration_v2_projection_runs SET status='running';`)
				case "run_end_before_native":
					sql(`UPDATE orchestration_v2_projection_runs SET completed_at='2026-11-12T01:01:00Z';`)
				case "native_end_before_start":
					sql(`UPDATE orchestration_v2_projection_run_attempts SET payload_json=json_set(payload_json,'$.completedAt','2026-11-12T00:59:59Z');`)
				case "future_native_end":
					sql(`UPDATE orchestration_v2_projection_run_attempts SET payload_json=json_set(payload_json,'$.completedAt','2026-11-12T01:03:00Z');`)
				case "future_run_end":
					sql(`UPDATE orchestration_v2_projection_runs SET completed_at='2026-11-12T01:03:00Z';`)
				case "unknown":
					sql(`UPDATE orchestration_v2_projection_runs SET status='unexpected_terminal'; UPDATE orchestration_v2_projection_run_attempts SET status='unexpected_terminal';`)
				}
				r, err := store.read(sel, historical, now)
				if mode != "current" && mode != "historical" {
					if err == nil {
						t.Fatalf("unverified terminal admitted: %+v", r)
					}
					return
				}
				if err != nil || r.State != state || r.NativeStatus != status || r.RunStatus != status || r.CompletedAt != "2026-11-12T01:01:01Z" || r.RunCompletedAt != "2026-11-12T01:01:01.010Z" || r.StartedAt != "2026-11-12T01:00:00Z" {
					t.Fatalf("raw status/timestamps changed: %+v %v", r, err)
				}
				if historical {
					if _, err := store.read(sel, false, now); err == nil {
						t.Fatal("historical native used as current")
					}
				}
			})
		}
	}
}

func TestA102ActualCancellationStatusRequiresTerminalSourceTimes(t *testing.T) {
	for _, kind := range []string{"valid", "missing_native_end", "missing_run_end", "future_end", "backward_end", "wrong_run_status", "run_end_before_native"} {
		t.Run(kind, func(t *testing.T) {
			store, sql := t3Fixture(t)
			now := time.Date(2026, 11, 12, 1, 2, 0, 0, time.UTC)
			sql(`UPDATE orchestration_v2_projection_runs SET status='cancelled',completed_at='2026-11-12T01:01:01.010Z'; UPDATE orchestration_v2_projection_run_attempts SET status='cancelled',payload_json=json_set(payload_json,'$.completedAt','2026-11-12T01:01:01Z');`)
			switch kind {
			case "missing_native_end":
				sql(`UPDATE orchestration_v2_projection_run_attempts SET payload_json=json_remove(payload_json,'$.completedAt');`)
			case "missing_run_end":
				sql(`UPDATE orchestration_v2_projection_runs SET completed_at=NULL;`)
			case "future_end":
				sql(`UPDATE orchestration_v2_projection_run_attempts SET payload_json=json_set(payload_json,'$.completedAt','2099-01-01T00:00:00Z');`)
			case "backward_end":
				sql(`UPDATE orchestration_v2_projection_run_attempts SET payload_json=json_set(payload_json,'$.completedAt','2026-11-12T00:59:59Z');`)
			case "wrong_run_status":
				sql(`UPDATE orchestration_v2_projection_runs SET status='completed';`)
			case "run_end_before_native":
				sql(`UPDATE orchestration_v2_projection_runs SET completed_at='2026-11-12T01:00:59Z';`)
			}
			r, err := store.read(testT3Selector(), false, now)
			if kind != "valid" {
				if err == nil {
					t.Fatal("unverified cancellation admitted", kind, r)
				}
				return
			}
			if err != nil || r.State != StateInterrupted || r.NativeStatus != "cancelled" || r.RunStatus != "cancelled" || r.CompletedAt != "2026-11-12T01:01:01Z" || r.RunCompletedAt != "2026-11-12T01:01:01.010Z" {
				t.Fatal("cancelled source relabelled/completion latency lost", r, err)
			}
		})
	}
}

func TestA102CancelledPriorEpisodeResumesWithoutFakeStop(t *testing.T) {
	store, sql := t3Fixture(t)
	project := t.TempDir()
	now := time.Date(2026, 11, 12, 1, 1, 0, 0, time.UTC)
	sel := testT3Selector()
	if _, err := BindRuntimeAgent(project, sel.RuntimeAgentID, "FIXTURE-1", "/root/controller", "", now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	first, _, err := observeT3Execution(project, sel, now, store.read)
	if err != nil {
		t.Fatal(err)
	}
	sql(`UPDATE orchestration_v2_projection_runs SET status='cancelled',completed_at='2026-11-12T01:01:01Z';UPDATE orchestration_v2_projection_run_attempts SET status='cancelled',payload_json=json_set(payload_json,'$.completedAt','2026-11-12T01:01:01Z');INSERT INTO orchestration_v2_projection_runs VALUES('thread','run-2','codex','running',NULL,2,'provider-thread');INSERT INTO orchestration_v2_projection_run_attempts(thread_id,run_id,attempt_id,status,payload_json,attempt_ordinal,provider_thread_id) VALUES('thread','run-2','native-attempt-2','running','{"nativeThreadId":"native-session","startedAt":"2026-11-12T01:01:02Z"}',1,'provider-thread');`)
	current, _, err := observeT3Execution(project, sel, now.Add(3*time.Second), store.read)
	if err != nil {
		t.Fatal(err)
	}
	if current.AttemptID == first.AttemptID || current.Terminal || current.TaskID != "FIXTURE-1" {
		t.Fatal("not a distinct actual resumed episode", current)
	}
	ledger, err := BuildLedger(project, 0, now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	oldFound := false
	for _, a := range ledger.Attempts {
		if a.AttemptID == first.AttemptID {
			oldFound = true
			if !a.Terminal || a.CurrentState != StateInterrupted || a.NativeReceipt.NativeStatus != "cancelled" || a.NativeReceipt.CompletedAt != "2026-11-12T01:01:01Z" {
				t.Fatal("old cancellation rewritten", a)
			}
		}
	}
	if !oldFound {
		t.Fatal("old episode erased")
	}
	if _, err := os.Stat(filepath.Join(project, "_task_mecca", "data", "activity")); !os.IsNotExist(err) {
		t.Fatal("native recovery manufactured Activity")
	}
}

func TestA102AllTerminalPriorEpisodesResumeWithoutFakeStop(t *testing.T) {
	for status, state := range map[string]CanonicalState{"completed": StateCompleted, "cancelled": StateInterrupted, "interrupted": StateInterrupted, "errored": StateErrored, "failed": StateErrored} {
		t.Run(status, func(t *testing.T) {
			store, sql := t3Fixture(t)
			project := t.TempDir()
			now := time.Date(2026, 11, 12, 1, 1, 0, 0, time.UTC)
			sel := testT3Selector()
			if _, err := BindRuntimeAgent(project, sel.RuntimeAgentID, "FIXTURE-1", "/root/controller", "", now.Add(-time.Minute)); err != nil {
				t.Fatal(err)
			}
			first, _, err := observeT3Execution(project, sel, now, store.read)
			if err != nil {
				t.Fatal(err)
			}
			sql(strings.ReplaceAll(`UPDATE orchestration_v2_projection_runs SET status='cancelled',completed_at='2026-11-12T01:01:01Z';UPDATE orchestration_v2_projection_run_attempts SET status='cancelled',payload_json=json_set(payload_json,'$.completedAt','2026-11-12T01:01:01Z');INSERT INTO orchestration_v2_projection_runs VALUES('thread','run-2','codex','running',NULL,2,'provider-thread');INSERT INTO orchestration_v2_projection_run_attempts(thread_id,run_id,attempt_id,status,payload_json,attempt_ordinal,provider_thread_id) VALUES('thread','run-2','native-attempt-2','running','{"nativeThreadId":"native-session","startedAt":"2026-11-12T01:01:02Z"}',1,'provider-thread');`, "cancelled", status))
			current, _, err := observeT3Execution(project, sel, now.Add(3*time.Second), store.read)
			if err != nil {
				t.Fatal(err)
			}
			if current.AttemptID == first.AttemptID || current.Terminal || current.TaskID != "FIXTURE-1" {
				t.Fatal("not a distinct actual resumed episode", current)
			}
			ledger, err := BuildLedger(project, 0, now.Add(3*time.Second))
			if err != nil {
				t.Fatal(err)
			}
			oldFound := false
			for _, a := range ledger.Attempts {
				if a.AttemptID == first.AttemptID {
					oldFound = true
					if !a.Terminal || a.CurrentState != state || a.NativeReceipt.NativeStatus != status || a.NativeReceipt.CompletedAt != "2026-11-12T01:01:01Z" {
						t.Fatal("old cancellation rewritten", a)
					}
				}
			}
			if !oldFound {
				t.Fatal("old episode erased")
			}
			if _, err := os.Stat(filepath.Join(project, "_task_mecca", "data", "activity")); !os.IsNotExist(err) {
				t.Fatal("native recovery manufactured Activity")
			}

		})
	}
}

func TestT3CurrentExecutionCanonicalProjectPins(t *testing.T) {
	store, sql := t3Fixture(t)
	sql(`ALTER TABLE orchestration_v2_projection_threads ADD COLUMN project_id TEXT;
	CREATE TABLE projection_projects(project_id TEXT,workspace_root TEXT,deleted_at TEXT,archived_at TEXT);
	UPDATE orchestration_v2_projection_threads SET project_id='canonical';
	INSERT INTO projection_projects VALUES('canonical','/fixture/canonical',NULL,NULL);`)
	sel := testT3Selector()
	sel.CanonicalProject = "/fixture/canonical"
	now := time.Now().UTC()
	r, err := store.read(sel, false, now)
	if err != nil || r.CanonicalProject != sel.CanonicalProject {
		t.Fatalf("exact canonical project rejected %+v %v", r, err)
	}
	for _, bad := range []string{"/other/project", "/fixture/source-worktree"} {
		wrong := sel
		wrong.CanonicalProject = bad
		if _, err := store.read(wrong, false, now); err == nil {
			t.Fatal("cross-project/source-worktree accepted")
		}
	}
	for _, column := range []string{"deleted_at", "archived_at"} {
		sql("UPDATE projection_projects SET " + column + "='2026-11-17';")
		if _, err := store.read(sel, false, now); err == nil {
			t.Fatalf("%s project accepted", column)
		}
		sql("UPDATE projection_projects SET " + column + "=NULL;")
	}
	// Legacy selectors preserve their previous unscoped observation contract.
	legacy := testT3Selector()
	if r, err := store.read(legacy, false, now); err != nil || r.CanonicalProject != "" {
		t.Fatal("legacy reader changed", err)
	}
}
func TestT3SourceReadsLiveStructuredIdentityAndFailsClosed(t *testing.T) {
	store, sql := t3Fixture(t)
	now := time.Date(2026, 11, 12, 1, 1, 0, 0, time.UTC)
	sel := testT3Selector()
	r, err := store.read(sel, false, now)
	if err != nil || r.State != StateRunning || r.Origin != "t3_local_projection_v2" {
		t.Fatalf("receipt=%+v %v", r, err)
	}
	for _, bad := range []T3Selector{{ThreadID: "wrong", SessionID: sel.SessionID, RuntimeAgentID: sel.RuntimeAgentID}, {ThreadID: sel.ThreadID, SessionID: "wrong", RuntimeAgentID: "wrong"}, {ThreadID: sel.ThreadID, SessionID: sel.SessionID, RuntimeAgentID: "other"}, {ThreadID: sel.ThreadID, SessionID: sel.SessionID, RuntimeAgentID: sel.RuntimeAgentID, RunID: "old-run"}} {
		if _, err = store.read(bad, false, now); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	sql(`UPDATE orchestration_v2_projection_run_attempts SET status='completed';`)
	if _, err = store.read(sel, false, now); err == nil {
		t.Fatal("accepted conflicting terminal/running")
	}
	sql(`UPDATE orchestration_v2_projection_run_attempts SET status='running',payload_json='{"nativeThreadId":"native-session","startedAt":"2099-01-01T00:00:00Z"}';`)
	if _, err = store.read(sel, false, now); err == nil {
		t.Fatal("accepted future start")
	}
	sql(`DROP TABLE orchestration_v2_projection_runs;`)
	if _, err = store.read(sel, false, now); err == nil {
		t.Fatal("accepted unsupported schema")
	}
}
func TestT3PendingDispatchSameEpisodeDuplicateAndRace(t *testing.T) {
	store, _ := t3Fixture(t)
	project := t.TempDir()
	now := time.Date(2026, 11, 12, 1, 1, 0, 0, time.UTC)
	sel := testT3Selector()
	pending, err := BindRuntimeAgent(project, sel.RuntimeAgentID, "A-67", "/root/controller", "", now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if pending.Provider != "" || pending.SessionID != "" || pending.CurrentState != StateRuntimeUnknown {
		t.Fatalf("pending=%+v", pending)
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, _, e := observeT3Execution(project, sel, now, store.read)
			if e != nil {
				t.Error(e)
				return
			}
			if a.AttemptID != pending.AttemptID || a.Provider != "codex" || a.HookActivityCount != 0 || !HasObservedExecution(a) {
				t.Errorf("observed=%+v", a)
			}
		}()
	}
	wg.Wait()
	ledger, err := BuildLedger(project, 20, now)
	if err != nil || len(ledger.Attempts) != 1 || ledger.Attempts[0].EvidenceCount != 3 {
		t.Fatalf("duplicate ledger=%+v %v", ledger, err)
	}
	// Source changes between samples: no state or closure is committed.
	before := ledger.Attempts[0].EvidenceCount
	count := 0
	_, _, err = observeT3Execution(project, sel, now, func(s T3Selector, h bool, n time.Time) (T3Receipt, error) {
		r, e := store.read(s, h, n)
		count++
		if count == 2 {
			r.NativeAttemptID = "raced"
		}
		return r, e
	})
	if err == nil {
		t.Fatal("accepted race")
	}
	ledger, _ = BuildLedger(project, 20, now)
	if ledger.Attempts[0].EvidenceCount != before {
		t.Fatal("race wrote evidence")
	}
}
func TestT3ResumeHasDistinctEpisodeAndNeverReopensTerminal(t *testing.T) {
	store, sql := t3Fixture(t)
	project := t.TempDir()
	now := time.Date(2026, 11, 12, 1, 1, 0, 0, time.UTC)
	sel := testT3Selector()
	_, err := BindRuntimeAgent(project, sel.RuntimeAgentID, "A-67", "/root/controller", "", now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := observeT3Execution(project, sel, now, store.read)
	if err != nil {
		t.Fatal(err)
	}
	sql(`UPDATE orchestration_v2_projection_runs SET status='completed',completed_at='2026-11-12T01:01:01Z'; UPDATE orchestration_v2_projection_run_attempts SET status='completed',payload_json=json_set(payload_json,'$.completedAt','2026-11-12T01:01:01Z');
 INSERT INTO orchestration_v2_projection_runs VALUES('thread','run-2','codex','running',NULL,2,'provider-thread');
 INSERT INTO orchestration_v2_projection_run_attempts(thread_id,run_id,attempt_id,status,payload_json,attempt_ordinal,provider_thread_id) VALUES('thread','run-2','native-attempt-2','running','{"nativeThreadId":"native-session","startedAt":"2026-11-12T01:01:02Z"}',1,'provider-thread');`)
	second, _, err := observeT3Execution(project, sel, now.Add(3*time.Second), store.read)
	if err != nil {
		t.Fatal(err)
	}
	if second.AttemptID == first.AttemptID || second.AgentPath != first.AgentPath || second.TaskID != first.TaskID || !HasObservedExecution(second) {
		t.Fatalf("resume=%+v", second)
	}
	ledger, _ := BuildLedger(project, 20, now.Add(3*time.Second))
	if len(ledger.Attempts) != 2 {
		t.Fatal(ledger)
	}
	for _, a := range ledger.Attempts {
		if a.AttemptID == first.AttemptID && !a.Terminal {
			t.Fatal("old episode reopened")
		}
	}
	old := sel
	old.RunID = "run-1"
	if _, _, err = observeT3Execution(project, old, now.Add(4*time.Second), store.read); err == nil {
		t.Fatal("stale episode accepted")
	}
}
func TestT3ArbitraryJSONCannotAuthorizeAndDeadOwnerCannotObserve(t *testing.T) {
	var sel T3Selector
	if err := json.Unmarshal([]byte(`{"thread_id":"thread","session_id":"native-session","runtime_agent_id":"native-session","state":"running","observed_at":"now"}`), &sel); err != nil {
		t.Fatal(err)
	}
	store, sql := t3Fixture(t)
	sql(`UPDATE orchestration_v2_projection_runs SET status='queued';`)
	if _, err := store.read(sel, false, time.Now()); err == nil {
		t.Fatal("caller state overrode source")
	}
	// No owner lock: even a plausible database cannot prove a live T3 server.
	dir := t.TempDir()
	for _, name := range []string{"statev2.sqlite", "server-owner.sqlite"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, err := (t3Store{dir: dir}).read(sel, false, time.Now())
	if err == nil || !strings.Contains(err.Error(), "ownership") {
		t.Fatalf("owner=%v", err)
	}
}

func TestT3FreshnessAndSourceConflictAtAuthorizationBoundary(t *testing.T) {
	store, sql := t3Fixture(t)
	t.Setenv("HOME", filepath.Dir(filepath.Dir(store.dir)))
	project := t.TempDir()
	now := time.Date(2026, 11, 12, 1, 1, 0, 0, time.UTC)
	sel := testT3Selector()
	a, _, err := ObserveT3Execution(project, sel, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = VerifyT3Attempt(a, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err = VerifyT3Attempt(a, now.Add(3*time.Minute)); err == nil {
		t.Fatal("stale stored observation accepted")
	}
	if err = VerifyT3Attempt(a, now.Add(-time.Second)); err == nil {
		t.Fatal("future observation accepted")
	}
	sql(`UPDATE orchestration_v2_projection_run_attempts SET payload_json=json_set(payload_json,'$.nativeThreadId','another-session');`)
	if err = VerifyT3Attempt(a, now.Add(time.Second)); err == nil {
		t.Fatal("changed identity authorized")
	}
}
func TestT3RetryWithinRunGetsDistinctNativeEpisode(t *testing.T) {
	store, sql := t3Fixture(t)
	project := t.TempDir()
	now := time.Date(2026, 11, 12, 1, 1, 0, 0, time.UTC)
	sel := testT3Selector()
	first, _, err := observeT3Execution(project, sel, now, store.read)
	if err != nil {
		t.Fatal(err)
	}
	sql(`UPDATE orchestration_v2_projection_run_attempts SET status='errored',payload_json=json_set(payload_json,'$.completedAt','2026-11-12T01:01:01Z');INSERT INTO orchestration_v2_projection_run_attempts(thread_id,run_id,attempt_id,status,payload_json,attempt_ordinal,provider_thread_id) VALUES('thread','run-1','native-retry-2','running','{"nativeThreadId":"native-session","startedAt":"2026-11-12T01:01:02Z"}',2,'provider-thread');`)
	retry, _, err := observeT3Execution(project, sel, now.Add(3*time.Second), store.read)
	if err != nil || first.AttemptID == retry.AttemptID {
		t.Fatal(retry, err)
	}
	ledger, _ := BuildLedger(project, 20, now.Add(3*time.Second))
	for _, a := range ledger.Attempts {
		if a.AttemptID == first.AttemptID && (!a.Terminal || a.CurrentState != StateErrored) {
			t.Fatal("retry reopened terminal", a)
		}
	}
}

func TestT3ObservationBeforeBindingRemainsObservedAndPendingResumeIsSafe(t *testing.T) {
	store, sql := t3Fixture(t)
	project := t.TempDir()
	now := time.Date(2026, 11, 12, 1, 1, 0, 0, time.UTC)
	sel := testT3Selector()
	observed, _, err := observeT3Execution(project, sel, now, store.read)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := BindAttempt(project, observed.AttemptID, "A-67", "/root/controller/w", "dispatch", "", nil, now.Add(time.Second))
	if err != nil || !HasObservedExecution(bound) {
		t.Fatal(bound, err)
	}
	sql(`UPDATE orchestration_v2_projection_runs SET status='completed',completed_at='2026-11-12T01:01:02Z';UPDATE orchestration_v2_projection_run_attempts SET status='completed',payload_json=json_set(payload_json,'$.completedAt','2026-11-12T01:01:02Z');`)
	_, _, err = observeT3Execution(project, sel, now.Add(3*time.Second), store.read)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := BindRuntimeAgent(project, sel.RuntimeAgentID, "A-67", "/root/controller/w", "", now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = observeT3Execution(project, sel, now.Add(5*time.Second), store.read); err == nil {
		t.Fatal("pending resume adopted old completed execution")
	}
	sql(`INSERT INTO orchestration_v2_projection_runs VALUES('thread','run-2','codex','running',NULL,2,'provider-thread');INSERT INTO orchestration_v2_projection_run_attempts(thread_id,run_id,attempt_id,status,payload_json,attempt_ordinal,provider_thread_id) VALUES('thread','run-2','native-attempt-2','running','{"nativeThreadId":"native-session","startedAt":"2026-11-12T01:01:06Z"}',1,'provider-thread');`)
	resumed, _, err := observeT3Execution(project, sel, now.Add(7*time.Second), store.read)
	if err != nil || resumed.AttemptID != pending.AttemptID || !HasObservedExecution(resumed) {
		t.Fatal(resumed, err)
	}
	if _, err = BindAttempt(project, resumed.AttemptID, "A-66", "/root/controller/new-worker", "explicit", "", nil, now.Add(8*time.Second)); err != nil {
		t.Fatal(err)
	}
	refreshed, _, err := observeT3Execution(project, sel, now.Add(9*time.Second), store.read)
	if err != nil || refreshed.TaskID != "A-66" || refreshed.AgentPath != "/root/controller/new-worker" {
		t.Fatal("historical binding overwrote current assignment", refreshed, err)
	}
}

func TestA102ResumePreservesHistoricalReadCause(t *testing.T) {
	store, _ := t3Fixture(t)
	project := t.TempDir()
	now := time.Date(2026, 11, 12, 1, 1, 0, 0, time.UTC)
	sel := testT3Selector()
	if _, _, err := observeT3Execution(project, sel, now, store.read); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("actual prior cancellation source unavailable")
	read := func(s T3Selector, historical bool, at time.Time) (T3Receipt, error) {
		if historical {
			return T3Receipt{}, cause
		}
		r, err := store.read(s, historical, at)
		r.RunID = "new-run"
		r.NativeAttemptID = "new-attempt"
		return r, err
	}
	_, _, err := observeT3Execution(project, sel, now.Add(time.Second), read)
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "previous execution episode not verified terminal") {
		t.Fatalf("original error lost: %v", err)
	}
}
