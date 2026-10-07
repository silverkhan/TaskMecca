package handoff

import (
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"

	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"strings"
	"time"
)

const lockStaleAfter = 30 * time.Second

func JournalPath(project string) string {
	return filepath.Join(project, "_task_mecca", ".runtime", "handoffs", "events.jsonl")
}

func lockPath(project string) string { return JournalPath(project) + ".lock" }

func withLock(project string, fn func() error) error {
	path := lockPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			defer os.Remove(path)
			return fn()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > lockStaleAfter {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("handoff journal lock timeout: %s", path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func appendRecordUnlocked(project string, record Record) error {
	path := JournalPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(payload)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func BuildLedger(project string, now time.Time) (Ledger, error) {
	return buildLedgerUnlocked(project, now)
}

func buildLedgerUnlocked(project string, now time.Time) (Ledger, error) {
	return handoffjournal.BuildLedger(project, now)
}

func Inspect(project, handoffID string, now time.Time) (Handoff, bool, error) {
	ledger, err := BuildLedger(project, now)
	if err != nil {
		return Handoff{}, false, err
	}
	row, ok := ledger.Handoffs[strings.TrimSpace(handoffID)]
	return row, ok, nil
}
