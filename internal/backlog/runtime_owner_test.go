package backlog

import (
	"encoding/json"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestA79RuntimeOwnerPolicyPreservesCoordinationGap(t *testing.T) {
	project := t.TempDir()
	now := time.Now().UTC().Add(-10 * time.Minute)
	p := rolepolicy.Policy{SchemaVersion: 1, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/pairi", NativeID: "implementation-native"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Reviewer: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/raichyu"}, Method: "worker", Workspace: project, Scope: []string{"source.go"}}
	encoded, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	raw := "# A-79 coordination\n\n## 작업 개요\n- Agent: /root/controller/raichyu\n\n## 작업 정의\n\n### 목표\nPreserve implementation evidence.\n\n" + rolepolicy.Fence + string(encoded) + "\n```\n\n### 수용 기준\n- [ ] Keep implementation gap visible.\n"
	parsed, _, e := rolepolicy.Load(project, "A-79", raw)
	if e != nil || parsed == nil || parsed.Implementation.AgentPath != "/root/controller/pairi" {
		t.Fatal("invalid policy fixture", parsed, e)
	}
	dir := filepath.Join(project, "_task_mecca/data/backlog")
	if e = os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(dir, "000079.A-79.coordination.doing.md")
	if e = os.WriteFile(file, []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	for i, actor := range []struct {
		path, id string
		state    runtimeobs.CanonicalState
	}{{"/root/controller/pairi", "implementation-native", runtimeobs.StateCompleted}, {"/root/controller/raichyu", "reviewer-native", runtimeobs.StateRunning}} {
		at := now.Add(time.Duration(i) * time.Second)
		assignment := actor.id + "-assignment"
		attempt := actor.id + "-attempt"
		if _, e = runtimeobs.RecordAssignment(project, assignment, "A-79", actor.path, at); e != nil {
			t.Fatal(e)
		}
		for _, event := range []runtimeobs.ExecutionEvent{{EventKind: "state", ObservedAt: at.Format(time.RFC3339Nano), AttemptID: attempt, Provider: "codex", SessionID: actor.id, RuntimeAgentID: actor.id, State: actor.state, Terminal: actor.state == runtimeobs.StateCompleted, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}, {EventKind: "binding", ObservedAt: at.Add(time.Millisecond).Format(time.RFC3339Nano), AttemptID: attempt, TaskID: "A-79", AgentPath: actor.path, BindingSource: "dispatch", BindingEvidence: map[string]string{"assignment_id": assignment}, EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative}} {
			if e = runtimeobs.AppendExecutionEvent(project, event); e != nil {
				t.Fatal(e)
			}
		}
	}
	rows := []Record{{ID: "A-79", State: "doing", Location: "active", Path: file, RawMarkdown: raw, Fields: map[string]string{"Agent": "/root/controller/raichyu"}}}
	got := runtimeLedgerSignals(project, rows, now.Add(3*time.Second))["A-79"]
	assertOwnerRecovery(t, project, "turn_ended_progress_collection_gap", "/root/controller/pairi")
	others, ok := got["other_actors"].([]map[string]any)
	if !ok || len(others) != 1 || others[0]["attempt_id"] != "reviewer-native-attempt" {
		t.Fatalf("other actor evidence=%+v", got)
	}
	if e = runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: now.Add(4 * time.Second).Format(time.RFC3339Nano), AttemptID: "reviewer-native-attempt", Provider: "codex", SessionID: "reviewer-native", RuntimeAgentID: "reviewer-native", State: runtimeobs.StateCompleted, Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); e != nil {
		t.Fatal(e)
	}
	after := runtimeLedgerSignals(project, rows, now.Add(5*time.Second))["A-79"]
	if after["attempt_id"] != "implementation-native-attempt" || after["health"] != "progress_collection_gap" {
		t.Fatalf("review completion changed implementer: %+v", after)
	}
	assertOwnerRecovery(t, project, "turn_ended_progress_collection_gap", "/root/controller/pairi")
	coordination, coordinateErr := Coordinate(project, "", 4)
	t.Logf("supported Coordinate result=%+v error=%v", coordination, coordinateErr)
	if got["attempt_id"] != "implementation-native-attempt" || got["health"] != "progress_collection_gap" {
		t.Fatalf("authoritative implementation policy ignored; completed implementation gap replaced: %+v", got)
	}
}

func ownerFixture(t *testing.T, legacy, native string) (string, Record, time.Time) {
	t.Helper()
	project := t.TempDir()
	at := time.Now().UTC().Add(-10 * time.Minute)
	p := rolepolicy.Policy{SchemaVersion: 1, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/pairi", NativeID: native}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Reviewer: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/auditor"}, Method: "worker", Workspace: project, Scope: []string{"source.go"}}
	encoded, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	raw := "# A-79 owner\n\n## 작업 개요\n- Agent: " + legacy + "\n\n## 작업 정의\n\n### 목표\nPreserve owner.\n\n" + rolepolicy.Fence + string(encoded) + "\n```\n\n### 수용 기준\n- [ ] Keep gap.\n"
	dir := filepath.Join(project, "_task_mecca/data/backlog")
	if e = os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(dir, "000079.A-79.owner.doing.md")
	if e = os.WriteFile(file, []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	return project, Record{ID: "A-79", State: "doing", Location: "active", Path: file, RawMarkdown: raw, Fields: map[string]string{"Agent": legacy}}, at
}
func ownerAttempt(t *testing.T, project, path, native, id string, at time.Time, state runtimeobs.CanonicalState) {
	t.Helper()
	assignment := id + "-assignment"
	if _, e := runtimeobs.RecordAssignment(project, assignment, "A-79", path, at); e != nil {
		t.Fatal(e)
	}
	for _, event := range []runtimeobs.ExecutionEvent{
		{EventKind: "state", ObservedAt: at.Format(time.RFC3339Nano), AttemptID: id, Provider: "codex", SessionID: native, RuntimeAgentID: native, State: state, Terminal: state == runtimeobs.StateCompleted, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved},
		{EventKind: "binding", ObservedAt: at.Add(time.Millisecond).Format(time.RFC3339Nano), AttemptID: id, TaskID: "A-79", AgentPath: path, BindingSource: "dispatch", BindingEvidence: map[string]string{"assignment_id": assignment}, EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative},
	} {
		if e := runtimeobs.AppendExecutionEvent(project, event); e != nil {
			t.Fatal(e)
		}
	}
}
func assertOwnerRecovery(t *testing.T, project, code, agent string) {
	t.Helper()
	got, e := Coordinate(project, "", 4)
	if e != nil {
		t.Fatal(e)
	}
	queue, ok := got["recovery_queue"].([]map[string]any)
	if !ok || len(queue) != 1 || queue[0]["id"] != "A-79" || queue[0]["code"] != code || queue[0]["agent"] != agent {
		t.Fatalf("recovery=%+v", got["recovery_queue"])
	}
}
func TestA79RuntimeOwnerNativeAndAssignmentBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, native string
		prepared     bool
		health, code string
	}{
		{"same_native", "impl", false, "progress_collection_gap", "turn_ended_progress_collection_gap"},
		{"foreign_native", "foreign", false, "implementation_owner_unverified", "implementation_owner_collection_gap"},
		{"new_prepared_assignment", "impl", true, "assignment_unobserved", "worker_start_collection_gap"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project, row, at := ownerFixture(t, "/root/controller/auditor", "impl")
			ownerAttempt(t, project, "/root/controller/pairi", tc.native, "implementation", at, runtimeobs.StateCompleted)
			ownerAttempt(t, project, "/root/controller/auditor", "reviewer", "review", at.Add(time.Second), runtimeobs.StateRunning)
			if tc.prepared {
				if _, e := runtimeobs.RecordAssignment(project, "new-implementation-assignment", "A-79", "/root/controller/pairi", at.Add(2*time.Second)); e != nil {
					t.Fatal(e)
				}
			}
			signal := runtimeLedgerSignals(project, []Record{row}, time.Now())["A-79"]
			if signal["health"] != tc.health || signal["attempt_id"] == "review" {
				t.Fatalf("signal=%+v", signal)
			}
			assertOwnerRecovery(t, project, tc.code, "/root/controller/pairi")
		})
	}
}
func TestA79RuntimeOwnerUnknownAndMalformedFailClosed(t *testing.T) {
	for _, raw := range []string{"", rolepolicy.Fence + "{invalid}\n```"} {
		for _, legacy := range []string{"", "unknown", "/root/controller/pairi, /root/controller/auditor"} {
			t.Run(legacy+raw, func(t *testing.T) {
				project, row, at := ownerFixture(t, legacy, "impl")
				row.RawMarkdown = raw
				// Persist the same invalid/legacy-only contract for the actual coordination path.
				if e := os.WriteFile(row.Path, []byte("# A-79 owner\n\n## 작업 개요\n- Agent: "+legacy+"\n\n"+raw), 0600); e != nil {
					t.Fatal(e)
				}
				ownerAttempt(t, project, "/root/controller/auditor", "reviewer", "review", at, runtimeobs.StateRunning)
				signal := runtimeLedgerSignals(project, []Record{row}, time.Now())["A-79"]
				if signal["health"] != "implementation_owner_unverified" || signal["attempt_id"] != nil {
					t.Fatalf("unknown ownership guessed: %+v", signal)
				}
				assertOwnerRecovery(t, project, "implementation_owner_collection_gap", "")
			})
		}
	}
}
func TestA79RuntimeOwnerCurrentEpisodeNotOldObservation(t *testing.T) {
	project, row, at := ownerFixture(t, "/root/controller/auditor", "impl")
	ownerAttempt(t, project, "/root/controller/pairi", "impl", "old", at, runtimeobs.StateCompleted)
	ownerAttempt(t, project, "/root/controller/pairi", "impl", "current", at.Add(time.Second), runtimeobs.StateRunning)
	// Refresh old history after the current dispatch: it must not replace the current assignment.
	if e := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: at.Add(3 * time.Second).Format(time.RFC3339Nano), AttemptID: "old", Provider: "codex", SessionID: "impl", RuntimeAgentID: "impl", State: runtimeobs.StateCompleted, Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); e != nil {
		t.Fatal(e)
	}
	got := runtimeLedgerSignals(project, []Record{row}, time.Now())["A-79"]
	if got["attempt_id"] != "current" || got["health"] != "active" {
		t.Fatalf("stale episode reused: %+v", got)
	}
}

func TestA79RuntimeOwnerAmbiguousAssignmentAndUnobservedIdentity(t *testing.T) {
	for _, ambiguous := range []bool{true, false} {
		t.Run(fmt.Sprint(ambiguous), func(t *testing.T) {
			project, row, at := ownerFixture(t, "/root/controller/auditor", "impl")
			if ambiguous {
				ownerAttempt(t, project, "/root/controller/pairi", "impl", "first", at, runtimeobs.StateCompleted)
				if _, e := runtimeobs.RecordAssignment(project, "equally-current", "A-79", "/root/controller/pairi", at); e != nil {
					t.Fatal(e)
				}
			} else {
				if _, e := runtimeobs.RecordAssignment(project, "unobserved", "A-79", "/root/controller/pairi", at); e != nil {
					t.Fatal(e)
				}
				if e := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: at.Format(time.RFC3339Nano), AttemptID: "manual", Provider: "codex", RuntimeAgentID: "impl", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityInferred}); e != nil {
					t.Fatal(e)
				}
				if _, e := runtimeobs.BindAttempt(project, "manual", "A-79", "/root/controller/pairi", "dispatch", "", map[string]string{"assignment_id": "unobserved"}, at); e != nil {
					t.Fatal(e)
				}
			}
			ownerAttempt(t, project, "/root/controller/auditor", "reviewer", "review", at.Add(time.Second), runtimeobs.StateRunning)
			signal := runtimeLedgerSignals(project, []Record{row}, time.Now())["A-79"]
			if signal["health"] != "implementation_owner_unverified" {
				t.Fatalf("unverified ownership became progress: %+v", signal)
			}
			assertOwnerRecovery(t, project, "implementation_owner_collection_gap", "/root/controller/pairi")
		})
	}
}

func TestA79RuntimeOwnerCompletionRequiresMatchingImplementationReport(t *testing.T) {
	for _, mode := range []string{"matching", "legacy_matching", "reviewer_report", "foreign_task", "foreign_contract", "old_attempt"} {
		t.Run(mode, func(t *testing.T) {
			project, row, at := ownerFixture(t, "/root/controller/auditor", "impl")
			if mode == "legacy_matching" {
				row.RawMarkdown = strings.ReplaceAll(row.RawMarkdown[:strings.Index(row.RawMarkdown, rolepolicy.Fence)], "/root/controller/auditor", "/root/controller/pairi")
				row.Fields["Agent"] = "/root/controller/pairi"
				if e := os.WriteFile(row.Path, []byte(row.RawMarkdown), 0600); e != nil {
					t.Fatal(e)
				}
			}
			originalAgent := row.Fields["Agent"]
			ownerAttempt(t, project, "/root/controller/pairi", "impl", "implementation", at, runtimeobs.StateRunning)
			if e := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: at.Add(time.Second).Format(time.RFC3339Nano), AttemptID: "implementation", Provider: "codex", SessionID: "impl", RuntimeAgentID: "impl", State: runtimeobs.StateCompleted, Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); e != nil {
				t.Fatal(e)
			}
			ownerAttempt(t, project, "/root/controller/auditor", "reviewer", "review", at.Add(2*time.Second), runtimeobs.StateCompleted)
			report := completionHandoffRecord{HandoffID: "implementation-done", Kind: "prepared", Event: "worker_done", TaskID: "A-79", Contract: completionContract(row.RawMarkdown), SourceAgent: "/root/controller/pairi", SourceAttempt: "implementation", TargetAgent: "/root/controller", TargetAttempt: "controller", Provider: "codex", At: at.Add(3 * time.Second).Format(time.RFC3339Nano)}
			switch mode {
			case "reviewer_report":
				report.SourceAgent = "/root/controller/auditor"
				report.SourceAttempt = "review"
			case "foreign_task":
				report.TaskID = "A-80"
			case "foreign_contract":
				report.Contract = "different"
			case "old_attempt":
				report.SourceAttempt = "previous"
			}
			writeReviewJournal(t, project, report)
			signals := runtimeLedgerSignals(project, []Record{row}, time.Now())
			applyCompletionReviews(project, []Record{row}, signals, time.Now())
			signal := signals["A-79"]
			if mode == "matching" || mode == "legacy_matching" {
				if signal["completion_review"] == nil || signal["health"] == "progress_collection_gap" {
					t.Fatalf("matching report lost: %+v", signal)
				}
			} else if signal["health"] != "progress_collection_gap" || signal["completion_review"] != nil {
				t.Fatalf("foreign report promoted implementation review: %+v", signal)
			}
			if row.Fields["Agent"] != originalAgent {
				t.Fatal("projection mutated original record")
			}
		})
	}
}

func TestA79RuntimeOwnerDoesNotRetargetUnrelatedCompletedTask(t *testing.T) {
	project, row, at := ownerFixture(t, "/root/controller/auditor", "impl")
	ownerAttempt(t, project, "/root/controller/pairi", "impl", "implementation", at, runtimeobs.StateCompleted)
	other := Record{ID: "A-80", State: "done", Location: "completed", RawMarkdown: "# A-80 historic result"}
	got := runtimeLedgerSignals(project, []Record{row, other}, time.Now())
	if got["A-79"]["health"] != "progress_collection_gap" || got["A-80"] != nil || other.State != "done" {
		t.Fatalf("unrelated completion altered: %+v", got)
	}
}

func TestA102RuntimeOwnerPreparedAssignmentCoexistsWithActualPhase(t *testing.T) {
	for _, mode := range []string{"prepared", "pending_bound", "foreign_native", "wrong_task", "wrong_role", "wrong_assignment", "missing_start", "phase_stopped", "terminal", "future_observed", "ambiguous_current"} {
		t.Run(mode, func(t *testing.T) {
			project, row, at := ownerFixture(t, "/root/controller/pairi", "impl")
			native := "impl"
			if mode == "foreign_native" {
				native = "foreign"
			}
			state := runtimeobs.StateRunning
			if mode == "terminal" {
				state = runtimeobs.StateCompleted
			}
			ownerAttempt(t, project, "/root/controller/pairi", native, "current8bd", at, state)
			phase := activity.Event{SchemaVersion: 1, EventID: "a102-current-phase", TaskID: "A-79", Action: activity.Start, Role: activity.RoleWorker, AgentID: native, SessionID: native, RuntimeAttemptID: "current8bd", AssignmentID: "current8bd-assignment", Type: activity.Requirements, PhaseEvent: "requirements_started", Confidence: activity.Confirmed, Source: "agent_report", OccurredAt: at.Add(10 * time.Millisecond), RecordedAt: at.Add(10 * time.Millisecond)}
			if mode == "wrong_role" {
				phase.Role = activity.RoleRegistrar
			}
			if mode == "wrong_assignment" {
				phase.AssignmentID = "foreign-assignment"
			}
			if mode != "missing_start" {
				if _, err := (activity.Ledger{Project: project}).Append(phase); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "phase_stopped" {
				phase.EventID = "a102-current-stop"
				phase.Action = activity.Stop
				phase.PhaseEvent = "phase_completed"
				phase.OccurredAt = at.Add(20 * time.Millisecond)
				phase.RecordedAt = phase.OccurredAt
				if _, err := (activity.Ledger{Project: project}).Append(phase); err != nil {
					t.Fatal(err)
				}
			}
			task := "A-79"
			if mode == "wrong_task" {
				task = "A-80"
				row.ID = task
			}
			if _, err := runtimeobs.RecordAssignment(project, "future088e", task, "/root/controller/pairi", at.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			if mode == "pending_bound" || mode == "future_observed" {
				quality, source, state := runtimeobs.QualityInferred, runtimeobs.EvidenceManualBinding, runtimeobs.StateRuntimeUnknown
				if mode == "future_observed" {
					quality, source, state = runtimeobs.QualityObserved, runtimeobs.EvidenceHook, runtimeobs.StateRunning
				}
				if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: at.Add(2 * time.Second).Format(time.RFC3339Nano), AttemptID: "future-pending", Provider: "codex", SessionID: "impl", RuntimeAgentID: "impl", State: state, EvidenceSource: source, ObservationQuality: quality}); err != nil {
					t.Fatal(err)
				}
				if _, err := runtimeobs.BindAttempt(project, "future-pending", "A-79", "/root/controller/pairi", "dispatch", "", map[string]string{"assignment_id": "future088e"}, at.Add(2*time.Second)); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "ambiguous_current" {
				ownerAttempt(t, project, "/root/controller/pairi", "impl", "another-current", at.Add(100*time.Millisecond), runtimeobs.StateRunning)
				phase.EventID = "a102-another-phase"
				phase.RuntimeAttemptID = "another-current"
				phase.AssignmentID = "another-current-assignment"
				phase.OccurredAt = at.Add(200 * time.Millisecond)
				phase.RecordedAt = phase.OccurredAt
				if _, err := (activity.Ledger{Project: project}).Append(phase); err != nil {
					t.Fatal(err)
				}
			}
			signal := runtimeLedgerSignals(project, []Record{row}, at.Add(3*time.Second))[task]
			if mode == "prepared" || mode == "pending_bound" {
				if signal["attempt_id"] != "current8bd" || signal["assignment_id"] != "current8bd-assignment" || signal["recent_activity_report"] == nil || signal["human_action_runtime_verified"] != true {
					t.Fatalf("current execution hidden by future request: %+v", signal)
				}
			} else if signal["attempt_id"] == "current8bd" {
				t.Fatalf("invalid current evidence bypassed future assignment: %+v", signal)
			}
			if mode == "ambiguous_current" && signal["owner_resolution_error"] != "implementation_assignment_ambiguous" {
				t.Fatalf("ambiguous live phases selected: %+v", signal)
			}
		})
	}
}

func TestA79RuntimeOwnerLegacyTerminalWithoutReportRetainsRecovery(t *testing.T) {
	project, row, at := ownerFixture(t, "/root/controller/pairi", "impl")
	row.RawMarkdown = row.RawMarkdown[:strings.Index(row.RawMarkdown, rolepolicy.Fence)]
	if e := os.WriteFile(row.Path, []byte(row.RawMarkdown), 0600); e != nil {
		t.Fatal(e)
	}
	ownerAttempt(t, project, "/root/controller/pairi", "impl", "implementation", at, runtimeobs.StateCompleted)
	signals := runtimeLedgerSignals(project, []Record{row}, time.Now())
	applyCompletionReviews(project, []Record{row}, signals, time.Now())
	signal := signals["A-79"]
	if signal["health"] != "progress_collection_gap" || signal["completion_review"] != nil {
		t.Fatalf("terminal-only legacy observation promoted to review: %+v", signal)
	}
	progress := signal["progress_evidence"].(map[string]any)
	if progress["phase"] != "assigned_prepared" || progress["technical_crash"] != "unproven" {
		t.Fatalf("phase fabricated: %+v", progress)
	}
	assertOwnerRecovery(t, project, "turn_ended_progress_collection_gap", "/root/controller/pairi")
}
