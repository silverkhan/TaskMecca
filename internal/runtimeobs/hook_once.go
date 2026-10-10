package runtimeobs

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/silverkhan/TaskMecca/internal/projectguard"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type hookClaim struct {
	EventID    string `json:"event_id"`
	ObservedAt string `json:"observed_at"`
	State      string `json:"state"`
}

// appendHookEventOnce serializes global and project hook invocations across
// processes. A pending marker is recovered against the journal after a crash;
// a completed marker prevents a second raw record for the same provider event.
func appendHookEventOnce(project string, event *ExecutionEvent) error {
	releaseGuard, guardErr := projectguard.AcquireWrite(project)
	if guardErr != nil {
		return guardErr
	}
	defer releaseGuard()

	release, err := lockHookEvents(project)
	if err != nil {
		return err
	}
	defer release()
	if event.EventID == "" {
		event.EventID = eventIDFor(*event)
	}
	// Tool hooks without a provider tool-use ID may legitimately carry the same
	// payload repeatedly. Do not collapse those events by timing alone.
	if event.EventKind == "activity" && event.ToolUseID == "" {
		return AppendExecutionEvent(project, *event)
	}
	key := sha256.Sum256([]byte(strings.Join([]string{event.Provider, event.AttemptID, event.HookEventName, event.ToolUseID, event.RawSHA256}, "\x00")))
	markerID := hex.EncodeToString(key[:])
	marker := filepath.Join(ExecutionRootPath(project), "hook_ids", markerID[:2], markerID)
	data, err := os.ReadFile(marker)
	if err == nil {
		var prior hookClaim
		if json.Unmarshal(data, &prior) == nil {
			previousAt, firstErr := time.Parse(time.RFC3339Nano, prior.ObservedAt)
			currentAt, secondErr := time.Parse(time.RFC3339Nano, event.ObservedAt)
			if firstErr == nil && secondErr == nil && currentAt.Sub(previousAt) > -5*time.Second && currentAt.Sub(previousAt) < 5*time.Second {
				if prior.State == "done" {
					event.EventID = prior.EventID
					return nil
				}
				exists, scanErr := rawHookEventExists(project, prior.EventID)
				if scanErr != nil {
					return scanErr
				}
				if exists {
					event.EventID = prior.EventID
					prior.State = "done"
					updated, _ := json.Marshal(prior)
					return os.WriteFile(marker, updated, 0600)
				}
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0700); err != nil {
		return err
	}
	claim := hookClaim{EventID: event.EventID, ObservedAt: event.ObservedAt, State: "pending"}
	claimData, _ := json.Marshal(claim)
	if err := os.WriteFile(marker, claimData, 0600); err != nil {
		return err
	}
	if err := AppendExecutionEvent(project, *event); err != nil {
		return err
	}
	claim.State = "done"
	claimData, _ = json.Marshal(claim)
	return os.WriteFile(marker, claimData, 0600)
}

func rawHookEventExists(project, eventID string) (bool, error) {
	files, err := executionEventFiles(project)
	if err != nil {
		return false, err
	}
	for _, path := range files {
		file, openErr := os.Open(path)
		if openErr != nil {
			return false, openErr
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
		found := false
		for scanner.Scan() {
			var event ExecutionEvent
			if json.Unmarshal(scanner.Bytes(), &event) == nil && event.EventID == eventID {
				found = true
				break
			}
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if scanErr != nil {
			return false, scanErr
		}
		if closeErr != nil {
			return false, closeErr
		}
		if found {
			return true, nil
		}
	}
	return false, nil
}
