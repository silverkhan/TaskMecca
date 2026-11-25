package activity

import (
	"fmt"
	"sort"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
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
	PreviousClaim  *WorkflowPreviousClaimEvidence `json:"previous_claim,omitempty"`
	Rounds         []WorkflowRoundScope           `json:"rounds,omitempty"`
	AttemptID      string                         `json:"attempt_id"`
	AssignmentID   string                         `json:"assignment_id"`
	AgentPath      string                         `json:"agent_path"`
	RuntimeAgentID string                         `json:"runtime_agent_id"`
	ThreadID       string                         `json:"thread_id,omitempty"`
	Source         string                         `json:"source"`
	BindingSource  string                         `json:"binding_source,omitempty"`
	HandoffIDs     []string                       `json:"handoff_ids,omitempty"`
	StartedAt      string                         `json:"started_at,omitempty"`
	ObservedAt     string                         `json:"observed_at,omitempty"`
	CompletedAt    string                         `json:"completed_at,omitempty"`
}

type WorkflowPreviousClaimEvidence struct {
	ProjectVerifiedAt *time.Time `json:"project_verified_at,omitempty"`
	ProjectProofBasis string     `json:"project_proof_basis,omitempty"`
	HandoffID         string     `json:"handoff_id"`
	CurrentAttemptID  string     `json:"current_attempt_id"`
	NativeCompletedAt time.Time  `json:"native_completed_at"`
	TransferredAt     time.Time  `json:"transferred_at"`
	Basis             string     `json:"basis"`
	Limitation        string     `json:"limitation"`
	ChainTransfers    int        `json:"chain_transfers,omitempty"`
	ClaimWindowStart  *time.Time `json:"claim_window_start,omitempty"`
}

// A missing historical project field needs a separate exact local host proof.
// This is display input only; it never modifies the persisted receipt or gate.
func workflowClaimProject(in WorkflowInput, a runtimeobs.Attempt) string {
	if a.NativeReceipt == nil {
		return ""
	}
	r := a.NativeReceipt
	if r.CanonicalProject != "" {
		return r.CanonicalProject
	}
	p, ok := in.NativeProjectReceipts[a.AttemptID]
	seen, valid := workflowTime(p.ObservedAt)
	if !ok || in.SourceProject == "" || in.OperationalControllerNative != a.RuntimeAgentID || a.TaskID != "" || a.AgentPath != "/root/controller" || a.BindingState != runtimeobs.BindingBound || p.CanonicalProject != in.SourceProject || p.Origin != "t3_local_projection_v2" || !valid || (!in.Now.IsZero() && seen.After(in.Now)) || p.Provider != a.Provider || p.Provider != r.Provider || p.SessionID != a.RuntimeAgentID || p.SessionID != a.SessionID || p.SessionID != r.SessionID || p.ThreadID != r.ThreadID || p.RunID != r.RunID || p.NativeAttemptID != r.NativeAttemptID || p.State != r.State || p.StartedAt != r.StartedAt || p.CompletedAt != r.CompletedAt {
		return ""
	}
	return p.CanonicalProject
}

func workflowSessionBindingTime(a runtimeobs.Attempt) time.Time {
	bound, _ := workflowTime(a.BindingAt)
	if a.NativeReceipt != nil {
		if start, ok := workflowTime(a.NativeReceipt.StartedAt); ok && bound.Before(start) {
			bound = start
		}
	}
	return bound
}

func workflowPreviousClaimTime(a runtimeobs.Attempt) time.Time {
	if a.NativeReceipt != nil {
		if end, ok := workflowTime(a.NativeReceipt.CompletedAt); ok {
			return end
		}
	}
	return workflowSessionBindingTime(a)
}

type workflowClaimWindow struct {
	view  handoffjournal.Handoff
	start time.Time
}

// Reconstruct only a complete, monotone Controller claim chain whose final fold
// exactly matches BuildLedger. Raw records alone are never accepted authority.
func workflowClaimWindows(in WorkflowInput, final handoffjournal.Handoff) ([]workflowClaimWindow, bool) {
	if len(in.HandoffRecords) == 0 || final.HandoffID == "" || final.TaskID != in.TaskID || final.ContractSHA256 == "" || final.ClaimedBy != "/root/controller" {
		return nil, false
	}
	var target handoffjournal.Target
	var previous time.Time
	var claimStart time.Time
	claimed, prepared, records := "", false, 0
	var windows []workflowClaimWindow
	for _, r := range in.HandoffRecords {
		if r.HandoffID != final.HandoffID {
			continue
		}
		records++
		if r.TaskID != "" && r.TaskID != final.TaskID || r.ContractSHA256 != "" && r.ContractSHA256 != final.ContractSHA256 {
			return nil, false
		}
		if r.RecordKind != "prepared" && r.RecordKind != "retargeted" && r.RecordKind != "claimed" && r.RecordKind != "claim_transferred" {
			continue
		}
		at, ok := workflowTime(r.ObservedAt)
		if !ok || (!previous.IsZero() && at.Before(previous)) || (!in.Now.IsZero() && at.After(in.Now)) {
			return nil, false
		}
		previous = at
		switch r.RecordKind {
		case "prepared", "retargeted":
			if claimed != "" || (r.RecordKind == "prepared" && prepared) || (r.RecordKind == "retargeted" && !prepared) {
				return nil, false
			}
			if r.RecordKind == "prepared" {
				if r.TaskID != final.TaskID || r.ContractSHA256 != final.ContractSHA256 {
					return nil, false
				}
				prepared = true
			}
			target = handoffjournal.Target{AgentPath: r.TargetAgentPath, AttemptID: r.TargetAttemptID, RuntimeAgentID: r.TargetRuntimeAgentID, SessionID: r.TargetSessionID, Provider: r.Provider}
		case "claimed":
			if !prepared || claimed != "" || r.ClaimedBy != "/root/controller" || r.ClaimedAttemptID == "" || r.ClaimedAttemptID != target.AttemptID || target.AgentPath != "/root/controller" || target.Provider != final.Target.Provider || target.SessionID != final.Target.SessionID || target.RuntimeAgentID != final.Target.RuntimeAgentID {
				return nil, false
			}
			if first, ok := workflowTime(final.ClaimedAt); !ok || !first.Equal(at) {
				return nil, false
			}
			claimed = r.ClaimedAttemptID
			claimStart = at
		case "claim_transferred":
			if claimed == "" || r.PreviousClaimedAttemptID != claimed || r.ClaimedBy != "/root/controller" || r.ClaimedAttemptID == "" || r.ClaimedAttemptID == claimed || r.TargetAttemptID != r.ClaimedAttemptID || r.TargetAgentPath != "/root/controller" || r.Provider != target.Provider || r.TargetSessionID != target.SessionID || r.TargetRuntimeAgentID != target.RuntimeAgentID {
				return nil, false
			}
			view := final
			view.LastTransferFrom = claimed
			view.LastTransferredAt = r.ObservedAt
			view.ClaimedAttemptID = r.ClaimedAttemptID
			view.Target = handoffjournal.Target{AgentPath: r.TargetAgentPath, AttemptID: r.TargetAttemptID, RuntimeAgentID: r.TargetRuntimeAgentID, SessionID: r.TargetSessionID, Provider: r.Provider}
			oldCount := 0
			var old runtimeobs.Attempt
			for _, a := range in.ExecutionObservations {
				if a.AttemptID == claimed {
					old = a
					oldCount++
				}
			}
			if oldCount != 1 || old.NativeReceipt == nil {
				return nil, false
			}
			end, ok := workflowTime(old.NativeReceipt.CompletedAt)
			if !ok || end.Before(claimStart) {
				return nil, false
			}
			snapshot := in
			snapshot.Handoffs = map[string]handoffjournal.Handoff{view.HandoffID: view}
			snapshot.HandoffRecords = nil
			if workflowLastPreviousClaim(snapshot, old, old.BindingEvidence["assignment_id"], view.HandoffID, end, true) == nil {
				return nil, false
			}
			windows = append(windows, workflowClaimWindow{view: view, start: claimStart})
			claimed = r.ClaimedAttemptID
			target = view.Target
			claimStart = at
		}
	}
	if records != final.RecordCount || len(windows) != final.TransferCount || len(windows) == 0 || claimed != final.ClaimedAttemptID || target.AttemptID != final.Target.AttemptID || target.Provider != final.Target.Provider || target.SessionID != final.Target.SessionID || target.RuntimeAgentID != final.Target.RuntimeAgentID {
		return nil, false
	}
	last := windows[len(windows)-1].view
	if last.LastTransferFrom != final.LastTransferFrom || last.LastTransferredAt != final.LastTransferredAt {
		return nil, false
	}
	return windows, true
}

func workflowPreviousClaim(in WorkflowInput, a runtimeobs.Attempt, assignment, handoff string, at time.Time) *WorkflowPreviousClaimEvidence {
	var found *WorkflowPreviousClaimEvidence
	for key, h := range in.Handoffs {
		if handoff != "" && key != handoff && h.HandoffID != handoff {
			continue
		}
		windows, valid := workflowClaimWindows(in, h)
		if !valid {
			continue
		}
		for _, window := range windows {
			if window.view.LastTransferFrom != a.AttemptID || (a.TaskID == "" && at.Before(window.start)) {
				continue
			}
			snapshot := in
			snapshot.Handoffs = map[string]handoffjournal.Handoff{h.HandoffID: window.view}
			snapshot.HandoffRecords = nil
			proof := workflowLastPreviousClaim(snapshot, a, assignment, h.HandoffID, at, true)
			if proof == nil {
				continue
			}
			if found != nil {
				return nil
			}
			proof.Basis = "validated_append_only_claim_chain_same_verified_native"
			proof.CurrentAttemptID = h.ClaimedAttemptID
			proof.ChainTransfers = len(windows)
			if a.NativeReceipt.CanonicalProject == "" {
				proof.ProjectProofBasis = "separate_exact_readonly_host_project_receipt; persisted_receipt_unchanged"
				verifiedAt, _ := workflowTime(in.NativeProjectReceipts[a.AttemptID].ObservedAt)
				proof.ProjectVerifiedAt = &verifiedAt
			}
			start := window.start
			proof.ClaimWindowStart = &start
			proof.Limitation = "Read-only session identity from complete validated claim history; no current authorization, review permission or Activity duration is inferred"
			found = proof
		}
	}
	if found != nil {
		return found
	}
	return workflowLastPreviousClaim(in, a, assignment, handoff, at, false)
}

// A lifecycle point can carry its own exact Controller attempt/assignment even
// though it is not Activity and has no handoff field. Keep missing-author facts
// separate, and still require the exact task's claimed recipient identity.
func workflowControllerLifecycleFact(in WorkflowInput, a runtimeobs.Attempt, f WorkflowRoleFact) bool {
	if f.Source != "controller_report" || (f.Kind != "assigned" && f.Kind != "started" && f.Kind != "completed") || f.AttemptID == "" || f.AttemptID != a.AttemptID || f.AssignmentID == "" || f.AssignmentID != a.BindingEvidence["assignment_id"] || a.AgentPath != "/root/controller" || !workflowNativeSessionAssociation(in, a, f.AssignmentID, f.HandoffID) || !workflowSessionClaim(in, a, f.HandoffID) {
		return false
	}
	start, valid := workflowTime(a.NativeReceipt.StartedAt)
	if !valid || f.At.Before(start) || (!in.Now.IsZero() && f.At.After(in.Now)) {
		return false
	}
	if a.NativeReceipt.CompletedAt != "" {
		end, valid := workflowTime(a.NativeReceipt.CompletedAt)
		if !valid || f.At.After(end) {
			return false
		}
	}
	return true
}

// Display provenance only. This never makes an old claim current or authorizes
// Activity/review. The folded journal exposes only its last previous claimant;
// earlier transfers remain unknown rather than being inferred from native/name.
func workflowLastPreviousClaim(in WorkflowInput, a runtimeobs.Attempt, assignment, handoff string, at time.Time, globalHistory bool) *WorkflowPreviousClaimEvidence {
	global := globalHistory && in.OperationalControllerNative != "" && in.OperationalControllerNative == a.RuntimeAgentID && a.TaskID == "" && a.BindingEvidence["assignment_id"] == "" && assignment == ""
	if (a.TaskID != in.TaskID && !global) || a.AgentPath != "/root/controller" || a.BindingState != runtimeobs.BindingBound || !a.Terminal || a.StateEvidenceSource != runtimeobs.EvidenceT3 || !runtimeobs.HasObservedExecution(a) || a.NativeReceipt == nil || a.NativeReceipt.NativeAttemptID == "" || a.NativeReceipt.ThreadID == "" || a.NativeReceipt.RunID == "" || a.NativeReceipt.SessionID != a.SessionID || a.RuntimeAgentID != a.SessionID || (a.BindingEvidence["assignment_id"] == "" && !global) || (assignment != "" && assignment != a.BindingEvidence["assignment_id"]) {
		return nil
	}
	end, ok := workflowTime(a.NativeReceipt.CompletedAt)
	bound, boundOK := workflowTime(a.BindingAt)
	observed, observedOK := workflowTime(a.NativeReceipt.ObservedAt)
	started, startedOK := workflowTime(a.NativeReceipt.StartedAt)
	if bound.Before(started) {
		bound = started
	} // genuine continuing assignment may predate the new native episode
	terminal := a.CurrentState == runtimeobs.StateCompleted || a.CurrentState == runtimeobs.StateInterrupted || a.CurrentState == runtimeobs.StateErrored
	if !terminal || !ok || !boundOK || !observedOK || !startedOK || end.Before(bound) || observed.Before(end) || at.IsZero() || at.Before(bound) || at.After(end) || (!in.Now.IsZero() && (end.After(in.Now) || observed.After(in.Now))) {
		return nil
	}
	var proof *WorkflowPreviousClaimEvidence
	for key, h := range in.Handoffs {
		if handoff != "" && key != handoff && h.HandoffID != handoff {
			continue
		}
		transfer, valid := workflowTime(h.LastTransferredAt)
		if h.HandoffID == "" || h.TaskID != in.TaskID || h.LastTransferFrom != a.AttemptID || h.TransferCount < 1 || !valid || transfer.Before(end) || (!in.Now.IsZero() && transfer.After(in.Now)) || h.ClaimedBy != a.AgentPath || h.ClaimedAttemptID == "" || h.ClaimedAttemptID == a.AttemptID || h.Target.AttemptID != h.ClaimedAttemptID || h.Target.AgentPath != a.AgentPath || h.Target.Provider != a.Provider || h.Target.SessionID != a.SessionID || h.Target.RuntimeAgentID != a.RuntimeAgentID {
			continue
		}
		count := 0
		for _, current := range in.ExecutionObservations {
			if current.AttemptID != h.ClaimedAttemptID {
				continue
			}
			count++
			if current.TaskID != a.TaskID || current.AgentPath != a.AgentPath || current.BindingState != runtimeobs.BindingBound || current.BindingEvidence["assignment_id"] != a.BindingEvidence["assignment_id"] || (current.BindingEvidence["runtime_agent_id"] != "" && current.BindingEvidence["runtime_agent_id"] != a.RuntimeAgentID) || current.Provider != a.Provider || current.RuntimeAgentID != a.RuntimeAgentID || current.SessionID != a.SessionID || current.StateEvidenceSource != runtimeobs.EvidenceT3 || !runtimeobs.HasObservedExecution(current) || current.NativeReceipt == nil || current.NativeReceipt.NativeAttemptID == a.NativeReceipt.NativeAttemptID || workflowClaimProject(in, a) == "" || workflowClaimProject(in, current) != workflowClaimProject(in, a) || current.NativeReceipt.ThreadID != a.NativeReceipt.ThreadID || !workflowSessionClaim(in, current, h.HandoffID) {
				count += 2
				continue
			}
			start, valid := workflowTime(current.NativeReceipt.StartedAt)
			seen, seenOK := workflowTime(current.NativeReceipt.ObservedAt)
			if !valid || !seenOK || seen.Before(transfer) || (!in.Now.IsZero() && seen.After(in.Now)) || start.Before(end) || start.After(transfer) {
				count += 2
			}
			if current.NativeReceipt.CompletedAt != "" {
				finished, valid := workflowTime(current.NativeReceipt.CompletedAt)
				if !valid || finished.Before(transfer) || finished.After(seen) {
					count += 2
				}
			} else if current.Terminal {
				count += 2
			}
		}
		if count != 1 {
			continue
		}
		if proof != nil {
			return nil
		}
		proof = &WorkflowPreviousClaimEvidence{HandoffID: h.HandoffID, CurrentAttemptID: h.ClaimedAttemptID, NativeCompletedAt: end, TransferredAt: transfer, Basis: "exact_last_claim_transfer_same_verified_native", Limitation: "Display association for the last previous claimant only; no current authorization or earlier-history inference"}
	}
	return proof
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
		if a.TaskID != in.TaskID && !(a.TaskID == "" && role == "controller" && (workflowSessionClaim(in, a, "") || workflowPreviousClaim(in, a, "", "", workflowPreviousClaimTime(a)) != nil)) {
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
			if string(e.Role) != role || (e.AgentID != a.RuntimeAgentID && e.AgentID != a.AgentPath) || (e.SessionID != "" && e.SessionID != a.SessionID) || (e.Provider != "" && e.Provider != a.Provider) || (!workflowSessionAssociation(in, a, e.AssignmentID, e.HandoffID) && workflowPreviousClaim(in, a, e.AssignmentID, e.HandoffID, e.OccurredAt) == nil) {
				conflict = true
			}
		}
		if !conflict {
			attempts[a.AttemptID] = a
		}
	}
	result := map[string]*WorkflowRow{}
	resolve := func(row *WorkflowRow, attempt, assignment, handoff string, nativeFact bool, at time.Time) *WorkflowRow {
		a, ok := attempts[attempt]
		ok = ok && row.Role == workflowActorRole(a.AgentPath) && (row.AgentID == a.AgentPath || row.AgentID == a.RuntimeAgentID) && (workflowSessionAssociation(in, a, assignment, handoff) || nativeFact && workflowNativeSessionAssociation(in, a, assignment, handoff) || workflowPreviousClaim(in, a, assignment, handoff, at) != nil)
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
					if identity.Attempts[i].PreviousClaim == nil {
						identity.Attempts[i].PreviousClaim = workflowPreviousClaim(in, a, assignment, handoff, at)
					}
					found = true
				}
			}
			if !found {
				e := WorkflowSessionAttempt{Rounds: workflowAppendRound(nil, row.Round, row.RoundStatus), AttemptID: attempt, AssignmentID: a.BindingEvidence["assignment_id"], AgentPath: a.AgentPath, RuntimeAgentID: a.RuntimeAgentID, Source: string(a.StateEvidenceSource), BindingSource: a.BindingSource, StartedAt: a.StartedAt, ObservedAt: a.LastObservedAt, CompletedAt: a.EndedAt, PreviousClaim: workflowPreviousClaim(in, a, assignment, handoff, at)}
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
			resolve(row, "", "", "", false, time.Time{})
		}
		for _, v := range row.Intervals {
			out := resolve(row, v.AttemptID, v.AssignmentID, v.HandoffID, v.Kind == "native_observed" && !v.Measured && v.Evidence == string(runtimeobs.EvidenceT3), v.Start)
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
			out := resolve(row, f.AttemptID, f.AssignmentID, f.HandoffID, f.Kind == "observed_execution" && f.Source == string(runtimeobs.EvidenceT3) || workflowControllerLifecycleFact(in, attempts[f.AttemptID], f), f.At)
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

// An explicit receipt is a constraint, never a hint that a valid assignment
// may override. A source receipt or an exact claimed recipient must be local.
func workflowExplicitSessionHandoff(in WorkflowInput, a runtimeobs.Attempt, id string) bool {
	count := 0
	for key, h := range in.Handoffs {
		if key != id && h.HandoffID != id {
			continue
		}
		count++
		if count > 1 || h.HandoffID != id || h.TaskID != in.TaskID {
			return false
		}
		source := h.SourceAgentPath == a.AgentPath && h.SourceAttemptID == a.AttemptID
		// If the receipt explicitly targets/claims this exact attempt, its target
		// identity must also agree; a contradictory native is not a source fallback.
		targetsAttempt := h.Target.AttemptID == a.AttemptID || h.ClaimedAttemptID == a.AttemptID
		if targetsAttempt && !workflowSessionClaim(in, a, id) {
			return false
		}
		if !source && !workflowSessionClaim(in, a, id) {
			return false
		}
	}
	return count == 1
}

// Native execution identity is independent of permission to report Controller
// Activity or review. This read-only join applies only to exact task-bound T3
// observation facts; taskless global Controllers still require a claimed handoff.
func workflowNativeSessionAssociation(in WorkflowInput, a runtimeobs.Attempt, assignment, handoff string) bool {
	expected := a.BindingEvidence["assignment_id"]
	return a.TaskID == in.TaskID && a.TaskID != "" && a.BindingState == runtimeobs.BindingBound && a.StateEvidenceSource == runtimeobs.EvidenceT3 && runtimeobs.HasObservedExecution(a) && a.NativeReceipt != nil && a.NativeReceipt.ThreadID != "" && a.NativeReceipt.RunID != "" && a.NativeReceipt.ThreadID != a.SessionID && a.RuntimeAgentID == a.SessionID && a.BindingEvidence["runtime_agent_id"] == a.RuntimeAgentID && expected != "" && assignment == expected && handoff == ""
}

func workflowSessionAssociation(in WorkflowInput, a runtimeobs.Attempt, assignment, handoff string) bool {
	role := workflowActorRole(a.AgentPath)
	if a.TaskID != in.TaskID && !(role == "controller" && a.TaskID == "") {
		return false
	}
	expected := a.BindingEvidence["assignment_id"]
	if assignment != "" && expected != "" && assignment != expected {
		return false
	}
	explicit := handoff
	if role == "controller" && explicit == "" && assignment != "" {
		explicit = assignment
	}
	if explicit != "" && !workflowExplicitSessionHandoff(in, a, explicit) {
		return false
	}
	if role == "controller" && expected == "" && assignment != "" && handoff != "" && assignment != handoff {
		return false
	}
	if role == "controller" {
		if handoff == "" && assignment != "" {
			handoff = assignment
		}
		return workflowSessionClaim(in, a, handoff)
	}
	if a.TaskID != in.TaskID {
		return false
	}
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
