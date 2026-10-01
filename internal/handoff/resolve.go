package handoff

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func resolveTarget(project, targetPath, sourceAttemptID, targetAttemptID string, now time.Time) (Target, Action, bool, string, error) {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return Target{}, ActionHold, false, "target_path_required", errors.New("target agent path is required")
	}
	ledger, err := runtimeobs.BuildLedger(project, 20, now)
	if err != nil {
		return Target{}, ActionHold, false, "runtime_ledger_error", err
	}

	var sourceSession string
	if strings.TrimSpace(sourceAttemptID) != "" {
		for _, attempt := range ledger.Attempts {
			if attempt.AttemptID == sourceAttemptID {
				sourceSession = attempt.SessionID
				break
			}
		}
	}

	var chosen *runtimeobs.Attempt
	if strings.TrimSpace(targetAttemptID) != "" {
		for i := range ledger.Attempts {
			attempt := &ledger.Attempts[i]
			if attempt.AttemptID != targetAttemptID {
				continue
			}
			if attempt.BindingState != runtimeobs.BindingBound {
				return Target{}, ActionHold, false, "target_not_bound", fmt.Errorf("target attempt %s is not exactly bound", targetAttemptID)
			}
			if attempt.AgentPath != targetPath {
				return Target{}, ActionHold, false, "target_path_mismatch", fmt.Errorf("target attempt %s is bound to %s, not %s", targetAttemptID, attempt.AgentPath, targetPath)
			}
			chosen = attempt
			break
		}
	}

	if chosen == nil {
		candidates := []runtimeobs.Attempt{}
		for _, attempt := range ledger.Attempts {
			if attempt.BindingState != runtimeobs.BindingBound || attempt.AgentPath != targetPath {
				continue
			}
			if sourceSession != "" && attempt.SessionID != sourceSession {
				continue
			}
			candidates = append(candidates, attempt)
		}
		if len(candidates) == 0 {
			return Target{AgentPath: targetPath}, ActionHold, false, "target_missing", nil
		}
		if len(candidates) > 1 {
			return Target{AgentPath: targetPath}, ActionHold, false, "target_ambiguous", nil
		}
		chosen = &candidates[0]
	}

	target := Target{
		AgentPath: chosen.AgentPath, AttemptID: chosen.AttemptID,
		RuntimeAgentID: chosen.RuntimeAgentID, SessionID: chosen.SessionID,
		Provider: chosen.Provider, State: string(chosen.CurrentState),
	}
	action, fresh, reason := actionFor(*chosen, targetPath)
	return target, action, fresh, reason, nil
}

func actionFor(attempt runtimeobs.Attempt, targetPath string) (Action, bool, string) {
	provider := strings.ToLower(strings.TrimSpace(attempt.Provider))
	state := attempt.CurrentState
	switch state {
	case runtimeobs.StateStarting, runtimeobs.StateRunning:
		return ActionMessageRunning, false, "target_running"
	case runtimeobs.StateCompleted:
		if provider == "codex" && targetPath == "/root" {
			return ActionReportOnly, false, "codex_root_cannot_be_resumed"
		}
		if provider == "codex" || provider == "claude" {
			return ActionResumeCompleted, true, "completed_target_is_resumable"
		}
		return ActionHold, false, "provider_resume_capability_unknown"
	case runtimeobs.StateInterrupted:
		return ActionHold, false, "target_interrupted_or_cancelled"
	case runtimeobs.StateErrored:
		return ActionHold, false, "target_errored"
	case runtimeobs.StateShutdown:
		return ActionHold, false, "target_shutdown"
	case runtimeobs.StateWaitingUser, runtimeobs.StateWaitingApproval:
		return ActionHold, false, "target_waiting"
	default:
		return ActionHold, false, "target_state_unknown"
	}
}
