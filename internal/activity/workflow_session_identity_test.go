package activity

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func a97Attempt(id, native, path string) runtimeobs.Attempt {
	_, at := workflowFixture()
	return runtimeobs.Attempt{AttemptID: id, TaskID: "T", AgentPath: path, RuntimeAgentID: native, SessionID: native, Provider: "codex", BindingState: runtimeobs.BindingBound, BindingEvidence: map[string]string{"assignment_id": "assignment-" + id, "runtime_agent_id": native}, StartedAt: at(1).Format(time.RFC3339Nano), LastObservedAt: at(8).Format(time.RFC3339Nano), StateEvidenceSource: runtimeobs.EvidenceHook, StateObservationQuality: runtimeobs.QualityObserved, CurrentState: runtimeobs.StateRunning}
}

func TestA102PreviousClaimDisplayJoinDoesNotRestoreAuthority(t *testing.T) {
	at := func(n int) time.Time { return time.Date(2026, 11, 25, 0, 0, n, 0, time.UTC) }
	for _, mode := range []string{"valid", "terminal_after_transfer", "terminal_before_transfer", "other_native", "other_task", "other_thread", "other_project", "other_assignment", "missing_binding", "future_observation", "same_native_attempt", "unobserved", "missing_terminal", "future_transfer", "missing_transfer", "older_than_last", "activity_after_terminal", "ambiguous_current"} {
		t.Run(mode, func(t *testing.T) {
			old := a97Attempt("before", "native", "/root/controller")
			old.BindingAt = at(2).Format(time.RFC3339Nano)
			old.BindingEvidence["assignment_id"] = "continuing"
			old.TurnID = "native-before"
			old.Terminal = true
			old.CurrentState = runtimeobs.StateErrored
			old.EndedAt = at(6).Format(time.RFC3339Nano)
			old.LastObservedAt = old.EndedAt
			old.StateEvidenceSource = runtimeobs.EvidenceT3
			old.NativeReceipt = &runtimeobs.T3Receipt{Origin: "t3_local_projection_v2", Provider: "codex", SessionID: "native", CanonicalProject: "/canonical", ThreadID: "actual-thread", RunID: "run-before", NativeAttemptID: old.TurnID, State: old.CurrentState, StartedAt: at(1).Format(time.RFC3339Nano), CompletedAt: at(5).Format(time.RFC3339Nano), ObservedAt: old.LastObservedAt, NativeStatus: "failed", RunStatus: "failed"}
			next := a97Attempt("after", "native", "/root/controller")
			next.BindingAt = at(6).Format(time.RFC3339Nano)
			next.BindingEvidence["assignment_id"] = "continuing"
			next.TurnID = "native-after"
			next.StartedAt = at(6).Format(time.RFC3339Nano)
			next.LastObservedAt = at(9).Format(time.RFC3339Nano)
			next.StateEvidenceSource = runtimeobs.EvidenceT3
			next.NativeReceipt = &runtimeobs.T3Receipt{Origin: "t3_local_projection_v2", Provider: "codex", SessionID: "native", CanonicalProject: "/canonical", ThreadID: "actual-thread", RunID: "run-after", NativeAttemptID: next.TurnID, State: next.CurrentState, StartedAt: next.StartedAt, ObservedAt: next.LastObservedAt}
			h := handoffjournal.Handoff{HandoffID: "intake", TaskID: "T", ClaimedBy: "/root/controller", ClaimedAttemptID: "after", ClaimedAt: at(2).Format(time.RFC3339Nano), LastTransferFrom: "before", LastTransferredAt: at(7).Format(time.RFC3339Nano), TransferCount: 1, Target: handoffjournal.Target{AgentPath: "/root/controller", AttemptID: "after", Provider: "codex", SessionID: "native", RuntimeAgentID: "native"}}
			event := Event{EventID: "old-phase", TaskID: "T", Role: RoleController, AgentID: "native", RuntimeAttemptID: "before", HandoffID: "intake", Action: Start, Type: Handoff, OccurredAt: at(3), RecordedAt: at(3)}
			switch mode {
			case "terminal_after_transfer", "terminal_before_transfer":
				next.Terminal = true
				next.CurrentState = runtimeobs.StateCompleted
				next.NativeReceipt.State = next.CurrentState
				next.NativeReceipt.CompletedAt = at(8).Format(time.RFC3339Nano)
				if mode == "terminal_before_transfer" {
					next.NativeReceipt.CompletedAt = at(6).Format(time.RFC3339Nano)
				}
			case "other_native":
				next.SessionID = "other"
				next.RuntimeAgentID = "other"
			case "other_task":
				next.TaskID = "foreign"
			case "other_thread":
				next.NativeReceipt.ThreadID = "other-thread"
			case "other_project":
				next.NativeReceipt.CanonicalProject = "/other-project"
			case "missing_binding":
				old.BindingAt = ""
			case "future_observation":
				next.NativeReceipt.ObservedAt = at(11).Format(time.RFC3339Nano)
				next.LastObservedAt = next.NativeReceipt.ObservedAt
			case "same_native_attempt":
				next.TurnID = old.TurnID
				next.NativeReceipt.NativeAttemptID = old.TurnID
			case "other_assignment":
				next.BindingEvidence["assignment_id"] = "new-assignment"
			case "unobserved":
				old.StateObservationQuality = runtimeobs.QualityInferred
			case "missing_terminal":
				old.Terminal = false
			case "future_transfer":
				h.LastTransferredAt = at(11).Format(time.RFC3339Nano)
			case "missing_transfer":
				h.LastTransferredAt = ""
			case "older_than_last":
				h.LastTransferFrom = "other-previous"
			case "activity_after_terminal":
				event.OccurredAt = at(6)
			}
			in := WorkflowInput{TaskID: "T", Now: at(10), ExecutionObservations: []runtimeobs.Attempt{old, next}, Handoffs: map[string]handoffjournal.Handoff{"intake": h}, Events: []Event{event}}
			if mode == "ambiguous_current" {
				in.ExecutionObservations = append(in.ExecutionObservations, next)
			}
			rows := map[string]*WorkflowRow{"old": {ID: "old", Role: "controller", AgentID: "native", Intervals: []WorkflowInterval{{ID: "old-phase", Kind: "activity", AttemptID: "before", HandoffID: "intake", Start: event.OccurredAt}}}, "next": {ID: "next", Role: "controller", AgentID: "native", Intervals: []WorkflowInterval{{ID: "next-native", Kind: "native_observed", AttemptID: "after", AssignmentID: "continuing", Start: at(6), Evidence: string(runtimeobs.EvidenceT3)}}}}
			out := workflowSessionRows(in, rows)
			joined := false
			for _, row := range out {
				if row.SessionIdentity.Status == "verified" {
					for _, a := range row.SessionIdentity.Attempts {
						if a.AttemptID == "before" {
							joined = a.PreviousClaim != nil && len(row.SessionIdentity.Attempts) == 2
						}
					}
				}
			}
			if joined != (mode == "valid" || mode == "terminal_after_transfer") {
				t.Fatalf("mode=%s joined=%v rows=%+v", mode, joined, out)
			}
			if workflowSessionAssociation(in, old, "", "intake") || workflowSessionClaim(in, old, "intake") {
				t.Fatal("display join restored historical authorization")
			}
		})
	}
}

func TestA102ThreeTransferHistorySurvivesNextResume(t *testing.T) {
	at := func(n int) time.Time { return time.Date(2026, 11, 25, 0, 0, n, 0, time.UTC) }
	for _, mode := range []string{"task_bound", "global_claim_scoped", "missing_middle", "fork", "duplicate_claim", "foreign_transfer", "nonmonotone", "native_overlap", "unobserved_old", "final_mismatch", "global_before_claim", "global_missing_claim", "global_wrong_operational_native", "global_wrong_explicit_task", "global_missing_operational_owner", "global_project_verified", "global_project_missing", "global_project_foreign", "global_project_native", "global_project_state", "global_project_end", "global_project_future"} {
		t.Run(mode, func(t *testing.T) {
			global := mode == "global_claim_scoped" || strings.HasPrefix(mode, "global_")
			in := WorkflowInput{TaskID: "T", Now: at(40), Handoffs: map[string]handoffjournal.Handoff{}, OperationalControllerNative: "native"}
			rows := map[string]*WorkflowRow{}
			for n, id := range []string{"a", "b", "c", "d"} {
				a := a97Attempt(id, "native", "/root/controller")
				a.BindingAt = at(2).Format(time.RFC3339Nano)
				a.BindingEvidence["assignment_id"] = "continuing"
				a.TurnID = "native-" + id
				a.StartedAt = at(1 + n*5).Format(time.RFC3339Nano)
				a.LastObservedAt = at(30).Format(time.RFC3339Nano)
				a.StateEvidenceSource = runtimeobs.EvidenceT3
				if global {
					a.TaskID = ""
					a.BindingEvidence["assignment_id"] = ""
				}
				if n < 3 {
					a.Terminal = true
					a.CurrentState = runtimeobs.StateErrored
					a.EndedAt = at(5 + n*5).Format(time.RFC3339Nano)
				}
				a.NativeReceipt = &runtimeobs.T3Receipt{Origin: "t3_local_projection_v2", Provider: "codex", SessionID: "native", CanonicalProject: "/canonical", ThreadID: "actual-thread", RunID: "run-" + id, NativeAttemptID: a.TurnID, State: a.CurrentState, StartedAt: a.StartedAt, CompletedAt: a.EndedAt, ObservedAt: a.LastObservedAt}
				in.ExecutionObservations = append(in.ExecutionObservations, a)
				when := at(4 + n*5)
				if mode == "global_before_claim" && n == 0 {
					when = at(2)
				}
				in.Events = append(in.Events, Event{EventID: "phase-" + id, TaskID: "T", Role: RoleController, AgentID: "native", RuntimeAttemptID: id, AssignmentID: a.BindingEvidence["assignment_id"], HandoffID: "intake", Action: Start, Type: Handoff, OccurredAt: when, RecordedAt: when})
				round := n
				rows[id] = &WorkflowRow{ID: id, Role: "controller", AgentID: "native", Round: &round, Intervals: []WorkflowInterval{{ID: "phase-" + id, Kind: "activity", AttemptID: id, AssignmentID: a.BindingEvidence["assignment_id"], HandoffID: "intake", Start: when}}}
			}
			h := handoffjournal.Handoff{HandoffID: "intake", TaskID: "T", ContractSHA256: "contract", ClaimedBy: "/root/controller", ClaimedAttemptID: "d", ClaimedAt: at(3).Format(time.RFC3339Nano), TransferCount: 3, LastTransferFrom: "c", LastTransferredAt: at(17).Format(time.RFC3339Nano), RecordCount: 5, Target: handoffjournal.Target{AgentPath: "/root/controller", AttemptID: "d", Provider: "codex", SessionID: "native", RuntimeAgentID: "native"}}
			in.HandoffRecords = []handoffjournal.Record{{HandoffID: "intake", RecordKind: "prepared", TaskID: "T", ContractSHA256: "contract", TargetAgentPath: "/root/controller", TargetAttemptID: "a", TargetRuntimeAgentID: "native", TargetSessionID: "native", Provider: "codex", ObservedAt: at(2).Format(time.RFC3339Nano)}, {HandoffID: "intake", RecordKind: "claimed", ClaimedBy: "/root/controller", ClaimedAttemptID: "a", ObservedAt: at(3).Format(time.RFC3339Nano)}}
			ids := []string{"a", "b", "c", "d"}
			for n := 0; n < 3; n++ {
				in.HandoffRecords = append(in.HandoffRecords, handoffjournal.Record{HandoffID: "intake", RecordKind: "claim_transferred", ClaimedBy: "/root/controller", PreviousClaimedAttemptID: ids[n], ClaimedAttemptID: ids[n+1], TargetAgentPath: "/root/controller", TargetAttemptID: ids[n+1], TargetRuntimeAgentID: "native", TargetSessionID: "native", Provider: "codex", ObservedAt: at(7 + n*5).Format(time.RFC3339Nano)})
			}
			switch mode {
			case "global_wrong_operational_native":
				in.OperationalControllerNative = "foreign"
			case "global_missing_operational_owner":
				in.OperationalControllerNative = ""
			case "global_wrong_explicit_task":
				in.ExecutionObservations[0].TaskID = "OTHER"
			case "global_missing_claim":
				in.HandoffRecords = nil
			case "missing_middle":
				in.HandoffRecords = append(in.HandoffRecords[:3], in.HandoffRecords[4:]...)
			case "fork":
				in.HandoffRecords[3].PreviousClaimedAttemptID = "foreign"
			case "duplicate_claim":
				in.HandoffRecords = append(in.HandoffRecords, in.HandoffRecords[1])
				h.RecordCount++
			case "foreign_transfer":
				in.HandoffRecords[3].TargetSessionID = "foreign"
			case "nonmonotone":
				in.HandoffRecords[3].ObservedAt = at(6).Format(time.RFC3339Nano)
			case "native_overlap":
				in.ExecutionObservations[0].NativeReceipt.CompletedAt = at(8).Format(time.RFC3339Nano)
			case "unobserved_old":
				in.ExecutionObservations[0].StateObservationQuality = runtimeobs.QualityInferred
			case "final_mismatch":
				h.TransferCount = 4
			}
			in.Handoffs["intake"] = h
			if strings.HasPrefix(mode, "global_project_") {
				in.SourceProject = "/canonical"
				in.NativeProjectReceipts = map[string]runtimeobs.T3Receipt{}
				for i := range in.ExecutionObservations {
					a := &in.ExecutionObservations[i]
					proof := *a.NativeReceipt
					in.NativeProjectReceipts[a.AttemptID] = proof
					a.NativeReceipt.CanonicalProject = ""
				}
				proof := in.NativeProjectReceipts["a"]
				switch mode {
				case "global_project_missing":
					delete(in.NativeProjectReceipts, "a")
				case "global_project_foreign":
					proof.CanonicalProject = "/foreign"
				case "global_project_native":
					proof.SessionID = "foreign"
				case "global_project_state":
					proof.State = runtimeobs.StateRunning
				case "global_project_end":
					proof.CompletedAt = at(6).Format(time.RFC3339Nano)
				case "global_project_future":
					proof.ObservedAt = at(41).Format(time.RFC3339Nano)
				}
				if mode != "global_project_missing" {
					in.NativeProjectReceipts["a"] = proof
				}
			}

			out := workflowSessionRows(in, rows)
			verified := map[string]WorkflowSessionAttempt{}
			for _, r := range out {
				if r.SessionIdentity.Status == "verified" {
					for _, a := range r.SessionIdentity.Attempts {
						verified[a.AttemptID] = a
					}
				}
			}
			valid := mode == "task_bound" || mode == "global_claim_scoped" || mode == "global_project_verified"
			if valid {
				if len(out) != 1 || len(verified) != 4 {
					t.Fatalf("same verified native split after third transfer: %+v", out)
				}
				for _, id := range []string{"a", "b", "c"} {
					p := verified[id].PreviousClaim
					if p == nil || p.ChainTransfers != 3 || p.Basis != "validated_append_only_claim_chain_same_verified_native" {
						t.Fatalf("history proof absent for %s: %+v", id, p)
					}
				}
			} else if _, promoted := verified["a"]; promoted {
				t.Fatalf("invalid/unknown history promoted oldest actor: %s %+v", mode, out)
			}
			for _, a := range in.ExecutionObservations[:3] {
				if workflowSessionClaim(in, a, "intake") {
					t.Fatal("history changed current claim authority")
				}
			}
		})
	}
}

func TestA102ControllerLifecyclePointNeedsItsOwnExactAuthor(t *testing.T) {
	_, at := workflowFixture()
	for _, mode := range []string{"valid", "missing_author", "foreign_assignment", "foreign_claim", "unobserved", "after_native_end"} {
		t.Run(mode, func(t *testing.T) {
			a := a97Attempt("controller", "native", "/root/controller")
			a.BindingAt = a.StartedAt
			a.TurnID = "native-attempt"
			a.StateEvidenceSource = runtimeobs.EvidenceT3
			a.NativeReceipt = &runtimeobs.T3Receipt{Origin: "t3_local_projection_v2", Provider: a.Provider, SessionID: a.SessionID, ThreadID: "actual-thread", RunID: "actual-run", NativeAttemptID: a.TurnID, State: a.CurrentState, StartedAt: a.StartedAt, ObservedAt: a.LastObservedAt}
			h := handoffjournal.Handoff{HandoffID: "intake", TaskID: "T", ClaimedBy: a.AgentPath, ClaimedAttemptID: a.AttemptID, ClaimedAt: at(2).Format(time.RFC3339Nano), Target: handoffjournal.Target{AgentPath: a.AgentPath, AttemptID: a.AttemptID, RuntimeAgentID: a.RuntimeAgentID, SessionID: a.SessionID, Provider: a.Provider}}
			f := WorkflowRoleFact{ID: "completed", Kind: "completed", Source: "controller_report", At: at(5), AttemptID: a.AttemptID, AssignmentID: a.BindingEvidence["assignment_id"]}
			switch mode {
			case "missing_author":
				f.AttemptID = ""
			case "foreign_assignment":
				f.AssignmentID = "worker-assignment"
			case "foreign_claim":
				h.Target.SessionID = "other"
			case "unobserved":
				a.StateObservationQuality = runtimeobs.QualityInferred
			case "after_native_end":
				a.Terminal = true
				a.CurrentState = runtimeobs.StateCompleted
				a.NativeReceipt.State = a.CurrentState
				a.NativeReceipt.CompletedAt = at(4).Format(time.RFC3339Nano)
			}
			in := WorkflowInput{TaskID: "T", Now: at(10), ExecutionObservations: []runtimeobs.Attempt{a}, Handoffs: map[string]handoffjournal.Handoff{"intake": h}}
			if workflowSessionAssociation(in, a, a.BindingEvidence["assignment_id"], "") {
				t.Fatal("lifecycle-only join changed Activity association")
			}
			out := workflowSessionRows(in, map[string]*WorkflowRow{"facts": {ID: "facts", Role: "controller", AgentID: "/root/controller", RoleFacts: []WorkflowRoleFact{{ID: "assigned", Kind: "assigned", Source: "controller_report", At: at(3), AssignmentID: "worker-assignment"}, f}}})
			joined := false
			unknown := false
			for _, r := range out {
				for _, fact := range r.RoleFacts {
					if fact.ID == "completed" && r.SessionIdentity.Status == "verified" {
						joined = true
					}
					if fact.ID == "assigned" && r.SessionIdentity.Status == "unverified" {
						unknown = true
					}
				}
			}
			if joined != (mode == "valid") || !unknown {
				t.Fatalf("lifecycle author inferred: %s %+v", mode, out)
			}
		})
	}
}

func TestA97SessionAliasesMergeOnlyExactBoundEvidence(t *testing.T) {
	_, at := workflowFixture()
	n := 0
	ms := int64(60000)
	a := a97Attempt("one", "native-one", "/root/controller")
	rows := map[string]*WorkflowRow{
		"path":   {ID: "path", Role: "controller", AgentID: a.AgentPath, Round: &n, Intervals: []WorkflowInterval{{ID: "source", AttemptID: a.AttemptID, AssignmentID: "assignment-one", Start: at(1)}}},
		"native": {ID: "native", Role: "controller", AgentID: a.RuntimeAgentID, Round: &n, Intervals: []WorkflowInterval{{ID: "activity", AttemptID: a.AttemptID, AssignmentID: "assignment-one", Start: at(2), Measured: true, DurationMillis: &ms}}},
	}
	in := WorkflowInput{TaskID: "T", ExecutionObservations: []runtimeobs.Attempt{a}, Handoffs: a97Claims(a)}
	before, _ := json.Marshal(rows)
	out := workflowSessionRows(in, rows)
	if len(out) != 1 {
		t.Fatalf("aliases split: %+v", out)
	}
	for _, r := range out {
		if r.SessionIdentity.Status != "verified" || len(r.Intervals) != 2 || len(r.SessionIdentity.Aliases) != 2 || r.ObservedActivityMillis == nil || *r.ObservedActivityMillis != ms {
			t.Fatalf("lost identity/activity %+v", r)
		}
	}
	after, _ := json.Marshal(rows)
	if string(before) != string(after) {
		t.Fatal("source mutated")
	}
}

func TestA97SessionConflictsNeverMerge(t *testing.T) {
	a := a97Attempt("one", "native-one", "/root/controller")
	for _, tc := range []struct {
		name   string
		change func(*WorkflowInput, *WorkflowRow)
	}{
		{"duplicateAttempt", func(in *WorkflowInput, r *WorkflowRow) {
			in.ExecutionObservations = append(in.ExecutionObservations, a)
		}},
		{"wrongTask", func(in *WorkflowInput, r *WorkflowRow) { in.ExecutionObservations[0].TaskID = "other" }},
		{"unbound", func(in *WorkflowInput, r *WorkflowRow) {
			in.ExecutionObservations[0].BindingState = runtimeobs.BindingUnbound
		}},
		{"unobserved", func(in *WorkflowInput, r *WorkflowRow) {
			in.ExecutionObservations[0].StateObservationQuality = runtimeobs.QualityInferred
		}},
		{"wrongAssignment", func(in *WorkflowInput, r *WorkflowRow) { r.Intervals[0].AssignmentID = "wrong" }},
		{"wrongRole", func(in *WorkflowInput, r *WorkflowRow) { r.Role = "worker" }},
		{"wrongSessionEvent", func(in *WorkflowInput, r *WorkflowRow) {
			in.Events = []Event{{TaskID: "T", Role: RoleController, AgentID: a.RuntimeAgentID, RuntimeAttemptID: a.AttemptID, SessionID: "other"}}
		}},
		{"nativeBindingConflict", func(in *WorkflowInput, r *WorkflowRow) {
			in.ExecutionObservations[0].BindingEvidence = map[string]string{"runtime_agent_id": "other", "assignment_id": "assignment-one"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := WorkflowInput{TaskID: "T", ExecutionObservations: []runtimeobs.Attempt{a}, Handoffs: a97Claims(a)}
			r := &WorkflowRow{ID: "path", Role: "controller", AgentID: a.AgentPath, Intervals: []WorkflowInterval{{ID: "v", AttemptID: "one", AssignmentID: "assignment-one"}}}
			tc.change(&in, r)
			out := workflowSessionRows(in, map[string]*WorkflowRow{"path": r})
			for _, v := range out {
				if v.SessionIdentity.Status != "unverified" || v.AgentID != r.AgentID {
					t.Fatalf("invented link %+v", v)
				}
			}
		})
	}
}

func TestA97SamePathDifferentSessionsAndResumeRetainAttempts(t *testing.T) {
	a := a97Attempt("first", "native-one", "/root/controller/kkobugi")
	b := a97Attempt("second", "native-two", a.AgentPath)
	c := a97Attempt("resume", a.SessionID, a.AgentPath)
	n := 0
	path := &WorkflowRow{ID: "path", Role: "worker", AgentID: a.AgentPath, Round: &n, Intervals: []WorkflowInterval{}, RoleFacts: []WorkflowRoleFact{}}
	for _, v := range []runtimeobs.Attempt{a, b, c} {
		path.Intervals = append(path.Intervals, WorkflowInterval{ID: v.AttemptID, AttemptID: v.AttemptID, AssignmentID: v.BindingEvidence["assignment_id"]})
	}
	out := workflowSessionRows(WorkflowInput{TaskID: "T", ExecutionObservations: []runtimeobs.Attempt{a, b, c}}, map[string]*WorkflowRow{"path": path})
	if len(out) != 2 {
		t.Fatalf("different sessions merged %+v", out)
	}
	for _, r := range out {
		if r.SessionIdentity.NativeID == a.SessionID && (len(r.Intervals) != 2 || len(r.SessionIdentity.Attempts) != 2) {
			t.Fatalf("resume lost %+v", r)
		}
	}
}

func TestA97ProjectionPreservesMeasurementAndRawEvidence(t *testing.T) {
	b, at := workflowFixture()
	a := a97Attempt("worker", "native-worker", "/root/controller/kkobugi")
	events := []Event{{EventID: "start", TaskID: "T", Role: RoleWorker, AgentID: a.RuntimeAgentID, RuntimeAttemptID: a.AttemptID, AssignmentID: "assignment-worker", Action: Start, Type: Implementation, OccurredAt: at(2), Explanation: "원문 설명"}, {EventID: "stop", TaskID: "T", Role: RoleWorker, AgentID: a.RuntimeAgentID, RuntimeAttemptID: a.AttemptID, AssignmentID: "assignment-worker", Action: Stop, Type: Implementation, OccurredAt: at(4)}}
	facts := []LifecycleFact{{TaskID: "T", EventID: "r", Kind: "registered", OccurredAt: b}, {TaskID: "T", EventID: "s", Kind: "started", Actor: a.AgentPath, AttemptID: a.AttemptID, AssignmentID: "assignment-worker", OccurredAt: at(1)}}
	in := WorkflowInput{TaskID: "T", Events: events, Summary: Summarize("T", events), Facts: facts, Now: at(10)}
	old := SummarizeWorkflow(in)
	in.ExecutionObservations = []runtimeobs.Attempt{a}
	got := SummarizeWorkflow(in)
	if !reflect.DeepEqual(old.Measurement, got.Measurement) {
		t.Fatalf("measurement changed %+v / %+v", old.Measurement, got.Measurement)
	}
	if len(got.Rows) != 1 {
		t.Fatalf("unexpected rows %+v", got.Rows)
	}
	found := false
	for _, r := range got.Rows {
		if r.Role == "worker" {
			if r.SessionIdentity.Status != "verified" {
				t.Fatal(r)
			}
			for _, v := range r.Intervals {
				if v.Explanation == "원문 설명" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("raw explanation lost")
	}
}

func TestA97AllRolesKeepRoleAndRoundEvidence(t *testing.T) {
	for _, path := range []string{"/root/controller", "/root/registrar", "/root/controller/kkobugi", "/root/controller/reviewer/pikachyu", "/root/controller/sub-pairi"} {
		t.Run(path, func(t *testing.T) {
			a := a97Attempt("one", "native", path)
			role := workflowActorRole(path)
			n0, n1 := 0, 1
			rows := map[string]*WorkflowRow{}
			for i, n := range []*int{&n0, &n1} {
				id := string(rune('a' + i))
				rows[id] = &WorkflowRow{ID: id, Role: role, AgentID: path, Round: n, Intervals: []WorkflowInterval{{ID: id, AttemptID: a.AttemptID, AssignmentID: "assignment-one"}}}
			}
			rows["wrong-role"] = &WorkflowRow{ID: "wrong-role", Role: "unknown", AgentID: path, Intervals: []WorkflowInterval{{ID: "wrong", AttemptID: a.AttemptID}}}
			out := workflowSessionRows(WorkflowInput{TaskID: "T", ExecutionObservations: []runtimeobs.Attempt{a}, Handoffs: a97Claims(a)}, rows)
			// Correct the new A97 expectation: verified sessions cross work-round
			// boundaries, while role separation and source rounds remain mandatory.
			if len(out) != 2 {
				t.Fatalf("same session split by round or role merged %+v", out)
			}
			for _, r := range out {
				if r.Role == "unknown" && r.SessionIdentity.Status != "unverified" {
					t.Fatal("role inferred from name")
				}
				if r.Role == role && r.SessionIdentity.NativeID != "native" {
					t.Fatal("common role not resolved")
				}
				if r.Role == role {
					if r.Round != nil || r.RoundStatus != "multiple" || len(r.Rounds) != 2 || len(r.SessionIdentity.Attempts) != 1 || len(r.SessionIdentity.Attempts[0].Rounds) != 2 {
						t.Fatalf("session round provenance missing: %+v", r)
					}
					rounds := map[int]bool{}
					for _, v := range r.Intervals {
						if v.Round == nil {
							t.Fatal("interval round omitted")
						}
						rounds[*v.Round] = true
					}
					if !rounds[0] || !rounds[1] {
						t.Fatal("source rounds collapsed", rounds)
					}
				}
			}
		})
	}
}

func a97Claims(a runtimeobs.Attempt) map[string]handoffjournal.Handoff {
	_, at := workflowFixture()
	id := a.BindingEvidence["assignment_id"]
	if id == "" {
		id = "handoff-controller"
	}
	return map[string]handoffjournal.Handoff{id: {HandoffID: id, TaskID: "T", ClaimedBy: a.AgentPath, ClaimedAttemptID: a.AttemptID, ClaimedAt: at(2).Format(time.RFC3339Nano), Target: handoffjournal.Target{AgentPath: a.AgentPath, AttemptID: a.AttemptID, RuntimeAgentID: a.RuntimeAgentID, SessionID: a.SessionID, Provider: a.Provider}}}
}

func TestA97SupportedControllerClaimAndRegistrarBinding(t *testing.T) {
	_, at := workflowFixture()
	for _, role := range []string{"controller", "registrar"} {
		t.Run(role, func(t *testing.T) {
			path := "/root/" + role
			a := a97Attempt(role, "native-"+role, path)
			a.BindingEvidence = map[string]string{"runtime_agent_id": a.RuntimeAgentID}
			a.BindingSource = "explicit"
			a.BindingAt = at(1).Format(time.RFC3339Nano)
			if role == "controller" {
				a.TaskID = ""
			}
			in := WorkflowInput{TaskID: "T", ExecutionObservations: []runtimeobs.Attempt{a}, Now: at(10)}
			if role == "controller" {
				in.Handoffs = a97Claims(a)
			}
			rows := map[string]*WorkflowRow{"path": {ID: "path", Role: role, AgentID: path, RoleFacts: []WorkflowRoleFact{{ID: "fact", AttemptID: a.AttemptID}}}, "native": {ID: "native", Role: role, AgentID: a.RuntimeAgentID, Intervals: []WorkflowInterval{{ID: "activity", AttemptID: a.AttemptID}}}}
			in.Events = []Event{{TaskID: "T", Role: Role(role), AgentID: path, RuntimeAttemptID: a.AttemptID, SessionID: a.SessionID, Provider: a.Provider}}
			out := workflowSessionRows(in, rows)
			if len(out) != 1 {
				t.Fatalf("supported schema remained split %+v", out)
			}
			for _, r := range out {
				if r.SessionIdentity.Status != "verified" || r.SessionIdentity.Attempts[0].AssignmentID != "" {
					t.Fatalf("assignment invented %+v", r)
				}
			}
			if role == "controller" {
				for _, change := range []func(*handoffjournal.Handoff){func(h *handoffjournal.Handoff) { h.TaskID = "foreign" }, func(h *handoffjournal.Handoff) { h.ClaimedAttemptID = "other" }, func(h *handoffjournal.Handoff) { h.ClaimedBy = "/root/registrar" }, func(h *handoffjournal.Handoff) { h.Target.RuntimeAgentID = "other" }, func(h *handoffjournal.Handoff) { h.Target.SessionID = "other" }, func(h *handoffjournal.Handoff) { h.Target.Provider = "other" }, func(h *handoffjournal.Handoff) { h.ClaimedAt = "" }} {
					bad := in
					bad.Handoffs = a97Claims(a)
					for k, h := range bad.Handoffs {
						change(&h)
						bad.Handoffs[k] = h
					}
					for _, r := range workflowSessionRows(bad, rows) {
						if r.SessionIdentity.Status == "verified" {
							t.Fatal("foreign/unclaimed identity accepted")
						}
					}
				}
			} else {
				bad := in
				bad.ExecutionObservations = append([]runtimeobs.Attempt(nil), in.ExecutionObservations...)
				bad.ExecutionObservations[0].TaskID = ""
				for _, r := range workflowSessionRows(bad, rows) {
					if r.SessionIdentity.Status == "verified" {
						t.Fatal("taskless Registrar accepted")
					}
				}
			}
		})
	}
}

func TestA97TasklessReaderCandidateNeedsTaskClaim(t *testing.T) {
	_, at := workflowFixture()
	project := t.TempDir()
	for _, e := range []runtimeobs.ExecutionEvent{
		{EventID: "hook", EventKind: "state", AttemptID: "controller", Provider: "codex", SessionID: "native-controller", RuntimeAgentID: "native-controller", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved, ObservedAt: at(1).Format(time.RFC3339Nano)},
		{EventID: "binding", EventKind: "binding", AttemptID: "controller", AgentPath: "/root/controller", BindingSource: "explicit", EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative, ObservedAt: at(2).Format(time.RFC3339Nano)},
	} {
		if err := runtimeobs.AppendExecutionEvent(project, e); err != nil {
			t.Fatal(err)
		}
	}
	attempts, err := ReadWorkflowExecutions(project, "T", at(10))
	if err != nil || len(attempts) != 1 {
		t.Fatalf("candidate lost: %v %+v", err, attempts)
	}
	in := WorkflowInput{TaskID: "T", ExecutionObservations: attempts, Now: at(10)}
	if got := SummarizeWorkflow(in); len(got.Rows) != 0 || len(got.RuntimeTiming.Intervals) != 0 {
		t.Fatal("candidate alone entered task projection", got)
	}
	a := attempts[0]
	in.Handoffs = a97Claims(a)
	in.Events = []Event{{EventID: "activity", TaskID: "T", Role: RoleController, AgentID: a.RuntimeAgentID, RuntimeAttemptID: a.AttemptID, HandoffID: "handoff-controller", Action: Start, Type: Handoff, OccurredAt: at(3)}}
	in.Summary = Summarize("T", in.Events)
	got := SummarizeWorkflow(in)
	found := false
	for _, row := range got.Rows {
		if row.SessionIdentity.Status == "verified" && row.SessionIdentity.NativeID == a.SessionID {
			found = true
		}
	}
	if !found {
		t.Fatal("exact claim did not connect reader candidate", got)
	}
}

func TestA97SessionRoundProvenanceWithResumedAttempts(t *testing.T) {
	a := a97Attempt("one", "native", "/root/controller/kkobugi")
	b := a97Attempt("two", "native", "/root/controller/kkobugi")
	n0, n1 := 0, 1
	rows := map[string]*WorkflowRow{
		"first":   {ID: "first", Role: "worker", AgentID: a.AgentPath, Round: &n0, RoundStatus: "initial", Intervals: []WorkflowInterval{{ID: "activity-before", AttemptID: a.AttemptID, AssignmentID: "assignment-one"}}, RoleFacts: []WorkflowRoleFact{{ID: "fact-before", AttemptID: a.AttemptID, AssignmentID: "assignment-one"}}},
		"resumed": {ID: "resumed", Role: "worker", AgentID: b.RuntimeAgentID, Round: &n1, RoundStatus: "rework", Intervals: []WorkflowInterval{{ID: "activity-after", AttemptID: b.AttemptID, AssignmentID: "assignment-two"}}, RoleFacts: []WorkflowRoleFact{{ID: "fact-after", AttemptID: b.AttemptID, AssignmentID: "assignment-two"}}},
	}
	before, _ := json.Marshal(rows)
	out := workflowSessionRows(WorkflowInput{TaskID: "T", ExecutionObservations: []runtimeobs.Attempt{a, b}}, rows)
	if len(out) != 1 {
		t.Fatalf("resume split session %+v", out)
	}
	for _, r := range out {
		if len(r.Intervals) != 2 || len(r.RoleFacts) != 2 || len(r.SessionIdentity.Attempts) != 2 || len(r.Rounds) != 2 || r.Round != nil {
			t.Fatal("lost source provenance", r)
		}
		for _, v := range r.Intervals {
			if v.Round == nil || (*v.Round == 0 && v.ID != "activity-before") || (*v.Round == 1 && v.ID != "activity-after") {
				t.Fatal("wrong interval round", v)
			}
		}
		for _, v := range r.RoleFacts {
			if v.Round == nil || (*v.Round == 0 && v.ID != "fact-before") || (*v.Round == 1 && v.ID != "fact-after") {
				t.Fatal("wrong fact round", v)
			}
		}
		for _, v := range r.SessionIdentity.Attempts {
			if len(v.Rounds) != 1 || v.Rounds[0].Round == nil || (*v.Rounds[0].Round == 0 && v.AttemptID != "one") || (*v.Rounds[0].Round == 1 && v.AttemptID != "two") {
				t.Fatal("wrong attempt round", v)
			}
		}
	}
	after, _ := json.Marshal(rows)
	if string(before) != string(after) {
		t.Fatal("input source mutated")
	}
}

func TestA97ExplicitHandoffConstrainsEverySessionRole(t *testing.T) {
	_, at := workflowFixture()
	for _, path := range []string{"/root/controller/kkobugi", "/root/controller/reviewer/pikachyu", "/root/controller"} {
		t.Run(path, func(t *testing.T) {
			a := a97Attempt("one", "native-one", path)
			for _, tc := range []struct {
				name   string
				change func(*WorkflowInput, *handoffjournal.Handoff, *string)
				want   bool
			}{
				{"exactClaim", nil, true},
				{"foreignTask", func(_ *WorkflowInput, h *handoffjournal.Handoff, _ *string) { h.TaskID = "FOREIGN" }, false},
				{"wrongAssignment", func(_ *WorkflowInput, _ *handoffjournal.Handoff, assignment *string) {
					*assignment = "foreign-assignment"
				}, false},
				{"wrongNative", func(_ *WorkflowInput, h *handoffjournal.Handoff, _ *string) {
					h.Target.RuntimeAgentID = "foreign-native"
				}, false},
				{"wrongSession", func(_ *WorkflowInput, h *handoffjournal.Handoff, _ *string) { h.Target.SessionID = "foreign-session" }, false},
				{"wrongProvider", func(_ *WorkflowInput, h *handoffjournal.Handoff, _ *string) { h.Target.Provider = "foreign-provider" }, false},
				{"wrongActor", func(_ *WorkflowInput, h *handoffjournal.Handoff, _ *string) { h.Target.AgentPath = "/root/registrar" }, false},
				{"wrongAttempt", func(_ *WorkflowInput, h *handoffjournal.Handoff, _ *string) { h.Target.AttemptID = "foreign-attempt" }, false},
				{"wrongClaim", func(_ *WorkflowInput, h *handoffjournal.Handoff, _ *string) { h.ClaimedAttemptID = "foreign-attempt" }, false},
				{"missingClaimTime", func(_ *WorkflowInput, h *handoffjournal.Handoff, _ *string) { h.ClaimedAt = "" }, false},
				{"futureClaim", func(_ *WorkflowInput, h *handoffjournal.Handoff, _ *string) {
					h.ClaimedAt = at(20).Format(time.RFC3339Nano)
				}, false},
				{"contradictoryID", func(_ *WorkflowInput, h *handoffjournal.Handoff, _ *string) { h.HandoffID = "foreign-id" }, false},
				{"duplicateReceipt", func(in *WorkflowInput, h *handoffjournal.Handoff, _ *string) {
					other := *h
					other.TaskID = "FOREIGN"
					in.Handoffs["duplicate"] = other
				}, false},
				{"sourceCannotOverrideContradictoryRecipient", func(_ *WorkflowInput, h *handoffjournal.Handoff, _ *string) {
					h.SourceAgentPath, h.SourceAttemptID = a.AgentPath, a.AttemptID
					h.Target.SessionID = "foreign-session"
				}, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					assignment := a.BindingEvidence["assignment_id"]
					h := a97Claims(a)[assignment]
					h.HandoffID = "explicit-handoff"
					in := WorkflowInput{TaskID: "T", Now: at(10), ExecutionObservations: []runtimeobs.Attempt{a}, Handoffs: map[string]handoffjournal.Handoff{}}
					if tc.change != nil {
						tc.change(&in, &h, &assignment)
					}
					in.Handoffs["explicit-handoff"] = h
					rows := map[string]*WorkflowRow{"source": {ID: "source", Role: workflowActorRole(path), AgentID: path,
						Intervals: []WorkflowInterval{{ID: "activity", AttemptID: a.AttemptID, AssignmentID: assignment, HandoffID: "explicit-handoff"}},
						RoleFacts: []WorkflowRoleFact{{ID: "fact", AttemptID: a.AttemptID, AssignmentID: assignment, HandoffID: "explicit-handoff"}}}}
					before, _ := json.Marshal(rows)
					for _, row := range workflowSessionRows(in, rows) {
						if got := row.SessionIdentity.Status == "verified"; got != tc.want {
							t.Fatalf("verified=%v want=%v: %+v", got, tc.want, row)
						}
						if len(row.Intervals) != 1 || len(row.RoleFacts) != 1 {
							t.Fatal("source evidence lost", row)
						}
					}
					after, _ := json.Marshal(rows)
					if string(before) != string(after) {
						t.Fatal("raw source mutated")
					}
				})
			}
		})
	}
}

func TestA97SourceHandoffKeepsCounterpartDistinct(t *testing.T) {
	for _, path := range []string{"/root/controller/kkobugi", "/root/controller/reviewer/pikachyu"} {
		t.Run(path, func(t *testing.T) {
			a := a97Attempt("one", "native-one", path)
			h := handoffjournal.Handoff{HandoffID: "source-handoff", TaskID: "T", SourceAgentPath: path, SourceAttemptID: a.AttemptID,
				Target: handoffjournal.Target{AgentPath: "/root/controller", AttemptID: "counterpart-attempt", RuntimeAgentID: "counterpart-native", SessionID: "counterpart-session", Provider: "codex"}}
			for _, tc := range []struct {
				name   string
				change func(*handoffjournal.Handoff)
				want   bool
			}{
				{"exactSourceDifferentRecipient", nil, true},
				{"foreignTask", func(h *handoffjournal.Handoff) { h.TaskID = "FOREIGN" }, false},
				{"wrongSourceActor", func(h *handoffjournal.Handoff) { h.SourceAgentPath = "/root/registrar" }, false},
				{"wrongSourceAttempt", func(h *handoffjournal.Handoff) { h.SourceAttemptID = "foreign-attempt" }, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					copy := h
					if tc.change != nil {
						tc.change(&copy)
					}
					in := WorkflowInput{TaskID: "T", ExecutionObservations: []runtimeobs.Attempt{a}, Handoffs: map[string]handoffjournal.Handoff{h.HandoffID: copy}}
					row := &WorkflowRow{ID: "source", Role: workflowActorRole(path), AgentID: path, Intervals: []WorkflowInterval{{ID: "activity", AttemptID: a.AttemptID, HandoffID: h.HandoffID}}}
					for _, got := range workflowSessionRows(in, map[string]*WorkflowRow{"source": row}) {
						if (got.SessionIdentity.Status == "verified") != tc.want {
							t.Fatal("source/counterpart association incorrect", got)
						}
					}
				})
			}
			deleteSource := WorkflowInput{TaskID: "T", ExecutionObservations: []runtimeobs.Attempt{a}}
			if workflowSessionAssociation(deleteSource, a, a.BindingEvidence["assignment_id"], h.HandoffID) {
				t.Fatal("missing explicit receipt accepted")
			}
		})
	}
}

func TestA102NativeFactJoinDoesNotAuthorizeActivityOrReview(t *testing.T) {
	_, at := workflowFixture()
	makeAttempt := func() runtimeobs.Attempt {
		a := a97Attempt("native-episode", "actual-native", "/root/controller")
		a.TurnID = "provider-attempt"
		a.BindingSource = "dispatch"
		a.StateEvidenceSource = runtimeobs.EvidenceT3
		a.NativeReceipt = &runtimeobs.T3Receipt{Origin: "t3_local_projection_v2", Provider: a.Provider, SessionID: a.SessionID, ThreadID: "actual-thread", RunID: "actual-run", NativeAttemptID: a.TurnID, State: a.CurrentState, ObservedAt: a.LastObservedAt, StartedAt: a.StartedAt}
		return a
	}
	for _, kind := range []string{"valid", "wrong_task", "wrong_assignment", "wrong_native", "taskless_global", "conflicting_handoff", "wrong_source"} {
		t.Run(kind, func(t *testing.T) {
			a := makeAttempt()
			assignment := a.BindingEvidence["assignment_id"]
			handoff := ""
			source := string(runtimeobs.EvidenceT3)
			switch kind {
			case "wrong_task":
				a.TaskID = "other"
			case "wrong_assignment":
				assignment = "other"
			case "wrong_native":
				a.RuntimeAgentID = "other"
			case "taskless_global":
				a.TaskID = ""
			case "conflicting_handoff":
				handoff = "foreign"
			case "wrong_source":
				source = "unverified"
			}
			in := WorkflowInput{TaskID: "T", ExecutionObservations: []runtimeobs.Attempt{a}, Now: at(10)}
			rows := map[string]*WorkflowRow{"native": {ID: "native", Role: "controller", AgentID: "actual-native", Intervals: []WorkflowInterval{{ID: "native", Kind: "native_observed", AttemptID: a.AttemptID, AssignmentID: assignment, HandoffID: handoff, Evidence: source, Start: at(1)}}, RoleFacts: []WorkflowRoleFact{{ID: "native-fact", Kind: "observed_execution", AttemptID: a.AttemptID, AssignmentID: assignment, HandoffID: handoff, Source: source, At: at(1)}}}, "activity": {ID: "activity", Role: "controller", AgentID: "actual-native", Intervals: []WorkflowInterval{{ID: "activity", Kind: "activity_open", AttemptID: a.AttemptID, AssignmentID: assignment, Start: at(2)}}}}
			out := workflowSessionRows(in, rows)
			verified := 0
			for _, r := range out {
				if r.SessionIdentity.Status == "verified" {
					verified++
					if len(r.Intervals) != 1 || r.Intervals[0].Kind != "native_observed" || len(r.RoleFacts) != 1 {
						t.Fatal("Activity joined native fact authority", r)
					}
				}
			}
			want := 0
			if kind == "valid" {
				want = 1
			}
			if verified != want {
				t.Fatalf("%s verified=%d want=%d", kind, verified, want)
			}
			if workflowSessionAssociation(in, a, assignment, handoff) {
				t.Fatal("native join upgraded Controller claim/Activity authority")
			}
		})
	}
}
