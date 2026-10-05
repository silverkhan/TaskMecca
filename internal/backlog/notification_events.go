package backlog

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "os"
    "path/filepath"
    "sync"
    "time"
)

const notificationJournalVersion = 2

var notificationJournalMu sync.Mutex

type notificationObservation struct {
    FileState    string `json:"file_state"`
    UpdatedAt    string `json:"updated_at,omitempty"`
    ConditionKey string `json:"condition_key,omitempty"`
    StartedAt    string `json:"started_at,omitempty"`
}

type notificationJournal struct {
    Version int `json:"version"`
    Items   map[string]notificationObservation `json:"items"`
    Events  []map[string]any `json:"events"`
}

func notificationEventID(taskID, kind, at string) string {
    sum := sha256.Sum256([]byte(taskID + "\x00" + kind + "\x00" + at))
    return hex.EncodeToString(sum[:12])
}

func notificationEventExists(events []map[string]any, id string) bool {
    for _, event := range events {
        if toString(event["id"]) == id {
            return true
        }
    }
    return false
}

func ReadNotificationEvents(project string) ([]map[string]any, error) {
    notificationJournalMu.Lock()
    defer notificationJournalMu.Unlock()

    path := filepath.Join(project, "_task_mecca", ".runtime", "notification_events.json")
    data, err := os.ReadFile(path)
    if os.IsNotExist(err) {
        return []map[string]any{}, nil
    }
    if err != nil {
        return nil, err
    }

    journal := notificationJournal{}
    if err := json.Unmarshal(data, &journal); err != nil {
        return nil, err
    }
    if journal.Events == nil {
        return []map[string]any{}, nil
    }
    return append([]map[string]any{}, journal.Events...), nil
}

func NotificationEvents(project string, items map[string]map[string]any) ([]map[string]any, error) {
    notificationJournalMu.Lock()
    defer notificationJournalMu.Unlock()

    path := filepath.Join(project, "_task_mecca", ".runtime", "notification_events.json")
    journal := notificationJournal{
        Version: notificationJournalVersion,
        Items:   map[string]notificationObservation{},
        Events:  []map[string]any{},
    }

    loaded := false
    itemsPresent := true
    data, err := os.ReadFile(path)
    switch {
    case err == nil:
        loaded = true
        if err := json.Unmarshal(data, &journal); err != nil {
            return nil, err
        }
        itemsPresent = journal.Items != nil
        if journal.Items == nil {
            journal.Items = map[string]notificationObservation{}
        }
        if journal.Events == nil {
            journal.Events = []map[string]any{}
        }
    case os.IsNotExist(err):
        itemsPresent = false
    default:
        return nil, err
    }

    // A missing journal, an older schema, or a journal without observations is
    // a baseline pass. Reconciliation may recover historical lifecycle/runtime
    // truth, but baseline state must never be emitted as a new notification.
    baseline := !loaded || journal.Version < notificationJournalVersion || !itemsPresent
    dirty := false
    if journal.Version != notificationJournalVersion {
        journal.Version = notificationJournalVersion
        dirty = true
    }

    now := time.Now().Format(time.RFC3339Nano)
    for id, item := range items {
        fileState := toString(item["file_state"])
        updatedAt := toString(item["updated_at"])
        previous, seen := journal.Items[id]
        registeredNow := false

        if !baseline && !seen {
            at := updatedAt
            if at == "" {
                at = now
            }
            eventID := notificationEventID(id, "registered", at)
            if !notificationEventExists(journal.Events, eventID) {
                journal.Events = append(journal.Events, map[string]any{
                    "id": eventID, "task_id": id, "kind": "registered", "at": at,
                    "title": toString(item["title"]), "task_updated_at": updatedAt,
                })
                dirty = true
            }
            registeredNow = true
        }

        if !baseline && seen && fileState == "done" && previous.FileState != "done" {
            at := ""
            if lifecycle, ok := item["lifecycle"].(map[string]any); ok {
                at = toString(lifecycle["completed_at"])
            }
            if at == "" {
                at = toString(item["completed_at"])
            }
            if at == "" {
                at = updatedAt
            }
            if at == "" {
                at = now
            }
            eventID := notificationEventID(id, "completed", at)
            if !notificationEventExists(journal.Events, eventID) {
                journal.Events = append(journal.Events, map[string]any{
                    "id": eventID,
                    "task_id": id,
                    "kind": "completed",
                    "at": at,
                    "title": toString(item["title"]),
                    "task_updated_at": updatedAt,
                })
                dirty = true
            }
        }

        // Notification is a consumer of the canonical lifecycle. Historical
        // recovery bindings remain valid lifecycle evidence, but the control
        // tower marks them notification-suppressed so they cannot backfill
        // "started" messages.
        startedAt := ""
        startedSuppressed := false
        if lifecycle, ok := item["lifecycle"].(map[string]any); ok {
            startedAt = toString(lifecycle["started_at"])
            if suppressed, ok := lifecycle["started_notification_suppressed"].(bool); ok {
                startedSuppressed = suppressed
            }
        }
        startedChanged := seen && previous.StartedAt != startedAt
        if !baseline && startedAt != "" && !startedSuppressed && (startedChanged || (!seen && registeredNow)) {
            eventID := notificationEventID(id, "started", startedAt)
            if !notificationEventExists(journal.Events, eventID) {
                journal.Events = append(journal.Events, map[string]any{
                    "id": eventID, "task_id": id, "kind": "started", "at": startedAt,
                    "title": toString(item["title"]), "message": "작업이 착수 상태로 전환되었습니다.",
                    "task_updated_at": updatedAt,
                })
                dirty = true
            }
        }

        conditionKey := ""
        if condition, ok := item["notification_condition"].(map[string]any); ok && len(condition) > 0 {
            kind := toString(condition["kind"])
            conditionKey = toString(condition["key"])
            if conditionKey == "" {
                conditionKey = kind + "\x00" + toString(condition["reason_type"])
            }

            // Operational conditions have no trustworthy historical transition
            // timestamp. Only changes from an already-observed canonical state
            // are live notification episodes; first observation is baseline.
            if !baseline && seen && kind != "" && previous.ConditionKey != conditionKey {
                at := now
                eventID := notificationEventID(id, kind, at)
                if !notificationEventExists(journal.Events, eventID) {
                    journal.Events = append(journal.Events, map[string]any{
                        "id": eventID, "task_id": id, "kind": kind, "at": at,
                        "title": toString(item["title"]),
                        "reason_type": toString(condition["reason_type"]),
                        "message": toString(condition["message"]),
                        "resume_condition": toString(condition["resume_condition"]),
                        "attempt_id": toString(condition["attempt_id"]),
                        "runtime_state": toString(condition["runtime_state"]),
                        "task_updated_at": updatedAt,
                    })
                    dirty = true
                }
            }
        }

        next := notificationObservation{
            FileState: fileState, UpdatedAt: updatedAt, ConditionKey: conditionKey, StartedAt: startedAt,
        }
        if !seen || previous != next {
            journal.Items[id] = next
            dirty = true
        }
    }

    if len(journal.Events) > 250 {
        journal.Events = append([]map[string]any{}, journal.Events[len(journal.Events)-250:]...)
        dirty = true
    }

    if !dirty {
        return append([]map[string]any{}, journal.Events...), nil
    }
    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
        return nil, err
    }
    encoded, err := json.MarshalIndent(journal, "", "  ")
    if err != nil {
        return nil, err
    }
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, append(encoded, '\n'), 0644); err != nil {
        return nil, err
    }
    if err := os.Rename(tmp, path); err != nil {
        return nil, err
    }

    return append([]map[string]any{}, journal.Events...), nil
}
