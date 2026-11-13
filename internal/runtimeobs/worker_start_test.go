package runtimeobs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestA72WorkerStartExactCanonicalBindingAndRetry(t *testing.T) {
	store, _ := t3Fixture(t)
	project, source := t.TempDir(), t.TempDir()
	now := time.Date(2026, 11, 12, 1, 10, 0, 0, time.UTC)
	_, err := RecordAssignment(project, "assignment", "A-72", "/root/controller/isanghaessi", now.Add(-15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	in := WorkerStart{project, source, "A-72", "assignment", "/root/controller/isanghaessi", testT3Selector()}
	observe := func(p string, s T3Selector, n time.Time) (Attempt, T3Receipt, error) {
		return observeT3Execution(p, s, n, store.read)
	}
	failed, err := observeWorkerStart(in, now, func(string, T3Selector, time.Time) (Attempt, T3Receipt, error) {
		return Attempt{}, T3Receipt{}, errors.New("transport unavailable")
	})
	if err == nil || failed.Status != "collection_gap" || failed.Stage != "observation" {
		t.Fatalf("failure hidden %+v %v", failed, err)
	}
	got, err := observeWorkerStart(in, now, observe)
	if err != nil || got.Status != "bound" || got.Attempt.BindingEvidence["assignment_id"] != "assignment" || got.Attempt.TaskID != "A-72" {
		t.Fatalf("retry %+v %v", got, err)
	}
	if got.Context.CanonicalProject == got.Context.SourceWorktree || got.Attempt.NativeStartedAt != "2026-11-12T01:00:00Z" || got.ReceivedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("mixed timing or project %+v", got)
	}
	again, err := observeWorkerStart(in, now.Add(time.Second), observe)
	if err != nil || again.Attempt.AttemptID != got.Attempt.AttemptID {
		t.Fatalf("duplicate START created attempt %+v %v", again, err)
	}
	ledger, err := BuildLedger(project, 20, now.Add(time.Second))
	if err != nil || len(ledger.Attempts) != 1 {
		t.Fatalf("duplicate %+v %v", ledger, err)
	}
	if _, err := os.Stat(filepath.Join(source, "_task_mecca")); !os.IsNotExist(err) {
		t.Fatal("source worktree used as operating ledger")
	}
}

func TestA72WorkerStartMissingMismatchedIdentityFailsBeforeWrite(t *testing.T) {
	store, _ := t3Fixture(t)
	project := t.TempDir()
	now := time.Date(2026, 11, 12, 1, 10, 0, 0, time.UTC)
	_, err := RecordAssignment(project, "assignment", "A-72", "/root/controller/isanghaessi", now)
	if err != nil {
		t.Fatal(err)
	}
	base := WorkerStart{project, t.TempDir(), "A-72", "assignment", "/root/controller/isanghaessi", testT3Selector()}
	for _, tc := range []struct {
		name   string
		change func(*WorkerStart)
	}{
		{"missingNative", func(i *WorkerStart) { i.Selector.SessionID = "" }},
		{"nativeAsT3", func(i *WorkerStart) { i.Selector.ThreadID = i.Selector.SessionID }},
		{"wrongT3", func(i *WorkerStart) { i.Selector.ThreadID = "other" }},
		{"wrongNative", func(i *WorkerStart) { i.Selector.SessionID = "other"; i.Selector.RuntimeAgentID = "other" }},
		{"wrongTask", func(i *WorkerStart) { i.TaskID = "B-633" }},
		{"wrongRole", func(i *WorkerStart) { i.WorkerPath = "/root/controller" }},
		{"wrongWorker", func(i *WorkerStart) { i.WorkerPath = "/root/controller/other" }},
		{"missingAssignment", func(i *WorkerStart) { i.AssignmentID = "missing" }},
		{"sourceAsCanonical", func(i *WorkerStart) { i.CanonicalProject = i.SourceWorktree }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			tc.change(&in)
			got, err := observeWorkerStart(in, now, func(p string, s T3Selector, n time.Time) (Attempt, T3Receipt, error) {
				return observeT3Execution(p, s, n, store.read)
			})
			if err == nil || got.Status != "collection_gap" {
				t.Fatalf("accepted %+v", got)
			}
			ledger, err := BuildLedger(project, 20, now)
			if err != nil || len(ledger.Attempts) != 0 {
				t.Fatalf("bad identity mutated runtime %+v %v", ledger, err)
			}
		})
	}
}

func TestA72WorkerStartBindingFailurePreservesObservationAndRetries(t *testing.T) {
	store, _ := t3Fixture(t)
	project := t.TempDir()
	now := time.Date(2026, 11, 12, 1, 10, 0, 0, time.UTC)
	if _, err := RecordAssignment(project, "assignment", "A-72", "/root/controller/isanghaessi", now); err != nil {
		t.Fatal(err)
	}
	in := WorkerStart{project, t.TempDir(), "A-72", "assignment", "/root/controller/isanghaessi", testT3Selector()}
	journal := assignmentJournalPath(project)
	first := true
	observe := func(p string, s T3Selector, n time.Time) (Attempt, T3Receipt, error) {
		a, r, err := observeT3Execution(p, s, n, store.read)
		if err == nil && first {
			first = false
			if e := os.Rename(journal, journal+".temporarily-unavailable"); e != nil {
				t.Fatal(e)
			}
		}
		return a, r, err
	}
	failed, err := observeWorkerStart(in, now, observe)
	if err == nil || failed.Stage != "binding" || failed.Status != "collection_gap" || failed.Attempt.AttemptID == "" {
		t.Fatalf("partial write hidden %+v %v", failed, err)
	}
	if err = os.Rename(journal+".temporarily-unavailable", journal); err != nil {
		t.Fatal(err)
	}
	retry, err := observeWorkerStart(in, now.Add(time.Second), observe)
	if err != nil || retry.Status != "bound" || retry.Attempt.AttemptID != failed.Attempt.AttemptID {
		t.Fatalf("retry %+v %v", retry, err)
	}
}
