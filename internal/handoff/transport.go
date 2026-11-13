package handoff

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/contractscope"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// AddressKind describes a tool's addressing namespace, never a display name.
type AddressKind string

const (
	NativeSessionID     AddressKind = "native_session_id"
	T3ThreadID          AddressKind = "t3_thread_id"
	CollaborationHandle AddressKind = "collaboration_agent_handle"
)

type TransportAddress struct {
	Kind  AddressKind `json:"kind"`
	Value string      `json:"value"`
}

// ValidateAddress deliberately has no UUID/name/semantic-role fallback. A
// collaboration handle must come from that tool's creation/discovery receipt;
// a native runtime ID is not such a receipt, even when it looks like a handle.
func ValidateAddress(tool string, address TransportAddress) error {
	want := T3ThreadID
	switch tool {
	case "t3_thread_send":
	case "collaboration.send_message", "collaboration.followup_task":
		want = CollaborationHandle
	default:
		return fmt.Errorf("unsupported transport tool: %s", tool)
	}
	if address.Kind != want || strings.TrimSpace(address.Value) == "" {
		return fmt.Errorf("%s requires %s, got %s", tool, want, address.Kind)
	}
	return nil
}

// TransportPlan is advisory. It cannot prove delivery, creation, START, claim or
// applied. The tool invocation and its actual receipt remain a separate step.
type TransportPlan struct {
	HandoffID              string           `json:"handoff_id"`
	EventType              EventType        `json:"event_type"`
	T3RunID                string           `json:"t3_run_id,omitempty"`
	Target                 Target           `json:"target"`
	Tool                   string           `json:"tool,omitempty"`
	Address                TransportAddress `json:"address"`
	Mode                   string           `json:"mode,omitempty"`
	RequiresFreshPreflight bool             `json:"requires_fresh_preflight"`
	Supported              bool             `json:"supported"`
	Reason                 string           `json:"reason"`
	ActionOwner            string           `json:"action_owner"`
	ResumeCondition        string           `json:"resume_condition"`
}

// PlanTransport reads the existing handoff and revalidates its exact current
// episode through the inherited A72 T3 adapter. Caller JSON cannot supply a
// successful native observation or enable an unsupported runtime.
func PlanTransport(project, id string, now time.Time) (TransportPlan, error) {
	row, ok, err := Inspect(project, id, now)
	if err != nil {
		return TransportPlan{}, err
	}
	if !ok {
		return TransportPlan{}, fmt.Errorf("handoff not found: %s", id)
	}
	p := TransportPlan{HandoffID: id, EventType: row.EventType, Target: row.Target, Reason: "exact_tool_address_unavailable", ActionOwner: row.SourceAgentPath,
		ResumeCondition: "Observe exact current target; obtain a supported tool-specific address and retry this same report/handoff; do not create another agent."}
	if row.Applied {
		p.Reason = "already_applied"
		return p, nil
	}
	if row.ClaimedBy != "" {
		p.Reason = "already_claimed_continue_existing_recovery"
		p.ActionOwner = row.ClaimedBy
		return p, nil
	}
	if row.Action == ActionHold || row.Action == ActionReportOnly {
		p.Reason = "handoff_" + string(row.Action)
		return p, nil
	}
	inspection, err := backlog.Inspect(project, "", row.TaskID)
	if err != nil {
		return p, err
	}
	raw, _ := inspection["raw_markdown"].(string)
	if matches, _ := contractscope.MatchesFingerprint(raw, row.ContractSHA256); inspection["exists"] != true || !matches {
		p.Reason = "contract_changed_or_missing"
		return p, nil
	}
	ledger, err := runtimeobs.BuildLedger(project, 20, now)
	if err != nil {
		return p, err
	}
	if row.EventType == EventWorkerDone || row.EventType == EventWorkerBlocked {
		started := false
		for _, a := range ledger.Attempts {
			if a.AttemptID == row.SourceAttemptID && a.TaskID == row.TaskID && a.AgentPath == row.SourceAgentPath && a.BindingState == runtimeobs.BindingBound && a.StartedAt != "" && a.StateObservationQuality == runtimeobs.QualityObserved && (a.StateEvidenceSource == runtimeobs.EvidenceT3 || a.StateEvidenceSource == runtimeobs.EvidenceHook) {
				assignmentID := a.BindingEvidence["assignment_id"]
				if assignmentID != "" {
					assignment, e := runtimeobs.FindAssignment(project, assignmentID)
					started = e == nil && assignment.TaskID == row.TaskID && assignment.AgentPath == row.SourceAgentPath
				}
			}
		}
		if !started {
			p.Reason = "source_start_not_observed"
			return p, nil
		}
	}
	for _, a := range ledger.Attempts {
		if a.AttemptID != row.Target.AttemptID {
			continue
		}
		if a.BindingState != runtimeobs.BindingBound || a.AgentPath != row.Target.AgentPath || a.SessionID != row.Target.SessionID || a.RuntimeAgentID != row.Target.RuntimeAgentID {
			p.Reason = "current_attempt_identity_mismatch"
			return p, nil
		}
		if a.StateEvidenceSource != runtimeobs.EvidenceT3 || a.NativeReceipt == nil {
			p.Reason = "collaboration_handle_not_observed"
			return p, nil
		}
		if err := runtimeobs.VerifyT3Attempt(a, now); err != nil {
			p.Reason = err.Error()
			return p, nil
		}
		action, _, reason, err := actionFor(project, a, row.Target.AgentPath)
		if err != nil {
			return p, err
		}
		if action != ActionMessageRunning && action != ActionResumeCompleted {
			p.Reason = reason
			return p, nil
		}
		return planT3Transport(p, a)
	}
	p.Reason = "target_attempt_not_found"
	return p, nil
}

func planT3Transport(p TransportPlan, a runtimeobs.Attempt) (TransportPlan, error) {
	if a.NativeReceipt == nil || a.NativeReceipt.ThreadID == "" || a.NativeReceipt.ThreadID == a.SessionID || a.RuntimeAgentID == "" {
		return p, errors.New("distinct observed native/T3 identities required")
	}
	switch a.CurrentState {
	case runtimeobs.StateRunning, runtimeobs.StateStarting:
	case runtimeobs.StateCompleted:
		p.RequiresFreshPreflight = true
	default:
		p.Reason = "target_not_messageable_or_resumable"
		return p, nil
	}
	p.Tool = "t3_thread_send"
	p.T3RunID = a.NativeReceipt.RunID
	p.Address = TransportAddress{T3ThreadID, a.NativeReceipt.ThreadID}
	p.Mode = "auto"
	p.Supported = true
	p.Reason = "exact_current_t3_route; transport_ack_is_not_claim_or_applied"
	return p, ValidateAddress(p.Tool, p.Address)
}
