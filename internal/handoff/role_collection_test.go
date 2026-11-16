package handoff

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// Temp local host projection + mock tool transport, never host MCP success.
func TestA81FormalControllerCollectionRecoversSameHandoffBeforeClaim(t *testing.T) {
	if _, e := exec.LookPath("sqlite3"); e != nil {
		t.Skip("sqlite3 unavailable")
	}
	home, project := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	writeSimpleTask(t, project, "A-1")
	if b, e := exec.Command("git", "init", "--quiet", project).CombinedOutput(); e != nil {
		t.Fatalf("%s %v", b, e)
	}
	dir := filepath.Join(home, ".t3", "userdata")
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	quote := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "''") + "'" }
	sql := func(q string) {
		t.Helper()
		b, e := exec.Command("sqlite3", filepath.Join(dir, "statev2.sqlite"), q).CombinedOutput()
		if e != nil {
			t.Fatalf("%s %v", b, e)
		}
	}
	sql(`CREATE TABLE orchestration_v2_projection_threads(thread_id TEXT,deleted_at TEXT,archived_at TEXT,project_id TEXT,payload_json TEXT);
CREATE TABLE projection_projects(project_id TEXT,workspace_root TEXT,deleted_at TEXT,archived_at TEXT);
CREATE TABLE orchestration_v2_projection_runs(thread_id TEXT,run_id TEXT,provider TEXT,status TEXT,completed_at TEXT,ordinal INTEGER,provider_thread_id TEXT);
CREATE TABLE orchestration_v2_projection_run_attempts(thread_id TEXT,run_id TEXT,attempt_id TEXT,status TEXT,payload_json TEXT,attempt_ordinal INTEGER,provider_thread_id TEXT,provider TEXT);
CREATE TABLE orchestration_v2_projection_provider_threads(provider_thread_id TEXT,thread_id TEXT,driver TEXT,status TEXT);
INSERT INTO projection_projects VALUES('project',` + quote(project) + `,NULL,NULL);
INSERT INTO orchestration_v2_projection_threads VALUES('parent',NULL,NULL,'project','{}');
INSERT INTO orchestration_v2_projection_threads VALUES('thread-exact',NULL,NULL,'project',json_object('lineage',json_object('parentThreadId','parent','relationshipToParent','subagent'),'worktreePath',` + quote(project) + `));
INSERT INTO orchestration_v2_projection_provider_threads VALUES('pt','thread-exact','codex','active');
INSERT INTO orchestration_v2_projection_runs VALUES('thread-exact','run-exact','codex','running',NULL,1,'pt');
INSERT INTO orchestration_v2_projection_run_attempts VALUES('thread-exact','run-exact','native-attempt','running','{"nativeThreadId":"native-session","startedAt":"2026-11-12T01:00:00Z"}',1,'pt','codex');`)
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
	if scan := bufio.NewScanner(out); !scan.Scan() || scan.Text() != "ready" {
		t.Fatal("fixture owner lock")
	}
	now := time.Date(2026, 11, 12, 1, 1, 0, 0, time.UTC)
	if _, e = runtimeobs.RecordAssignment(project, "assignment", "A-1", "/root/controller", now); e != nil {
		t.Fatal(e)
	}
	// Supported prepared dispatch preserves exact native identity without
	// pretending execution occurred. An identity-less held target cannot be
	// silently retargeted by role path alone.
	pending, e := runtimeobs.BindRuntimeAgentForAssignment(project, "assignment", "native-session", "", now)
	if e != nil {
		t.Fatal(e)
	}
	req := PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", TargetAttemptID: pending.AttemptID, ExecutionAuthorized: true}
	held, e := Prepare(req, now)
	if e != nil || held.Action != ActionHold {
		t.Fatalf("held %+v %v", held, e)
	}
	start := runtimeobs.RoleStart{CanonicalProject: project, SourceWorktree: project, TaskID: "A-1", AssignmentID: "assignment", Role: "controller", AgentPath: "/root/controller", ParentThreadID: "parent", Selector: runtimeobs.T3Selector{ThreadID: "thread-exact", SessionID: "native-session", RuntimeAgentID: "native-session"}}
	bound, e := runtimeobs.ObserveT3RoleStart(start, now.Add(time.Second))
	if e != nil || bound.Status != "bound" {
		t.Fatalf("collection %+v %v", bound, e)
	}
	req.TargetAttemptID = bound.Attempt.AttemptID
	recovered, e := Prepare(req, now.Add(2*time.Second))
	if e != nil || recovered.HandoffID != held.HandoffID || recovered.Action != ActionMessageRunning {
		t.Fatalf("recovery %+v %v", recovered, e)
	}
	// Actual tool behavior remains mocked; ACK cannot claim or apply.
	p := fixturePlan()
	p.HandoffID = held.HandoffID
	p.Target.AttemptID = bound.Attempt.AttemptID
	f := &fixtureTransport{state: "running", run: "run-exact"}
	if _, e = deliverPlannedReport(project, p, "fixture.md", "sha", f); e != nil {
		t.Fatal(e)
	}
	row, _, e := Inspect(project, held.HandoffID, now.Add(3*time.Second))
	if e != nil || row.ClaimedBy != "" || row.Applied {
		t.Fatalf("transport claimed %+v %v", row, e)
	}
	claim, e := Claim(project, held.HandoffID, "/root/controller", bound.Attempt.AttemptID, now.Add(4*time.Second))
	if e != nil || !claim.Claimed {
		t.Fatalf("claim %+v %v", claim, e)
	}
	duplicate, e := Claim(project, held.HandoffID, "/root/controller", bound.Attempt.AttemptID, now.Add(5*time.Second))
	if e != nil || !duplicate.AlreadyClaimed {
		t.Fatalf("duplicate %+v %v", duplicate, e)
	}
}
