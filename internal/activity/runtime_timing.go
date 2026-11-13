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
	AttemptID         string     `json:"attempt_id"`
	AgentID           string     `json:"agent_id"`
	SessionID         string     `json:"session_id"`
	ThreadID          string     `json:"t3_thread_id"`
	NativeStartedAt   time.Time  `json:"native_started_at"`
	NativeCompletedAt *time.Time `json:"native_completed_at,omitempty"`
	ObservedAt        time.Time  `json:"observed_at"`
}

func summarizeRuntimeTiming(in WorkflowInput, axis WorkflowAxis, lifecycle LifecycleCoverage) RuntimeTiming {
	out := RuntimeTiming{Source: "persisted_verified_t3_receipts", Status: "not_recorded", Intervals: []RuntimeTimingInterval{}}
	counts := map[string]int{}
	for _, a := range in.ExecutionObservations {
		counts[a.AttemptID]++
	}
	intervals := []timingInterval{}
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
		out.Intervals = append(out.Intervals, RuntimeTimingInterval{a.AttemptID, a.RuntimeAgentID, a.SessionID, r.ThreadID, start, completed, observed})
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
