package handoff

// Context is evidence carried by the existing handoff, not an approval or a
// provider/session manager. Historical references never become completion targets.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/contractscope"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type ExecutionContext struct {
	SchemaVersion       int            `json:"schema_version"`
	ControllerAttemptID string         `json:"controller_attempt_id"`
	Targets             []TaskEvidence `json:"targets"`
	References          []string       `json:"references,omitempty"`
}

type TaskEvidence struct {
	TaskID            string            `json:"task_id"`
	ContractSHA256    string            `json:"contract_sha256"`
	AcceptanceSHA256  string            `json:"acceptance_sha256"`
	OriginalHandoffID string            `json:"original_handoff_id"`
	Phase             string            `json:"phase"`
	SourceAttemptID   string            `json:"source_attempt_id,omitempty"`
	AssignmentID      string            `json:"assignment_id,omitempty"`
	SourceWorkspace   string            `json:"source_workspace,omitempty"`
	Source            rolepolicy.Source `json:"source"`
	ReportFile        string            `json:"report_file,omitempty"`
	ReportSHA256      string            `json:"report_sha256,omitempty"`
	Changes           []string          `json:"changes"`
	Tests             []CheckEvidence   `json:"tests"`
	OriginalErrors    []string          `json:"original_errors,omitempty"`
}

type CheckEvidence struct {
	Command  string `json:"command"`
	Result   string `json:"result"`
	Evidence string `json:"evidence"`
}

type ContextCheck struct {
	RegistrationLineage         *RegistrationLineage    `json:"registration_lineage,omitempty"`
	ImplementationOwner         *rolepolicy.Owner       `json:"implementation_owner,omitempty"`
	OtherActors                 []AssignmentObservation `json:"other_actors,omitempty"`
	KnownReports                []TaskEvidence          `json:"known_reports,omitempty"`
	TaskID                      string                  `json:"task_id"`
	ReportedTargets             []TaskEvidence          `json:"reported_targets,omitempty"`
	WorkerRuntime               *WorkerObservation      `json:"worker_runtime,omitempty"`
	Status                      string                  `json:"status"`
	CurrentControllerAttemptID  string                  `json:"current_controller_attempt_id"`
	ReportedControllerAttemptID string                  `json:"reported_controller_attempt_id,omitempty"`
	Target                      *TaskEvidence           `json:"target,omitempty"`
	ActionOwner                 string                  `json:"action_owner"`
	RemainingStage              string                  `json:"remaining_stage"`
	ResumeCondition             string                  `json:"resume_condition"`
	OriginalError               string                  `json:"original_error,omitempty"`
}

type WorkerObservation struct {
	AttemptID string                    `json:"attempt_id"`
	NativeID  string                    `json:"native_id"`
	State     runtimeobs.CanonicalState `json:"state"`
	TurnEnded bool                      `json:"turn_ended"`
}

// An applied registration is an immutable intake receipt, not a completion or
// ownership-transfer target. Link its verified native lineage without rewriting it.
type RegistrationLineage struct {
	Mode                       string `json:"mode"`
	OriginalHandoffID          string `json:"original_handoff_id"`
	OriginalTargetAttemptID    string `json:"original_target_attempt_id"`
	OriginalClaimedAttemptID   string `json:"original_claimed_attempt_id,omitempty"`
	OriginalAppliedAt          string `json:"original_applied_at,omitempty"`
	CurrentControllerAttemptID string `json:"current_controller_attempt_id"`
	HistoricalTargetMismatch   bool   `json:"historical_target_mismatch"`
	Provider                   string `json:"provider"`
	NativeID                   string `json:"native_id"`
	RuntimeAgentID             string `json:"runtime_agent_id"`
}

func registrationLineage(project string, original Handoff, current Target, now time.Time) (*RegistrationLineage, error) {
	proof := &RegistrationLineage{Mode: "current_registration_target", OriginalHandoffID: original.HandoffID, OriginalTargetAttemptID: original.Target.AttemptID, OriginalClaimedAttemptID: original.ClaimedAttemptID, OriginalAppliedAt: original.AppliedAt, CurrentControllerAttemptID: current.AttemptID, Provider: current.Provider, NativeID: current.SessionID, RuntimeAgentID: current.RuntimeAgentID}
	if original.Target.AttemptID == current.AttemptID && (original.ClaimedAttemptID == "" || original.ClaimedAttemptID == current.AttemptID) && sameVerifiedRuntimeIdentity(original.Target, current) {
		return proof, nil
	}
	deny := func() (*RegistrationLineage, error) {
		return nil, errors.New("original_handoff_current_attempt_mismatch")
	}
	if original.EventType != EventRegistrationReady || !original.Applied || original.ClaimedBy != "/root/controller" || original.ClaimedAttemptID == "" || original.ClaimedAttemptID != original.Target.AttemptID || original.ClaimedAttemptID == current.AttemptID {
		return deny()
	}
	ledger, e := runtimeobs.BuildLedger(project, 20, now)
	if e != nil {
		return nil, e
	}
	for _, old := range ledger.Attempts {
		if old.AttemptID != original.ClaimedAttemptID {
			continue
		}
		id := runtimeobs.PolicyIdentity(old)
		if !old.Terminal || !id.Bound || !id.Observed || id.NativeID == "" || old.AgentPath != "/root/controller" {
			return deny()
		}
		actual := Target{AgentPath: old.AgentPath, Provider: old.Provider, SessionID: old.SessionID, RuntimeAgentID: old.RuntimeAgentID}
		if !sameVerifiedRuntimeIdentity(actual, original.Target) || !sameVerifiedRuntimeIdentity(actual, current) {
			return deny()
		}
		proof.Mode = "applied_registration_same_native_history"
		proof.HistoricalTargetMismatch = true
		return proof, nil
	}
	return deny()
}

func contextImplementationOwner(project, task, raw string, inspection map[string]any) (rolepolicy.Owner, error) {
	p, _, e := rolepolicy.Load(project, task, raw)
	if e != nil {
		return rolepolicy.Owner{}, e
	}
	if p != nil {
		return p.Implementation, nil
	}
	// Legacy operational Agent is usable only as an explicit owner/path. Missing
	// or multi-owner metadata must be collected, never replaced by latest-task wins.
	fields, _ := inspection["fields"].(map[string]string)
	path := strings.TrimSpace(fields["Agent"])
	if !rolepolicy.WorkerPath(path) {
		return rolepolicy.Owner{}, errors.New("implementation_owner_unresolved: verify canonical implementation owner or structured role policy; do not infer from latest task assignment")
	}
	return rolepolicy.Owner{Role: "worker", AgentPath: path}, nil
}

type AssignmentObservation struct {
	AssignmentID string             `json:"assignment_id"`
	AgentPath    string             `json:"agent_path"`
	Runtime      *WorkerObservation `json:"runtime,omitempty"`
	Phase        string             `json:"phase,omitempty"`
}

func contextOtherActors(task string, owner rolepolicy.Owner, assignments []runtimeobs.Assignment, attempts []runtimeobs.Attempt, events []activity.Event) []AssignmentObservation {
	var out []AssignmentObservation
	for _, assignment := range assignments {
		if assignment.TaskID != task || assignment.AgentPath == owner.AgentPath {
			continue
		}
		o := AssignmentObservation{AssignmentID: assignment.AssignmentID, AgentPath: assignment.AgentPath}
		latest := ""
		for _, a := range attempts {
			at := a.StartedAt
			if a.BindingAt > at {
				at = a.BindingAt
			}
			if a.TaskID != task || a.AgentPath != assignment.AgentPath || a.BindingEvidence["assignment_id"] != assignment.AssignmentID || !runtimeobs.PolicyIdentity(a).Observed || a.BindingState != runtimeobs.BindingBound || at <= latest {
				continue
			}
			latest = at
			o.Runtime = &WorkerObservation{AttemptID: a.AttemptID, NativeID: a.SessionID, State: a.CurrentState, TurnEnded: a.Terminal}
		}
		if o.Runtime != nil {
			for _, event := range events {
				if event.TaskID == task && event.AssignmentID == assignment.AssignmentID && event.RuntimeAttemptID == o.Runtime.AttemptID && event.PhaseEvent != "" {
					o.Phase = event.PhaseEvent
				}
			}
		}
		out = append(out, o)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].AssignmentID < out[j].AssignmentID })
	return out
}

func contextFailure(task, attempt, stage string, err error) (ContextCheck, error) {
	return ContextCheck{TaskID: task, Status: "collection_gap", CurrentControllerAttemptID: attempt,
		ActionOwner: "/root/controller", RemainingStage: stage,
		ResumeCondition: "Preserve the original report/error; verify exact task " + task + ", current native attempt, contract, assignment and handoff; collect missing evidence before completion. Native resume requires fresh Full Access and supported transport; Root ACK is not a gate.", OriginalError: err.Error()}, err
}

func AcceptanceFingerprint(raw string) (string, error) {
	section := contractscope.Section(raw)
	lines := strings.Split(section, "\n")
	var criteria []string
	in := false
	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			if in {
				break
			}
			heading := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "### ")))
			in = heading == "수용 기준" || heading == "acceptance criteria"
			continue
		}
		if in {
			criteria = append(criteria, line)
		}
	}
	text := strings.TrimSpace(strings.Join(criteria, "\n"))
	if text == "" {
		return "", errors.New("acceptance_criteria_missing")
	}
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:]), nil
}

func readExecutionContext(path string) (ExecutionContext, error) {
	var c ExecutionContext
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	if c.SchemaVersion != 1 || c.ControllerAttemptID == "" || len(c.Targets) == 0 {
		return c, errors.New("task_context_schema_or_targets_missing")
	}
	seen := map[string]bool{}
	for _, t := range c.Targets {
		if t.TaskID == "" || seen[t.TaskID] {
			return c, errors.New("duplicate_or_empty_context_target")
		}
		seen[t.TaskID] = true
	}
	return c, nil
}

func currentController(project, attempt string, now time.Time) (Target, error) {
	if attempt == "" {
		return Target{}, errors.New("exact_current_controller_attempt_required")
	}
	t, action, _, _, e := resolveTarget(project, "/root/controller", "", attempt, now)
	if e != nil {
		return t, e
	}
	if t.AttemptID != attempt || (action != ActionMessageRunning && action != ActionResumeCompleted) {
		return t, errors.New("current_controller_identity_unverified")
	}
	if e = verifyCurrentReceiptTarget(project, t, now); e != nil {
		return t, e
	}
	l, e := runtimeobs.BuildLedger(project, 20, now)
	if e != nil {
		return t, e
	}
	var chosen *runtimeobs.Attempt
	for i := range l.Attempts {
		if l.Attempts[i].AttemptID == attempt {
			chosen = &l.Attempts[i]
			break
		}
	}
	if chosen == nil {
		return t, errors.New("current_controller_attempt_missing")
	}
	id := runtimeobs.PolicyIdentity(*chosen)
	if !id.Bound || !id.Observed || id.NativeID == "" {
		return t, errors.New("current_controller_observation_unverified")
	}
	for _, other := range l.Attempts {
		if other.AttemptID == attempt || other.AgentPath != "/root/controller" || other.BindingState != runtimeobs.BindingBound {
			continue
		}
		otherID := runtimeobs.PolicyIdentity(other)
		if !otherID.Observed {
			continue
		}
		if other.SessionID == chosen.SessionID && other.StartedAt > chosen.StartedAt {
			return t, errors.New("stale_controller_attempt")
		}
		if !other.Terminal && other.SessionID != chosen.SessionID {
			return t, errors.New("concurrent_controller_identity_conflict")
		}
	}
	return t, nil
}

// CheckContext validates only the named target. An unrelated historical target
// remains readable even when its old source checkout/report is no longer present.
// To complete multiple tasks, call the same gate separately for EACH target.
func CheckContext(project, task, attempt string, c ExecutionContext, completion bool, now time.Time) (out ContextCheck, err error) {
	fail := func(e error) (ContextCheck, error) {
		out, err := contextFailure(task, attempt, "context_verification", e)
		out.ReportedControllerAttemptID = c.ControllerAttemptID
		out.ReportedTargets = c.Targets
		return out, err
	}
	if c.SchemaVersion != 1 || c.ControllerAttemptID != attempt {
		return fail(errors.New("context_controller_attempt_mismatch"))
	}
	controller, e := currentController(project, attempt, now)
	if e != nil {
		return fail(e)
	}
	var t *TaskEvidence
	seen := map[string]bool{}
	for i := range c.Targets {
		id := c.Targets[i].TaskID
		if id == "" || seen[id] {
			return fail(errors.New("duplicate_or_empty_context_target"))
		}
		seen[id] = true
		if id == task {
			t = &c.Targets[i]
		}
	}
	if t == nil {
		return fail(errors.New("completion_target_mismatch"))
	}
	i, e := backlog.Inspect(project, "", task)
	if e != nil {
		return fail(e)
	}
	if i["exists"] != true || i["duplicate_count"] != 1 {
		return fail(errors.New("unique_current_contract_required"))
	}
	raw, _ := i["raw_markdown"].(string)
	if t.ContractSHA256 == "" {
		return fail(errors.New("context_contract_missing"))
	}
	if matches, _ := contractscope.MatchesFingerprint(raw, t.ContractSHA256); !matches {
		return fail(errors.New("context_contract_changed"))
	}
	ac, e := AcceptanceFingerprint(raw)
	if e != nil {
		return fail(e)
	}
	if ac != t.AcceptanceSHA256 {
		return fail(errors.New("context_acceptance_changed"))
	}
	original, ok, e := Inspect(project, t.OriginalHandoffID, now)
	if e != nil {
		return fail(e)
	}
	if !ok || original.TaskID != task || original.EventType != EventRegistrationReady {
		return fail(errors.New("original_handoff_task_mismatch"))
	}
	if matches, _ := contractscope.MatchesFingerprint(raw, original.ContractSHA256); !matches {
		return fail(errors.New("original_handoff_contract_changed"))
	}
	lineage, e := registrationLineage(project, original, controller, now)
	if e != nil {
		return fail(e)
	}
	switch t.Phase {
	case "assigned_prepared", "requirements_started", "research_started", "implementation_started", "targeted_test_started", "full_regression_started", "build_started", "worker_completed", "controller_review_pending", "controller_review", "controller_finalizing", "waiting":
	default:
		return fail(errors.New("unsupported_context_phase"))
	}
	if completion {
		if t.Phase != "worker_completed" {
			return fail(errors.New("completion_phase_evidence_missing"))
		}
		if e = checkCompletionTarget(project, *t, now); e != nil {
			return fail(e)
		}
	}
	return ContextCheck{RegistrationLineage: lineage, TaskID: task, Status: "verified", CurrentControllerAttemptID: attempt, ReportedControllerAttemptID: c.ControllerAttemptID, Target: t, ActionOwner: "/root/controller", RemainingStage: t.Phase, ResumeCondition: "Continue the existing handoff/review path; this check does not approve, claim, complete, wake or install anything."}, nil
}

func checkCompletionTarget(project string, t TaskEvidence, now time.Time) error {
	a, e := exactPolicyAttempt(project, t.TaskID, t.SourceAttemptID, now)
	if e != nil {
		return e
	}
	id := runtimeobs.PolicyIdentity(a)
	if !id.Bound || !id.Observed || id.NativeID == "" {
		return errors.New("completion_source_identity_unverified")
	}
	if t.AssignmentID == "" || a.BindingEvidence["assignment_id"] != t.AssignmentID {
		return errors.New("completion_assignment_mismatch")
	}
	assignment, e := runtimeobs.FindAssignment(project, t.AssignmentID)
	if e != nil {
		return e
	}
	if assignment.TaskID != t.TaskID || assignment.AgentPath != id.AgentPath {
		return errors.New("completion_assignment_owner_mismatch")
	}
	if !filepath.IsAbs(t.SourceWorkspace) || len(t.Source.Files) == 0 {
		return errors.New("completion_source_evidence_missing")
	}
	scope := make([]string, 0, len(t.Source.Files))
	for p := range t.Source.Files {
		scope = append(scope, p)
	}
	sort.Strings(scope)
	// Load from canonical requirements, including lossless policy supplements.
	raw, e := rolepolicy.TaskMarkdown(project, t.TaskID)
	if e != nil {
		return e
	}
	p, _, e := rolepolicy.Load(project, t.TaskID, raw)
	if e != nil {
		return e
	}
	if p != nil {
		scope = p.Scope
	}
	source, e := rolepolicy.Snapshot(t.SourceWorkspace, scope)
	if e != nil {
		return e
	}
	if source.Digest() != t.Source.Digest() {
		return errors.New("completion_source_changed")
	}
	if !filepath.IsAbs(t.ReportFile) {
		return errors.New("absolute_completion_report_required")
	}
	h, e := rolepolicy.HashFile(t.ReportFile)
	if e != nil {
		return e
	}
	if h != t.ReportSHA256 || h == "" {
		return errors.New("completion_report_changed")
	}
	// The report's explicit machine declaration prevents attaching a B-621
	// report to B-638 merely by passing B-638 on the command line.
	b, e := os.ReadFile(t.ReportFile)
	if e != nil {
		return e
	}
	var declared []TaskEvidence
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "TASK_MECCA_REPORT ") {
			var d struct {
				Targets []TaskEvidence `json:"targets"`
			}
			if e = json.Unmarshal([]byte(strings.TrimPrefix(line, "TASK_MECCA_REPORT ")), &d); e != nil {
				return e
			}
			declared = append(declared, d.Targets...)
		}
	}
	count := 0
	for _, d := range declared {
		if d.TaskID == t.TaskID {
			count++
			if d.ContractSHA256 != t.ContractSHA256 || d.AcceptanceSHA256 != t.AcceptanceSHA256 || d.SourceAttemptID != t.SourceAttemptID || d.Source.Digest() != t.Source.Digest() {
				return errors.New("completion_report_target_mismatch")
			}
		}
	}
	if count != 1 {
		return errors.New("completion_report_target_missing_or_ambiguous")
	}
	return nil
}

func completionEvent(event EventType) bool {
	return event == EventWorkerDone || event == EventCompletionReport
}

func prepareContext(req PrepareRequest, now time.Time) (string, error) {
	if !completionEvent(req.EventType) {
		return "", nil
	}
	if req.ContextFile == "" {
		return "", errors.New("completion_context_required: collect exact task/contract/AC/current attempt/original handoff/report/source evidence; owner=/root/controller; remaining=completion collection; preserve original report")
	}
	c, e := readExecutionContext(req.ContextFile)
	if e != nil {
		return "", e
	}
	checked, e := CheckContext(req.Project, req.TaskID, req.TargetAttemptID, c, true, now)
	if e != nil {
		return "", e
	}
	t := checked.Target
	if t.SourceAttemptID != req.SourceAttemptID || filepath.Clean(t.SourceWorkspace) != filepath.Clean(req.SourceWorkspace) || t.ReportFile != req.ReportFile {
		return "", errors.New("completion_request_target_mismatch")
	}
	a, e := exactPolicyAttempt(req.Project, req.TaskID, req.SourceAttemptID, now)
	if e != nil {
		return "", e
	}
	if a.AgentPath != req.SourceAgentPath {
		return "", errors.New("completion_source_path_mismatch")
	}
	b, e := json.Marshal(c)
	return string(b), e
}

func checkStoredContext(project string, row Handoff, current string, now time.Time) (ContextCheck, error) {
	if row.ContextJSON == "" {
		return contextFailure(row.TaskID, current, "completion_collection", errors.New("historical_completion_context_unverified: preserve history; collect scoped evidence for current acceptance"))
	}
	var c ExecutionContext
	if e := json.Unmarshal([]byte(row.ContextJSON), &c); e != nil {
		return contextFailure(row.TaskID, current, "context_verification", e)
	}
	reported := c.ControllerAttemptID
	if reported != current {
		// Existing verified retarget/transfer owns replacement. Preserve the
		// original observation; never rewrite the report's historical attempt.
		if row.Target.AttemptID != current || (row.TransferCount == 0 && !verifiedContextReplacement(project, reported, row.Target, now)) {
			return contextFailure(row.TaskID, current, "context_verification", errors.New("context_controller_attempt_mismatch"))
		}
		c.ControllerAttemptID = current
	}
	out, e := CheckContext(project, row.TaskID, current, c, true, now)
	out.ReportedControllerAttemptID = reported
	return out, e
}

func verifiedContextReplacement(project, old string, current Target, now time.Time) bool {
	l, e := runtimeobs.BuildLedger(project, 20, now)
	if e != nil {
		return false
	}
	for _, a := range l.Attempts {
		if a.AttemptID == old && a.Terminal {
			return sameVerifiedRuntimeIdentity(Target{AgentPath: a.AgentPath, Provider: a.Provider, SessionID: a.SessionID, RuntimeAgentID: a.RuntimeAgentID}, current)
		}
	}
	return false
}

// StartContext is an observed resume snapshot built from existing journals.
// It does not interpret a finished turn as implementation, review or success.
func StartContext(project, handoffID, current string, now time.Time) (ContextCheck, error) {
	row, ok, e := Inspect(project, handoffID, now)
	if e != nil {
		return contextFailure("", current, "start_resume", e)
	}
	if !ok {
		return contextFailure("", current, "start_resume", errors.New("original_handoff_missing"))
	}
	if completionEvent(row.EventType) {
		return checkStoredContext(project, row, current, now)
	}
	i, e := backlog.Inspect(project, "", row.TaskID)
	if e != nil {
		return contextFailure(row.TaskID, current, "start_resume", e)
	}
	raw, _ := i["raw_markdown"].(string)
	ac, e := AcceptanceFingerprint(raw)
	if e != nil {
		return contextFailure(row.TaskID, current, "start_resume", e)
	}
	owner, ownerErr := contextImplementationOwner(project, row.TaskID, raw, i)
	if ownerErr != nil {
		return contextFailure(row.TaskID, current, "implementation_owner_verification", ownerErr)
	}
	t := TaskEvidence{TaskID: row.TaskID, ContractSHA256: row.ContractSHA256, AcceptanceSHA256: ac, OriginalHandoffID: row.HandoffID, Phase: "assigned_prepared", Changes: []string{}, Tests: []CheckEvidence{}}
	assignments, e := runtimeobs.ListAssignments(project)
	if e != nil {
		return contextFailure(row.TaskID, current, "start_resume", e)
	}
	latestAssignmentAt := ""
	for _, a := range assignments {
		if a.TaskID == row.TaskID && a.AgentPath == owner.AgentPath && a.AssignedAt == latestAssignmentAt && t.AssignmentID != "" && a.AssignmentID != t.AssignmentID {
			return contextFailure(row.TaskID, current, "implementation_owner_verification", errors.New("implementation_assignment_ambiguous"))
		}
		if a.TaskID == row.TaskID && a.AgentPath == owner.AgentPath && a.AssignedAt > latestAssignmentAt {
			latestAssignmentAt = a.AssignedAt
			t.AssignmentID = a.AssignmentID
		}
	}
	var worker *WorkerObservation
	runtimeLedger, e := runtimeobs.BuildLedger(project, 20, now)
	if e != nil {
		return contextFailure(row.TaskID, current, "native_observation", e)
	}
	latestWorkerAt := ""
	var selectedIdentity rolepolicy.Identity
	for _, a := range runtimeLedger.Attempts {
		at := a.StartedAt
		if a.BindingAt > at {
			at = a.BindingAt
		}
		id := runtimeobs.PolicyIdentity(a)
		if t.AssignmentID == "" || a.TaskID != row.TaskID || a.AgentPath != owner.AgentPath || a.BindingState != runtimeobs.BindingBound || a.BindingEvidence["assignment_id"] != t.AssignmentID || !id.Observed || id.NativeID == "" {
			continue
		}
		if worker != nil && at == latestWorkerAt && a.AttemptID != worker.AttemptID {
			return contextFailure(row.TaskID, current, "implementation_owner_verification", errors.New("implementation_attempt_ambiguous"))
		}
		if at <= latestWorkerAt {
			continue
		}
		latestWorkerAt = at
		t.SourceAttemptID = a.AttemptID
		selectedIdentity = id
		worker = &WorkerObservation{AttemptID: a.AttemptID, NativeID: a.SessionID, State: a.CurrentState, TurnEnded: a.Terminal}
	}
	if worker != nil && (rolepolicy.Role(selectedIdentity.AgentPath) != owner.Role || (owner.NativeID != "" && owner.NativeID != selectedIdentity.NativeID)) {
		return contextFailure(row.TaskID, current, "implementation_owner_verification", errors.New("implementation_identity_mismatch"))
	}
	events, e := (activity.Ledger{Project: project}).List(row.TaskID)
	if e != nil {
		return contextFailure(row.TaskID, current, "start_resume", e)
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].OccurredAt.Before(events[j].OccurredAt) })
	for _, event := range events {
		if t.AssignmentID != "" && worker != nil && event.RuntimeAttemptID == worker.AttemptID && event.Role == activity.RoleWorker && event.AssignmentID == t.AssignmentID && event.PhaseEvent != "" {
			a, e := exactPolicyAttempt(project, row.TaskID, event.RuntimeAttemptID, now)
			if e != nil || a.BindingEvidence["assignment_id"] != t.AssignmentID {
				continue
			}
			t.Phase = event.PhaseEvent
			t.SourceAttemptID = event.RuntimeAttemptID
		}
	}
	c := ExecutionContext{SchemaVersion: 1, ControllerAttemptID: current, Targets: []TaskEvidence{t}}
	out, e := CheckContext(project, row.TaskID, current, c, false, now)
	if e == nil {
		out.ImplementationOwner = &owner
		out.OtherActors = contextOtherActors(row.TaskID, owner, assignments, runtimeLedger.Attempts, events)
		out.Status = "progress_observation"
		out.WorkerRuntime = worker
		if worker != nil && worker.TurnEnded {
			out.Status = "collection_gap"
			out.RemainingStage = "collect_completion_or_review_evidence_at_" + t.Phase
		}
		ledger, readErr := BuildLedger(project, now)
		if readErr != nil {
			return contextFailure(row.TaskID, current, "report_collection", readErr)
		}
		for _, h := range ledger.Handoffs {
			if h.TaskID != row.TaskID || h.ContextJSON == "" {
				continue
			}
			var prior ExecutionContext
			if json.Unmarshal([]byte(h.ContextJSON), &prior) != nil {
				continue
			}
			for _, target := range prior.Targets {
				if target.TaskID == row.TaskID {
					out.KnownReports = append(out.KnownReports, target)
				}
			}
		}
		sort.SliceStable(out.KnownReports, func(i, j int) bool { return out.KnownReports[i].ReportFile < out.KnownReports[j].ReportFile })
		out.ResumeCondition = "Inspect exact native Worker and current-phase evidence, existing reports and recovery_queue. Assigned/prepared or a finished turn is not actual implementation START, review or completion. Collect only the actual current phase; no backfill or automatic wake."
	}
	return out, e
}
