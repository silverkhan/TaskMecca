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

const notificationJournalVersion = 4
const runtimeUnknownNotificationGrace = 60 * time.Second
const runtimeUnknownStartupNotificationGrace = 2 * time.Minute

var notificationJournalMu sync.Mutex

type notificationObservation struct {
    FileState         string `json:"file_state"`
    UpdatedAt         string `json:"updated_at,omitempty"`
    ConditionKey      string `json:"condition_key,omitempty"`
    ConditionSince    string `json:"condition_since,omitempty"`
    ConditionConsumed bool   `json:"condition_consumed,omitempty"`
    StartedAt         string `json:"started_at,omitempty"`
}

type notificationJournal struct {
    Version int `json:"version"`
    Items   map[string]notificationObservation `json:"items"`
    Phases  map[string]int `json:"phases,omitempty"`
    Events  []map[string]any `json:"events"`
}

func notificationLifecyclePhase(kind string) int {
    switch kind {
    case "registered":
        return 10
    case "started":
        return 20
    case "finalize":
        return 30
    case "completed":
        return 40
    default:
        return 0
    }
}

func maxNotificationPhase(a,b int) int {
    if b>a { return b }
    return a
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
        Phases:  map[string]int{},
        Events:  []map[string]any{},
    }

    loaded := false
    itemsPresent := true
    phasesPresent := true
    data, err := os.ReadFile(path)
    switch {
    case err == nil:
        loaded = true
        if err := json.Unmarshal(data, &journal); err != nil {
            return nil, err
        }
        itemsPresent = journal.Items != nil
        phasesPresent = journal.Phases != nil
        if journal.Items == nil {
            journal.Items = map[string]notificationObservation{}
        }
        if journal.Phases == nil {
            journal.Phases = map[string]int{}
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
    baseline := !loaded || journal.Version < notificationJournalVersion || !itemsPresent || !phasesPresent
    // Persist even an empty first baseline. Otherwise a service that starts
    // before Registrar creates the first backlog keeps looking "uninitialized"
    // to the notification journal and suppresses that first genuine registration.
    dirty := !loaded || !itemsPresent || !phasesPresent
    if journal.Version != notificationJournalVersion {
        journal.Version = notificationJournalVersion
        dirty = true
    }

    // Rebuild durable per-task lifecycle floors from the retained event history
    // and the last observed item state. This makes schema migration and partial
    // observation loss monotonic instead of replaying older lifecycle stages.
    for _,event:=range journal.Events {
        id:=toString(event["task_id"])
        if id=="" { continue }
        rank:=notificationLifecyclePhase(toString(event["kind"]))
        if rank>journal.Phases[id] { journal.Phases[id]=rank; dirty=true }
    }
    for id,obs:=range journal.Items {
        rank:=10
        if obs.StartedAt!="" { rank=20 }
        if obs.FileState=="done" { rank=40 }
        if rank>journal.Phases[id] { journal.Phases[id]=rank; dirty=true }
    }

    nowTime := time.Now()
    now := nowTime.Format(time.RFC3339Nano)
    for id, item := range items {
        fileState := toString(item["file_state"])
        updatedAt := toString(item["updated_at"])
        previous, seen := journal.Items[id]
        lifecycle, _ := item["lifecycle"].(map[string]any)
        startedAt := ""
        startedSuppressed := false
        if lifecycle != nil {
            startedAt = toString(lifecycle["started_at"])
            if suppressed, ok := lifecycle["started_notification_suppressed"].(bool); ok {
                startedSuppressed = suppressed
            }
        }

        phase:=journal.Phases[id]
        if previous.FileState=="done" { phase=maxNotificationPhase(phase,40) }
        if baseline {
            phase=maxNotificationPhase(phase,10)
            if startedAt!="" { phase=maxNotificationPhase(phase,20) }
            if fileState=="done" { phase=maxNotificationPhase(phase,40) }
        }
        registeredNow := false

        if !baseline && !seen && phase < 10 {
            at := toString(lifecycle["created_at"])
            if at == "" {
                at = updatedAt
            }
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
            phase=maxNotificationPhase(phase,10)
        }

        // Notification is a consumer of the canonical lifecycle. Historical
        // recovery bindings remain valid lifecycle evidence, but notifications
        // must never regress after a later lifecycle stage was already observed.
        startedChanged := seen && previous.StartedAt != startedAt
        if !baseline && phase < 20 && startedAt != "" && !startedSuppressed && (startedChanged || !seen) {
            eventID := notificationEventID(id, "started", startedAt)
            if !notificationEventExists(journal.Events, eventID) {
                journal.Events = append(journal.Events, map[string]any{
                    "id": eventID, "task_id": id, "kind": "started", "at": startedAt,
                    "title": toString(item["title"]), "message": "작업이 착수 상태로 전환되었습니다.",
                    "task_updated_at": updatedAt,
                })
                dirty = true
            }
            phase=maxNotificationPhase(phase,20)
        }

        conditionKey := ""
        conditionSince := ""
        conditionConsumed := false
        if fileState!="done" && phase < 40 {
        if condition, ok := item["notification_condition"].(map[string]any); ok && len(condition) > 0 {
            kind := toString(condition["kind"])
            conditionKey = toString(condition["key"])
            if conditionKey == "" {
                conditionKey = kind + "\x00" + toString(condition["reason_type"])
            }

            // User-actionable conditions are emitted immediately when a new
            // episode is observed. runtime_unknown is different: it represents
            // uncertainty in sensing, not a confirmed failure. Keep it visible
            // in the UI immediately, but require a stable dwell before pushing
            // an external alert so short hook gaps do not create false alarms.
            conditionChanged := (seen && previous.ConditionKey != conditionKey) || (!seen && registeredNow)
            if conditionChanged {
                conditionSince = now
                conditionConsumed = false
            } else if seen {
                conditionSince = previous.ConditionSince
                conditionConsumed = previous.ConditionConsumed
                if conditionSince == "" {
                    conditionSince = now
                }
            } else {
                conditionSince = now
            }

            if baseline {
                conditionConsumed = true
            } else if kind != "" && !conditionConsumed {
                emit := false
                at := now
                if kind == "runtime_unknown" {
                    if since, parseErr := time.Parse(time.RFC3339Nano, conditionSince); parseErr == nil {
                        grace:=runtimeUnknownNotificationGrace
                        // Before canonical start is established, runtime_unknown
                        // commonly represents correlation/startup convergence.
                        // Give that path a longer dwell so a normally starting
                        // worker does not emit a false external warning.
                        if startedAt=="" { grace=runtimeUnknownStartupNotificationGrace }
                        threshold := since.Add(grace)
                        if !nowTime.Before(threshold) {
                            emit = true
                            at = threshold.Format(time.RFC3339Nano)
                        }
                    }
                } else if conditionChanged || !conditionConsumed {
                    emit = true
                }

                if emit {
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
                    conditionConsumed = true
                    if kind=="finalize" { phase=maxNotificationPhase(phase,30) }
                }
            }
        }
        } else {
            conditionConsumed = true
        }

        completedChanged := fileState == "done" &&
            ((seen && previous.FileState != "done") || (!seen && phase < 40))
        if !baseline && phase < 40 && completedChanged {
            at := toString(lifecycle["completed_at"])
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
            phase=maxNotificationPhase(phase,40)
        }

        next := notificationObservation{
            FileState: fileState, UpdatedAt: updatedAt, ConditionKey: conditionKey,
            ConditionSince: conditionSince, ConditionConsumed: conditionConsumed, StartedAt: startedAt,
        }
        if !seen || previous != next {
            journal.Items[id] = next
            dirty = true
        }
        if phase>journal.Phases[id] {
            journal.Phases[id]=phase
            dirty=true
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
