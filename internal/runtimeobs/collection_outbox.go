package runtimeobs

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/projectguard"
)

// CollectionReceipt records delivery, not permission, native work or Activity.
// A request prepared before identity discovery deliberately contains no actor.
// Pending/failed records survive interruption; a later ACK never erases them.
type CollectionReceipt struct {
	Activity        *CollectionActivity `json:"activity,omitempty"`
	ID              string              `json:"id"`
	Key             string              `json:"key"`
	RequestID       string              `json:"request_id,omitempty"`
	Status          string              `json:"status"`
	At              time.Time           `json:"at"`
	Boundary        *OperationBoundary  `json:"boundary,omitempty"`
	Error           string              `json:"error,omitempty"`
	Owner           string              `json:"owner"`
	ResumeCondition string              `json:"resume_condition"`
}

type CollectionActivity struct {
	Role          string `json:"role"`
	EventID       string `json:"event_id"`
	TaskID        string `json:"task_id"`
	AgentID       string `json:"agent_id"`
	AttemptID     string `json:"attempt_id"`
	PayloadSHA256 string `json:"payload_sha256"`
}

func CollectionOutboxPath(project string) string {
	return filepath.Join(project, "_task_mecca", ".runtime", "collection-outbox.jsonl")
}

func collectionPathSafe(project string) error {
	if err := boundaryPathSafe(project); err != nil {
		return err
	}
	if st, err := os.Lstat(CollectionOutboxPath(project)); err == nil {
		if !st.Mode().IsRegular() {
			return errors.New("collection outbox must be a regular file")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func ReadCollectionReceipts(project string) ([]CollectionReceipt, error) {
	if err := collectionPathSafe(project); err != nil {
		return nil, err
	}
	f, err := os.Open(CollectionOutboxPath(project))
	if os.IsNotExist(err) {
		return []CollectionReceipt{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := []CollectionReceipt{}
	var failures []error
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 2<<20)
	for line := 1; s.Scan(); line++ {
		var r CollectionReceipt
		if err := json.Unmarshal(s.Bytes(), &r); err != nil || validateCollectionReceipt(r) != nil || r.ID != collectionReceiptID(r) {
			failures = append(failures, fmt.Errorf("invalid collection receipt line %d", line))
			continue
		}
		out = append(out, r)
	}
	return out, errors.Join(append(failures, s.Err())...)
}

func collectionReceiptID(r CollectionReceipt) string {
	r.ID = ""
	raw, _ := json.Marshal(r)
	sum := sha256.Sum256(raw)
	return "collection-" + hex.EncodeToString(sum[:])
}

func validateCollectionReceipt(r CollectionReceipt) error {
	if r.Activity != nil {
		a := r.Activity
		if a.EventID == "" || a.TaskID == "" || a.AgentID == "" || a.AttemptID == "" || len(a.PayloadSHA256) != 64 {
			return errors.New("exact Activity reference and content hash required")
		}
	}
	if strings.TrimSpace(r.Key) == "" || r.At.IsZero() || r.Owner == "" || r.ResumeCondition == "" {
		return errors.New("collection receipt needs stable key, current observation, owner and resume condition")
	}
	if r.Status != "pending" && r.Status != "failed" && r.Status != "ack" {
		return errors.New("invalid collection delivery status")
	}
	if r.Status == "failed" && r.Error == "" {
		return errors.New("original collection error required")
	}
	if r.Boundary != nil {
		if err := validateBoundary(*r.Boundary); err != nil {
			return err
		}
	}
	return nil
}

// RecordCollectionReceipt is append-only and synchronous. It is used by the
// existing trusted collector paths, never as a replacement attribution gate.
func RecordCollectionReceipt(project string, r CollectionReceipt) (CollectionReceipt, error) {
	if err := validateCollectionReceipt(r); err != nil {
		return r, err
	}
	r.ID = collectionReceiptID(r)
	raw, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	if len(raw) > 2<<20-2 {
		return r, errors.New("collection receipt exceeds journal row size")
	}
	if err := collectionPathSafe(project); err != nil {
		return r, err
	}
	release, err := projectguard.AcquireStateLock(project, "collection-outbox")
	if err != nil {
		return r, err
	}
	defer release()
	if err := collectionPathSafe(project); err != nil {
		return r, err
	}
	if _, err := ReadCollectionReceipts(project); err != nil {
		return r, err
	}
	f, err := os.OpenFile(CollectionOutboxPath(project), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return r, err
	}
	raw = append(raw, '\n')
	n, err := f.Write(raw)
	if err == nil && n != len(raw) {
		err = errors.New("short collection receipt append")
	}
	if err == nil {
		err = f.Sync()
	}
	return r, errors.Join(err, f.Close())
}

func BoundaryCollectionReceipt(b OperationBoundary, status, originalError string, now time.Time) CollectionReceipt {
	copy := b
	return CollectionReceipt{Key: "boundary:" + boundaryID(b), RequestID: b.RequestID, Status: status, At: now.UTC(), Boundary: &copy, Error: originalError, Owner: "collector:" + b.Role, ResumeCondition: "Verify the same exact native/task/operation and retry the same boundary key; reconcile an uncertain append before ACK. Preserve original failure; never infer missing Activity or replay historical duration."}
}

func RequestCollectionReceipt(request, key, status, originalError string, now time.Time) CollectionReceipt {
	return CollectionReceipt{Key: key, RequestID: request, Status: status, At: now.UTC(), Error: originalError, Owner: "registration_collector", ResumeCondition: "Verify current native identity and supported registration request; resume this request with its original error. Preparation has no authoritative native/task association."}
}
