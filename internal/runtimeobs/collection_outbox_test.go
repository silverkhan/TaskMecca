package runtimeobs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestA102ManagedAdmissionNeedsExactCurrentOperationACK(t *testing.T) {
	p := t.TempDir()
	now := time.Now().UTC()
	if err := AppendExecutionEvent(p, ExecutionEvent{EventID: "fixture-state", EventKind: "state", AttemptID: "fixture-run", Provider: "codex", SessionID: "fixture-native", RuntimeAgentID: "fixture-native", State: StateRunning, ObservedAt: now.Format(time.RFC3339Nano), EvidenceSource: EvidenceHook, ObservationQuality: QualityObserved}); err != nil {
		t.Fatal(err)
	}
	if _, err := BindAttempt(p, "fixture-run", "FIXTURE-1", "/root/registrar", "explicit", "", nil, now); err != nil {
		t.Fatal(err)
	}
	check := func() error {
		return RequireCollectionACK(p, "FIXTURE-1", "fixture-native", "fixture-run", "registrar", now)
	}
	if check() == nil {
		t.Fatal("no operation admitted")
	}
	b := OperationBoundary{Key: "fixture-start", OperationID: "fixture-operation", Provider: "codex", NativeID: "fixture-native", AttemptID: "fixture-run", Role: "registrar", Kind: "start", TaskIDs: []string{"FIXTURE-1"}, Source: "offline_fixture", Evidence: "synthetic explicit fixture start"}
	b, err := PrepareOperationBoundary(b, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RecordCollectionReceipt(p, BoundaryCollectionReceipt(b, "pending", "", now)); err != nil {
		t.Fatal(err)
	}
	if check() == nil {
		t.Fatal("pending ACK admitted")
	}
	if _, _, err := AppendOperationBoundary(p, b, now); err != nil {
		t.Fatal(err)
	}
	if err := check(); err != nil {
		t.Fatal("ACK did not admit own operation", err)
	}
	phase := b
	phase.Key = "fixture-next"
	phase.Kind = "phase"
	phase, err = PrepareOperationBoundary(phase, now.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RecordCollectionReceipt(p, BoundaryCollectionReceipt(phase, "failed", "fixture original phase append failed", now)); err != nil {
		t.Fatal(err)
	}
	if check() == nil {
		t.Fatal("unresolved phase failure admitted")
	}
	if _, _, err := AppendOperationBoundary(p, phase, now.Add(2*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if err := check(); err != nil {
		t.Fatal("same-key recovery ACK did not admit", err)
	}
	if RequireCollectionACK(p, "FIXTURE-1", "fixture-native", "fixture-foreign-run", "registrar", now) == nil {
		t.Fatal("foreign episode admitted")
	}
	rows, err := ReadCollectionReceipts(p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		if r.Status == "failed" && r.Error == "fixture original phase append failed" {
			found = true
		}
	}
	if !found {
		t.Fatal("recovery erased original failure")
	}
}

func TestA98OutboxPendingFailureRetryAndACKPreserveOriginal(t *testing.T) {
	p := t.TempDir()
	now := time.Now().UTC()
	b := boundaryFixture()
	// Force the actual operation append to fail after durable preparation.
	if err := os.MkdirAll(OperationBoundaryPath(p), 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := AppendOperationBoundary(p, b, now); err == nil {
		t.Fatal("failed append claimed success")
	}
	rows, err := ReadCollectionReceipts(p)
	if err != nil || len(rows) != 2 || rows[0].Status != "pending" || rows[1].Status != "failed" || rows[1].Error == "" {
		t.Fatalf("failure not durable %v %+v", err, rows)
	}
	originalError := rows[1].Error
	if err := os.Remove(OperationBoundaryPath(p)); err != nil {
		t.Fatal(err)
	}
	actual, inserted, err := AppendOperationBoundary(p, b, now.Add(time.Minute))
	if err != nil || !inserted || !actual.ObservedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("retry backdated/failed %+v %v", actual, err)
	}
	if _, again, err := AppendOperationBoundary(p, b, now.Add(2*time.Minute)); err != nil || again {
		t.Fatalf("retry duplicated boundary %v", err)
	}
	rows, err = ReadCollectionReceipts(p)
	if err != nil || rows[1].Error != originalError || rows[len(rows)-1].Status != "ack" {
		t.Fatal("failure erased or ACK absent", err, rows)
	}
	boundaries, err := ReadOperationBoundaries(p)
	if err != nil || len(boundaries) != 1 {
		t.Fatal("operation duplicated", err, boundaries)
	}
}

func TestA98RequestBeforeNativeHasNoAuthoritativeActor(t *testing.T) {
	p := t.TempDir()
	r := RequestCollectionReceipt("request", "stable-request-key", "pending", "", time.Now().UTC())
	if _, err := RecordCollectionReceipt(p, r); err != nil {
		t.Fatal(err)
	}
	rows, err := ReadCollectionReceipts(p)
	if err != nil || len(rows) != 1 || rows[0].Boundary != nil || rows[0].Activity != nil || rows[0].Status != "pending" {
		t.Fatal("request acquired actor", err, rows)
	}
	// A process exit after this durable pending record is explicitly recoverable,
	// and reading it neither closes execution nor manufactures an ACK.
	if rows[0].Owner == "" || rows[0].ResumeCondition == "" {
		t.Fatal("recovery responsibility missing")
	}
}

func TestA98OutboxWriteDeniedCorruptionAndSymlinkFailClosed(t *testing.T) {
	p := t.TempDir()
	foreign := filepath.Join(t.TempDir(), "preserve")
	if err := os.WriteFile(foreign, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(CollectionOutboxPath(p)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, CollectionOutboxPath(p)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := AppendOperationBoundary(p, boundaryFixture(), time.Now()); err == nil {
		t.Fatal("unwritable intent allowed append")
	}
	if _, err := os.Stat(OperationBoundaryPath(p)); !os.IsNotExist(err) {
		t.Fatal("boundary written despite missing durable intent")
	}
	raw, _ := os.ReadFile(foreign)
	if string(raw) != "original" {
		t.Fatal("foreign journal modified")
	}
	if err := os.Remove(CollectionOutboxPath(p)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(CollectionOutboxPath(p), []byte("{invalid\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCollectionReceipts(p); err == nil {
		t.Fatal("corrupt pending hidden")
	}
	if _, _, err := AppendOperationBoundary(p, boundaryFixture(), time.Now()); err == nil {
		t.Fatal("corrupt durable intent ignored")
	}
}

func TestA98OutboxDetectsValidJSONContentCorruption(t *testing.T) {
	p := t.TempDir()
	r, err := RecordCollectionReceipt(p, RequestCollectionReceipt("request", "key", "pending", "", time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	r.Status = "ack"
	raw, _ := json.Marshal(r)
	if err := os.WriteFile(CollectionOutboxPath(p), append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCollectionReceipts(p); err == nil {
		t.Fatal("altered receipt became a successful ACK")
	}
	if _, err := RecordCollectionReceipt(p, RequestCollectionReceipt("request", "key", "ack", "", time.Now())); err == nil {
		t.Fatal("corrupt receipt overwritten by retry")
	}
}
