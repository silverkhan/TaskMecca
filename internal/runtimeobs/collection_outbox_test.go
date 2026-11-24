package runtimeobs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
