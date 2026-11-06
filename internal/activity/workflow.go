package activity

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// Workflow is an additive read model. These observations never write Activity
// events, assign work, authorize execution or reconcile the Control Tower.
type Workflow struct {
	Version     int               `json:"version"`
	TaskID      string            `json:"task_id"`
	ComputedAt  time.Time         `json:"computed_at"`
	Axis        WorkflowAxis      `json:"axis"`
	Lifecycle   LifecycleCoverage `json:"lifecycle"`
	Measurement Measurement       `json:"measurement"`
	Current     WorkflowCurrent   `json:"current"`
	Rounds      []WorkRound       `json:"rounds"`
	Rows        []WorkflowRow     `json:"rows"`
	Unmeasured  []WorkflowGap     `json:"unmeasured_intervals"`
	Reporting   ReportingCoverage `json:"reporting"`
	Snapshot    map[string]any    `json:"control_snapshot"`
}
type WorkflowAxis struct {
	Start          *time.Time `json:"start,omitempty"`
	End            *time.Time `json:"end,omitempty"`
	Basis          string     `json:"basis"`
	BoundaryStatus string     `json:"boundary_status"`
	ElapsedMillis  *int64     `json:"elapsed_ms,omitempty"`
	Open           bool       `json:"open"`
}
type MeasuredMetric struct {
	Status       string `json:"status"`
	Millis       *int64 `json:"duration_ms,omitempty"`
	EventCount   int    `json:"event_count"`
	SegmentCount int    `json:"segment_count"`
}
type Measurement struct {
	Status             string         `json:"status"`
	Activity           MeasuredMetric `json:"activity"`
	Worker             MeasuredMetric `json:"worker"`
	Controller         MeasuredMetric `json:"controller"`
	Registrar          MeasuredMetric `json:"registrar"`
	Review             MeasuredMetric `json:"review"`
	Tests              MeasuredMetric `json:"tests"`
	ObservedWallMillis int64          `json:"observed_wall_ms"`
	AgentTimeMillis    int64          `json:"agent_time_ms"`
	UnmeasuredMillis   *int64         `json:"unmeasured_ms,omitempty"`
	CoveragePercent    *float64       `json:"coverage_percent,omitempty"`
}
type WorkflowCurrent struct {
	Phase           string `json:"phase"`
	Basis           string `json:"basis"`
	ProcessingRole  string `json:"processing_role,omitempty"`
	ProcessingAgent string `json:"processing_agent,omitempty"`
	AssignedWorker  string `json:"assigned_worker,omitempty"`
	NextAction      string `json:"next_action"`
	Round           *int   `json:"round,omitempty"`
	RoundStatus     string `json:"round_status"`
	HandoffID       string `json:"handoff_id,omitempty"`
}
type WorkRound struct {
	Number              int       `json:"number"`
	Kind                string    `json:"kind"`
	StartedAt           time.Time `json:"started_at"`
	Basis               string    `json:"basis"`
	Reason              string    `json:"reason,omitempty"`
	RejectionEventID    string    `json:"rejection_event_id,omitempty"`
	HandoffID           string    `json:"handoff_id,omitempty"`
	PreviousRound       *int      `json:"previous_round,omitempty"`
	AssignmentIDs       []string  `json:"assignment_ids"`
	AttemptIDs          []string  `json:"attempt_ids"`
	HandoffIDs          []string  `json:"handoff_ids"`
	Commits             []string  `json:"commits"`
	VerificationResults []string  `json:"verification_results"`
}
type WorkflowNameEvidence struct {
	AttemptID string `json:"attempt_id"`
	AgentPath string `json:"agent_path"`
	Status    string `json:"status"`
}

type WorkflowRow struct {
	NameEvidence           []WorkflowNameEvidence `json:"name_evidence,omitempty"`
	DisplayName            string                 `json:"display_name,omitempty"`
	DisplayNameSource      string                 `json:"display_name_source,omitempty"`
	ObservedActivityMillis *int64                 `json:"observed_activity_ms,omitempty"`
	ID                     string                 `json:"id"`
	Role                   string                 `json:"role"`
	AgentID                string                 `json:"agent_id"`
	Round                  *int                   `json:"round,omitempty"`
	RoundStatus            string                 `json:"round_status"`
	Intervals              []WorkflowInterval     `json:"intervals"`
}
type WorkflowInterval struct {
	EndEvidence    string     `json:"end_evidence,omitempty"`
	ID             string     `json:"id"`
	Kind           string     `json:"kind"`
	Type           string     `json:"type"`
	Start          time.Time  `json:"start"`
	End            *time.Time `json:"end,omitempty"`
	Measured       bool       `json:"measured"`
	DurationMillis *int64     `json:"duration_ms,omitempty"`
	AttemptID      string     `json:"attempt_id,omitempty"`
	AssignmentID   string     `json:"assignment_id,omitempty"`
	HandoffID      string     `json:"handoff_id,omitempty"`
	Evidence       string     `json:"evidence,omitempty"`
	Outcome        string     `json:"outcome,omitempty"`
	Lane           int        `json:"lane"`
}
type WorkflowGap struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Kind  string    `json:"kind"`
}
type ReportingCoverage struct {
	Status                    string             `json:"status"`
	ExpectedExecutions        int                `json:"expected_executions"`
	ExecutionsWithoutActivity int                `json:"executions_without_activity"`
	ExpectedReviews           int                `json:"expected_reviews"`
	ReviewsWithoutActivity    int                `json:"reviews_without_activity"`
	Findings                  []ReportingFinding `json:"findings"`
}
type ReportingFinding struct {
	Code          string `json:"code"`
	AttemptID     string `json:"attempt_id,omitempty"`
	AssignmentID  string `json:"assignment_id,omitempty"`
	HandoffID     string `json:"handoff_id,omitempty"`
	RequiredPhase string `json:"required_phase,omitempty"`
}
type WorkflowInput struct {
	ExecutionObservations []runtimeobs.Attempt
	TaskID                string
	Events                []Event
	Summary               Summary
	Facts                 []LifecycleFact
	Handoffs              map[string]handoffjournal.Handoff
	Control               map[string]any
	Now                   time.Time
}
type reviewBoundary struct {
	at                         time.Time
	id, handoff, reason, basis string
}

func metric(events []Event, segments []Segment, accept func(Role, Type) bool) MeasuredMetric {
	out := MeasuredMetric{Status: "not_recorded"}
	for _, e := range events {
		if accept(e.Role, e.Type) {
			out.EventCount++
		}
	}
	var ms int64
	for _, s := range segments {
		if accept(s.Role, s.Type) {
			out.SegmentCount++
			ms += s.DurationMillis
		}
	}
	if out.SegmentCount > 0 {
		out.Millis = &ms
		out.Status = "observed"
	} else if out.EventCount > 0 {
		out.Status = "partial"
	}
	return out
}
func workflowTime(s string) (time.Time, bool) {
	t, e := time.Parse(time.RFC3339Nano, s)
	return t, e == nil
}
func appendUnique(values []string, s string) []string {
	if s == "" {
		return values
	}
	for _, v := range values {
		if v == s {
			return values
		}
	}
	return append(values, s)
}
func roundNumber(at time.Time, bounds []reviewBoundary, known bool) *int {
	if !known {
		return nil
	}
	n := 0
	for _, b := range bounds {
		if b.at.After(at) {
			break
		}
		n++
	}
	return &n
}
func roundStatus(n *int) string {
	if n == nil {
		return "unclassified"
	}
	if *n == 0 {
		return "initial"
	}
	return "rework"
}

func SummarizeWorkflow(in WorkflowInput) Workflow {
	if in.Now.IsZero() {
		in.Now = time.Now().UTC()
	}
	events, _ := EffectiveEvents(in.Events)
	filtered := events[:0]
	for _, e := range events {
		if e.TaskID == in.TaskID {
			filtered = append(filtered, e)
		}
	}
	events = filtered
	w := Workflow{Version: 1, TaskID: in.TaskID, ComputedAt: in.Now, Rows: []WorkflowRow{}, Rounds: []WorkRound{}, Unmeasured: []WorkflowGap{}, Snapshot: map[string]any{}, Reporting: ReportingCoverage{Status: "not_expected", Findings: []ReportingFinding{}}}
	w.Lifecycle = SummarizeLifecycle(in.TaskID, in.Facts, in.Summary.Segments)
	w.Axis = WorkflowAxis{Start: w.Lifecycle.RegisteredAt, End: w.Lifecycle.CompletedAt, Basis: "registered_completed", BoundaryStatus: "complete"}
	if w.Axis.Start == nil {
		w.Axis.Start = in.Summary.FirstObservedAt
		w.Axis.Basis = "activity_only_incomplete"
		w.Axis.BoundaryStatus = "registration_missing"
	}
	if w.Axis.End == nil && w.Axis.Start != nil {
		now := in.Now
		w.Axis.End = &now
		w.Axis.Open = true
		w.Axis.Basis = "registered_current"
		if w.Lifecycle.RegisteredAt == nil {
			w.Axis.Basis = "activity_only_incomplete"
		}
	}
	if w.Axis.Start == nil {
		w.Axis.BoundaryStatus = "unknown"
		w.Axis.Basis = "unknown"
	}
	if w.Axis.Start != nil && w.Axis.End != nil {
		if w.Axis.End.Before(*w.Axis.Start) {
			w.Axis.BoundaryStatus = "conflict"
		} else {
			ms := w.Axis.End.Sub(*w.Axis.Start).Milliseconds()
			w.Axis.ElapsedMillis = &ms
		}
	}
	m := Measurement{Activity: metric(events, in.Summary.Segments, func(Role, Type) bool { return true }), Worker: metric(events, in.Summary.Segments, func(r Role, _ Type) bool { return r == RoleWorker }), Controller: metric(events, in.Summary.Segments, func(r Role, _ Type) bool { return r == RoleController || r == RoleSubController }), Registrar: metric(events, in.Summary.Segments, func(r Role, _ Type) bool { return r == RoleRegistrar }), Review: metric(events, in.Summary.Segments, func(_ Role, t Type) bool { return t == ControllerReview || t == ControllerReReview }), Tests: metric(events, in.Summary.Segments, func(_ Role, t Type) bool { return t == TargetedTest || t == FullRegression || t == BuildCheck }), AgentTimeMillis: in.Summary.AgentTimeMillis}
	m.Status = m.Activity.Status
	measured := []timingInterval{}
	if w.Axis.ElapsedMillis != nil {
		for _, s := range in.Summary.Segments {
			if v, ok := clipInterval(s.StartedAt, s.EndedAt, *w.Axis.Start, *w.Axis.End); ok {
				measured = append(measured, v)
			}
		}
		m.ObservedWallMillis = unionDuration(measured)
		unknown := *w.Axis.ElapsedMillis - m.ObservedWallMillis
		m.UnmeasuredMillis = &unknown
		if *w.Axis.ElapsedMillis > 0 {
			pct := float64(m.ObservedWallMillis) * 100 / float64(*w.Axis.ElapsedMillis)
			m.CoveragePercent = &pct
		}
		if m.Activity.EventCount > 0 && unknown > 0 {
			m.Status = "partial"
		}
		sort.Slice(measured, func(i, j int) bool { return measured[i].start.Before(measured[j].start) })
		cursor := *w.Axis.Start
		for _, v := range measured {
			if v.start.After(cursor) {
				w.Unmeasured = append(w.Unmeasured, WorkflowGap{cursor, v.start, "activity_unmeasured"})
			}
			if v.end.After(cursor) {
				cursor = v.end
			}
		}
		if cursor.Before(*w.Axis.End) {
			w.Unmeasured = append(w.Unmeasured, WorkflowGap{cursor, *w.Axis.End, "activity_unmeasured"})
		}
	}
	if in.Summary.InvalidTransitions+in.Summary.InvalidCorrections+w.Lifecycle.InvalidFacts > 0 || w.Axis.BoundaryStatus == "conflict" {
		m.Status = "conflict"
	}
	w.Measurement = m
	boundaries := reviewBoundaries(in.TaskID, events, in.Handoffs)
	handoffs := []handoffjournal.Handoff{}
	for _, h := range in.Handoffs {
		if h.TaskID == in.TaskID {
			handoffs = append(handoffs, h)
		}
	}
	sort.Slice(handoffs, func(i, j int) bool { return handoffs[i].PreparedAt < handoffs[j].PreparedAt })
	known := w.Lifecycle.RegisteredAt != nil || len(events) > 0
	if known {
		start := in.Now
		if w.Axis.Start != nil {
			start = *w.Axis.Start
		}
		w.Rounds = append(w.Rounds, WorkRound{Number: 0, Kind: "initial", StartedAt: start, Basis: "registered_or_reported_activity", AssignmentIDs: []string{}, AttemptIDs: []string{}, HandoffIDs: []string{}, Commits: []string{}, VerificationResults: []string{}})
		for i, b := range boundaries {
			prev := i
			w.Rounds = append(w.Rounds, WorkRound{Number: i + 1, Kind: "rework", StartedAt: b.at, Basis: b.basis, Reason: b.reason, RejectionEventID: b.id, HandoffID: b.handoff, PreviousRound: &prev, AssignmentIDs: []string{}, AttemptIDs: []string{}, HandoffIDs: []string{}, Commits: []string{}, VerificationResults: []string{}})
		}
	}
	rows := map[string]*WorkflowRow{}
	add := func(role, agent string, n *int, v WorkflowInterval) {
		label := "unknown"
		if n != nil {
			label = fmt.Sprint(*n)
		}
		key := role + "|" + agent + "|" + label
		row := rows[key]
		if row == nil {
			row = &WorkflowRow{ID: key, Role: role, AgentID: agent, Round: n, RoundStatus: roundStatus(n), Intervals: []WorkflowInterval{}}
			rows[key] = row
		}
		row.Intervals = append(row.Intervals, v)
		if v.Measured && v.DurationMillis != nil {
			if row.ObservedActivityMillis == nil {
				ms := int64(0)
				row.ObservedActivityMillis = &ms
			}
			*row.ObservedActivityMillis += *v.DurationMillis
		}
		if n != nil && *n < len(w.Rounds) {
			r := &w.Rounds[*n]
			r.AttemptIDs = appendUnique(r.AttemptIDs, v.AttemptID)
			r.AssignmentIDs = appendUnique(r.AssignmentIDs, v.AssignmentID)
			r.HandoffIDs = appendUnique(r.HandoffIDs, v.HandoffID)
		}
	}
	nativeByAttempt := map[string]string{}
	eventByStart := map[string]Event{}
	reviewOutcomeByEnd := map[string]string{}
	eventByActor := map[string][]Event{}
	eventByAttempt := map[string]int{}
	eventByHandoff := map[string]int{}
	for _, e := range events {
		if e.Action == Stop && e.ReviewOutcome != "" {
			reviewOutcomeByEnd[actorKey(e)+"|"+e.OccurredAt.Format(time.RFC3339Nano)] = e.ReviewOutcome
		}
		eventByAttempt[e.RuntimeAttemptID]++
		if e.Role == RoleWorker {
			nativeByAttempt[e.RuntimeAttemptID] = e.AgentID
		}
		if e.HandoffID != "" {
			if e.Role == RoleController {
				eventByHandoff[e.HandoffID+"|"+e.RuntimeAttemptID+"|"+e.AgentID]++
			}
		}
		eventByActor[actorKey(e)] = append(eventByActor[actorKey(e)], e)
		if e.Action == Start || e.Action == Switch {
			eventByStart[actorKey(e)+"|"+e.OccurredAt.Format(time.RFC3339Nano)] = e
		}
	}
	for _, s := range in.Summary.Segments {
		n := roundNumber(s.StartedAt, boundaries, known)
		end, ms := s.EndedAt, s.DurationMillis
		v := WorkflowInterval{ID: s.AttemptID + "|" + s.StartedAt.Format(time.RFC3339Nano), Kind: "activity", Type: string(s.Type), Start: s.StartedAt, End: &end, Measured: true, DurationMillis: &ms, AttemptID: s.AttemptID}
		if e, ok := eventByStart[string(s.Role)+"\x00"+s.AgentID+"\x00"+s.AttemptID+"|"+s.StartedAt.Format(time.RFC3339Nano)]; ok {
			v.ID = e.EventID
			v.AssignmentID = e.AssignmentID
			v.HandoffID = e.HandoffID
			v.Evidence = e.Evidence
		}

		if s.RunEvidence != nil {
			v.Outcome = s.RunEvidence.Result
			if n != nil && *n < len(w.Rounds) {
				r := &w.Rounds[*n]
				r.Commits = appendUnique(r.Commits, s.RunEvidence.GitSHA)
				r.VerificationResults = appendUnique(r.VerificationResults, s.RunEvidence.Result)
			}
		}
		if outcome := reviewOutcomeByEnd[string(s.Role)+"\x00"+s.AgentID+"\x00"+s.AttemptID+"|"+s.EndedAt.Format(time.RFC3339Nano)]; outcome != "" {
			v.Outcome = outcome
		}
		add(string(s.Role), s.AgentID, n, v)
	}
	latestWorker := (*Event)(nil)
	latestOpen := (*Event)(nil)
	for _, history := range eventByActor {
		if len(history) == 0 {
			continue
		}
		latest := history[0]
		for _, e := range history[1:] {
			if e.OccurredAt.After(latest.OccurredAt) || (e.OccurredAt.Equal(latest.OccurredAt) && (e.RecordedAt.After(latest.RecordedAt) || (e.RecordedAt.Equal(latest.RecordedAt) && e.EventID > latest.EventID))) {
				latest = e
			}
		}
		if latest.Action != Start && latest.Action != Switch {
			continue
		}
		if latestOpen == nil || latest.OccurredAt.After(latestOpen.OccurredAt) {
			copy := latest
			latestOpen = &copy
		}
		add(string(latest.Role), latest.AgentID, roundNumber(latest.OccurredAt, boundaries, known), WorkflowInterval{ID: latest.EventID, Kind: "activity_open", Type: string(latest.Type), Start: latest.OccurredAt, AttemptID: latest.RuntimeAttemptID, AssignmentID: latest.AssignmentID, HandoffID: latest.HandoffID, Evidence: latest.Evidence})
		if latest.Role == RoleWorker && (latestWorker == nil || latest.OccurredAt.After(latestWorker.OccurredAt)) {
			copy := latest
			latestWorker = &copy
		}
	}
	facts := []LifecycleFact{}
	for _, f := range in.Facts {
		if f.TaskID == in.TaskID {
			facts = append(facts, f)
		}
	}
	sort.Slice(facts, func(i, j int) bool { return facts[i].OccurredAt.Before(facts[j].OccurredAt) })
	type executionBoundary struct {
		at    time.Time
		basis string
	}
	reportsByAttempt := map[string][]executionBoundary{}
	for _, h := range handoffs {
		if h.SourceAttemptID == "" || (h.EventType != handoffjournal.EventWorkerDone && h.EventType != handoffjournal.EventWorkerBlocked) {
			continue
		}
		if at, ok := workflowTime(h.PreparedAt); ok && !at.After(in.Now) {
			reportsByAttempt[h.SourceAttemptID] = append(reportsByAttempt[h.SourceAttemptID], executionBoundary{at, string(h.EventType) + "_report"})
		}
	}
	terminalByAttempt := map[string]runtimeobs.Attempt{}
	ambiguousAttempts := map[string]bool{}
	for _, a := range in.ExecutionObservations {
		if _, ok := terminalByAttempt[a.AttemptID]; ok {
			ambiguousAttempts[a.AttemptID] = true
		}
		terminalByAttempt[a.AttemptID] = a
	}
	seenExec := map[string]bool{}
	for _, f := range facts {
		if f.TaskID != in.TaskID || (f.Kind != "started" && f.Kind != "resumed") {
			continue
		}
		key := f.AttemptID
		if key == "" {
			key = f.EventID
		}
		if !seenExec[key] {
			seenExec[key] = true
			w.Reporting.ExpectedExecutions++
			if f.AttemptID == "" || eventByAttempt[f.AttemptID] == 0 {
				w.Reporting.ExecutionsWithoutActivity++
				w.Reporting.Findings = append(w.Reporting.Findings, ReportingFinding{Code: "execution_activity_missing", AttemptID: f.AttemptID, AssignmentID: f.AssignmentID, RequiredPhase: "implementation_started"})
			}
		}
		var end *time.Time
		endEvidence := "unobserved"
		reports := reportsByAttempt[f.AttemptID]
		j := sort.Search(len(reports), func(j int) bool { return !reports[j].at.Before(f.OccurredAt) })
		if j < len(reports) {
			at := reports[j].at
			end = &at
			endEvidence = reports[j].basis
		}
		if a, ok := terminalByAttempt[f.AttemptID]; ok && !ambiguousAttempts[f.AttemptID] && a.TaskID == in.TaskID && a.BindingState == runtimeobs.BindingBound && a.Terminal && a.RuntimeAgentID != "" && a.SessionID != "" && a.Provider != "" && runtimeobs.HasObservedExecution(a) && (a.CurrentState == runtimeobs.StateCompleted || a.CurrentState == runtimeobs.StateErrored || a.CurrentState == runtimeobs.StateInterrupted || a.CurrentState == runtimeobs.StateShutdown) && (f.Actor == "" || f.Actor == a.AgentPath) {
			if at, ok := workflowTime(a.EndedAt); ok && !at.Before(f.OccurredAt) && !at.After(in.Now) && (end == nil || at.Before(*end)) {
				end = &at
				endEvidence = "native_terminal"
			}
		}

		// These are reported execution boundaries, not measurements of active work.
		agent := f.Actor
		if agent == "/root/controller" || agent == "/root/registrar" {
			agent = "unclassified_worker"
		}
		if native := nativeByAttempt[f.AttemptID]; native != "" {
			agent = native
		}
		add("worker", agent, roundNumber(f.OccurredAt, boundaries, known), WorkflowInterval{ID: f.EventID, Kind: "execution_report", Type: "unclassified_execution", Start: f.OccurredAt, End: end, EndEvidence: endEvidence, AttemptID: f.AttemptID, AssignmentID: f.AssignmentID, Evidence: f.EvidenceSource})
	}
	for _, h := range handoffs {
		claimed, ok := workflowTime(h.ClaimedAt)
		if !ok || h.ClaimedBy != "/root/controller" || (h.EventType != handoffjournal.EventWorkerDone && h.EventType != handoffjournal.EventWorkerBlocked && h.EventType != handoffjournal.EventCompletionReport) {
			continue
		}
		w.Reporting.ExpectedReviews++
		if eventByHandoff[h.HandoffID+"|"+h.ClaimedAttemptID+"|"+h.Target.RuntimeAgentID] == 0 {
			w.Reporting.ReviewsWithoutActivity++
			w.Reporting.Findings = append(w.Reporting.Findings, ReportingFinding{Code: "review_activity_missing", AttemptID: h.ClaimedAttemptID, HandoffID: h.HandoffID, RequiredPhase: "review_started"})
		}
		var end *time.Time
		outcome := ""
		if step, ok := h.Steps["acceptance"]; ok {
			if at, ok := workflowTime(step.ObservedAt); ok && !at.Before(claimed) {
				end = &at
				outcome = step.Result
			}
		}
		add("controller", h.Target.RuntimeAgentID, roundNumber(claimed, boundaries, known), WorkflowInterval{ID: h.HandoffID, Kind: "handoff_review", Type: "unclassified_review", Start: claimed, End: end, AttemptID: h.ClaimedAttemptID, HandoffID: h.HandoffID, Outcome: outcome, Evidence: "claimed_handoff"})
	}
	if w.Reporting.ExpectedExecutions+w.Reporting.ExpectedReviews > 0 {
		w.Reporting.Status = "recorded"
		if w.Reporting.ExecutionsWithoutActivity+w.Reporting.ReviewsWithoutActivity > 0 {
			w.Reporting.Status = "missing"
		}
	}
	for _, row := range rows {
		// Names require an exact, unambiguous task/attempt/native identity binding.
		// Never derive aliases from Activity UUIDs or unbound paths.
		name := ""
		conflict := false
		namedAttempts := map[string]bool{}
		for _, interval := range row.Intervals {
			if interval.AttemptID == "" {
				continue
			}
			a, ok := terminalByAttempt[interval.AttemptID]
			if !ok || ambiguousAttempts[interval.AttemptID] || a.TaskID != in.TaskID || a.BindingState != runtimeobs.BindingBound || a.RuntimeAgentID == "" || a.AgentPath == "" || (row.AgentID != a.RuntimeAgentID && row.AgentID != a.AgentPath) {
				continue
			}
			candidate := workflowReadableAgentName(a.AgentPath)
			if !namedAttempts[a.AttemptID] {
				status := "readable_bound_runtime_path"
				if candidate == "" {
					status = "opaque_or_unreadable_runtime_path_leaf"
				}
				row.NameEvidence = append(row.NameEvidence, WorkflowNameEvidence{AttemptID: a.AttemptID, AgentPath: a.AgentPath, Status: status})
				namedAttempts[a.AttemptID] = true
			}
			if candidate == "" {
				continue
			}
			if name != "" && name != candidate {
				conflict = true
			}
			name = candidate
		}
		if !conflict && name != "" {
			row.DisplayName = name
			row.DisplayNameSource = "bound_runtime_agent_path"
		}
		sort.SliceStable(row.Intervals, func(i, j int) bool { return row.Intervals[i].Start.Before(row.Intervals[j].Start) })
		lanes := []time.Time{}
		for i := range row.Intervals {
			v := &row.Intervals[i]
			lane := 0
			for lane < len(lanes) && lanes[lane].After(v.Start) {
				lane++
			}
			end := v.Start
			if v.Kind == "activity_open" {
				end = in.Now
			}
			if v.End != nil {
				end = *v.End
			}
			if lane == len(lanes) {
				lanes = append(lanes, end)
			} else {
				lanes[lane] = end
			}
			v.Lane = lane
		}
		w.Rows = append(w.Rows, *row)
	}
	sort.Slice(w.Rows, func(i, j int) bool { return w.Rows[i].ID < w.Rows[j].ID })
	w.Current = WorkflowCurrent{Phase: "unknown", Basis: "insufficient_evidence", NextAction: "verify_execution_evidence", RoundStatus: "unclassified"}
	if len(facts) > 0 {
		latest := facts[len(facts)-1]
		w.Current.Round = roundNumber(latest.OccurredAt, boundaries, known)
		w.Current.RoundStatus = roundStatus(w.Current.Round)
		for i := len(facts) - 1; i >= 0; i-- {
			if facts[i].Kind == "started" || facts[i].Kind == "resumed" {
				w.Current.AssignedWorker = facts[i].Actor
				break
			}
		}
		switch latest.Kind {
		case "registered":
			w.Current.Phase = "queued"
			w.Current.NextAction = "assign_worker"
		case "assigned":
			w.Current.Phase = "dispatch_wait"
			w.Current.NextAction = "confirm_worker_start"
		case "started", "resumed":
			w.Current.Phase = "implementation"
			w.Current.ProcessingRole = "worker"
			w.Current.ProcessingAgent = latest.Actor
			w.Current.NextAction = "implement_and_verify"
		case "waiting":
			w.Current.Phase = "wait_unclassified"
			w.Current.NextAction = "verify_wait_reason"
		case "completed":
			w.Current.Phase = "completed"
			w.Current.NextAction = "none"
		}
		w.Current.Basis = "reported_lifecycle"
	}
	for _, h := range handoffs {
		if h.Applied || h.ClaimedAt == "" || (h.EventType != handoffjournal.EventWorkerDone && h.EventType != handoffjournal.EventWorkerBlocked && h.EventType != handoffjournal.EventCompletionReport) {
			continue
		}
		w.Current.Phase = "controller_review"
		w.Current.ProcessingRole = "controller"
		w.Current.ProcessingAgent = h.Target.RuntimeAgentID
		w.Current.HandoffID = h.HandoffID
		w.Current.NextAction = "review_report"
		w.Current.Basis = "claimed_handoff"
		if step, ok := h.Steps["acceptance"]; ok {
			if step.Result == "ok" {
				w.Current.Phase = "finalizing"
				w.Current.NextAction = "finalize_backlog_and_external_result"
			} else if step.Result == "failed" {
				w.Current.Phase = "rework_requested"
				w.Current.NextAction = "dispatch_correction"
			}
		}
	}
	// A newer Worker start supersedes an older review outcome; it does not erase it.
	if len(facts) > 0 {
		f := facts[len(facts)-1]
		if f.Kind == "started" || f.Kind == "resumed" {
			if len(boundaries) > 0 && f.OccurredAt.After(boundaries[len(boundaries)-1].at) {
				w.Current.Phase = "rework"
				w.Current.ProcessingRole = "worker"
				w.Current.ProcessingAgent = f.Actor
				w.Current.NextAction = "implement_and_verify"
				w.Current.Basis = "reported_rework_execution"
			}
		}
	}
	if latestWorker != nil {
		newer := true
		for _, h := range handoffs {
			if at, ok := workflowTime(h.ClaimedAt); ok && at.After(latestWorker.OccurredAt) {
				newer = false
			}
			if step, ok := h.Steps["acceptance"]; ok {
				if at, ok := workflowTime(step.ObservedAt); ok && at.After(latestWorker.OccurredAt) {
					newer = false
				}
			}
		}
		if newer {
			w.Current.Round = roundNumber(latestWorker.OccurredAt, boundaries, known)
			w.Current.RoundStatus = roundStatus(w.Current.Round)
			w.Current.Phase = "implementation"
			if w.Current.Round != nil && *w.Current.Round > 0 {
				w.Current.Phase = "rework"
			}
			w.Current.Basis = "open_reported_activity"
			w.Current.ProcessingRole = "worker"
			w.Current.ProcessingAgent = latestWorker.AgentID
			w.Current.NextAction = "implement_and_verify"
		}
	}
	if snapshot, ok := in.Control["snapshot"].(map[string]any); ok {
		w.Snapshot = snapshot
	}
	if a, ok := in.Control["activity"].(map[string]any); ok {
		if review, ok := a["completion_review"].(map[string]any); ok {
			if state, ok := review["state"].(string); ok {
				switch state {
				case "needs_user":
					w.Current.Phase = "user_decision_wait"
					w.Current.NextAction = "user_decision"
				case "controller_recovery":
					w.Current.Phase = "recovery"
					w.Current.NextAction = "verify_execution_evidence"
				}
			}
		}
	}
	if state, _ := in.Control["file_state"].(string); state == "done" && w.Lifecycle.CompletedAt != nil {
		w.Current.Phase = "completed"
		w.Current.NextAction = "none"
	}
	// Explicit wait reports can distinguish CI from user decisions. Generic waits
	// remain unclassified rather than being relabelled as a user hold.
	if latestOpen != nil && w.Current.Phase != "completed" {
		e := latestOpen
		if e.PhaseEvent == "external_ci_wait_started" || strings.EqualFold(e.ProposedType, "external_ci_wait") {
			w.Current.Phase = "external_ci_wait"
			w.Current.Basis = "explicit_activity_report"
			w.Current.NextAction = "await_ci_result"
		}
	}

	return w
}

// SuggestedRoundFromEvidence is shared by recording and the read model. A new
// native attempt or an activity switch never creates a rework boundary.
func SuggestedRoundFromEvidence(task string, events []Event, handoffs map[string]handoffjournal.Handoff) int {
	return len(reviewBoundaries(task, events, handoffs))
}
func reviewBoundaries(task string, events []Event, handoffs map[string]handoffjournal.Handoff) []reviewBoundary {
	events, _ = EffectiveEvents(events)
	out := []reviewBoundary{}
	seen := map[string]bool{}
	reviewed := map[string]bool{}
	for _, e := range events {
		if e.TaskID != task || e.Role != RoleController || e.Action != Stop || (e.Type != ControllerReview && e.Type != ControllerReReview) || seen[e.EventID] {
			continue
		}
		seen[e.EventID] = true
		if e.HandoffID != "" {
			reviewed[e.HandoffID] = true
		}
		if e.ReviewOutcome == "rejected" || e.ReviewOutcome == "needs_changes" {
			out = append(out, reviewBoundary{e.OccurredAt, e.EventID, e.HandoffID, e.Evidence, "activity_review_outcome"})
		}
	}
	for _, h := range handoffs {
		if h.TaskID != task || reviewed[h.HandoffID] {
			continue
		}
		if step, ok := h.Steps["acceptance"]; ok && step.Result == "failed" {
			if at, ok := workflowTime(step.ObservedAt); ok {
				out = append(out, reviewBoundary{at, "", h.HandoffID, step.Evidence, "handoff_acceptance_failed"})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].at.Equal(out[j].at) {
			return out[i].id+out[i].handoff < out[j].id+out[j].handoff
		}
		return out[i].at.Before(out[j].at)
	})
	return out
}

// Exact binding proves identity, not that an opaque ID leaf is a human name.
// Reject generic ID-derived aliases without manufacturing a replacement alias.
func workflowReadableAgentName(agentPath string) string {
	parts := strings.Split(strings.Trim(agentPath, "/"), "/")
	candidate := parts[len(parts)-1]
	if candidate == "" || candidate == "root" || strings.ContainsAny(candidate, " \t\n") || len(candidate) > 32 || strings.Count(candidate, "-") >= 4 {
		return ""
	}
	lower := strings.ToLower(candidate)
	opaque := func(token string) bool {
		digits, hex := true, true
		hasDigit := false
		for _, c := range token {
			if c < '0' || c > '9' {
				digits = false
			} else {
				hasDigit = true
			}
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				hex = false
			}
		}
		return token != "" && (digits || hex && (len(token) >= 6 || len(token) >= 4 && hasDigit))
	}
	if opaque(strings.NewReplacer("-", "", "_", "", ".", "", ":", "").Replace(lower)) {
		return ""
	}
	for _, token := range strings.FieldsFunc(lower, func(c rune) bool { return c == '-' || c == '_' || c == '.' || c == ':' }) {
		if len(token) >= 4 && opaque(token) {
			return ""
		}
	}
	for _, prefix := range []string{"agent", "worker", "controller", "registrar", "run", "attempt", "session", "thread", "native", "id", "uuid"} {
		if strings.HasPrefix(lower, prefix) && opaque(strings.TrimLeft(strings.TrimPrefix(lower, prefix), "-_.:")) {
			return ""
		}
	}
	return candidate
}
