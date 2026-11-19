package backlog

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// HumanActionCondition is the shared outbound policy. An observation gap is
// diagnostic even after a long dwell; changing its label cannot authorize it.
func HumanActionCondition(condition map[string]any) bool {
	if condition["human_action_verified"] != true {
		return false
	}
	switch toString(condition["kind"]) {
	case "intervention", "approval", "interrupted":
	default:
		return false
	}
	switch toString(condition["reason_type"]) {
	case "user_intervention", "approval_required", "execution_interrupted":
	default:
		return false
	}
	message := strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(toString(condition["message"]), "_", " ")), " "))
	switch message {
	case "no signal", "no activity", "worker missing", "runtime unknown", "observation gap", "무신호", "관측 공백", "실행 관측 공백":
		return false
	}
	return meaningfulHumanAction(toString(condition["message"])) && meaningfulHumanAction(toString(condition["resume_condition"]))
}

// Finalize is retained internal lifecycle evidence. External senders still use
// HumanActionCondition, so completion-review events never bypass the human gate.
func recordableNotificationCondition(condition map[string]any) bool {
	return HumanActionCondition(condition) || (toString(condition["kind"]) == "finalize" && toString(condition["reason_type"]) == "completion_pending")
}

func meaningfulHumanAction(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "-", "unknown", "none", "n/a", "미확인":
		return false
	}
	return true
}

// Recent Activity is independently attributed reporting, never CPU/progress or
// proof that an execution is still alive. Do not turn stale health into active.
func attributedActivityEvents(a runtimeobs.Attempt, events []activity.Event, now time.Time) []activity.Event {
	identity := runtimeobs.PolicyIdentity(a)
	if !identity.Observed || identity.NativeID == "" || a.RuntimeAgentID != identity.NativeID || a.BindingState != runtimeobs.BindingBound || a.BindingEvidence["assignment_id"] == "" {
		return nil
	}
	lower := implementationDispatchTime(a)
	valid := []activity.Event{}
	for _, event := range events {
		if event.Role != activity.RoleWorker || event.TaskID != a.TaskID || event.AgentID != identity.NativeID || event.RuntimeAttemptID != a.AttemptID || event.AssignmentID != a.BindingEvidence["assignment_id"] || event.Action == activity.Correct {
			continue
		}
		if event.SessionID != "" && event.SessionID != identity.NativeID {
			continue
		}
		if event.OccurredAt.IsZero() || event.RecordedAt.IsZero() || event.OccurredAt.Before(lower) || event.RecordedAt.Before(lower) || event.OccurredAt.After(now) || event.RecordedAt.After(now) {
			continue
		}
		valid = append(valid, event)
	}
	return valid
}

func currentActivityEvidence(a runtimeobs.Attempt, events []activity.Event, now time.Time) map[string]any {
	events = attributedActivityEvents(a, events, now)
	var latest *activity.Event
	for i := range events {
		event := &events[i]
		if now.Sub(event.OccurredAt) > 2*time.Minute {
			continue
		}
		if latest == nil || event.OccurredAt.After(latest.OccurredAt) {
			latest = event
		}
	}
	if latest == nil {
		return nil
	}
	return map[string]any{"event_id": latest.EventID, "at": latest.OccurredAt.Format(time.RFC3339Nano), "type": latest.Type, "confidence": latest.Confidence, "source": "exact_current_activity_report", "limitation": "Agent-reported phase; not continuous progress, CPU activity or current liveness."}
}

type HumanActionDecision struct {
	Condition map[string]any
	EventID   string
	Since     string
}

// CurrentHumanActionDecisions re-reads the canonical inputs at the send boundary.
// NotificationEvents owns the shared cause episode clock. Both external paths
// use its ID, so the existing transport lock deduplicates concurrent senders.
// Historical events and sent channel records are never rewritten or replayed.
func CurrentHumanActionDecisions(project, root string) (map[string]HumanActionDecision, error) {
	rows, err := Catalog(project, root)
	if err != nil {
		return nil, err
	}
	control := reconcileControlTower(project, root, rows)
	if control.Err != nil {
		return nil, control.Err
	}
	items := map[string]map[string]any{}
	activeCount := map[string]int{}
	for _, row := range rows {
		if row.Location == "active" {
			activeCount[row.ID]++
		}
	}
	for id := range control.NotificationCondition {
		if activeCount[id] != 1 {
			delete(control.NotificationCondition, id)
		}
	}
	for id, row := range preferredRows(rows) {
		items[id] = map[string]any{"id": id, "title": row.Title, "file_state": row.State, "updated_at": row.Mtime, "lifecycle": control.Timings[id], "notification_condition": control.NotificationCondition[id]}
	}
	if _, err := NotificationEvents(project, items); err != nil {
		return nil, err
	}
	notificationJournalMu.Lock()
	defer notificationJournalMu.Unlock()
	journal, ok := cachedNotificationJournal(filepath.Join(project, "_task_mecca", ".runtime", "notification_events.json"))
	if !ok {
		return map[string]HumanActionDecision{}, nil
	}
	decisions := map[string]HumanActionDecision{}
	for id, condition := range control.NotificationCondition {
		observation, found := journal.Items[id]
		if !found || !HumanActionCondition(condition) || observation.ConditionKey != toString(condition["key"]) || observation.ConditionSince == "" {
			continue
		}
		eventID := observation.ConditionEventID
		if eventID == "" {
			eventID = humanActionEventID(id, observation.ConditionKey, observation.ConditionSince)
		}
		decisions[id] = HumanActionDecision{Condition: condition, Since: observation.ConditionSince, EventID: eventID}
	}
	return decisions, nil
}

func humanActionEventID(task, key, since string) string {
	return notificationEventID(task, "human_action", key+"\x00"+since)
}

// Match checks an original journal event against the current episode. Older
// events without the exact cause identity cannot authorize a new attempt.
func (decision HumanActionDecision) Match(event map[string]any) bool {
	if !HumanActionCondition(decision.Condition) || toString(event["kind"]) != toString(decision.Condition["kind"]) {
		return false
	}
	if toString(event["condition_key"]) != toString(decision.Condition["key"]) || toString(event["condition_since"]) != decision.Since {
		return false
	}
	return toString(event["message"]) == toString(decision.Condition["message"]) && toString(event["resume_condition"]) == toString(decision.Condition["resume_condition"])
}

func HumanActionKind(kind string) bool {
	switch kind {
	case "intervention", "approval", "interrupted", "stalled", "runtime_unknown", "finalize":
		return true
	}
	return false
}

func legacyConditionKey(condition map[string]any) string {
	key := toString(condition["key"])
	prefix := strings.Split(key, "\x00")[0]
	attempt := toString(condition["attempt_id"])
	if attempt == "" {
		if toString(condition["assignment_id"]) != "" {
			return ""
		}
		return prefix
	}
	return prefix + "\x00" + attempt
}
