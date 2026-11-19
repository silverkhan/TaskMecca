package activity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
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
	ActorName         string            `json:"actor_name"`
	ActorNameSource   string            `json:"actor_name_source"`
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

// PersistedNameEvidence reads a bounded recent suffix of persisted metadata.
// Archive size never discards already observed matching names. It does not call
// providers or interpret name metadata as liveness or historical Activity.
func PersistedNameEvidence(project string) map[string]runtimeobs.RootNameObservation {
	names, _ := readPersistedNameEvidence(project, nil, 8<<20, 64)
	return names
}

type nameReadStats struct {
	BytesRead       int64
	FilesRead       int
	BudgetExhausted bool
}
type nameJournalFile struct {
	path  string
	order time.Time
}

func readPersistedNameEvidence(project string, wanted map[string]bool, byteBudget int64, fileBudget int) (map[string]runtimeobs.RootNameObservation, nameReadStats) {
	out := map[string]runtimeobs.RootNameObservation{}
	stats := nameReadStats{}
	paths, e := filepath.Glob(filepath.Join(runtimeobs.ExecutionRawDir(project), "*.jsonl"))
	if e != nil {
		return out, stats
	}
	paths = append(paths, runtimeobs.ExecutionLegacyJournalPath(project))
	files := []nameJournalFile{}
	for _, path := range paths {
		info, e := os.Stat(path)
		if e != nil || !info.Mode().IsRegular() {
			continue
		}
		order := info.ModTime()
		if day, e := time.Parse("2006-01-02.jsonl", filepath.Base(path)); e == nil {
			order = day.Add(24*time.Hour - time.Nanosecond)
		}
		files = append(files, nameJournalFile{path, order})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].order.Equal(files[j].order) {
			return files[i].path > files[j].path
		}
		return files[i].order.After(files[j].order)
	})
	latest := map[string]time.Time{}
	ambiguous := map[string]bool{}
	for _, entry := range files {
		if stats.FilesRead >= fileBudget || stats.BytesRead >= byteBudget {
			stats.BudgetExhausted = true
			break
		}
		f, e := os.Open(entry.path)
		if e != nil {
			break
		}
		info, e := f.Stat()
		if e != nil {
			f.Close()
			break
		}
		take := info.Size()
		if left := byteBudget - stats.BytesRead; take > left {
			take = left
		}
		if take <= 0 {
			f.Close()
			continue
		}
		data := make([]byte, int(take))
		n, e := f.ReadAt(data, info.Size()-take)
		f.Close()
		stats.FilesRead++
		stats.BytesRead += int64(n)
		if e != nil || int64(n) != take {
			break
		}
		data = data[:n]
		truncated := take < info.Size()
		// Never parse a partial leading record, or search older files after the
		// suffix budget omitted records that might contain a newer matching name.
		if truncated {
			if i := bytes.IndexByte(data, '\n'); i >= 0 {
				data = data[i+1:]
			} else {
				data = nil
			}
		}
		lines := bytes.Split(data, []byte{'\n'})
		opaqueBarrier := false
		for i := len(lines) - 1; i >= 0; i-- {
			line := bytes.TrimSpace(lines[i])
			if len(line) == 0 {
				continue
			}
			if len(line) > 2<<20 {
				opaqueBarrier = true
				break
			}
			var envelope struct {
				EventKind  string          `json:"event_kind"`
				Provider   string          `json:"provider"`
				SessionID  string          `json:"session_id"`
				ObservedAt string          `json:"observed_at"`
				RootName   json.RawMessage `json:"root_name"`
			}
			if json.Unmarshal(line, &envelope) != nil {
				opaqueBarrier = true
				break
			}
			if envelope.EventKind != "session_metadata" || len(envelope.RootName) == 0 || string(envelope.RootName) == "null" || envelope.Provider == "" || envelope.SessionID == "" {
				continue
			}
			key := envelope.Provider + "\x00" + envelope.SessionID
			if wanted != nil && !wanted[key] {
				continue
			}
			var observation runtimeobs.RootNameObservation
			decodeErr := json.Unmarshal(envelope.RootName, &observation)
			observedAt := envelope.ObservedAt
			if observedAt == "" {
				observedAt = observation.CheckedAt
			}
			at, timeErr := time.Parse(time.RFC3339Nano, observedAt)
			if timeErr != nil {
				ambiguous[key] = true
				delete(out, key)
				continue
			}
			if ambiguous[key] {
				continue
			}
			if previous, ok := latest[key]; ok && !at.After(previous) {
				continue
			}
			latest[key] = at
			// A malformed/unavailable latest snapshot invalidates only its own key.
			// Keep the tombstone so an older valid snapshot cannot revive it.
			if decodeErr != nil {
				out[key] = runtimeobs.RootNameObservation{}
				continue
			}
			out[key] = observation
		}
		if truncated {
			stats.BudgetExhausted = true
		}
		if truncated || opaqueBarrier {
			break
		}
	}
	return out, stats
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
	wanted := map[string]bool{}
	for _, a := range attempts {
		if a.BindingState == runtimeobs.BindingBound && (task == "" || strings.EqualFold(a.TaskID, task)) && a.Provider != "" && a.SessionID != "" {
			wanted[a.Provider+"\x00"+a.SessionID] = true
		}
	}
	names, _ := readPersistedNameEvidence(project, wanted, 8<<20, 64)
	latestTaskAttempt := ""
	for _, a := range attempts {
		if a.AttemptID == "" || a.SessionID == "" || a.BindingState != runtimeobs.BindingBound || task != "" && !strings.EqualFold(a.TaskID, task) {
			continue
		}
		v := ReviewerPresentationEntry{AttemptID: a.AttemptID, NativeID: a.SessionID, TaskID: a.TaskID, Purpose: "unknown", Necessity: "unknown", ActualState: "unknown", ContextStatus: "unknown", SourceDelta: []json.RawMessage{}, Errors: []string{}}
		if supplied, ok := verified[a.AttemptID]; ok && supplied.NativeID == a.SessionID && supplied.AttemptID == a.AttemptID && supplied.TaskID == a.TaskID {
			v = supplied
		}
		v.ActorName = ""
		v.ActorNameSource = ""
		if a.RuntimeAgentID != "" && a.Provider != "" && (rolepolicy.WorkerPath(a.AgentPath) || rolepolicy.ReviewerPath(a.AgentPath)) {
			v.ActorName = filepath.Base(a.AgentPath)
			v.ActorNameSource = "bound_runtime_agent_path"
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
		if ok && o.Version == 1 && o.Status == "verified" && o.IdentityVerified && o.HostProjectID != "" && o.T3ThreadID != "" && o.HostStatus == "verified" && strings.TrimSpace(o.Name) != "" && (o.Source == "provider_name" || o.Source == "t3_title") && err == nil && !at.After(time.Now().UTC()) && a.NativeReceipt != nil && a.NativeReceipt.SessionID == a.SessionID && a.NativeReceipt.ThreadID == o.T3ThreadID {
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
