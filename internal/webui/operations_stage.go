package webui

import (
	"sort"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/handoff"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// Stage clocks are immutable source timestamps, not the monitor's polling time.
// This projection never reports a backlog completion or edits runtime evidence.
type operationStage struct {
	Project            string `json:"project"`
	TaskID             string `json:"task_id"`
	AssignmentID       string `json:"assignment_id"`
	AttemptID          string `json:"attempt_id,omitempty"`
	HandoffID          string `json:"handoff_id,omitempty"`
	Stage              string `json:"stage"`
	Evidence           string `json:"evidence"`
	Since              string `json:"since"`
	GraceUntil         string `json:"grace_until,omitempty"`
	ControllerReview   bool   `json:"controller_review,omitempty"`
	Failed             bool   `json:"failed,omitempty"`
	Reason             string `json:"reason,omitempty"`
	ObservationQuality string `json:"observation_quality,omitempty"`
	Severity           string `json:"severity,omitempty"`
	ActionRequired     bool   `json:"action_required,omitempty"`
	Action             string `json:"action,omitempty"`
}

func (s operationStage) deferred(now time.Time) bool {
	until, err := time.Parse(time.RFC3339Nano, s.GraceUntil)
	return !s.Failed && err == nil && now.Before(until)
}

// Retired assignments are historical policy evidence, not pending work.
// Keep proof on recovered incidents, never present a released hold as task
// completion. Failed handoffs remain independent actionable current stages.
func currentOperationStages(stages []operationStage, policies map[string]operationPolicy) []operationStage {
	out := make([]operationStage, 0, len(stages))
	for _, stage := range stages {
		if policy, ok := policies[strings.ToUpper(stage.TaskID)]; ok && policy.assignments[stage.AssignmentID] && !stage.Failed {
			continue
		}
		out = append(out, stage)
	}
	return out
}

func exactControllerStageLive(h handoff.Handoff, attempts []runtimeobs.Attempt, now time.Time) bool {
	claimedAt, err := time.Parse(time.RFC3339Nano, h.ClaimedAt)
	if err != nil || claimedAt.After(now) {
		return false
	}
	var exact *runtimeobs.Attempt
	for i := range attempts {
		a := &attempts[i]
		if a.AttemptID != h.Target.AttemptID {
			continue
		}
		if exact != nil {
			return false
		}
		exact = a
	}
	if exact == nil || exact.AgentPath != h.Target.AgentPath || exact.RuntimeAgentID != h.Target.RuntimeAgentID || exact.SessionID != h.Target.SessionID || exact.Provider != h.Target.Provider || exact.BindingState != runtimeobs.BindingBound || exact.StateEvidenceSource != runtimeobs.EvidenceHook || (exact.StateObservationQuality != runtimeobs.QualityObserved && exact.StateObservationQuality != runtimeobs.QualityAuthoritative) || exact.CurrentState != runtimeobs.StateRunning || exact.Terminal {
		return false
	}
	last, err := time.Parse(time.RFC3339Nano, exact.LastActivityAt)
	return err == nil && !last.Before(claimedAt.Add(-15*time.Minute)) && !last.After(now)
}

func operationStages(project string, ledger runtimeobs.Ledger, now time.Time) []operationStage {
	for _, finding := range ledger.Findings {
		if finding.Severity != "info" {
			return nil
		}
	}
	assignments, err := runtimeobs.ListAssignments(project)
	if err != nil {
		return nil
	}
	transfers, err := handoff.BuildLedger(project, now)
	if err != nil || len(transfers.Findings) != 0 {
		return nil
	}
	stages := projectOperationStages(project, assignments, ledger.Attempts, transfers, now)
	reviews := backlog.CompletionReviewStates(project, "", now)
	for i := range stages {
		if r := reviews[stages[i].TaskID]; r != nil {
			if !applyCompletionReviewStage(&stages[i], r) {
				continue
			}
		}
	}
	return stages
}

func applyCompletionReviewStage(stage *operationStage, review map[string]any) bool {
	state, _ := review["state"].(string)
	confirmedFailure, _ := review["confirmed_failure"].(bool)
	stage.ControllerReview = true
	stage.Since, _ = review["since"].(string)
	stage.GraceUntil, _ = review["grace_until"].(string)
	stage.Evidence, _ = review["diagnostic"].(string)
	stage.Reason, _ = review["reason"].(string)
	stage.ObservationQuality, _ = review["observation_quality"].(string)
	stage.Severity, _ = review["severity"].(string)
	stage.ActionRequired, _ = review["action_required"].(bool)
	stage.Action, _ = review["action"].(string)
	stage.Failed = false

	switch state {
	case "controller_review_pending":
		if stage.Reason == "controller_claim_missing" {
			stage.Stage = "handoff_pending"
		} else {
			stage.Stage = "handoff_evidence_unverified"
		}
	case "controller_review":
		stage.Stage = "controller_review"
	case "controller_finalizing":
		stage.Stage = "review_complete"
	case "controller_recovery":
		if !confirmedFailure {
			stage.Stage = "handoff_evidence_unverified"
			stage.Severity = "warning"
			return true
		}
		stage.Stage = "handoff_failed"
		stage.Failed = true
		stage.Severity = "error"
	default:
		return false
	}
	return true
}

func projectOperationStages(project string, assignments []runtimeobs.Assignment, attempts []runtimeobs.Attempt, transfers handoff.Ledger, now time.Time) []operationStage {
	latest := map[string]runtimeobs.Assignment{}
	ambiguous := map[string]bool{}
	for _, a := range assignments {
		at, err := time.Parse(time.RFC3339Nano, a.AssignedAt)
		if err != nil || at.After(now) {
			continue
		}
		id := strings.ToUpper(a.TaskID)
		prior, exists := latest[id]
		before, _ := time.Parse(time.RFC3339Nano, prior.AssignedAt)
		if !exists || at.After(before) {
			latest[id] = a
			ambiguous[id] = false
		} else if at.Equal(before) && prior.AssignmentID != a.AssignmentID {
			ambiguous[id] = true
		}
	}
	out := []operationStage{}
	for id, a := range latest {
		if ambiguous[id] {
			continue
		}
		s := operationStage{Project: project, TaskID: id, AssignmentID: a.AssignmentID, Stage: "assignment_pending", Evidence: "latest durable assignment", Since: a.AssignedAt}
		duration := runtimeobs.AssignmentObservationGrace
		matched := []runtimeobs.Attempt{}
		for _, attempt := range attempts {
			if attempt.BindingState == runtimeobs.BindingBound && strings.EqualFold(attempt.TaskID, id) && attempt.AgentPath == a.AgentPath && attempt.BindingEvidence["assignment_id"] == a.AssignmentID {
				matched = append(matched, attempt)
			}
		}
		if len(matched) > 1 {
			continue
		}
		if len(matched) == 1 {
			attempt := matched[0]
			s.AttemptID = attempt.AttemptID
			s.Stage = "runtime_observation_pending"
			s.Evidence = "latest assignment bound to runtime"
			duration = 0
			if attempt.CurrentState == runtimeobs.StateRunning && !attempt.Terminal && attempt.StateEvidenceSource == runtimeobs.EvidenceHook && (attempt.StateObservationQuality == runtimeobs.QualityObserved || attempt.StateObservationQuality == runtimeobs.QualityAuthoritative) {
				s.Stage = "worker_running"
			}
			if attempt.CurrentState == runtimeobs.StateCompleted && attempt.Terminal && attempt.StateEvidenceSource == runtimeobs.EvidenceHook && (attempt.StateObservationQuality == runtimeobs.QualityObserved || attempt.StateObservationQuality == runtimeobs.QualityAuthoritative) {
				s.Stage = "worker_report_pending"
				s.Since = attempt.EndedAt
				s.Evidence = "observed worker completion; report pending"
				duration = 5 * time.Minute
			}
			var selected *handoff.Handoff
			duplicate := false
			for _, h := range transfers.Handoffs {
				if h.EventType != handoff.EventWorkerDone || !strings.EqualFold(h.TaskID, id) || h.SourceAgentPath != a.AgentPath || h.SourceAttemptID != attempt.AttemptID {
					continue
				}
				prepared, e := time.Parse(time.RFC3339Nano, h.PreparedAt)
				assigned, _ := time.Parse(time.RFC3339Nano, a.AssignedAt)
				if e != nil || prepared.Before(assigned) || prepared.After(now) {
					continue
				}
				if selected != nil {
					duplicate = true
				}
				copy := h
				selected = &copy
			}
			if duplicate {
				continue
			}
			if selected != nil {
				h := *selected
				s.HandoffID = h.HandoffID
				s.Stage = "handoff_pending"
				s.Since = h.PreparedAt
				s.Evidence = "durable worker_done handoff"
				duration = 5 * time.Minute
				if h.Target.AgentPath == "/root/controller" && h.ClaimedBy == h.Target.AgentPath && h.ClaimedAttemptID != "" && h.Target.AttemptID != "" && h.Target.RuntimeAgentID != "" && h.Target.SessionID != "" && h.Target.Provider != "" && h.Target.AttemptID == h.ClaimedAttemptID && exactControllerStageLive(h, attempts, now) {
					s.Stage = "controller_review"
					s.Since = h.ClaimedAt
					s.Evidence = "handoff claimed by Controller"
					duration = 15 * time.Minute
				}
				if step, ok := h.Steps["acceptance"]; ok && step.Result == "ok" && s.Stage == "controller_review" {
					s.Stage = "review_complete"
					s.Since = step.ObservedAt
					s.Evidence = "Controller acceptance recorded; finalization pending"
					duration = 10 * time.Minute
				}
				if h.Applied {
					s.Stage = "handoff_applied"
					s.Since = h.AppliedAt
					s.Evidence = "handoff applied; backlog completion remains separately verified"
					duration = 0
				}
				failedAt := ""
				for _, step := range h.Steps {
					if step.Result == "failed" {
						at, parseErr := time.Parse(time.RFC3339Nano, step.ObservedAt)
						candidate := step.ObservedAt
						if parseErr != nil || at.After(now) {
							at, parseErr = time.Parse(time.RFC3339Nano, h.PreparedAt)
							candidate = h.PreparedAt
						}
						prior, _ := time.Parse(time.RFC3339Nano, failedAt)
						if parseErr == nil && !at.After(now) && (failedAt == "" || at.Before(prior)) {
							failedAt = candidate
						}
					}
				}
				if failedAt != "" {
					s.Stage = "handoff_failed"
					s.Since = failedAt
					s.Evidence = "handoff step failed"
					s.Failed = true
					duration = 0
				}
			}
		}
		since, e := time.Parse(time.RFC3339Nano, s.Since)
		if e != nil || since.After(now) {
			continue
		}
		if duration > 0 {
			s.GraceUntil = since.Add(duration).UTC().Format(time.RFC3339Nano)
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TaskID < out[j].TaskID })
	return out
}

func stageForIncident(item operationIncident, stages []operationStage) *operationStage {
	for _, s := range stages {
		if strings.EqualFold(s.TaskID, item.TaskID) && (item.AttemptID == "" || item.AttemptID == s.AttemptID) {
			copy := s
			return &copy
		}
	}
	return nil
}

func stageSuppresses(item operationIncident, stages []operationStage, now time.Time) bool {
	if item.Kind != "runtime_unknown" && item.Kind != "no_signal" {
		return false
	}
	if item.Kind == "runtime_unknown" && item.Evidence != "runtime identity or hook execution unverified" && item.Evidence != "doing assignment has no linked runtime attempt" {
		return false
	}
	s := stageForIncident(item, stages)
	return s != nil && s.deferred(now)
}

func latestOperationRecovery(item operationIncident, attempts []runtimeobs.Attempt, stages []operationStage, now time.Time) bool {
	if item.Kind != "runtime_unknown" && item.Kind != "no_signal" {
		return false
	}
	if item.Kind == "runtime_unknown" && item.Evidence != "runtime identity or hook execution unverified" && item.Evidence != "doing assignment has no linked runtime attempt" {
		return false
	}
	detected, err := time.Parse(time.RFC3339Nano, item.DetectedAt)
	if item.DetectedAt == "" {
		detected, err = time.Parse(time.RFC3339Nano, item.LastObservedAt)
	}
	if err != nil {
		return false
	}
	for _, s := range stages {
		if !strings.EqualFold(s.TaskID, item.TaskID) || s.AttemptID == "" || s.AttemptID == item.AttemptID {
			continue
		}
		for _, a := range attempts {
			if a.AttemptID != s.AttemptID || a.BindingState != runtimeobs.BindingBound || a.Terminal || a.CurrentState != runtimeobs.StateRunning || a.StateEvidenceSource != runtimeobs.EvidenceHook || (a.StateObservationQuality != runtimeobs.QualityObserved && a.StateObservationQuality != runtimeobs.QualityAuthoritative) {
				continue
			}
			observed, e := time.Parse(time.RFC3339Nano, a.LastObservedAt)
			if e == nil && !observed.Before(detected) && !observed.After(now) {
				return true
			}
		}
	}
	return false
}

func operationSourceAlreadyResolved(signal operationIncident, journal operationJournal) bool {
	if signal.Kind != "runtime_unknown" && signal.Kind != "no_signal" {
		return false
	}
	for _, prior := range journal.Incidents {
		if prior.RecoveredAt == "" || prior.RecoveryEvidence != "same assignment has recent observed Hook execution; prior runtime evidence unchanged" {
			continue
		}
		sameSourceClock := prior.LastObservedAt == signal.LastObservedAt || (prior.RecoveryEvidence == "same assignment has recent observed Hook execution; prior runtime evidence unchanged" && prior.RecoveredAt == signal.LastObservedAt)
		if prior.Project == signal.Project && prior.AttemptID == signal.AttemptID && prior.AssignmentID == signal.AssignmentID && prior.TaskID == signal.TaskID && prior.AgentPath == signal.AgentPath && prior.Kind == signal.Kind && prior.Evidence == signal.Evidence && sameSourceClock {
			return true
		}
	}
	return false
}

func operationLatestAssignmentSourceAlreadyResolved(signal operationIncident, journal operationJournal) bool {
	if signal.Kind != "runtime_unknown" && signal.Kind != "no_signal" {
		return false
	}
	for _, prior := range journal.Incidents {
		if prior.RecoveredAt == "" || prior.RecoveryEvidence != "latest assignment has observed execution evidence; prior runtime evidence unchanged" {
			continue
		}
		if prior.Project == signal.Project && prior.AttemptID == signal.AttemptID && prior.AssignmentID == signal.AssignmentID && prior.TaskID == signal.TaskID && prior.AgentPath == signal.AgentPath && prior.Kind == signal.Kind && prior.Evidence == signal.Evidence && prior.LastObservedAt == signal.LastObservedAt {
			return true
		}
	}
	return false
}

func operationCanonicalTaskIsDoing(project, taskID string) bool {
	rows, ok := operationCanonicalRows(project)
	if !ok {
		return false
	}
	matches := rows[strings.ToUpper(strings.TrimSpace(taskID))]
	return len(matches) == 1 && matches[0].Location == "active" && matches[0].State == "doing"
}
