package webui

import (
	"bytes"
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/handoff"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOperationSourceAlreadyResolvedSuppressesSameObservedAssignmentAfterRestart(t *testing.T) {
	observedAt := "2026-10-12T11:01:00Z"
	signal := operationIncident{
		Project: "/project", AttemptID: "run-1", AssignmentID: "assignment-1",
		TaskID: "A-5", AgentPath: "/root/controller/worker", Kind: "runtime_unknown",
		Evidence: "runtime identity or hook execution unverified", LastObservedAt: observedAt,
	}
	journal := operationJournal{Incidents: []operationIncident{{
		Project: signal.Project, AttemptID: signal.AttemptID, AssignmentID: signal.AssignmentID,
		TaskID: signal.TaskID, AgentPath: signal.AgentPath, Kind: signal.Kind,
		Evidence: signal.Evidence, LastObservedAt: "2026-10-12T11:00:00Z",
		RecoveredAt:      observedAt,
		RecoveryEvidence: "same assignment has recent observed Hook execution; prior runtime evidence unchanged",
	}}}
	if !operationSourceAlreadyResolved(signal, journal, time.Date(2026,10,12,11,1,1,0,time.UTC)) {
		t.Fatal("same source observation should stay resolved after a journal reload")
	}
	signal.AssignmentID = "assignment-2"
	if operationSourceAlreadyResolved(signal, journal, time.Date(2026,10,12,11,1,1,0,time.UTC)) {
		t.Fatal("a distinct assignment must create a new warning episode")
	}
}

func TestOperationStagesBoundedGraceAndFailures(t *testing.T) {
	base := time.Date(2026, 10, 7, 8, 0, 0, 0, time.UTC)
	stamp := func(d time.Duration) string { return base.Add(d).Format(time.RFC3339Nano) }
	assignments := []runtimeobs.Assignment{{AssignmentID: "new", TaskID: "A-19", AgentPath: "/worker", AssignedAt: stamp(0)}}
	attempts := []runtimeobs.Attempt{{AttemptID: "run", TaskID: "A-19", AgentPath: "/worker", BindingState: runtimeobs.BindingBound, BindingEvidence: map[string]string{"assignment_id": "new"}}, {AttemptID: "controller", TaskID: "A-19", AgentPath: "/root/controller", RuntimeAgentID: "controller-id", SessionID: "controller-session", Provider: "codex", BindingState: runtimeobs.BindingBound, CurrentState: runtimeobs.StateRunning, LastActivityAt: stamp(-9 * time.Minute), StateEvidenceSource: runtimeobs.EvidenceHook, StateObservationQuality: runtimeobs.QualityObserved}}
	h := handoff.Handoff{HandoffID: "done", EventType: handoff.EventWorkerDone, TaskID: "A-19", SourceAgentPath: "/worker", SourceAttemptID: "run", PreparedAt: stamp(time.Minute), Steps: map[string]handoff.StepState{}}
	ledger := handoff.Ledger{Handoffs: map[string]handoff.Handoff{"done": h}}
	stages := projectOperationStages("fixture", assignments, attempts, ledger, base.Add(2*time.Minute))
	if len(stages) != 1 || stages[0].Stage != "handoff_pending" || !stages[0].deferred(base.Add(5*time.Minute)) {
		t.Fatalf("pending=%+v", stages)
	}
	again := projectOperationStages("fixture", assignments, attempts, ledger, base.Add(5*time.Minute))
	if again[0].GraceUntil != stages[0].GraceUntil || again[0].deferred(base.Add(6*time.Minute)) {
		t.Fatal("poll reset / inclusive grace boundary")
	}
	incident := operationIncident{TaskID: "A-19", AttemptID: "run", Kind: "runtime_unknown", Evidence: "runtime identity or hook execution unverified"}
	if !stageSuppresses(incident, stages, base.Add(2*time.Minute)) {
		t.Fatal("normal gap not deferred")
	}
	incident.Kind = "errored"
	if stageSuppresses(incident, stages, base.Add(2*time.Minute)) {
		t.Fatal("real error hidden")
	}
	h.ClaimedBy = "/root/controller"
	h.Target.AgentPath = "/root/controller"
	h.ClaimedAttemptID = "controller"
	h.ClaimedAt = stamp(2 * time.Minute)
	h.Target.AttemptID = "controller"
	h.Target.RuntimeAgentID = "controller-id"
	h.Target.SessionID = "controller-session"
	h.Target.Provider = "codex"
	ledger.Handoffs["done"] = h
	review := projectOperationStages("fixture", assignments, attempts, ledger, base.Add(3*time.Minute))[0]
	if review.Stage != "controller_review" || review.GraceUntil != stamp(17*time.Minute) {
		t.Fatalf("review=%+v", review)
	}
	h.Steps["acceptance"] = handoff.StepState{Result: "ok", ObservedAt: stamp(4 * time.Minute)}
	ledger.Handoffs["done"] = h
	complete := projectOperationStages("fixture", assignments, attempts, ledger, base.Add(5*time.Minute))[0]
	if complete.Stage != "review_complete" || complete.GraceUntil != stamp(14*time.Minute) {
		t.Fatalf("review completion=%+v", complete)
	}
	h.Steps["dispatch"] = handoff.StepState{Result: "failed", ObservedAt: stamp(time.Minute)}
	ledger.Handoffs["done"] = h
	failed := projectOperationStages("fixture", assignments, attempts, ledger, base.Add(5*time.Minute))[0]
	if !failed.Failed || failed.deferred(base.Add(5*time.Minute)) {
		t.Fatalf("failure hidden=%+v", failed)
	}
	h.Steps["external"] = handoff.StepState{Result: "failed", ObservedAt: stamp(4 * time.Minute)}
	ledger.Handoffs["done"] = h
	for n := 0; n < 20; n++ {
		current := projectOperationStages("fixture", assignments, attempts, ledger, base.Add(5*time.Minute))[0]
		if current.Since != stamp(time.Minute) {
			t.Fatal("failed map iteration changed clock")
		}
	}
	h.Steps["broken"] = handoff.StepState{Result: "failed", ObservedAt: "bad-clock"}
	ledger.Handoffs["done"] = h
	for n := 0; n < 20; n++ {
		current := projectOperationStages("fixture", assignments, attempts, ledger, base.Add(5*time.Minute))[0]
		if current.Since != h.PreparedAt || current.deferred(base.Add(5*time.Minute)) {
			t.Fatal("malformed failure obtained grace")
		}
	}
}

func TestExactControllerStageLiveAllowsElevenMinuteActivityButFailsClosedOnIdentityAndAge(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	claimed := now.Add(-12 * time.Minute)
	h := handoff.Handoff{ClaimedAt: claimed.Format(time.RFC3339Nano), Target: handoff.Target{AgentPath: "/root/controller", AttemptID: "controller-run", RuntimeAgentID: "native", SessionID: "session", Provider: "codex"}}
	a := runtimeobs.Attempt{AttemptID: "controller-run", AgentPath: "/root/controller", RuntimeAgentID: "native", SessionID: "session", Provider: "codex", BindingState: runtimeobs.BindingBound, CurrentState: runtimeobs.StateRunning, StateEvidenceSource: runtimeobs.EvidenceHook, StateObservationQuality: runtimeobs.QualityObserved, LastActivityAt: now.Add(-11 * time.Minute).Format(time.RFC3339Nano)}
	if !exactControllerStageLive(h, []runtimeobs.Attempt{a}, now) {
		t.Fatal("exact running Controller with 11m-old activity inside 15m claim grace must remain under review")
	}
	wrong := a
	wrong.SessionID = "other-session"
	if exactControllerStageLive(h, []runtimeobs.Attempt{wrong}, now) {
		t.Fatal("mismatched session must fail closed")
	}
	stale := a
	stale.LastActivityAt = now.Add(-30 * time.Minute).Format(time.RFC3339Nano)
	if exactControllerStageLive(h, []runtimeobs.Attempt{stale}, now) {
		t.Fatal("activity outside claim grace must fail closed")
	}
}

func appendSyntheticStageRecord(t *testing.T, project string, r handoff.Record) {
	t.Helper()
	path := handoff.JournalPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	raw, _ := json.Marshal(r)
	if _, err = f.Write(append(raw, '\n')); err != nil {
		t.Fatal(err)
	}
}

func TestOperationStageScanTransitionDedupAndBacklogPreservation(t *testing.T) {
	project := testOperationProject(t)
	base := time.Now().UTC().Add(-time.Hour)
	stamp := func(d time.Duration) string { return base.Add(d).Format(time.RFC3339Nano) }
	a, err := runtimeobs.RecordAssignment(project, "synthetic-assignment", "A-24", "/root/controller/worker", base)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := runtimeobs.BindRuntimeAgentForAssignment(project, a.AssignmentID, "synthetic-worker", "", base.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "_task_mecca", "data", "backlog", "000024.A-24.synthetic.doing.md")
	original := []byte("# A-24 Synthetic\n- Agent: /root/controller/worker\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	appendSyntheticStageRecord(t, project, handoff.Record{HandoffID: "synthetic-done", RecordKind: "prepared", EventType: handoff.EventWorkerDone, TaskID: "A-24", SourceAgentPath: a.AgentPath, SourceAttemptID: attempt.AttemptID, TargetAgentPath: "/root/controller", TargetAttemptID: "controller-run", TargetRuntimeAgentID: "controller-id", TargetSessionID: "controller-session", Provider: "codex", ObservedAt: stamp(2 * time.Second)})
	scan := func(d time.Duration) operationJournal {
		t.Helper()
		j, e := scanOperationProject(project, base.Add(d))
		if e != nil {
			t.Fatal(e)
		}
		return j
	}
	active := func(j operationJournal) []operationIncident {
		out := []operationIncident{}
		for _, i := range j.Incidents {
			if i.RecoveredAt == "" {
				out = append(out, i)
			}
		}
		return out
	}
	if got := active(scan(time.Minute)); len(got) != 0 {
		t.Fatalf("normal handoff warned: %+v", got)
	}
	stalled := active(scan(5*time.Minute + 2*time.Second))
	if len(stalled) != 1 || stalled[0].Kind != "handoff_stalled" {
		t.Fatalf("stall=%+v", stalled)
	}
	repeat := active(scan(5*time.Minute + 3*time.Second))
	if len(repeat) != 1 || repeat[0].ID != stalled[0].ID {
		t.Fatalf("duplicate notification episode=%+v", repeat)
	}
	appendSyntheticStageRecord(t, project, handoff.Record{HandoffID: "synthetic-done", RecordKind: "claimed", ClaimedBy: "/root/controller", ClaimedAttemptID: "controller-run", ObservedAt: stamp(6 * time.Minute)})
	// A verified running Controller also needs provider Hook evidence newer
	// than the configured Hook timestamp. The exact bound attempt events below
	// alone do not prove that the provider hooks remain installed.
	// Install provider hooks with a configuration timestamp on the same
	// synthetic clock as this scenario; hookVerified requires post-config
	// evidence, so wall-clock timestamps would invalidate this fixture.
	hookSetup, err := runtimeobs.EnsureHooks(project, "codex")
	if err != nil {
		t.Fatal(err)
	}
	configuredAt := base.Add(5 * time.Minute)
	if err := os.Chtimes(hookSetup.Path, configuredAt, configuredAt); err != nil {
		t.Fatal(err)
	}
	hookSetup, err = runtimeobs.HookStatus(project, "codex")
	if err != nil {
		t.Fatal(err)
	}
	controllerEvent := runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: stamp(-5 * time.Minute), AttemptID: "controller-run", Provider: "codex", SessionID: "controller-session", RuntimeAgentID: "controller-id", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}
	if err := runtimeobs.AppendExecutionEvent(project, controllerEvent); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.BindAttempt(project, "controller-run", "A-24", "/root/controller", "explicit", "", map[string]string{"assignment_id": "synthetic-assignment"}, base.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	controllerActivity := runtimeobs.ExecutionEvent{EventKind: "activity", ObservedAt: stamp(-5*time.Minute + time.Second), AttemptID: "controller-run", Provider: "codex", SessionID: "controller-session", RuntimeAgentID: "controller-id", EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}
	if err := runtimeobs.AppendExecutionEvent(project, controllerActivity); err != nil {
		t.Fatal(err)
	}
	hookProofAt := configuredAt.Add(time.Second)
	if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "activity", ObservedAt: hookProofAt.Format(time.RFC3339Nano), AttemptID: "controller-run", Provider: "codex", SessionID: "controller-session", RuntimeAgentID: "controller-id", EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}); err != nil {
		t.Fatal(err)
	}
	if err := runtimeobs.AppendExecutionEvent(project, controllerEvent); err != nil {
		t.Fatal(err)
	}
	if got := active(scan(7 * time.Minute)); len(got) != 0 {
		t.Fatalf("claim review grace warned=%+v", got)
	}
	reviewStall := active(scan(21 * time.Minute))
	if len(reviewStall) != 1 || reviewStall[0].Kind != "handoff_stalled" {
		t.Fatalf("review stall=%+v", reviewStall)
	}
	appendSyntheticStageRecord(t, project, handoff.Record{HandoffID: "synthetic-done", RecordKind: "step_succeeded", Step: "acceptance", Result: "ok", ObservedAt: stamp(22 * time.Minute)})
	controllerLate := runtimeobs.ExecutionEvent{EventKind: "activity", ObservedAt: stamp(22 * time.Minute), AttemptID: "controller-run", Provider: "codex", SessionID: "controller-session", RuntimeAgentID: "controller-id", EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}
	if err := runtimeobs.AppendExecutionEvent(project, controllerLate); err != nil {
		t.Fatal(err)
	}
	if got := active(scan(23 * time.Minute)); len(got) != 0 {
		t.Fatalf("review complete finalization grace=%+v", got)
	}
	current, _ := os.ReadFile(path)
	if !bytes.Equal(current, original) {
		t.Fatal("worker/review completion mutated backlog doing")
	}
	appendSyntheticStageRecord(t, project, handoff.Record{HandoffID: "synthetic-done", RecordKind: "step_failed", Step: "external", Result: "failed", ObservedAt: stamp(24 * time.Minute)})
	failed := active(scan(24 * time.Minute))
	if len(failed) != 1 || failed[0].Kind != "handoff_failed" {
		t.Fatalf("failure=%+v", failed)
	}
	again := active(scan(24*time.Minute + time.Second))
	if len(again) != 1 || again[0].ID != failed[0].ID {
		t.Fatal("failed episode duplicate")
	}
}

func TestOperationStageVerifiedCompletionClosesStallWithoutRuntimeMutation(t *testing.T) {
	f := newCompletionFixture(t)
	journal, e := readOperationJournal(f.project)
	if e != nil {
		t.Fatal(e)
	}
	journal.Incidents[0].Kind = "handoff_stalled"
	journal.Incidents[0].Evidence = "durable worker_done handoff"
	if e := writeOperationJournal(f.project, journal); e != nil {
		t.Fatal(e)
	}
	before, e := runtimeobs.BuildLedger(f.project, 3, f.base.Add(11*time.Second))
	if e != nil {
		t.Fatal(e)
	}
	after, e := scanOperationProject(f.project, f.base.Add(11*time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if after.Incidents[0].RecoveredAt == "" || after.Incidents[0].Resolution == nil {
		t.Fatalf("verified done leaves stall=%+v", after.Incidents)
	}
	runtimeAfter, _ := runtimeobs.BuildLedger(f.project, 3, f.base.Add(11*time.Second))
	old, _ := json.Marshal(before.Attempts)
	now, _ := json.Marshal(runtimeAfter.Attempts)
	if !bytes.Equal(old, now) {
		t.Fatal("runtime fact fabricated")
	}
}

func TestOperationStageDoesNotHideUnconfirmedTerminalError(t *testing.T) {
	now := time.Now().UTC()
	stage := []operationStage{{TaskID: "A-25", AttemptID: "run", GraceUntil: now.Add(time.Minute).Format(time.RFC3339Nano)}}
	for _, state := range []runtimeobs.CanonicalState{runtimeobs.StateErrored, runtimeobs.StateInterrupted, runtimeobs.StateShutdown} {
		signal, ok := classifyOperation("fixture", runtimeobs.Attempt{AttemptID: "run", TaskID: "A-25", CurrentState: state, Terminal: true}, now, false)
		if !ok || stageSuppresses(signal.incident, stage, now) {
			t.Fatalf("unconfirmed actual terminal hidden: %+v", signal)
		}
	}
}

func TestOperationStageResolvedOldAttemptDoesNotRecur(t *testing.T) {
	project := testOperationProject(t)
	task := filepath.Join(project, "_task_mecca", "data", "backlog", "000458.B-458.active.doing.md")
	if err := os.MkdirAll(filepath.Dir(task), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(task, []byte("# B-458 Active\n- Agent: /root/controller/worker\n"), 0644); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(-time.Hour)
	old, e := runtimeobs.RecordAssignment(project, "old-assignment", "B-458", "/root/controller/worker", base)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := runtimeobs.BindRuntimeAgentForAssignment(project, old.AssignmentID, "old-worker", "", base.Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	initial, e := scanOperationProject(project, base.Add(2*time.Second))
	if e != nil || len(initial.Incidents) != 1 {
		t.Fatalf("initial=%+v %v", initial, e)
	}
	latest, e := runtimeobs.RecordAssignment(project, "new-assignment", "B-458", "/root/controller/worker", base.Add(time.Minute))
	if e != nil {
		t.Fatal(e)
	}
	if _, e := runtimeobs.BindRuntimeAgentForAssignment(project, latest.AssignmentID, "new-worker", "", base.Add(time.Minute+time.Second)); e != nil {
		t.Fatal(e)
	}
	newHook := testOperationHook(t, project, `{"session_id":"latest-session","turn_id":"latest-turn","hook_event_name":"SubagentStart","agent_id":"new-worker"}`, base.Add(time.Minute+2*time.Second))
	originalVerifier := operationHookVerifier
	operationHookVerifier = func(string, string) bool { return true }
	defer func() { operationHookVerifier = originalVerifier }()
	resolved, e := scanOperationProject(project, base.Add(time.Minute+3*time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if resolved.Incidents[0].RecoveredAt == "" {
		t.Fatalf("old warning unresolved=%+v", resolved.Incidents)
	}
	sequence := resolved.NextSequence
	for n := 4; n <= 6; n++ {
		j, e := scanOperationProject(project, base.Add(time.Minute+time.Duration(n)*time.Second))
		if e != nil {
			t.Fatal(e)
		}
		if j.NextSequence != sequence {
			t.Fatalf("old source re-alerted %+v", j.Incidents)
		}
		for _, i := range j.Incidents {
			if i.RecoveredAt == "" {
				t.Fatalf("old source active=%+v", i)
			}
		}
	}
	if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", AttemptID: newHook.AttemptID, State: runtimeobs.StateErrored, Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved, ObservedAt: base.Add(time.Minute + 7*time.Second).Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	j, err := scanOperationProject(project, base.Add(time.Minute+8*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	active := []operationIncident{}
	for _, i := range j.Incidents {
		if i.RecoveredAt == "" {
			active = append(active, i)
		}
	}
	if len(active) != 1 || active[0].Kind != "errored" || active[0].AttemptID != newHook.AttemptID {
		t.Fatalf("new error hidden or old warning reappeared=%+v", active)
	}
	if err := os.Remove(task); err != nil {
		t.Fatal(err)
	}
	j, err = scanOperationProject(project, base.Add(time.Minute+9*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	foundUnknown := false
	for _, i := range j.Incidents {
		if i.Kind == "runtime_unknown" && i.RecoveredAt == "" {
			foundUnknown = true
		}
	}
	if !foundUnknown {
		encoded, _ := json.Marshal(j.Incidents)
		t.Fatalf("unavailable canonical policy blindly trusted historical suppression: incidents=%s", encoded)
	}
}

func TestOperationStageLatestAssignmentAndHistoricalRecovery(t *testing.T) {
	base := time.Date(2026, 10, 7, 8, 0, 0, 0, time.UTC)
	stamp := func(d time.Duration) string { return base.Add(d).Format(time.RFC3339Nano) }
	assignments := []runtimeobs.Assignment{{AssignmentID: "old", TaskID: "B-457", AgentPath: "/worker", AssignedAt: stamp(0)}, {AssignmentID: "new", TaskID: "B-457", AgentPath: "/worker", AssignedAt: stamp(time.Minute)}}
	attempts := []runtimeobs.Attempt{{AttemptID: "new-run", TaskID: "B-457", AgentPath: "/worker", BindingState: runtimeobs.BindingBound, BindingEvidence: map[string]string{"assignment_id": "new"}, CurrentState: runtimeobs.StateRunning, StateEvidenceSource: runtimeobs.EvidenceHook, StateObservationQuality: runtimeobs.QualityObserved, LastObservedAt: stamp(2 * time.Minute)}}
	stages := projectOperationStages("fixture", assignments, attempts, handoff.Ledger{}, base.Add(3*time.Minute))
	if len(stages) != 1 || stages[0].AssignmentID != "new" {
		t.Fatalf("stale assignment=%+v", stages)
	}
	historical := operationIncident{TaskID: "B-457", Kind: "runtime_unknown", Evidence: "doing assignment has no linked runtime attempt", DetectedAt: stamp(time.Minute)}
	if !latestOperationRecovery(historical, attempts, stages, base.Add(3*time.Minute)) {
		t.Fatal("execution-ID-less assigned warning not recovered")
	}
	historical.Kind = "errored"
	if latestOperationRecovery(historical, attempts, stages, base.Add(3*time.Minute)) {
		t.Fatal("real error falsely recovered")
	}
	historical.Kind = "runtime_unknown"
	attempts[0].StateEvidenceSource = ""
	if latestOperationRecovery(historical, attempts, stages, base.Add(3*time.Minute)) {
		t.Fatal("unobserved execution falsely recovered")
	}
	// Reassignment invalidates the old report and starts only the assignment clock.
	assignments = append(assignments, runtimeobs.Assignment{AssignmentID: "newest", TaskID: "B-457", AgentPath: "/other", AssignedAt: stamp(2 * time.Minute)})
	stages = projectOperationStages("fixture", assignments, attempts, handoff.Ledger{}, base.Add(3*time.Minute))
	if stages[0].Stage != "assignment_pending" || stages[0].AttemptID != "" {
		t.Fatalf("reassignment=%+v", stages)
	}
}

func TestCompletionReviewStageSeparatesUnverifiedEvidenceFromFailure(t *testing.T) {
	base := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	cases := []struct {
		name         string
		confirmed    bool
		wantStage    string
		wantFailed   bool
		wantSeverity string
	}{
		{name: "unverified linkage", wantStage: "handoff_evidence_unverified", wantSeverity: "warning"},
		{name: "confirmed failure", confirmed: true, wantStage: "handoff_failed", wantFailed: true, wantSeverity: "error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stage := operationStage{Project: "fixture", TaskID: "SYNTH-47", AssignmentID: "assignment-synthetic-47"}
			ok := applyCompletionReviewStage(&stage, map[string]any{
				"state":               "controller_recovery",
				"since":               base,
				"diagnostic":          "synthetic evidence",
				"reason":              "synthetic_reason",
				"observation_quality": "insufficient",
				"action_required":     true,
				"action":              "inspect_exact_linkage",
				"severity":            "warning",
				"confirmed_failure":   tc.confirmed,
			})
			if !ok || stage.Stage != tc.wantStage || stage.Failed != tc.wantFailed || stage.Severity != tc.wantSeverity {
				t.Fatalf("stage=%+v", stage)
			}
			if stage.Reason != "synthetic_reason" || stage.ObservationQuality != "insufficient" || !stage.ActionRequired || stage.Action != "inspect_exact_linkage" {
				t.Fatalf("classification evidence lost: %+v", stage)
			}
		})
	}
}
