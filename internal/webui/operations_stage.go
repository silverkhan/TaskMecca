package webui

import (
	"sort"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoff"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// Stage clocks are immutable source timestamps, not the monitor's polling time.
// This projection never reports a backlog completion or edits runtime evidence.
type operationStage struct {
	Project      string `json:"project"`
	TaskID       string `json:"task_id"`
	AssignmentID string `json:"assignment_id"`
	AttemptID    string `json:"attempt_id,omitempty"`
	HandoffID    string `json:"handoff_id,omitempty"`
	Stage        string `json:"stage"`
	Evidence     string `json:"evidence"`
	Since        string `json:"since"`
	GraceUntil   string `json:"grace_until,omitempty"`
	Failed       bool   `json:"failed,omitempty"`
}

func (s operationStage) deferred(now time.Time) bool {
	until, err := time.Parse(time.RFC3339Nano, s.GraceUntil)
	return !s.Failed && err == nil && now.Before(until)
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
	return projectOperationStages(project, assignments, ledger.Attempts, transfers, now)
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
		duration := 2 * time.Minute
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
				if h.Target.AgentPath == "/root/controller" && h.ClaimedBy == h.Target.AgentPath && h.ClaimedAttemptID != "" && (h.Target.AttemptID == "" || h.Target.AttemptID == h.ClaimedAttemptID) {
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
		if prior.RecoveredAt == "" || prior.RecoveryEvidence != "latest assignment has observed execution evidence; prior runtime evidence unchanged" {
			continue
		}
		if prior.Project == signal.Project && prior.AttemptID == signal.AttemptID && prior.TaskID == signal.TaskID && prior.AgentPath == signal.AgentPath && prior.Kind == signal.Kind && prior.Evidence == signal.Evidence && prior.LastObservedAt == signal.LastObservedAt {
			return true
		}
	}
	return false
}
