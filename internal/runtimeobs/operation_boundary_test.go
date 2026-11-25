package runtimeobs

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func boundaryFixture() OperationBoundary {
	return OperationBoundary{Key: "start", OperationID: "registration-request", Provider: "codex", NativeID: "native", AttemptID: "attempt", Role: "registrar", Kind: "start", Source: "test_fixture", Evidence: "actual current boundary"}
}

func TestOperationBoundaryCurrentIdempotentAndConflict(t *testing.T) {
	p := t.TempDir()
	now := time.Now().UTC()
	b := boundaryFixture()
	past := now.Add(-time.Hour)
	b.SourceAt = &past
	first, yes, err := AppendOperationBoundary(p, b, now)
	if err != nil || !yes {
		t.Fatalf("%v %v", yes, err)
	}
	again, yes, err := AppendOperationBoundary(p, b, now.Add(time.Hour))
	if err != nil || yes || !again.ObservedAt.Equal(now) {
		t.Fatalf("retry altered observation: %+v %v", again, err)
	}
	if !first.SourceAt.Equal(past) || !first.ObservedAt.Equal(now) {
		t.Fatal("source time backfilled measured start")
	}
	b.Evidence = "conflict"
	if _, _, err = AppendOperationBoundary(p, b, now); err == nil {
		t.Fatal("conflict accepted")
	}
	rows, err := ReadOperationBoundaries(p)
	if err != nil || len(rows) != 1 {
		t.Fatalf("%d %v", len(rows), err)
	}
}

func TestOperationBoundaryConcurrentAndCorruptJournal(t *testing.T) {
	p := t.TempDir()
	now := time.Now().UTC()
	b := boundaryFixture()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := AppendOperationBoundary(p, b, now); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	rows, err := ReadOperationBoundaries(p)
	if err != nil || len(rows) != 1 {
		t.Fatalf("%d %v", len(rows), err)
	}
	f, _ := os.OpenFile(OperationBoundaryPath(p), os.O_APPEND|os.O_WRONLY, 0600)
	f.WriteString("{invalid\n")
	f.Close()
	rows, err = ReadOperationBoundaries(p)
	if err == nil || len(rows) != 1 {
		t.Fatal("collection failure discarded or hidden")
	}
	b.Key = "new"
	if _, _, err = AppendOperationBoundary(p, b, now); err == nil {
		t.Fatal("corrupt journal written through")
	}
}

func TestOperationBoundaryRejectSymlinkAndMissingIdentity(t *testing.T) {
	p := t.TempDir()
	b := boundaryFixture()
	b.NativeID = ""
	if _, _, err := AppendOperationBoundary(p, b, time.Now()); err == nil {
		t.Fatal("missing identity")
	}
	b = boundaryFixture()
	AppendOperationBoundary(p, b, time.Now())
	os.Remove(OperationBoundaryPath(p))
	target := p + "/foreign"
	os.WriteFile(target, []byte("preserve"), 0600)
	os.Symlink(target, OperationBoundaryPath(p))
	if _, _, err := AppendOperationBoundary(p, b, time.Now()); err == nil {
		t.Fatal("symlink accepted")
	}
	raw, _ := os.ReadFile(target)
	if string(raw) != "preserve" {
		t.Fatal("foreign write")
	}
}

func TestOperationBoundaryNativeRetryAndLateTaskLink(t *testing.T) {
	p := t.TempDir()
	now := time.Now().UTC()
	a := Attempt{AttemptID: "episode"}
	r := T3Receipt{Provider: "codex", SessionID: "native", NativeAttemptID: "native-attempt", ThreadID: "thread", State: StateRunning, StartedAt: now.Add(-time.Hour).Format(time.RFC3339Nano)}
	if err := collectNativeBoundary(p, a, r, now); err != nil {
		t.Fatal(err)
	}
	a.TaskID = "A-1"
	a.BindingState = BindingBound
	a.AgentPath = "/root/controller/naong"
	if err := collectNativeBoundary(p, a, r, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	rows, err := ReadOperationBoundaries(p)
	if err != nil || len(rows) != 2 || !rows[0].ObservedAt.Equal(now) || rows[1].Kind != "task_link" {
		t.Fatalf("%+v %v", rows, err)
	}
}

func TestA93T3UniqueRunningWithQueuedSuccessors(t *testing.T) {
	store, sql := t3Fixture(t)
	now := time.Date(2026, 11, 12, 1, 2, 0, 0, time.UTC)
	sql(`INSERT INTO orchestration_v2_projection_runs VALUES('thread','run-2','codex','queued',NULL,2,'provider-thread'); INSERT INTO orchestration_v2_projection_runs VALUES('thread','run-3','codex','queued',NULL,3,'provider-thread');`)
	r, err := store.read(testT3Selector(), false, now)
	if err != nil || r.RunID != "run-1" {
		t.Fatalf("running displaced by queue: %+v %v", r, err)
	}
	sel := testT3Selector()
	sel.SessionID = "other"
	sel.RuntimeAgentID = "other"
	if _, err = store.read(sel, false, now); err == nil {
		t.Fatal("identity weakened")
	}
	sql(`UPDATE orchestration_v2_projection_runs SET status='running' WHERE run_id='run-2';`)
	if _, err = store.read(testT3Selector(), false, now); err == nil {
		t.Fatal("multiple running accepted")
	}
	sql(`UPDATE orchestration_v2_projection_runs SET status='completed' WHERE run_id='run-2';`)
	if _, err = store.read(testT3Selector(), false, now); err == nil {
		t.Fatal("stale running selected behind terminal")
	}
}

func TestOperationBoundaryWaitEvidenceComplete(t *testing.T) {
	w := &BoundaryWait{Scope: "actor", Reason: "review", ExpectedEvent: "report", NextOwner: "reviewer", LastEvidence: "request", ResumeCondition: "accepted"}
	if !w.Evidenced() {
		t.Fatal("complete wait")
	}
	w.Reason = "unknown"
	if w.Evidenced() {
		t.Fatal("unknown fabricated")
	}
	if strings.TrimSpace(w.Reason) == "" {
		t.Fatal("fixture")
	}
	w.Reason = "review"
	w.LastEvidence = " UNKNOWN "
	if w.Evidenced() {
		t.Fatal("unknown evidence cannot establish a wait")
	}
}

func TestOperationBoundaryRoleDoesNotTreatWorkerPathAsController(t *testing.T) {
	for path, role := range map[string]string{"/root/controller": "controller", "/root/registrar": "registrar", "/root/controller/naong": "worker", "/root/controller/reviewer/raichyu": "reviewer", "/root/controller/sub-x": "unknown"} {
		if got := boundaryRoleFromPath(path); got != role {
			t.Fatalf("%s: %s", path, got)
		}
	}
}

func TestA102EpisodeResumeRequiresExactGenuineContinuity(t *testing.T) {
	now := time.Date(2026, 11, 25, 2, 0, 0, 0, time.UTC)
	makeAttempts := func() (Attempt, Attempt, OperationBoundary, T3Receipt) {
		receipt := T3Receipt{Origin: "t3_local_projection_v2", Provider: "codex", SessionID: "native", ThreadID: "thread", RunID: "old-run", NativeAttemptID: "old-native", State: StateInterrupted, StartedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), CompletedAt: now.Add(-20 * time.Second).Format(time.RFC3339Nano), ObservedAt: now.Format(time.RFC3339Nano)}
		old := Attempt{AttemptID: "old", Provider: "codex", RuntimeAgentID: "native", SessionID: "native", TurnID: "old-native", TaskID: "FIXTURE-1", AgentPath: "/root/controller/pairi", BindingState: BindingBound, BindingEvidence: map[string]string{"assignment_id": "continuing"}, Terminal: true, CurrentState: StateInterrupted, StateObservationQuality: QualityObserved, StateEvidenceSource: EvidenceT3, LastObservedAt: now.Format(time.RFC3339Nano), NativeReceipt: &receipt}
		current := old
		current.AttemptID = "current"
		current.Terminal = false
		current.CurrentState = StateRunning
		current.TurnID = "current-native"
		fresh := receipt
		fresh.RunID = "new-run"
		fresh.NativeAttemptID = "current-native"
		fresh.State = StateRunning
		fresh.StartedAt = now.Add(-10 * time.Second).Format(time.RFC3339Nano)
		fresh.CompletedAt = ""
		current.NativeReceipt = &fresh
		current.BindingEvidence = map[string]string{"assignment_id": "continuing"}
		related := OperationBoundary{ID: "actual-old-start", NativeID: "native", AttemptID: "old", ThreadID: "thread", Role: "worker", TaskIDs: []string{"FIXTURE-1"}}
		return current, old, related, receipt
	}
	for _, kind := range []string{"valid", "different_native", "different_task", "different_role", "different_assignment", "nonterminal", "foreign_boundary", "foreign_thread", "unlinked", "overlap", "wrong_terminal", "missing_end"} {
		t.Run(kind, func(t *testing.T) {
			current, old, related, terminal := makeAttempts()
			switch kind {
			case "different_native":
				current.RuntimeAgentID = "other"
			case "different_task":
				current.TaskID = "FIXTURE-2"
			case "different_role":
				current.AgentPath = "/root/controller"
			case "different_assignment":
				current.BindingEvidence["assignment_id"] = "other"
			case "nonterminal":
				old.Terminal = false
			case "foreign_boundary":
				related.AttemptID = "foreign"
			case "foreign_thread":
				terminal.ThreadID = "foreign"
			case "unlinked":
				related.TaskIDs = nil
			case "overlap":
				current.NativeReceipt.StartedAt = now.Add(-30 * time.Second).Format(time.RFC3339Nano)
			case "wrong_terminal":
				terminal.NativeAttemptID = "foreign"
			case "missing_end":
				terminal.CompletedAt = ""
			}
			err := ValidateEpisodeResume(current, old, related, terminal, now)
			if (err == nil) != (kind == "valid") {
				t.Fatalf("%s: %v", kind, err)
			}
			if related.Kind == "end" || related.SourceAt != nil {
				t.Fatal("validation rewrote old boundary")
			}
		})
	}
}
