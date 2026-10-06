package backlog

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestDurableLifecycleConcurrentRetryAndInterruptedClaim(t *testing.T) {
	project := t.TempDir()
	at := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	input := LifecycleTransition{EventID: "registration-a1", TaskID: "A-1", Kind: "registered", OccurredAt: at.Format(time.RFC3339Nano), Actor: "/root/registrar", EvidenceSource: "registrar_report"}
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := RecordLifecycleTransition(project, input, at); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	scan, err := ReadLifecycleTransitions(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Events) != 1 || scan.Events[0].EventID != "registration-a1" || scan.Events[0].OccurredAt != input.OccurredAt {
		t.Fatalf("duplicate or changed event after concurrent retry: %+v", scan)
	}
	// Simulate a crash after the atomic directory claim but before event.json.
	if err := os.Mkdir(filepath.Join(lifecycleEventsDir(project), "incomplete-a1"), 0755); err != nil {
		t.Fatal(err)
	}
	scan, err = ReadLifecycleTransitions(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Events) != 1 || len(scan.Findings) != 1 || scan.Findings[0].Code != "incomplete_event" {
		t.Fatalf("interrupted write was hidden or fabricated: %+v", scan)
	}
}

func TestDurableLifecycleRejectsConflictingEventID(t *testing.T) {
	project := t.TempDir()
	at := time.Now().UTC()
	first := LifecycleTransition{EventID: "same-id", TaskID: "A-1", Kind: "assigned", Actor: "/root/controller", EvidenceSource: "controller_report"}
	if _, err := RecordLifecycleTransition(project, first, at); err != nil {
		t.Fatal(err)
	}
	second := first
	second.TaskID = "A-2"
	if _, err := RecordLifecycleTransition(project, second, at); err == nil {
		t.Fatal("conflicting event ID was accepted")
	}
	if _, err := RecordLifecycleTransition(project, LifecycleTransition{EventID: "fake-start", TaskID: "A-2", Kind: "started", Actor: "/root/controller", EvidenceSource: "filename_observation"}, at); err == nil {
		t.Fatal("filename observation was promoted into Worker execution")
	}
}
