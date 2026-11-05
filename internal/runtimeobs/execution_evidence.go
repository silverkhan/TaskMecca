package runtimeobs

import "time"

// HasObservedExecution verifies the provenance behind the folded state. A
// reconciled state is evidence only when an exact binding and real Hook activity
// were folded into this same attempt. Merely naming a source is insufficient.
func HasObservedExecution(a Attempt) bool {
	if a.StateObservationQuality != QualityObserved && a.StateObservationQuality != QualityAuthoritative {
		return false
	}
	if a.StateEvidenceSource == EvidenceHook {
		return true
	}
	if a.StateEvidenceSource != EvidenceReconciled || a.BindingState != BindingBound || a.Terminal {
		return false
	}
	if a.AttemptID == "" || a.RuntimeAgentID == "" || a.SessionID == "" || a.Provider == "" {
		return false
	}
	if a.BindingSource != "explicit" && a.BindingSource != "dispatch" {
		return false
	}
	if id := a.BindingEvidence["runtime_agent_id"]; id != "" && id != a.RuntimeAgentID {
		return false
	}
	binding, err := time.Parse(time.RFC3339Nano, a.BindingAt)
	if err != nil {
		return false
	}
	hook, err := time.Parse(time.RFC3339Nano, a.LastHookActivityAt)
	if err != nil || a.HookActivityCount < 1 {
		return false
	}
	latest, err := time.Parse(time.RFC3339Nano, a.LastObservedAt)
	if err != nil {
		return false
	}
	started, err := time.Parse(time.RFC3339Nano, a.StartedAt)
	if err != nil {
		return false
	}
	return !hook.After(latest) && !binding.After(latest) && !started.Before(binding) && !started.After(latest)
}
