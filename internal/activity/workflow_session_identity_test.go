package activity

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func a97Attempt(id, native, path string) runtimeobs.Attempt {
	_, at := workflowFixture()
	return runtimeobs.Attempt{AttemptID: id, TaskID: "T", AgentPath: path, RuntimeAgentID: native, SessionID: native, Provider: "codex", BindingState: runtimeobs.BindingBound, BindingEvidence: map[string]string{"assignment_id": "assignment-" + id, "runtime_agent_id": native}, StartedAt: at(1).Format(time.RFC3339Nano), LastObservedAt: at(8).Format(time.RFC3339Nano), StateEvidenceSource: runtimeobs.EvidenceHook, StateObservationQuality: runtimeobs.QualityObserved, CurrentState: runtimeobs.StateRunning}
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
