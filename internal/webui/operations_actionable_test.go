package webui

import (
	stdcontext "context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/notify"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// Temp-only typed ownership/assignment/runtime fixture. Only the bounded
// official observation is mocked; canonical selection and identity checks use
// the same loader as production. No host, native Activity or real delivery.
func a83TypedActionFixture(t *testing.T, project, task, reason, next string, state runtimeobs.CanonicalState) {
	t.Helper()
	now := time.Now().UTC()
	native, assignment, attempt := "fixture-native-"+task, "fixture-assignment-"+task, "fixture-attempt-"+task
	p := rolepolicy.Policy{SchemaVersion: 1, Method: "worker", Workspace: project, Scope: []string{"fixture.go"},
		Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/fixture", NativeID: native},
		Operational:    rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"},
		Reviewer:       rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/reviewer", NativeID: "fixture-reviewer-" + task}}
	policy, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	raw := "# " + task + " Human action fixture\n- Agent: /root/controller/fixture\n- 대기유형: user\n- 대기: " + reason + "\n- 재개조건: " + next + "\n\n## 작업 정의\n### 목표\nFixture only\n" + rolepolicy.Fence + string(policy) + "\n```\n"
	if err := os.WriteFile(filepath.Join(folder, "000001."+task+".fixture.hold.md"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.RecordAssignment(project, assignment, task, p.Implementation.AgentPath, now.Add(-2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	r := &runtimeobs.T3Receipt{CanonicalProject: project, ThreadID: "fixture-thread-" + task, RunID: "fixture-run-" + task, NativeAttemptID: "fixture-episode-" + task, SessionID: native, Provider: "codex", State: state, StartedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), ObservedAt: now.Format(time.RFC3339Nano), Origin: "t3_local_projection_v2"}
	terminal := state == runtimeobs.StateErrored || state == runtimeobs.StateInterrupted || state == runtimeobs.StateShutdown || state == runtimeobs.StateCompleted
	if terminal {
		r.CompletedAt = now.Format(time.RFC3339Nano)
	}
	if state != runtimeobs.StateRunning {
		start := *r
		start.State, start.ObservedAt, start.CompletedAt = runtimeobs.StateRunning, r.StartedAt, ""
		if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", AttemptID: attempt, Provider: "codex", SessionID: native, RuntimeAgentID: native, TurnID: r.NativeAttemptID, ObservedAt: start.ObservedAt, State: start.State, EvidenceSource: runtimeobs.EvidenceT3, ObservationQuality: runtimeobs.QualityObserved, NativeReceipt: &start}); err != nil {
			t.Fatal(err)
		}
	}
	if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", AttemptID: attempt, Provider: "codex", SessionID: native, RuntimeAgentID: native, TurnID: r.NativeAttemptID, ObservedAt: r.ObservedAt, State: state, Terminal: terminal, EvidenceSource: runtimeobs.EvidenceT3, ObservationQuality: runtimeobs.QualityObserved, NativeReceipt: r}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.BindAttempt(project, attempt, task, p.Implementation.AgentPath, "explicit", "", map[string]string{"assignment_id": assignment, "runtime_agent_id": native}, now); err != nil {
		t.Fatal(err)
	}
}

func installA83MockCurrentExecution(t *testing.T) {
	t.Helper()
	previous := operationExecutionLoader
	t.Cleanup(func() { operationExecutionLoader = previous })
	operationExecutionLoader = func(project string, ledger runtimeobs.Ledger, now time.Time, refresh bool) map[string]runtimeobs.CurrentExecutionSnapshot {
		rows, available := operationCanonicalRows(project)
		assignments, err := runtimeobs.ListAssignments(project)
		return loadCurrentExecutionEvidence(stdcontext.Background(), project, ledger, rows, available, assignments, err, now, refresh, rolepolicy.Load,
			func(_ stdcontext.Context, s runtimeobs.CurrentExecutionSnapshot, at time.Time) runtimeobs.CurrentExecutionSnapshot {
				if !s.Matched || s.Receipt == nil || s.Identity.Project != project || s.Identity.NativeID != s.SessionID || s.Identity.NativeID != s.RuntimeAgentID {
					t.Fatal("mock reached without exact current typed fixture", s)
				}
				s.Fresh, s.OfficialCurrent = true, true
				s.ObservedAt = at.Format(time.RFC3339Nano)
				s.FailedStage, s.ActionOwner, s.Limitation = "", "", "mock official read; fixture only"
				return s
			})
	}
}

func TestA83TypedCurrentHumanActionRejectsWrongNativeAndUnsupportedObservation(t *testing.T) {
	project := testOperationProject(t)
	a83TypedActionFixture(t, project, "A-83", "Owner access required", "Owner: grant requested access", runtimeobs.StateRunning)
	installA83MockCurrentExecution(t)
	now := time.Now().UTC()
	ledger, err := runtimeobs.BuildLedger(project, 3, now)
	if err != nil {
		t.Fatal(err)
	}
	valid := operationExecutionLoader(project, ledger, now, true)["A-83"]
	if !valid.Matched || !valid.OfficialCurrent || valid.Identity.NativeID != "fixture-native-A-83" || valid.Identity.AssignmentID != "fixture-assignment-A-83" {
		t.Fatal("typed fixture failed to establish exact current evidence", valid)
	}
	j, err := scanOperationProject(project, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(j.Actions) != 1 {
		t.Fatal("missing explicit human action", j)
	}
	id := j.Actions[0].ID
	current := operationExecutionLoader
	operationExecutionLoader = func(project string, ledger runtimeobs.Ledger, at time.Time, refresh bool) map[string]runtimeobs.CurrentExecutionSnapshot {
		result := current(project, ledger, at, refresh)
		if refresh {
			s := result["A-83"]
			s.OfficialCurrent = false
			s.Fresh = false
			s.FailedStage = "official_observation"
			s.ActionOwner = "/root/controller"
			s.Limitation = "mock unsupported observation"
			result["A-83"] = s
		}
		return result
	}
	oldDeliver := operationDeliver
	defer func() { operationDeliver = oldDeliver }()
	sent := 0
	operationDeliver = func(_ string, events []notify.Event) []error { sent += len(events); return nil }
	deliverOperationIncidents(project, j)
	if sent != 0 {
		t.Fatal("unsupported observation externally delivered")
	}
	stored, err := readOperationJournal(project)
	if err != nil || len(stored.Actions) != 1 || stored.Actions[0].ID != id || stored.Actions[0].ResolvedAt != "" {
		t.Fatal("collector trouble lost real unresolved cause", stored, err)
	}
	operationExecutionLoader = current
	path := filepath.Join(project, "_task_mecca/data/backlog/000001.A-83.fixture.hold.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the same legacy Agent; it cannot replace a changed typed native.
	changed := strings.Replace(string(raw), "fixture-native-A-83", "different-current-typed-native", 1)
	if changed == string(raw) {
		t.Fatal("fixture typed native not found")
	}
	if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	wrong := operationExecutionLoader(project, ledger, time.Now().UTC(), true)["A-83"]
	if wrong.Matched || wrong.OfficialCurrent || wrong.FailedStage == "" {
		t.Fatal("legacy Agent or old native substituted current typed owner", wrong)
	}
	deliverOperationIncidents(project, j)
	if sent != 0 {
		t.Fatal("wrong current typed native externally delivered")
	}
}

// Sanitized, source-derived six distinct B644 episodes, not six retries.
// Source: 20261117-B644-stalled-notification-trigger-audit.json, source commit
// 7db135d7. Original incident IDs/timestamps/roles are retained; no live ledger
// or notification transport is read or changed by these fixtures.
func TestA83B644SixNoSignalEpisodesAreDiagnostic(t *testing.T) {
	fixtures := []struct{ id, attempt, agent, last, detected string }{
		{"ops-69056a09cd383f5f", "run-99d66fac64ac40b7", "/root/controller", "2026-11-17T03:00:25.968429Z", "2026-11-17T03:30:37.488266Z"},
		{"ops-2e4cc1d43705681a", "run-04f6de3f1fbe8006", "/root/controller/compact_spec", "2026-11-17T03:10:01.257929Z", "2026-11-17T03:40:07.491567Z"},
		{"ops-eb338700769b81ce", "run-d5d40e93ce02d064", "/root/controller/r2_baseline", "2026-11-17T03:09:53.353518Z", "2026-11-17T03:40:07.491567Z"},
		{"ops-1abdd861716efc7a", "run-2c02866237365db8", "/root/controller/common_wf", "2026-11-17T03:10:08.803415Z", "2026-11-17T03:40:22.491449Z"},
		{"ops-7085bc5a3a31619a", "run-a90499c7304152df", "/root/controller/compact_spec", "2026-11-17T03:50:12.107316Z", "2026-11-17T04:20:24.497708Z"},
		{"ops-dc9809d902f3231c", "run-de377b3d3f0c8a24", "/root/controller/common_wf", "2026-11-17T03:50:17.397715Z", "2026-11-17T04:20:24.497708Z"},
	}
	t.Setenv("TASK_MECCA_STALE_WARN_SECONDS", "1800")
	for _, f := range fixtures {
		t.Run(f.id, func(t *testing.T) {
			now, _ := time.Parse(time.RFC3339Nano, f.detected)
			a := runtimeobs.Attempt{AttemptID: f.attempt, TaskID: "B-644", AgentPath: f.agent, CurrentState: runtimeobs.StateRunning, StateEvidenceSource: runtimeobs.EvidenceHook, StateObservationQuality: runtimeobs.QualityObserved, LastObservedAt: f.last}
			s, ok := classifyOperation("/fixture/empfund", a, now, false)
			if !ok || s.incident.Kind != "no_signal" || s.incident.LastObservedAt != f.last || s.incident.Quality != "verification_required" {
				t.Fatalf("source diagnostics lost %+v", s)
			}
			if s.alert || operationNotificationKind(s.incident) != "" {
				t.Fatalf("observation gap became external stalled: %+v", s)
			}
		})
	}
}

func a83HumanRows(project string) map[string][]backlog.Record {
	return map[string][]backlog.Record{"A-83": {{ID: "A-83", Location: "active", State: "hold", Path: filepath.Join(project, "_task_mecca/data/backlog/task.md"), Mtime: time.Now().UTC().Format(time.RFC3339Nano), Fields: map[string]string{"대기유형": "user", "대기": "Repository access is required.", "재개조건": "Repository owner: grant repository access."}}}}
}

func a83Evidence(now time.Time, state runtimeobs.CanonicalState) map[string]runtimeobs.CurrentExecutionSnapshot {
	return map[string]runtimeobs.CurrentExecutionSnapshot{"A-83": {Identity: runtimeobs.CurrentExecutionIdentity{Project: "/fixture/project", TaskID: "A-83", AgentPath: "/root/controller/worker", NativeID: "native", AssignmentID: "assignment", Provider: "codex"}, AttemptID: "actual-current", Matched: true, Fresh: true, OfficialCurrent: true, ObservedAt: now.Format(time.RFC3339Nano), State: state}}
}

func TestA83ActionDedupAcrossSourcesEpisodesAndResolveNewIncident(t *testing.T) {
	now := time.Now().UTC()
	rows := a83HumanRows("/fixture/project")
	j := operationJournal{Project: "/fixture/project", Incidents: []operationIncident{{ID: "controller-episode", TaskID: "A-83"}, {ID: "worker-episode", TaskID: "A-83"}}}
	original := append([]operationIncident(nil), j.Incidents...)
	reconcileOperationActions(&j, rows, true, now)
	if len(j.Actions) != 1 || len(j.Actions[0].SourceIncidentIDs) != 2 {
		t.Fatalf("source episodes not aggregated %+v", j)
	}
	id := j.Actions[0].ID
	j.Incidents = append(j.Incidents, operationIncident{ID: "resumed-native-episode", TaskID: "A-83"})
	reconcileOperationActions(&j, rows, true, now.Add(time.Second))
	if len(j.Actions) != 1 || j.Actions[0].ID != id || len(j.Actions[0].SourceIncidentIDs) != 3 || !reflect.DeepEqual(original, j.Incidents[:2]) {
		t.Fatal("resume duplicated action or altered original source")
	}
	// A read outage is not a resolution; no new action ID when it recovers.
	reconcileOperationActions(&j, nil, false, now.Add(2*time.Second))
	reconcileOperationActions(&j, rows, true, now.Add(3*time.Second))
	if j.Actions[0].ResolvedAt != "" || len(j.Actions) != 1 {
		t.Fatal("collector outage resolved human reason")
	}
	rows["A-83"][0].State = "doing"
	rows["A-83"][0].Fields["대기"] = ""
	reconcileOperationActions(&j, rows, true, now.Add(4*time.Second))
	if j.Actions[0].ResolvedAt == "" {
		t.Fatal("explicit resolution not recorded")
	}
	rows = a83HumanRows("/fixture/project")
	reconcileOperationActions(&j, rows, true, now.Add(5*time.Second))
	if len(j.Actions) != 2 || j.Actions[1].ID == id {
		t.Fatal("resolve then new occurrence suppressed")
	}
	rows["A-83"][0].Fields["대기"] = "Different human action: expired signing certificate"
	reconcileOperationActions(&j, rows, true, now.Add(6*time.Second))
	if len(j.Actions) != 3 || j.Actions[1].ResolvedAt == "" || j.Actions[2].ID == j.Actions[1].ID {
		t.Fatal("new real reason suppressed")
	}
}

func TestA83FreshRunningDoesNotHideRealHumanBlocker(t *testing.T) {
	now := time.Now().UTC()
	rows := a83HumanRows("/fixture/project")
	j := operationJournal{Project: "/fixture/project"}
	reconcileOperationActions(&j, rows, true, now)
	for _, state := range []runtimeobs.CanonicalState{runtimeobs.StateRunning, runtimeobs.StateWaitingApproval, runtimeobs.StateWaitingUser, runtimeobs.StateErrored, runtimeobs.StateInterrupted} {
		if _, ok := operationActionEligible(j.Actions[0], rows, true, a83Evidence(now, state)); !ok {
			t.Fatalf("real blocker hidden by %s", state)
		}
	}
	for _, bad := range []string{"stale", "unsupported", "terminal", "reason cleared", "next action missing", "normal recovery"} {
		t.Run(bad, func(t *testing.T) {
			r := a83HumanRows("/fixture/project")
			evidence := a83Evidence(now, runtimeobs.StateRunning)
			s := evidence["A-83"]
			switch bad {
			case "stale":
				s.ObservedAt = now.Add(-3 * time.Minute).Format(time.RFC3339Nano)
			case "unsupported":
				s.OfficialCurrent = false
				s.FailedStage = "official_observation"
			case "terminal":
				s.State = runtimeobs.StateCompleted
			case "reason cleared":
				r["A-83"][0].Fields["대기"] = ""
			case "next action missing":
				r["A-83"][0].Fields["재개조건"] = ""
			case "normal recovery":
				r["A-83"][0].Fields["대기유형"] = "recovery"
			}
			evidence["A-83"] = s
			if _, ok := operationActionEligible(j.Actions[0], r, true, evidence); ok {
				t.Fatal("diagnostic/cleared cause externally eligible")
			}
		})
	}
}

func TestA83LongComputeCollectorTroubleAndRecoveryNeverCreateHumanActions(t *testing.T) {
	now := time.Now().UTC()
	for _, diagnostic := range []operationIncident{
		{ID: "long-compute", Kind: "no_signal", Quality: "verification_required", LastObservedAt: now.Add(-72 * time.Hour).Format(time.RFC3339Nano)},
		{ID: "collector-error", Kind: "runtime_unknown", Quality: "verification_required", Evidence: "collector read failed; execution unconfirmed"},
		{ID: "controller-recovery", Kind: "handoff_stalled", Quality: "verification_required", Evidence: "supported Controller recovery in progress"},
	} {
		t.Run(diagnostic.ID, func(t *testing.T) {
			rows := a83HumanRows("/fixture/project")
			rows["A-83"][0].Fields["대기유형"] = "recovery"
			diagnostic.TaskID = "A-83"
			j := operationJournal{Project: "/fixture/project", Incidents: []operationIncident{diagnostic}, ExecutionEvidence: a83Evidence(now, runtimeobs.StateRunning)}
			reconcileOperationActions(&j, rows, true, now)
			if len(j.Actions) != 0 || operationNotificationKind(diagnostic) != "" || j.Incidents[0].ID != diagnostic.ID {
				t.Fatal("diagnostic fabricated a human action or lost source")
			}
		})
	}
}

func TestA83ExplicitHumanCausePreservesApprovalAndErrorNotificationMeaning(t *testing.T) {
	project := testOperationProject(t)
	path := filepath.Join(project, "_task_mecca/data/backlog/000083.A-83.fixture.hold.md")
	if err := os.WriteFile(path, []byte("# A-83 fixture\n- 대기유형: user\n- 대기: Owner access approval required\n- 재개조건: Owner: approve requested access\n"), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	rows, available := operationCanonicalRows(project)
	j := operationJournal{Project: project}
	reconcileOperationActions(&j, rows, available, now)
	for state, kind := range map[runtimeobs.CanonicalState]string{runtimeobs.StateWaitingApproval: "approval", runtimeobs.StateInterrupted: "interrupted", runtimeobs.StateErrored: "interrupted", runtimeobs.StateRunning: "intervention"} {
		j.ExecutionEvidence = a83Evidence(now, state)
		events := operationActionableIncidents(j)
		if len(events) != 1 || events[0].Kind != kind || events[0].Message != "Owner access approval required" || events[0].ResumeCondition != "Owner: approve requested access" {
			t.Fatalf("actual %s action semantics lost %+v", state, events)
		}
	}
}

func TestA83DeliveryRevalidatesResolutionStateAndIdentityAndRetryID(t *testing.T) {
	for _, race := range []string{"none", "resolved", "new reason", "native changed", "collector failure"} {
		t.Run(race, func(t *testing.T) {
			project := testOperationProject(t)
			now := time.Now().UTC()
			path := filepath.Join(project, "_task_mecca/data/backlog/000083.A-83.fixture.hold.md")
			write := func(reason string) {
				t.Helper()
				body := "# A-83 fixture\n- Agent: /root/controller/worker\n- 대기유형: user\n- 대기: " + reason + "\n- 재개조건: Repository owner: grant repository access.\n"
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write("Repository access is required.")
			rows, available := operationCanonicalRows(project)
			if !available {
				t.Fatal("fixture catalog unavailable")
			}
			j := operationJournal{Project: project, Version: 1, Incidents: []operationIncident{{ID: "source-event", TaskID: "A-83", Kind: "no_signal", Quality: "verification_required"}}}
			reconcileOperationActions(&j, rows, true, now)
			if len(j.Actions) != 1 {
				t.Fatalf("missing explicit reason %+v", rows)
			}
			if err := writeOperationJournal(project, j); err != nil {
				t.Fatal(err)
			}
			oldLoader, oldDeliver := operationExecutionLoader, operationDeliver
			defer func() { operationExecutionLoader = oldLoader; operationDeliver = oldDeliver }()
			operationExecutionLoader = func(_ string, _ runtimeobs.Ledger, at time.Time, refresh bool) map[string]runtimeobs.CurrentExecutionSnapshot {
				e := a83Evidence(at, runtimeobs.StateRunning)
				if refresh {
					switch race {
					case "resolved":
						write("")
					case "new reason":
						write("Different access approval needed")
					case "collector failure":
						s := e["A-83"]
						s.OfficialCurrent = false
						s.FailedStage = "collector"
						e["A-83"] = s
					}
				}
				if !refresh && race == "native changed" {
					s := e["A-83"]
					s.Identity.NativeID = "new-native"
					e["A-83"] = s
				}
				return e
			}
			var events []notify.Event
			sent := map[string]bool{}
			delivered := 0
			operationDeliver = func(_ string, batch []notify.Event) []error {
				for _, e := range batch {
					events = append(events, e)
					if !sent[e.ID] {
						sent[e.ID] = true
						delivered++
					}
				}
				return nil
			}
			deliverOperationIncidents(project, j)
			if race == "none" {
				deliverOperationIncidents(project, j) // retry/restart uses durable reason ID
				if len(events) != 2 || delivered != 1 || events[0].ID != events[1].ID || events[0].Message == "" || events[0].ResumeCondition == "" {
					t.Fatalf("retry identity/action lost %+v", events)
				}
			} else if len(events) != 0 {
				t.Fatalf("racing %s delivered obsolete cause %+v", race, events)
			}
			stored, err := readOperationJournal(project)
			if err != nil || len(stored.Incidents) != 1 || stored.Incidents[0].ID != "source-event" {
				t.Fatal("source audit altered")
			}
		})
	}
}

func TestA83IndependentDeliveryRejectsLatestCompletedState(t *testing.T) {
	project := testOperationProject(t)
	path := filepath.Join(project, "_task_mecca/data/backlog/000083.A-83.fixture.hold.md")
	if err := os.WriteFile(path, []byte("# A-83 fixture\n- Agent: /root/controller/worker\n- 대기유형: user\n- 대기: Repository access is required.\n- 재개조건: Repository owner: grant repository access.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	rows, available := operationCanonicalRows(project)
	j := operationJournal{Project: project, Version: 1}
	reconcileOperationActions(&j, rows, available, time.Now().UTC())
	if len(j.Actions) != 1 {
		t.Fatal("fixture action unavailable")
	}
	if err := writeOperationJournal(project, j); err != nil {
		t.Fatal(err)
	}
	oldLoader, oldDeliver := operationExecutionLoader, operationDeliver
	defer func() { operationExecutionLoader = oldLoader; operationDeliver = oldDeliver }()
	operationExecutionLoader = func(_ string, _ runtimeobs.Ledger, at time.Time, refresh bool) map[string]runtimeobs.CurrentExecutionSnapshot {
		state := runtimeobs.StateRunning
		if !refresh {
			state = runtimeobs.StateCompleted
		}
		return a83Evidence(at, state)
	}
	delivered := 0
	operationDeliver = func(_ string, events []notify.Event) []error { delivered += len(events); return nil }
	deliverOperationIncidents(project, j)
	if delivered != 0 {
		t.Fatalf("delivered %d obsolete action despite latest same-identity/attempt state=completed", delivered)
	}
}

func TestA83DeliveryRequiresStableLatestExecutionState(t *testing.T) {
	for _, tc := range []struct {
		name             string
		official, latest runtimeobs.CanonicalState
		wantKind         string
	}{
		{"completed during boundary", runtimeobs.StateRunning, runtimeobs.StateCompleted, ""},
		{"error during boundary", runtimeobs.StateRunning, runtimeobs.StateErrored, ""},
		{"interrupted during boundary", runtimeobs.StateRunning, runtimeobs.StateInterrupted, ""},
		{"approval during boundary", runtimeobs.StateRunning, runtimeobs.StateWaitingApproval, ""},
		{"unknown during boundary", runtimeobs.StateRunning, runtimeobs.StateRuntimeUnknown, ""},
		{"approval resumed", runtimeobs.StateWaitingApproval, runtimeobs.StateRunning, ""},
		{"error recovered", runtimeobs.StateErrored, runtimeobs.StateRunning, ""},
		{"stable running real blocker", runtimeobs.StateRunning, runtimeobs.StateRunning, "intervention"},
		{"stable confirmed error", runtimeobs.StateErrored, runtimeobs.StateErrored, "interrupted"},
		{"stable confirmed interruption", runtimeobs.StateInterrupted, runtimeobs.StateInterrupted, "interrupted"},
		{"stable approval", runtimeobs.StateWaitingApproval, runtimeobs.StateWaitingApproval, "approval"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := testOperationProject(t)
			path := filepath.Join(project, "_task_mecca/data/backlog/000083.A-83.fixture.hold.md")
			if err := os.WriteFile(path, []byte("# A-83 fixture\n- Agent: /root/controller/worker\n- 대기유형: user\n- 대기: Owner access approval required\n- 재개조건: Owner: approve requested access\n"), 0600); err != nil {
				t.Fatal(err)
			}
			rows, available := operationCanonicalRows(project)
			original := operationIncident{ID: "original-source", TaskID: "A-83", Kind: "no_signal", Quality: "verification_required", Severity: "warning", Evidence: "original observation gap"}
			j := operationJournal{Project: project, Version: 1, Incidents: []operationIncident{original}}
			reconcileOperationActions(&j, rows, available, time.Now().UTC())
			if len(j.Actions) != 1 {
				t.Fatal("fixture human cause unavailable")
			}
			actionID := j.Actions[0].ID
			if err := writeOperationJournal(project, j); err != nil {
				t.Fatal(err)
			}
			oldLoader, oldDeliver := operationExecutionLoader, operationDeliver
			defer func() { operationExecutionLoader = oldLoader; operationDeliver = oldDeliver }()
			operationExecutionLoader = func(_ string, _ runtimeobs.Ledger, at time.Time, refresh bool) map[string]runtimeobs.CurrentExecutionSnapshot {
				state := tc.latest
				if refresh {
					state = tc.official
				}
				return a83Evidence(at, state) // identical identity and attempt on both reads
			}
			var events []notify.Event
			operationDeliver = func(_ string, batch []notify.Event) []error { events = append(events, batch...); return nil }
			deliverOperationIncidents(project, j)
			if tc.wantKind == "" {
				if len(events) != 0 {
					t.Fatalf("state changed %s -> %s but sent obsolete action %+v", tc.official, tc.latest, events)
				}
			} else if len(events) != 1 || events[0].Kind != tc.wantKind || events[0].ID == "" || events[0].Message == "" || events[0].ResumeCondition == "" {
				t.Fatalf("stable real human action semantics lost: %+v", events)
			}

			if tc.wantKind != "" {
				decisions, gateErr := currentNotificationDecisions(project, "")
				if gateErr != nil || decisions["A-83"].EventID != events[0].ID {
					t.Fatalf("delivery lacks shared current episode ID: %+v %v", events, gateErr)
				}
				deliverOperationIncidents(project, j)
				if len(events) != 2 || events[0].ID != events[1].ID {
					t.Fatalf("retry changed transport idempotency key: %+v", events)
				}
			}
			stored, err := readOperationJournal(project)
			if err != nil || len(stored.Incidents) != 1 || !reflect.DeepEqual(stored.Incidents[0], original) || len(stored.Actions) != 1 || stored.Actions[0].ID != actionID || stored.Actions[0].ResolvedAt != "" {
				t.Fatalf("boundary rejection changed source or fabricated human-cause resolution: %+v %v", stored, err)
			}
		})
	}
}
