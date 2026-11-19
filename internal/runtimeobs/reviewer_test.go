package runtimeobs

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestA90ReviewerSTARTUsesReadOnlyPolicyAndActualNative(t *testing.T) {
	in, store, _, now := roleFixture(t, "worker")
	in.Role = "reviewer"
	in.AgentPath = "/root/controller/reviewer/raichyu"
	in.AssignmentID = "review-assignment"
	if _, err := RecordAssignment(in.CanonicalProject, in.AssignmentID, in.TaskID, in.AgentPath, now); err != nil {
		t.Fatal(err)
	}
	p := &rolepolicy.Policy{SchemaVersion: 1, Method: "worker", Workspace: in.SourceWorktree, Scope: []string{"product.go"}, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/isanghaessi", NativeID: "impl-native"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Reviewer: rolepolicy.Owner{Role: "reviewer", AgentPath: in.AgentPath, NativeID: in.Selector.SessionID}}
	dir := filepath.Join(in.CanonicalProject, "_task_mecca/data/backlog")
	os.MkdirAll(dir, 0700)
	writePolicy := func() {
		raw, _ := json.Marshal(p)
		os.WriteFile(filepath.Join(dir, "000081.A-81.a90.todo.md"), []byte("# A-81 review fixture\n\n## 요건 정의서\n"+rolepolicy.Fence+string(raw)+"\n```\n\n## 실행 정보\n"), 0600)
	}
	writePolicy()
	os.WriteFile(filepath.Join(in.SourceWorktree, "product.go"), []byte("package candidate\n"), 0600)
	for _, e := range []ExecutionEvent{
		{EventKind: "state", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: "impl-run", Provider: "codex", SessionID: "impl-native", RuntimeAgentID: "impl-native", State: StateRunning, EvidenceSource: EvidenceHook, ObservationQuality: QualityObserved},
		{EventKind: "binding", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: "impl-run", TaskID: in.TaskID, AgentPath: p.Implementation.AgentPath, BindingSource: "dispatch", EvidenceSource: EvidenceManualBinding, ObservationQuality: QualityAuthoritative},
	} {
		if err := AppendExecutionEvent(in.CanonicalProject, e); err != nil {
			t.Fatal(err)
		}
	}
	observe := func(project string, s T3Selector, n time.Time) (Attempt, T3Receipt, error) {
		return observeT3Execution(project, s, n, store.read)
	}
	got, err := observeRoleCollection(in, now, store.verifyRoleScope, observe)
	if err != nil || got.Status != "bound" || got.Attempt.AgentPath != in.AgentPath {
		t.Fatal(got, err)
	}
	p.Implementation.NativeID = in.Selector.SessionID
	writePolicy()
	if _, err := observeRoleCollection(in, now.Add(time.Second), store.verifyRoleScope, observe); err == nil {
		t.Fatal("self review accepted after role rename")
	}
}

func TestA90ImplementationHistorySurvivesChangedOwnerPath(t *testing.T) {
	in, _, _, now := roleFixture(t, "worker")
	for _, e := range []ExecutionEvent{
		{EventKind: "state", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: "past-run", Provider: "codex", SessionID: "past-native", RuntimeAgentID: "past-native", State: StateRunning, EvidenceSource: EvidenceHook, ObservationQuality: QualityObserved},
		{EventKind: "binding", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: "past-run", TaskID: in.TaskID, AgentPath: "/root/controller/former", BindingSource: "dispatch", BindingEvidence: map[string]string{"assignment_id": "past-assignment"}, EvidenceSource: EvidenceManualBinding, ObservationQuality: QualityAuthoritative},
	} {
		if err := AppendExecutionEvent(in.CanonicalProject, e); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(in.CanonicalProject, "_task_mecca/data/activity/events")
	os.MkdirAll(dir, 0700)
	raw := `{"task_id":"A-81","type":"implementation","runtime_attempt_id":"past-run","agent_id":"past-native","assignment_id":"past-assignment"}`
	os.WriteFile(filepath.Join(dir, "past.json"), []byte(raw), 0600)
	p := &rolepolicy.Policy{Implementation: rolepolicy.Owner{AgentPath: "/root/controller/newowner"}}
	history, err := ProductImplementationHistory(in.CanonicalProject, in.TaskID, p, now)
	if err != nil || len(history) != 1 {
		t.Fatal(history, err)
	}
	reviewer := history[0]
	reviewer.AgentPath = "/root/controller/reviewer/newname"
	reviewer.AttemptID = "another-turn"
	if err := rolepolicy.ValidateImplementationHistory(in.TaskID, reviewer, history); err == nil {
		t.Fatal("new owner/path hid past product implementation")
	}
}

func TestA90PriorTaskImplementationOfSameSourceIsNotIndependent(t *testing.T) {
	in, _, _, now := roleFixture(t, "worker")
	for _, e := range []ExecutionEvent{
		{EventKind: "state", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: "old-task-run", Provider: "codex", SessionID: "reused-native", RuntimeAgentID: "reused-native", State: StateRunning, EvidenceSource: EvidenceHook, ObservationQuality: QualityObserved},
		{EventKind: "binding", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: "old-task-run", TaskID: "A-80", AgentPath: "/root/controller/oldowner", BindingSource: "dispatch", EvidenceSource: EvidenceManualBinding, ObservationQuality: QualityAuthoritative},
	} {
		if err := AppendExecutionEvent(in.CanonicalProject, e); err != nil {
			t.Fatal(err)
		}
	}
	p := &rolepolicy.Policy{SchemaVersion: 1, Method: "worker", Scope: []string{"product.go"}, Implementation: rolepolicy.Owner{AgentPath: "/root/controller/oldowner"}, Reviewer: rolepolicy.Owner{NativeID: "reused-native"}}
	dir := filepath.Join(in.CanonicalProject, "_task_mecca/data/backlog")
	os.MkdirAll(dir, 0700)
	write := func() {
		raw, _ := json.Marshal(p)
		os.WriteFile(filepath.Join(dir, "000080.A-80.fixture.todo.md"), []byte("# A-80\n\n## 요건 정의서\n"+rolepolicy.Fence+string(raw)+"\n```\n\n## 실행 정보\n"), 0600)
	}
	write()
	current := *p
	current.Implementation.AgentPath = "/root/controller/newowner"
	history, err := ProductImplementationHistory(in.CanonicalProject, in.TaskID, &current, now)
	if err != nil || len(history) != 1 || history[0].TaskID != "A-80" {
		t.Fatal(history, err)
	}
	reviewer := history[0]
	reviewer.TaskID = in.TaskID
	reviewer.AgentPath = "/root/controller/reviewer/newname"
	if err := rolepolicy.ValidateImplementationHistory(in.TaskID, reviewer, history); err == nil {
		t.Fatal("same-source prior-task contributor accepted")
	}
	p.Scope = []string{"unrelated.go"}
	write()
	history, err = ProductImplementationHistory(in.CanonicalProject, in.TaskID, &current, now)
	if err != nil || len(history) != 0 {
		t.Fatal("unrelated product scope incorrectly excluded reviewer", history, err)
	}
}
