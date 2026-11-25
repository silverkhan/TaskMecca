package webui

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/contextlife"
	"github.com/silverkhan/TaskMecca/internal/contractscope"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/reviewcontinuity"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestA102CollectionReadRoutesCannotEscapeOrReconcile(t *testing.T) {
	project := "/owned/collection-fixture-explicit"
	folder := project + "/_task_mecca/data/backlog"
	allowed := []string{"/api/activity?task=FIXTURE-1", "/api/tasks/FIXTURE-1?projection=content", "/api/tasks/FIXTURE-1?projection=lifecycle", "/api/backlog/tasks?projection=initial", "/app.js", "/style.css", "/tasks/FIXTURE-1"}
	for _, path := range allowed {
		if !collectionReadRequest(project, folder, httptest.NewRequest("GET", path, nil)) {
			t.Fatal("read route rejected", path)
		}
	}
	denied := []string{"/api/tasks/FIXTURE-1", "/api/runtime/query", "/api/attention", "/api/snapshot", "/api/events", "/api/workload", "/api/notifications", "/api/hub/projects?project=/foreign", "/api/activity?task=FIXTURE-1&project=/foreign", "/api/activity?project=/owned/collection-fixture-explicit&project=/foreign", "/api/backlog/tasks?backlog=/foreign", "/api/activity?root=/foreign", "/api/activity?runtime_diagnostic=1", "/api/web/restart", "/api/upgrade", "/api/channel", "/api/operations/action", "/sw.js", "/manifest.webmanifest"}
	for _, path := range denied {
		if collectionReadRequest(project, folder, httptest.NewRequest("GET", path, nil)) {
			t.Fatal("unsafe read/reconcile route admitted", path)
		}
	}
	for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
		if collectionReadRequest(project, folder, httptest.NewRequest(method, "/api/activity?task=FIXTURE-1", nil)) {
			t.Fatal("mutation admitted", method)
		}
	}
}

func TestA102CollectionHandlerRejectsBeforeAnyBootWork(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	t.Setenv("TASK_MECCA_PREVIEW_MODE", "1")
	t.Setenv("TASK_MECCA_COLLECTION_SOURCE_PROJECT", t.TempDir())
	fixture := t.TempDir()
	before, err := os.ReadDir(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Handler(fixture, "", "fixture-only"); err == nil {
		t.Fatal("unproven fixture started product handler")
	}
	after, err := os.ReadDir(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatal("rejected fixture wrote boot state")
	}
	escaped := filepath.Join(fixture, "escape")
	if err := os.Symlink(t.TempDir(), escaped); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(fixture, "_task_mecca"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(escaped, filepath.Join(fixture, "_task_mecca", "data")); err != nil {
		t.Fatal(err)
	}
	if collectionReadTree(fixture) == nil {
		t.Fatal("nested source read symlink admitted")
	}
}

func TestA91WorkflowPresentationPreservesMeasurementsAndCache(t *testing.T) {
	project := t.TempDir()
	facts := []activity.LifecycleFact{{EventID: "fixture", TaskID: "A-1", OccurredAt: time.Now().UTC().Add(-time.Hour)}}
	w := readActivityWorkflow(project, "A-1", nil, activity.Summarize("A-1", nil), facts)
	before := w.Snapshot
	again := readActivityWorkflow(project, "A-1", nil, activity.Summarize("A-1", nil), facts)
	if !w.ComputedAt.Equal(again.ComputedAt) || !reflect.DeepEqual(w.Axis, again.Axis) || !reflect.DeepEqual(w.Measurement, again.Measurement) {
		t.Fatal("presentation changed cache or measurement")
	}
	if before["reviewer_presentation"] == nil || again.Snapshot["reviewer_presentation"] == nil {
		t.Fatal("cachehit omitted presentation")
	}
	again.Snapshot["probe"] = "new"
	if before["probe"] != nil {
		t.Fatal("mutated cached snapshot")
	}
	encoded, _ := json.Marshal(again.Snapshot["reviewer_presentation"])
	var p activity.ReviewerPresentation
	if json.Unmarshal(encoded, &p) != nil || p.Version != 1 || p.Task.ActualState != "unknown" {
		t.Fatal(string(encoded))
	}
}

// Exercise the real persisted policy/source/runtime join, both with and without
// a continuity input. A saved input must not make foreign CLM evidence current.
func TestA91CurrentCLMIdentityMatrix(t *testing.T) {
	type clmCase struct {
		name, provider, attempt, owner, role, trust, want string
		percent                                           float64
		nilObservation                                    bool
	}
	cases := []clmCase{
		{name: "nil", nilObservation: true},
		{name: "empty_provider", provider: "", attempt: "run-review-fixture"},
		{name: "foreign_provider", provider: "claude", attempt: "run-review-fixture"},
		{name: "empty_attempt", provider: "codex"},
		{name: "foreign_attempt", provider: "codex", attempt: "run-other"},
		{name: "foreign_owner", provider: "codex", attempt: "run-review-fixture", owner: "run-other"},
		{name: "foreign_role", provider: "codex", attempt: "run-review-fixture", role: "reviewer:other"},
	}
	for _, pct := range []float64{30, 65, 80, 90, 95} {
		cases = append(cases, clmCase{name: fmt.Sprintf("matching_%g", pct), provider: "codex", attempt: "run-review-fixture", trust: "provider_verified", want: "measured", percent: pct})
	}
	for _, trust := range []string{"provider_structured", "reported_unverified"} {
		cases = append(cases, clmCase{name: trust, provider: "codex", attempt: "run-review-fixture", trust: trust, want: trust, percent: 30})
	}
	for _, saved := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("saved_%t/%s", saved, tc.name), func(t *testing.T) {

				t.Setenv("TASK_MECCA_HOME", t.TempDir())
				project := t.TempDir()
				registerWebFixture(t, project)
				source := t.TempDir()
				if b, e := exec.Command("git", "init", "-q", source).CombinedOutput(); e != nil {
					t.Fatal(string(b), e)
				}
				if e := os.WriteFile(filepath.Join(source, "product.go"), []byte("package fixture\n"), 0600); e != nil {
					t.Fatal(e)
				}
				p := rolepolicy.Policy{SchemaVersion: 1, Method: "worker", Workspace: source, Scope: []string{"product.go"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/fixtureworker", NativeID: "native-implementation"}, Reviewer: rolepolicy.Owner{Role: "reviewer", AgentPath: "/root/controller/reviewer/fixturereviewer", NativeID: "native-reviewer"}, ReviewRequirement: &rolepolicy.ReviewRequirement{Scope: []string{"product.go"}, Rationale: "explicit fixture contract", Risk: rolepolicy.ReviewRisk{Impact: "high", Complexity: "complex", Reversibility: "easy", Verifiability: "established", ExplicitRequired: true}}}
				b, _ := json.Marshal(p)
				dir := filepath.Join(project, "_task_mecca", "data", "backlog")
				os.MkdirAll(dir, 0700)
				os.WriteFile(filepath.Join(dir, "000001.A-1.fixture.doing.md"), []byte("# A-1 Fixture\n## 요건 정의서\n"+rolepolicy.Fence+string(b)+"\n```\n### 수용 기준\n- [ ] explicit fixture\n"), 0600)
				at := time.Now().UTC().Add(-time.Minute)
				seed := func(run, native, path, assignment string) {
					e := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventID: run, EventKind: "state", AttemptID: run, Provider: "codex", SessionID: native, RuntimeAgentID: native, State: runtimeobs.StateRunning, ObservedAt: at.Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved})
					if e != nil {
						t.Fatal(e)
					}
					if _, e = runtimeobs.BindAttempt(project, run, "A-1", path, "explicit", "", map[string]string{"assignment_id": assignment}, at.Add(time.Second)); e != nil {
						t.Fatal(e)
					}
				}
				seed("run-impl-fixture", "native-implementation", p.Implementation.AgentPath, "assignment-impl")
				seed("run-review-fixture", "native-reviewer", p.Reviewer.AgentPath, "assignment-review")
				got := readReviewerPresentation(project, "A-1", nil)
				v, ok := got.ByAttempt["run-review-fixture"]
				if !ok || !v.PurposeVerified || v.Purpose != "review" || v.Necessity != "required" || v.ActualState != "unknown" || v.ContextStatus != "unknown" || v.SessionGeneration != nil {
					t.Fatal("actual persisted policy/source did not project", got)
				}

				owner := tc.owner
				if owner == "" {
					owner = "run-review-fixture"
				}
				role := tc.role
				if role == "" {
					role = "reviewer:fixturereviewer"
				}
				trust := tc.trust
				if trust == "" {
					trust = "provider_verified"
				}
				percent := tc.percent
				if percent == 0 {
					percent = 30
				}
				state := contextlife.State{Version: 1, Role: role, Generation: 1, OwnerAttempt: owner}
				if !tc.nilObservation {
					state.Observation = &contextlife.Observation{AttemptID: tc.attempt, Provider: tc.provider, WindowTokens: 100, UsagePercent: &percent, Trust: trust, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
				}
				raw, e := json.Marshal(state)
				if e != nil {
					t.Fatal(e)
				}
				path := filepath.Join(project, "_task_mecca", ".runtime", "context_lifecycle", "roles", "reviewer-fixturereviewer.json")
				if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(path, raw, 0600); e != nil {
					t.Fatal(e)
				}
				if saved {
					report := contextlife.StatusReport{State: state}
					raw, e = json.Marshal(report)
					if e != nil {
						t.Fatal(e)
					}
					if e = os.WriteFile(filepath.Join(source, "context.json"), raw, 0600); e != nil {
						t.Fatal(e)
					}
					sum := sha256.Sum256(raw)
					identity := func(attempt, native, agent string) rolepolicy.Identity {
						return rolepolicy.Identity{AttemptID: attempt, NativeID: native, RuntimeAgentID: native, Provider: "codex", AgentPath: agent, TaskID: "A-1", Bound: true, Observed: true}
					}
					snapshot, e := rolepolicy.Snapshot(source, p.Scope)
					if e != nil {
						t.Fatal(e)
					}
					markdown, e := os.ReadFile(filepath.Join(dir, "000001.A-1.fixture.doing.md"))
					if e != nil {
						t.Fatal(e)
					}
					request := rolepolicy.ReviewRequest{TaskID: "A-1", AssignmentID: "assignment-review", ContractSHA256: contractscope.Fingerprint(string(markdown)), Requirement: *p.ReviewRequirement, Source: snapshot, Implementation: identity("run-impl-fixture", "native-implementation", p.Implementation.AgentPath), Reviewer: identity("run-review-fixture", "native-reviewer", p.Reviewer.AgentPath), ContextRef: "context.json"}
					input := reviewcontinuity.Input{Current: reviewcontinuity.Snapshot{Request: request, SessionGeneration: 1}, ContextRef: reviewcontinuity.Reference{Path: "context.json", SHA256: fmt.Sprintf("%x", sum)}, Host: reviewcontinuity.Host{Kind: "t3"}}
					raw, e = json.Marshal(map[string]any{"policy_sha256": rolepolicy.PolicyDigest(&p), "input": input})
					if e != nil {
						t.Fatal(e)
					}
					path = filepath.Join(project, "_task_mecca", ".runtime", "reviewer_continuity", "run-review-fixture.json")
					if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
						t.Fatal(e)
					}
					if e = os.WriteFile(path, raw, 0600); e != nil {
						t.Fatal(e)
					}
				}
				got = readReviewerPresentation(project, "A-1", nil)
				v, ok = got.ByAttempt["run-review-fixture"]
				want := tc.want
				if want == "" {
					want = "unknown"
				}
				if !ok || !v.PurposeVerified || v.ContextStatus != want {
					t.Fatalf("want %s: %+v", want, v)
				}
				if saved && want != "unknown" && percent < 90 {
					if v.ContextRef != "context.json" || v.SessionGeneration == nil || *v.SessionGeneration != 1 {
						t.Fatalf("valid saved input was not consumed: %+v", v)
					}
				}
				if want == "unknown" && (v.SessionGeneration != nil || v.ContextRef != "") {
					t.Fatalf("foreign evidence derived continuity: %+v", v)
				}
				if v.ReviewRound != nil || v.ChainID != "" || len(v.SourceDelta) != 0 {
					t.Fatalf("unrelated continuity inferred: %+v", v)
				}
			})
		}
	}
}

func TestA99ActualAssignedWorkerPurposeSourceAndCurrentProjection(t *testing.T) {
	project, source := t.TempDir(), t.TempDir()
	if raw, err := exec.Command("git", "init", "-q", source).CombinedOutput(); err != nil {
		t.Fatal(err, string(raw))
	}
	os.WriteFile(filepath.Join(source, "product.go"), []byte("package product\n"), 0600)
	p := rolepolicy.Policy{SchemaVersion: 1, Method: "worker", Workspace: source, Scope: []string{"product.go"}, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/impl", NativeID: "impl-native"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Reviewer: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/pikachyu", NativeID: "review-native"}, ReviewRequirement: &rolepolicy.ReviewRequirement{Scope: []string{"product.go"}, Risk: rolepolicy.ReviewRisk{Impact: "high", Complexity: "complex", Reversibility: "easy", Verifiability: "established", ExplicitRequired: true, TrustBoundary: true}, Rationale: "actual independent review"}}
	rawPolicy, _ := json.Marshal(p)
	raw := "# A-1 fixture\n\n## 작업 정의\n### 목표\n정확한 검수\n### 수용 기준\n- [ ] 원문 보존\n\n### 역할 정책\n" + rolepolicy.Fence + string(rawPolicy) + "\n```\n"
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	os.MkdirAll(folder, 0700)
	os.WriteFile(filepath.Join(folder, "000001.A-1.fixture.doing.md"), []byte(raw), 0600)
	now := time.Now().UTC().Add(-time.Minute)
	for _, actor := range []struct{ id, native, path, assignment string }{{"impl", "impl-native", p.Implementation.AgentPath, "impl-assignment"}, {"review", "review-native", p.Reviewer.AgentPath, "review-assignment"}} {
		runtimeobs.RecordAssignment(project, actor.assignment, "A-1", actor.path, now)
		runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventID: actor.id, EventKind: "state", AttemptID: actor.id, Provider: "codex", SessionID: actor.native, RuntimeAgentID: actor.native, State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved, ObservedAt: now.Format(time.RFC3339Nano)})
		if _, err := runtimeobs.BindAttempt(project, actor.id, "A-1", actor.path, "dispatch", "", map[string]string{"assignment_id": actor.assignment}, now); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, _ := rolepolicy.Snapshot(source, p.Scope)
	e := activity.Event{SchemaVersion: 1, EventID: "explicit-review", TaskID: "A-1", Action: activity.Start, Role: activity.RoleWorker, AgentID: "review-native", RuntimeAttemptID: "review", AssignmentID: "review-assignment", Provider: "codex", SessionID: "review-native", ContractSHA256: contractscope.Fingerprint(raw), TaskRevision: activity.ReviewSourceRevision(snapshot), Type: activity.Other, Explanation: "explicit review", Evidence: "fixture source-linked review", Confidence: activity.Inferred, OccurredAt: now.Add(time.Second), RecordedAt: now.Add(time.Second)}
	if _, err := (activity.Ledger{Project: project}).Apply(e); err != nil {
		t.Fatal(err)
	}
	got := readReviewerPresentation(project, "A-1", nil)
	v := got.ByAttempt["review"]
	if !v.LinkageVerified || !v.PurposeVerified || v.StoredRole != "worker" || v.ActualState != "reviewing" || v.AssignmentID != "review-assignment" {
		t.Fatalf("actual Worker purpose not projected %+v", v)
	}
	model := readActivityWorkflow(project, "A-1", []activity.Event{e}, activity.Summarize("A-1", []activity.Event{e}), nil)
	found := false
	for _, row := range model.Rows {
		for _, interval := range row.Intervals {
			if interval.ID == e.EventID {
				found = true
				if interval.Type != "other" || !interval.PurposeVerified || interval.Purpose != "review" || interval.TaskRevision != e.TaskRevision {
					t.Fatal("individual purpose missing or raw type changed", interval)
				}
			}
		}
	}
	if !found {
		t.Fatal("source Activity absent from actual workflow API")
	}
	w := activity.Workflow{TaskID: "A-1", Current: activity.WorkflowCurrent{Phase: "implementation", NextAction: "implement_and_verify"}, Snapshot: map[string]any{}, Rows: []activity.WorkflowRow{}}
	projected := attachReviewerPresentation(project, "A-1", w)
	if projected.Current.Phase != "independent_review" || projected.Current.NextAction != "await_independent_review" {
		t.Fatal("review regressed into implementation", projected.Current)
	}
	os.WriteFile(filepath.Join(source, "product.go"), []byte("package changed\n"), 0600)
	changed := readReviewerPresentation(project, "A-1", nil).ByAttempt["review"]
	if changed.ActualState == "reviewing" || changed.ActualState == "accepted" {
		t.Fatal("old source review reused", changed)
	}
}

func TestA102ClaimHistoryReadFailsClosed(t *testing.T) {
	for _, mode := range []string{"valid", "malformed", "missing_id", "symlink", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			p := t.TempDir()
			path := handoffjournal.JournalPath(p)
			if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
				t.Fatal(e)
			}
			raw := []byte("{\"handoff_id\":\"unit\",\"record_kind\":\"prepared\"}\n")
			if mode == "malformed" {
				raw = append(raw, []byte("{broken\n")...)
			}
			if mode == "missing_id" {
				raw = []byte("{}\n")
			}
			if mode == "symlink" {
				target := filepath.Join(t.TempDir(), "foreign")
				os.WriteFile(target, raw, 0600)
				if e := os.Symlink(target, path); e != nil {
					t.Fatal(e)
				}
			} else {
				if e := os.WriteFile(path, raw, 0600); e != nil {
					t.Fatal(e)
				}
				if mode == "oversize" {
					if e := os.Truncate(path, (32<<20)+1); e != nil {
						t.Fatal(e)
					}
				}
			}
			got := readWorkflowClaimRecords(p)
			if (mode == "valid") != (len(got) == 1) {
				t.Fatalf("%s: %+v", mode, got)
			}
		})
	}
}

// Opt-in read-only probe of the same product projection, not an actor simulator.
// The OS profile denies writes to the supplied source; only output is writable.
func TestA102ReadActualWorkflowProjection(t *testing.T) {
	project, task, out := os.Getenv("A102_READ_PROJECT"), os.Getenv("A102_READ_TASK"), os.Getenv("A102_READ_OUTPUT")
	if project == "" || task == "" || out == "" {
		t.Skip("opt-in actual read-only evidence")
	}
	events, e := (activity.Ledger{Project: project}).List(task)
	if e != nil {
		t.Fatal(e)
	}
	facts, _, e := readActivityLifecycle(project, task)
	if e != nil {
		t.Fatal(e)
	}
	workflow := readActivityWorkflow(project, task, events, activity.Summarize(task, events), facts)
	raw, e := json.MarshalIndent(workflow, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(out, raw, 0600); e != nil {
		t.Fatal(e)
	}
}
