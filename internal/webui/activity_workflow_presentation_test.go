package webui

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/contextlife"
	"github.com/silverkhan/TaskMecca/internal/contractscope"
	"github.com/silverkhan/TaskMecca/internal/reviewcontinuity"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

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
