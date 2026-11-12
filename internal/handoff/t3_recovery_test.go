package handoff

import (
	"bufio"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestT3HeldRegistrationRealSourceRecoveryAndCurrentClaimFence(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 unavailable")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".t3", "userdata")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	sql := func(q string) {
		t.Helper()
		b, e := exec.Command("sqlite3", filepath.Join(dir, "statev2.sqlite"), q).CombinedOutput()
		if e != nil {
			t.Fatal(string(b), e)
		}
	}
	sql(`CREATE TABLE orchestration_v2_projection_threads(thread_id TEXT,deleted_at TEXT,archived_at TEXT);
 CREATE TABLE orchestration_v2_projection_runs(thread_id TEXT,run_id TEXT,provider TEXT,status TEXT,completed_at TEXT,ordinal INTEGER,provider_thread_id TEXT);
 CREATE TABLE orchestration_v2_projection_run_attempts(thread_id TEXT,run_id TEXT,attempt_id TEXT,status TEXT,payload_json TEXT,attempt_ordinal INTEGER,provider_thread_id TEXT,provider TEXT DEFAULT 'codex');
 CREATE TABLE orchestration_v2_projection_provider_threads(provider_thread_id TEXT,thread_id TEXT,driver TEXT,status TEXT);
 INSERT INTO orchestration_v2_projection_threads VALUES('thread',NULL,NULL);
 INSERT INTO orchestration_v2_projection_provider_threads VALUES('pt','thread','codex','active');
 INSERT INTO orchestration_v2_projection_runs VALUES('thread','t3-run-1','codex','running',NULL,1,'pt');
 INSERT INTO orchestration_v2_projection_run_attempts(thread_id,run_id,attempt_id,status,payload_json,attempt_ordinal,provider_thread_id) VALUES('thread','t3-run-1','native-attempt-1','running','{"nativeThreadId":"native-session","startedAt":"2026-11-12T01:00:00Z"}',1,'pt');`)
	owner := exec.Command("sqlite3", filepath.Join(dir, "server-owner.sqlite"))
	in, e := owner.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	out, e := owner.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = owner.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = in.Close(); _ = owner.Wait() })
	fmt.Fprintln(in, "BEGIN EXCLUSIVE;SELECT 'ready';")
	scan := bufio.NewScanner(out)
	if !scan.Scan() {
		t.Fatal("owner lock")
	}
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Date(2026, 11, 12, 1, 1, 0, 0, time.UTC)
	pending, e := runtimeobs.BindRuntimeAgent(project, "native-session", "", "/root/controller", "", now)
	if e != nil {
		t.Fatal(e)
	}
	req := PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", TargetAttemptID: pending.AttemptID, ExecutionAuthorized: true}
	held, e := Prepare(req, now)
	if e != nil || held.Action != ActionHold {
		t.Fatal(held, e)
	}
	sel := runtimeobs.T3Selector{ThreadID: "thread", SessionID: "native-session", RuntimeAgentID: "native-session"}
	actual, _, e := runtimeobs.ObserveT3Execution(project, sel, now.Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if actual.AttemptID != pending.AttemptID {
		t.Fatal("lost pending identity")
	}
	recovered, e := Prepare(req, now.Add(2*time.Second))
	if e != nil || recovered.HandoffID != held.HandoffID || recovered.Action != ActionMessageRunning {
		t.Fatal(recovered, e)
	}
	// A fresh source change after Prepare must fence Claim, even within the TTL.
	sql(`UPDATE orchestration_v2_projection_runs SET status='completed',completed_at='2026-11-12T01:01:03Z';UPDATE orchestration_v2_projection_run_attempts SET status='completed',payload_json=json_set(payload_json,'$.completedAt','2026-11-12T01:01:03Z');`)
	claim, e := Claim(project, held.HandoffID, "/root/controller", actual.AttemptID, now.Add(4*time.Second))
	if e != nil || !claim.ClaimConflict {
		t.Fatalf("stale claim=%+v %v", claim, e)
	}
	sql(`INSERT INTO orchestration_v2_projection_runs VALUES('thread','t3-run-2','codex','running',NULL,2,'pt');INSERT INTO orchestration_v2_projection_run_attempts(thread_id,run_id,attempt_id,status,payload_json,attempt_ordinal,provider_thread_id) VALUES('thread','t3-run-2','native-attempt-2','running','{"nativeThreadId":"native-session","startedAt":"2026-11-12T01:01:05Z"}',1,'pt');`)
	resumed, _, e := runtimeobs.ObserveT3Execution(project, sel, now.Add(6*time.Second))
	if e != nil || resumed.AttemptID == actual.AttemptID {
		t.Fatal(resumed, e)
	}
	req.TargetAttemptID = resumed.AttemptID
	recovered, e = Prepare(req, now.Add(7*time.Second))
	if e != nil || recovered.HandoffID != held.HandoffID || recovered.Target.AttemptID != resumed.AttemptID || recovered.Action != ActionMessageRunning {
		t.Fatal(recovered, e)
	}
	claim, e = Claim(project, held.HandoffID, "/root/controller", resumed.AttemptID, now.Add(8*time.Second))
	if e != nil || !claim.Claimed {
		t.Fatal(claim, e)
	}
	again, e := Claim(project, held.HandoffID, "/root/controller", resumed.AttemptID, now.Add(9*time.Second))
	if e != nil || !again.AlreadyClaimed {
		t.Fatal(again, e)
	}
	ledger, e := BuildLedger(project, now.Add(10*time.Second))
	if e != nil || len(ledger.Handoffs) != 1 {
		t.Fatal(ledger, e)
	}
}
