package activity

import (
	"errors"
	"sort"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"time"
)

// ReviewSourceRevision binds presentation to exact file contents, independently
// from the role/name and from a report's acceptance status.
func ReviewSourceRevision(source rolepolicy.Source) string {
	return source.Digest()
}

// VerifyReviewPurpose returns trace evidence, never execution permission. The
// caller retains its existing authorization and locked attribution gates.
func VerifyReviewPurpose(project string, p *rolepolicy.Policy, attempts []runtimeobs.Attempt, assignments []runtimeobs.Assignment, e Event) (runtimeobs.Attempt, rolepolicy.Source, error) {
	fail := func() (runtimeobs.Attempt, rolepolicy.Source, error) {
		return runtimeobs.Attempt{}, rolepolicy.Source{}, errors.New("exact independent review purpose/task/native/assignment/source is unverified")
	}
	if p == nil || p.ReviewRequirement == nil || rolepolicy.ValidateRequirementScope(p) != nil || p.Reviewer.Role != string(e.Role) || (e.Role != RoleWorker && e.Role != RoleReviewer) || e.Type == Implementation {
		return fail()
	}
	d, err := p.ReviewRequirement.Decide()
	if err != nil || d.Status == "omitted" {
		return fail()
	}
	var actual runtimeobs.Attempt
	count := 0
	for _, a := range attempts {
		if a.AttemptID == e.RuntimeAttemptID {
			actual = a
			count++
		}
	}
	if count != 1 || actual.TaskID != e.TaskID || actual.BindingState != runtimeobs.BindingBound || !runtimeobs.HasObservedExecution(actual) || actual.Provider == "" || actual.SessionID == "" || actual.SessionID != e.AgentID || actual.RuntimeAgentID != e.AgentID || actual.AgentPath != p.Reviewer.AgentPath || actual.SessionID != p.Reviewer.NativeID || p.Implementation.NativeID == actual.SessionID || actual.BindingEvidence["assignment_id"] != e.AssignmentID || e.AssignmentID == "" || (e.Provider != "" && e.Provider != actual.Provider) || (e.SessionID != "" && e.SessionID != actual.SessionID) {
		return fail()
	}
	count = 0
	for _, a := range assignments {
		if a.AssignmentID == e.AssignmentID && a.TaskID == e.TaskID && a.AgentPath == actual.AgentPath {
			count++
		}
	}
	if count != 1 {
		return fail()
	}
	history, err := runtimeobs.ProductImplementationHistory(project, e.TaskID, p, time.Now().UTC())
	if err != nil || rolepolicy.ValidateImplementationHistory(e.TaskID, rolepolicy.Identity{AttemptID: actual.AttemptID, NativeID: actual.SessionID, TaskID: e.TaskID, AgentPath: actual.AgentPath, Provider: actual.Provider, Bound: true, Observed: true}, history) != nil {
		return fail()
	}
	implementation := false
	for _, h := range history {
		if h.TaskID == e.TaskID && h.NativeID == p.Implementation.NativeID && h.AgentPath == p.Implementation.AgentPath && h.Bound && h.Observed {
			implementation = true
		}
	}
	if !implementation {
		return fail()
	}
	source, err := rolepolicy.Snapshot(p.Workspace, p.Scope)
	if err != nil || len(source.Files) == 0 {
		return fail()
	}
	return actual, source, nil
}

type WorkflowReviewActor struct {
	AttemptID    string `json:"attempt_id"`
	NativeID     string `json:"native_id"`
	Role         string `json:"role"`
	State        string `json:"state"`
	AssignmentID string `json:"assignment_id"`
}

// ApplyReviewPurpose projects already verified purpose evidence. It never
// changes Activity role/type, reported durations, approval or final acceptance.
func ApplyReviewPurpose(w Workflow, p ReviewerPresentation) Workflow {
	active := []WorkflowReviewActor{}
	latest := map[string]ReviewerPresentationEntry{}
	for _, v := range p.ByAttempt {
		if !v.PurposeVerified || !v.LinkageVerified || v.Purpose != "review" || v.TaskID != w.TaskID {
			continue
		}
		key := v.NativeID + "|" + v.StoredRole
		prior, ok := latest[key]
		if !ok || v.StartedAt > prior.StartedAt || (v.StartedAt == prior.StartedAt && v.AttemptID > prior.AttemptID) {
			latest[key] = v
		}
	}
	for _, v := range latest {
		active = append(active, WorkflowReviewActor{AttemptID: v.AttemptID, NativeID: v.NativeID, Role: v.StoredRole, State: v.ActualState, AssignmentID: v.AssignmentID})
	}
	sort.Slice(active, func(i, j int) bool { return active[i].AttemptID < active[j].AttemptID })
	w.Current.IndependentReviews = active
	w.Rows = append([]WorkflowRow(nil), w.Rows...)
	for i := range w.Rows {
		row := &w.Rows[i]
		if row.Intervals != nil {
			cloned := make([]WorkflowInterval, len(row.Intervals))
			copy(cloned, row.Intervals)
			row.Intervals = cloned
		}
		for j := range row.Intervals {
			interval := &row.Intervals[j]
			v, ok := p.ByAttempt[interval.AttemptID]
			if ok && v.LinkageVerified && v.PurposeVerified && v.Purpose == "review" && v.TaskID == w.TaskID && v.Source != nil && interval.AssignmentID == v.AssignmentID && interval.ContractSHA256 == v.ContractSHA256 && interval.TaskRevision == ReviewSourceRevision(*v.Source) && interval.Provider == v.Provider && interval.SessionID == v.NativeID {
				interval.Purpose = "review"
				interval.PurposeVerified = true
			}
		}
		for _, v := range p.ByAttempt {
			if v.NativeID != row.AgentID || !v.NameVerified {
				continue
			}
			if row.SessionDisplay == nil || v.NameObservedAt > row.SessionDisplay.NameObservedAt {
				copy := v
				row.SessionDisplay = &copy
			}
		}
	}
	if w.Current.Phase == "completed" || w.Current.Phase == "rework" || w.Current.Phase == "rework_requested" {
		return w
	}
	if len(active) == 0 {
		unclassified := false
		for _, row := range w.Rows {
			if row.Role != "worker" || row.AgentID != w.Current.ProcessingAgent {
				continue
			}
			for _, v := range row.Intervals {
				if v.Kind == "activity_open" && v.Type == string(Other) {
					unclassified = true
				}
			}
		}
		if !unclassified {
			return w
		}
	}
	if w.Current.Phase == "finalizing" {
		w.Current.NextOwner = "controller"
		return w
	}
	for _, row := range w.Rows {
		if row.Role != "controller" {
			continue
		}
		for _, v := range row.Intervals {
			if v.Kind == "activity_open" && v.End == nil && (v.Type == string(ControllerReview) || v.Type == string(ControllerReReview)) {
				w.Current.Phase = "controller_review"
				w.Current.ProcessingRole = "controller"
				w.Current.ProcessingAgent = row.AgentID
				w.Current.NextAction = "review_report"
				w.Current.NextOwner = "controller"
				return w
			}
		}
	}
	// Final Controller acceptance remains separate even after review stop/DONE.
	if w.Current.ProcessingRole == "controller" && w.Current.Phase == "controller_review" {
		w.Current.NextOwner = "controller"
		return w
	}
	for _, v := range active {
		if v.State == "source_unverified" || v.State == "identity_unverified" {
			w.Current.Phase = "review_recovery"
			w.Current.Basis = "review_evidence_unverified"
			w.Current.NextAction = "verify_review_evidence"
			w.Current.NextOwner = "controller"
			return w
		}
		if v.State == "reviewing" || v.State == "requested" || v.State == "result_recorded" || v.State == "unknown" {
			w.Current.Phase = "independent_review"
			w.Current.Basis = "verified_review_assignment_and_source"
			w.Current.NextAction = "await_independent_review"
			w.Current.NextOwner = v.Role
			if v.State == "result_recorded" {
				w.Current.NextAction = "review_report"
				w.Current.NextOwner = "controller"
			}
			w.Current.ProcessingRole = v.Role
			w.Current.ProcessingAgent = v.NativeID
			break
		}
	}
	return w
}
