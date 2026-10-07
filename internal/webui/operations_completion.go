package webui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// This closes an observation warning, never an execution attempt. Original
// evidence and runtime_unknown remain intact, even after task completion.
type operationCompletionResolution struct {
	Reason       string `json:"reason"`
	EventID      string `json:"event_id"`
	CompletedAt  string `json:"completed_at"`
	AssignmentID string `json:"assignment_id"`
	AttemptID    string `json:"attempt_id"`
	TaskPath     string `json:"task_path"`
	RecordedAt   string `json:"recorded_at"`
}

type operationCompletion struct {
	project     string
	taskID      string
	agentPath   string
	assignedAt  time.Time
	completedAt time.Time
	resolution  operationCompletionResolution
}

func operationCompletionEvidence(project string, ledger runtimeobs.Ledger, now time.Time) map[string]operationCompletion {
	out := map[string]operationCompletion{}
	for _, finding := range ledger.Findings {
		if finding.Severity != "info" {
			return out
		}
	}
	scan, err := backlog.ReadLifecycleTransitions(project)
	if err != nil || len(scan.Findings) != 0 {
		return out
	}
	assignments, err := runtimeobs.ListAssignments(project)
	if err != nil {
		return out
	}
	candidates, err := backlog.Discover(project, "")
	if err != nil {
		return out
	}
	rows := map[string][]backlog.Record{}
	for _, candidate := range candidates {
		// All canonical backlog namespaces, not just the Web's selected folder.
		if filepath.Dir(candidate.Path) != filepath.Join(project, "_task_mecca", "data") {
			continue
		}
		catalog, err := backlog.Catalog(project, candidate.Path)
		if err != nil {
			return map[string]operationCompletion{}
		}
		for _, row := range catalog {
			rows[strings.ToUpper(row.ID)] = append(rows[strings.ToUpper(row.ID)], row)
		}
	}
	for _, attempt := range ledger.Attempts {
		id := strings.ToUpper(attempt.TaskID)
		matches := rows[id]
		if len(matches) != 1 || matches[0].State != "done" || attempt.BindingState != runtimeobs.BindingBound {
			continue
		}
		row := matches[0]
		if !operationMeaningfulEvidence(row.Fields["결과"]) || !operationMeaningfulEvidence(row.Fields["검증"]) {
			continue
		}
		// Even an unconfirmed error/interruption is not a mere identity gap.
		if attempt.CurrentState != runtimeobs.StateRuntimeUnknown && attempt.CurrentState != runtimeobs.StateCompleted {
			continue
		}
		if attempt.CurrentState == runtimeobs.StateRuntimeUnknown && attempt.Terminal {
			continue
		}
		assignmentID := attempt.BindingEvidence["assignment_id"]
		if assignmentID == "" || attempt.AgentPath == "" {
			continue
		}
		var assignment runtimeobs.Assignment
		count := 0
		for _, item := range assignments {
			if item.AssignmentID == assignmentID {
				assignment = item
				count++
			}
		}
		if count != 1 || !strings.EqualFold(assignment.TaskID, id) || assignment.AgentPath != attempt.AgentPath || row.Fields["Agent"] != attempt.AgentPath {
			continue
		}
		assigned, err := time.Parse(time.RFC3339Nano, assignment.AssignedAt)
		if err != nil {
			continue
		}
		var completion backlog.LifecycleTransition
		count = 0
		for _, event := range scan.Events {
			if strings.EqualFold(event.TaskID, id) && event.Kind == "completed" && event.AttemptID == attempt.AttemptID && event.AssignmentID == assignmentID {
				completion = event
				count++
			}
		}
		if count != 1 || completion.Actor != "/root/controller" || completion.EvidenceSource != "controller_verified" || !operationMeaningfulEvidence(completion.EvidenceRef) {
			continue
		}
		completed, err := time.Parse(time.RFC3339Nano, completion.OccurredAt)
		if err != nil || completed.Before(assigned) || completed.After(now) {
			continue
		}
		recorded, err := time.Parse(time.RFC3339Nano, completion.RecordedAt)
		if err != nil || recorded.Before(completed) || recorded.After(now) {
			continue
		}
		valid := true
		for _, event := range scan.Events {
			if !strings.EqualFold(event.TaskID, id) || event.EventID == completion.EventID {
				continue
			}
			at, err := time.Parse(time.RFC3339Nano, event.OccurredAt)
			if err != nil || !at.Before(completed) {
				valid = false
			}
		}
		for _, item := range assignments {
			if !strings.EqualFold(item.TaskID, id) || item.AssignmentID == assignmentID {
				continue
			}
			at, err := time.Parse(time.RFC3339Nano, item.AssignedAt)
			if err != nil || !at.Before(assigned) {
				valid = false
			}
		}
		// Any late activity/binding/terminal hook stays a separate live problem.
		for _, value := range []string{attempt.FirstObservedAt, attempt.BindingAt, attempt.LastObservedAt, attempt.LastActivityAt, attempt.StartedAt, attempt.EndedAt} {
			if value == "" {
				continue
			}
			at, err := time.Parse(time.RFC3339Nano, value)
			if err != nil || at.Before(assigned) || at.After(completed) {
				valid = false
			}
		}
		if attempt.FirstObservedAt == "" || attempt.BindingAt == "" || attempt.LastObservedAt == "" || !valid {
			continue
		}
		reason := "task_completed_observation_unverified"
		if attempt.CurrentState == runtimeobs.StateCompleted {
			reason = "task_completed_runtime_observed"
		}
		out[attempt.AttemptID] = operationCompletion{project: project, taskID: id, agentPath: attempt.AgentPath, assignedAt: assigned, completedAt: completed,
			resolution: operationCompletionResolution{Reason: reason, EventID: completion.EventID, CompletedAt: completion.OccurredAt,
				AssignmentID: assignmentID, AttemptID: attempt.AttemptID, TaskPath: row.Path, RecordedAt: now.UTC().Format(time.RFC3339Nano)}}
	}
	return out
}

func operationMeaningfulEvidence(value string) bool {
	value = strings.ToLower(strings.Trim(value, " -—\t\n\r"))
	return value != "" && value != "-" && value != "—" && value != "unknown" && value != "미확인"
}

func operationCompletedUnknown(incident operationIncident, completions map[string]operationCompletion) *operationCompletionResolution {
	if incident.AttemptID == "" && incident.Kind == "runtime_unknown" && incident.Quality == "verification_required" && incident.Evidence == "doing assignment has no linked runtime attempt" {
		detected, err := time.Parse(time.RFC3339Nano, incident.DetectedAt)
		if err != nil {
			return nil
		}
		var proof *operationCompletionResolution
		for _, completion := range completions {
			if filepath.Clean(incident.Project) != filepath.Clean(completion.project) || !strings.EqualFold(incident.TaskID, completion.taskID) || detected.After(completion.completedAt) {
				continue
			}
			if proof != nil {
				return nil
			}
			copy := completion.resolution
			copy.Reason = "latest_assignment_controller_verified_completion"
			proof = &copy
		}
		return proof
	}
	eligible := incident.Kind == "runtime_unknown" && incident.Evidence == "runtime identity or hook execution unverified"
	eligible = eligible || incident.Kind == "no_signal" || incident.Kind == "handoff_stalled"
	if !eligible || incident.Quality != "verification_required" || incident.AttemptID == "" {
		return nil
	}
	completion, ok := completions[incident.AttemptID]
	if !ok || filepath.Clean(incident.Project) != filepath.Clean(completion.project) || !strings.EqualFold(incident.TaskID, completion.taskID) || incident.AgentPath != completion.agentPath {
		return nil
	}
	observed, err := time.Parse(time.RFC3339Nano, incident.LastObservedAt)
	if err != nil || observed.Before(completion.assignedAt) || observed.After(completion.completedAt) {
		return nil
	}
	resolution := completion.resolution
	return &resolution
}

// Supported one-project reconciliation intentionally never calls delivery,
// edits task files/lifecycle facts, or manufactures runtime terminal events.
func ReconcileCompletedOperation(project, incidentID string, now time.Time, dryRun bool) (map[string]any, error) {
	operationMu.Lock()
	defer operationMu.Unlock()
	if !dryRun {
		release, err := lockOperationJournal(project)
		if err != nil {
			return nil, err
		}
		defer release()
	}
	journal, err := readOperationJournal(project)
	if err != nil {
		return nil, err
	}
	if filepath.Clean(journal.Project) != filepath.Clean(project) {
		return nil, fmt.Errorf("operations journal project scope mismatch")
	}
	matches := 0
	for _, incident := range journal.Incidents {
		if incident.ID == incidentID {
			matches++
		}
	}
	if matches != 1 {
		return nil, fmt.Errorf("exact unique incident required: %s", incidentID)
	}
	ledger, err := runtimeobs.BuildLedger(project, 3, now)
	if err != nil {
		return nil, err
	}
	completions := operationCompletionEvidence(project, ledger, now)
	for i := range journal.Incidents {
		incident := &journal.Incidents[i]
		if incident.ID != incidentID {
			continue
		}
		if incident.Resolution != nil && incident.RecoveredAt != "" {
			return map[string]any{"project": project, "dry_run": dryRun, "notifications_sent": false, "changed": false, "incident": *incident}, nil
		}
		resolution := operationCompletedUnknown(*incident, completions)
		if resolution == nil || incident.RecoveredAt != "" {
			return nil, fmt.Errorf("incident %s lacks unique verified completion mapping", incidentID)
		}
		incident.Resolution = resolution
		incident.RecoveredAt = now.UTC().Format(time.RFC3339Nano)
		if !dryRun {
			if err := writeOperationJournal(project, journal); err != nil {
				return nil, err
			}
		}
		return map[string]any{"project": project, "dry_run": dryRun, "notifications_sent": false, "changed": true, "incident": *incident}, nil
	}
	return nil, fmt.Errorf("exact incident not found: %s", incidentID)
}
