package backlog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/silverkhan/TaskMecca/internal/projectguard"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// HumanActionCondition is the shared outbound policy. An observation gap is
// diagnostic even after a long dwell; changing its label cannot authorize it.
func HumanActionCondition(condition map[string]any) bool {
	proof, ok := condition["human_action_basis"].(*humanActionProof)
	if !ok || proof == nil || toString(condition["task_id"]) == "" || proof.condition == nil || reflect.ValueOf(condition).UnsafePointer() != reflect.ValueOf(proof.condition).UnsafePointer() {
		return false
	}
	fingerprint, err := humanConditionFingerprint(condition, proof)
	return err == nil && fingerprint == proof.fingerprint && validHumanActionBasis(condition)
}

// Only canonical construction may validate an unsealed candidate.
func validHumanActionBasis(condition map[string]any) bool {
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
	proof, ok := condition["human_action_basis"].(*humanActionProof)
	if !ok || proof == nil || proof.request != toString(condition["message"]) || proof.next != toString(condition["resume_condition"]) {
		return false
	}
	switch proof.source {
	case "observed_current_execution":
		return runtimeHumanActionKind(proof.state) == kind && proof.state == toString(condition["runtime_state"])
	case "observed_controller_wait":
		return kind == "intervention" && proof.handoff != "" && proof.contract != ""
	case "canonical_requested_action":
		parsed := requestedHumanAction(proof.request, proof.next)
		return parsed != nil && parsed.category == proof.category
	}
	return false
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

// The supported grammar consumes the ENTIRE assertion/action, including its
// tense and qualifiers. Arbitrary clauses are not a resource name. Unknown,
// negated, conditional, or future prose cannot be salvaged by keyword matches.
const humanNoun = `(?:repository|database|deployment|billing|project|service|external|owner|administrator|access|permission|credentials?|approval|decision|choice|policy|input|answer|question|result|value|existing|requested|different|read|write)`
const humanObject = `(?:(?:the|a) )?(?:` + humanNoun + `(?: ` + humanNoun + `)*)`

const presentHumanClause = `(?:` + humanObject + ` (?:is |are )?(?:required|needed|denied|blocked)(?: (?:now|to (?:continue|resume) (?:deployment|work|execution|the task)))?)`

var presentHumanAssertion = regexp.MustCompile(`(?i)^(?:` + presentHumanClause + `(?:; ` + presentHumanClause + `)?|(?:grant|provide|restore|approve|answer) ` + humanObject + `|(?:user|owner) answer to the (?:deployment|billing|project) question is required)\.?$`)
var unconditionalHumanAction = regexp.MustCompile(`(?i)^(?:repository )?(owner|user|administrator|admin|root|operator):? (?:please )?(grant|approve|restore|provide|allow|authorize|answer|reply|choose|confirm|decide|select)s? (` + humanObject + `|answer to the (?:deployment|billing|project) question)\.?$`)
var presentKoreanAssertion = regexp.MustCompile(`^(?:(?:배포를|작업을|실행을) (?:계속|재개)하려면 )?(?:저장소 |데이터베이스 |프로젝트 |배포 |서비스 |계정 |사용자 )?(?:접근 권한(?: 부여)?|권한(?: 부여)?|승인|결정|입력|답변)(?:이|가)? (?:필요합니다|필요함|필요해요|필요|대기)\.?$`)
var unconditionalKoreanAction = regexp.MustCompile(`^(관리자|사용자|소유자|담당자):? ((?:(?:저장소|데이터베이스|프로젝트|배포|서비스|계정) )?(?:접근 권한|권한)|(?:요청된 )?(?:결과의 승인 여부|승인 여부|입력|답변|결정|질문|배포 질문)|배포)(?:을|를|에) (부여|승인|제공|답변|입력|선택|확정)해 주세요\.?$`)
var accessSubject = regexp.MustCompile(`(?i)\b(access|permission|credentials?)\b|권한`)
var decisionSubject = regexp.MustCompile(`(?i)\b(approval|decision|choice|policy)\b|승인|결정`)
var inputSubject = regexp.MustCompile(`(?i)\b(answer|question|input|value)\b|답변|질문|입력`)

// This private in-memory type is never accepted from serialized JSON or a
// freeform "proof" map. Only canonical projection constructors mint evidence.
type humanActionProof struct {
	source, category, request, next, state, handoff, contract string
	condition                                                 map[string]any
	fingerprint                                               string
}

func requestedHumanAction(request, next string) *humanActionProof {
	request = strings.TrimSpace(request)
	next = strings.TrimSpace(next)
	category := ""
	if (presentHumanAssertion.MatchString(request) && unconditionalHumanAction.MatchString(next)) || (presentKoreanAssertion.MatchString(request) && unconditionalKoreanAction.MatchString(next)) {
		required, ok := parseRequestedHumanObject(request)
		performed, actionOK := parsePerformedHumanObject(next)
		if ok && actionOK && required.object == performed.object && required.intent == performed.intent && required.privilege == performed.privilege {
			// A deictic 'requested' action may refer to the one explicit request;
			// an unrelated explicit entity can never be inferred or substituted.
			if required.resource == performed.resource || (required.resource != "" && performed.resource == "" && performed.requested) {
				category = required.intent + ":" + required.resource + ":" + required.object + ":" + required.privilege
			}
		}
	}
	if category == "" {
		return nil
	}
	return &humanActionProof{source: "canonical_requested_action", category: category, request: request, next: next}
}

// A supported sentence is parsed into an entity, object, privilege and intent.
// Topic overlap is never authority. Mixed requirements must describe one and
// the same operation, otherwise they need a more precise canonical request.
type parsedHumanObject struct {
	resource, object, privilege, intent string
	requested                           bool
}

var humanResourceWords = map[string][]string{
	"repository": {"repository", "저장소"}, "database": {"database", "데이터베이스"},
	"billing": {"billing", "청구"}, "deployment": {"deployment", "배포"},
	"project": {"project", "프로젝트"}, "service": {"service", "서비스"}, "account": {"account", "계정"},
}
var humanPurpose = regexp.MustCompile(`(?i) (?:now|to (?:continue|resume) (?:deployment|work|execution|the task))\.?$`)
var koreanHumanPurpose = regexp.MustCompile(`^(?:배포를|작업을|실행을) (?:계속|재개)하려면 `)
var humanActionVerb = regexp.MustCompile(`(?i)^(?:(?:repository )?(?:owner|user|administrator|admin|root|operator):? (?:please )?)?(grant|approve|restore|provide|allow|authorize|answer|reply|choose|confirm|decide|select)s? `)

func parseRequestedHumanObject(request string) (parsedHumanObject, bool) {
	request = koreanHumanPurpose.ReplaceAllString(strings.ToLower(strings.TrimSpace(request)), "")
	request = humanPurpose.ReplaceAllString(request, "")
	clauses := strings.Split(request, "; ")
	var result parsedHumanObject
	for i, clause := range clauses {
		parsed, ok := parseHumanObject(clause, "", false)
		if !ok || (i > 0 && (parsed.resource != result.resource || parsed.object != result.object || parsed.privilege != result.privilege || parsed.intent != result.intent)) {
			return parsedHumanObject{}, false
		}
		result = parsed
	}
	return result, true
}
func parsePerformedHumanObject(action string) (parsedHumanObject, bool) {
	action = strings.ToLower(strings.TrimSpace(action))
	verb := ""
	if match := humanActionVerb.FindStringSubmatch(action); len(match) > 1 {
		verb = match[1]
	}
	if strings.Contains(action, "부여해") {
		verb = "grant"
	} else if strings.Contains(action, "승인해") {
		verb = "approve"
	} else if strings.Contains(action, "답변해") {
		verb = "answer"
	} else if strings.Contains(action, "입력해") || strings.Contains(action, "제공해") {
		verb = "provide"
	} else if strings.Contains(action, "선택해") || strings.Contains(action, "확정해") {
		verb = "decide"
	}
	if verb == "" {
		return parsedHumanObject{}, false
	}
	return parseHumanObject(action, verb, true)
}
func parseHumanObject(text, verb string, action bool) (parsedHumanObject, bool) {
	p := parsedHumanObject{requested: strings.Contains(text, "requested") || strings.Contains(text, "요청된")}
	for canonical, words := range humanResourceWords {
		for _, word := range words {
			if strings.Contains(text, word) {
				if p.resource != "" && p.resource != canonical {
					return p, false
				}
				p.resource = canonical
			}
		}
	}
	access := accessSubject.MatchString(text)
	input := inputSubject.MatchString(text)
	decision := decisionSubject.MatchString(text)
	// An answer about approval is still input; approving input is not answering it.
	if input && (!access) {
		p.object = "input"
		p.intent = "provide_input"
		if action && verb != "answer" && verb != "reply" && verb != "provide" {
			return p, false
		}
	}
	if access {
		p.object = "access"
		p.intent = "grant_access"
		if strings.Contains(text, "credential") {
			p.object = "credentials"
		}
		if strings.Contains(text, "read") {
			p.privilege = "read"
		}
		if strings.Contains(text, "write") {
			if p.privilege != "" {
				return p, false
			}
			p.privilege = "write"
		}
		if !action && decision {
			p.intent = "approve_access"
		}
		if action {
			switch verb {
			case "grant", "restore", "allow":
			case "provide":
				if p.object != "credentials" {
					return p, false
				}
			case "approve", "authorize":
				p.intent = "approve_access"
			default:
				return p, false
			}
		}
	} else if !input && decision {
		p.object = "decision"
		p.intent = "provide_decision"
		if action && verb != "approve" && verb != "choose" && verb != "confirm" && verb != "decide" && verb != "select" && verb != "provide" {
			return p, false
		}
	}
	return p, p.object != ""
}

// Bind every exported field plus the private basis identity, and bind the
// original map instance. Copied proofs cannot authorize a different condition.
func humanConditionFingerprint(condition map[string]any, proof *humanActionProof) (string, error) {
	fields := map[string]any{}
	for key, value := range condition {
		if key != "human_action_basis" {
			fields[key] = value
		}
	}
	body, err := json.Marshal(struct {
		Fields map[string]any
		Basis  []string
	}{fields, []string{proof.source, proof.category, proof.request, proof.next, proof.state, proof.handoff, proof.contract}})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
func sealHumanActionCondition(condition map[string]any) {
	proof, ok := condition["human_action_basis"].(*humanActionProof)
	if !ok || proof == nil {
		return
	}
	copy := *proof
	copy.condition = condition
	copy.fingerprint = ""
	condition["human_action_basis"] = &copy
	fingerprint, err := humanConditionFingerprint(condition, &copy)
	if err != nil {
		condition["human_action_verified"] = false
		return
	}
	copy.fingerprint = fingerprint
}

func controllerWaitBasis(review map[string]any, markdown string, now time.Time) *humanActionProof {
	// This reason/quality pair is produced only after the completion projector
	// validates the exact claimed Controller execution. A needs_user label or
	// generic diagnostic (including a handoff hold label) cannot supply it.
	since, err := time.Parse(time.RFC3339Nano, toString(review["since"]))
	if review["state"] != "needs_user" || review["reason"] != "exact_controller_claim_running" || review["observation_quality"] != "observed" || !completionContractMatches(markdown, toString(review["contract_sha256"])) || err != nil || since.After(now) || !meaningfulHumanAction(toString(review["handoff_id"])) || !meaningfulHumanAction(toString(review["contract_sha256"])) {
		return nil
	}
	return &humanActionProof{source: "observed_controller_wait", handoff: toString(review["handoff_id"]), contract: toString(review["contract_sha256"])}
}

// Finalize is retained internal lifecycle evidence. External senders still use
// HumanActionCondition, so completion-review events never bypass the human gate.
func recordableNotificationConditionForTask(task string, condition map[string]any) bool {
	if kind := toString(condition["kind"]); kind == "intervention" || kind == "approval" || kind == "interrupted" {
		return toString(condition["task_id"]) == task && HumanActionCondition(condition)
	}
	return recordableNotificationCondition(condition)
}

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
	Condition           map[string]any
	EventID             string
	Since               string
	LegacyNotBefore     string
	LegacyPreviousSince string
}

// CurrentHumanActionDecisions re-reads the canonical inputs at the send boundary.
// NotificationEvents owns the shared cause episode clock. Both external paths
// use its ID, so the existing transport lock deduplicates concurrent senders.
// Historical events and sent channel records are never rewritten or replayed.
func CurrentHumanActionDecisions(project, root string) (map[string]HumanActionDecision, error) {
	release, err := projectguard.AcquireStateLock(project, "notification_state")
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
	if _, err := notificationEventsLocked(project, items); err != nil {
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
	Key         string `json:"key,omitempty"`
	Since       string `json:"since,omitempty"`
	OriginSince string `json:"origin_since,omitempty"`
	NotBefore   string `json:"not_before,omitempty"`
	EventID     string `json:"event_id,omitempty"`
	Assignment  string `json:"assignment_id,omitempty"`
	Uncertain   bool   `json:"uncertain,omitempty"`
	Checksum    string `json:"checksum"`
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
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&state); err != nil {
			return err
		}
		if err = decoder.Decode(new(any)); err != io.EOF {
			return errors.New("extra human action boundary JSON")
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
		if seen {
			if err := validateHumanActionBoundary(project, id, old, journal, decisions[id], items[id], signals[id], now); err != nil {
				return err
			}
		}
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
				next.Checksum = humanBoundaryChecksum(next)
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
			if old.Key == "" && old.NotBefore != "" {
				next.NotBefore = decision.Since
			}
			next.OriginSince = decision.Since
			if old.Key == key && old.EventID == decision.EventID && old.OriginSince != "" {
				next.OriginSince = old.OriginSince
			}
			previous := previousHumanEpisode(id, key, next.OriginSince, journal)
			if previous != "" {
				next.NotBefore = next.OriginSince
			}
			decision.LegacyPreviousSince = previous
			next.Assignment, next.EventID, next.Uncertain = assignment, decision.EventID, false
			next.Key, next.Since = key, decision.Since
			decision.LegacyNotBefore = next.NotBefore
			decisions[id] = decision
		} else if old.Key != "" {
			if authoritativeHumanCauseClear(old.Key, items[id], signals[id]) {
				next.Key, next.Since, next.EventID, next.OriginSince = "", "", "", ""
				next.NotBefore = now.Format(time.RFC3339Nano)
				next.Uncertain = false
			} else {
				next.Uncertain = true
			}
		}
		next.Checksum = humanBoundaryChecksum(next)
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
	return persistHumanActionState(path, append(body, '\n'))
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

func HumanActionRequiresRuntime(condition map[string]any) bool {
	proof, ok := condition["human_action_basis"].(*humanActionProof)
	return ok && proof != nil && proof.source == "observed_current_execution"
}

func humanBoundaryChecksum(value humanActionBoundary) string {
	value.Checksum = ""
	body, _ := json.Marshal(value)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func validateHumanActionBoundary(project, task string, b humanActionBoundary, journal notificationJournal, current HumanActionDecision, item, signal map[string]any, now time.Time) error {
	bad := func() error { return errors.New("inconsistent human action boundary tuple") }
	if b.Checksum == "" || b.Checksum != humanBoundaryChecksum(b) {
		return bad()
	}
	if b.Key == "" {
		if b.Since != "" || b.EventID != "" || b.OriginSince != "" {
			return bad()
		}
		return nil
	}
	if b.Since == "" || b.EventID == "" || b.OriginSince == "" {
		return bad()
	}
	checkpoint, err := time.Parse(time.RFC3339Nano, b.Since)
	if err != nil || checkpoint.After(now) {
		return bad()
	}
	originTime, e := time.Parse(time.RFC3339Nano, b.OriginSince)
	if e != nil || originTime.After(checkpoint) {
		return bad()
	}
	previous := previousHumanEpisode(task, b.Key, b.OriginSince, journal)
	if previous != "" && b.NotBefore != b.OriginSince {
		return bad()
	}
	if b.NotBefore != "" && b.NotBefore != b.OriginSince {
		return bad()
	}
	parts := strings.Split(b.Key, "\x00")
	if len(parts) < 2 || (b.Assignment != "" && (len(parts) != 3 || parts[2] != b.Assignment)) || (b.Assignment == "" && len(parts) != 2) {
		return bad()
	}
	foundCheckpoint := false
	observation, hasObservation := journal.Items[task]
	if hasObservation && observation.ConditionKey == b.Key && observation.ConditionSince == b.Since {
		foundCheckpoint = true
	}
	var origin map[string]any
	gap := false
	for _, event := range journal.Events {
		if toString(event["task_id"]) != task {
			continue
		}
		if toString(event["id"]) == b.EventID {
			origin = event
		}
		if toString(event["condition_key"]) == b.Key && toString(event["condition_since"]) == b.Since && toString(event["assignment_id"]) == b.Assignment {
			foundCheckpoint = true
		}
		since, e := time.Parse(time.RFC3339Nano, toString(event["condition_since"]))
		if e == nil && !since.Before(checkpoint) && toString(event["assignment_id"]) == "" && sameHumanCauseKey(b.Key, toString(event["condition_key"])) {
			gap = true
		}
	}
	if origin != nil {
		if !HumanActionKind(toString(origin["kind"])) {
			return bad()
		}
		if toString(origin["condition_key"]) != "" {
			if toString(origin["condition_key"]) != b.Key || toString(origin["assignment_id"]) != b.Assignment || !foundCheckpoint {
				return bad()
			}
			since, e := time.Parse(time.RFC3339Nano, toString(origin["condition_since"]))
			if e != nil || since.After(checkpoint) || !since.Equal(originTime) {
				return bad()
			}
			if b.NotBefore != "" {
				floor, e := time.Parse(time.RFC3339Nano, b.NotBefore)
				if e != nil || since.Before(floor) {
					return bad()
				}
			}
			if !since.Equal(checkpoint) {
				bridge := false
				for _, event := range journal.Events {
					if toString(event["task_id"]) != task {
						continue
					}
					at, e := time.Parse(time.RFC3339Nano, toString(event["condition_since"]))
					if e != nil || !at.After(since) || at.After(checkpoint) {
						continue
					}
					if toString(event["kind"]) == "completed" || !sameHumanCauseKey(b.Key, toString(event["condition_key"])) {
						return bad()
					}
					if toString(event["assignment_id"]) == "" {
						bridge = true
					}
				}
				if !bridge {
					return bad()
				}
			}
		} else {
			// A retained pre-schema canonical event must be the journal's explicit
			// migration alias, not an arbitrary transport/source ID supplied here.
			if !hasObservation || observation.ConditionEventID != b.EventID || observation.ConditionKey != b.Key || observation.ConditionSince != b.Since {
				return bad()
			}
		}
	} else {
		// A consumed cold-start baseline has a persisted canonical observation but
		// deliberately no outbound event. Its identity is deterministic, not caller
		// supplied. Existing historical assignment facts permit a later transition.
		if b.EventID != humanActionEventID(task, b.Key, b.OriginSince) || b.Since != b.OriginSince {
			return bad()
		}
		if !foundCheckpoint {
			// A last-known consumed baseline may legitimately be superseded by
			// a NEW canonical episode. Its checksum and deterministic identity
			// are mandatory; the successor must be independently persisted.
			successor := false
			for _, event := range journal.Events {
				at, e := time.Parse(time.RFC3339Nano, toString(event["condition_since"]))
				if e == nil && at.After(checkpoint) && toString(event["task_id"]) == task && toString(event["condition_key"]) != "" {
					successor = true
				}
				completed, e := time.Parse(time.RFC3339Nano, toString(event["at"]))
				if e == nil && completed.After(checkpoint) && toString(event["task_id"]) == task && toString(event["kind"]) == "completed" {
					successor = true
				}
			}
			if !successor && !authoritativeHumanCauseClear(b.Key, item, signal) {
				return bad()
			}
			if b.Assignment != "" {
				assignment, e := runtimeobs.FindAssignment(project, b.Assignment)
				assigned, te := time.Parse(time.RFC3339Nano, assignment.AssignedAt)
				if e != nil || te != nil || assignment.TaskID != task || assigned.After(originTime) {
					return bad()
				}
			}
		}
	}
	if b.Uncertain && current.Condition != nil && toString(current.Condition["key"]) == b.Key && toString(current.Condition["assignment_id"]) == b.Assignment && !gap {
		return bad()
	}
	return nil
}

// The commit point is rename. Temp file Sync and directory Sync establish the
// normal filesystem durability boundary; a post-rename sync error suppresses
// delivery and retains a complete committed record, never a partial overwrite.
var humanStateFileSync = func(file *os.File) error { return file.Sync() }
var humanStateDirectorySync = func(dir *os.File) error { return dir.Sync() }
var persistHumanActionState = writeHumanActionState

func writeHumanActionState(path string, body []byte) error {
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	temp, err := os.CreateTemp(filepath.Dir(path), ".human-action-state-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err = temp.Chmod(0600); err == nil {
		_, err = temp.Write(body)
	}
	if err == nil {
		err = humanStateFileSync(temp)
	}
	closeErr := temp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	return humanStateDirectorySync(dir)
}

// An earlier canonical cause/completion is independent evidence that this is
// not the first episode. Identity-collection gaps alone are not recovery.
func previousHumanEpisode(task, key, origin string, journal notificationJournal) string {
	start, e := time.Parse(time.RFC3339Nano, origin)
	if e != nil {
		return ""
	}
	var latest time.Time
	for _, event := range journal.Events {
		if toString(event["task_id"]) != task {
			continue
		}
		if !HumanActionKind(toString(event["kind"])) && toString(event["kind"]) != "completed" {
			continue
		}
		raw := toString(event["condition_since"])
		if raw == "" {
			raw = toString(event["at"])
		}
		at, e := time.Parse(time.RFC3339Nano, raw)
		if e != nil || !at.Before(start) {
			continue
		}
		// Legacy completion stamps have second precision. They cannot prove
		// ordering against an origin inside that same second.
		if toString(event["kind"]) == "completed" && !at.Add(time.Second).Before(start) {
			continue
		}
		if toString(event["kind"]) != "completed" && toString(event["assignment_id"]) == "" && sameHumanCauseKey(key, toString(event["condition_key"])) {
			continue
		}
		if at.After(latest) {
			latest = at
		}
	}
	if latest.IsZero() {
		return ""
	}
	return latest.Format(time.RFC3339Nano)
}
