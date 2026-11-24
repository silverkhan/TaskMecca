package activity

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// ReadWorkflowExecutions reads only persisted evidence. Unlike reconciliation it
// never scans provider sessions, repairs bindings, appends events or takes a
// monitor lock. A missing/stale Control Tower snapshot cannot hide terminal evidence.
func ReadWorkflowExecutions(project, task string, now time.Time) ([]runtimeobs.Attempt, error) {
	ledger, err := runtimeobs.BuildLedger(project, 0, now)
	if err != nil {
		return nil, err
	}
	for _, f := range ledger.Findings {
		if f.Code == "invalid_event_line" {
			return nil, fmt.Errorf("invalid runtime evidence: %s", f.Code)
		}
	}
	out := []runtimeobs.Attempt{}
	for _, a := range ledger.Attempts {
		// Taskless Controller candidates need an exact current-task claim in the
		// session resolver. Supplying a candidate alone creates no task facts.
		if a.TaskID == task || (a.TaskID == "" && a.AgentPath == "/root/controller" && a.BindingState == runtimeobs.BindingBound) {
			out = append(out, a)
		}
	}
	return out, nil
}

// Role facts are inspectable source points. They never contain a work duration.
type WorkflowRoleFact struct {
	ID                string    `json:"id"`
	Kind              string    `json:"kind"`
	At                time.Time `json:"at"`
	Source            string    `json:"source"`
	Reason            string    `json:"reason"`
	MeasurementStatus string    `json:"measurement_status"`
	AttemptID         string    `json:"attempt_id,omitempty"`
	AssignmentID      string    `json:"assignment_id,omitempty"`
	HandoffID         string    `json:"handoff_id,omitempty"`
}
type WorkflowGapEvidence struct {
	Source     string    `json:"source"`
	ObservedAt time.Time `json:"observed_at"`
	Reason     string    `json:"reason"`
	EventID    string    `json:"event_id,omitempty"`
	AttemptID  string    `json:"attempt_id,omitempty"`
}
type RoleReportingCoverage struct {
	ExecutionCoverage  *ExecutionCoverage `json:"execution_coverage,omitempty"`
	Role               string             `json:"role"`
	Status             string             `json:"status"`
	FactCount          int                `json:"fact_count"`
	ActivityEventCount int                `json:"activity_event_count"`
}

func workflowCoverageItems(w Workflow) []ExecutionCoverageItem {
	out := []ExecutionCoverageItem{}
	for _, row := range w.Rows {
		for _, interval := range row.Intervals {
			category := "observation_handoff"
			if interval.Measured && interval.End != nil {
				category = "confirmed_closed_activity"
			} else if interval.End == nil && (interval.Kind == "activity" || interval.Kind == "activity_open") {
				category = "open_start"
			} else if interval.Kind == "activity_unreported" {
				category = "unmeasured"
			}
			out = append(out, ExecutionCoverageItem{ID: coverageID(row.Role, row.AgentID, interval.ID), Category: category, Role: row.Role, ActorID: row.AgentID, AttemptID: interval.AttemptID, Start: interval.Start, End: interval.End, DurationMillis: interval.DurationMillis, Reason: interval.Evidence})
		}
		for _, fact := range row.RoleFacts {
			category := "observation_handoff"
			if fact.Kind == "collection_failed" {
				category = "collection_failure"
			}
			out = append(out, ExecutionCoverageItem{ID: coverageID(row.Role, row.AgentID, fact.ID), Category: category, Role: row.Role, ActorID: row.AgentID, AttemptID: fact.AttemptID, HandoffID: fact.HandoffID, Start: fact.At, Reason: fact.Reason})
		}
	}
	for i, gap := range w.Unmeasured {
		out = append(out, ExecutionCoverageItem{ID: fmt.Sprintf("unmeasured:%d", i), Category: "unmeasured", Role: "unknown", Start: gap.Start, End: &gap.End, Reason: gap.Status})
	}
	return out
}

// Optional explicit caller evidence, never inferred from missing Activity or
// diagnostic wording. A failed report is a point, not a failure-duration span.
type WorkflowCollectionFailure struct {
	ID, TaskID, Role, AgentID, AttemptID, Source, Reason string
	At                                                   time.Time
}

func workflowActorRole(path string) string {
	if path == "/root/registrar" {
		return "registrar"
	}
	if path == "/root/controller" {
		return "controller"
	}
	if strings.HasPrefix(path, "/root/controller/sub-") && !strings.Contains(strings.TrimPrefix(path, "/root/controller/"), "/") {
		return "sub_controller"
	}
	if r := rolepolicy.Role(path); r != "" {
		return r
	}
	return "unknown"
}

func workflowObservedAttempts(in WorkflowInput) []runtimeobs.Attempt {
	counts := map[string]int{}
	for _, a := range in.ExecutionObservations {
		counts[a.AttemptID]++
	}
	out := []runtimeobs.Attempt{}
	for _, a := range in.ExecutionObservations {
		if a.AttemptID == "" || counts[a.AttemptID] != 1 || a.TaskID != in.TaskID || a.BindingState != runtimeobs.BindingBound || a.RuntimeAgentID == "" || a.SessionID == "" || a.Provider == "" || !runtimeobs.HasObservedExecution(a) {
			continue
		}
		if native := a.BindingEvidence["runtime_agent_id"]; native != "" && native != a.RuntimeAgentID {
			continue
		}
		if a.StateEvidenceSource == runtimeobs.EvidenceT3 && (a.NativeReceipt.ThreadID == "" || a.NativeReceipt.RunID == "" || a.NativeReceipt.ThreadID == a.SessionID) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func workflowRoleFacts(in WorkflowInput, w *Workflow, rows map[string]*WorkflowRow, add func(string, string, *int, WorkflowInterval), bounds []reviewBoundary, known bool, events []Event) {
	coverage := map[string]*RoleReportingCoverage{}
	for _, role := range []string{"worker", "controller", "registrar"} {
		coverage[role] = &RoleReportingCoverage{Role: role, Status: "not_recorded"}
	}
	roleCoverage := func(role string) *RoleReportingCoverage {
		if coverage[role] == nil {
			coverage[role] = &RoleReportingCoverage{Role: role, Status: "not_recorded"}
		}
		return coverage[role]
	}
	seen := map[string]bool{}
	missing := map[string]bool{}
	fact := func(role, agent string, f WorkflowRoleFact) {
		if f.ID == "" || f.At.IsZero() || f.At.After(in.Now) || seen[role+"|"+agent+"|"+f.ID] {
			return
		}
		seen[role+"|"+agent+"|"+f.ID] = true
		n := roundNumber(f.At, bounds, known)
		label := "unknown"
		if n != nil {
			label = fmt.Sprint(*n)
		}
		key := role + "|" + agent + "|" + label
		if rows[key] == nil {
			rows[key] = &WorkflowRow{ID: key, Role: role, AgentID: agent, Round: n, RoundStatus: roundStatus(n), Intervals: []WorkflowInterval{}}
		}
		f.MeasurementStatus = "unmeasured"
		rows[key].RoleFacts = append(rows[key].RoleFacts, f)
		roleCoverage(role).FactCount++
	}
	for _, f := range in.Facts {
		if f.TaskID != in.TaskID {
			continue
		}
		// Execution relays already have their original interval. Registration,
		// assignment, wait and completion remain source points for their actor.
		if f.Kind == "started" || f.Kind == "resumed" {
			continue
		}
		if f.Actor == "" {
			continue
		} // event kind is not proof of a role or identity
		source := f.EvidenceSource
		if source == "" {
			source = "lifecycle_ledger"
		}
		fact(workflowActorRole(f.Actor), f.Actor, WorkflowRoleFact{ID: f.EventID, Kind: f.Kind, At: f.OccurredAt, Source: source, Reason: "Reported lifecycle fact; no role Activity measurement is implied", AttemptID: f.AttemptID, AssignmentID: f.AssignmentID})
	}
	for _, a := range workflowObservedAttempts(in) {
		at, ok := workflowTime(a.FirstObservedAt)
		if !ok {
			at, ok = workflowTime(a.StartedAt)
		}
		if !ok {
			continue
		}
		role := workflowActorRole(a.AgentPath)
		recorded := false
		for _, e := range events {
			if string(e.Role) == role && e.RuntimeAttemptID == a.AttemptID && e.AgentID == a.RuntimeAgentID {
				recorded = true
				break
			}
		}
		if !recorded {
			missing[role] = true
			w.Reporting.Findings = append(w.Reporting.Findings, ReportingFinding{Code: "role_execution_activity_missing", Role: role, AttemptID: a.AttemptID, AssignmentID: a.BindingEvidence["assignment_id"], RequiredPhase: "current_actual_phase"})
		}
		fact(role, a.RuntimeAgentID, WorkflowRoleFact{ID: "runtime:" + a.AttemptID, Kind: "observed_execution", At: at, Source: string(a.StateEvidenceSource), Reason: "Exact task-bound native execution observed; phase activity remains independently reported", AttemptID: a.AttemptID, AssignmentID: a.BindingEvidence["assignment_id"]})
		if role != "worker" {
			// A bounded observation range, never an agent-active interval. Do not
			// extrapolate a running session from last observation to current time.
			end, valid := workflowTime(a.LastObservedAt)
			if a.Terminal {
				end, valid = workflowTime(a.EndedAt)
			}
			if valid && !end.Before(at) && !end.After(in.Now) {
				add(role, a.RuntimeAgentID, roundNumber(at, bounds, known), WorkflowInterval{ID: "runtime:" + a.AttemptID, Kind: "native_observed", Type: "unclassified_execution", Start: at, End: &end, AttemptID: a.AttemptID, AssignmentID: a.BindingEvidence["assignment_id"], Evidence: string(a.StateEvidenceSource), EndEvidence: "observed_boundary"})
			}
		}
	}
	for _, h := range in.Handoffs {
		if h.TaskID != in.TaskID {
			continue
		}
		if at, ok := workflowTime(h.PreparedAt); ok && h.SourceAgentPath != "" {
			fact(workflowActorRole(h.SourceAgentPath), h.SourceAgentPath, WorkflowRoleFact{ID: "prepared:" + h.HandoffID, Kind: "handoff_prepared", At: at, Source: "handoff_journal", Reason: "Handoff prepared; delivery and activity are separate", AttemptID: h.SourceAttemptID, HandoffID: h.HandoffID})
		}
		if step, ok := h.Steps["dispatch"]; ok && step.Result == "ok" && h.DeliveryStatus == "dispatch_ok" && h.SourceAgentPath != "" {
			if at, valid := workflowTime(step.ObservedAt); valid {
				fact(workflowActorRole(h.SourceAgentPath), h.SourceAgentPath, WorkflowRoleFact{ID: "delivered:" + h.HandoffID, Kind: "handoff_delivered", At: at, Source: "handoff_journal", Reason: step.Evidence, AttemptID: h.SourceAttemptID, HandoffID: h.HandoffID})
			}
		}
		if at, ok := workflowTime(h.ClaimedAt); ok && h.ClaimedBy != "" {
			fact(workflowActorRole(h.ClaimedBy), h.Target.RuntimeAgentID, WorkflowRoleFact{ID: "claimed:" + h.HandoffID, Kind: "handoff_claimed", At: at, Source: "handoff_journal", Reason: "Actual handoff claim; no measured coordination or review duration", AttemptID: h.ClaimedAttemptID, HandoffID: h.HandoffID})
		}
	}
	for _, f := range in.CollectionFailures {
		if f.TaskID == in.TaskID && f.Source != "" && f.Reason != "" {
			role := f.Role
			if !Role(role).Valid() {
				role = "unknown"
			}
			fact(role, f.AgentID, WorkflowRoleFact{ID: f.ID, Kind: "collection_failed", At: f.At, Source: f.Source, Reason: f.Reason, AttemptID: f.AttemptID})
		}
	}
	for _, e := range events {
		roleCoverage(string(e.Role)).ActivityEventCount++
	}
	w.Reporting.Scope = "observed_evidence_only"
	roles := []string{}
	for role := range coverage {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	partial := false
	for _, role := range roles {
		c := coverage[role]
		if c.ActivityEventCount > 0 {
			c.Status = "recorded"
		} else if c.FactCount > 0 {
			c.Status = "partial"
		}
		if missing[role] {
			c.Status = "partial"
		}
		if c.Status != "recorded" {
			partial = true
		}
		w.Reporting.Roles = append(w.Reporting.Roles, *c)
	}
	if partial && w.Reporting.Status != "missing" {
		w.Reporting.Status = "partial"
	}
	for _, row := range rows {
		sort.Slice(row.RoleFacts, func(i, j int) bool {
			if row.RoleFacts[i].At.Equal(row.RoleFacts[j].At) {
				return row.RoleFacts[i].ID < row.RoleFacts[j].ID
			}
			return row.RoleFacts[i].At.Before(row.RoleFacts[j].At)
		})
	}
}

type workflowGapSource struct {
	start, end    time.Time
	status, agent string
	evidence      WorkflowGapEvidence
}

// Lifecycle wait intervals are task-global candidates. Only unique endpoint
// facts for the same exact actor, execution and assignment support a role wait.
// A logical role name alone cannot pair different native episodes.
func workflowWaitPair(task string, v WaitInterval, facts map[string][]LifecycleFact, now time.Time) (LifecycleFact, string) {
	from, to := facts[v.FromEventID], facts[v.ToEventID]
	if v.FromEventID == "" || v.ToEventID == "" || v.FromEventID == v.ToEventID || len(from) != 1 || len(to) != 1 {
		return LifecycleFact{}, "wait/resume endpoints are missing or ambiguous"
	}
	a, b := from[0], to[0]
	if a.TaskID != task || b.TaskID != task || a.EventID != v.FromEventID || b.EventID != v.ToEventID || a.Kind != "waiting" || b.Kind != "resumed" || a.OccurredAt.IsZero() || b.OccurredAt.IsZero() || !a.OccurredAt.Equal(v.StartedAt) || !b.OccurredAt.Equal(v.EndedAt) || !b.OccurredAt.After(a.OccurredAt) || b.OccurredAt.After(now) {
		return a, "wait/resume task, kind or original time boundary does not match"
	}
	if a.Actor == "" || a.Actor != b.Actor || !Role(workflowActorRole(a.Actor)).Valid() {
		return a, "wait/resume actor identity is missing, unsupported or different"
	}
	if a.AttemptID == "" || a.AttemptID != b.AttemptID || a.AssignmentID == "" || a.AssignmentID != b.AssignmentID {
		return a, "wait/resume exact attempt or assignment evidence is missing or different"
	}
	return a, ""
}

func workflowGapEvidence(in WorkflowInput, w *Workflow) {
	sources := []workflowGapSource{}
	facts := map[string][]LifecycleFact{}
	for _, f := range in.Facts {
		if f.TaskID == in.TaskID {
			facts[f.EventID] = append(facts[f.EventID], f)
		}
	}
	paths := map[string]string{}
	for _, a := range workflowObservedAttempts(in) {
		paths[a.RuntimeAgentID] = a.AgentPath
	}
	matches := func(row, source string) bool {
		return row == "" || (source != "" && (source == row || paths[row] == source || paths[source] == row))
	}
	for _, v := range w.Lifecycle.Intervals {
		if v.Kind == "explicit_wait" {
			from, rejected := workflowWaitPair(in.TaskID, v, facts, in.Now)
			status, reason := "confirmed_wait", "Explicit waiting/resume reports match the same actor, exact attempt and assignment; no active work inferred"
			if rejected != "" {
				status, reason = "unmeasured", "Task-global wait candidate is not a confirmed role wait: "+rejected
			}
			sources = append(sources, workflowGapSource{start: v.StartedAt, end: v.EndedAt, status: status, agent: from.Actor, evidence: WorkflowGapEvidence{Source: "reported_lifecycle", ObservedAt: v.StartedAt, EventID: v.FromEventID, AttemptID: from.AttemptID, Reason: reason}})
		}
	}
	for _, a := range workflowObservedAttempts(in) {
		start, ok := workflowTime(a.StartedAt)
		end, valid := workflowTime(a.LastObservedAt)
		if a.Terminal {
			end, valid = workflowTime(a.EndedAt)
		}
		if !ok || !valid || !end.After(start) || end.After(in.Now) {
			continue
		}
		sources = append(sources, workflowGapSource{start: start, end: end, status: "native_observed", agent: a.RuntimeAgentID, evidence: WorkflowGapEvidence{Source: string(a.StateEvidenceSource), ObservedAt: end, AttemptID: a.AttemptID, Reason: "Native execution observed; activity within this range is unmeasured, not idle or measured work"}})
	}
	classify := func(gaps []WorkflowGap, agent string, attempts map[string]bool) []WorkflowGap {
		applies := func(s workflowGapSource) bool {
			return matches(agent, s.agent) && (agent == "" || s.status != "confirmed_wait" || attempts[s.evidence.AttemptID])
		}
		out := []WorkflowGap{}
		for _, g := range gaps {
			cuts := []time.Time{g.Start, g.End}
			for _, s := range sources {
				if !applies(s) {
					continue
				}
				for _, at := range []time.Time{s.start, s.end} {
					if at.After(g.Start) && at.Before(g.End) {
						cuts = append(cuts, at)
					}
				}
			}
			sort.Slice(cuts, func(i, j int) bool { return cuts[i].Before(cuts[j]) })
			for i := 1; i < len(cuts); i++ {
				if !cuts[i].After(cuts[i-1]) {
					continue
				}
				piece := WorkflowGap{Start: cuts[i-1], End: cuts[i], Kind: "activity_unmeasured", Status: "unmeasured", Evidence: []WorkflowGapEvidence{{Source: "activity_ledger", ObservedAt: in.Now, Reason: "No closed measured Activity covers this span; insufficient evidence to infer activity or inactivity"}}}
				for _, s := range sources {
					if !applies(s) {
						continue
					}
					if !s.start.After(piece.Start) && !s.end.Before(piece.End) {
						piece.Evidence = append(piece.Evidence, s.evidence)
						if s.status == "confirmed_wait" || piece.Status == "unmeasured" {
							piece.Status = s.status
						}
					}
				}
				for _, f := range in.CollectionFailures {
					if f.TaskID == in.TaskID && f.Source != "" && f.Reason != "" && (agent == "" || f.AgentID == agent) && !f.At.Before(piece.Start) && f.At.Before(piece.End) {
						piece.Evidence = append(piece.Evidence, WorkflowGapEvidence{Source: f.Source, ObservedAt: f.At, EventID: f.ID, AttemptID: f.AttemptID, Reason: "Collection failure observed at this point; the span is NOT a measured failure duration: " + f.Reason})
						if piece.Status == "unmeasured" {
							piece.Status = "collection_failed"
						}
					}
				}
				out = append(out, piece)
			}
		}
		return out
	}
	w.Unmeasured = classify(w.Unmeasured, "", nil)
	if w.Axis.Start == nil || w.Axis.End == nil {
		return
	}
	for i := range w.Rows {
		r := &w.Rows[i]
		attempts := map[string]bool{}
		for _, v := range r.Intervals {
			if v.AttemptID != "" {
				attempts[v.AttemptID] = true
			}
		}
		for _, f := range r.RoleFacts {
			if f.AttemptID != "" {
				attempts[f.AttemptID] = true
			}
		}
		lo, hi := *w.Axis.Start, *w.Axis.End
		if r.Round != nil && *r.Round >= 0 && *r.Round < len(w.Rounds) {
			if w.Rounds[*r.Round].StartedAt.After(lo) {
				lo = w.Rounds[*r.Round].StartedAt
			}
			if *r.Round+1 < len(w.Rounds) && w.Rounds[*r.Round+1].StartedAt.Before(hi) {
				hi = w.Rounds[*r.Round+1].StartedAt
			}
		}
		// Bounds describe evidence-backed participation, not a claim that the
		// actor was absent outside it. A point alone has no inferred duration.
		var first, last time.Time
		include := func(t time.Time) {
			if t.IsZero() {
				return
			}
			if first.IsZero() || t.Before(first) {
				first = t
			}
			if last.IsZero() || t.After(last) {
				last = t
			}
		}
		for _, v := range r.Intervals {
			include(v.Start)
			if v.End != nil {
				include(*v.End)
			}
		}
		for _, f := range r.RoleFacts {
			include(f.At)
		}
		for _, s := range sources {
			if matches(r.AgentID, s.agent) && (s.evidence.AttemptID == "" || attempts[s.evidence.AttemptID]) {
				include(s.start)
				include(s.end)
			}
		}
		outside := []WorkflowGap{}
		if !first.IsZero() && !last.IsZero() {
			if first.After(lo) {
				end := first
				if end.After(hi) {
					end = hi
				}
				outside = append(outside, participationGap(lo, end, "before_participation"))
			}
			if last.Before(hi) {
				start := last
				if start.Before(lo) {
					start = lo
				}
				outside = append(outside, participationGap(start, hi, "after_participation"))
			}
			if first.After(lo) {
				lo = first
			}
			if last.Before(hi) {
				hi = last
			}
			r.Participation = &WorkflowParticipation{Start: first, End: last, Basis: "observed_bounds_not_exclusive_presence", Status: "bounded"}
		}
		measured := []timingInterval{}
		for _, v := range r.Intervals {
			if v.Measured && v.End != nil {
				if x, ok := clipInterval(v.Start, *v.End, lo, hi); ok {
					measured = append(measured, x)
				}
			}
		}
		sort.Slice(measured, func(i, j int) bool { return measured[i].start.Before(measured[j].start) })
		cursor := lo
		gaps := []WorkflowGap{}
		for _, v := range measured {
			if v.start.After(cursor) {
				gaps = append(gaps, WorkflowGap{Start: cursor, End: v.start})
			}
			if v.end.After(cursor) {
				cursor = v.end
			}
		}
		if cursor.Before(hi) {
			gaps = append(gaps, WorkflowGap{Start: cursor, End: hi})
		}
		r.Gaps = append(outside, classify(gaps, r.AgentID, attempts)...)
		sort.SliceStable(r.Gaps, func(i, j int) bool { return r.Gaps[i].Start.Before(r.Gaps[j].Start) })
	}
}

// Resolve the exact attempt and binding, never a logical name or another run.
// Duplicate attempt candidates fail closed, including contradictory terminal/live states.
func workflowTerminalLookup(in WorkflowInput) func(string, string, string, string, time.Time) (time.Time, runtimeobs.EvidenceSource, bool) {
	attempts := map[string]runtimeobs.Attempt{}
	ambiguous := map[string]bool{}
	for _, a := range in.ExecutionObservations {
		if _, exists := attempts[a.AttemptID]; exists {
			ambiguous[a.AttemptID] = true
		}
		attempts[a.AttemptID] = a
	}
	return func(id, actor, native, assignment string, start time.Time) (time.Time, runtimeobs.EvidenceSource, bool) {
		a, ok := attempts[id]
		invalid := func() (time.Time, runtimeobs.EvidenceSource, bool) { return time.Time{}, "", false }
		if !ok || id == "" || ambiguous[id] || a.TaskID != in.TaskID || a.BindingState != runtimeobs.BindingBound || !a.Terminal || a.RuntimeAgentID == "" || a.SessionID == "" || a.Provider == "" || !runtimeobs.HasObservedExecution(a) {
			return invalid()
		}
		if (actor != "" && actor != a.AgentPath) || (native != "" && native != a.RuntimeAgentID) || (assignment != "" && assignment != a.BindingEvidence["assignment_id"]) {
			return invalid()
		}
		if boundNative := a.BindingEvidence["runtime_agent_id"]; boundNative != "" && boundNative != a.RuntimeAgentID {
			return invalid()
		}
		switch a.CurrentState {
		case runtimeobs.StateCompleted, runtimeobs.StateErrored, runtimeobs.StateInterrupted, runtimeobs.StateShutdown:
		default:
			return invalid()
		}
		end, valid := workflowTime(a.EndedAt)
		if !valid || end.Before(start) || end.After(in.Now) {
			return invalid()
		}
		if a.StateEvidenceSource == runtimeobs.EvidenceT3 {
			r := a.NativeReceipt
			if r == nil || r.ThreadID == "" || r.RunID == "" {
				return invalid()
			}
			completed, valid := workflowTime(r.CompletedAt)
			// Folded EndedAt is the observation boundary, not native CompletedAt.
			// Preserve that clock: do not backdate an Activity or invent phase time.
			observed, observedOK := workflowTime(r.ObservedAt)
			if !valid || !observedOK || completed.After(observed) || completed.After(end) || observed.After(in.Now) {
				return invalid()
			}
		}
		return end, a.StateEvidenceSource, true
	}
}
