package handoff

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func repairAppliedIntake(t *testing.T) (PrepareRequest, ExecutionContext, time.Time, Handoff) {
	t.Helper()
	req, c, now := a79ContextFixture(t)
	id := c.Targets[0].OriginalHandoffID
	claimed, e := Claim(req.Project, id, "/root/controller", req.TargetAttemptID, now)
	if e != nil || !claimed.Claimed {
		t.Fatal(claimed, e)
	}
	if _, e = MarkAs(req.Project, id, "applied", "ok", "registration intake only; not task completion", req.TargetAttemptID, now); e != nil {
		t.Fatal(e)
	}
	original, _, e := Inspect(req.Project, id, now)
	if e != nil {
		t.Fatal(e)
	}
	appendA79ControllerAttempt(t, req.Project, req.TargetAttemptID, runtimeobs.StateCompleted, now.Add(time.Second))
	appendA79ControllerAttempt(t, req.Project, "controller-next", runtimeobs.StateRunning, now.Add(2*time.Second))
	c.ControllerAttemptID = "controller-next"
	req.TargetAttemptID = "controller-next"
	writeA79Context(t, req.ContextFile, c)
	return req, c, now.Add(3 * time.Second), original
}

func TestA79RepairAppliedIntakeLineageDistinctCompletionAndEpisodes(t *testing.T) {
	req, c, now, original := repairAppliedIntake(t)
	before, _ := json.Marshal(original)
	var stdout, stderr bytes.Buffer
	code := RunCLI([]string{"context", original.HandoffID, "--project", req.Project, "--target-attempt", req.TargetAttemptID, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatal(code, stdout.String(), stderr.String())
	}
	var context ContextCheck
	if e := json.Unmarshal(stdout.Bytes(), &context); e != nil {
		t.Fatal(e)
	}
	if context.RegistrationLineage == nil || context.RegistrationLineage.Mode != "applied_registration_same_native_history" || !context.RegistrationLineage.HistoricalTargetMismatch || context.RegistrationLineage.OriginalTargetAttemptID != original.Target.AttemptID || context.Status != "collection_gap" {
		t.Fatal(context)
	}
	completion, e := Prepare(req, now)
	if e != nil || completion.HandoffID == original.HandoffID {
		t.Fatal(completion, e)
	}
	// Preparing the existing registration is still an immutable duplicate, never
	// a fake new intake nor a transfer of the already-applied receipt.
	dup, e := Prepare(PrepareRequest{Project: req.Project, TaskID: req.TaskID, EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAttemptID: req.TargetAttemptID, ExecutionAuthorized: true}, now)
	if e != nil || !dup.Duplicate || dup.HandoffID != original.HandoffID || dup.Target.AttemptID != original.Target.AttemptID {
		t.Fatal(dup, e)
	}
	appendA79ControllerAttempt(t, req.Project, req.TargetAttemptID, runtimeobs.StateCompleted, now.Add(time.Second))
	appendA79ControllerAttempt(t, req.Project, "controller-third", runtimeobs.StateRunning, now.Add(2*time.Second))
	if _, e = CheckContext(req.Project, req.TaskID, req.TargetAttemptID, c, true, now.Add(3*time.Second)); e == nil {
		t.Fatal("stale second episode accepted")
	}
	c.ControllerAttemptID = "controller-third"
	req.TargetAttemptID = c.ControllerAttemptID
	writeA79Context(t, req.ContextFile, c)
	retarget, e := Prepare(req, now.Add(3*time.Second))
	if e != nil || !retarget.Duplicate || retarget.HandoffID != completion.HandoffID {
		t.Fatal(retarget, e)
	}
	claimed, e := Claim(req.Project, completion.HandoffID, "/root/controller", req.TargetAttemptID, now.Add(4*time.Second))
	if e != nil || !claimed.Claimed {
		t.Fatal(claimed, e)
	}
	checked, e := StartContext(req.Project, completion.HandoffID, req.TargetAttemptID, now.Add(4*time.Second))
	if e != nil || checked.ReportedControllerAttemptID != "controller-next" || checked.RegistrationLineage.OriginalClaimedAttemptID != original.ClaimedAttemptID {
		t.Fatal(checked, e)
	}
	for _, step := range []string{"acceptance", "applied", "applied"} {
		if _, e = MarkAs(req.Project, completion.HandoffID, step, "ok", "verified exact distinct completion", req.TargetAttemptID, now.Add(4*time.Second)); e != nil {
			t.Fatal(e)
		}
	}
	after, _, e := Inspect(req.Project, original.HandoffID, now.Add(5*time.Second))
	b, _ := json.Marshal(after)
	if e != nil || !bytes.Equal(before, b) {
		t.Fatal("applied registration rewritten", after, e)
	}
}

func TestA79RepairLineageRejectsForeignOrIncompleteEvidence(t *testing.T) {
	for _, kind := range []string{"foreign_native", "foreign_provider", "missing_claim", "unapplied", "foreign_original_target", "old_unobserved", "wrong_task", "changed_contract", "wrong_assignment"} {
		t.Run(kind, func(t *testing.T) {
			req, c, now, original := repairAppliedIntake(t)
			current, e := currentController(req.Project, req.TargetAttemptID, now)
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "foreign_native":
				current.SessionID = "foreign-native"
				current.RuntimeAgentID = "foreign-native"
			case "foreign_provider":
				current.Provider = "claude"
			case "missing_claim":
				original.ClaimedAttemptID = ""
			case "unapplied":
				original.Applied = false
			case "foreign_original_target":
				original.Target.RuntimeAgentID = "foreign-native"
			case "old_unobserved":
				original.ClaimedAttemptID = "unobserved-history"
				original.Target.AttemptID = "unobserved-history"
				for _, event := range []runtimeobs.ExecutionEvent{
					{EventKind: "state", ObservedAt: now.Add(-time.Hour).Format(time.RFC3339Nano), AttemptID: "unobserved-history", Provider: "codex", SessionID: "controller-native", RuntimeAgentID: "controller-native", State: runtimeobs.StateCompleted, Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityInferred},
					{EventKind: "binding", ObservedAt: now.Add(-time.Hour).Add(time.Second).Format(time.RFC3339Nano), AttemptID: "unobserved-history", AgentPath: "/root/controller", BindingSource: "test", EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative},
				} {
					if e = runtimeobs.AppendExecutionEvent(req.Project, event); e != nil {
						t.Fatal(e)
					}
				}
			case "wrong_task":
				c.Targets[0].OriginalHandoffID = c.Targets[1].OriginalHandoffID
			case "changed_contract":
				c.Targets[0].ContractSHA256 = strings.Repeat("0", 64)
			case "wrong_assignment":
				c.Targets[0].AssignmentID = c.Targets[1].AssignmentID
			}
			if kind == "wrong_task" || kind == "changed_contract" || kind == "wrong_assignment" {
				if _, e = CheckContext(req.Project, req.TaskID, req.TargetAttemptID, c, true, now); e == nil {
					t.Fatal("foreign task/contract/assignment accepted")
				}
				return
			}
			if proof, e := registrationLineage(req.Project, original, current, now); e == nil || proof != nil {
				t.Fatal("invalid applied intake lineage accepted", proof, e)
			}
		})
	}
}

func TestA79RepairActuallyObservedForeignControllerCannotInheritAppliedIntake(t *testing.T) {
	req, c, now, _ := repairAppliedIntake(t)
	appendA79ControllerAttempt(t, req.Project, req.TargetAttemptID, runtimeobs.StateCompleted, now)
	for _, event := range []runtimeobs.ExecutionEvent{
		{EventKind: "state", ObservedAt: now.Add(time.Second).Format(time.RFC3339Nano), AttemptID: "foreign-current", Provider: "codex", SessionID: "foreign-native", RuntimeAgentID: "foreign-native", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved},
		{EventKind: "binding", ObservedAt: now.Add(2 * time.Second).Format(time.RFC3339Nano), AttemptID: "foreign-current", AgentPath: "/root/controller", BindingSource: "test", EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative},
	} {
		if e := runtimeobs.AppendExecutionEvent(req.Project, event); e != nil {
			t.Fatal(e)
		}
	}
	c.ControllerAttemptID = "foreign-current"
	if _, e := currentController(req.Project, c.ControllerAttemptID, now.Add(3*time.Second)); e != nil {
		t.Fatal("fixture must verify actual foreign current before lineage refusal", e)
	}
	if _, e := CheckContext(req.Project, req.TaskID, c.ControllerAttemptID, c, true, now.Add(3*time.Second)); e == nil || !strings.Contains(e.Error(), "original_handoff_current_attempt_mismatch") {
		t.Fatal("foreign current inherited intake", e)
	}
}

func appendRepairActor(t *testing.T, project, task, path, assignment, attempt, native string, state runtimeobs.CanonicalState, at time.Time) {
	t.Helper()
	if _, e := runtimeobs.FindAssignment(project, assignment); e != nil {
		if _, e := runtimeobs.RecordAssignment(project, assignment, task, path, at); e != nil {
			t.Fatal(e)
		}
	}
	for _, event := range []runtimeobs.ExecutionEvent{
		{EventKind: "state", ObservedAt: at.Format(time.RFC3339Nano), AttemptID: attempt, Provider: "codex", SessionID: native, RuntimeAgentID: native, State: state, Terminal: state == runtimeobs.StateCompleted, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved},
		{EventKind: "binding", ObservedAt: at.Add(time.Millisecond).Format(time.RFC3339Nano), AttemptID: attempt, TaskID: task, AgentPath: path, BindingSource: "dispatch", BindingEvidence: map[string]string{"assignment_id": assignment}, EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative},
	} {
		if e := runtimeobs.AppendExecutionEvent(project, event); e != nil {
			t.Fatal(e)
		}
	}
}

func TestA79RepairReviewerCannotReplaceActualImplementationPhase(t *testing.T) {
	req, c, now := a79ContextFixture(t)
	original := c.Targets[0].OriginalHandoffID
	_, e := (activity.Ledger{Project: req.Project}).Append(activity.Event{SchemaVersion: 1, EventID: "implementation-phase", TaskID: req.TaskID, Role: activity.RoleWorker, AgentID: req.SourceAttemptID + "-native", RuntimeAttemptID: req.SourceAttemptID, AssignmentID: c.Targets[0].AssignmentID, Action: activity.Start, Type: activity.Implementation, PhaseEvent: "implementation_started", Evidence: "actual temp-fixture implementation phase", Confidence: activity.Inferred, OccurredAt: now.Add(-time.Minute), RecordedAt: now.Add(-time.Minute)})
	if e != nil {
		t.Fatal(e)
	}
	for i, state := range []runtimeobs.CanonicalState{runtimeobs.StateRunning, runtimeobs.StateCompleted} {
		appendRepairActor(t, req.Project, req.TaskID, "/root/controller/arbitrary_review_owner", "review-assignment", "review-attempt", "review-native", state, now.Add(time.Duration(i)*time.Second))
		got, e := StartContext(req.Project, original, req.TargetAttemptID, now.Add(3*time.Second))
		if e != nil {
			t.Fatal(e)
		}
		if got.Target.AssignmentID != c.Targets[0].AssignmentID || got.Target.SourceAttemptID != req.SourceAttemptID || got.Target.Phase != "implementation_started" || got.Status != "collection_gap" || got.WorkerRuntime.State != runtimeobs.StateCompleted {
			t.Fatal("reviewer overwrote implementer", got)
		}
		if len(got.OtherActors) != 1 || got.OtherActors[0].Runtime.State != state {
			t.Fatal("other actor observation lost", got)
		}
	}
}

func TestA79RepairOwnerPolicyReassignmentAndUnknownLegacy(t *testing.T) {
	for _, kind := range []string{"unknown_legacy", "multiple_legacy", "explicit_reassignment", "policy_owner", "policy_native_mismatch"} {
		t.Run(kind, func(t *testing.T) {
			req, c, now := a79ContextFixture(t)
			file := filepath.Join(req.Project, "_task_mecca/data/backlog/000001."+req.TaskID+".handoff-test.todo.md")
			raw, e := os.ReadFile(file)
			if e != nil {
				t.Fatal(e)
			}
			path := "/root/controller/pairi"
			switch kind {
			case "unknown_legacy":
				path = "-"
			case "multiple_legacy":
				path = "/root/controller/pairi, /root/controller/another"
			case "explicit_reassignment":
				path = "/root/controller/replacement"
			case "policy_owner", "policy_native_mismatch":
				p := rolepolicy.Policy{SchemaVersion: 1, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/pairi", NativeID: req.SourceAttemptID + "-native"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Reviewer: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/arbitrary_review_owner"}, Method: "worker", Workspace: req.SourceWorkspace, Scope: []string{"source.go"}}
				if kind == "policy_native_mismatch" {
					p.Implementation.NativeID = "foreign-native"
				}
				b, _ := json.Marshal(p)
				raw = []byte(strings.Replace(string(raw), "### 수용 기준", rolepolicy.Fence+string(b)+"\n```\n\n### 수용 기준", 1))
				path = "/root/controller/arbitrary_review_owner"
			}
			raw = []byte(strings.Replace(string(raw), "- Agent: /root/controller/pairi", "- Agent: "+path, 1))
			if e = os.WriteFile(file, raw, 0600); e != nil {
				t.Fatal(e)
			}
			if kind == "explicit_reassignment" {
				appendRepairActor(t, req.Project, req.TaskID, path, "replacement-assignment", "replacement-attempt", "replacement-native", runtimeobs.StateRunning, now)
			}
			// Policy source changed only in policy cases; use a legitimate matching
			// registration fixture for that new contract, not a stale receipt bypass.
			if strings.HasPrefix(kind, "policy_") {
				h, e := Prepare(PrepareRequest{Project: req.Project, TaskID: req.TaskID, EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAttemptID: req.TargetAttemptID, ExecutionAuthorized: true}, now)
				if e != nil {
					t.Fatal(e)
				}
				c.Targets[0].OriginalHandoffID = h.HandoffID
			}
			got, e := StartContext(req.Project, c.Targets[0].OriginalHandoffID, req.TargetAttemptID, now.Add(2*time.Second))
			if kind == "unknown_legacy" || kind == "multiple_legacy" || kind == "policy_native_mismatch" {
				if e == nil || got.Status != "collection_gap" {
					t.Fatal("unverified owner guessed", got, e)
				}
				return
			}
			if e != nil {
				t.Fatal(got, e)
			}
			expected := req.SourceAttemptID
			if kind == "explicit_reassignment" {
				expected = "replacement-attempt"
			}
			if got.Target.SourceAttemptID != expected {
				t.Fatal("wrong verified owner", got)
			}
		})
	}
}
