package runtimeobs

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

const b639T3 = "thread:delegated-task:command%3Amcp%3Aeef2e499-d425-4776-b1cc-5da583543e73%3Adelegate-task%3Aa65f3b07-2420-4814-a93d-27aec7600e62"
const b639Native = "01a1e2c1-7d76-7360-9fab-434be8de3265"

// Offline reproduction: the original guard fails before any host observation,
// for BOTH UUID-looking and opaque T3 IDs when passed the Controller role.
// It proves this source condition, not the unrecorded original executable version.
func TestA81B639OriginalFirstRejectionIsRoleNotUUID(t *testing.T) {
	for _, id := range []string{b639T3, "6b16c2e6-9c34-49ae-99a8-5b610343403c"} {
		in := WorkerStart{"/Users/hanati/projects/empfund", "/Users/hanati/.t3/worktrees/empfund/t3-ca90345a-1e83532959ea792b", "B-639", "assignment-bcd3c8d10faf1d2b6c64123933bed5aa", "/root/controller", T3Selector{ThreadID: id, SessionID: b639Native, RuntimeAgentID: b639Native}}
		called := false
		got, err := observeWorkerStart(in, time.Now(), func(string, T3Selector, time.Time) (Attempt, T3Receipt, error) {
			called = true
			return Attempt{}, T3Receipt{}, nil
		})
		want := "exact canonical project, source worktree, task, assignment, worker path and distinct native/T3 identities required"
		if err == nil || err.Error() != want || got.Stage != "identity" || called {
			t.Fatalf("original rejection changed: %+v %v called=%v", got, err, called)
		}
	}
}

func roleFixture(t *testing.T, role string) (RoleStart, t3Store, func(string), time.Time) {
	t.Helper()
	store, sql := t3Fixture(t)
	p := t.TempDir()
	if b, e := exec.Command("git", "init", "--quiet", p).CombinedOutput(); e != nil {
		t.Fatalf("temp repository: %s %v", b, e)
	}
	path := "/root/controller"
	if role == "worker" {
		path += "/naong"
	}
	now := time.Date(2026, 11, 12, 1, 10, 0, 0, time.UTC)
	if _, e := RecordAssignment(p, "assignment", "A-81", path, now); e != nil {
		t.Fatal(e)
	}
	// Deliberately synthetic projection, exclusively under t.TempDir().
	sql(`ALTER TABLE orchestration_v2_projection_threads ADD COLUMN project_id TEXT;
ALTER TABLE orchestration_v2_projection_threads ADD COLUMN payload_json TEXT;
CREATE TABLE projection_projects(project_id TEXT,workspace_root TEXT,deleted_at TEXT,archived_at TEXT);
INSERT INTO projection_projects VALUES('project',` + sqlLiteral(p) + `,NULL,NULL);
UPDATE orchestration_v2_projection_threads SET thread_id=` + sqlLiteral(b639T3) + `,project_id='project',payload_json=json_object('lineage',json_object('parentThreadId','parent','relationshipToParent','subagent'),'worktreePath',` + sqlLiteral(p) + `);
INSERT INTO orchestration_v2_projection_threads VALUES('parent',NULL,NULL,'project','{}');
UPDATE orchestration_v2_projection_runs SET thread_id=` + sqlLiteral(b639T3) + `;
UPDATE orchestration_v2_projection_provider_threads SET thread_id=` + sqlLiteral(b639T3) + `;
UPDATE orchestration_v2_projection_run_attempts SET thread_id=` + sqlLiteral(b639T3) + `,payload_json=json_object('nativeThreadId',` + sqlLiteral(b639Native) + `,'startedAt','2026-11-12T01:00:00Z');`)
	return RoleStart{p, p, "A-81", "assignment", role, path, "parent", T3Selector{ThreadID: b639T3, SessionID: b639Native, RuntimeAgentID: b639Native}}, store, sql, now
}

func TestA81FormalRoleCollectionOpaqueBindingAndRetry(t *testing.T) {
	for _, role := range []string{"controller", "worker"} {
		t.Run(role, func(t *testing.T) {
			in, store, _, now := roleFixture(t, role)
			observe := func(p string, s T3Selector, n time.Time) (Attempt, T3Receipt, error) {
				return observeT3Execution(p, s, n, store.read)
			}
			got, err := observeRoleCollection(in, now, store.verifyRoleScope, observe)
			if err != nil || got.Status != "bound" || got.Attempt.TaskID != "A-81" || got.Attempt.AgentPath != in.AgentPath || got.Attempt.BindingEvidence["assignment_id"] != in.AssignmentID {
				t.Fatalf("%+v %v", got, err)
			}
			for i := 1; i <= 2; i++ {
				retry, e := observeRoleCollection(in, now.Add(time.Duration(i)*time.Second), store.verifyRoleScope, observe)
				if e != nil || retry.Attempt.AttemptID != got.Attempt.AttemptID {
					t.Fatalf("retry %+v %v", retry, e)
				}
			}
			ledger, e := BuildLedger(in.CanonicalProject, 20, now.Add(2*time.Second))
			if e != nil || len(ledger.Attempts) != 1 {
				t.Fatalf("duplicate attempt %+v %v", ledger, e)
			}
			// Runtime binding never injects Hook events or Activity.
			if _, e = os.Stat(filepath.Join(in.CanonicalProject, "_task_mecca", "data", "activity")); !os.IsNotExist(e) {
				t.Fatal("collection wrote Activity")
			}
		})
	}
}

func TestA81FormalCollectionFailsClosedWithoutRuntimeWrite(t *testing.T) {
	for _, tc := range []struct {
		name, stage string
		change      func(*RoleStart, func(string))
	}{
		{"role", "role", func(i *RoleStart, _ func(string)) { i.Role = "worker" }},
		{"subcontroller", "role", func(i *RoleStart, _ func(string)) { i.AgentPath = "/root/controller/sub-other" }},
		{"nativeAsThread", "identity", func(i *RoleStart, _ func(string)) { i.Selector.ThreadID = b639Native }},
		{"threadAsNative", "identity", func(i *RoleStart, _ func(string)) { i.Selector.SessionID = b639T3; i.Selector.RuntimeAgentID = b639T3 }},
		{"wrapper", "identity", func(i *RoleStart, _ func(string)) {
			i.Selector.ThreadID = "thread:provider:codex:native-thread:" + b639Native
		}},
		{"wrongParent", "scope", func(i *RoleStart, _ func(string)) { i.ParentThreadID = "other" }},
		{"foreignParent", "scope", func(_ *RoleStart, q func(string)) {
			q("UPDATE orchestration_v2_projection_threads SET project_id='other' WHERE thread_id='parent';")
		}},
		{"foreignProject", "scope", func(_ *RoleStart, q func(string)) { q("UPDATE projection_projects SET workspace_root='/unavailable';") }},
		{"canonicalAsWrongSource", "scope", func(i *RoleStart, _ func(string)) { i.SourceWorktree = t.TempDir() }},
		{"wrongNative", "observation", func(i *RoleStart, _ func(string)) {
			i.Selector.SessionID = "other"
			i.Selector.RuntimeAgentID = "other"
		}},
		{"oldRun", "observation", func(i *RoleStart, _ func(string)) { i.Selector.RunID = "old" }},
		{"assignment", "assignment", func(i *RoleStart, _ func(string)) { i.AssignmentID = "missing" }},
		{"task", "assignment", func(i *RoleStart, _ func(string)) { i.TaskID = "B-639" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, store, sql, now := roleFixture(t, "controller")
			tc.change(&in, sql)
			got, e := observeRoleCollection(in, now, store.verifyRoleScope, func(p string, s T3Selector, n time.Time) (Attempt, T3Receipt, error) {
				return observeT3Execution(p, s, n, store.read)
			})
			if e == nil || got.Stage != tc.stage {
				t.Fatalf("%+v %v", got, e)
			}
			ledger, e := BuildLedger(in.CanonicalProject, 20, now)
			if e != nil || len(ledger.Attempts) != 0 {
				t.Fatalf("rejected identity wrote runtime %+v %v", ledger, e)
			}
		})
	}
}

func TestA81FormalScopeRepositoryAndBridgeLimits(t *testing.T) {
	in, store, sql, _ := roleFixture(t, "controller")
	other := t.TempDir()
	if b, e := exec.Command("git", "init", "--quiet", other).CombinedOutput(); e != nil {
		t.Fatalf("%s %v", b, e)
	}
	in.SourceWorktree = other
	sql(`UPDATE orchestration_v2_projection_threads SET payload_json=json_set(payload_json,'$.worktreePath',` + sqlLiteral(other) + `) WHERE thread_id=` + sqlLiteral(b639T3) + `;`)
	if e := store.verifyRoleScope(in); e == nil || !strings.Contains(e.Error(), "source_repository_mismatch") {
		t.Fatalf("foreign repository accepted %v", e)
	}
	sql("DROP TABLE projection_projects;")
	if e := store.verifyRoleScope(in); e == nil || !strings.Contains(e.Error(), "host_scope_unavailable") {
		t.Fatalf("unsupported bridge accepted %v", e)
	}
}

func TestA81FormalCollectionPartialObservationAndDelayedRetry(t *testing.T) {
	in, store, _, now := roleFixture(t, "controller")
	calls := 0
	scope := func(i RoleStart) error {
		calls++
		if calls == 3 {
			return errors.New("host scope changed")
		}
		return store.verifyRoleScope(i)
	}
	observe := func(p string, s T3Selector, n time.Time) (Attempt, T3Receipt, error) {
		return observeT3Execution(p, s, n, store.read)
	}
	got, e := observeRoleCollection(in, now, scope, observe)
	if e == nil || got.Stage != "observation" || got.Attempt.AttemptID == "" || got.Attempt.BindingState == BindingBound {
		t.Fatalf("partial observation hidden %+v %v", got, e)
	}
	retry, e := observeRoleCollection(in, now.Add(time.Minute), store.verifyRoleScope, observe)
	if e != nil || retry.Attempt.AttemptID != got.Attempt.AttemptID || retry.Status != "bound" {
		t.Fatalf("delayed retry %+v %v", retry, e)
	}
}

func TestA81FormalWorkerRetainsImplementationPolicyWorkspaceGate(t *testing.T) {
	in, store, _, now := roleFixture(t, "worker")
	policy := rolepolicy.Policy{SchemaVersion: 1, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: in.AgentPath, NativeID: b639Native}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Workspace: t.TempDir(), Method: "worker", Scope: []string{"internal"}}
	dir := filepath.Join(in.CanonicalProject, "_task_mecca", "data", "backlog")
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	write := func() {
		t.Helper()
		b, _ := json.Marshal(policy)
		if e := os.WriteFile(filepath.Join(dir, "000081.A-81.fixture.todo.md"), []byte("# A-81 Fixture\n\n## 요건 정의서\n"+rolepolicy.Fence+string(b)+"\n```\n\n## 실행 정보\n"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	write()
	observe := func(p string, s T3Selector, n time.Time) (Attempt, T3Receipt, error) {
		return observeT3Execution(p, s, n, store.read)
	}
	got, e := observeRoleCollection(in, now, store.verifyRoleScope, observe)
	if e == nil || got.Stage != "binding" || !strings.Contains(e.Error(), "role_policy_hold") || got.Attempt.BindingState == BindingBound {
		t.Fatalf("Worker policy bypassed %+v %v", got, e)
	}
	policy.Workspace = in.SourceWorktree
	write()
	retry, e := observeRoleCollection(in, now.Add(time.Second), store.verifyRoleScope, observe)
	if e != nil || retry.Status != "bound" || retry.Attempt.AttemptID != got.Attempt.AttemptID {
		t.Fatalf("policy retry %+v %v", retry, e)
	}
}
