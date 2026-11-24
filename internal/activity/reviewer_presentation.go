package activity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/orchestration"
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
	Provider           string                          `json:"provider,omitempty"`
	GenerationEvidence string                          `json:"generation_evidence,omitempty"`
	StartedAt          string                          `json:"started_at,omitempty"`
	LinkageVerified    bool                            `json:"linkage_verified,omitempty"`
	StoredRole         string                          `json:"stored_role,omitempty"`
	AssignmentID       string                          `json:"assignment_id,omitempty"`
	ContractSHA256     string                          `json:"contract_sha256,omitempty"`
	AcceptanceSHA256   string                          `json:"acceptance_sha256,omitempty"`
	Source             *rolepolicy.Source              `json:"source,omitempty"`
	PolicySHA256       string                          `json:"policy_sha256,omitempty"`
	ContextSupport     string                          `json:"context_support,omitempty"`
	NameEvidence       *runtimeobs.RootNameObservation `json:"name_evidence,omitempty"`
	ActorName          string                          `json:"actor_name"`
	ActorNameSource    string                          `json:"actor_name_source"`
	AttemptID          string                          `json:"attempt_id"`
	NativeID           string                          `json:"native_id"`
	TaskID             string                          `json:"task_id"`
	OfficialName       string                          `json:"official_name"`
	NameVerified       bool                            `json:"name_verified"`
	NameSource         string                          `json:"name_source"`
	NameObservedAt     string                          `json:"name_observed_at"`
	Purpose            string                          `json:"purpose"`
	PurposeVerified    bool                            `json:"purpose_verified"`
	Necessity          string                          `json:"necessity"`
	ActualState        string                          `json:"actual_state"`
	ChainID            string                          `json:"chain_id"`
	ReviewRound        *int                            `json:"review_round"`
	SessionGeneration  *int                            `json:"session_generation"`
	ContextStatus      string                          `json:"context_status"`
	ContinuityRef      string                          `json:"continuity_ref"`
	ContextRef         string                          `json:"context_ref"`
	SourceDelta        []json.RawMessage               `json:"source_delta"`
	Errors             []string                        `json:"errors"`
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
	path        string
	order       time.Time
	unavailable bool
}

func readPersistedNameEvidence(project string, wanted map[string]bool, byteBudget int64, fileBudget int) (map[string]runtimeobs.RootNameObservation, nameReadStats) {
	out := map[string]runtimeobs.RootNameObservation{}
	stats := nameReadStats{}
	paths, e := filepath.Glob(filepath.Join(runtimeobs.ExecutionRawDir(project), "*.jsonl"))
	if e != nil {
		return out, stats
	}
	legacy := runtimeobs.ExecutionLegacyJournalPath(project)
	// Unlike a glob result, an absent legacy journal was never discovered.
	if _, err := os.Lstat(legacy); err == nil || !os.IsNotExist(err) {
		paths = append(paths, legacy)
	}
	files := []nameJournalFile{}
	for _, path := range paths {
		info, e := os.Stat(path)
		unavailable := e != nil || !info.Mode().IsRegular()
		// An unavailable undated file has no provable position: block fallback.
		order := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
		if !unavailable {
			order = info.ModTime()
		}
		if day, e := time.Parse("2006-01-02.jsonl", filepath.Base(path)); e == nil {
			order = day.Add(24*time.Hour - time.Nanosecond)
		}
		files = append(files, nameJournalFile{path, order, unavailable})
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
		// Preserve parsed newer proof, but unresolved keys cannot skip a gap.
		if entry.unavailable {
			break
		}
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
		v.Provider = a.Provider
		v.StartedAt = a.StartedAt
		if a.NativeReceipt != nil && a.NativeReceipt.StartedAt != "" {
			v.StartedAt = a.NativeReceipt.StartedAt
		}
		v.StoredRole = workflowActorRole(a.AgentPath)
		v.ContextSupport = "not_collected"
		if rolepolicy.WorkerPath(a.AgentPath) {
			v.ContextSupport = "unsupported"
		} else if rolepolicy.ReviewerPath(a.AgentPath) && v.ContextStatus != "" && v.ContextStatus != "unknown" {
			v.ContextSupport = "supported"
		}
		v.ActorNameSource = ""
		if a.RuntimeAgentID != "" && a.Provider != "" && (rolepolicy.WorkerPath(a.AgentPath) || rolepolicy.ReviewerPath(a.AgentPath)) {
			v.ActorName = filepath.Base(a.AgentPath)
			v.ActorNameSource = "bound_runtime_agent_path"
		}
		v.OfficialName = ""
		v.NameVerified = false
		v.NameSource = ""
		v.NameObservedAt = ""
		v.NameEvidence = nil
		if v.SessionGeneration != nil {
			v.GenerationEvidence = "verified_continuity_only"
		}
		if v.ActualState != "omitted" {
			v.ActualState = "unknown"
		}
		o, ok := names[a.Provider+"\x00"+a.SessionID]
		at, err := time.Parse(time.RFC3339Nano, o.CheckedAt)
		if ok && o.Version == 1 && o.Status == "verified" && o.IdentityVerified && o.HostProjectID != "" && o.T3ThreadID != "" && o.HostStatus == "verified" && strings.TrimSpace(o.Name) != "" && (o.Source == "provider_name" || o.Source == "t3_title") && err == nil && !at.After(time.Now().UTC()) && a.NativeReceipt != nil && a.NativeReceipt.SessionID == a.SessionID && a.NativeReceipt.ThreadID == o.T3ThreadID && (a.NativeReceipt.Provider == "" || a.NativeReceipt.Provider == a.Provider) && (a.NativeReceipt.CanonicalProject == "" || a.NativeReceipt.CanonicalProject == project) {
			v.OfficialName = o.Name
			v.NameVerified = true
			v.NameSource = o.Source
			v.NameObservedAt = o.CheckedAt
			copy := o
			v.NameEvidence = &copy
			if generation := verifiedDisplayGeneration(project, a, o); generation != nil {
				v.SessionGeneration = generation
				v.GenerationEvidence = "managed_contract_and_exact_native"
			}
		}
		// Only actual review Activity changes actual_state; running is not review.
		if v.PurposeVerified {
			if v.LinkageVerified {
				v.ActualState = "requested"
			}
			effective, _ := EffectiveEvents(events)
			sort.SliceStable(effective, func(i, j int) bool { return effective[i].OccurredAt.Before(effective[j].OccurredAt) })
			for _, e := range effective {
				if e.TaskID != a.TaskID || e.RuntimeAttemptID != a.AttemptID || e.AgentID != a.RuntimeAgentID || (e.Type != ControllerReview && e.Type != ControllerReReview && !(v.LinkageVerified && e.Type != Implementation && e.TaskRevision != "")) || (v.AssignmentID != "" && e.AssignmentID != v.AssignmentID) {
					continue
				}
				if v.Source != nil && (e.ContractSHA256 != v.ContractSHA256 || e.TaskRevision != ReviewSourceRevision(*v.Source)) {
					v.Errors = append(v.Errors, "review Activity source/contract linkage missing or changed")
					v.ActualState = "source_unverified"
					continue
				}
				if v.LinkageVerified && (e.Provider != a.Provider || e.SessionID != a.SessionID) {
					v.Errors = append(v.Errors, "review Activity native/provider mismatch")
					v.ActualState = "identity_unverified"
					continue
				}
				if v.LinkageVerified && (e.Role == RoleController || string(e.Role) != v.StoredRole) {
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
		if a.TaskID == task && v.PurposeVerified && (latestTaskAttempt == "" || v.StartedAt > latestTaskAttempt) {
			latestTaskAttempt = v.StartedAt
			out.Task.ActualState = v.ActualState
		}
	}
	return out
}

var managedDisplayGeneration = regexp.MustCompile(`^(D[0-9]{4}_[0-9]{2,})_(Worker|Reviewer)_([A-Za-z][A-Za-z0-9_-]{0,39})_G([1-9][0-9]*)$`)

func verifiedDisplayGeneration(project string, a runtimeobs.Attempt, o runtimeobs.RootNameObservation) *int {
	// A host title alone is never a generation. Require exact persisted native
	// metadata plus the canonical managed naming contract for the bound role.
	if !o.IdentityVerified || o.HostProjectID == "" || o.T3ThreadID == "" || o.Status != "verified" || o.HostStatus != "verified" || o.NativeName == "" || o.Name != o.NativeName || a.NativeReceipt == nil || a.NativeReceipt.ThreadID != o.T3ThreadID || a.NativeReceipt.SessionID != a.SessionID || a.NativeReceipt.Provider != a.Provider || (a.NativeReceipt.CanonicalProject != "" && a.NativeReceipt.CanonicalProject != project) {
		return nil
	}
	m := managedDisplayGeneration.FindStringSubmatch(o.NativeName)
	if len(m) != 5 {
		return nil
	}
	role := workflowActorRole(a.AgentPath)
	if !strings.EqualFold(role, m[2]) || filepath.Base(a.AgentPath) != m[3] {
		return nil
	}
	generation, err := strconv.Atoi(m[4])
	if err != nil {
		return nil
	}
	group, err := orchestration.FindGroupByID(project, m[1])
	if err != nil || group.Provider != a.Provider {
		return nil
	}
	expected, err := orchestration.SessionName(group, role, m[3], generation)
	if err != nil || expected != o.NativeName {
		return nil
	}
	return &generation
}
