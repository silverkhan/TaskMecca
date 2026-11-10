package backlog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// RecoveryEventSnapshot provides a read-only, deterministic view of previously
// stranded work when an orchestration event occurs. It never resumes an agent,
// changes a backlog state, records a lifecycle event, or sends a notification.
// The caller (Root/Controller) must verify native liveness before acting.
func RecoveryEventSnapshot(project, root, event, triggerTask string) (map[string]any, error) {
	event = strings.TrimSpace(event)
	triggerTask = strings.TrimSpace(triggerTask)
	switch event {
	case "registration", "controller_start", "controller_resume", "worker_report":
	default:
		return nil, fmt.Errorf("unsupported recovery event %q", event)
	}
	if (event == "registration" || event == "worker_report") && triggerTask == "" {
		return nil, fmt.Errorf("%s requires an exact trigger task ID", event)
	}

	// Coordinate reconciles *all* active doing tasks. The trigger is not a
	// filter: otherwise an unrelated, older orphan could remain invisible.
	coord, err := Coordinate(project, root, 3)
	if err != nil {
		return nil, err
	}
	raw, ok := coord["recovery_queue"].([]map[string]any)
	if !ok {
		return nil, fmt.Errorf("coordinate recovery_queue has an unexpected shape")
	}
	items := make([]map[string]any, 0, len(raw))
	seen := make(map[string]bool)
	requiresUser := 0
	for _, gap := range raw {
		id := strings.TrimSpace(toString(gap["id"]))
		if id == "" || seen[id] || (event == "registration" && id == triggerTask) {
			continue
		}
		seen[id] = true
		code := toString(gap["code"])
		health := toString(gap["health"])
		action := toString(gap["action"])
		needsUser := code == "user_decision_required_backlog_doing"
		if needsUser {
			requiresUser++
		}
		recovery, _ := gap["recovery"].(map[string]any)
		dirtyCount := 0
		if recovery != nil {
			dirtyCount, _ = recovery["uncommitted_change_count"].(int)
		}
		identity := id + "\x00" + code + "\x00" + health + "\x00" + toString(gap["agent"])
		fingerprint := sha256.Sum256([]byte(identity))
		items = append(items, map[string]any{
			"id": id,
			"title": toString(gap["title"]),
			"code": code,
			"health": health,
			"assigned_agent": toString(gap["agent"]),
			"action": action,
			"requires_user": needsUser,
			"requires_native_verification": true,
			"uncommitted_change_count": dirtyCount,
			"incident_key": hex.EncodeToString(fingerprint[:16]),
		})
	}
	// Controller startup and worker-report events must also discover existing
	// explicit user holds, not only orphaned doing tasks. The caller then
	// invokes the existing flow next decision function for each such hold.
	// A read-only scan never sends a message or treats a recorded dispatch as
	// an unprocessed user decision.
	catalog, err := Catalog(project, root)
	if err != nil { return nil, err }
	userHolds := make([]map[string]any, 0)
	seenHolds := make(map[string]bool)
	for _, row := range catalog {
		if row.Location!="active" || row.State!="hold" ||
			!strings.EqualFold(strings.TrimSpace(row.Fields["대기유형"]),"user") {
			continue
		}
		if seenHolds[row.ID] { continue }
		seenHolds[row.ID]=true
		reason:=strings.TrimSpace(row.Fields["대기"])
		resume:=strings.TrimSpace(row.Fields["재개조건"])
		complete:=reason!="" && reason!="-" && resume!="" && resume!="-"
		action:="complete_canonical_user_hold_first"
		if complete { action="flow_next:user_decision_required" }
		userHolds=append(userHolds,map[string]any{
			"id":row.ID, "title":row.Title,
			"evidence_complete":complete,
			"next_action":action,
			"read_only":true,
		})
	}
	sort.Slice(userHolds,func(i,j int)bool {
		return toString(userHolds[i]["id"]) < toString(userHolds[j]["id"])
	})
	sort.Slice(items, func(i, j int) bool {
		return toString(items[i]["id"]) < toString(items[j]["id"])
	})
	// A repeated event against unchanged evidence has the same digest. This
	// allows callers to suppress duplicate reports without changing the
	// canonical lifecycle or pretending that notification was delivered.
	fingerprintParts := []string{event, triggerTask}
	for _, item := range items {
		fingerprintParts = append(fingerprintParts, toString(item["incident_key"]))
	}
	for _, hold := range userHolds {
		fingerprintParts=append(fingerprintParts,"user_hold:"+toString(hold["id"])+":"+toString(hold["next_action"]))
	}
	digest := sha256.Sum256([]byte(strings.Join(fingerprintParts, "\x00")))
	return map[string]any{
		"status": func() string {
			if len(items) > 0 { return "candidates_found" }
			if len(userHolds) > 0 {return "user_holds_found"}
			return "no_confirmed_candidates"
		}(),
		"event": event,
		"trigger_task": triggerTask,
		"source": "coordinate.recovery_queue",
		"scope": "all_active_doing",
		"read_only": true,
		"candidate_count": len(items),
		"requires_user_count": requiresUser,
		"pending_user_hold_count":len(userHolds),
		"pending_user_holds":userHolds,
		"candidates": items,
		"report_to_root": event == "registration" && len(items) > 0,
		"event_fingerprint": hex.EncodeToString(digest[:16]),
		"notice": "Candidates are observations, not proof of a dead agent or authorization to resume. Verify exact native runtime state, contract, preflight, and preserved changes before any action.",
	}, nil
}
