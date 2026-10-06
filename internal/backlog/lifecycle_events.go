package backlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// LifecycleTransition is a reported fact, not a deduction from a filename.
// The caller supplies a stable ID before changing the backlog state so that
// retries, concurrent writers and restarts refer to the same fact.
type LifecycleTransition struct {
	EventID        string `json:"event_id"`
	TaskID         string `json:"task_id"`
	Kind           string `json:"kind"`
	OccurredAt     string `json:"occurred_at"`
	RecordedAt     string `json:"recorded_at"`
	Actor          string `json:"actor"`
	EvidenceSource string `json:"evidence_source"`
	EvidenceRef    string `json:"evidence_ref,omitempty"`
	AssignmentID   string `json:"assignment_id,omitempty"`
	AttemptID      string `json:"attempt_id,omitempty"`
}

type LifecycleFinding struct {
	Code    string `json:"code"`
	EventID string `json:"event_id,omitempty"`
	Message string `json:"message"`
}

type LifecycleEventScan struct {
	Events   []LifecycleTransition `json:"events"`
	Findings []LifecycleFinding    `json:"findings"`
}

var lifecycleEventIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func lifecycleEventsDir(project string) string {
	return filepath.Join(project, "_task_mecca", "data", "lifecycle", "events")
}

func lifecycleTransitionPath(project, eventID string) string {
	return filepath.Join(lifecycleEventsDir(project), eventID, "event.json")
}

func validateLifecycleTransition(event LifecycleTransition) error {
	if !lifecycleEventIDPattern.MatchString(event.EventID) {
		return errors.New("stable event ID must contain only letters, numbers, '-' or '_'")
	}
	if event.TaskID == "" || event.Actor == "" || event.EvidenceSource == "" {
		return errors.New("task ID, actor and evidence source are required")
	}
	switch event.Kind {
	case "registered", "assigned", "started", "waiting", "resumed", "completed":
	default:
		return fmt.Errorf("unsupported lifecycle kind: %s", event.Kind)
	}
	if _, err := time.Parse(time.RFC3339Nano, event.OccurredAt); err != nil {
		return fmt.Errorf("invalid occurrence time: %w", err)
	}
	if event.Kind == "started" && event.AttemptID == "" && event.EvidenceSource != "worker_report" {
		return errors.New("started requires a runtime attempt or explicit Worker report")
	}
	return nil
}

// RecordLifecycleTransition writes one immutable file per event. A directory
// claim is atomic; readers ignore a claim without event.json and report it as
// interrupted rather than inventing a transition.
func RecordLifecycleTransition(project string, event LifecycleTransition, now time.Time) (LifecycleTransition, error) {
	event.TaskID = strings.ToUpper(strings.TrimSpace(event.TaskID))
	event.Actor = strings.TrimSpace(event.Actor)
	event.EventID = strings.TrimSpace(event.EventID)
	event.OccurredAt = strings.TrimSpace(event.OccurredAt)
	if event.OccurredAt == "" {
		event.OccurredAt = now.UTC().Format(time.RFC3339Nano)
	}
	if err := validateLifecycleTransition(event); err != nil {
		return LifecycleTransition{}, err
	}
	path := lifecycleTransitionPath(project, event.EventID)
	if err := os.MkdirAll(lifecycleEventsDir(project), 0755); err != nil {
		return LifecycleTransition{}, err
	}
	if err := os.Mkdir(filepath.Dir(path), 0755); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return LifecycleTransition{}, err
		}
		var data []byte
		var readErr error
		for i := 0; i < 50; i++ {
			data, readErr = os.ReadFile(path)
			if readErr == nil || !errors.Is(readErr, os.ErrNotExist) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if readErr != nil {
			return LifecycleTransition{}, fmt.Errorf("event %s has an incomplete claim: %w", event.EventID, readErr)
		}
		var old LifecycleTransition
		if readErr = json.Unmarshal(data, &old); readErr != nil {
			return LifecycleTransition{}, readErr
		}
		if old.TaskID != event.TaskID || old.Kind != event.Kind || old.Actor != event.Actor || old.EvidenceSource != event.EvidenceSource || old.EvidenceRef != event.EvidenceRef || old.AssignmentID != event.AssignmentID || old.AttemptID != event.AttemptID {
			return LifecycleTransition{}, fmt.Errorf("event ID %s conflicts with existing lifecycle fact", event.EventID)
		}
		return old, nil
	}
	event.RecordedAt = now.UTC().Format(time.RFC3339Nano)
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return LifecycleTransition{}, err
	}
	temp := filepath.Join(filepath.Dir(path), "event.tmp")
	f, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return LifecycleTransition{}, err
	}
	if _, err = f.Write(append(data, '\n')); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return LifecycleTransition{}, err
	}
	if err := os.Rename(temp, path); err != nil {
		return LifecycleTransition{}, err
	}
	return event, nil
}

func ReadLifecycleTransitions(project string) (LifecycleEventScan, error) {
	out := LifecycleEventScan{Events: []LifecycleTransition{}, Findings: []LifecycleFinding{}}
	entries, err := os.ReadDir(lifecycleEventsDir(project))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, readErr := os.ReadFile(lifecycleTransitionPath(project, entry.Name()))
		if readErr != nil {
			out.Findings = append(out.Findings, LifecycleFinding{Code: "incomplete_event", EventID: entry.Name(), Message: "전환 기록이 중단되어 사실과 시각을 확인해야 합니다."})
			continue
		}
		var event LifecycleTransition
		if readErr = json.Unmarshal(data, &event); readErr != nil || event.EventID != entry.Name() || validateLifecycleTransition(event) != nil {
			out.Findings = append(out.Findings, LifecycleFinding{Code: "invalid_event", EventID: entry.Name(), Message: "전환 기록을 검증할 수 없어 원본 확인이 필요합니다."})
			continue
		}
		out.Events = append(out.Events, event)
	}
	sort.Slice(out.Events, func(i, j int) bool {
		if out.Events[i].OccurredAt == out.Events[j].OccurredAt {
			return out.Events[i].EventID < out.Events[j].EventID
		}
		return out.Events[i].OccurredAt < out.Events[j].OccurredAt
	})
	return out, nil
}

func LifecycleEventID(taskID, kind, key string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(taskID)) + "\x00" + kind + "\x00" + key))
	return "lifecycle-" + hex.EncodeToString(sum[:])[:24]
}
