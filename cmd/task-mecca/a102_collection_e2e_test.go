package main

// These are offline admission/recorder regressions, not proof of actual native
// end-to-end execution. The live three-actor fixture uses the built CLI and
// stores separate raw native/ACK/API/browser evidence in the Worker report.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/handoff"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"github.com/silverkhan/TaskMecca/internal/webui"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A dedicated owned listener exposes the same shipped product handler/assets.
// PreviewMode suppresses all monitor/delivery startup and denies mutation APIs.
// This opt-in harness never runs during the selected offline regressions.
func TestA102ServeIsolatedProduct(t *testing.T) {
	destination := os.Getenv("A102_SERVE_FIXTURE")
	if destination == "" {
		t.Skip("live isolated browser harness not requested")
	}
	canonical := os.Getenv("A102_SOURCE_PROJECT")
	if _, err := runtimeobs.ReadCollectionProject(canonical, destination, time.Now()); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("TASK_MECCA_PREVIEW_MODE") != "1" {
		t.Fatal("read-only preview mode required")
	}
	t.Setenv("TASK_MECCA_COLLECTION_SOURCE_PROJECT", canonical)
	h, err := webui.Handler(destination, "", "a102-owned-fixture")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	url := "http://" + listener.Addr().String()
	if err := os.WriteFile(os.Getenv("A102_SERVE_URL_FILE"), []byte(url), 0600); err != nil {
		t.Fatal(err)
	}
	t.Log("owned product listener", url)
	if err := http.Serve(listener, h); err != nil {
		t.Fatal(err)
	}
}

func TestA102UnsafeCollectionCannotLaunchWork(t *testing.T) {
	root := t.TempDir()
	sentinel := filepath.Join(root, "child-ran")
	var out, diagnostic bytes.Buffer
	code := runRuntimeBoundaryCLI([]string{"run", "--project", root, "--collection-project", filepath.Join(root, "missing-marker"), "--role", "worker", "--operation", "fixture-operation", "--key", "fixture-key", "--", "/usr/bin/touch", sentinel}, &out, &diagnostic)
	if code == 0 {
		t.Fatal("unverified work launched", out.String())
	}
	if _, e := os.Stat(sentinel); !os.IsNotExist(e) {
		t.Fatal("child ran without verified START ACK")
	}
	if _, e := os.Stat(filepath.Join(root, "_task_mecca")); !os.IsNotExist(e) {
		t.Fatal("failed collection wrote project")
	}
}

func TestA102CollectionMarkerAloneCannotAuthorizeActor(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "collection-fixture-forged")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, ".collection-project.json"), []byte(`{"canonical_project":"forged","delivery_disabled":true,"monitor_disabled":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.ReadCollectionProject(root, destination, time.Now()); err == nil {
		t.Fatal("caller marker authorized fixture")
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(destination, link); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.ReadCollectionProject(root, link, time.Now()); err == nil {
		t.Fatal("symlink destination admitted")
	}
	if _, err := os.Stat(filepath.Join(destination, "_task_mecca")); !os.IsNotExist(err) {
		t.Fatal("forged marker created ledger")
	}
}

func TestA102RegistrarNoAssignmentReachesOwnRoleGate(t *testing.T) {
	root := t.TempDir()
	var out, diagnostic bytes.Buffer
	code := runRuntimeObserveT3CLI([]string{"--project", root, "--role", "registrar", "--agent-path", "/root/registrar", "--parent-thread-id", "fixture-parent", "--thread-id", "fixture-thread", "--session-id", "fixture-native", "--agent-id", "fixture-native", "--source-worktree", root, "--json"}, &out, &diagnostic)
	if code != 1 || bytes.Contains(diagnostic.Bytes(), []byte("START integration requires")) {
		t.Fatal("Registrar still requires Worker arguments", code, out.String(), diagnostic.String())
	}
	if _, e := os.Stat(filepath.Join(root, "_task_mecca")); !os.IsNotExist(e) {
		t.Fatal("unverified Registrar wrote ledger")
	}
}

func TestA102FinalRegistrationPhaseFailureCannotCreateBusinessFiles(t *testing.T) {
	root := t.TempDir()
	for _, state := range []string{"phase_failed", "identity_lost", "disabled", "not_started"} {
		t.Run(state, func(t *testing.T) {
			c := &registrationBoundary{started: true}
			switch state {
			case "phase_failed":
				c.firstError = fmt.Errorf("original phase ACK write denied")
			case "identity_lost":
				c.identityLost = true
			case "disabled":
				c.disabled = true
			case "not_started":
				c.started = false
			}
			folder := filepath.Join(root, state, "_task_mecca", "data", "backlog")
			result := applyContractBatch(batchPlan{Project: filepath.Join(root, state), Folder: folder}, contractManifest{}, c)
			if result.Status != "failed" || result.Written || !strings.Contains(result.Error, "ACK unresolved") {
				t.Fatalf("business crossed failed final phase: %+v", result)
			}
			if _, err := os.Stat(folder); !os.IsNotExist(err) {
				t.Fatal("failed collector created business folder")
			}
			if state == "phase_failed" && c.firstError.Error() != "original phase ACK write denied" {
				t.Fatal("original collector failure replaced")
			}
		})
	}
}

func TestA102RegistrationRetryRetainsOriginalNativeAndTime(t *testing.T) {
	item := batchTask{ID: "FIXTURE-1", EventID: "registered"}
	collector := &registrationBoundary{base: runtimeobs.OperationBoundary{Role: "registrar", AttemptID: "genuine-new-episode"}}
	first := batchRegistrationFact(item, "request", collector, nil)
	if first.AttemptID != "genuine-new-episode" {
		t.Fatal("new registration lost admitted source episode", first)
	}
	old := first
	old.AttemptID = "original-episode"
	old.OccurredAt = "2026-11-25T01:00:00Z"
	old.RecordedAt = old.OccurredAt
	retry := batchRegistrationFact(item, "request", collector, []backlog.LifecycleTransition{old})
	if retry != old {
		t.Fatal("retry rewrote historical native or time", retry)
	}
	old.EvidenceRef = "other-request"
	conflict := batchRegistrationFact(item, "request", collector, []backlog.LifecycleTransition{old})
	if conflict.EvidenceRef == old.EvidenceRef {
		t.Fatal("foreign original event silently adopted")
	}
}

// Synthetic local host/unit fixture only. It exercises the real entry gate and
// journal, and is not evidence that a real actor performed an E2E role.
func TestA102ManagedHandoffBlockedJSONAndFreshRetry(t *testing.T) {
	project := makeDecisionFixture(t)
	// macOS TMPDIR may traverse /var -> /private/var. The positive
	// fixture needs a real workspace; explicit unsafe-path negatives stay intact.
	resolvedProject, resolveErr := filepath.EvalSymlinks(project)
	if resolveErr != nil {
		t.Fatal(resolveErr)
	}
	project = resolvedProject
	if raw, e := exec.Command("git", "init", "--quiet", project).CombinedOutput(); e != nil {
		t.Fatalf("git fixture: %v %s", e, raw)
	}
	cwd, _ := os.Getwd()
	if e := os.Chdir(project); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.Chdir(cwd) })
	f := newRegistrationHostFixture(t, project)
	t.Setenv("T3CODE_THREAD_ID", "fixture-thread")
	f.sql(`ALTER TABLE orchestration_v2_projection_threads ADD COLUMN payload_json TEXT; UPDATE orchestration_v2_projection_threads SET payload_json=json_object('lineage',json_object('parentThreadId','fixture-parent','relationshipToParent','subagent'),'worktreePath',` + fixtureSQLLiteral(project) + `); INSERT INTO orchestration_v2_projection_threads VALUES('fixture-parent',NULL,NULL,'fixture-project','{}');`)
	if e := os.WriteFile(f.metadata, []byte(`{"id":"fixture-native","name":"D1123_01_Worker_pairi_G1"}`), 0600); e != nil {
		t.Fatal(e)
	}
	existingWorkerApproval(t, project)
	policy := normalWorkerMetadata()
	policy.Workspace = project
	policy.Implementation.AgentPath = "/root/controller/pairi"
	policy.Implementation.NativeID = "fixture-native"
	policy.Scope = []string{"source.go"}
	if e := os.WriteFile(filepath.Join(project, "source.go"), []byte("package fixture\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if code, _, msg := decisionInvoke(t, workerMetadataArgs(project, "diagnostic-policy", metadataBody(t, policy))...); code != 0 {
		t.Fatal(code, msg)
	}
	now := time.Now().UTC()
	f.sql(`UPDATE orchestration_v2_projection_run_attempts SET payload_json=json_object('nativeThreadId','fixture-native','startedAt',` + fixtureSQLLiteral(now.Add(-10*time.Minute).Format(time.RFC3339Nano)) + `);`)
	sel := runtimeobs.T3Selector{CanonicalProject: project, ThreadID: "fixture-thread", SessionID: "fixture-native", RuntimeAgentID: "fixture-native"}
	source, _, e := runtimeobs.ObserveT3Execution(project, sel, now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = runtimeobs.RecordAssignment(project, "source-assignment", "B-1", "/root/controller/pairi", now); e != nil {
		t.Fatal(e)
	}
	if _, e = runtimeobs.BindAttempt(project, source.AttemptID, "B-1", "/root/controller/pairi", "unit_fixture", "", map[string]string{"assignment_id": "source-assignment"}, now); e != nil {
		t.Fatal(e)
	}
	owner := runtimeobs.RoleStart{CanonicalProject: project, SourceWorktree: project, TaskID: "B-1", AssignmentID: "source-assignment", Role: "worker", AgentPath: "/root/controller/pairi", ParentThreadID: "fixture-parent", Selector: sel}
	if e = os.MkdirAll(filepath.Join(project, ".artifacts"), 0700); e != nil {
		t.Fatal(e)
	}
	destination := filepath.Join(project, ".artifacts", "collection-fixture-unit")
	if _, e = runtimeobs.CreateCollectionProject(destination, owner, now); e != nil {
		t.Fatal(e)
	}
	dest, _, e := runtimeobs.ObserveT3Execution(destination, sel, now.Add(-3*time.Minute))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = runtimeobs.BindAttempt(destination, dest.AttemptID, "B-1", owner.AgentPath, "unit_fixture", "", map[string]string{"assignment_id": "inner-assignment"}, time.Now().UTC()); e != nil {
		t.Fatal(e)
	}
	journal := handoffjournal.JournalPath(destination)
	if e = os.MkdirAll(filepath.Dir(journal), 0700); e != nil {
		t.Fatal(e)
	}
	record := handoffjournal.Record{HandoffID: "unit-hid", RecordKind: "prepared", TaskID: "B-1", EventType: handoffjournal.EventWorkerBlocked, ContractSHA256: strings.Repeat("a", 64), SourceAgentPath: owner.AgentPath, SourceAttemptID: dest.AttemptID, ObservedAt: now.Format(time.RFC3339Nano)}
	original, _ := json.Marshal(record)
	original = append(original, '\n')
	if e = os.WriteFile(journal, original, 0600); e != nil {
		t.Fatal(e)
	}
	args := []string{"mark", "unit-hid", "--project", project, "--collection-project", destination, "--source-attempt", dest.AttemptID, "--step", "unit-recovery", "--result", "ok", "--json"}
	invoke := func() (int, map[string]any, string) {
		var out, errs bytes.Buffer
		code := handoff.RunCLI(args, &out, &errs)
		var got map[string]any
		if e := json.Unmarshal(out.Bytes(), &got); e != nil {
			t.Fatalf("invalid JSON: %d %q %s", code, out.String(), errs.String())
		}
		return code, got, errs.String()
	}
	unchanged := func() {
		raw, _ := os.ReadFile(journal)
		if !bytes.Equal(raw, original) {
			t.Fatal("blocked business journal changed")
		}
	}
	code, got, msg := invoke()
	if code == 0 || got["stage"] != "collection_ack" || got["executed"] != false || got["original_error"] != strings.TrimSpace(msg) || got["last_good_ack"] != nil {
		t.Fatalf("missing ACK: %d %+v %s", code, got, msg)
	}
	unchanged()
	b := runtimeobs.OperationBoundary{Kind: "start", Provider: "codex", NativeID: "fixture-native", AttemptID: dest.AttemptID, ThreadID: "fixture-thread", Role: "worker", OperationID: "unit-op", RequestID: "unit-request", Key: "unit-start", TaskIDs: []string{"B-1"}, Source: "unit_fixture", Evidence: "synthetic gate test"}
	b, _, e = runtimeobs.AppendOperationBoundary(destination, b, time.Now().UTC())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = runtimeobs.RecordCollectionReceipt(destination, runtimeobs.BoundaryCollectionReceipt(b, "ack", "", time.Now().UTC())); e != nil {
		t.Fatal(e)
	}
	// Stale persisted source observation is retained as raw evidence. Refresh uses
	// the normal host reader; there is no freshness exception or caller receipt.
	if e = runtimeobs.VerifyT3Attempt(dest, time.Now().UTC()); e == nil {
		t.Fatal("stale attempt accepted")
	}
	// The stale rejection occurs in service target validation, after entry. Its
	// diagnostic must conservatively retain unknown business execution.
	backlogDir := filepath.Join(destination, "_task_mecca", "data", "backlog")
	if e = os.MkdirAll(backlogDir, 0700); e != nil {
		t.Fatal(e)
	}
	contract, _ := os.ReadFile(filepath.Join(project, "_task_mecca", "data", "backlog", "000001.B-1.test.todo.md"))
	if e = os.WriteFile(filepath.Join(backlogDir, "000001.B-1.test.todo.md"), contract, 0600); e != nil {
		t.Fatal(e)
	}
	markArgs := args
	args = []string{"prepare", "B-1", "--project", project, "--collection-project", destination, "--source-attempt", dest.AttemptID, "--from", owner.AgentPath, "--to", owner.AgentPath, "--target-attempt", dest.AttemptID, "--event", "registration_ready", "--execution-authorized", "--json"}
	code, got, msg = invoke()
	if code == 0 || !strings.Contains(msg, "T3 observation stale") || got["executed"] != nil || got["business_execution"] != "unknown" || got["original_error"] != strings.TrimSpace(msg) || got["operation_id"] != "unit-op" || got["last_good_ack"] == nil {
		t.Fatalf("stale diagnostic: %d %+v %s", code, got, msg)
	}
	unchanged()
	args = markArgs
	t.Setenv("CODEX_THREAD_ID", "foreign-native")
	code, got, msg = invoke()
	if code == 0 || got["stage"] != "actor_identity" || got["executed"] != false || got["actor_identity_status"] != "unverified" {
		t.Fatalf("identity: %d %+v %s", code, got, msg)
	}
	unchanged()
	t.Setenv("CODEX_THREAD_ID", "fixture-native")
	if _, _, e = runtimeobs.ObserveT3Execution(project, sel, time.Now().UTC()); e != nil {
		t.Fatal(e)
	}
	if _, _, e = runtimeobs.ObserveT3Execution(destination, sel, time.Now().UTC()); e != nil {
		t.Fatal(e)
	}
	code, got, msg = invoke()
	if code != 0 {
		t.Fatalf("same fresh retry blocked: %d %+v %s", code, got, msg)
	}
	raw, _ := os.ReadFile(journal)
	if bytes.Equal(raw, original) {
		t.Fatal("successful retry did not write business journal")
	}
}
