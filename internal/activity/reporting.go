package activity

import (
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// ReportingRequirement is an operational reminder following a real boundary,
// never a fabricated start or authorization to backfill missing history.
type ReportingRequirement struct {
	Status         string   `json:"status"`
	RequiredPhase  string   `json:"required_phase"`
	Role           Role     `json:"role"`
	AttemptID      string   `json:"attempt_id,omitempty"`
	AgentID        string   `json:"agent_id,omitempty"`
	AssignmentID   string   `json:"assignment_id,omitempty"`
	HandoffID      string   `json:"handoff_id,omitempty"`
	ActionRequired bool     `json:"action_required"`
	Arguments      []string `json:"arguments,omitempty"`
	Diagnostic     string   `json:"diagnostic,omitempty"`
}

func CheckReporting(project, task string, role Role, attempt, assignment, handoff, phase string, now time.Time) ReportingRequirement {
	out := ReportingRequirement{Status: "unverified", Role: role, AttemptID: attempt, AssignmentID: assignment, HandoffID: handoff, RequiredPhase: phase}
	events, err := (Ledger{Project: project}).ListCached(task)
	if err != nil {
		out.Status = "error"
		out.Diagnostic = err.Error()
		return out
	}
	ledger, err := runtimeobs.BuildLedger(project, 10, now)
	if err != nil {
		out.Diagnostic = err.Error()
		return out
	}
	var actual *runtimeobs.Attempt
	for i := range ledger.Attempts {
		if ledger.Attempts[i].AttemptID == attempt {
			if actual != nil {
				out.Diagnostic = "ambiguous exact attempt"
				return out
			}
			actual = &ledger.Attempts[i]
		}
	}
	if actual == nil || actual.BindingState != runtimeobs.BindingBound || actual.RuntimeAgentID == "" {
		out.Diagnostic = "exact bound runtime required; observe before reporting"
		return out
	}
	if actual.TaskID != "" && actual.TaskID != task {
		out.Diagnostic = "runtime task mismatch"
		return out
	}
	if role == RoleWorker && assignment == "" {
		assignment = actual.BindingEvidence["assignment_id"]
		out.AssignmentID = assignment
	}
	if role == RoleWorker && (actual.AgentPath == "/root/controller" || assignment == "") {
		out.Diagnostic = "Worker assignment required"
		return out
	}
	if role == RoleController && (actual.AgentPath != "/root/controller" || handoff == "") {
		out.Diagnostic = "claimed Controller handoff required"
		return out
	}
	for _, e := range events {
		if e.AgentID == actual.RuntimeAgentID && e.RuntimeAttemptID == attempt && e.Role == role && (assignment == "" || e.AssignmentID == assignment) && (handoff == "" || e.HandoffID == handoff) {
			matches := e.PhaseEvent == phase
			if phase == "requirements_started" {
				matches = e.Action == Start || e.Action == Switch
			}
			if phase == "review_outcome" {
				matches = e.ReviewOutcome != ""
			}
			if matches {
				out.Status = "recorded"
				return out
			}
		}
	}
	out.AgentID = actual.RuntimeAgentID
	out.Status = "not_recorded"
	out.ActionRequired = true
	out.Diagnostic = "Report the actual phase now with a stable event ID; do not reconstruct past duration. Attribution and claim are checked by activity report."
	if actual.Terminal {
		out.Diagnostic = "Reporting was missed before this attempt ended. Preserve the gap; do not backfill or reuse the terminal actor."
		return out
	}
	if phase == "review_outcome" {
		out.Diagnostic = "Report the actual review outcome using review_accepted, review_rejected or review_needs_changes; no synthetic review start."
		return out
	}
	out.Arguments = []string{"activity", "report", task, "--project", project, "--role", string(role), "--agent", actual.RuntimeAgentID, "--attempt", attempt, "--event", phase, "--event-id", "<stable-current-event-id>", "--evidence", "<actual-current-phase-evidence>"}
	if assignment != "" {
		out.Arguments = append(out.Arguments, "--assignment", assignment)
	}
	if handoff != "" {
		out.Arguments = append(out.Arguments, "--handoff", handoff)
	}
	return out
}
