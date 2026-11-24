package activity

import (
	"fmt"
	"sort"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// Session identity is a read projection. Names and logical paths never establish
// identity, and this projection cannot bind, assign or authorize an actor.
type WorkflowSessionIdentity struct {
	Status   string                   `json:"status"`
	Provider string                   `json:"provider,omitempty"`
	NativeID string                   `json:"native_id,omitempty"`
	Aliases  []string                 `json:"aliases"`
	Attempts []WorkflowSessionAttempt `json:"attempts"`
	Reason   string                   `json:"reason,omitempty"`
}

type WorkflowSessionAttempt struct {
	Rounds         []WorkflowRoundScope `json:"rounds,omitempty"`
	AttemptID      string               `json:"attempt_id"`
	AssignmentID   string               `json:"assignment_id"`
	AgentPath      string               `json:"agent_path"`
	RuntimeAgentID string               `json:"runtime_agent_id"`
	ThreadID       string               `json:"thread_id,omitempty"`
	Source         string               `json:"source"`
	BindingSource  string               `json:"binding_source,omitempty"`
	HandoffIDs     []string             `json:"handoff_ids,omitempty"`
	StartedAt      string               `json:"started_at,omitempty"`
	ObservedAt     string               `json:"observed_at,omitempty"`
	CompletedAt    string               `json:"completed_at,omitempty"`
}

func workflowSessionRows(in WorkflowInput, rows map[string]*WorkflowRow) map[string]*WorkflowRow {
	attempts := map[string]runtimeobs.Attempt{}
	counts := map[string]int{}
	for _, a := range in.ExecutionObservations {
		counts[a.AttemptID]++
	}
	for _, a := range in.ExecutionObservations {
		role := workflowActorRole(a.AgentPath)
		if counts[a.AttemptID] != 1 || a.AttemptID == "" || a.BindingState != runtimeobs.BindingBound || a.RuntimeAgentID == "" || a.SessionID == "" || a.Provider == "" || role == "unknown" || !runtimeobs.HasObservedExecution(a) {
			continue
		}
		if a.TaskID != in.TaskID && !(a.TaskID == "" && role == "controller" && workflowSessionClaim(in, a, "")) {
			continue
		}
		if native := a.BindingEvidence["runtime_agent_id"]; native != "" && native != a.RuntimeAgentID {
			continue
		}
		if a.StateEvidenceSource == runtimeobs.EvidenceT3 && (a.NativeReceipt.ThreadID == "" || a.NativeReceipt.RunID == "" || a.NativeReceipt.ThreadID == a.SessionID) {
			continue
		}
		conflict := false
		for _, e := range in.Events {
			if e.TaskID != in.TaskID || e.RuntimeAttemptID != a.AttemptID {
				continue
			}
			if string(e.Role) != role || (e.AgentID != a.RuntimeAgentID && e.AgentID != a.AgentPath) || (e.SessionID != "" && e.SessionID != a.SessionID) || (e.Provider != "" && e.Provider != a.Provider) || !workflowSessionAssociation(in, a, e.AssignmentID, e.HandoffID) {
				conflict = true
			}
		}
		if !conflict {
			attempts[a.AttemptID] = a
		}
	}
	result := map[string]*WorkflowRow{}
	resolve := func(row *WorkflowRow, attempt, assignment, handoff string) *WorkflowRow {
		a, ok := attempts[attempt]
		ok = ok && row.Role == workflowActorRole(a.AgentPath) && (row.AgentID == a.AgentPath || row.AgentID == a.RuntimeAgentID) && workflowSessionAssociation(in, a, assignment, handoff)
		key := row.ID
		if ok {
			key = fmt.Sprintf("%s|session:%s:%s", row.Role, a.Provider, a.SessionID)
		}
		out := result[key]
		if out == nil {
			out = &WorkflowRow{ID: key, Role: row.Role, AgentID: row.AgentID, Round: row.Round, RoundStatus: row.RoundStatus, Intervals: []WorkflowInterval{}, RoleFacts: []WorkflowRoleFact{}, SessionIdentity: &WorkflowSessionIdentity{Status: "unverified", Aliases: []string{}, Attempts: []WorkflowSessionAttempt{}, Reason: "Exact task-bound observed attempt, assignment and actor identity are required; names and paths alone are insufficient"}}
			if ok {
				out.AgentID = a.RuntimeAgentID
				out.SessionIdentity.Status = "verified"
				out.SessionIdentity.Reason = ""
				out.SessionIdentity.Provider = a.Provider
				out.SessionIdentity.NativeID = a.SessionID
			}
			result[key] = out
		}
		out.Rounds = workflowAppendRound(out.Rounds, row.Round, row.RoundStatus)
		if len(out.Rounds) > 1 {
			out.Round = nil
			out.RoundStatus = "multiple"
		}
		identity := out.SessionIdentity
		identity.Aliases = appendUnique(identity.Aliases, row.AgentID)
		if ok {
			found := false
			for i, e := range identity.Attempts {
				if e.AttemptID == attempt {
					identity.Attempts[i].Rounds = workflowAppendRound(identity.Attempts[i].Rounds, row.Round, row.RoundStatus)
					found = true
				}
			}
			if !found {
				e := WorkflowSessionAttempt{Rounds: workflowAppendRound(nil, row.Round, row.RoundStatus), AttemptID: attempt, AssignmentID: a.BindingEvidence["assignment_id"], AgentPath: a.AgentPath, RuntimeAgentID: a.RuntimeAgentID, Source: string(a.StateEvidenceSource), BindingSource: a.BindingSource, StartedAt: a.StartedAt, ObservedAt: a.LastObservedAt, CompletedAt: a.EndedAt}
				for id, h := range in.Handoffs {
					if workflowSessionClaim(in, a, id) {
						e.HandoffIDs = append(e.HandoffIDs, h.HandoffID)
					}
				}
				sort.Strings(e.HandoffIDs)
				if a.NativeReceipt != nil {
					e.ThreadID = a.NativeReceipt.ThreadID
					e.StartedAt = a.NativeReceipt.StartedAt
					e.CompletedAt = a.NativeReceipt.CompletedAt
				}
				identity.Attempts = append(identity.Attempts, e)
			}
		}
		return out
	}
	for _, row := range rows {
		if len(row.Intervals)+len(row.RoleFacts) == 0 {
			resolve(row, "", "", "")
		}
		for _, v := range row.Intervals {
			out := resolve(row, v.AttemptID, v.AssignmentID, v.HandoffID)
			v.Round = workflowRoundCopy(row.Round)
			v.RoundStatus = row.RoundStatus
			out.Intervals = append(out.Intervals, v)
			if v.Measured && v.DurationMillis != nil {
				if out.ObservedActivityMillis == nil {
					ms := int64(0)
					out.ObservedActivityMillis = &ms
				}
				*out.ObservedActivityMillis += *v.DurationMillis
			}
		}
		for _, f := range row.RoleFacts {
			out := resolve(row, f.AttemptID, f.AssignmentID, f.HandoffID)
			f.Round = workflowRoundCopy(row.Round)
			f.RoundStatus = row.RoundStatus
			out.RoleFacts = append(out.RoleFacts, f)
		}
	}
	for _, row := range result {
		sort.Strings(row.SessionIdentity.Aliases)
		sort.Slice(row.SessionIdentity.Attempts, func(i, j int) bool {
			return row.SessionIdentity.Attempts[i].AttemptID < row.SessionIdentity.Attempts[j].AttemptID
		})
		sort.SliceStable(row.RoleFacts, func(i, j int) bool {
			if row.RoleFacts[i].At.Equal(row.RoleFacts[j].At) {
				return row.RoleFacts[i].ID < row.RoleFacts[j].ID
			}
			return row.RoleFacts[i].At.Before(row.RoleFacts[j].At)
		})
		sort.SliceStable(row.Intervals, func(i, j int) bool {
			if row.Intervals[i].Start.Equal(row.Intervals[j].Start) {
				return row.Intervals[i].ID < row.Intervals[j].ID
			}
			return row.Intervals[i].Start.Before(row.Intervals[j].Start)
		})
	}
	return result
}

func roundKey(n *int) string {
	if n == nil {
		return "unknown"
	}
	return fmt.Sprint(*n)
}

// Claim evidence is task-specific. A bare role binding never makes a taskless
// Controller belong to every task; even a real claim for another task fails.
func workflowSessionClaim(in WorkflowInput, a runtimeobs.Attempt, id string) bool {
	for key, h := range in.Handoffs {
		if id != "" && key != id && h.HandoffID != id {
			continue
		}
		if h.TaskID != in.TaskID || h.ClaimedBy != a.AgentPath || h.ClaimedAttemptID != a.AttemptID || h.Target.AgentPath != a.AgentPath || h.Target.RuntimeAgentID != a.RuntimeAgentID || h.Target.SessionID != a.SessionID || h.Target.Provider != a.Provider || (h.Target.AttemptID != "" && h.Target.AttemptID != a.AttemptID) {
			continue
		}
		if at, ok := workflowTime(h.ClaimedAt); ok && (in.Now.IsZero() || !at.After(in.Now)) {
			return true
		}
	}
	return false
}

func workflowSessionAssociation(in WorkflowInput, a runtimeobs.Attempt, assignment, handoff string) bool {
	role := workflowActorRole(a.AgentPath)
	if role == "controller" {
		if handoff == "" && assignment != "" {
			handoff = assignment
		}
		return workflowSessionClaim(in, a, handoff)
	}
	if a.TaskID != in.TaskID {
		return false
	}
	expected := a.BindingEvidence["assignment_id"]
	if expected != "" {
		return assignment == "" || assignment == expected
	}
	if assignment != "" || handoff != "" || (role != "registrar" && role != "sub_controller") {
		return false
	}
	// Supported Registrar/sub-Controller task binding has no Worker assignment.
	if a.BindingSource != "explicit" && a.BindingSource != "dispatch" {
		return false
	}
	at, ok := workflowTime(a.BindingAt)
	return ok && (in.Now.IsZero() || !at.After(in.Now))
}

func workflowRoundCopy(n *int) *int {
	if n == nil {
		return nil
	}
	copy := *n
	return &copy
}
func workflowAppendRound(scopes []WorkflowRoundScope, n *int, status string) []WorkflowRoundScope {
	for _, s := range scopes {
		if roundKey(s.Round) == roundKey(n) {
			return scopes
		}
	}
	if status == "" {
		status = roundStatus(n)
	}
	scopes = append(scopes, WorkflowRoundScope{Round: workflowRoundCopy(n), Status: status})
	sort.Slice(scopes, func(i, j int) bool {
		if scopes[i].Round == nil {
			return scopes[j].Round != nil
		}
		if scopes[j].Round == nil {
			return false
		}
		return *scopes[i].Round < *scopes[j].Round
	})
	return scopes
}
