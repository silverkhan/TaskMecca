package runtimeobs

// Operation boundaries are current observations, independent of task allocation.
// They never supply execution authority or rewrite the Activity journal.
import (
	"bufio"
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

	"github.com/silverkhan/TaskMecca/internal/projectguard"
)

type BoundaryWait struct {
	Scope           string `json:"scope"` // actor or task_next_stage
	Reason          string `json:"actual_reason"`
	ExpectedEvent   string `json:"expected_event"`
	NextOwner       string `json:"next_owner"`
	LastEvidence    string `json:"last_evidence"`
	ResumeCondition string `json:"resume_condition"`
}

func (w *BoundaryWait) Evidenced() bool {
	if w == nil || (w.Scope != "actor" && w.Scope != "task_next_stage") {
		return false
	}
	for _, value := range []string{w.Reason, w.ExpectedEvent, w.NextOwner, w.LastEvidence, w.ResumeCondition} {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "", "unknown", "미확인", "-":
			return false
		}
	}
	return true
}

type OperationBoundary struct {
	Explanation         string        `json:"explanation,omitempty"`
	ExplanationLanguage string        `json:"explanation_language,omitempty"`
	ExplanationOrigin   string        `json:"explanation_origin,omitempty"`
	RequestID           string        `json:"request_id,omitempty"`
	ID                  string        `json:"id"`
	Key                 string        `json:"key"`
	OperationID         string        `json:"operation_id"`
	Provider            string        `json:"provider"`
	NativeID            string        `json:"native_id"`
	AttemptID           string        `json:"attempt_id"`
	ThreadID            string        `json:"thread_id,omitempty"`
	Role                string        `json:"role"`
	Kind                string        `json:"kind"`
	Phase               string        `json:"phase,omitempty"`
	RelatedID           string        `json:"related_id,omitempty"` // wait ID, handoff ID or failed boundary key
	ObservedAt          time.Time     `json:"observed_at"`
	SourceAt            *time.Time    `json:"source_at,omitempty"` // original evidence time; never the measured start
	TaskIDs             []string      `json:"task_ids,omitempty"`
	Source              string        `json:"source"`
	Evidence            string        `json:"evidence"`
	Wait                *BoundaryWait `json:"wait,omitempty"`
	Error               string        `json:"error,omitempty"`
}

func boundaryID(b OperationBoundary) string {
	raw, _ := json.Marshal([]string{b.Provider, b.NativeID, b.AttemptID, b.OperationID, b.Key})
	h := sha256.Sum256(raw)
	return "boundary-" + hex.EncodeToString(h[:])
}
func OperationBoundaryPath(project string) string {
	return filepath.Join(project, "_task_mecca", ".runtime", "execution-boundaries.jsonl")
}

func validateBoundary(b OperationBoundary) error {
	if b.ExplanationLanguage != "" && b.ExplanationLanguage != "ko" && b.ExplanationLanguage != "en" {
		return errors.New("unsupported boundary explanation language")
	}
	if b.ExplanationOrigin != "" && b.ExplanationOrigin != "written" && b.ExplanationOrigin != "type_default" {
		return errors.New("unsupported boundary explanation origin")
	}
	if b.Provider == "" || b.NativeID == "" || b.AttemptID == "" || b.OperationID == "" || b.Key == "" || b.Evidence == "" || b.Source == "" || b.ObservedAt.IsZero() {
		return errors.New("boundary requires exact native/attempt, stable operation/key and evidence")
	}
	if b.Provider != "codex" && b.Provider != "claude" {
		return errors.New("unsupported native boundary provider")
	}
	for _, value := range []string{b.NativeID, b.AttemptID, b.OperationID, b.Key, b.Evidence, b.Source} {
		if strings.TrimSpace(value) == "" {
			return errors.New("blank boundary identity/key/evidence")
		}
	}
	switch b.Role {
	case "controller", "registrar", "worker", "reviewer", "unknown":
	default:
		return errors.New("unsupported boundary role")
	}
	switch b.Kind {
	case "start", "phase", "wait", "resume", "end", "task_link", "observation", "handoff_request", "handoff_delivery", "handoff_claim", "collection_failure":
	default:
		return errors.New("unsupported boundary kind")
	}
	if (b.Kind == "resume" || b.Kind == "handoff_request" || b.Kind == "handoff_delivery" || b.Kind == "handoff_claim") && b.RelatedID == "" {
		return errors.New("related wait/handoff boundary required")
	}
	if b.Kind == "task_link" && len(b.TaskIDs) == 0 {
		return errors.New("canonical tasks required for association")
	}
	if b.Kind == "collection_failure" && b.Error == "" {
		return errors.New("original collection error required")
	}
	if b.Wait != nil && b.Kind != "wait" {
		return errors.New("wait metadata only belongs to a wait boundary")
	}
	return nil
}

// Reads never create storage. A corrupt line is reported along with valid rows,
// so a collection failure cannot silently become a complete/empty timeline.
func ReadOperationBoundaries(project string) ([]OperationBoundary, error) {
	path := OperationBoundaryPath(project)
	if err := boundaryPathSafe(project); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return []OperationBoundary{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := []OperationBoundary{}
	seen := map[string]OperationBoundary{}
	var failures []error
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 1024*1024)
	for line := 1; s.Scan(); line++ {
		var b OperationBoundary
		if err := json.Unmarshal(s.Bytes(), &b); err != nil {
			failures = append(failures, fmt.Errorf("boundary line %d: %w", line, err))
			continue
		}
		if err := validateBoundary(b); err != nil || b.ID != boundaryID(b) {
			failures = append(failures, fmt.Errorf("invalid boundary line %d: %v", line, err))
			continue
		}
		if prior, ok := seen[b.ID]; ok {
			if !reflect.DeepEqual(prior, b) {
				failures = append(failures, fmt.Errorf("boundary ID conflict at line %d", line))
			}
			continue
		}
		seen[b.ID] = b
		out = append(out, b)
	}
	if err := s.Err(); err != nil {
		failures = append(failures, err)
	}
	return out, errors.Join(failures...)
}

func boundaryPathSafe(project string) error {
	for _, p := range []string{filepath.Join(project, "_task_mecca"), filepath.Join(project, "_task_mecca", ".runtime"), OperationBoundaryPath(project)} {
		st, err := os.Lstat(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("boundary symlink refused: %s", p)
		}
	}
	return nil
}

// AppendOperationBoundary is idempotent by native+attempt+operation+key. Retries
// preserve the first observation time; conflicting reuse fails without mutation.
// now is supplied by the supported collector, never accepted as a CLI timestamp.
func AppendOperationBoundary(project string, b OperationBoundary, now time.Time) (OperationBoundary, bool, error) {
	var err error
	b, err = PrepareOperationBoundary(b, now)
	if err != nil {
		return b, false, err
	}
	if _, err = RecordCollectionReceipt(project, BoundaryCollectionReceipt(b, "pending", "", now)); err != nil {
		return b, false, fmt.Errorf("durable boundary intent unavailable: %w", err)
	}
	var recorded bool
	func() {
		var release func()
		release, err = projectguard.AcquireStateLock(project, "execution-boundaries")
		if err != nil {
			return
		}
		defer release()
		b, recorded, err = appendPreparedBoundary(project, b)
	}()
	if err != nil {
		_, failureErr := RecordCollectionReceipt(project, BoundaryCollectionReceipt(b, "failed", err.Error(), time.Now().UTC()))
		return b, false, errors.Join(err, failureErr)
	}
	if _, ackErr := RecordCollectionReceipt(project, BoundaryCollectionReceipt(b, "ack", "", time.Now().UTC())); ackErr != nil {
		return b, recorded, fmt.Errorf("boundary appended; ACK uncertain, reconcile same key before retry: %w", ackErr)
	}
	return b, recorded, nil
}

// PrepareOperationBoundary performs the same validation used by the writer,
// without creating a journal/lock or backdating any measured boundary.
func PrepareOperationBoundary(b OperationBoundary, now time.Time) (OperationBoundary, error) {
	if now.IsZero() {
		return b, errors.New("current observation time required")
	}
	b.ObservedAt = now.UTC()
	b.ID = boundaryID(b)
	b.TaskIDs = append([]string(nil), b.TaskIDs...)
	sort.Strings(b.TaskIDs)
	b.TaskIDs = compactTasks(b.TaskIDs)
	if err := validateBoundary(b); err != nil {
		return b, err
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return b, err
	}
	if len(raw) > 1024*1024-2 {
		return b, errors.New("boundary exceeds supported journal row size")
	}
	return b, nil
}

func appendPreparedBoundary(project string, b OperationBoundary) (OperationBoundary, bool, error) {
	rows, err := ReadOperationBoundaries(project)
	if err != nil {
		return b, false, err
	}
	for _, old := range rows {
		if old.ID == b.ID {
			same := b
			same.ObservedAt = old.ObservedAt
			if reflect.DeepEqual(same, old) {
				return old, false, nil
			}
			return b, false, errors.New("boundary key reused with conflicting evidence")
		}
	}
	if err = boundaryPathSafe(project); err != nil {
		return b, false, err
	}
	f, err := os.OpenFile(OperationBoundaryPath(project), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return b, false, err
	}
	raw, err := json.Marshal(b)
	if err == nil {
		raw = append(raw, '\n')
		var n int
		n, err = f.Write(raw)
		if err == nil && n != len(raw) {
			err = errors.New("short boundary append")
		}
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	return b, err == nil, err
}

func compactTasks(xs []string) []string {
	out := xs[:0]
	for _, x := range xs {
		if x != "" && (len(out) == 0 || out[len(out)-1] != x) {
			out = append(out, x)
		}
	}
	return out
}

// Automatic native events are observations, never measured work/phase/wait.
func collectNativeBoundary(project string, a Attempt, r T3Receipt, now time.Time) error {
	role := boundaryRoleFromPath(a.AgentPath)
	key := "native_start_observed"
	if r.State != StateRunning {
		key = "native_end_observed"
	}
	sourceAt, _ := time.Parse(time.RFC3339Nano, r.StartedAt)
	if r.CompletedAt != "" {
		sourceAt, _ = time.Parse(time.RFC3339Nano, r.CompletedAt)
	}
	tasks := []string{}
	if a.TaskID != "" {
		tasks = append(tasks, a.TaskID)
	}
	// Task binding can arrive later: association is a separate immutable event.
	_, _, err := AppendOperationBoundary(project, OperationBoundary{Key: key, OperationID: "native:" + r.NativeAttemptID, Provider: r.Provider, NativeID: r.SessionID, AttemptID: a.AttemptID, ThreadID: r.ThreadID, Role: "unknown", Kind: "observation", SourceAt: &sourceAt, Source: "t3_structured", Evidence: r.NativeAttemptID + ":" + string(r.State)}, now)
	if err != nil {
		return err
	}
	if len(tasks) > 0 {
		_, _, err = AppendOperationBoundary(project, OperationBoundary{Key: "task_binding:" + a.TaskID, OperationID: "native:" + r.NativeAttemptID, Provider: r.Provider, NativeID: r.SessionID, AttemptID: a.AttemptID, ThreadID: r.ThreadID, Role: role, Kind: "task_link", TaskIDs: tasks, Source: "runtime_binding", Evidence: a.AttemptID + ":" + a.TaskID}, now)
	}
	return err
}

func collectHookBoundary(project string, e ExecutionEvent, now time.Time) error {
	// Hooks without an exact native session cannot establish a new identity.
	// Existing hook journal still preserves those events and their diagnostics.
	if e.SessionID == "" || e.AttemptID == "" {
		return nil
	}
	sourceAt, err := time.Parse(time.RFC3339Nano, e.ObservedAt)
	if err != nil {
		return err
	}
	_, _, err = AppendOperationBoundary(project, OperationBoundary{Key: "hook:" + e.EventID, OperationID: "native:" + e.AttemptID, Provider: e.Provider, NativeID: e.SessionID, AttemptID: e.AttemptID, Role: "unknown", Kind: "observation", SourceAt: &sourceAt, Source: "provider_hook", Evidence: e.EventID + ":" + e.HookEventName}, now)
	if err != nil {
		return err
	}
	ledger, err := BuildLedger(project, 0, now)
	if err != nil {
		return err
	}
	matches := []Attempt{}
	for _, a := range ledger.Attempts {
		if a.AttemptID == e.AttemptID && a.SessionID == e.SessionID && (e.RuntimeAgentID == "" || a.RuntimeAgentID == e.RuntimeAgentID) {
			matches = append(matches, a)
		}
	}
	if len(matches) == 1 && matches[0].BindingState == BindingBound && matches[0].TaskID != "" {
		a := matches[0]
		_, _, err = AppendOperationBoundary(project, OperationBoundary{Key: "task_binding:" + a.TaskID, OperationID: "native:" + e.AttemptID, Provider: e.Provider, NativeID: e.SessionID, AttemptID: e.AttemptID, Role: boundaryRoleFromPath(a.AgentPath), Kind: "task_link", TaskIDs: []string{a.TaskID}, Source: "runtime_binding", Evidence: a.AttemptID + ":" + a.TaskID}, now)
	}
	return err
}

func boundaryRoleFromPath(path string) string {
	switch path {
	case "/root/controller":
		return "controller"
	case "/root/registrar":
		return "registrar"
	}
	if strings.HasPrefix(path, "/root/controller/reviewer/") {
		return "reviewer"
	}
	if strings.HasPrefix(path, "/root/controller/") {
		name := strings.TrimPrefix(path, "/root/controller/")
		if name != "" && !strings.Contains(name, "/") && !strings.HasPrefix(name, "sub-") {
			return "worker"
		}
	}
	return "unknown"
}
