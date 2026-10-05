package backlog

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
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
    SignalIDs []string `json:"signal_ids,omitempty"`
    SignalsInitialized bool `json:"signals_initialized,omitempty"`
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
    // Persist even an empty first baseline. Otherwise a service that starts
    // before Registrar creates the first backlog keeps looking "uninitialized"
    // to the notification journal and suppresses that first genuine registration.
    dirty := !loaded || !itemsPresent
    if journal.Version != notificationJournalVersion {
        journal.Version = notificationJournalVersion
        dirty = true
    }

    now := time.Now().Format(time.RFC3339Nano)
    for id, item := range items {
        fileState := toString(item["file_state"])
        updatedAt := toString(item["updated_at"])
        previous, seen := journal.Items[id]
        lifecycle, _ := item["lifecycle"].(map[string]any)
        registeredNow := false

        if !baseline && !seen {
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
        }

        // Notification is a consumer of the canonical lifecycle. Historical
        // recovery bindings remain valid lifecycle evidence, but the control
        // tower marks them notification-suppressed so they cannot backfill
        // "started" messages.
        startedAt := ""
        startedSuppressed := false
        if lifecycle != nil {
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
            conditionChanged := (seen && previous.ConditionKey != conditionKey) || (!seen && registeredNow)
            if !baseline && kind != "" && conditionChanged {
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

        completedChanged := fileState == "done" &&
            ((seen && previous.FileState != "done") || (!seen && registeredNow))
        if !baseline && completedChanged {
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
        }

        next := notificationObservation{
            FileState: fileState, UpdatedAt: updatedAt, ConditionKey: conditionKey, StartedAt: startedAt,
        }
        if !seen || previous != next {
            journal.Items[id] = next
            dirty = true
        }
    }

    signalSeen:=map[string]bool{}
    for _,id:=range journal.SignalIDs { signalSeen[id]=true }
    signals,signalErr:=runtimeobs.ReadControlSignals(project,250)
    if signalErr==nil {
        initialized:=journal.SignalsInitialized
        for _,signal:=range signals {
            if signal.ID=="" || signalSeen[signal.ID] { continue }
            signalSeen[signal.ID]=true
            journal.SignalIDs=append(journal.SignalIDs,signal.ID)
            dirty=true
            // Existing signals establish their own baseline independently of
            // task lifecycle observations. This prevents an upgrade/restart
            // from replaying old approval prompts.
            if !initialized || baseline { continue }
            if signal.Kind!="approval" { continue }

            title:=""
            if item:=items[strings.ToUpper(strings.TrimSpace(signal.TaskID))];item!=nil {
                title=toString(item["title"])
            }
            if title=="" {
                provider:=strings.TrimSpace(signal.Provider)
                if provider=="" { provider="runtime" }
                title=strings.ToUpper(provider[:1])+provider[1:]+" 승인 요청"
            }
            message:=strings.TrimSpace(signal.Message)
            if signal.ToolName!="" {
                if message!="" { message+=" " }
                message+="요청 도구: "+signal.ToolName
            }
            eventID:="runtime-signal-"+signal.ID
            if !notificationEventExists(journal.Events,eventID) {
                journal.Events=append(journal.Events,map[string]any{
                    "id":eventID,"task_id":strings.ToUpper(strings.TrimSpace(signal.TaskID)),
                    "kind":"approval","at":signal.ObservedAt,"title":title,
                    "message":message,
                    "resume_condition":"에이전트 앱에서 승인 요청을 확인하고 필요한 결정을 내려주세요.",
                    "attempt_id":signal.AttemptID,"runtime_state":"waiting_approval",
                    "provider":signal.Provider,"session_id":signal.SessionID,
                })
                dirty=true
            }
        }
        if !journal.SignalsInitialized {
            journal.SignalsInitialized=true
            dirty=true
        }
    }
    if len(journal.SignalIDs)>500 {
        journal.SignalIDs=append([]string{},journal.SignalIDs[len(journal.SignalIDs)-500:]...)
        dirty=true
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
