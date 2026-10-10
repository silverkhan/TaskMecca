package webui

import (
	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/notify"
	"sort"
)

// Notification history reads existing evidence without reconciling or sending.
// An event alone is not evidence that any channel accepted a notification.
func notificationHistory(project string) ([]map[string]any, error) {
	events, err := backlog.ReadNotificationEvents(project)
	if err != nil {
		return nil, err
	}
	records, err := notify.DeliveryRecords(project, "", "")
	if err != nil {
		return nil, err
	}
	byID := map[string]map[string]any{}
	for _, event := range events {
		id, _ := event["id"].(string)
		if id == "" {
			continue
		}
		byID[id] = map[string]any{"event_id": id, "project": project, "task_id": event["task_id"], "kind": event["kind"], "event_at": event["at"], "title": event["title"], "message": event["message"], "telegram": map[string]any{"state": "unknown"}}
	}
	for _, record := range records {
		row := byID[record.EventID]
		if row == nil {
			row = map[string]any{"event_id": record.EventID, "project": project, "task_id": record.TaskID, "kind": record.Kind, "event_at": record.EventAt}
			byID[record.EventID] = row
		}
		// Do not include LastError, opaque ledger fields or recipient identity.
		row["telegram"] = map[string]any{"state": record.State, "attempts": record.Attempts, "sent_at": record.LastResponseAt, "last_attempt_at": record.LastAttemptAt, "duplicate_possible": record.DuplicatePossible}
	}
	// Browser delivery evidence is shared across devices; per-browser local
	// history is only a fallback and must never overwrite this server result.
	webRecords, err := webDeliveryHistory(project)
	if err != nil {
		return nil, err
	}
	for id, delivery := range webRecords {
		if row := byID[id]; row != nil {
			row["web"] = delivery
		}
	}
	rows := make([]map[string]any, 0, len(byID))
	for _, row := range byID {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, _ := rows[i]["event_at"].(string)
		b, _ := rows[j]["event_at"].(string)
		if a == b {
			return historyEventID(rows[i]) < historyEventID(rows[j])
		}
		return a > b
	})
	return rows, nil
}
