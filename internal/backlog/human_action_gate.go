package backlog

import (
	"encoding/json"
	"errors"
	"github.com/silverkhan/TaskMecca/internal/projectguard"
	"os"
	"path/filepath"
	"regexp"
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
	kind, reason := toString(condition["kind"]), toString(condition["reason_type"])
	if (kind != "intervention" && kind != "approval" && kind != "interrupted") || (reason != "user_intervention" && reason != "approval_required" && reason != "execution_interrupted") {
		return false
	}
	if !meaningfulHumanAction(toString(condition["message"])) || !meaningfulHumanAction(toString(condition["resume_condition"])) {
		return false
	}
	basis, _ := condition["human_action_basis"].(map[string]any)
	switch toString(basis["source"]) {
	case "observed_current_execution":
		return runtimeHumanActionKind(toString(basis["state"])) == kind && toString(basis["state"]) == toString(condition["runtime_state"])
	case "observed_controller_wait":
		return kind == "intervention" && meaningfulHumanAction(toString(basis["handoff_id"])) && meaningfulHumanAction(toString(basis["contract_sha256"]))
	default:
		// Low-level journal callers also pass explicit request/action pairs. A
		// verified flag or user/hold label alone never supplies such a request.
		return requestedHumanAction(toString(condition["message"]), toString(condition["resume_condition"])) != nil
	}
}

func runtimeHumanActionKind(state string) string {
	switch state {
	case "waiting_approval":
		return "approval"
	case "waiting_user":
		return "intervention"
	case "errored", "interrupted", "shutdown":
		return "interrupted"
	}
	return ""
}

var requestedActionActor = regexp.MustCompile(`(?i)\b(owner|user|administrator|admin|root|operator)\s*:?\s*(?:please\s+)?(grant|approve|restore|provide|allow|authorize|answer|reply|choose|confirm|decide|select)s?\b|(?:사용자|관리자|소유자|담당자)\s*:?`)
var requestedNeed = regexp.MustCompile(`(?i)\b(required?|requires?|needed?|needs?|request(?:ed)?|asks user|awaiting|denied|forbidden|blocked|grant|provide)\b|필요|요청|답변 대기|거부|차단|충돌`)

var requestedAccess = regexp.MustCompile(`(?i)\b(access|permission|credentials?|authorization)\b|권한|접근|인증`)
var requestedDecision = regexp.MustCompile(`(?i)\b(approval|decision|choice|policy)\b|승인|판단|선택|정책|확정`)
var requestedInput = regexp.MustCompile(`(?i)\b(input|question|answer|asks user)\b|입력|질문|답변`)
var accessAction = regexp.MustCompile(`(?i)\b(grant|approve|restore|provide|allow|authorize)s?\b|부여|허용|승인|제공`)
var decisionAction = regexp.MustCompile(`(?i)\b(approve|decide|choose|confirm|provide|answer|grant)s?\b|승인|확정|선택|답변|제공`)
var inputAction = regexp.MustCompile(`(?i)\b(answer|reply|provide|choose|select)s?\b|답변|입력|제공|선택`)

// Positive request evidence has TWO independent fields: a concrete blocker and
// a named person's action on that blocker. This is a small supported action
// grammar, not a blacklist of the unbounded ways to describe missing telemetry.
// Unknown prose stays diagnostic until a concrete request or runtime wait exists.
func requestedHumanAction(request, next string) map[string]any {
	if !requestedNeed.MatchString(request) || !requestedActionActor.MatchString(next) {
		return nil
	}
	category := ""
	switch {
	case requestedAccess.MatchString(request) && requestedAccess.MatchString(next) && accessAction.MatchString(next):
		category = "grant_access"
	case requestedDecision.MatchString(request) && (requestedDecision.MatchString(next) || requestedAccess.MatchString(next)) && decisionAction.MatchString(next):
		category = "provide_decision"
	case requestedInput.MatchString(request) && requestedInput.MatchString(next) && inputAction.MatchString(next):
		category = "provide_input"
	}
	if category == "" {
		return nil
	}
	return map[string]any{"source": "canonical_requested_action", "category": category, "request": request, "next_action": next}
}

func controllerWaitBasis(review map[string]any, markdown string, now time.Time) map[string]any {
	// This reason/quality pair is produced only after the completion projector
	// validates the exact claimed Controller execution. A needs_user label or
	// generic diagnostic (including a handoff hold label) cannot supply it.
	since, err := time.Parse(time.RFC3339Nano, toString(review["since"]))
	if review["state"] != "needs_user" || review["reason"] != "exact_controller_claim_running" || review["observation_quality"] != "observed" || !completionContractMatches(markdown, toString(review["contract_sha256"])) || err != nil || since.After(now) || !meaningfulHumanAction(toString(review["handoff_id"])) || !meaningfulHumanAction(toString(review["contract_sha256"])) {
		return nil
	}
	return map[string]any{"source": "observed_controller_wait", "handoff_id": review["handoff_id"], "contract_sha256": review["contract_sha256"], "observed_since": review["since"]}
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
	Condition       map[string]any
	EventID         string
	Since           string
	LegacyNotBefore string
}

// CurrentHumanActionDecisions re-reads the canonical inputs at the send boundary.
// NotificationEvents owns the shared cause episode clock. Both external paths
// use its ID, so the existing transport lock deduplicates concurrent senders.
// Historical events and sent channel records are never rewritten or replayed.
func CurrentHumanActionDecisions(project, root string) (map[string]HumanActionDecision, error) {
	release, err := projectguard.AcquireWrite(project)
	if err != nil {
		return nil, err
	}
	defer release()
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
		items[id] = map[string]any{"id": id, "title": row.Title, "file_state": row.State, "updated_at": row.Mtime, "lifecycle": control.Timings[id], "notification_condition": control.NotificationCondition[id], "canonical_wait_kind": row.Fields["대기유형"], "canonical_wait_reason": row.Fields["대기"]}
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
	if err := trackHumanActionBoundaries(project, items, control.Activity, journal, decisions, time.Now().UTC()); err != nil {
		return nil, err
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

// Only decision continuity is persisted here. The original notification,
// operation/source and sent journals are never migrated or rewritten by this
// tracker. Observed clears, cause changes and completion survive a restart.
type humanActionBoundary struct {
	Key        string `json:"key,omitempty"`
	Since      string `json:"since,omitempty"`
	NotBefore  string `json:"not_before,omitempty"`
	EventID    string `json:"event_id,omitempty"`
	Assignment string `json:"assignment_id,omitempty"`
	Uncertain  bool   `json:"uncertain,omitempty"`
}
type humanActionBoundaries struct {
	Version int                            `json:"version"`
	Items   map[string]humanActionBoundary `json:"items"`
}

func trackHumanActionBoundaries(project string, items, signals map[string]map[string]any, journal notificationJournal, decisions map[string]HumanActionDecision, now time.Time) error {
	path := filepath.Join(project, "_task_mecca", ".runtime", "human_action_boundaries.json")
	state := humanActionBoundaries{Version: 1, Items: map[string]humanActionBoundary{}}
	data, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(data, &state); err != nil {
			return err
		}
		if state.Version != 1 || state.Items == nil {
			return errors.New("invalid human action boundaries")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	dirty := false
	tasks := map[string]bool{}
	for id := range items {
		tasks[id] = true
	}
	for id := range state.Items {
		tasks[id] = true
	}
	for id := range tasks {
		old, seen := state.Items[id]
		for _, stamp := range []string{old.Since, old.NotBefore} {
			if stamp != "" {
				at, e := time.Parse(time.RFC3339Nano, stamp)
				if e != nil || at.After(now) {
					return errors.New("invalid human action boundary timestamp")
				}
			}
		}
		next := old
		decision, active := decisions[id]
		if active {
			key := toString(decision.Condition["key"])
			assignment := toString(decision.Condition["assignment_id"])
			if seen && old.Assignment != "" && assignment == "" && sameHumanCauseKey(old.Key, key) {
				// Lost identity collection is not recovery or a new assignment.
				next.Uncertain = true
				state.Items[id] = next
				dirty = dirty || next != old
				delete(decisions, id)
				continue
			}
			if !seen {
				// At upgrade, absence of prior episodes permits pre-backlog identity
				// inheritance. Any retained earlier cause/completion makes it uncertain.
				for _, event := range journal.Events {
					if toString(event["task_id"]) != id || toString(event["id"]) == decision.EventID {
						continue
					}
					if HumanActionKind(toString(event["kind"])) || toString(event["kind"]) == "completed" {
						next.NotBefore = decision.Since
					}
				}
			} else if old.Key != "" && (old.Key != key || (old.Since != decision.Since && !old.Uncertain)) {
				next.NotBefore = decision.Since
			} else if old.Key == key && old.EventID != "" && (old.Uncertain || old.Since == decision.Since) {
				decision.EventID = old.EventID
			}
			next.Assignment, next.EventID, next.Uncertain = assignment, decision.EventID, false
			next.Key, next.Since = key, decision.Since
			decision.LegacyNotBefore = next.NotBefore
			decisions[id] = decision
		} else if old.Key != "" {
			if authoritativeHumanCauseClear(old.Key, items[id], signals[id]) {
				next.Key, next.Since, next.EventID = "", "", ""
				next.NotBefore = now.Format(time.RFC3339Nano)
				next.Uncertain = false
			} else {
				next.Uncertain = true
			}
		}
		if !seen || old != next {
			state.Items[id] = next
			dirty = true
		}
	}
	if !dirty {
		return nil
	}
	body, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err = os.WriteFile(path+".tmp", append(body, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func sameHumanCauseKey(left, right string) bool {
	a, b := strings.Split(left, "\x00"), strings.Split(right, "\x00")
	return len(a) >= 2 && len(b) >= 2 && a[0] == b[0] && a[1] == b[1]
}
func authoritativeHumanCauseClear(key string, item, signal map[string]any) bool {
	state := toString(item["file_state"])
	if state == "done" || (state != "" && state != "doing" && state != "hold") {
		return true
	}
	if strings.HasPrefix(key, "hold:user") && item != nil {
		waitKind := toString(item["canonical_wait_kind"])
		if waitKind != "user" && waitKind != "external" {
			return true
		}
		parts := strings.Split(key, "\x00")
		return len(parts) >= 2 && parts[1] != strings.ToLower(strings.Join(strings.Fields(toString(item["canonical_wait_reason"])), " "))
	}
	if signal["human_action_runtime_verified"] == true {
		return toString(signal["runtime_state"]) == string(runtimeobs.StateRunning) || toString(signal["runtime_state"]) == string(runtimeobs.StateCompleted)
	}
	return false
}
