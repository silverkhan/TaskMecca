package main

import (
	"bytes"
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/handoff"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestA90RepairControllerCLIStillRequiresActualClaim(t *testing.T) {
	project := makeDecisionFixture(t)
	now := time.Now().UTC()
	if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: "controller-run", Provider: "codex", SessionID: "controller-native", RuntimeAgentID: "controller-native", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.BindAttempt(project, "controller-run", "B-1", "/root/controller", "dispatch", "", nil, now); err != nil {
		t.Fatal(err)
	}
	h, err := handoff.Prepare(handoff.PrepareRequest{Project: project, TaskID: "B-1", EventType: handoff.EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAttemptID: "controller-run", ExecutionAuthorized: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handoff.Claim(project, h.HandoffID, "/root/controller", "controller-run", now); err != nil {
		t.Fatal(err)
	}
	args := []string{"start", "B-1", "--project", project, "--role", "controller", "--agent", "controller-native", "--attempt", "controller-run", "--handoff", "missing-claim", "--type", "controller_review", "--event-id", "controller-start", "--evidence", "isolated actual Controller callback", "--json"}
	if code, _, msg := invokeActivity(t, args...); code == 0 {
		t.Fatal("missing Controller claim accepted", msg)
	}
	for i := range args {
		if args[i] == "--handoff" {
			args[i+1] = h.HandoffID
		}
	}
	if code, _, msg := invokeActivity(t, args...); code != 0 {
		t.Fatal("valid Controller claim rejected", code, msg)
	}
	args[0] = "stop"
	for i := range args {
		if args[i] == "--event-id" {
			args[i+1] = "controller-stop"
		}
	}
	if code, _, msg := invokeActivity(t, args...); code != 0 {
		t.Fatal("Controller stop", code, msg)
	}
}

// This fixture calls the real CLI writer/ApplyVerified callback. It does not
// substitute a helper-level attribution result for an Activity recording.
func a90RepairReviewerFixture(t *testing.T, approved bool) (string, string, *rolepolicy.Policy) {
	t.Helper()
	project, source := makeDecisionFixture(t), t.TempDir()
	if out, err := exec.Command("git", "init", "-q", source).CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	if err := os.WriteFile(filepath.Join(source, "source.go"), []byte("package candidate\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p := &rolepolicy.Policy{SchemaVersion: 1, Method: "worker", Workspace: source, Scope: []string{"source.go"}, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/isanghaessi", NativeID: "implementation-native"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Reviewer: rolepolicy.Owner{Role: "reviewer", AgentPath: "/root/controller/reviewer/raichyu", NativeID: "review-native"}, ReviewRequirement: &rolepolicy.ReviewRequirement{Scope: []string{"source.go"}, Rationale: "authority boundary requires independent review", Risk: rolepolicy.ReviewRisk{Impact: "high", Complexity: "complex", Reversibility: "difficult", Verifiability: "limited", BehaviorChanged: true, ExplicitRequired: true, TrustBoundary: true}}}
	raw, _ := json.Marshal(p)
	body := filepath.Join(t.TempDir(), "policy.md")
	if err := os.WriteFile(body, []byte(rolepolicy.Fence+string(raw)+"\n```"), 0600); err != nil {
		t.Fatal(err)
	}
	args := append(decisionRecordArgs(project, "review-policy", "supplement"), "--body-file", body, "--user-confirmed")
	if code, _, msg := decisionInvoke(t, args...); code != 0 {
		t.Fatal(code, msg)
	}
	if approved {
		if code, _, msg := decisionInvoke(t, append(decisionRecordArgs(project, "execution", "execution_approved"), "--user-confirmed")...); code != 0 {
			t.Fatal(code, msg)
		}
	}
	now := time.Now().UTC()
	for _, actor := range []struct{ attempt, native, path, assignment string }{{"implementation-run", p.Implementation.NativeID, p.Implementation.AgentPath, "implementation-assignment"}, {"review-run", p.Reviewer.NativeID, p.Reviewer.AgentPath, "review-assignment"}} {
		if _, err := runtimeobs.RecordAssignment(project, actor.assignment, "B-1", actor.path, now); err != nil {
			t.Fatal(err)
		}
		if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: actor.attempt, Provider: "codex", SessionID: actor.native, RuntimeAgentID: actor.native, State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
			t.Fatal(err)
		}
		if _, err := runtimeobs.BindAttempt(project, actor.attempt, "B-1", actor.path, "dispatch", "", map[string]string{"assignment_id": actor.assignment}, now); err != nil {
			t.Fatal(err)
		}
	}
	return project, source, p
}

func a90RepairActivityArgs(project, action, id, kind string) []string {
	return []string{action, "B-1", "--project", project, "--role", "reviewer", "--agent", "review-native", "--attempt", "review-run", "--assignment", "review-assignment", "--type", kind, "--event-id", id, "--evidence", "isolated actual CLI callback fixture", "--json"}
}

func TestA90RepairReviewerCLIStartSwitchStop(t *testing.T) {
	project, source, _ := a90RepairReviewerFixture(t, true)
	for _, phase := range []struct{ action, id, kind string }{{"start", "review-start", "controller_review"}, {"switch", "review-test", "targeted_test"}, {"stop", "review-stop", "targeted_test"}} {
		args := a90RepairActivityArgs(project, phase.action, phase.id, phase.kind)
		code, response, msg := invokeActivity(t, args...)
		if code != 0 || response["recorded"] != true {
			t.Fatalf("actual CLI %s: exit=%d stdout=%v stderr=%s", phase.action, code, response, msg)
		}
		if code, response, msg = invokeActivity(t, args...); code != 0 || response["recorded"] != false {
			t.Fatal("verified retry", code, response, msg)
		}
	}
	rows, err := (activity.Ledger{Project: project}).List("B-1")
	if err != nil || len(rows) != 3 {
		t.Fatal(rows, err)
	}
	for _, row := range rows {
		if row.Role != activity.RoleReviewer || row.AgentID != "review-native" || row.RuntimeAttemptID != "review-run" || row.AssignmentID != "review-assignment" {
			t.Fatal("recorded identity changed", row)
		}
	}
	if _, err := os.Stat(filepath.Join(source, "source.go")); err != nil {
		t.Fatal(err)
	}
}

func TestA90RepairReviewerCLIRejectsUnverifiedAndImplementation(t *testing.T) {
	for _, name := range []string{"implementation", "worker-disguise", "wrong-native", "wrong-attempt", "wrong-task", "wrong-assignment", "wrong-designated-native", "no-execution-gate", "unprepared-source", "terminal"} {
		t.Run(name, func(t *testing.T) {
			project, source, p := a90RepairReviewerFixture(t, name != "no-execution-gate")
			args := a90RepairActivityArgs(project, "start", "rejected", "controller_review")
			set := func(flag, value string) {
				for i := range args {
					if args[i] == flag {
						args[i+1] = value
						return
					}
				}
				t.Fatal(flag)
			}
			switch name {
			case "implementation":
				set("--type", "implementation")
			case "worker-disguise":
				set("--role", "worker")
			case "wrong-native":
				set("--agent", "foreign-native")
			case "wrong-attempt":
				set("--attempt", "implementation-run")
			case "wrong-task":
				args[1] = "B-2"
			case "wrong-assignment":
				set("--assignment", "implementation-assignment")
			case "wrong-designated-native":
				p.Reviewer.NativeID = "different-designated-native"
				raw, _ := json.Marshal(p)
				body := filepath.Join(t.TempDir(), "new-policy.md")
				if err := os.WriteFile(body, []byte(rolepolicy.Fence+string(raw)+"\n```"), 0600); err != nil {
					t.Fatal(err)
				}
				if code, _, msg := decisionInvoke(t, append(decisionRecordArgs(project, "changed-owner", "supplement"), "--body-file", body, "--user-confirmed")...); code != 0 {
					t.Fatal(code, msg)
				}
			case "unprepared-source":
				if err := os.Remove(filepath.Join(source, "source.go")); err != nil {
					t.Fatal(err)
				}
			case "terminal":
				if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), AttemptID: "review-run", Provider: "codex", SessionID: "review-native", RuntimeAgentID: "review-native", State: runtimeobs.StateCompleted, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
					t.Fatal(err)
				}
			}
			if code, _, msg := invokeActivity(t, args...); code == 0 {
				t.Fatal("invalid CLI actor/gate recorded", name, msg)
			}
			rows, err := (activity.Ledger{Project: project}).List("B-1")
			if err != nil || len(rows) != 0 {
				t.Fatal("rejected callback appended Activity", rows, err)
			}
		})
	}
}

func TestA90RepairReviewerCLIRechecksSourceOnStop(t *testing.T) {
	project, source, _ := a90RepairReviewerFixture(t, true)
	if code, _, msg := invokeActivity(t, a90RepairActivityArgs(project, "start", "begin", "controller_review")...); code != 0 {
		t.Fatal(code, msg)
	}
	if err := os.Remove(filepath.Join(source, "source.go")); err != nil {
		t.Fatal(err)
	}
	if code, _, msg := invokeActivity(t, a90RepairActivityArgs(project, "stop", "end", "controller_review")...); code == 0 || !strings.Contains(msg, "source") {
		t.Fatal("stale source accepted", code, msg)
	}
	rows, err := (activity.Ledger{Project: project}).List("B-1")
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
}

func TestA90RepairReviewerCLIRechecksExecutionGateOnStop(t *testing.T) {
	project, _, _ := a90RepairReviewerFixture(t, true)
	if code, _, msg := invokeActivity(t, a90RepairActivityArgs(project, "start", "begin", "controller_review")...); code != 0 {
		t.Fatal(code, msg)
	}
	if code, _, msg := decisionInvoke(t, append(decisionRecordArgs(project, "pending-after-start", "policy_pending"), "--policy-key", "review-scope")...); code != 0 {
		t.Fatal(code, msg)
	}
	if code, _, msg := invokeActivity(t, a90RepairActivityArgs(project, "stop", "end", "controller_review")...); code == 0 || !strings.Contains(msg, "authorization") {
		t.Fatal("revoked gate accepted", code, msg)
	}
	rows, err := (activity.Ledger{Project: project}).List("B-1")
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
}

func TestA90ReviewDecisionCLIHasNoDispatchOrToolClaim(t *testing.T) {
	p := filepath.Join(t.TempDir(), "risk.json")
	r := rolepolicy.ReviewRequirement{Scope: []string{"product.go"}, Rationale: "verified reversible local correction", Risk: rolepolicy.ReviewRisk{Impact: "low", Complexity: "simple", Reversibility: "easy", Verifiability: "established"}}
	raw, _ := json.Marshal(r)
	os.WriteFile(p, raw, 0600)
	var out, stderr bytes.Buffer
	if code := runRolePolicyCLI([]string{"review-decision", "--input", p, "--json"}, &out, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	var d map[string]any
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d["dispatch_authorized"] != false || d["external_tool_performed"] != false {
		t.Fatal(d)
	}
	if runtimeQueryRole("/root/controller/reviewer/raichyu") != "reviewer" || runtimeQueryRole("/root/controller/raichyu") != "worker" {
		t.Fatal("query role distinction")
	}
	r.Risk.ExplicitRequired = true
	r.Omit = true
	r.OmissionReason = "speed"
	raw, _ = json.Marshal(r)
	os.WriteFile(p, raw, 0600)
	out.Reset()
	stderr.Reset()
	if code := runRolePolicyCLI([]string{"review-decision", "--input", p}, &out, &stderr); code != 3 {
		t.Fatal(code, stderr.String())
	}
}
