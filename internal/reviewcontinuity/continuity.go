// Package reviewcontinuity makes read-only evidence decisions. It never launches
// sessions, grants authority, changes accepted results, or writes runtime ledgers.
package reviewcontinuity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/contextlife"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

type Reference struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Finding struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Resolved bool   `json:"resolved"`
}
type Snapshot struct {
	Request           rolepolicy.ReviewRequest `json:"request"`
	ChainID           string                   `json:"chain_id"`
	ReviewRound       *int                     `json:"review_round"`
	SessionGeneration int                      `json:"session_generation"`
	State             string                   `json:"state"`
	Report            Reference                `json:"report"`
	Findings          []Finding                `json:"findings"`
	Errors            []string                 `json:"errors"`
	Validation        []string                 `json:"validation"`
	Remaining         []string                 `json:"remaining"`
}

// Observation is independently collected runtime/Activity evidence, not session
// liveness. Its reference is checked before its state is used.
type Observation struct {
	AttemptID   string `json:"attempt_id"`
	NativeID    string `json:"native_id"`
	State       string `json:"state"`
	NativeState string `json:"native_state"`
	ObservedAt  string `json:"observed_at"`
}
type Host struct {
	Kind              string `json:"kind"`
	ReuseSupport      string `json:"reuse_support"`
	PolicyAllowsReuse bool   `json:"policy_allows_reuse"`
}
type Checkpoint struct {
	Version     int                    `json:"version"`
	Snapshots   []Snapshot             `json:"snapshots"`
	Context     contextlife.Checkpoint `json:"context"`
	Fingerprint string                 `json:"fingerprint"`
}
type Input struct {
	Current               Snapshot              `json:"current"`
	ImplementationHistory []rolepolicy.Identity `json:"implementation_history"`
	Continuity            Reference             `json:"continuity"`
	ContextRef            Reference             `json:"context_ref"`
	ObservationRef        Reference             `json:"observation_ref"`
	Host                  Host                  `json:"host"`
	ConflictingAttempts   []string              `json:"conflicting_attempts"`
}
type SourceChange struct {
	Path           string `json:"path"`
	PreviousSHA256 string `json:"previous_sha256"`
	CurrentSHA256  string `json:"current_sha256"`
}
type Result struct {
	AttemptID         string         `json:"attempt_id"`
	NativeID          string         `json:"native_id"`
	Purpose           string         `json:"purpose"`
	PurposeVerified   bool           `json:"purpose_verified"`
	Necessity         string         `json:"necessity"`
	ActualState       string         `json:"actual_state"`
	ChainID           string         `json:"chain_id"`
	ReviewRound       *int           `json:"review_round"`
	SessionGeneration *int           `json:"session_generation"`
	ContextStatus     string         `json:"context_status"`
	ContinuityRef     string         `json:"continuity_ref"`
	ContextRef        string         `json:"context_ref"`
	Mode              string         `json:"mode"`
	Reason            string         `json:"reason"`
	SourceDelta       []SourceChange `json:"source_delta"`
	Previous          []Snapshot     `json:"previous"`
	Errors            []string       `json:"errors"`
}

func hash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func validHash(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && s == strings.ToLower(s)
}
func confined(root, path string) (string, error) {
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", errors.New("unsafe evidence/source path")
	}
	r, e := filepath.EvalSymlinks(root)
	if e != nil {
		return "", e
	}
	p, e := filepath.EvalSymlinks(filepath.Join(r, path))
	if e != nil {
		return "", e
	}
	rel, e := filepath.Rel(r, p)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("evidence/source escapes workspace")
	}
	return p, nil
}
func ReadReference(root string, ref Reference) ([]byte, error) {
	if !validHash(ref.SHA256) {
		return nil, errors.New("evidence SHA256 required")
	}
	p, e := confined(root, ref.Path)
	if e != nil {
		return nil, e
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return nil, e
	}
	if hash(b) != ref.SHA256 {
		return nil, errors.New("evidence hash mismatch")
	}
	return b, nil
}
func validIdentity(i rolepolicy.Identity, task string) bool {
	return i.TaskID == task && i.AttemptID != "" && i.NativeID != "" && i.Provider != "" && i.Bound && i.Observed
}
func validateSnapshot(root string, s Snapshot, current bool) error {
	r := s.Request
	if r.TaskID == "" || r.AssignmentID == "" || !validHash(r.ContractSHA256) || !validIdentity(r.Reviewer, r.TaskID) || !rolepolicy.ReviewerPath(r.Reviewer.AgentPath) || !validIdentity(r.Implementation, r.TaskID) {
		return errors.New("task/assignment/contract/identity unverified")
	}
	if r.Reviewer.NativeID == r.Implementation.NativeID || (r.Reviewer.RuntimeAgentID != "" && r.Reviewer.RuntimeAgentID == r.Implementation.RuntimeAgentID) {
		return errors.New("same native implementation/review")
	}
	if s.SessionGeneration < 1 || s.ReviewRound != nil && *s.ReviewRound < 1 {
		return errors.New("invalid separate generation/round")
	}
	if current {
		actual, e := rolepolicy.Snapshot(root, r.Requirement.Scope)
		if e != nil {
			return fmt.Errorf("complete scoped source: %w", e)
		}
		if !reflect.DeepEqual(actual, r.Source) {
			return errors.New("complete scoped source mismatch")
		}
	}
	if len(r.Source.Files) == 0 {
		return errors.New("complete source map required")
	}
	for path, sha := range r.Source.Files {
		if !validHash(sha) {
			return errors.New("source SHA256 invalid")
		}
		if filepath.IsAbs(path) || filepath.Clean(path) != path || path == ".." || strings.HasPrefix(path, "../") {
			return errors.New("source path invalid")
		}
		if current {
			b, e := ReadReference(root, Reference{path, sha})
			if e != nil {
				return fmt.Errorf("source %s: %w", path, e)
			}
			_ = b
		}
	}
	if s.Report.Path != "" || s.Report.SHA256 != "" {
		if r.ReportFile != s.Report.Path {
			return errors.New("report reference mismatch")
		}
		if _, e := ReadReference(root, s.Report); e != nil {
			return e
		}
	}
	if s.State == "accepted" || s.State == "needs_changes" || s.State == "failed" {
		if s.Report.Path == "" {
			return errors.New("result report required")
		}
	}
	return nil
}
func checkpointHash(c Checkpoint) string { c.Fingerprint = ""; b, _ := json.Marshal(c); return hash(b) }
func BuildCheckpoint(root string, snapshots []Snapshot, context contextlife.Checkpoint) (Checkpoint, error) {
	c := Checkpoint{Version: 1, Snapshots: snapshots, Context: context}
	if len(snapshots) == 0 {
		return c, errors.New("checkpoint needs prior evidence")
	}
	for _, s := range snapshots {
		if e := validateSnapshot(root, s, false); e != nil {
			return c, e
		}
	}
	last := snapshots[len(snapshots)-1]
	role := "reviewer:" + strings.TrimPrefix(last.Request.Reviewer.AgentPath, "/root/controller/reviewer/")
	if context.Role != role || context.OwnerAttempt != last.Request.Reviewer.AttemptID || context.Fingerprint == "" {
		return c, errors.New("CLM checkpoint owner/role/fingerprint mismatch")
	}
	c.Fingerprint = checkpointHash(c)
	return c, nil
}
func ReadCheckpoint(root string, ref Reference) (Checkpoint, error) {
	var c Checkpoint
	b, e := ReadReference(root, ref)
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	if c.Version != 1 || c.Fingerprint != checkpointHash(c) {
		return c, errors.New("continuity checkpoint fingerprint mismatch")
	}
	checked, e := BuildCheckpoint(root, c.Snapshots, c.Context)
	if e != nil || checked.Fingerprint != c.Fingerprint {
		return c, errors.New("continuity checkpoint evidence invalid")
	}
	return c, nil
}
func Delta(previous, current rolepolicy.Source) []SourceChange {
	keys := map[string]bool{}
	for p := range previous.Files {
		keys[p] = true
	}
	for p := range current.Files {
		keys[p] = true
	}
	out := []SourceChange{}
	for p := range keys {
		if previous.Files[p] != current.Files[p] {
			out = append(out, SourceChange{p, previous.Files[p], current.Files[p]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Assess validates referenced bytes at the current boundary. Returned history is
// evidence, never a transferred acceptance or a dispatch/rotation permission.
func Assess(root string, in Input) (Result, error) {
	r := in.Current.Request
	out := Result{AttemptID: r.Reviewer.AttemptID, NativeID: r.Reviewer.NativeID, Purpose: "unknown", Necessity: "unknown", ActualState: "unknown", ContextStatus: "unknown", Mode: "blocked", SourceDelta: []SourceChange{}, Previous: []Snapshot{}, Errors: []string{}}
	fail := func(e error) (Result, error) {
		out.Errors = append(out.Errors, e.Error())
		out.Reason = e.Error()
		return out, e
	}
	if e := validateSnapshot(root, in.Current, true); e != nil {
		return fail(e)
	}
	d, e := r.Requirement.Decide()
	if e != nil {
		return fail(e)
	}
	out.Necessity = string(d.Level)
	if d.Status == "omitted" {
		out.ActualState = "omitted"
		out.Reason = "review omitted by policy"
		return out, nil
	}
	implementationFound := false
	for _, h := range in.ImplementationHistory {
		if validIdentity(h, r.TaskID) && h.NativeID == r.Implementation.NativeID && h.AgentPath == r.Implementation.AgentPath {
			implementationFound = true
		}
	}
	if !implementationFound {
		return fail(errors.New("verified implementation history required"))
	}
	if e = rolepolicy.ValidateImplementationHistory(r.TaskID, r.Reviewer, in.ImplementationHistory); e != nil {
		return fail(e)
	}
	out.Purpose = "review"
	out.PurposeVerified = true

	if len(in.ConflictingAttempts) > 0 {
		return fail(errors.New("concurrent assignment conflict"))
	}
	var observation Observation
	if in.ObservationRef.Path != "" {
		b, err := ReadReference(root, in.ObservationRef)
		if err != nil {
			return fail(err)
		}
		if json.Unmarshal(b, &observation) != nil || observation.AttemptID != r.Reviewer.AttemptID || observation.NativeID != r.Reviewer.NativeID {
			return fail(errors.New("observation identity mismatch"))
		}
		at, e := time.Parse(time.RFC3339Nano, observation.ObservedAt)
		if e != nil || at.After(time.Now().UTC()) || time.Since(at) > 5*time.Minute {
			return fail(errors.New("review observation stale or unknown"))
		}
		switch observation.State {
		case "requested", "started", "reviewing", "needs_changes", "result_recorded", "failed":
			out.ActualState = observation.State
		default:
			return fail(errors.New("unsupported review observation state"))
		}
	}
	var previous *Snapshot
	if r.ContinuityRef != "" || in.Continuity.Path != "" {
		if r.ContinuityRef != in.Continuity.Path {
			return fail(errors.New("continuity ref mismatch"))
		}
		c, err := ReadCheckpoint(root, in.Continuity)
		if err != nil {
			return fail(err)
		}
		out.Previous = c.Snapshots
		previous = &out.Previous[len(out.Previous)-1]
		if previous.Request.TaskID != r.TaskID || previous.ChainID == "" || previous.ChainID != in.Current.ChainID {
			return fail(errors.New("continuity task/chain mismatch"))
		}
		out.ChainID = in.Current.ChainID
		if previous.ReviewRound != nil && in.Current.ReviewRound != nil {
			if *in.Current.ReviewRound < *previous.ReviewRound || *in.Current.ReviewRound > *previous.ReviewRound+1 {
				return fail(errors.New("unverified review round gap"))
			}
			out.ReviewRound = in.Current.ReviewRound
		}
		out.ContinuityRef = r.ContinuityRef
		out.SourceDelta = Delta(previous.Request.Source, r.Source)
	}
	var context contextlife.StatusReport
	if r.ContextRef != "" || in.ContextRef.Path != "" {
		if r.ContextRef != in.ContextRef.Path {
			return fail(errors.New("context ref mismatch"))
		}
		b, err := ReadReference(root, in.ContextRef)
		if err != nil {
			return fail(err)
		}
		role := "reviewer:" + strings.TrimPrefix(r.Reviewer.AgentPath, "/root/controller/reviewer/")
		if json.Unmarshal(b, &context) != nil || context.State.Role != role || context.State.OwnerAttempt != r.Reviewer.AttemptID {
			return fail(errors.New("CLM context owner mismatch"))
		}
		out.ContextRef = r.ContextRef
		context.Decision = contextlife.Decide(role, context.State.Observation, 0, "")
		out.ContextStatus = context.Decision.ContextStatus
		if context.State.Generation > 0 && context.State.Generation == in.Current.SessionGeneration {
			g := context.State.Generation
			out.SessionGeneration = &g
		}
	}
	if context.Decision.Action == "protect" {
		return fail(errors.New("CLM protect: checkpoint and validated successor required before dispatch"))
	}
	if in.Host.Kind == "t3" {
		out.Mode = "new_delegate_task"
		out.Reason = "T3 new review/recheck requires a new delegate_task with prior evidence"
		return out, nil
	}
	out.Mode = "new_session"
	out.Reason = "same-session reuse unverified or unsupported"
	if (in.Host.Kind != "codex" && in.Host.Kind != "claude") || in.Host.ReuseSupport != "supported" || !in.Host.PolicyAllowsReuse {
		return out, nil
	}
	if previous == nil || !reflect.DeepEqual(previous.Request.Reviewer, r.Reviewer) || previous.Request.AssignmentID != r.AssignmentID || previous.Request.ContractSHA256 != r.ContractSHA256 || len(out.SourceDelta) > 0 {
		return out, nil
	}
	o := context.State.Observation
	if observation.NativeState != "running" || out.ActualState != "reviewing" || out.ContextStatus != "measured" || !context.Eligible || len(context.Blockers) > 0 || (context.State.Rotation != nil && context.State.Rotation.Stage != "committed") || context.Decision.Action != "continue" || o == nil || o.AttemptID != r.Reviewer.AttemptID || o.Provider != r.Reviewer.Provider || o.Trust != "provider_verified" || o.UsagePercent == nil || *o.UsagePercent < 0 || *o.UsagePercent >= 65 || o.WindowTokens <= 0 {
		return out, nil
	}
	at, e := time.Parse(time.RFC3339Nano, o.ObservedAt)
	if e != nil || at.After(time.Now().UTC()) || time.Since(at) > 5*time.Minute {
		return out, nil
	}
	out.Mode = "reuse_session"
	out.Reason = "supported host/policy and exact current identity, assignment, source, independence and CLM context verified"
	return out, nil
}
