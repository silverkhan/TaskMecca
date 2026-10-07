package webui

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/handoff"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func policyFixture(t *testing.T, state, source string, bound bool) completionFixture {
	t.Helper()
	f := completionFixture{project: testOperationProject(t), base: time.Date(2026, 10, 7, 7, 0, 0, 0, time.UTC), assignmentID: "assignment-policy"}
	_, err := runtimeobs.RecordAssignment(f.project, f.assignmentID, "A-5", "/root/controller/worker", f.base)
	if err != nil {
		t.Fatal(err)
	}
	if bound {
		a, err := runtimeobs.BindRuntimeAgentForAssignment(f.project, f.assignmentID, "worker", "", f.base.Add(time.Second))
		if err != nil {
			t.Fatal(err)
		}
		f.attemptID = a.AttemptID
	}
	j, err := scanOperationProject(f.project, f.base.Add(3*time.Minute))
	if err != nil || len(j.Incidents) != 1 {
		t.Fatalf("initial=%+v %v", j, err)
	}
	f.incidentID = j.Incidents[0].ID
	f.taskPath = filepath.Join(f.project, "_task_mecca", "data", "backlog", "000005.A-5.policy."+state+".md")
	if err := os.MkdirAll(filepath.Dir(f.taskPath), 0755); err != nil {
		t.Fatal(err)
	}
	body := "# A-5 Policy\n- Agent: /root/controller/worker\n## 결과\nController verified result\n## 검증\nRegression verified\n"
	kind := "completed"
	if state == "hold" {
		body = "# A-5 Policy\n- Agent: -\n- 변경범위: -\n- 대기유형: user\n- 대기: explicit user decision pending\n- 재개조건: user selects policy\n- 대기근거: Controller confirmed claim release\n"
		kind = "waiting"
	}
	if err := os.WriteFile(f.taskPath, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	event := backlog.LifecycleTransition{EventID: "policy-boundary", TaskID: "A-5", Kind: kind, Actor: "/root/controller", EvidenceSource: source, EvidenceRef: "Controller reviewed immutable evidence", OccurredAt: f.base.Add(10 * time.Minute).Format(time.RFC3339Nano)}
	if state == "hold" {
		event.AssignmentID, event.AttemptID = f.assignmentID, f.attemptID
	}
	if _, err := backlog.RecordLifecycleTransition(f.project, event, f.base.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestPolicyRetiresLegacyAssignmentsAndReleasedHoldsAcrossScans(t *testing.T) {
	for _, tc := range []struct {
		state, source string
		bound         bool
	}{{"done", "controller_report", false}, {"done", "controller_report", true}, {"done", "controller_verified", false}, {"hold", "worker_report", true}} {
		t.Run(tc.state+tc.source+map[bool]string{true: "bound", false: "unbound"}[tc.bound], func(t *testing.T) {
			f := policyFixture(t, tc.state, tc.source, tc.bound)
			now := f.base.Add(10*time.Minute + time.Second)
			before, _ := runtimeobs.BuildLedger(f.project, 3, now)
			taskBefore, _ := os.ReadFile(f.taskPath)
			j, err := scanOperationProject(f.project, now)
			if err != nil || j.Incidents[0].RecoveredAt == "" || j.Incidents[0].PolicyResolution == nil {
				t.Fatalf("not retired=%+v %v", j, err)
			}
			if j.Incidents[0].Resolution != nil || j.Incidents[0].EndedAt != "" {
				t.Fatal("manufactured completion")
			}
			if len(j.Stages) != 0 {
				t.Fatalf("retired assignment still current=%+v", j.Stages)
			}
			sequence := j.NextSequence
			for n := 2; n < 5; n++ {
				j, err = scanOperationProject(f.project, f.base.Add(10*time.Minute+time.Duration(n)*time.Second))
				if err != nil || j.NextSequence != sequence {
					t.Fatalf("recurrence=%+v %v", j, err)
				}
			}
			after, _ := runtimeobs.BuildLedger(f.project, 3, now)
			x, _ := json.Marshal(before.Attempts)
			y, _ := json.Marshal(after.Attempts)
			taskAfter, _ := os.ReadFile(f.taskPath)
			if !bytes.Equal(x, y) || !bytes.Equal(taskBefore, taskAfter) {
				t.Fatal("policy changed canonical/runtime facts")
			}
		})
	}
}

func TestPolicyTaskWideCompletionIncludesParallelLegacyAssignments(t *testing.T) {
	f := policyFixture(t, "done", "controller_report", false)
	for n, agent := range []string{"/root/controller/worker/layout_read", "/root/controller/worker/layout_scan"} {
		id := []string{"child-read", "child-scan"}[n]
		if _, err := runtimeobs.RecordAssignment(f.project, id, "A-5", agent, f.base.Add(time.Duration(n+1)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	ledger, _ := runtimeobs.BuildLedger(f.project, 3, f.base.Add(11*time.Minute))
	policies := operationPolicies(f.project, ledger, f.base.Add(11*time.Minute))
	if len(policies["A-5"].assignments) != 3 {
		t.Fatalf("parallel assignments=%+v", policies)
	}
	j, err := scanOperationProject(f.project, f.base.Add(11*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range j.Incidents {
		if i.RecoveredAt == "" {
			t.Fatalf("active=%+v", i)
		}
	}
}

func TestPolicyFailsClosedForLateActivityErrorsAndCanonicalAmbiguity(t *testing.T) {
	for _, change := range []string{"late_activity", "late_binding", "errored", "interrupted", "shutdown", "unconfirmed_terminal", "new_assignment", "reopened", "duplicate", "missing", "wrong_actor", "wrong_source", "placeholder", "future", "wrong_assignment", "wrong_attempt", "active_not_done"} {
		t.Run(change, func(t *testing.T) {
			f := policyFixture(t, "done", "controller_report", true)
			now := f.base.Add(11 * time.Minute)
			event := backlog.LifecycleTransition{EventID: "new-policy", TaskID: "A-5", Kind: "completed", Actor: "/root/controller", EvidenceSource: "controller_report", EvidenceRef: "verified evidence", OccurredAt: f.base.Add(10*time.Minute + time.Second).Format(time.RFC3339Nano)}
			switch change {
			case "late_activity", "errored", "interrupted", "shutdown", "unconfirmed_terminal":
				e := runtimeobs.ExecutionEvent{EventKind: "activity", AttemptID: f.attemptID, ObservedAt: f.base.Add(10*time.Minute + time.Second).Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}
				if change != "late_activity" {
					e.EventKind = "state"
					e.State = runtimeobs.CanonicalState(change)
					e.Terminal = true
					e.ObservedAt = f.base.Add(9 * time.Minute).Format(time.RFC3339Nano)
				}
				if change == "unconfirmed_terminal" {
					e.State = runtimeobs.StateErrored
					e.EvidenceSource = runtimeobs.EvidenceReconciled
					e.ObservationQuality = runtimeobs.QualityInferred
				}
				if err := runtimeobs.AppendExecutionEvent(f.project, e); err != nil {
					t.Fatal(err)
				}
			case "late_binding":
				if _, err := runtimeobs.BindRuntimeAgentForAssignment(f.project, f.assignmentID, "late-worker", "", f.base.Add(10*time.Minute+time.Second)); err != nil {
					t.Fatal(err)
				}
			case "new_assignment":
				if _, err := runtimeobs.RecordAssignment(f.project, "later-assignment", "A-5", "/root/controller/other", f.base.Add(10*time.Minute+time.Second)); err != nil {
					t.Fatal(err)
				}
			case "duplicate":
				if err := os.WriteFile(filepath.Join(filepath.Dir(f.taskPath), "000005.A-5.duplicate.done.md"), []byte("# A-5 Duplicate\n"), 0644); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(f.taskPath); err != nil {
					t.Fatal(err)
				}
			case "active_not_done":
				if err := os.Rename(f.taskPath, filepath.Join(filepath.Dir(f.taskPath), "000005.A-5.active.doing.md")); err != nil {
					t.Fatal(err)
				}
			default:
				switch change {
				case "reopened":
					event.Kind = "resumed"
				case "wrong_actor":
					event.Actor = "/root/controller/worker"
				case "wrong_source":
					event.EvidenceSource = "worker_report"
				case "placeholder":
					event.EvidenceRef = "-"
				case "future":
					event.OccurredAt = now.Add(time.Minute).Format(time.RFC3339Nano)
				case "wrong_assignment":
					event.AssignmentID = "unmatched"
				case "wrong_attempt":
					event.AttemptID = "unmatched"
				}
				recorded, _ := time.Parse(time.RFC3339Nano, event.OccurredAt)
				if _, err := backlog.RecordLifecycleTransition(f.project, event, recorded); err != nil {
					t.Fatal(err)
				}
			}
			ledger, err := runtimeobs.BuildLedger(f.project, 3, now)
			if err != nil {
				t.Fatal(err)
			}
			if len(operationPolicies(f.project, ledger, now)) != 0 {
				t.Fatal("unsafe policy accepted")
			}
		})
	}
}

func TestPolicyReleasedExternalAndDependencyHold(t *testing.T) {
	for _, kind := range []string{"external", "dependency"} {
		t.Run(kind, func(t *testing.T) {
			f := policyFixture(t, "hold", "controller_report", true)
			body, err := os.ReadFile(f.taskPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(f.taskPath, []byte(strings.ReplaceAll(string(body), "대기유형: user", "대기유형: "+kind)), 0644); err != nil {
				t.Fatal(err)
			}
			ledger, _ := runtimeobs.BuildLedger(f.project, 3, f.base.Add(11*time.Minute))
			if operationPolicies(f.project, ledger, f.base.Add(11*time.Minute))["A-5"].proof.Reason != "canonical_released_"+kind+"_hold" {
				t.Fatal("explicit released hold not recognized")
			}
		})
	}
}

func TestPolicySameAttemptReportingClosureIsNotNewExecution(t *testing.T) {
	for _, state := range []string{"hold", "done"} {
		t.Run(state, func(t *testing.T) {
			f := policyFixture(t, state, "controller_report", true)
			if err := runtimeobs.AppendExecutionEvent(f.project, runtimeobs.ExecutionEvent{EventKind: "state", State: runtimeobs.StateRunning, AttemptID: f.attemptID, ObservedAt: f.base.Add(2 * time.Second).Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
				t.Fatal(err)
			}
			if state == "done" {
				at := f.base.Add(10*time.Minute + time.Second)
				_, err := backlog.RecordLifecycleTransition(f.project, backlog.LifecycleTransition{EventID: "exact-completed", TaskID: "A-5", Kind: "completed", Actor: "/root/controller", EvidenceSource: "controller_verified", EvidenceRef: "verified report", AssignmentID: f.assignmentID, AttemptID: f.attemptID, OccurredAt: at.Format(time.RFC3339Nano)}, at)
				if err != nil {
					t.Fatal(err)
				}
			}
			at := f.base.Add(10*time.Minute + 2*time.Second)
			if err := runtimeobs.AppendExecutionEvent(f.project, runtimeobs.ExecutionEvent{EventKind: "activity", AttemptID: f.attemptID, ObservedAt: at.Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
				t.Fatal(err)
			}
			ledger, _ := runtimeobs.BuildLedger(f.project, 3, at)
			if len(operationPolicies(f.project, ledger, at)) != 0 {
				t.Fatal("still-running late activity concealed")
			}
			at = at.Add(time.Second)
			if err := runtimeobs.AppendExecutionEvent(f.project, runtimeobs.ExecutionEvent{EventKind: "state", State: runtimeobs.StateCompleted, Terminal: true, AttemptID: f.attemptID, ObservedAt: at.Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
				t.Fatal(err)
			}
			ledger, _ = runtimeobs.BuildLedger(f.project, 3, at)
			if len(operationPolicies(f.project, ledger, at)) != 1 {
				t.Fatal("actual same-attempt reporting closure not recognized")
			}
			// Even a completed attempt must not hide post-terminal activity.
			at = at.Add(time.Second)
			if err := runtimeobs.AppendExecutionEvent(f.project, runtimeobs.ExecutionEvent{EventKind: "activity", AttemptID: f.attemptID, ObservedAt: at.Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
				t.Fatal(err)
			}
			ledger, _ = runtimeobs.BuildLedger(f.project, 3, at)
			if len(operationPolicies(f.project, ledger, at)) != 0 {
				t.Fatal("post-terminal activity concealed")
			}
		})
	}
}

func TestPolicyKeepsHandoffFailureSeparateFromRetiredAssignment(t *testing.T) {
	f := policyFixture(t, "done", "controller_report", true)
	stamp := func(n int) string { return f.base.Add(time.Duration(n) * time.Minute).Format(time.RFC3339Nano) }
	appendSyntheticStageRecord(t, f.project, handoff.Record{HandoffID: "policy-handoff", RecordKind: "prepared", EventType: handoff.EventWorkerDone, TaskID: "A-5", SourceAgentPath: "/root/controller/worker", SourceAttemptID: f.attemptID, TargetAgentPath: "/root/controller", ObservedAt: stamp(9)})
	appendSyntheticStageRecord(t, f.project, handoff.Record{HandoffID: "policy-handoff", RecordKind: "claimed", ClaimedBy: "/root/controller", ClaimedAttemptID: "controller-run", ObservedAt: stamp(11)})
	appendSyntheticStageRecord(t, f.project, handoff.Record{HandoffID: "policy-handoff", RecordKind: "step_succeeded", Step: "acceptance", Result: "ok", ObservedAt: stamp(12)})
	appendSyntheticStageRecord(t, f.project, handoff.Record{HandoffID: "policy-handoff", RecordKind: "applied", ObservedAt: stamp(13)})
	j, err := scanOperationProject(f.project, f.base.Add(14*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range j.Incidents {
		if i.RecoveredAt == "" {
			t.Fatalf("successful bookkeeping re-alerted=%+v", i)
		}
	}
	appendSyntheticStageRecord(t, f.project, handoff.Record{HandoffID: "policy-handoff", RecordKind: "step_failed", Step: "external", Result: "failed", ObservedAt: stamp(15)})
	j, err = scanOperationProject(f.project, f.base.Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, i := range j.Incidents {
		if i.Kind == "handoff_failed" && i.RecoveredAt == "" {
			found = true
		}
	}
	if !found {
		t.Fatal("real handoff failure concealed by completion")
	}
}

func TestPolicyAssignmentOnlyRetirementDoesNotHideUnrelatedLegacyError(t *testing.T) {
	f := policyFixture(t, "done", "controller_report", false)
	hook := testOperationHook(t, f.project, `{"session_id":"legacy","turn_id":"legacy-turn","hook_event_name":"SubagentStart","agent_id":"legacy-worker"}`, f.base.Add(time.Minute))
	if _, err := runtimeobs.BindAttempt(f.project, hook.AttemptID, "A-5", "/root/controller/legacy", "explicit", "", nil, f.base.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := runtimeobs.AppendExecutionEvent(f.project, runtimeobs.ExecutionEvent{EventKind: "state", State: runtimeobs.StateErrored, Terminal: true, AttemptID: hook.AttemptID, ObservedAt: f.base.Add(3 * time.Minute).Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
		t.Fatal(err)
	}
	j, err := scanOperationProject(f.project, f.base.Add(11*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(j.Stages) != 0 {
		t.Fatalf("unrelated legacy attempt reconstructed retired stage=%+v", j.Stages)
	}
	found := false
	for _, i := range j.Incidents {
		if i.Kind == "errored" && i.AttemptID == hook.AttemptID && i.RecoveredAt == "" {
			found = true
		}
	}
	if !found {
		t.Fatal("unrelated legacy failure concealed")
	}
}

// Opt-in diagnostic: only read canonical/lifecycle/assignment/runtime/handoff
// evidence. Never invoke monitor scans, reconciliation, registration or writes
// against these operational projects; output contains metadata, not payloads.
func TestA28ReadOnlyActualPolicyCases(t *testing.T) {
	if os.Getenv("TASK_MECCA_A28_READ_ONLY_CASES") != "1" {
		t.Skip("explicit read-only diagnostic only")
	}
	now := time.Now().UTC()
	for project, ids := range map[string][]string{
		"/Users/hanati/projects/TaskMecca/TaskMecca": {"A-7", "A-8", "A-10", "A-11", "A-19", "A-14", "A-26", "A-27"},
		"/Users/hanati/projects/empfund":             {"B-453", "B-456"},
	} {
		ledger, err := runtimeobs.BuildLedger(project, 3, now)
		if err != nil {
			t.Fatal(err)
		}
		policies := operationPolicies(project, ledger, now)
		for _, id := range ids {
			p, ok := policies[id]
			t.Logf("task=%s accepted=%t reason=%s event=%s assignments=%d attempts=%d", id, ok, p.proof.Reason, p.proof.EventID, len(p.assignments), len(p.attempts))
			if !ok {
				rows, _ := operationCanonicalRows(project)
				for _, row := range rows[id] {
					t.Logf("canonical task=%s state=%s location=%s result=%t verification=%t release=%t wait=%s", id, row.State, row.Location, operationMeaningfulEvidence(row.Fields["결과"]), operationMeaningfulEvidence(row.Fields["검증"]), operationReleasedClaim(row), row.Fields["대기유형"])
				}
				scan, _ := backlog.ReadLifecycleTransitions(project)
				for _, e := range scan.Events {
					if e.TaskID == id {
						t.Logf("lifecycle task=%s kind=%s event=%s source=%s actor=%s occurred=%s recorded=%s ref=%t assignment=%s attempt=%s", id, e.Kind, e.EventID, e.EvidenceSource, e.Actor, e.OccurredAt, e.RecordedAt, operationMeaningfulEvidence(e.EvidenceRef), e.AssignmentID, e.AttemptID)
					}
				}
				for _, a := range ledger.Attempts {
					if a.TaskID == id {
						t.Logf("attempt task=%s id=%s state=%s terminal=%t binding=%s assignment=%s first=%s bound=%s observed=%s activity=%s started=%s ended=%s", id, a.AttemptID, a.CurrentState, a.Terminal, a.BindingState, a.BindingEvidence["assignment_id"], a.FirstObservedAt, a.BindingAt, a.LastObservedAt, a.LastActivityAt, a.StartedAt, a.EndedAt)
					}
				}
				hs, _ := handoff.BuildLedger(project, now)
				for _, h := range hs.Handoffs {
					if h.TaskID == id {
						for name, s := range h.Steps {
							if s.Result == "failed" {
								t.Logf("handoff failure task=%s id=%s step=%s", id, h.HandoffID, name)
							}
						}
					}
				}
				t.Errorf("actual case not classified: %s", id)
			}
		}
	}
}
