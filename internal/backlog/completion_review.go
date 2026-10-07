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
	prepared   completionHandoffRecord
	claim      completionHandoffRecord
	acceptance completionHandoffRecord
	applied    completionHandoffRecord
	failed     bool
	steps      map[string]string
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

func completionReview(row Record, worker *runtimeobs.Attempt, completedAt string, handoffs map[string]*completionHandoff, readErr error, attempts []runtimeobs.Attempt, now time.Time) map[string]any {
	state, since, diagnostic := "controller_review_pending", completedAt, "Controller 인계 확인을 기다리고 있습니다."
	grace := 5 * time.Minute
	handoffID := ""
	if readErr != nil && !os.IsNotExist(readErr) {
		state = "controller_recovery"
		diagnostic = "인계 근거를 읽을 수 없습니다: " + readErr.Error()
	}
	var selected *completionHandoff
	if worker != nil {
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
		if selected.failed || p.Action == "hold" {
			state = "controller_recovery"
			diagnostic = "Controller 인계 또는 검토 단계가 실패하거나 확인되지 않았습니다."
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
				diagnostic = "인계를 받은 Controller의 정확한 실행 근거를 확인할 수 없습니다."
			} else {
				state, since, grace, diagnostic = "controller_review", c.At, 15*time.Minute, "Controller가 결과와 수용 기준을 검토하고 있습니다."
				if controller.CurrentState == runtimeobs.StateWaitingUser || controller.CurrentState == runtimeobs.StateWaitingApproval {
					state, diagnostic = "needs_user", "Controller가 명시적으로 사용자 판단 또는 승인을 기다리고 있습니다."
				} else if controller.CurrentState != runtimeobs.StateRunning || controller.Terminal {
					state = "controller_recovery"
					diagnostic = "완료 검토를 맡은 Controller 실행이 중단되었거나 종료되었습니다."
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
	deadline := ""
	if !ok || at.After(now) {
		state = "controller_recovery"
		diagnostic = "완료 보고의 원천 시각을 확인할 수 없습니다."
	} else {
		deadline = at.Add(grace).Format(time.RFC3339Nano)
		if !now.Before(at.Add(grace)) && state != "controller_recovery" && state != "needs_user" {
			state = "controller_recovery"
			diagnostic = "Controller 인계·검토의 정상 대기 시간을 초과했습니다. Controller가 실행 근거를 확인하고 복구해야 합니다."
		}
	}
	return map[string]any{"state": state, "since": since, "grace_until": deadline, "diagnostic": diagnostic, "handoff_id": handoffID, "audience": "controller"}
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

func applyCompletionReviews(project string, rows []Record, activity map[string]map[string]any, now time.Time) {
	ledger, ledgerErr := runtimeobs.BuildLedger(project, 10, now)
	handoffs, handoffErr := readCompletionHandoffs(project)
	for _, row := range rows {
		if row.Location != "active" || row.State != "doing" {
			continue
		}
		signal := activity[row.ID]
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
		var workerErr error
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
					workerErr = fmt.Errorf("완료 보고 이후 Worker 실행이 오류 또는 중단 상태입니다: %s", worker.CurrentState)
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
		if workerErr != nil {
			readErr = workerErr
		}
		if ambiguous {
			readErr = fmt.Errorf("Worker execution identity is ambiguous")
		}
		review := completionReview(row, worker, completedAt, handoffs, readErr, ledger.Attempts, now)
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
