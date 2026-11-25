package backlog

import (
	"errors"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type runtimeImplementationOwner struct {
	rolepolicy.Owner
	Structured bool
}

// Match the context gate: structured implementation metadata is authoritative;
// otherwise only an explicit legacy Worker path is an ownership declaration.
func runtimeOwner(project string, row Record) (runtimeImplementationOwner, error) {
	p, _, e := rolepolicy.Load(project, row.ID, row.RawMarkdown)
	if e != nil {
		return runtimeImplementationOwner{}, e
	}
	if p != nil {
		owner := p.Implementation
		if owner.AgentPath == "" || owner.Role != rolepolicy.Role(owner.AgentPath) || owner.Role == "unknown" {
			return runtimeImplementationOwner{}, errors.New("implementation_policy_owner_invalid")
		}
		return runtimeImplementationOwner{Owner: owner, Structured: true}, nil
	}
	path := strings.TrimSpace(row.Fields["Agent"])
	if !rolepolicy.WorkerPath(path) {
		return runtimeImplementationOwner{}, errors.New("implementation_owner_unresolved")
	}
	return runtimeImplementationOwner{Owner: rolepolicy.Owner{Role: "worker", AgentPath: path}}, nil
}

func runtimeOwnerGap(task, reason string, owner runtimeImplementationOwner) map[string]any {
	return map[string]any{"health": "implementation_owner_unverified", "source": "execution_ledger", "owner_resolution_error": reason, "agent_path": owner.AgentPath, "implementation_owner": owner.Owner,
		"progress_evidence": map[string]any{"task_id": task, "phase": "implementation_owner_unverified", "action_owner": "/root/controller", "remaining_stage": "implementation_owner_verification", "technical_crash": "unproven", "resume_condition": "Verify current structured implementation owner/native or explicit legacy owner, exact assignment and observed bound implementation attempt. Preserve reviewer/other-actor evidence separately; never use latest-task or reviewer active state as fallback. Existing preflight/transport/approval gates remain."}}
}

func implementationDispatchTime(a runtimeobs.Attempt) time.Time {
	var latest time.Time
	for _, raw := range []string{a.BindingAt, a.StartedAt, a.FirstObservedAt} {
		if at, e := time.Parse(time.RFC3339Nano, raw); e == nil && at.After(latest) {
			latest = at
		}
	}
	return latest
}

// A future dispatch request does not replace an already observed execution.
// Keep this projection separate from assignment/START authorization: it only
// selects existing exact-owner evidence, and never grants a prepared attempt START.
func activeImplementationPhase(a runtimeobs.Attempt, owner runtimeImplementationOwner, assignment runtimeobs.Assignment, events []activity.Event, now time.Time) bool {
	identity := runtimeobs.PolicyIdentity(a)
	assignedAt, err := time.Parse(time.RFC3339Nano, assignment.AssignedAt)
	observedAt, observedErr := time.Parse(time.RFC3339Nano, a.LastObservedAt)
	if a.Terminal || a.CurrentState != runtimeobs.StateRunning || a.StartedAt == "" || a.BindingState != runtimeobs.BindingBound || a.TaskID != assignment.TaskID || a.AgentPath != owner.AgentPath || assignment.AgentPath != owner.AgentPath || a.BindingEvidence["assignment_id"] != assignment.AssignmentID || !identity.Observed || identity.NativeID == "" || a.RuntimeAgentID != identity.NativeID || identity.AgentPath != owner.AgentPath || rolepolicy.Role(identity.AgentPath) != owner.Role || (owner.NativeID != "" && identity.NativeID != owner.NativeID) || err != nil || observedErr != nil || assignedAt.After(now) || observedAt.After(now) || observedAt.Before(assignedAt) {
		return false
	}
	effective, _ := activity.EffectiveEvents(events)
	var latest *activity.Event
	for _, event := range attributedActivityEvents(a, effective, now) {
		if latest == nil || event.OccurredAt.After(latest.OccurredAt) {
			copy := event
			latest = &copy
		} else if event.OccurredAt.Equal(latest.OccurredAt) && event.EventID != latest.EventID {
			return false
		}
	}
	return latest != nil && (latest.Action == activity.Start || latest.Action == activity.Switch)
}

// Only actual owner-correlated implementation evidence can override legacy
// registry signals. Owner errors explicitly override rather than falling back to
// an unrelated reviewer or another native sharing the same display/path metadata.
func runtimeLedgerSignals(project string, rows []Record, now time.Time, shared ...requestRuntime) map[string]map[string]any {
	out := map[string]map[string]any{}
	expected := map[string]runtimeImplementationOwner{}
	for _, row := range rows {
		if row.Location != "active" || (row.State != "doing" && row.State != "hold") {
			continue
		}
		id := strings.ToUpper(strings.TrimSpace(row.ID))
		owner, e := runtimeOwner(project, row)
		if e != nil {
			// Ownership recovery concerns active implementation, not unrelated
			// completed or queued tasks lacking legacy owner metadata.
			out[id] = runtimeOwnerGap(id, e.Error(), owner)
			continue
		}
		expected[id] = owner
	}
	ledger, e := readRequestRuntime(project, 20, now, shared...)
	if e != nil {
		for id, owner := range expected {
			out[id] = runtimeOwnerGap(id, e.Error(), owner)
		}
		return out
	}
	assignments, e := runtimeobs.ListAssignments(project)
	if e != nil {
		for id, owner := range expected {
			out[id] = runtimeOwnerGap(id, e.Error(), owner)
		}
		return out
	}
	latestAssignment := map[string]runtimeobs.Assignment{}
	ambiguousAssignment := map[string]bool{}
	for _, a := range assignments {
		key := a.TaskID + "\x00" + a.AgentPath
		previous, ok := latestAssignment[key]
		if !ok || a.AssignedAt > previous.AssignedAt {
			latestAssignment[key] = a
			ambiguousAssignment[key] = false
		} else if a.AssignedAt == previous.AssignedAt && a.AssignmentID != previous.AssignmentID {
			ambiguousAssignment[key] = true
		}
	}
	// Only an unused/unobserved future assignment may coexist with the current
	// verified phase. Once its own native is observed, normal latest selection
	// must expose its missing START, identity or terminal evidence instead.
	for id, owner := range expected {
		key := id + "\x00" + owner.AgentPath
		future, ok := latestAssignment[key]
		if !ok || ambiguousAssignment[key] || !owner.Structured {
			continue
		}
		observedFuture := false
		for _, a := range ledger.Attempts {
			if a.TaskID == id && a.BindingEvidence["assignment_id"] == future.AssignmentID && runtimeobs.PolicyIdentity(a).Observed {
				observedFuture = true
			}
		}
		if observedFuture {
			continue
		}
		events, activityErr := (activity.Ledger{Project: project}).ListCached(id)
		if activityErr != nil {
			continue
		}
		var current runtimeobs.Assignment
		count := 0
		for _, a := range ledger.Attempts {
			if a.TaskID != id {
				continue
			}
			for _, assignment := range assignments {
				if assignment.AssignedAt < future.AssignedAt && activeImplementationPhase(a, owner, assignment, events, now) {
					current = assignment
					count++
				}
			}
		}
		if count == 1 {
			latestAssignment[key] = current
		} else if count > 1 {
			ambiguousAssignment[key] = true
		}
	}
	stale := map[string]bool{}
	for _, finding := range ledger.Findings {
		if finding.Code == "stale" && finding.AttemptID != "" {
			stale[finding.AttemptID] = true
		}
	}
	latest := map[string]runtimeobs.Attempt{}
	ambiguousAttempt := map[string]bool{}
	for _, a := range ledger.Attempts {
		id := strings.ToUpper(strings.TrimSpace(a.TaskID))
		owner, ok := expected[id]
		if !ok || a.BindingState != runtimeobs.BindingBound || a.AgentPath != owner.AgentPath {
			continue
		}
		key := id + "\x00" + owner.AgentPath
		assignment, assigned := latestAssignment[key]
		if ambiguousAssignment[key] {
			out[id] = runtimeOwnerGap(id, "implementation_assignment_ambiguous", owner)
			continue
		}
		if assigned && a.BindingEvidence["assignment_id"] != assignment.AssignmentID {
			continue
		}
		previous, ok := latest[id]
		at := implementationDispatchTime(a)
		previousAt := implementationDispatchTime(previous)
		if !ok || at.After(previousAt) {
			latest[id] = a
			ambiguousAttempt[id] = false
		} else if at.Equal(previousAt) && a.AttemptID != previous.AttemptID {
			ambiguousAttempt[id] = true
		}
	}
	for id, a := range latest {
		if out[id] != nil {
			continue
		}
		owner := expected[id]
		identity := runtimeobs.PolicyIdentity(a)
		_, assigned := latestAssignment[id+"\x00"+owner.AgentPath]
		if ambiguousAttempt[id] {
			out[id] = runtimeOwnerGap(id, "implementation_attempt_ambiguous", owner)
			continue
		}
		if owner.Structured && (!identity.Observed || identity.NativeID == "" || !assigned || identity.AgentPath != owner.AgentPath || rolepolicy.Role(identity.AgentPath) != owner.Role || (owner.NativeID != "" && identity.NativeID != owner.NativeID)) {
			signal := runtimeOwnerGap(id, "implementation_identity_or_assignment_unverified", owner)
			signal["unverified_attempt_id"] = a.AttemptID
			signal["candidate_native_id"] = a.SessionID
			out[id] = signal
			continue
		}
		health := "active"
		switch a.CurrentState {
		case runtimeobs.StateWaitingUser, runtimeobs.StateWaitingApproval:
			health = "needs_user"
		case runtimeobs.StateCompleted:
			health = "progress_collection_gap"
		case runtimeobs.StateErrored, runtimeobs.StateInterrupted, runtimeobs.StateShutdown:
			health = "execution_interrupted"
		case runtimeobs.StateRuntimeUnknown:
			health = "runtime_unknown"
		default:
			if stale[a.AttemptID] {
				health = "stale"
			}
		}
		if a.StartedAt == "" && !a.Terminal {
			health = "awaiting_start"
		}
		events, activityErr := (activity.Ledger{Project: project}).ListCached(id)
		signal := map[string]any{"health": health, "source": "execution_ledger", "attempt_id": a.AttemptID, "runtime_state": a.CurrentState, "last_activity_at": a.LastActivityAt, "binding_state": a.BindingState, "binding_source": a.BindingSource, "binding_at": a.BindingAt, "started_at": a.StartedAt, "runtime_agent_id": a.RuntimeAgentID, "assignment_id": a.BindingEvidence["assignment_id"], "evidence_source": a.StateEvidenceSource, "observation_quality": a.StateObservationQuality, "agent_path": owner.AgentPath, "implementation_owner": owner.Owner, "progress_evidence": workerProgress(a, attributedActivityEvents(a, events, now))}
		if activityErr != nil {
			signal["activity_read_error"] = activityErr.Error()
		}
		observed, observedErr := time.Parse(time.RFC3339Nano, a.LastObservedAt)
		assignedAt, assignedErr := time.Parse(time.RFC3339Nano, latestAssignment[id+"\x00"+owner.AgentPath].AssignedAt)
		exact := identity.Observed && identity.NativeID != "" && a.RuntimeAgentID == identity.NativeID && assigned && assignedErr == nil && !assignedAt.After(now) && observedErr == nil && !observed.After(now) && !observed.Before(assignedAt) && (owner.NativeID == "" || owner.NativeID == identity.NativeID)
		signal["human_action_runtime_verified"] = exact
		signal["native_id"] = identity.NativeID
		signal["assigned_at"] = latestAssignment[id+"\x00"+owner.AgentPath].AssignedAt
		if exact {
			signal["recent_activity_report"] = currentActivityEvidence(a, events, now)
		}
		out[id] = signal
	}
	owners := map[string]int{}
	for _, owner := range expected {
		owners[owner.AgentPath]++
	}
	for id, owner := range expected {
		if out[id] != nil {
			continue
		}
		key := id + "\x00" + owner.AgentPath
		assignment, assigned := latestAssignment[key]
		if ambiguousAssignment[key] {
			out[id] = runtimeOwnerGap(id, "implementation_assignment_ambiguous", owner)
			continue
		}
		candidates := []string{}
		ambiguous := false
		name := owner.AgentPath
		if slash := strings.LastIndex(name, "/"); slash >= 0 {
			name = name[slash+1:]
		}
		for _, a := range ledger.Attempts {
			if a.Terminal || (a.BindingState != runtimeobs.BindingUnbound && a.BindingState != runtimeobs.BindingAmbiguous) {
				continue
			}
			match := owner.NativeID != "" && (a.RuntimeAgentID == owner.NativeID || a.SessionID == owner.NativeID)
			if !owner.Structured {
				match = a.RuntimeAgentID == owner.AgentPath || a.RuntimeAgentID == name
			}
			if !match {
				continue
			}
			candidates = append(candidates, a.AttemptID)
			if a.BindingState == runtimeobs.BindingAmbiguous {
				ambiguous = true
			}
		}
		if !assigned && len(candidates) == 0 {
			if owner.Structured {
				out[id] = runtimeOwnerGap(id, "implementation_assignment_or_observation_missing", owner)
			}
			continue
		}
		health := "assignment_unobserved"
		if len(candidates) > 0 {
			health = "binding_pending"
		}
		if ambiguous || len(candidates) > 1 || (len(candidates) > 0 && owners[owner.AgentPath] > 1) {
			health = "binding_ambiguous"
		}
		out[id] = map[string]any{"health": health, "source": "execution_ledger", "assignment_id": assignment.AssignmentID, "assigned_at": assignment.AssignedAt, "candidate_attempt_ids": candidates, "candidate_count": len(candidates), "agent_path": owner.AgentPath, "implementation_owner": owner.Owner}
	}
	// Keep other task actors as observations, never as implementation progress.
	for id, signal := range out {
		owner, known := expected[id]
		others := []map[string]any{}
		for _, a := range ledger.Attempts {
			if a.TaskID != id || a.BindingState != runtimeobs.BindingBound || (known && a.AgentPath == owner.AgentPath) {
				continue
			}
			identity := runtimeobs.PolicyIdentity(a)
			others = append(others, map[string]any{"attempt_id": a.AttemptID, "agent_path": a.AgentPath, "native_id": identity.NativeID, "observed": identity.Observed, "assignment_id": a.BindingEvidence["assignment_id"], "runtime_state": a.CurrentState, "terminal": a.Terminal})
		}
		signal["other_actors"] = others
	}
	return out
}

func attemptSignalTime(a runtimeobs.Attempt) time.Time {
	for _, raw := range []string{a.LastObservedAt, a.EndedAt, a.LastActivityAt, a.StartedAt, a.FirstObservedAt} {
		if at, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			return at
		}
	}
	return time.Time{}
}

func mergeRuntimeSignals(base, ledger map[string]map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for id, signal := range base {
		out[id] = signal
	}
	for id, signal := range ledger {
		// A bound Execution Ledger attempt has stronger task correlation than the
		// legacy registry signal. Keep legacy data only when no bound signal exists.
		out[id] = signal
	}
	return out
}
