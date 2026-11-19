package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeBoundaryCLIFailsClosedForForeignNativeAndUnmeasuredList(t *testing.T) {
	p := t.TempDir()
	var out, errOut bytes.Buffer
	code := runRuntimeBoundaryCLI([]string{"record", "--project", p, "--session-id", "foreign", "--thread-id", "thread", "--role", "registrar"}, &out, &errOut)
	if code == 0 || !strings.Contains(errOut.String(), "own current") {
		t.Fatalf("%d %s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(p, "_task_mecca")); !os.IsNotExist(err) {
		t.Fatal("failed identity wrote storage")
	}
	out.Reset()
	errOut.Reset()
	code = runRuntimeBoundaryCLI([]string{"list", "--project", p, "--json"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), `"items":[]`) {
		t.Fatalf("%d %s %s", code, out.String(), errOut.String())
	}
	if _, err := os.Stat(filepath.Join(p, "_task_mecca")); !os.IsNotExist(err) {
		t.Fatal("read-only list wrote storage")
	}
}

func TestRegistrationBoundaryFixtureCurrentLifecycleAndTaskAssociations(t *testing.T) {
	p := t.TempDir()
	var diagnostics bytes.Buffer
	host := newRegistrationHostFixture(t, p)
	c := host.collector(&diagnostics)
	c.write("start", "start", "", nil, "")
	c.phase("registration_apply")
	c.link([]string{"A-1", "A-2"})
	c.finish(0, "")
	rows, err := runtimeobs.ReadOperationBoundaries(p)
	if err != nil || len(rows) != 4 {
		t.Fatalf("%d %v %s", len(rows), err, diagnostics.String())
	}
	for _, b := range rows {
		if b.RequestID != "stable-request" || b.ObservedAt.Before(time.Now().Add(-time.Minute)) {
			t.Fatal("lost request/current observation")
		}
	}
	if rows[2].Kind != "task_link" || len(rows[2].TaskIDs) != 2 {
		t.Fatal("batch association")
	}
}

// An isolated supported-host fixture: production ReadT3Execution reads a real
// temp SQLite projection plus a live ownership lock; official native metadata
// comes through the same stdio protocol as the supported host. No real session
// or canonical journal is queried/written by these tests.
type registrationHostFixture struct {
	t                      *testing.T
	project, dir, metadata string
}

func newRegistrationHostFixture(t *testing.T, project string) *registrationHostFixture {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".t3", "userdata")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "fixture-native")
	t.Setenv("T3CODE_THREAD_ID", "fixture-thread")
	f := &registrationHostFixture{t: t, project: project, dir: dir, metadata: filepath.Join(home, "native-metadata.json")}
	started := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	f.sql(`CREATE TABLE orchestration_v2_projection_threads(thread_id TEXT,deleted_at TEXT,archived_at TEXT,project_id TEXT);
 CREATE TABLE projection_projects(project_id TEXT,workspace_root TEXT,deleted_at TEXT,archived_at TEXT);
 CREATE TABLE orchestration_v2_projection_runs(thread_id TEXT,run_id TEXT,provider TEXT,status TEXT,completed_at TEXT,ordinal INTEGER,provider_thread_id TEXT);
 CREATE TABLE orchestration_v2_projection_run_attempts(thread_id TEXT,run_id TEXT,attempt_id TEXT,status TEXT,payload_json TEXT,attempt_ordinal INTEGER,provider_thread_id TEXT,provider TEXT DEFAULT 'codex');
 CREATE TABLE orchestration_v2_projection_provider_threads(provider_thread_id TEXT,thread_id TEXT,driver TEXT,status TEXT);
 INSERT INTO orchestration_v2_projection_threads VALUES('fixture-thread',NULL,NULL,'fixture-project');
 INSERT INTO projection_projects VALUES('fixture-project',` + fixtureSQLLiteral(project) + `,NULL,NULL);
 INSERT INTO orchestration_v2_projection_provider_threads VALUES('provider-thread','fixture-thread','codex','active');
 INSERT INTO orchestration_v2_projection_runs VALUES('fixture-thread','run-1','codex','running',NULL,1,'provider-thread');
 INSERT INTO orchestration_v2_projection_run_attempts VALUES('fixture-thread','run-1','native-attempt-1','running','{"nativeThreadId":"fixture-native","startedAt":"` + started + `"}',1,'provider-thread','codex');`)
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
	t.Cleanup(func() { in.Close(); owner.Wait() })
	fmt.Fprintln(in, "BEGIN EXCLUSIVE; SELECT 'ready';")
	scan := bufio.NewScanner(out)
	if !scan.Scan() || scan.Text() != "ready" {
		t.Fatal("isolated owner lock unavailable")
	}
	if err = os.WriteFile(f.metadata, []byte(`{"id":"fixture-native","name":"D1112_01_Registrar"}`), 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(home, "bin")
	if err = os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexec " + fixtureShellQuote(os.Args[0]) + " -test.run='^TestA93RepairMetadataHelper$'\n"
	if err = os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("A93_REPAIR_METADATA_HELPER", "1")
	t.Setenv("A93_REPAIR_METADATA_FILE", f.metadata)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return f
}
func fixtureSQLLiteral(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func fixtureShellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func (f *registrationHostFixture) sql(q string) {
	f.t.Helper()
	raw, err := exec.Command("sqlite3", filepath.Join(f.dir, "statev2.sqlite"), q).CombinedOutput()
	if err != nil {
		f.t.Fatalf("isolated SQL fixture: %v %s", err, raw)
	}
}

func (f *registrationHostFixture) collector(diagnostics io.Writer) *registrationBoundary {
	f.t.Helper()
	now := time.Now().UTC()
	sel := runtimeobs.T3Selector{CanonicalProject: f.project, ThreadID: "fixture-thread", SessionID: "fixture-native", RuntimeAgentID: "fixture-native"}
	r, err := runtimeobs.ReadT3Execution(sel, now)
	if err != nil {
		f.t.Fatal(err)
	}
	m, err := runtimeobs.ResolveCodexThreadMetadata(context.Background(), "fixture-native")
	if err != nil {
		f.t.Fatal(err)
	}
	// Seed observed evidence through the real supported journal API, using the
	// receipt just read from the isolated host. No registration START yet.
	err = runtimeobs.AppendExecutionEvent(f.project, runtimeobs.ExecutionEvent{EventID: "fixture-observed", EventKind: "state", ObservedAt: r.ObservedAt, AttemptID: "fixture-attempt", Provider: r.Provider, SessionID: r.SessionID, RuntimeAgentID: r.SessionID, TurnID: r.NativeAttemptID, State: r.State, EvidenceSource: runtimeobs.EvidenceT3, ObservationQuality: runtimeobs.QualityObserved, NativeReceipt: &r})
	if err != nil {
		f.t.Fatal(err)
	}
	return &registrationBoundary{project: f.project, stderr: diagnostics, startReceipt: r, nativeName: m.Name, base: runtimeobs.OperationBoundary{RequestID: "stable-request", OperationID: "stable-invocation", Provider: r.Provider, NativeID: r.SessionID, AttemptID: "fixture-attempt", ThreadID: r.ThreadID, Role: "registrar", Source: "isolated_fixture", Evidence: "fixture; not actual maintenance"}}
}

func TestA93RepairMetadataHelper(t *testing.T) {
	if os.Getenv("A93_REPAIR_METADATA_HELPER") != "1" {
		return
	}
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		if json.Unmarshal(s.Bytes(), &req) != nil || req.ID == nil {
			continue
		}
		var result any = map[string]any{}
		if req.Method == "thread/read" {
			raw, err := os.ReadFile(os.Getenv("A93_REPAIR_METADATA_FILE"))
			var metadata map[string]any
			if err != nil || json.Unmarshal(raw, &metadata) != nil {
				json.NewEncoder(os.Stdout).Encode(map[string]any{"id": req.ID, "error": map[string]any{"code": -1, "message": "isolated metadata unavailable"}})
				continue
			}
			result = map[string]any{"thread": metadata}
		}
		json.NewEncoder(os.Stdout).Encode(map[string]any{"id": req.ID, "result": result})
	}
	os.Exit(0)
}

func TestA93RepairHostChangesLeaveStartOpen(t *testing.T) {
	for _, kind := range []string{"own_native", "own_thread", "terminal", "new_run", "retry", "unknown_host", "unknown_role", "changed_role", "observed_terminal"} {
		t.Run(kind, func(t *testing.T) {
			p := t.TempDir()
			host := newRegistrationHostFixture(t, p)
			var diagnostics bytes.Buffer
			c := host.collector(&diagnostics)
			c.write("start", "start", "", nil, "")
			if !c.started {
				t.Fatalf("verified START failed: %s", diagnostics.String())
			}
			now := time.Now().UTC().Format(time.RFC3339Nano)
			switch kind {
			case "own_native":
				t.Setenv("CODEX_THREAD_ID", "other-native")
			case "own_thread":
				t.Setenv("T3CODE_THREAD_ID", "other-thread")
			case "terminal":
				host.sql(`UPDATE orchestration_v2_projection_runs SET status='completed',completed_at='` + now + `'; UPDATE orchestration_v2_projection_run_attempts SET status='completed',payload_json=json_set(payload_json,'$.completedAt','` + now + `');`)
			case "new_run":
				host.sql(`UPDATE orchestration_v2_projection_runs SET status='completed',completed_at='` + now + `'; INSERT INTO orchestration_v2_projection_runs VALUES('fixture-thread','run-2','codex','running',NULL,2,'provider-thread'); INSERT INTO orchestration_v2_projection_run_attempts VALUES('fixture-thread','run-2','native-attempt-2','running','{"nativeThreadId":"fixture-native","startedAt":"` + now + `"}',1,'provider-thread','codex');`)
			case "retry":
				host.sql(`INSERT INTO orchestration_v2_projection_run_attempts VALUES('fixture-thread','run-1','native-attempt-2','running','{"nativeThreadId":"fixture-native","startedAt":"` + now + `"}',2,'provider-thread','codex');`)
			case "unknown_host":
				if err := os.Rename(filepath.Join(host.dir, "statev2.sqlite"), filepath.Join(host.dir, "statev2-unavailable.sqlite")); err != nil {
					t.Fatal(err)
				}
			case "unknown_role":
				if err := os.WriteFile(host.metadata, []byte(`invalid`), 0600); err != nil {
					t.Fatal(err)
				}
			case "changed_role":
				if err := os.WriteFile(host.metadata, []byte(`{"id":"fixture-native","name":"D1112_01_Controller"}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "observed_terminal":
				r := c.startReceipt
				r.State = runtimeobs.StateCompleted
				r.CompletedAt = now
				r.ObservedAt = now
				if err := runtimeobs.AppendExecutionEvent(p, runtimeobs.ExecutionEvent{EventID: "fixture-terminal", EventKind: "state", ObservedAt: now, AttemptID: c.base.AttemptID, Provider: r.Provider, SessionID: r.SessionID, RuntimeAgentID: r.SessionID, TurnID: r.NativeAttemptID, State: r.State, Terminal: true, EvidenceSource: runtimeobs.EvidenceT3, ObservationQuality: runtimeobs.QualityObserved, NativeReceipt: &r}); err != nil {
					t.Fatal(err)
				}
			}
			c.phase("registration_apply")
			c.link([]string{"A-1"})
			c.finish(0, "")
			rows, err := runtimeobs.ReadOperationBoundaries(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || rows[0].Kind != "start" {
				t.Fatalf("stale/fabricated boundaries: %+v", rows)
			}
			status := c.status()
			if c.ended || !c.started || status.Status != "collection_failure" || status.OriginalError == "" || !c.identityLost {
				t.Fatalf("false success/open lost: %+v %s", status, diagnostics.String())
			}
			var businessOut, businessError bytes.Buffer
			code := runAgentJSON([]string{"--json"}, func(out, stderr io.Writer) int {
				if err := json.NewEncoder(out).Encode(map[string]any{"status": "success", "execution_collection": status}); err != nil {
					return 1
				}
				return 0
			}, &businessOut, &businessError)
			var business struct {
				Status     string                       `json:"status"`
				Collection registrationCollectionStatus `json:"execution_collection"`
			}
			if err := json.Unmarshal(businessOut.Bytes(), &business); err != nil || code != 0 || business.Status != "success" || business.Collection.Status != "collection_failure" || business.Collection.OriginalError == "" || business.Collection.Ended {
				t.Fatalf("business success hid collection failure: %d %s %v", code, businessOut.String(), err)
			}
			// Restoring environment must not reattribute this old operation.
			t.Setenv("CODEX_THREAD_ID", "fixture-native")
			t.Setenv("T3CODE_THREAD_ID", "fixture-thread")
			c.link([]string{"A-2"})
			again, _ := runtimeobs.ReadOperationBoundaries(p)
			if len(again) != 1 {
				t.Fatal("failed collector reopened")
			}
		})
	}
}

func TestA93RepairAutomaticStableHostRetainsBusinessResult(t *testing.T) {
	p := t.TempDir()
	newRegistrationHostFixture(t, p)
	var diagnostics bytes.Buffer
	c := beginRegistrationBoundary(p, "actual-fixture-request", &diagnostics)
	if c == nil || c.disabled || !c.started {
		t.Fatalf("stable automatic START: %s", diagnostics.String())
	}
	c.phase("registration_apply")
	c.link([]string{"A-1", "A-2"})
	c.finish(0, "")
	if c.status().Status != "observed" || !c.ended {
		t.Fatalf("stable END: %+v %s", c.status(), diagnostics.String())
	}
	rows, err := runtimeobs.ReadOperationBoundaries(p)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, b := range rows {
		if b.RequestID == "actual-fixture-request" {
			n++
		}
	}
	if n != 5 {
		t.Fatalf("registration start/phase/link/end count=%d", n)
	}
}

func TestA93RepairEveryFollowupRechecksImmediately(t *testing.T) {
	for _, step := range []string{"phase", "task_link", "end"} {
		t.Run(step, func(t *testing.T) {
			p := t.TempDir()
			host := newRegistrationHostFixture(t, p)
			var errors bytes.Buffer
			c := host.collector(&errors)
			c.write("start", "start", "", nil, "")
			if step != "phase" {
				c.phase("registration_apply")
			}
			if step == "end" {
				c.link([]string{"A-1"})
			}
			before, err := runtimeobs.ReadOperationBoundaries(p)
			if err != nil || !c.started {
				t.Fatalf("START/setup %v %s", err, errors.String())
			}
			t.Setenv("T3CODE_THREAD_ID", "different-current-thread")
			switch step {
			case "phase":
				c.phase("changed")
			case "task_link":
				c.link([]string{"A-1"})
			case "end":
				c.finish(0, "")
			}
			after, err := runtimeobs.ReadOperationBoundaries(p)
			if err != nil || len(after) != len(before) || c.ended || c.status().Status != "collection_failure" {
				t.Fatalf("unguarded %s: %+v %+v %v", step, after, c.status(), err)
			}
		})
	}
}

func TestA93RepairFailurePersistenceCannotBypassLostReceipt(t *testing.T) {
	p := t.TempDir()
	host := newRegistrationHostFixture(t, p)
	var diagnostics bytes.Buffer
	c := host.collector(&diagnostics)
	c.write("start", "start", "", nil, "")
	c.link([]string{"A-1"})
	c.link([]string{"A-2"}) // actual fixture journal conflict
	if c.firstError == nil || !strings.Contains(c.firstError.Error(), "conflicting evidence") {
		t.Fatalf("expected original key conflict: %s", diagnostics.String())
	}
	before, err := runtimeobs.ReadOperationBoundaries(p)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	host.sql(`INSERT INTO orchestration_v2_projection_run_attempts VALUES('fixture-thread','run-1','native-attempt-2','running','{"nativeThreadId":"fixture-native","startedAt":"` + now + `"}',2,'provider-thread','codex');`)
	c.finish(0, "")
	after, err := runtimeobs.ReadOperationBoundaries(p)
	if err != nil || len(after) != len(before) || c.ended {
		t.Fatalf("old failure/END persisted across retry: %+v %v", after, err)
	}
	status := c.status()
	if status.Status != "collection_failure" || !strings.Contains(status.OriginalError, "conflicting evidence") || !strings.Contains(status.OriginalError, "current running") {
		t.Fatalf("original error/current failure lost: %+v", status)
	}
}

func TestBoundaryNativeRoleMetadataMatching(t *testing.T) {
	for role, name := range map[string]string{"registrar": "D1112_01_Registrar", "controller": "D1112_01_Controller", "worker": "D1112_01_Worker_naong_G3", "reviewer": "D1112_01_Reviewer_raichyu_G6"} {
		if !boundaryRoleMatches(role, name) || boundaryRoleMatches("registrar", name) && role != "registrar" {
			t.Fatalf("%s %s", role, name)
		}
	}
	if boundaryRoleMatches("reviewer", "D1112_01_Worker_raichyu_G6") {
		t.Fatal("worker cannot claim reviewer")
	}
}

func TestRegistrationBoundaryUnsupportedStatusSurvivesAgentJSON(t *testing.T) {
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("T3CODE_THREAD_ID", "")
	var out, stderr bytes.Buffer
	code := runAgentJSON([]string{"--json"}, func(w, e io.Writer) int {
		c := beginRegistrationBoundary(t.TempDir(), "request", e)
		c.finish(0, "")
		if err := json.NewEncoder(w).Encode(map[string]any{"status": "success", "execution_collection": c.status()}); err != nil {
			return 1
		}
		return 0
	}, &out, &stderr)
	if code != 0 || !strings.Contains(out.String(), `"status":"unmeasured"`) || !strings.Contains(out.String(), "original_error") {
		t.Fatalf("collection error hidden: %d %s %s", code, out.String(), stderr.String())
	}
}
