package webui

import (
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// Read-only presentation: task state can explain an old observation gap, but
// can never supply runtime completion or remove the underlying incident.
const (
	operationCurrent    = "current_operation"
	operationHold       = "user_hold"
	operationHistory    = "observation_history"
	operationDiagnostic = "insufficient_evidence"
)

func operationViewCategory(item operationIncident, rows map[string][]backlog.Record, available bool, now time.Time) string {
	if item.PolicyResolution != nil && strings.Contains(item.PolicyResolution.Reason, "_hold") && available {
		matches := rows[strings.ToUpper(strings.TrimSpace(item.TaskID))]
		if len(matches) == 1 && matches[0].State == "hold" {
			return operationHold
		}
	}
	if item.RecoveredAt != "" || item.Kind == "normal_exit" || item.Kind == "monitor_gap" {
		return operationHistory
	}
	if item.Quality == "confirmed" || item.Kind == "handoff_failed" {
		return operationCurrent
	}
	if !available {
		return operationDiagnostic
	}
	matches := rows[strings.ToUpper(strings.TrimSpace(item.TaskID))]
	if len(matches) > 1 {
		return operationDiagnostic
	}
	if len(matches) == 1 {
		row := matches[0]
		if row.State == "doing" {
			return operationCurrent
		}
		// Classify only older observation gaps. Fresh errors, late activity and
		// failed handoffs are independent current issues, including on done tasks.
		if item.Kind != "runtime_unknown" && item.Kind != "no_signal" && item.Kind != "handoff_stalled" {
			return operationCurrent
		}
		clock := item.LastObservedAt
		if clock == "" {
			clock = item.DetectedAt
		}
		at, e := time.Parse(time.RFC3339Nano, clock)
		boundary, b := time.Parse(time.RFC3339Nano, row.Mtime)
		if e != nil || b != nil || at.IsZero() || boundary.IsZero() || boundary.After(now) || at.After(now) {
			return operationDiagnostic
		}
		if at.After(boundary) {
			return operationCurrent
		}
		if row.State == "done" {
			return operationHistory
		}
		if row.State == "hold" && (row.Fields["대기유형"] == "user" || row.Fields["대기유형"] == "external" || row.Fields["대기유형"] == "dependency") && operationMeaningfulEvidence(row.Fields["대기"]) && operationMeaningfulEvidence(row.Fields["재개조건"]) {
			return operationHold
		}
		return operationDiagnostic
	}
	clock := item.LastObservedAt
	if clock == "" {
		clock = item.DetectedAt
	}
	at, err := time.Parse(time.RFC3339Nano, clock)
	if err != nil || at.IsZero() || at.After(now) || now.Sub(at) >= runtimeobs.StaleWarnDuration() {
		return operationDiagnostic
	}
	return operationCurrent
}
