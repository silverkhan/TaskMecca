package activity

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// ReviewerPresentation is additive evidence, never an activity measurement.
// NativeID must match before consumers join an attempt to a session/row.
type ReviewerPresentation struct {
	Version   int                                  `json:"version"`
	ByAttempt map[string]ReviewerPresentationEntry `json:"by_attempt"`
	Task      ReviewerTaskPresentation             `json:"task"`
}
type ReviewerTaskPresentation struct {
	Necessity   string `json:"necessity"`
	ActualState string `json:"actual_state"`
	Reason      string `json:"reason"`
}
type ReviewerPresentationEntry struct {
	AttemptID         string            `json:"attempt_id"`
	NativeID          string            `json:"native_id"`
	TaskID            string            `json:"task_id"`
	OfficialName      string            `json:"official_name"`
	NameVerified      bool              `json:"name_verified"`
	NameSource        string            `json:"name_source"`
	NameObservedAt    string            `json:"name_observed_at"`
	Purpose           string            `json:"purpose"`
	PurposeVerified   bool              `json:"purpose_verified"`
	Necessity         string            `json:"necessity"`
	ActualState       string            `json:"actual_state"`
	ChainID           string            `json:"chain_id"`
	ReviewRound       *int              `json:"review_round"`
	SessionGeneration *int              `json:"session_generation"`
	ContextStatus     string            `json:"context_status"`
	ContinuityRef     string            `json:"continuity_ref"`
	ContextRef        string            `json:"context_ref"`
	SourceDelta       []json.RawMessage `json:"source_delta"`
	Errors            []string          `json:"errors"`
}

// PersistedNameEvidence reads journal bytes only. Invalid latest observations
// replace earlier names. No provider process, title parser or alias fallback.
func PersistedNameEvidence(project string) map[string]runtimeobs.RootNameObservation {
	out := map[string]runtimeobs.RootNameObservation{}
	files, e := filepath.Glob(filepath.Join(runtimeobs.ExecutionRawDir(project), "*.jsonl"))
	if e != nil || len(files) > 4096 {
		return out
	}
	files = append(files, runtimeobs.ExecutionLegacyJournalPath(project))
	total := int64(0)
	for _, path := range files {
		f, e := os.Open(path)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return map[string]runtimeobs.RootNameObservation{}
		}
		info, e := f.Stat()
		if e != nil {
			f.Close()
			return map[string]runtimeobs.RootNameObservation{}
		}
		total += info.Size()
		if total > 32<<20 {
			f.Close()
			return map[string]runtimeobs.RootNameObservation{}
		}
		scanner := bufio.NewScanner(io.LimitReader(f, 32<<20))
		scanner.Buffer(make([]byte, 4096), 2<<20)
		for scanner.Scan() {
			var event runtimeobs.ExecutionEvent
			if json.Unmarshal(scanner.Bytes(), &event) != nil {
				f.Close()
				return map[string]runtimeobs.RootNameObservation{}
			}
			if event.EventKind != "session_metadata" || event.RootName == nil || event.SessionID == "" || event.Provider == "" {
				continue
			}
			key := event.Provider + "\x00" + event.SessionID
			o := *event.RootName
			old, ok := out[key]
			if ok && old.CheckedAt > o.CheckedAt {
				continue
			}
			out[key] = o
		}
		e = scanner.Err()
		f.Close()
		if e != nil {
			return map[string]runtimeobs.RootNameObservation{}
		}
	}
	return out
}
func NewReviewerPresentation(project, task string, attempts []runtimeobs.Attempt, events []Event, policies map[string]*rolepolicy.Policy, verified map[string]ReviewerPresentationEntry) ReviewerPresentation {
	out := ReviewerPresentation{Version: 1, ByAttempt: map[string]ReviewerPresentationEntry{}, Task: ReviewerTaskPresentation{"unknown", "unknown", "review evidence unavailable"}}
	if p := policies[task]; p != nil && p.ReviewRequirement != nil {
		if d, e := p.ReviewRequirement.Decide(); e == nil {
			out.Task.Necessity = string(d.Level)
			out.Task.Reason = d.Reason
			if d.Status == "omitted" {
				out.Task.ActualState = "omitted"
			}
		}
	}
	names := PersistedNameEvidence(project)
	latestTaskAttempt := ""
	for _, a := range attempts {
		if a.AttemptID == "" || a.SessionID == "" || a.BindingState != runtimeobs.BindingBound || task != "" && !strings.EqualFold(a.TaskID, task) {
			continue
		}
		v := ReviewerPresentationEntry{AttemptID: a.AttemptID, NativeID: a.SessionID, TaskID: a.TaskID, Purpose: "unknown", Necessity: "unknown", ActualState: "unknown", ContextStatus: "unknown", SourceDelta: []json.RawMessage{}, Errors: []string{}}
		if supplied, ok := verified[a.AttemptID]; ok && supplied.NativeID == a.SessionID && supplied.AttemptID == a.AttemptID && supplied.TaskID == a.TaskID {
			v = supplied
		}
		v.OfficialName = ""
		v.NameVerified = false
		v.NameSource = ""
		v.NameObservedAt = ""
		if v.ActualState != "omitted" {
			v.ActualState = "unknown"
		}
		o, ok := names[a.Provider+"\x00"+a.SessionID]
		at, err := time.Parse(time.RFC3339Nano, o.CheckedAt)
		if (a.EndedAt == "" || o.CheckedAt <= a.EndedAt) && ok && o.Version == 1 && o.Status == "verified" && o.IdentityVerified && o.HostProjectID != "" && o.T3ThreadID != "" && o.HostStatus == "verified" && strings.TrimSpace(o.Name) != "" && (o.Source == "provider_name" || o.Source == "t3_title") && err == nil && !at.After(time.Now().UTC()) && a.NativeReceipt != nil && a.NativeReceipt.SessionID == a.SessionID && a.NativeReceipt.ThreadID == o.T3ThreadID {
			v.OfficialName = o.Name
			v.NameVerified = true
			v.NameSource = o.Source
			v.NameObservedAt = o.CheckedAt
		}
		// Only actual review Activity changes actual_state; running is not review.
		if v.PurposeVerified {
			for _, e := range events {
				if e.TaskID != a.TaskID || e.RuntimeAttemptID != a.AttemptID || e.AgentID != a.RuntimeAgentID || (e.Type != ControllerReview && e.Type != ControllerReReview) {
					continue
				}
				switch e.Action {
				case Start, Switch:
					v.ActualState = "reviewing"
				case Stop:
					v.ActualState = "result_recorded"
				}
			}
		}
		if a.Terminal && v.ActualState == "reviewing" {
			v.ActualState = "unknown"
			v.Errors = append(v.Errors, "terminal review has no observed result boundary")
		}
		out.ByAttempt[a.AttemptID] = v
		if a.TaskID == task && v.PurposeVerified && (latestTaskAttempt == "" || a.StartedAt > latestTaskAttempt) {
			latestTaskAttempt = a.StartedAt
			out.Task.ActualState = v.ActualState
		}
	}
	return out
}
