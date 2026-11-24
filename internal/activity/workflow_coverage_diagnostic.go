package activity

import (
	"sort"
	"time"
)

// These are read-only accounting dimensions. State evidence is never an
// instrumentation success or a substitute for closed Activity.
type WorkflowParticipation struct {
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
	Basis  string    `json:"basis"`
	Status string    `json:"status"`
}
type WorkflowCoverageDiagnostic struct {
	Scope                    string                    `json:"scope"`
	StateCoveredMillis       int64                     `json:"state_covered_ms"`
	UnaccountedStateMillis   int64                     `json:"unaccounted_state_ms"`
	ClosedActivityWallMillis int64                     `json:"closed_activity_wall_ms"`
	InstrumentationStatus    string                    `json:"instrumentation_status"`
	Findings                 []WorkflowRecoveryFinding `json:"findings"`
}
type WorkflowRecoveryFinding struct {
	TaskID                 string                `json:"task_id"`
	ActorID                string                `json:"actor_id,omitempty"`
	AttemptID              string                `json:"attempt_id,omitempty"`
	OperationID            string                `json:"operation_id,omitempty"`
	Start                  time.Time             `json:"start"`
	End                    time.Time             `json:"end"`
	Status                 string                `json:"status"`
	LastSuccessfulBoundary *time.Time            `json:"last_successful_boundary,omitempty"`
	Cause                  string                `json:"cause"`
	Owner                  string                `json:"owner"`
	ResumeCondition        string                `json:"resume_condition"`
	Evidence               []WorkflowGapEvidence `json:"evidence"`
}

func participationGap(start, end time.Time, status string) WorkflowGap {
	return WorkflowGap{Start: start, End: end, Kind: "participation_boundary", Status: status, Evidence: []WorkflowGapEvidence{{Source: "participation_bounds", ObservedAt: start, Reason: "Outside observed participation bounds; absence or inactivity is not established"}}}
}
func workflowClipMeasurement(m *Measurement, in WorkflowInput, events []Event, axis WorkflowAxis) {
	m.RawAgentTimeMillis = in.Summary.AgentTimeMillis
	if axis.Start == nil || axis.End == nil {
		return
	}
	segments := []Segment{}
	for _, s := range in.Summary.Segments {
		// A closed zero-length Activity is measured evidence, even at either
		// task boundary. It contributes no time, but must not become absence.
		// Keep clipInterval strict for positive intervals and state coverage.
		if s.StartedAt.Equal(s.EndedAt) && !s.StartedAt.Before(*axis.Start) && !s.EndedAt.After(*axis.End) {
			s.DurationMillis = 0
			segments = append(segments, s)
			continue
		}
		if x, ok := clipInterval(s.StartedAt, s.EndedAt, *axis.Start, *axis.End); ok {
			s.StartedAt = x.start
			s.EndedAt = x.end
			s.DurationMillis = x.end.Sub(x.start).Milliseconds()
			segments = append(segments, s)
		}
	}
	m.Activity = metric(events, segments, func(Role, Type) bool { return true })
	m.Worker = metric(events, segments, func(r Role, _ Type) bool { return r == RoleWorker })
	m.Controller = metric(events, segments, func(r Role, _ Type) bool { return r == RoleController || r == RoleSubController })
	m.Registrar = metric(events, segments, func(r Role, _ Type) bool { return r == RoleRegistrar })
	m.Review = metric(events, segments, func(r Role, t Type) bool {
		return r == RoleReviewer || t == ControllerReview || t == ControllerReReview
	})
	m.Tests = metric(events, segments, func(_ Role, t Type) bool { return t == TargetedTest || t == FullRegression || t == BuildCheck })
	m.AgentTimeMillis = 0
	for _, s := range segments {
		m.AgentTimeMillis += s.DurationMillis
	}
	m.OutsideTaskMillis = m.RawAgentTimeMillis - m.AgentTimeMillis
}
func workflowDiagnoseCoverage(w *Workflow) {
	d := WorkflowCoverageDiagnostic{Scope: "task_axis_state_evidence_separate_from_closed_activity", ClosedActivityWallMillis: w.Measurement.ObservedWallMillis, InstrumentationStatus: "not_established", Findings: []WorkflowRecoveryFinding{}}
	if w.Axis.Start == nil || w.Axis.End == nil || w.Axis.ElapsedMillis == nil {
		w.Coverage = d
		return
	}
	for i := range w.Rows {
		r := &w.Rows[i]
		var ms int64
		has := false
		for _, v := range r.Intervals {
			if v.Measured && v.End != nil {
				has = true
				if x, ok := clipInterval(v.Start, *v.End, *w.Axis.Start, *w.Axis.End); ok {
					ms += x.end.Sub(x.start).Milliseconds()
				}
			}
		}
		if has {
			r.ObservedActivityMillis = &ms
		}
	}
	known := []timingInterval{}
	for _, r := range w.Rows {
		for _, v := range r.Intervals {
			if v.End != nil && (v.Measured || v.Kind == "operation_execution") {
				if x, ok := clipInterval(v.Start, *v.End, *w.Axis.Start, *w.Axis.End); ok {
					known = append(known, x)
				}
			}
		}
	}
	for _, g := range w.Unmeasured {
		if g.Status == "confirmed_wait" || g.Status == "native_observed" {
			if x, ok := clipInterval(g.Start, g.End, *w.Axis.Start, *w.Axis.End); ok {
				known = append(known, x)
			}
		}
	}
	sort.Slice(known, func(i, j int) bool { return known[i].start.Before(known[j].start) })
	d.StateCoveredMillis = unionDuration(known)
	d.UnaccountedStateMillis = *w.Axis.ElapsedMillis - d.StateCoveredMillis
	if d.UnaccountedStateMillis < 0 {
		d.UnaccountedStateMillis = 0
	}
	// Findings retain sub-millisecond boundaries, including historical 491.949ms.
	for _, r := range w.Rows {
		for _, g := range r.Gaps {
			if g.Status == "before_participation" || g.Status == "after_participation" || g.Status == "confirmed_wait" {
				continue
			}
			f := WorkflowRecoveryFinding{TaskID: w.TaskID, ActorID: r.AgentID, Start: g.Start, End: g.End, Status: g.Status, Cause: "unknown", Owner: r.Role, ResumeCondition: "Report the next actual phase with exact task/actor/attempt; retry acknowledged collection failure without backfilling historical Activity", Evidence: g.Evidence}
			for _, e := range g.Evidence {
				if e.AttemptID != "" {
					f.AttemptID = e.AttemptID
				}
				if g.Status == "collection_failed" && e.Source != "activity_ledger" {
					f.Cause = e.Reason
				}
			}
			for _, v := range r.Intervals {
				if v.End != nil && !v.End.After(g.Start) && (f.LastSuccessfulBoundary == nil || v.End.After(*f.LastSuccessfulBoundary)) {
					at := *v.End
					f.LastSuccessfulBoundary = &at
				}
			}
			d.Findings = append(d.Findings, f)
		}
	}
	// Task gaps may precede every known actor. Keep them explicit and unattributed.
	for _, g := range w.Unmeasured {
		if g.Status == "unmeasured" || g.Status == "collection_failed" {
			d.Findings = append(d.Findings, WorkflowRecoveryFinding{TaskID: w.TaskID, Start: g.Start, End: g.End, Status: g.Status, Cause: "unknown", Owner: "controller", ResumeCondition: "Collect exact source/actor boundary; historical cause remains unknown until demonstrated", Evidence: g.Evidence})
		}
	}
	w.Coverage = d
}
