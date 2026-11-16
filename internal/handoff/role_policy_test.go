package handoff

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func a78PolicyFixture(t *testing.T) (string, string, *rolepolicy.Policy, time.Time) {
	t.Helper()
	project, source := t.TempDir(), t.TempDir()
	writeSimpleTask(t, project, "A-1")
	p := &rolepolicy.Policy{SchemaVersion: 1, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/kkobugi"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Reviewer: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/pikachu"}, Method: "worker", Workspace: source, Scope: []string{"source.go"}}
	file := filepath.Join(project, "_task_mecca/data/backlog/000001.A-1.handoff-test.todo.md")
	b, _ := os.ReadFile(file)
	encoded, _ := json.Marshal(p)
	os.WriteFile(file, []byte(strings.Replace(string(b), "### 수용 기준", rolepolicy.Fence+string(encoded)+"\n```\n\n### 수용 기준", 1)), 0600)
	now := time.Date(2026, 11, 16, 1, 0, 0, 0, time.UTC)
	for _, event := range []runtimeobs.ExecutionEvent{
		{EventKind: "state", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: "worker-run", Provider: "codex", SessionID: "worker-native", RuntimeAgentID: "worker-native", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved},
		{EventKind: "binding", ObservedAt: now.Add(time.Second).Format(time.RFC3339Nano), AttemptID: "worker-run", TaskID: "A-1", AgentPath: p.Implementation.AgentPath, BindingSource: "dispatch", EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative},
	} {
		if e := runtimeobs.AppendExecutionEvent(project, event); e != nil {
			t.Fatal(e)
		}
	}
	appendControllerAttempt(t, project, "controller-run", "controller-native", "controller-native", runtimeobs.StateRunning, now.Add(2*time.Second))
	return project, source, p, now.Add(time.Minute)
}

func TestA78SupportedHandoffAndAcceptanceHold(t *testing.T) {
	project, source, _, now := a78PolicyFixture(t)
	report := filepath.Join(source, "report.md")
	os.WriteFile(report, []byte("worker candidate, separate reviewer pending"), 0600)
	req := PrepareRequest{Project: project, TaskID: "A-1", EventType: EventWorkerDone, SourceAgentPath: "/root/controller/kkobugi", SourceAttemptID: "worker-run", SourceWorkspace: source, TargetAgentPath: "/root/controller", TargetAttemptID: "controller-run", ReportFile: report}
	wrong := req
	wrong.SourceWorkspace = "/wrong/workspace"
	if _, e := Prepare(wrong, now); e == nil {
		t.Fatal("workspace mismatch accepted")
	}
	prepared, e := Prepare(req, now)
	if e != nil || prepared.RolePolicy == nil || !prepared.RolePolicy.Allowed {
		t.Fatal(prepared, e)
	}
	claimed, e := Claim(project, prepared.HandoffID, "/root/controller", "controller-run", now.Add(time.Second))
	if e != nil || !claimed.Claimed {
		t.Fatal(claimed, e)
	}
	if _, e = MarkAs(project, prepared.HandoffID, "acceptance", "ok", "independent:true / Controller changed role", "controller-run", now.Add(2*time.Second)); e == nil || !strings.Contains(e.Error(), "independent_review_pending") {
		t.Fatal("fake independent acceptance", e)
	}
	if _, e = MarkAs(project, prepared.HandoffID, "applied", "ok", "same source claim", "controller-run", now.Add(3*time.Second)); e == nil {
		t.Fatal("applied bypassed review")
	}
	row, _, _ := Inspect(project, prepared.HandoffID, now)
	if _, exists := row.Steps["acceptance"]; exists || row.Applied {
		t.Fatal("failed gate changed ledger", row)
	}
	// Registration/claim/operational integration remains Controller work.
	if _, e = Prepare(PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAttemptID: "controller-run", ExecutionAuthorized: true}, now); e != nil {
		t.Fatal("normal operational handoff blocked", e)
	}
}

func TestA78ControllerExceptionNeedsActualSeparateReviewer(t *testing.T) {
	project, source, p, now := a78PolicyFixture(t)
	p.Method = "controller_exception"
	p.Implementation = rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}
	p.Reviewer.NativeID = "unobserved-native"
	p.Exception = &rolepolicy.Exception{Reason: "slot shortage fact", Evidence: "actual user limited policy decision", Scope: p.Scope, ApprovalEventID: "role-approval", ResumeWhen: "bind actual reviewer"}
	file := filepath.Join(project, "_task_mecca/data/backlog/000001.A-1.handoff-test.todo.md")
	b, _ := os.ReadFile(file)
	start := strings.Index(string(b), rolepolicy.Fence)
	end := strings.Index(string(b)[start+len(rolepolicy.Fence):], "\n```") + start + len(rolepolicy.Fence) + 4
	encoded, _ := json.Marshal(p)
	raw := string(b[:start]) + rolepolicy.Fence + string(encoded) + "\n```" + string(b[end:])
	os.WriteFile(file, []byte(raw), 0600)
	anchorFile := filepath.Join(source, "anchor")
	os.WriteFile(anchorFile, []byte(rolepolicy.DecisionScopeRaw(raw)), 0600)
	base, _ := rolepolicy.HashFile(anchorFile)
	dir := filepath.Join(project, "_task_mecca/data/contract_decisions/events")
	os.MkdirAll(dir, 0700)
	event, _ := json.Marshal(map[string]any{"event_id": "role-approval", "task_id": "A-1", "kind": "policy_confirmed", "policy_key": "implementation-role-exception", "user_confirmed": true, "base_sha256": base, "evidence": "user decision", "role_policy_sha256": rolepolicy.PolicyDigest(p)})
	os.WriteFile(filepath.Join(dir, "role-approval.json"), event, 0600)
	a, e := AssessPolicy(project, "A-1", "controller-run", source, false, now)
	if e == nil || a.Allowed || a.Reason != "exception_actual_separate_reviewer_unverified" {
		t.Fatal(a, e)
	}
}

func TestA78ActualReviewAssessmentRevalidatesProductSource(t *testing.T) {
	project, source, p, now := a78PolicyFixture(t)
	if b, e := exec.Command("git", "init", "-q", source).CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	os.WriteFile(filepath.Join(source, "source.go"), []byte("reviewed candidate"), 0600)
	report := filepath.Join(source, "review.md")
	os.WriteFile(report, []byte("actual separate reviewer report fixture"), 0600)
	p.Review = &rolepolicy.Review{AttemptID: "review-run", ReportFile: report, SourceWorkspace: source}
	file := filepath.Join(project, "_task_mecca/data/backlog/000001.A-1.handoff-test.todo.md")
	b, _ := os.ReadFile(file)
	start := strings.Index(string(b), rolepolicy.Fence)
	end := strings.Index(string(b)[start+len(rolepolicy.Fence):], "\n```") + start + len(rolepolicy.Fence) + 4
	encoded, _ := json.Marshal(p)
	os.WriteFile(file, []byte(string(b[:start])+rolepolicy.Fence+string(encoded)+"\n```"+string(b[end:])), 0600)
	for _, e := range []runtimeobs.ExecutionEvent{
		{EventKind: "state", ObservedAt: now.Add(-10 * time.Second).Format(time.RFC3339Nano), AttemptID: "review-run", Provider: "codex", SessionID: "reviewer-native", RuntimeAgentID: "reviewer-native", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved},
		{EventKind: "binding", ObservedAt: now.Add(-9 * time.Second).Format(time.RFC3339Nano), AttemptID: "review-run", TaskID: "A-1", AgentPath: p.Reviewer.AgentPath, BindingSource: "dispatch", EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative},
	} {
		if err := runtimeobs.AppendExecutionEvent(project, e); err != nil {
			t.Fatal(err)
		}
	}
	reviewed, _ := rolepolicy.Snapshot(source, p.Scope)
	hash, _ := rolepolicy.HashFile(report)
	statement := rolepolicy.ReviewStatement{TaskID: "A-1", ReviewerAttemptID: "review-run", ImplementationNativeID: "worker-native", Source: reviewed, ReportSHA256: hash, Result: "accepted"}
	reader := func(_ context.Context, a runtimeobs.Attempt, task, reportHash string, _ time.Time) (rolepolicy.ReviewStatement, error) {
		if a.SessionID != "reviewer-native" || task != "A-1" || reportHash != hash {
			t.Fatal("wrong actual query target", a, task, reportHash)
		}
		return statement, nil
	}
	a, e := assessPolicy(project, "A-1", "worker-run", source, true, now, reader)
	if e != nil || a.Review != "independent" {
		t.Fatal(a, e)
	}
	os.WriteFile(filepath.Join(source, "operations.md"), []byte("normal merge note"), 0600)
	if _, e = assessPolicy(project, "A-1", "worker-run", source, true, now, reader); e != nil {
		t.Fatal("operational change invalidated product review", e)
	}
	changingReader := func(ctx context.Context, a runtimeobs.Attempt, task, hash string, n time.Time) (rolepolicy.ReviewStatement, error) {
		os.WriteFile(filepath.Join(source, "source.go"), []byte("concurrent change"), 0600)
		return reader(ctx, a, task, hash, n)
	}
	if _, e = assessPolicy(project, "A-1", "worker-run", source, true, now, changingReader); e == nil || !strings.Contains(e.Error(), "during_review_check") {
		t.Fatal("concurrent change accepted", e)
	}
	os.WriteFile(filepath.Join(source, "source.go"), []byte("later product change"), 0600)
	a, e = assessPolicy(project, "A-1", "worker-run", source, true, now, reader)
	if e == nil || a.Review == "independent" {
		t.Fatal("source change reused review", a, e)
	}
}
