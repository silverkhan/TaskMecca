package activity

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// A detail projection, never an alternative denominator or a second Activity
// ledger. Shared operation duration must not be summed once for every task.
type ExecutionCoverage struct {
	Version          int                     `json:"version"`
	Items            []ExecutionCoverageItem `json:"items"`
	CollectionErrors []string                `json:"collection_errors"`
	Allocation       string                  `json:"allocation"`
}
type ExecutionCoverageItem struct {
	ACKStatus         string                        `json:"ack_status,omitempty"`
	CollectionReceipt *runtimeobs.CollectionReceipt `json:"collection_receipt,omitempty"`
	ActorID           string                        `json:"actor_id,omitempty"`
	HandoffID         string                        `json:"handoff_id,omitempty"`
	ID                string                        `json:"id"`
	Category          string                        `json:"category"`
	Role              string                        `json:"role"`
	NativeID          string                        `json:"native_id,omitempty"`
	AttemptID         string                        `json:"attempt_id,omitempty"`
	OperationID       string                        `json:"operation_id,omitempty"`
	TaskIDs           []string                      `json:"task_ids,omitempty"`
	Scope             string                        `json:"scope,omitempty"`
	Start             time.Time                     `json:"start"`
	End               *time.Time                    `json:"end,omitempty"`
	DurationMillis    *int64                        `json:"duration_ms,omitempty"`
	Reason            string                        `json:"reason"`
	Boundary          *runtimeobs.OperationBoundary `json:"boundary,omitempty"`
}

func operationKey(b runtimeobs.OperationBoundary) string {
	return strings.Join([]string{b.Provider, b.NativeID, b.AttemptID, b.OperationID}, "\x00")
}

func BuildExecutionCoverage(task string, rows []runtimeobs.OperationBoundary, w Workflow, readErr error) ExecutionCoverage {
	out := ExecutionCoverage{Version: 1, Items: []ExecutionCoverageItem{}, CollectionErrors: []string{}, Allocation: "operation_wall_time_once; task links are associations, never per-task exclusive time"}
	if readErr != nil {
		out.CollectionErrors = append(out.CollectionErrors, readErr.Error())
	}
	links := map[string]map[string]bool{}
	roles := map[string]string{}
	for _, b := range rows {
		k := operationKey(b)
		if links[k] == nil {
			links[k] = map[string]bool{}
		}
		for _, id := range b.TaskIDs {
			links[k][id] = true
		}
		if b.Role != "unknown" {
			roles[k] = b.Role
		}
	}
	sorted := append([]runtimeobs.OperationBoundary(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ObservedAt.Before(sorted[j].ObservedAt) })
	starts := map[string]int{}
	phases := map[string]int{}
	waits := map[string]int{}
	for _, b := range sorted {
		k := operationKey(b)
		if task != "" && !links[k][task] {
			continue
		}
		role := b.Role
		if role == "unknown" && roles[k] != "" {
			role = roles[k]
		}
		taskIDs := []string{}
		for id := range links[k] {
			taskIDs = append(taskIDs, id)
		}
		sort.Strings(taskIDs)
		copy := b
		item := ExecutionCoverageItem{ID: b.ID, Role: role, NativeID: b.NativeID, AttemptID: b.AttemptID, OperationID: b.OperationID, TaskIDs: taskIDs, Start: b.ObservedAt, Reason: b.Evidence, Boundary: &copy, Category: "observation_handoff"}
		switch b.Kind {
		case "start":
			item.Category = "open_start"
			if _, exists := starts[k]; exists {
				item.Category = "collection_failure"
				item.Reason = "duplicate operation START; original open interval retained"
			} else {
				starts[k] = len(out.Items)
			}
		case "phase":
			if i, ok := phases[k]; ok {
				end := b.ObservedAt
				out.Items[i].End = &end
				out.Items[i].Category = "closed_phase"
				ms := end.Sub(out.Items[i].Start).Milliseconds()
				out.Items[i].DurationMillis = &ms
			}
			item.Category = "open_phase"
			phases[k] = len(out.Items)
		case "end":
			if i, ok := phases[k]; ok {
				end := b.ObservedAt
				out.Items[i].End = &end
				out.Items[i].Category = "closed_phase"
				ms := end.Sub(out.Items[i].Start).Milliseconds()
				out.Items[i].DurationMillis = &ms
				delete(phases, k)
			}
			if i, ok := starts[k]; ok {
				out.Items[i].Category = "closed_operation"
				end := b.ObservedAt
				out.Items[i].End = &end
				ms := end.Sub(out.Items[i].Start).Milliseconds()
				out.Items[i].DurationMillis = &ms
				delete(starts, k)
			} else {
				item.Category = "collection_failure"
				item.Reason = "operation END without observed current START; no backfill"
			}
		case "wait":
			item.Category = "unknown"
			item.Reason = "unknown"
			if b.Wait != nil {
				item.Scope = b.Wait.Scope
			}
			if b.Wait.Evidenced() {
				item.Category = "evidenced_wait"
				item.Reason = b.Wait.Reason
			}
			waits[k+"\x00"+b.Key] = len(out.Items)
			waits[k+"\x00"+b.ID] = len(out.Items)
		case "resume":
			if b.PreviousAttemptID != "" {
				item.Category = "native_resume_unverified"
				item.Reason = "Historical stop remains unreported; resume source relation needs exact previous/current native and assignment evidence"
				if workflowNativeResume(w, b) {
					item.Category = "native_resume"
					item.Reason = "Same verified native/task/assignment resumed in a distinct episode; no old operation/wait END was inferred"
				}
				break
			}
			if i, ok := waits[k+"\x00"+b.RelatedID]; ok && out.Items[i].End == nil {
				end := b.ObservedAt
				out.Items[i].End = &end
				if out.Items[i].Category == "evidenced_wait" {
					ms := end.Sub(out.Items[i].Start).Milliseconds()
					out.Items[i].DurationMillis = &ms
				}
			} else {
				item.Category = "collection_failure"
				item.Reason = "resume has no unique matching open wait in this native operation"
			}
		case "collection_failure":
			item.Category = "collection_failure"
			item.Reason = b.Error
		}
		out.Items = append(out.Items, item)
	}
	// Existing Activity and lifecycle facts retain their original times and
	// overlap. Native/process observations cannot close these intervals.
	out.Items = append(out.Items, workflowCoverageItems(w)...)
	if readErr != nil {
		out.Items = append(out.Items, ExecutionCoverageItem{ID: "boundary-read-failure", Category: "collection_failure", Role: "unknown", Reason: readErr.Error()})
	}
	return out
}

func ReadExecutionCoverage(project, task string, w Workflow) ExecutionCoverage {
	rows, err := runtimeobs.ReadOperationBoundaries(project)
	out := BuildExecutionCoverage(task, rows, w, err)
	receipts, readErr := runtimeobs.ReadCollectionReceipts(project)
	if readErr != nil {
		out.CollectionErrors = append(out.CollectionErrors, readErr.Error())
		out.Items = append(out.Items, ExecutionCoverageItem{ID: "collection-outbox-read", Category: "collection_failure", Role: "unknown", Reason: readErr.Error(), Scope: "collection_delivery"})
	}
	links := map[string]map[string]bool{}
	operationLinks := map[string]map[string]bool{}
	latest := map[string]string{}
	resolved := map[string]time.Time{}
	for _, b := range rows {
		if b.RequestID != "" && links[b.RequestID] == nil {
			links[b.RequestID] = map[string]bool{}
		}
		key := operationKey(b)
		if operationLinks[key] == nil {
			operationLinks[key] = map[string]bool{}
		}
		for _, id := range b.TaskIDs {
			if b.RequestID != "" {
				links[b.RequestID][id] = true
			}
			operationLinks[key][id] = true
		}
	}
	for _, r := range receipts {
		latest[r.Key] = r.Status
		if r.Status == "ack" || r.Status == "failed" {
			resolved[r.Key] = r.At
		}
	}
	for i := range out.Items {
		if b := out.Items[i].Boundary; b != nil {
			out.Items[i].ACKStatus = latest["boundary:"+b.ID]
		}
	}
	for _, r := range receipts {
		requestObservation := r.Status == "pending" && r.Boundary == nil && r.Activity == nil
		if r.Status == "ack" || (r.Status == "pending" && latest[r.Key] != "pending" && !requestObservation) {
			continue
		}
		if task != "" && !(r.RequestID != "" && links[r.RequestID][task]) {
			linked := r.Activity != nil && r.Activity.TaskID == task
			if r.Boundary != nil {
				linked = linked || operationLinks[operationKey(*r.Boundary)][task]
				for _, id := range r.Boundary.TaskIDs {
					if id == task {
						linked = true
					}
				}
			}
			if !linked {
				continue
			}
		}
		copy := r
		item := ExecutionCoverageItem{ID: r.ID, Category: "collection_pending", Role: "unknown", Start: r.At, Scope: "collection_delivery", CollectionReceipt: &copy, Reason: "Collection intent persisted; append/ACK unverified. This is not execution or measured Activity."}
		if requestObservation {
			item.Category = "request_observation"
			item.Scope = "request_delivery_status"
			item.OperationID = r.Key
			item.Reason = "Request preparation observed before authoritative native/task identity. Later task links are associations; delivery status is not measured work."
			if end, ok := resolved[r.Key]; ok {
				item.End = &end
			}
		}
		if r.Activity != nil {
			item.Role = r.Activity.Role
			item.ActorID = r.Activity.AgentID
			item.AttemptID = r.Activity.AttemptID
			item.TaskIDs = []string{r.Activity.TaskID}
		}
		if r.Boundary != nil {
			b := r.Boundary
			item.Role = b.Role
			item.NativeID = b.NativeID
			item.AttemptID = b.AttemptID
			item.OperationID = b.OperationID
			item.TaskIDs = append([]string(nil), b.TaskIDs...)
		}
		if r.Status == "failed" {
			item.Category = "collection_failure"
			item.Reason = r.Error
			if latest[r.Key] == "ack" {
				item.Scope = "collection_delivery_recovered"
				item.Reason += "; later delivery acknowledged; original failure retained"
			}
		}
		out.Items = append(out.Items, item)
	}
	for i := range out.Items {
		item := &out.Items[i]
		if w.Axis.End != nil && !w.Axis.Open && !item.Start.IsZero() && !item.Start.Before(*w.Axis.End) {
			item.Scope = "after_task_completion"
		} else if w.Axis.End != nil && !w.Axis.Open && item.End != nil && item.End.After(*w.Axis.End) {
			item.Scope = "crosses_task_completion"
		}
	}
	return out
}

// AttachExecutionCoverage preserves the A90 Workflow type and all measurement
// fields. Detail lives under reporting.roles[].execution_coverage. Copy the
// reporting slice before enrichment so shared workflow-cache values stay pure.
func AttachExecutionCoverage(project, task string, w Workflow) Workflow {
	detail := ReadExecutionCoverage(project, task, w)
	w.Reporting.Roles = append([]RoleReportingCoverage(nil), w.Reporting.Roles...)
	byRole := map[string][]ExecutionCoverageItem{}
	for _, item := range detail.Items {
		role := item.Role
		if role == "" {
			role = "unknown"
		}
		byRole[role] = append(byRole[role], item)
	}
	keys := []string{}
	for role := range byRole {
		keys = append(keys, role)
	}
	sort.Strings(keys)
	for _, role := range keys {
		d := detail
		d.Items = byRole[role]
		idx := -1
		for i := range w.Reporting.Roles {
			if w.Reporting.Roles[i].Role == role {
				idx = i
				break
			}
		}
		if idx < 0 {
			w.Reporting.Roles = append(w.Reporting.Roles, RoleReportingCoverage{Role: role, Status: "not_recorded"})
			idx = len(w.Reporting.Roles) - 1
		}
		w.Reporting.Roles[idx].ExecutionCoverage = &d
	}
	workflowAttachOperationIntervals(&w, detail)
	workflowAttachReportedWait(&w, detail)
	workflowDiagnoseCoverage(&w)
	for _, key := range []string{"execution_evidence_error", "handoff_error"} {
		if cause, ok := w.Snapshot[key].(string); ok && cause != "" {
			w.Coverage.Findings = append(w.Coverage.Findings, WorkflowRecoveryFinding{TaskID: task, Start: w.ComputedAt, End: w.ComputedAt, Status: "collection_failed", Cause: cause, Owner: "controller", ResumeCondition: "Restore persisted source read and verify next actual boundary; historical gaps remain unknown", Evidence: []WorkflowGapEvidence{}})
		}
	}
	for _, item := range detail.Items {
		if item.Category == "collection_failure" || item.Category == "collection_pending" {
			end := item.Start
			if item.End != nil {
				end = *item.End
			}
			cause := "unknown"
			if item.Category == "collection_failure" {
				cause = item.Reason
			}
			actor := item.NativeID
			if actor == "" {
				actor = item.ActorID
			}
			var last *time.Time
			for _, prior := range detail.Items {
				if prior.Category == "collection_failure" || prior.Category == "collection_pending" || prior.AttemptID != item.AttemptID || prior.OperationID != item.OperationID || prior.NativeID != item.NativeID || prior.ActorID != item.ActorID {
					continue
				}
				at := prior.Start
				if prior.End != nil {
					at = *prior.End
				}
				if !at.After(item.Start) && (last == nil || at.After(*last)) {
					copy := at
					last = &copy
				}
			}
			w.Coverage.Findings = append(w.Coverage.Findings, WorkflowRecoveryFinding{TaskID: task, ActorID: actor, AttemptID: item.AttemptID, OperationID: item.OperationID, Start: item.Start, End: end, Status: item.Category, Cause: cause, Owner: item.Role, LastSuccessfulBoundary: last, ResumeCondition: "Retry exact durable collection key and verify append ACK; no historical Activity backfill", Evidence: []WorkflowGapEvidence{}})
		}
	}
	return w
}

func coverageID(role, actor, id string) string { return fmt.Sprintf("%s:%s:%s", role, actor, id) }

// Operation wall time is visible once per exact native/attempt/operation. It
// remains a shared task association, never confirmed Activity or task allocation.
func workflowAttachOperationIntervals(w *Workflow, detail ExecutionCoverage) {
	w.Rows = append([]WorkflowRow(nil), w.Rows...)
	for i := range w.Rows {
		w.Rows[i].Intervals = append([]WorkflowInterval(nil), w.Rows[i].Intervals...)
		w.Rows[i].RoleFacts = append([]WorkflowRoleFact(nil), w.Rows[i].RoleFacts...)
	}
	for _, item := range detail.Items {
		if item.Category != "native_resume" || item.Boundary == nil {
			continue
		}
		b := item.Boundary
		for i := range w.Rows {
			r := &w.Rows[i]
			if r.Role != item.Role || r.SessionIdentity == nil || r.SessionIdentity.Status != "verified" || r.SessionIdentity.NativeID != item.NativeID {
				continue
			}
			for _, a := range r.SessionIdentity.Attempts {
				if a.AttemptID == item.AttemptID {
					r.RoleFacts = append(r.RoleFacts, WorkflowRoleFact{ID: b.ID, Kind: "same_native_resumed", At: b.ObservedAt, Source: b.Source, SourceAt: b.SourceAt, PreviousAttemptID: b.PreviousAttemptID, AttemptID: b.AttemptID, AssignmentID: a.AssignmentID, Reason: b.Explanation + " (" + b.PreviousAttemptID + " -> " + b.AttemptID + ")", MeasurementStatus: "point_not_activity"})
				}
			}
		}
	}
	seen := map[string]bool{}
	for _, item := range detail.Items {
		if item.Boundary == nil || item.Boundary.Provider == "" || item.NativeID == "" || item.AttemptID == "" || item.OperationID == "" || (item.Category != "closed_operation" && item.Category != "open_start") {
			continue
		}
		linked := false
		for _, task := range item.TaskIDs {
			if task == w.TaskID {
				linked = true
			}
		}
		if !linked {
			continue
		}
		key := item.Boundary.Provider + "|" + item.NativeID + "|" + item.AttemptID + "|" + item.OperationID
		if seen[key] {
			continue
		}
		seen[key] = true
		index := -1
		for i, r := range w.Rows {
			if r.Role != item.Role || r.SessionIdentity == nil || r.SessionIdentity.Status != "verified" || r.SessionIdentity.NativeID != item.NativeID || r.SessionIdentity.Provider != item.Boundary.Provider {
				continue
			}
			for _, a := range r.SessionIdentity.Attempts {
				if a.AttemptID == item.AttemptID {
					index = i
				}
			}
		}
		if index < 0 {
			w.Rows = append(w.Rows, WorkflowRow{ID: "operation|" + key, Role: item.Role, AgentID: item.NativeID, RoundStatus: "unclassified", Intervals: []WorkflowInterval{}, RoleFacts: []WorkflowRoleFact{}, Gaps: []WorkflowGap{}})
			index = len(w.Rows) - 1
		}
		b := item.Boundary
		v := WorkflowInterval{ID: "operation|" + key, Kind: "operation_execution", Type: "operation", Start: item.Start, End: item.End, Measured: false, AttemptID: item.AttemptID, OperationID: item.OperationID, Scope: item.Scope, Evidence: item.Reason, Explanation: b.Explanation, ExplanationLanguage: b.ExplanationLanguage, ExplanationOrigin: b.ExplanationOrigin}
		if item.End == nil {
			for _, native := range w.RuntimeTiming.Intervals {
				if native.AttemptID == item.AttemptID && native.SessionID == item.NativeID && native.NativeCompletedAt != nil {
					end := *native.NativeCompletedAt
					v.Kind = "operation_unreported"
					v.EndEvidence = "native_terminal_operation_end_unreported"
					v.EndEvidenceSource = runtimeobs.EvidenceT3
					v.RuntimeTerminal = &WorkflowTerminalEvidence{ObservedAt: end, Source: runtimeobs.EvidenceT3}
					v.Evidence += "; native execution ended, operation END was not reported; displayed end is only the verified native bound, never a measured operation stop"
					if end.Before(v.Start) {
						end = v.Start
						v.Scope = "reported_after_native_completion"
					}
					v.End = &end
				}
			}
		}
		w.Rows[index].Intervals = append(w.Rows[index].Intervals, v)
		if item.End != nil {
			first, last := item.Start, *item.End
			if prior := w.Rows[index].Participation; prior != nil {
				if prior.Start.Before(first) {
					first = prior.Start
				}
				if prior.End.After(last) {
					last = prior.End
				}
			}
			status, basis := "bounded", "observed_bounds_with_exact_operation_task_association_shared_not_exclusive"
			if prior := w.Rows[index].Participation; prior != nil && strings.HasPrefix(prior.Status, "open_") {
				status = prior.Status
				basis = prior.Basis
			}
			w.Rows[index].Participation = &WorkflowParticipation{Start: first, End: last, Basis: basis, Status: status}
			gaps := []WorkflowGap{}
			for _, g := range w.Rows[index].Gaps {
				if g.Status == "before_participation" && g.End.After(first) {
					g.End = first
				}
				if g.Status == "after_participation" && g.Start.Before(last) {
					g.Start = last
				}
				if g.End.After(g.Start) {
					gaps = append(gaps, g)
				}
			}
			w.Rows[index].Gaps = gaps
		}
	}
}

func workflowNativeResume(w Workflow, b runtimeobs.OperationBoundary) bool {
	var old, current *RuntimeTimingInterval
	for i := range w.RuntimeTiming.Intervals {
		v := &w.RuntimeTiming.Intervals[i]
		if v.AttemptID == b.PreviousAttemptID && v.SessionID == b.NativeID {
			old = v
		}
		if v.AttemptID == b.AttemptID && v.SessionID == b.NativeID {
			current = v
		}
	}
	if b.SourceAt == nil || old == nil || current == nil || old.NativeCompletedAt == nil || !b.SourceAt.Equal(*old.NativeCompletedAt) || old.ThreadID != current.ThreadID || current.ThreadID != b.ThreadID || current.NativeStartedAt.Before(*old.NativeCompletedAt) || b.ObservedAt.Before(current.NativeStartedAt) {
		return false
	}
	for _, row := range w.Rows {
		if row.Role != b.Role || row.SessionIdentity == nil || row.SessionIdentity.Status != "verified" || row.SessionIdentity.NativeID != b.NativeID {
			continue
		}
		previousAssignment, currentAssignment := "", ""
		for _, a := range row.SessionIdentity.Attempts {
			if a.AttemptID == b.PreviousAttemptID {
				previousAssignment = a.AssignmentID
			}
			if a.AttemptID == b.AttemptID {
				currentAssignment = a.AssignmentID
			}
		}
		if previousAssignment != "" && previousAssignment == currentAssignment {
			return true
		}
	}
	return false
}

// This describes an acknowledged reported state, never idle/Activity duration.
type WorkflowReportedWait struct {
	BoundaryID          string                  `json:"boundary_id"`
	OperationID         string                  `json:"operation_id"`
	AttemptID           string                  `json:"attempt_id"`
	ObservedAt          time.Time               `json:"observed_at"`
	Source              string                  `json:"source"`
	Explanation         string                  `json:"explanation"`
	ExplanationLanguage string                  `json:"explanation_language,omitempty"`
	Wait                runtimeobs.BoundaryWait `json:"wait"`
}

func workflowAttachReportedWait(w *Workflow, detail ExecutionCoverage) {
	if w.Current.ReportedWait != nil {
		w.Current.ReportedWait = nil
		if w.Current.Phase == "reported_wait" {
			w.Current.Phase = "unknown"
			w.Current.Basis = "reported_wait_no_longer_current"
			w.Current.NextAction = "verify_execution_evidence"
		}
	}

	switch w.Current.Phase {
	case "completed", "controller_review", "independent_review", "finalizing":
		return
	}
	if w.Lifecycle.CompletedAt != nil {
		return
	}
	var selected *ExecutionCoverageItem
	for i := range detail.Items {
		item := &detail.Items[i]
		b := item.Boundary
		if item.Category != "evidenced_wait" || item.End != nil || item.ACKStatus != "ack" || b == nil || !b.Wait.Evidenced() || b.ObservedAt.After(w.ComputedAt) {
			continue
		}
		linked := false
		for _, task := range item.TaskIDs {
			if task == w.TaskID {
				linked = true
			}
		}
		if !linked {
			continue
		}
		verified := false
		for _, row := range w.Rows {
			if row.Role != b.Role || row.SessionIdentity == nil || row.SessionIdentity.Status != "verified" || row.SessionIdentity.Provider != b.Provider || row.SessionIdentity.NativeID != b.NativeID {
				continue
			}
			for _, a := range row.SessionIdentity.Attempts {
				if a.AttemptID == b.AttemptID {
					verified = true
				}
			}
		}
		current := false
		for _, native := range w.RuntimeTiming.Intervals {
			if native.SessionID == b.NativeID && native.AttemptID == b.AttemptID && native.NativeCompletedAt == nil && !native.NativeStartedAt.After(b.ObservedAt) {
				current = true
			}
			if native.SessionID == b.NativeID && native.NativeStartedAt.After(b.ObservedAt) {
				verified = false
			}
		}
		if !verified || !current {
			continue
		}
		superseded := false
		for _, later := range detail.Items {
			next := later.Boundary
			if next == nil || next.NativeID != b.NativeID || next.AttemptID != b.AttemptID || !next.ObservedAt.After(b.ObservedAt) {
				continue
			}
			switch next.Kind {
			case "start", "phase", "resume", "end", "wait", "handoff_request", "handoff_claim":
				superseded = true
			}
		}
		for _, row := range w.Rows {
			for _, v := range row.Intervals {
				if v.Kind == "activity_open" && (v.AttemptID == b.AttemptID || v.Start.After(b.ObservedAt)) {
					superseded = true
				}
			}
		}
		if superseded {
			continue
		}
		if selected == nil || selected.Start.Before(item.Start) {
			selected = item
		}
	}
	if selected == nil {
		return
	}
	b := selected.Boundary
	w.Current.Phase = "reported_wait"
	w.Current.Basis = "acknowledged_operation_wait"
	w.Current.ProcessingRole = b.Role
	w.Current.ProcessingAgent = b.NativeID
	w.Current.NextOwner = b.Wait.NextOwner
	w.Current.NextAction = "resume_reported_operation"
	w.Current.ReportedWait = &WorkflowReportedWait{BoundaryID: b.ID, OperationID: b.OperationID, AttemptID: b.AttemptID, ObservedAt: b.ObservedAt, Source: b.Source, Explanation: b.Explanation, ExplanationLanguage: b.ExplanationLanguage, Wait: *b.Wait}
}
