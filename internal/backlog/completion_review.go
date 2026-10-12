package backlog

// This is a read-only projection of the optional dev handoff journal. Stable
// does not own that journal and must never manufacture claims or completion.
import (
	"crypto/sha256"
	"encoding/hex"

	"fmt"
	"os"

	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type completionHandoffRecord struct {
	HandoffID       string `json:"handoff_id"`
	Kind            string `json:"record_kind"`
	Event           string `json:"event_type"`
	TaskID          string `json:"task_id"`
	Contract        string `json:"contract_sha256"`
	SourceAgent     string `json:"source_agent_path"`
	SourceAttempt   string `json:"source_attempt_id"`
	TargetAgent     string `json:"target_agent_path"`
	TargetAttempt   string `json:"target_attempt_id"`
	TargetRuntimeID string `json:"target_runtime_agent_id"`
	TargetSession   string `json:"target_session_id"`
	Provider        string `json:"provider"`
	ClaimedBy       string `json:"claimed_by"`
	ClaimedAttempt  string `json:"claimed_attempt_id"`
	Step            string `json:"step"`
	Result          string `json:"result"`
	At              string `json:"observed_at"`
	Action          string `json:"action"`
}

type completionHandoff struct {
	prepared    completionHandoffRecord
	claim       completionHandoffRecord
	acceptance  completionHandoffRecord
	applied     completionHandoffRecord
	failed      bool
	failureStep string
	steps       map[string]string
}

func readCompletionHandoffs(project string) (map[string]*completionHandoff, error) {
	ledger, err := handoffjournal.BuildLedger(project, time.Now())
	if err != nil {
		return nil, err
	}
	if len(ledger.Findings) > 0 {
		return nil, fmt.Errorf("handoff journal: %s", ledger.Findings[0].Message)
	}
	rows := map[string]*completionHandoff{}
	for id, v := range ledger.Handoffs {
		h := &completionHandoff{steps: map[string]string{}}
		h.prepared = completionHandoffRecord{HandoffID: id, Event: string(v.EventType), TaskID: v.TaskID, Contract: v.ContractSHA256, SourceAgent: v.SourceAgentPath, SourceAttempt: v.SourceAttemptID, TargetAgent: v.Target.AgentPath, TargetAttempt: v.Target.AttemptID, TargetRuntimeID: v.Target.RuntimeAgentID, TargetSession: v.Target.SessionID, Provider: v.Target.Provider, Action: string(v.Action), At: v.PreparedAt}
		h.claim = completionHandoffRecord{At: v.ClaimedAt, ClaimedBy: v.ClaimedBy, ClaimedAttempt: v.ClaimedAttemptID}
		for name, step := range v.Steps {
			if name == "root-reported" {
				continue
			}
			h.steps[name] = step.Result
			if step.Result != "ok" {
				h.failed = true
				if step.Result == "failed" && h.failureStep == "" {
					h.failureStep = name
				}
			}
			if name == "acceptance" && step.Result == "ok" {
				h.acceptance = completionHandoffRecord{At: step.ObservedAt}
			}
		}
		if v.Applied {
			h.applied = completionHandoffRecord{At: v.AppliedAt}
		}
		rows[id] = h
	}
	return rows, nil
}

// Same contract-section normalization as dev's handoff.ContractFingerprint.
func completionContract(markdown string) string {
	markdown = strings.ReplaceAll(strings.ReplaceAll(markdown, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(markdown, "\n")
	start, end := -1, len(lines)
	for i, line := range lines {
		s := strings.TrimSpace(line)
		if start < 0 && (s == "## 작업 정의" || s == "## 요건 정의서") {
			start = i
			continue
		}
		if start >= 0 && strings.HasPrefix(s, "## ") {
			end = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	for i := start; i < end; i++ {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(strings.Join(lines[start:end], "\n"))))
	return hex.EncodeToString(sum[:])
}

func completionTimeBefore(left, right string) bool {
	l, lOK := parseTime(left)
	r, rOK := parseTime(right)
	return lOK && rOK && l.Before(r)
}

func completionReviewProjection(state, since string, grace time.Duration, diagnostic, reason, quality, action, severity string, failed bool, handoffID, contract string) map[string]any {
	graceUntil := ""
	if at, ok := parseTime(since); ok {
		graceUntil = at.Add(grace).Format(time.RFC3339Nano)
	}
	return map[string]any{
		"state": state, "since": since, "grace_until": graceUntil, "diagnostic": diagnostic,
		"reason": reason, "observation_quality": quality, "action_required": action != "",
		"action": action, "severity": severity, "confirmed_failure": failed,
		"handoff_id": handoffID, "contract_sha256": contract, "audience": "controller",
	}
}

func completionReview(row Record, worker *runtimeobs.Attempt, assignmentID, completedAt string, handoffs map[string]*completionHandoff, readErr error, attempts []runtimeobs.Attempt, now time.Time) map[string]any {
	state, since, diagnostic := "controller_review_pending", completedAt, "Controller 인계 확인을 기다리고 있습니다."
	reason, quality, action, severity := "controller_claim_missing", "insufficient", "verify_controller_claim", "info"
	confirmedFailure := false
	grace := 5 * time.Minute
	handoffID := ""
	if readErr != nil && !os.IsNotExist(readErr) {
		state = "controller_recovery"
		diagnostic = "인계 원천 근거를 읽을 수 없어 내부 검토가 필요합니다."
	}
	var selected *completionHandoff
	if worker != nil && assignmentID != "" && worker.BindingEvidence["assignment_id"] == assignmentID {
		contract := completionContract(row.RawMarkdown)
		for id, h := range handoffs {
			p := h.prepared
			if p.Event != "worker_done" || !strings.EqualFold(p.TaskID, row.ID) || p.SourceAgent != row.Fields["Agent"] || p.SourceAttempt != worker.AttemptID || contract == "" || p.Contract != contract {
				continue
			}
			at, ok := parseTime(p.At)
			started, startedOK := parseTime(worker.StartedAt)
			// DONE is sent before the Worker turn's terminal hook. A report
			// can therefore precede EndedAt while belonging to this attempt.
			if !ok || !startedOK || at.Before(started) || at.After(now) {
				continue
			}
			// The first matching report is the phase anchor; repeated polls/reports
			// must not extend the normal completion-review grace period.
			if selected == nil || completionTimeBefore(p.At, selected.prepared.At) {
				selected = h
				handoffID = id
			}
		}
	}
	if selected != nil {
		p, c := selected.prepared, selected.claim
		since = p.At
		if selected.failureStep != "" {
			state = "controller_recovery"
			reason, quality, action, severity, confirmedFailure = "handoff_step_failed", "confirmed", "investigate_failed_handoff_step", "error", true
			diagnostic = fmt.Sprintf("인계 단계 %q의 실패가 명시적으로 기록되었습니다.", selected.failureStep)
		} else if selected.failed {
			state = "controller_review_pending"
			reason, quality, action, severity = "handoff_step_outcome_unknown", "insufficient", "inspect_handoff_step_result", "warning"
			diagnostic = "인계 단계 결과가 확정되지 않아 내부 검토가 필요합니다."
		} else if p.Action == "hold" {
			state = "needs_user"
			reason, quality, action, severity = "handoff_on_user_hold", "observed", "wait_for_user_decision", "warning"
			diagnostic = "인계가 사용자 판단을 기다리는 hold 상태입니다."
		}
		if c.At != "" && state != "controller_recovery" {
			var controller *runtimeobs.Attempt
			for i := range attempts {
				a := &attempts[i]
				if a.AttemptID == p.TargetAttempt {
					if controller != nil {
						controller = nil
						break
					}
					controller = a
				}
			}
			valid := p.TargetAgent == "/root/controller" && p.TargetAttempt != "" && p.TargetRuntimeID != "" && p.TargetSession != "" && p.Provider != "" && c.ClaimedBy == p.TargetAgent && c.ClaimedAttempt == p.TargetAttempt && controller != nil && controller.AgentPath == p.TargetAgent && controller.RuntimeAgentID == p.TargetRuntimeID && controller.SessionID == p.TargetSession && controller.Provider == p.Provider && controller.BindingState == runtimeobs.BindingBound && controller.StateEvidenceSource == runtimeobs.EvidenceHook && (controller.StateObservationQuality == runtimeobs.QualityObserved || controller.StateObservationQuality == runtimeobs.QualityAuthoritative)
			claimAt, claimOK := parseTime(c.At)
			preparedAt, _ := parseTime(p.At)
			if !valid || !claimOK || claimAt.Before(preparedAt) || claimAt.After(now) {
				state = "controller_recovery"
				reason, quality, action, severity = "controller_claim_unverified", "insufficient", "verify_exact_controller_binding", "warning"
				diagnostic = "인계를 받은 Controller의 정확한 실행 근거를 확인할 수 없습니다."
			} else {
				lastActivity, activityOK := parseTime(controller.LastActivityAt)
				activityPlausible := activityOK && !lastActivity.After(now) && !lastActivity.Before(claimAt.Add(-15*time.Minute))
				state, since, grace, diagnostic = "controller_review", c.At, 15*time.Minute, "Controller가 결과와 수용 기준을 검토하고 있습니다."
				reason, quality, action, severity = "exact_controller_claim_running", "observed", "review_worker_result", "info"
				if !activityPlausible {
					state = "controller_recovery"
					reason, quality, action, severity = "controller_runtime_activity_unverified", "insufficient", "verify_controller_runtime_activity", "warning"
					diagnostic = "정확한 Controller 런타임 활동 시각을 확인할 수 없어 내부 검토를 복구 대기 상태로 표시합니다."
				}
				if state != "controller_recovery" && (controller.CurrentState == runtimeobs.StateWaitingUser || controller.CurrentState == runtimeobs.StateWaitingApproval) {
					state, diagnostic = "needs_user", "Controller가 명시적으로 사용자 판단 또는 승인을 기다리고 있습니다."
				} else if state != "controller_recovery" && (controller.CurrentState != runtimeobs.StateRunning || controller.Terminal) {
					state = "controller_recovery"
					if controller.Terminal && controller.StateEvidenceSource == runtimeobs.EvidenceHook && (controller.StateObservationQuality == runtimeobs.QualityObserved || controller.StateObservationQuality == runtimeobs.QualityAuthoritative) && (controller.CurrentState == runtimeobs.StateErrored || controller.CurrentState == runtimeobs.StateInterrupted || controller.CurrentState == runtimeobs.StateShutdown) {
						reason, quality, action, severity, confirmedFailure = "controller_execution_failed", "confirmed", "investigate_controller_execution", "error", true
						diagnostic = "Controller 실행의 오류 또는 중단이 Hook 상태로 관측되었습니다."
					} else {
						reason, quality, action, severity = "controller_claim_not_running", "insufficient", "verify_controller_execution_state", "warning"
						diagnostic = "Controller claim은 확인됐지만 running 또는 명시적 실패 근거가 없습니다."
					}
				}
				if state == "controller_review" {
					accepted := selected.acceptance.At
					if accepted == "" {
						accepted = selected.applied.At
					}
					if accepted != "" {
						at, ok := parseTime(accepted)
						if !ok || at.Before(claimAt) || at.After(now) {
							state = "controller_recovery"
							diagnostic = "완료 검토 단계의 시각 근거를 확인할 수 없습니다."
						} else {
							state, since, grace, diagnostic = "controller_finalizing", accepted, 10*time.Minute, "Controller 검증을 마쳤으며 백로그 완료 처리를 기다리고 있습니다."
						}
					}
				}
			}
		}
	}
	at, ok := parseTime(since)
	if !ok || at.After(now) {
		state = "controller_review_pending"
		reason, quality, action, severity = "source_timestamp_unverified", "insufficient", "verify_source_timestamp", "warning"
		diagnostic = "완료 보고의 원천 시각이 유효하지 않거나 미래 시각이어서 내부 검토가 필요합니다."
	} else {
		if !now.Before(at.Add(grace)) && state != "controller_recovery" && state != "needs_user" {
			state = "controller_recovery"
			diagnostic = "Controller 인계·검토의 정상 대기 시간을 초과했습니다. Controller가 실행 근거를 확인하고 복구해야 합니다."
		}
	}
	return completionReviewProjection(state, since, grace, diagnostic, reason, quality, action, severity, confirmedFailure, handoffID, completionContract(row.RawMarkdown))
}

func reportedCompletionAt(row Record, worker *runtimeobs.Attempt, handoffs map[string]*completionHandoff, now time.Time) string {
	if worker == nil {
		return ""
	}
	contract := completionContract(row.RawMarkdown)
	started, ok := parseTime(worker.StartedAt)
	if !ok {
		return ""
	}
	result := ""
	for _, h := range handoffs {
		p := h.prepared
		at, ok := parseTime(p.At)
		if p.Event == "worker_done" && strings.EqualFold(p.TaskID, row.ID) && p.SourceAgent == worker.AgentPath && p.SourceAttempt == worker.AttemptID && contract != "" && p.Contract == contract && ok && !at.Before(started) && !at.After(now) {
			if result == "" || completionTimeBefore(p.At, result) {
				result = p.At
			}
		}
	}
	return result
}

func latestCompletionAssignment(assignments []runtimeobs.Assignment, taskID string, now time.Time) (string, bool) {
	var latest *runtimeobs.Assignment
	var latestAt time.Time
	ambiguous := false
	for i := range assignments {
		a := &assignments[i]
		if !strings.EqualFold(a.TaskID, taskID) {
			continue
		}
		at, ok := parseTime(a.AssignedAt)
		if !ok || at.After(now) {
			continue
		}
		if latest == nil || at.After(latestAt) {
			latest, latestAt, ambiguous = a, at, false
		} else if at.Equal(latestAt) && a.AssignmentID != latest.AssignmentID {
			ambiguous = true
		}
	}
	if latest == nil || ambiguous {
		return "", false
	}
	return latest.AssignmentID, true
}

func applyCompletionReviews(project string, rows []Record, activity map[string]map[string]any, now time.Time, shared ...requestRuntime) {
	var ledger runtimeobs.Ledger
	var ledgerErr error
	if len(shared) > 0 {
		ledger, ledgerErr = readRequestRuntime(project, 10, now, shared...)
	} else {
		ledger, ledgerErr = runtimeobs.BuildLedger(project, 10, now)
	}
	handoffs, handoffErr := readCompletionHandoffs(project)
	assignments, assignmentErr := runtimeobs.ListAssignments(project)
	for _, row := range rows {
		if row.Location != "active" || row.State != "doing" {
			continue
		}
		signal := activity[row.ID]
		assignmentID, assignmentOK := latestCompletionAssignment(assignments, row.ID, now)
		if !assignmentOK {
			assignmentID = ""
		}
		if signal == nil || toString(signal["health"]) == "needs_user" {
			continue
		}
		var worker *runtimeobs.Attempt
		ambiguous := false
		for i := range ledger.Attempts {
			a := &ledger.Attempts[i]
			if a.BindingState != runtimeobs.BindingBound || !strings.EqualFold(a.TaskID, row.ID) || a.AgentPath != row.Fields["Agent"] {
				continue
			}
			if worker == nil || completionTimeBefore(worker.StartedAt, a.StartedAt) {
				worker = a
				ambiguous = false
			} else if !completionTimeBefore(a.StartedAt, worker.StartedAt) && !completionTimeBefore(worker.StartedAt, a.StartedAt) {
				ambiguous = true
			}
		}
		completedAt := toString(signal["last_activity_at"])
		workerFailed := false
		if worker != nil && !ambiguous {
			if worker.CurrentState == runtimeobs.StateWaitingUser || worker.CurrentState == runtimeobs.StateWaitingApproval {
				signal["health"] = "needs_user"
				continue
			}
			reportAt := reportedCompletionAt(row, worker, handoffs, now)
			terminalDone := worker.CurrentState == runtimeobs.StateCompleted && worker.Terminal && worker.StateEvidenceSource == runtimeobs.EvidenceHook && (worker.StateObservationQuality == runtimeobs.QualityObserved || worker.StateObservationQuality == runtimeobs.QualityAuthoritative)
			if !terminalDone && reportAt == "" {
				// A newer active/error attempt must not inherit an old completed heartbeat.
				if toString(signal["health"]) == "awaiting_finalize" {
					signal["health"] = "runtime_unknown"
				}
				continue
			}
			completedAt = worker.EndedAt
			if !terminalDone {
				completedAt = reportAt
				if worker.CurrentState != runtimeobs.StateRunning || worker.Terminal {
					workerFailed = worker.Terminal && worker.StateEvidenceSource == runtimeobs.EvidenceHook && (worker.StateObservationQuality == runtimeobs.QualityObserved || worker.StateObservationQuality == runtimeobs.QualityAuthoritative) && (worker.CurrentState == runtimeobs.StateErrored || worker.CurrentState == runtimeobs.StateInterrupted || worker.CurrentState == runtimeobs.StateShutdown)
				}
			}
			signal["runtime_state"] = string(worker.CurrentState)
		} else if toString(signal["health"]) != "awaiting_finalize" && !(worker != nil && ambiguous && worker.CurrentState == runtimeobs.StateCompleted) {
			continue
		}
		readErr := handoffErr
		if ledgerErr != nil {
			readErr = ledgerErr
		}
		review := completionReview(row, worker, assignmentID, completedAt, handoffs, readErr, ledger.Attempts, now)
		if ambiguous {
			review["reason"], review["observation_quality"], review["action"], review["action_required"], review["severity"] = "worker_attempt_ambiguous", "conflicting", "inspect_worker_attempts", true, "warning"
			review["diagnostic"] = "동일 작업에 연결된 Worker attempt가 모호해 정확한 실행을 선택할 수 없습니다."
		}
		if workerFailed {
			review["state"], review["reason"], review["observation_quality"], review["action"], review["action_required"], review["severity"], review["confirmed_failure"] = "controller_recovery", "worker_execution_failed", "confirmed", "investigate_worker_execution", true, "error", true
			review["diagnostic"] = "Worker 실행의 오류 또는 중단이 Hook 상태로 관측되었습니다."
		}
		if assignmentErr != nil {
			review["reason"] = "assignment_evidence_unavailable"
			review["observation_quality"] = "insufficient"
			review["action"] = "inspect_assignment_evidence"
			review["action_required"] = true
			review["severity"] = "warning"
		}
		signal["completion_review"] = review
		signal["health"] = review["state"]
	}
}

func applyCompletionReviewItem(item map[string]any, signal map[string]any) {
	if toString(item["file_state"]) != "doing" || toString(item["state"]) == "needs_user" || signal == nil {
		return
	}
	health := toString(signal["health"])
	switch health {
	case "controller_review_pending", "controller_review", "controller_finalizing", "controller_recovery":
		item["state"] = health
		item["completion_review"] = signal["completion_review"]
	}
}

func completionUserWaitMessage(signal map[string]any) string {
	if review, ok := signal["completion_review"].(map[string]any); ok && review["state"] == "needs_user" {
		return toString(review["diagnostic"])
	}
	return "워커 런타임이 사용자 입력 또는 조치를 기다리고 있습니다."
}

// CompletionReviewStates shares exact-ledger review decisions with operation monitoring.
func CompletionReviewStates(project, root string, now time.Time) map[string]map[string]any {
	rows, err := CachedCatalog(project, root)
	if err != nil {
		return nil
	}
	timings, _ := lifecycleTimings(project, root, rows)
	activity := runtimeActivity(project, rows, timings)
	activity = mergeRuntimeSignals(activity, runtimeLedgerSignals(project, rows, now))
	applyCompletionReviews(project, rows, activity, now)
	out := map[string]map[string]any{}
	for id, signal := range activity {
		if r, ok := signal["completion_review"].(map[string]any); ok {
			out[id] = r
		}
	}
	return out
}
