package webui

import (
	stdcontext "context"
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestA91PresentationEnvelopeKeepsEveryOriginalKey(t *testing.T) {
	original := map[string]any{"task_ids": []string{"A-88"}, "active": map[string]any{"items": []any{}}, "measurement": nil, "large": json.RawMessage("12345678901234567890")}
	p := activity.ReviewerPresentation{Version: 1, ByAttempt: map[string]activity.ReviewerPresentationEntry{}}
	envelope := reviewerPresentationEnvelope(original, p)
	for k, v := range original {
		want, _ := json.Marshal(v)
		got, _ := json.Marshal(envelope[k])
		if string(got) != string(want) {
			t.Fatal("original key changed", k, string(got), string(want))
		}
	}
	if len(envelope) != len(original)+1 {
		t.Fatal(envelope)
	}
}
func TestA91WorkloadListAndDetailActuallyCarryPresentation(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	project := t.TempDir()
	registerWebFixture(t, project)
	oldRefresh, oldObserve := refreshWorkloadNames, observeWorkloadRoot
	defer func() { refreshWorkloadNames = oldRefresh; observeWorkloadRoot = oldObserve }()
	refreshWorkloadNames = func(string, runtimeobs.Ledger, time.Time) (bool, error) { return false, nil }
	observeWorkloadRoot = func(stdcontext.Context, runtimeobs.RootSession, time.Time) runtimeobs.RootSessionExecution {
		return runtimeobs.RootSessionExecution{State: runtimeobs.StateRuntimeUnknown}
	}
	if e := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventID: "fixture-root", EventKind: "state", AttemptID: "run-fixture", Provider: "codex", SessionID: "native-fixture", RuntimeAgentID: "native-fixture", State: runtimeobs.StateCompleted, Terminal: true, ObservedAt: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); e != nil {
		t.Fatal(e)
	}
	request := func(root string) map[string]json.RawMessage {
		q := url.Values{"project": {project}}
		if root != "" {
			q.Set("root_session_id", root)
		}
		r := httptest.NewRecorder()
		workloadSessionsHandler(r, httptest.NewRequest(http.MethodGet, "/api/runtime/workload-sessions?"+q.Encode(), nil))
		if r.Code != 200 {
			t.Fatal(r.Code, r.Body.String())
		}
		var body map[string]json.RawMessage
		if json.Unmarshal(r.Body.Bytes(), &body) != nil || body["reviewer_presentation"] == nil {
			t.Fatal(r.Body.String())
		}
		return body
	}
	list := request("")
	var data runtimeobs.SessionWorkload
	b, _ := json.Marshal(list)
	if json.Unmarshal(b, &data) != nil {
		t.Fatal("list schema")
	}
	var id string
	for _, r := range data.Historical.Items {
		id = r.RootSessionID
	}
	for _, r := range data.Active.Items {
		id = r.RootSessionID
	}
	if id == "" {
		t.Fatal("fixture root missing", string(b))
	}
	detail := request(id)
	if detail["root_session"] == nil {
		t.Fatal("detail keys lost", detail)
	}
}
func TestA91TaskDetailActuallyCarriesPresentation(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	project := t.TempDir()
	registerWebFixture(t, project)
	dir := filepath.Join(project, "_task_mecca", "data", "backlog")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "000001.A-1.fixture.done.md"), []byte("# A-1 Fixture\n## 작업 정의\nOriginal body\n"), 0600)
	h, e := Handler(project, "", "test")
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/tasks/A-1", nil))
	var body map[string]json.RawMessage
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &body) != nil || body["reviewer_presentation"] == nil {
		t.Fatal(r.Code, r.Body.String())
	}
}

func TestA91PresentationReadsActualPolicySourceAndImplementationHistory(t *testing.T) {
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
	// Actual prior implementation cannot be erased by a Reviewer role/path.
	event := activity.Event{SchemaVersion: 1, EventID: "fixture-prior-implementation", TaskID: "A-1", Role: activity.RoleWorker, AgentID: "native-reviewer", RuntimeAttemptID: "run-review-fixture", AssignmentID: "assignment-review", Source: "agent_report", Action: activity.Start, Type: activity.Implementation, Confidence: activity.Inferred, Evidence: "fixture prior product implementation", OccurredAt: at, RecordedAt: at}
	if _, e := (activity.Ledger{Project: project}).Append(event); e != nil {
		t.Fatal(e)
	}
	got = readReviewerPresentation(project, "A-1", nil)
	if got.ByAttempt["run-review-fixture"].PurposeVerified {
		t.Fatal("self-review history hidden by new role", got)
	}
}
