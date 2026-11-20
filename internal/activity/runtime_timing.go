package activity

import (
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"time"
)

// RuntimeTiming is independent of Activity coverage and reported lifecycle
// waits. Persisted native receipts prove execution boundaries, never phases.
type RuntimeTiming struct {
	Source                    string                  `json:"source"`
	Status                    string                  `json:"status"`
	DurationMillis            *int64                  `json:"duration_ms,omitempty"`
	ReportedWaitOverlapMillis int64                   `json:"reported_wait_overlap_ms"`
	Intervals                 []RuntimeTimingInterval `json:"intervals"`
	Rejected                  int                     `json:"rejected"`
}
type RuntimeTimingInterval struct {
	AttemptID                string     `json:"attempt_id"`
	AgentID                  string     `json:"agent_id"`
	SessionID                string     `json:"session_id"`
	ThreadID                 string     `json:"t3_thread_id"`
	NativeStartedAt          time.Time  `json:"native_started_at"`
	NativeCompletedAt        *time.Time `json:"native_completed_at,omitempty"`
	ObservedAt               time.Time  `json:"observed_at"`
	FirstActivityStatus      string     `json:"first_activity_status"`
	FirstActivityAt          *time.Time `json:"first_activity_at,omitempty"`
	FirstActivityEventID     string     `json:"first_activity_event_id,omitempty"`
	FirstActivityDelayMillis *int64     `json:"first_activity_delay_ms,omitempty"`
}

func summarizeRuntimeTiming(in WorkflowInput, axis WorkflowAxis, lifecycle LifecycleCoverage) RuntimeTiming {
	out := RuntimeTiming{Source: "persisted_verified_t3_receipts", Status: "not_recorded", Intervals: []RuntimeTimingInterval{}}
	counts := map[string]int{}
	for _, a := range in.ExecutionObservations {
		counts[a.AttemptID]++
	}
	intervals := []timingInterval{}
	// Index effective phase starts once; identity and native-window checks below
	// remain separate from native duration and reported lifecycle waits.
	effective, _ := EffectiveEvents(in.Events)
	starts := map[string][]Event{}
	for _, e := range effective {
		if e.TaskID == in.TaskID && e.EventID != "" && (e.Action == Start || e.Action == Switch) {
			starts[actorKey(e)] = append(starts[actorKey(e)], e)
		}
	}
	for _, a := range in.ExecutionObservations {
		r := a.NativeReceipt
		if a.TaskID != in.TaskID {
			continue
		}
		if counts[a.AttemptID] != 1 || a.BindingState != runtimeobs.BindingBound || a.RuntimeAgentID == "" || a.SessionID == "" || !runtimeobs.HasObservedExecution(a) || a.StateEvidenceSource != runtimeobs.EvidenceT3 || r == nil || r.ThreadID == "" || r.ThreadID == a.SessionID || r.RunID == "" || r.NativeAttemptID == "" || r.SessionID != a.SessionID || a.RuntimeAgentID != a.SessionID || r.Provider != a.Provider || r.State != a.CurrentState {
			out.Rejected++
			continue
		}
		start, e1 := time.Parse(time.RFC3339Nano, r.StartedAt)
		observed, e2 := time.Parse(time.RFC3339Nano, r.ObservedAt)
		if e1 != nil || e2 != nil || start.After(observed) || observed.After(in.Now) {
			out.Rejected++
			continue
		}
		end := observed // Running snapshots establish only the observed prefix.
		var completed *time.Time
		if a.Terminal {
			v, err := time.Parse(time.RFC3339Nano, r.CompletedAt)
			if err != nil || v.Before(start) || v.After(observed) || r.State == runtimeobs.StateRunning {
				out.Rejected++
				continue
			}
			end = v
			completed = &v
		} else if r.State != runtimeobs.StateRunning || r.CompletedAt != "" {
			out.Rejected++
			continue
		}
		item := RuntimeTimingInterval{AttemptID: a.AttemptID, AgentID: a.RuntimeAgentID, SessionID: a.SessionID, ThreadID: r.ThreadID, NativeStartedAt: start, NativeCompletedAt: completed, ObservedAt: observed, FirstActivityStatus: "not_recorded"}
		role := Role(workflowActorRole(a.AgentPath))
		boundNative := a.BindingEvidence["runtime_agent_id"]
		if role.Valid() && (boundNative == "" || boundNative == a.RuntimeAgentID) {
			key := actorKey(Event{Role: role, AgentID: a.RuntimeAgentID, RuntimeAttemptID: a.AttemptID})
			for _, e := range starts[key] {
				if e.AssignmentID != a.BindingEvidence["assignment_id"] || e.OccurredAt.Before(start) || e.OccurredAt.After(in.Now) || e.RecordedAt.After(in.Now) || completed != nil && e.OccurredAt.After(*completed) || e.Provider != "" && e.Provider != a.Provider || e.SessionID != "" && e.SessionID != a.SessionID {
					continue
				}
				if item.FirstActivityAt != nil && (e.OccurredAt.After(*item.FirstActivityAt) || e.OccurredAt.Equal(*item.FirstActivityAt) && e.EventID >= item.FirstActivityEventID) {
					continue
				}
				at, delay := e.OccurredAt, e.OccurredAt.Sub(start).Milliseconds()
				item.FirstActivityStatus, item.FirstActivityAt, item.FirstActivityEventID, item.FirstActivityDelayMillis = "observed", &at, e.EventID, &delay
			}
		}
		out.Intervals = append(out.Intervals, item)
		if axis.Start != nil && axis.End != nil {
			if clipped, ok := clipInterval(start, end, *axis.Start, *axis.End); ok {
				intervals = append(intervals, clipped)
			}
		}
	}
	if len(out.Intervals) > 0 {
		out.Status = "observed"
		if axis.Start != nil && axis.End != nil {
			duration := unionDuration(intervals)
			out.DurationMillis = &duration
			overlaps := []timingInterval{}
			for _, r := range intervals {
				for _, w := range lifecycle.Intervals {
					if v, ok := clipInterval(r.start, r.end, w.StartedAt, w.EndedAt); ok {
						overlaps = append(overlaps, v)
					}
				}
			}
			out.ReportedWaitOverlapMillis = unionDuration(overlaps)
		}
	}
	return out
}
