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
	return w
}

func coverageID(role, actor, id string) string { return fmt.Sprintf("%s:%s:%s", role, actor, id) }
