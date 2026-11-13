package webui

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func qualityAttempt(now time.Time) runtimeobs.Attempt {
	stamp := func(d time.Duration) string { return now.Add(d).Format(time.RFC3339Nano) }
	return runtimeobs.Attempt{AttemptID: "fixture-attempt", TaskID: "F-1", AgentPath: "/root/controller/fixture", Provider: "codex", SessionID: "fixture-session", RuntimeAgentID: "fixture-session", TurnID: "fixture-native-attempt", BindingState: runtimeobs.BindingBound, BindingEvidence: map[string]string{"assignment_id": "fixture-assignment"}, CurrentState: runtimeobs.StateCompleted, Terminal: true, StateEvidenceSource: runtimeobs.EvidenceT3, StateObservationQuality: runtimeobs.QualityObserved, StartedAt: stamp(-time.Hour), EndedAt: stamp(-time.Minute), LastObservedAt: stamp(-time.Minute), NativeReceipt: &runtimeobs.T3Receipt{ThreadID: "fixture-thread", RunID: "fixture-run", NativeAttemptID: "fixture-native-attempt", SessionID: "fixture-session", Provider: "codex", State: runtimeobs.StateCompleted, StartedAt: stamp(-time.Hour), CompletedAt: stamp(-time.Minute), ObservedAt: stamp(-time.Minute), Origin: "t3_local_projection_v2"}}
}

func TestWarningQualityExactStructuredCompletionMatrix(t *testing.T) {
	now := time.Date(2026, 11, 13, 2, 0, 0, 0, time.UTC)
	original := qualityAttempt(now)
	item := operationIncident{AttemptID: original.AttemptID, TaskID: original.TaskID, AgentPath: original.AgentPath, AssignmentID: "fixture-assignment", Provider: original.Provider, SessionID: original.SessionID, TurnID: original.TurnID, Kind: "runtime_unknown", Quality: "verification_required", DetectedAt: now.Format(time.RFC3339Nano), LastObservedAt: original.LastObservedAt}
	// Includes residue detected after completion, as well as a later terminal event.
	for _, detected := range []string{now.Format(time.RFC3339Nano), now.Add(-2 * time.Minute).Format(time.RFC3339Nano)} {
		warning := item
		warning.DetectedAt = detected
		if _, _, ok := operationExactAttemptRecovered(warning, []runtimeobs.Attempt{original}, now); !ok {
			t.Fatal("validated structured completion rejected")
		}
	}
	cases := map[string]func(*runtimeobs.Attempt){
		"provider":              func(a *runtimeobs.Attempt) { a.Provider = "claude" },
		"session":               func(a *runtimeobs.Attempt) { a.SessionID = "other" },
		"runtime":               func(a *runtimeobs.Attempt) { a.RuntimeAgentID = "other" },
		"task":                  func(a *runtimeobs.Attempt) { a.TaskID = "F-2" },
		"assignment":            func(a *runtimeobs.Attempt) { a.BindingEvidence = map[string]string{"assignment_id": "other"} },
		"episode":               func(a *runtimeobs.Attempt) { a.AttemptID = "new-episode" },
		"native attempt":        func(a *runtimeobs.Attempt) { a.TurnID = "other" },
		"late activity":         func(a *runtimeobs.Attempt) { a.LastActivityAt = now.Format(time.RFC3339Nano) },
		"future":                func(a *runtimeobs.Attempt) { a.LastObservedAt = now.Add(time.Minute).Format(time.RFC3339Nano) },
		"inferred":              func(a *runtimeobs.Attempt) { a.StateObservationQuality = runtimeobs.QualityInferred },
		"ambiguous":             func(a *runtimeobs.Attempt) { a.BindingState = runtimeobs.BindingAmbiguous },
		"missing receipt":       func(a *runtimeobs.Attempt) { a.NativeReceipt = nil },
		"forged source":         func(a *runtimeobs.Attempt) { a.NativeReceipt.Origin = "caller" },
		"missing source thread": func(a *runtimeobs.Attempt) { a.NativeReceipt.ThreadID = "" },
		"bad timestamp":         func(a *runtimeobs.Attempt) { a.NativeReceipt.ObservedAt = "bad" },
		"actual error": func(a *runtimeobs.Attempt) {
			a.CurrentState = runtimeobs.StateErrored
			a.NativeReceipt.State = a.CurrentState
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			a := qualityAttempt(now)
			mutate(&a)
			if _, _, ok := operationExactAttemptRecovered(item, []runtimeobs.Attempt{a}, now); ok {
				t.Fatal("unsafe recovery")
			}
		})
	}
	for _, field := range []string{"future detection", "future source", "corrupt source"} {
		bad := item
		switch field {
		case "future detection":
			bad.DetectedAt = now.Add(time.Hour).Format(time.RFC3339Nano)
		case "future source":
			bad.LastObservedAt = now.Add(time.Hour).Format(time.RFC3339Nano)
		case "corrupt source":
			bad.LastObservedAt = "bad"
		}
		if _, _, ok := operationExactAttemptRecovered(bad, []runtimeobs.Attempt{original}, now); ok {
			t.Fatal(field + " recovered")
		}
	}
	errorItem := item
	errorItem.Quality = "confirmed"
	errorItem.Kind = "errored"
	if _, _, ok := operationExactAttemptRecovered(errorItem, []runtimeobs.Attempt{original}, now); ok {
		t.Fatal("confirmed error retired")
	}
	signal, ok := classifyOperation("fixture", original, now, false)
	if !ok || signal.alert || signal.incident.Kind != "normal_exit" {
		t.Fatalf("structured completed=%+v", signal)
	}
	errored := qualityAttempt(now)
	errored.CurrentState = runtimeobs.StateErrored
	errored.NativeReceipt.State = errored.CurrentState
	signal, ok = classifyOperation("fixture", errored, now, false)
	if !ok || !signal.alert || signal.incident.Kind != "errored" || signal.incident.Quality != "confirmed" {
		t.Fatalf("structured error=%+v", signal)
	}
}

func TestWarningQualityProjectHookSetupIsNotAttemptProof(t *testing.T) {
	now := time.Now().UTC()
	a := qualityAttempt(now)
	a.Terminal = false
	a.CurrentState = runtimeobs.StateRunning
	a.StateEvidenceSource = runtimeobs.EvidenceManualBinding
	a.EndedAt = ""
	for _, setup := range []bool{false, true} {
		signal, ok := classifyOperation("fixture", a, now, setup)
		if !ok || signal.incident.Kind != "runtime_unknown" {
			t.Fatalf("setup became per-attempt evidence: %+v", signal)
		}
	}
	a.StateEvidenceSource = runtimeobs.EvidenceHook
	signal, ok := classifyOperation("fixture", a, now.Add(runtimeobs.StaleWarnDuration()), false)
	if !ok || signal.incident.Kind != "no_signal" {
		t.Fatalf("attempt evidence ignored: %+v", signal)
	}
}

func TestWarningQualityFindingScopeAndCorruption(t *testing.T) {
	f := newCompletionFixture(t)
	now := f.base.Add(11 * time.Second)
	ledger, err := runtimeobs.BuildLedger(f.project, 3, now)
	if err != nil {
		t.Fatal(err)
	}
	other := runtimeobs.Attempt{AttemptID: "other-attempt", TaskID: "F-99"}
	ledger.Attempts = append(ledger.Attempts, other)
	for _, tc := range []struct {
		name, code, attempt string
		allow               bool
	}{
		{"unrelated stale", "stale", other.AttemptID, true},
		{"own stale", "stale", f.attemptID, true},
		{"unrelated ambiguity", "binding_ambiguous", other.AttemptID, true},
		{"same task ambiguity", "binding_ambiguous", "same-task", false},
		{"own ambiguity", "binding_ambiguous", f.attemptID, false},
		{"parse corruption", "invalid_event_line", "", false},
		{"orphan identity", "binding_ambiguous", "missing", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := ledger
			l.Attempts = append([]runtimeobs.Attempt{}, ledger.Attempts...)
			if tc.attempt == "same-task" {
				l.Attempts = append(l.Attempts, runtimeobs.Attempt{AttemptID: "same-task", TaskID: "A-5"})
			}
			l.Findings = []runtimeobs.LedgerFinding{{Severity: "warning", Code: tc.code, AttemptID: tc.attempt}}
			_, ok := operationCompletionEvidence(f.project, l, now)[f.attemptID]
			if ok != tc.allow {
				t.Fatalf("completion mapping=%v expected %v", ok, tc.allow)
			}
		})
	}
}

func TestWarningQualityAuditCategoryMatrixPreservesEvidence(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-time.Hour).Format(time.RFC3339Nano)
	boundary := now.Add(-time.Minute).Format(time.RFC3339Nano)
	rows := map[string][]backlog.Record{
		"F-DOING": {{State: "doing"}},
		"F-DONE":  {{State: "done", Mtime: boundary}},
		"F-HOLD":  {{State: "hold", Mtime: boundary, Fields: map[string]string{"대기유형": "user", "대기": "user decides", "재개조건": "explicit approval"}}},
	}
	before := []operationIncident{}
	counts := map[string]int{}
	for _, group := range []struct {
		task, category string
		n              int
	}{{"F-DOING", operationCurrent, 1}, {"F-DONE", operationHistory, 11}, {"F-HOLD", operationHold, 5}, {"", operationDiagnostic, 4}} {
		for i := 0; i < group.n; i++ {
			item := operationIncident{TaskID: group.task, Kind: "runtime_unknown", Quality: "verification_required", LastObservedAt: old, DetectedAt: old}
			before = append(before, item)
			got := operationViewCategory(item, rows, true, now)
			if got != group.category {
				t.Fatalf("task %s=%s", group.task, got)
			}
			counts[got]++
		}
	}
	// Three terminal residues use exact validated receipts, never task state.
	for i := 0; i < 3; i++ {
		a := qualityAttempt(now)
		item := operationIncident{AttemptID: a.AttemptID, TaskID: a.TaskID, AgentPath: a.AgentPath, Kind: "runtime_unknown", Quality: "verification_required", DetectedAt: old, LastObservedAt: old}
		if _, _, ok := operationExactAttemptRecovered(item, []runtimeobs.Attempt{a}, now); !ok {
			t.Fatal("terminal residue not recoverable")
		}
	}
	if counts[operationCurrent] != 1 || counts[operationHistory] != 11 || counts[operationHold] != 5 || counts[operationDiagnostic] != 4 {
		t.Fatal(counts)
	}
	// A second current task adds a warning without changing historic grouping.
	rows["F-NEXT"] = []backlog.Record{{State: "doing"}}
	if operationViewCategory(operationIncident{TaskID: "F-NEXT", Kind: "runtime_unknown"}, rows, true, now) != operationCurrent {
		t.Fatal("new doing missing link hidden")
	}
	for _, task := range []string{"F-DONE", "F-HOLD"} {
		item := operationIncident{TaskID: task, Kind: "errored", Quality: "confirmed", LastObservedAt: old}
		if operationViewCategory(item, rows, true, now) != operationCurrent {
			t.Fatal("actual error hidden")
		}
		item.Kind = "no_signal"
		item.Quality = "verification_required"
		item.LastObservedAt = now.Format(time.RFC3339Nano)
		if operationViewCategory(item, rows, true, now) != operationCurrent {
			t.Fatal("late activity hidden")
		}
	}
	raw, _ := json.Marshal(before)
	for _, item := range before {
		operationViewCategory(item, rows, true, now)
	}
	after, _ := json.Marshal(before)
	if !bytes.Equal(raw, after) {
		t.Fatal("view mutated history")
	}
	if operationViewCategory(before[0], nil, false, now) != operationDiagnostic {
		t.Fatal("read failure assumed current state")
	}
}

func TestWarningQualityDryRunStructuredAndPolicyNoMutation(t *testing.T) {
	// Policy and completion use only t.TempDir ledgers; no service or delivery.
	f := newCompletionFixture(t)
	now := f.base.Add(11 * time.Second)
	before, err := os.ReadFile(operationJournalPath(f.project))
	if err != nil {
		t.Fatal(err)
	}
	result, err := ReconcileCompletedOperation(f.project, f.incidentID, now, true)
	if err != nil || result["changed"] != true || result["notifications_sent"] != false {
		t.Fatalf("%v %v", result, err)
	}
	after, _ := os.ReadFile(operationJournalPath(f.project))
	if !bytes.Equal(before, after) {
		t.Fatal("dry run wrote")
	}
	// Malformed journal input remains a global unknown and blocks recovery.
	path := runtimeobs.ExecutionJournalPath(f.project)
	raw, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, []byte("not-json\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileCompletedOperation(f.project, f.incidentID, now, true); err == nil {
		t.Fatal("corrupt input allowed recovery")
	}
}

// Explicitly opt-in read-only source comparison. Never calls scan, reconcile
// apply, delivery, hooks, service lifecycle or any runtime writer.
func TestWarningQualityReadOnlyActualEvidence(t *testing.T) {
	project := os.Getenv("TASK_MECCA_A71_READONLY_PROJECT")
	if project == "" {
		t.Skip("read-only actual comparison requires explicit project")
	}
	now := time.Now().UTC()
	journal, err := readOperationJournal(project)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := runtimeobs.BuildLedger(project, 3, now)
	if err != nil {
		t.Fatal(err)
	}
	rows, available := operationCanonicalRows(project)
	counts := map[string]int{}
	recovery := 0
	for _, item := range journal.Incidents {
		if item.RecoveredAt != "" || item.Kind == "normal_exit" {
			continue
		}
		counts[operationViewCategory(item, rows, available, now)]++
		if _, _, ok := operationExactAttemptRecovered(item, operationUnconflictedAttempts(ledger), now); ok {
			recovery++
		}
	}
	t.Logf("read_only=true writes=0 categories=%v exact_execution_recoverable=%d canonical_available=%v", counts, recovery, available)
}

func TestWarningQualityStructuredFreshnessEpisodeAndRestart(t *testing.T) {
	now := time.Date(2026, 11, 13, 2, 0, 0, 0, time.UTC)
	a := qualityAttempt(now)
	a.CurrentState = runtimeobs.StateRunning
	a.Terminal = false
	a.EndedAt = ""
	a.NativeReceipt.State = a.CurrentState
	a.NativeReceipt.CompletedAt = ""
	a.LastObservedAt = now.Add(-time.Second).Format(time.RFC3339Nano)
	a.NativeReceipt.ObservedAt = a.LastObservedAt
	warning := operationIncident{Project: "fixture", AttemptID: a.AttemptID, TaskID: a.TaskID, AgentPath: a.AgentPath, AssignmentID: "fixture-assignment", Kind: "no_signal", Quality: "verification_required", Evidence: "observed execution has no recent signal", DetectedAt: now.Add(-3 * time.Minute).Format(time.RFC3339Nano), LastObservedAt: now.Add(-4 * time.Minute).Format(time.RFC3339Nano)}
	at, why, ok := operationExactAttemptRecovered(warning, []runtimeobs.Attempt{a}, now)
	if !ok {
		t.Fatal("fresh structured execution did not recover exact source")
	}
	if _, ok := operationEvidenceRecovery(warning, []runtimeobs.Attempt{a}, now); !ok {
		t.Fatal("assignment recovery excluded structured evidence")
	}
	prior := warning
	prior.RecoveredAt = at
	prior.RecoveryEvidence = why
	journal := operationJournal{Incidents: []operationIncident{prior}}
	// A restart rebuilds suppression from durable exact source evidence.
	encoded, _ := json.Marshal(journal)
	var restored operationJournal
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	index := indexResolvedSources(restored)
	signal := warning
	signal.LastObservedAt = a.LastObservedAt
	if !index.alreadyResolved(signal, now) {
		t.Fatal("unchanged resolved source rewarned on restart")
	}
	staleNow := now.Add(runtimeobs.StaleWarnDuration())
	if index.alreadyResolved(signal, staleNow) {
		t.Fatal("new silence episode suppressed forever")
	}
	if _, _, ok := operationExactAttemptRecovered(warning, []runtimeobs.Attempt{a}, staleNow); ok {
		t.Fatal("stale running receipt treated as fresh")
	}
	signal.AttemptID = "new-episode"
	if index.alreadyResolved(signal, now) {
		t.Fatal("new attempt inherited old resolution")
	}
	signal = warning
	signal.Kind = "errored"
	signal.Quality = "confirmed"
	if index.alreadyResolved(signal, now) {
		t.Fatal("new actual error inherited old resolution")
	}
}

func TestWarningQualityPastHoldDoesNotBecomeCurrentDecisionAfterResume(t *testing.T) {
	now := time.Now().UTC()
	item := operationIncident{TaskID: "F-1", RecoveredAt: now.Format(time.RFC3339Nano), PolicyResolution: &operationPolicyResolution{Reason: "canonical_released_user_hold"}}
	for _, state := range []string{"doing", "done"} {
		if operationViewCategory(item, map[string][]backlog.Record{"F-1": {{State: state}}}, true, now) != operationHistory {
			t.Fatal("old hold reappeared as current decision")
		}
	}
	if operationViewCategory(item, map[string][]backlog.Record{"F-1": {{State: "hold"}}}, true, now) != operationHold {
		t.Fatal("current hold lost representative history")
	}
}

func TestWarningQualityUnreadableJournalStaysVisibleAsDiagnostic(t *testing.T) {
	// HOME isolates only the read-only managed-project inventory. No service
	// commands or LaunchAgent helpers are called by this snapshot fixture.
	t.Setenv("HOME", t.TempDir())
	registryHome := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", registryHome)
	project := testOperationProject(t)
	registry, _ := json.Marshal(map[string]any{"projects": []map[string]string{{"path": project}}})
	if err := os.WriteFile(filepath.Join(registryHome, "projects.json"), registry, 0600); err != nil {
		t.Fatal(err)
	}
	path := operationJournalPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("invalid-json"), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot := operationSnapshot(project)
	diagnostics := snapshot["diagnostics"].([]operationIncident)
	if len(diagnostics) != 1 || diagnostics[0].Category != operationDiagnostic || diagnostics[0].Quality != "verification_required" {
		t.Fatalf("read failure disappeared: %+v", diagnostics)
	}
	if len(snapshot["active"].([]operationIncident)) != 0 {
		t.Fatal("parse failure inferred an operating state")
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "invalid-json" {
		t.Fatal("snapshot changed corrupt source")
	}
}
