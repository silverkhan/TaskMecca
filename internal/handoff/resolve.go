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
	action, fresh, reason, capabilityErr := actionFor(project, *chosen, targetPath)
	if capabilityErr != nil {
		return target, ActionHold, false, "capability_evidence_error", capabilityErr
	}
	return target, action, fresh, reason, nil
}

func actionFor(project string, attempt runtimeobs.Attempt, targetPath string) (Action, bool, string, error) {
	provider := strings.ToLower(strings.TrimSpace(attempt.Provider))
	caps, err := LoadCapabilities(project, provider)
	if err != nil {
		return ActionHold, false, "capability_evidence_error", err
	}
	state := attempt.CurrentState
	switch state {
	case runtimeobs.StateStarting, runtimeobs.StateRunning:
		if caps.CanMessageRunning == CapabilitySupported {
			return ActionMessageRunning, false, "target_running", nil
		}
		return ActionHold, false, "message_running_capability_" + string(caps.CanMessageRunning), nil
	case runtimeobs.StateCompleted:
		agentType:=strings.ToLower(strings.TrimSpace(attempt.AgentType))
		if provider=="claude" && (agentType=="explore" || agentType=="plan") {
			return ActionHold, false, "claude_one_shot_agent_not_resumable", nil
		}
		if targetPath == "/root" {
			switch caps.CanResumeRoot {
			case CapabilitySupported:
				return ActionResumeCompleted, true, "root_resume_capability_supported", nil
			case CapabilityUnsupported:
				return ActionReportOnly, false, "root_resume_capability_unsupported", nil
			default:
				return ActionReportOnly, false, "root_resume_capability_unknown", nil
			}
		}
		if caps.CanResumeCompleted == CapabilitySupported {
			return ActionResumeCompleted, true, "completed_target_is_resumable", nil
		}
		return ActionHold, false, "resume_completed_capability_" + string(caps.CanResumeCompleted), nil
	case runtimeobs.StateInterrupted:
		return ActionHold, false, "target_interrupted_or_cancelled", nil
	case runtimeobs.StateErrored:
		return ActionHold, false, "target_errored", nil
	case runtimeobs.StateShutdown:
		return ActionHold, false, "target_shutdown", nil
	case runtimeobs.StateWaitingUser, runtimeobs.StateWaitingApproval:
		return ActionHold, false, "target_waiting", nil
	default:
		return ActionHold, false, "target_state_unknown", nil
	}
}
