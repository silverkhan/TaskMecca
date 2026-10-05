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

var notificationJournalMu sync.Mutex

type notificationObservation struct {
    FileState string `json:"file_state"`
    UpdatedAt string `json:"updated_at,omitempty"`
    ConditionKey string `json:"condition_key,omitempty"`
    StartedAt string `json:"started_at,omitempty"`
}

type notificationJournal struct {
    Version int `json:"version"`
    Items map[string]notificationObservation `json:"items"`
    Events []map[string]any `json:"events"`
}

func notificationEventID(taskID,kind,at string) string {
    sum:=sha256.Sum256([]byte(taskID+"\x00"+kind+"\x00"+at))
    return hex.EncodeToString(sum[:12])
}

func notificationEventExists(events []map[string]any,id string) bool {
    for _,event:=range events {
        if toString(event["id"])==id { return true }
    }
    return false
}

func ReadNotificationEvents(project string) ([]map[string]any,error) {
    notificationJournalMu.Lock()
    defer notificationJournalMu.Unlock()
    path:=filepath.Join(project,"_task_mecca",".runtime","notification_events.json")
    data,err:=os.ReadFile(path)
    if os.IsNotExist(err) { return []map[string]any{},nil }
    if err!=nil { return nil,err }
    journal:=notificationJournal{}
    if err:=json.Unmarshal(data,&journal); err!=nil { return nil,err }
    if journal.Events==nil { return []map[string]any{},nil }
    return append([]map[string]any{},journal.Events...),nil
}

func NotificationEvents(project string,items map[string]map[string]any) ([]map[string]any,error) {
    notificationJournalMu.Lock()
    defer notificationJournalMu.Unlock()
    path:=filepath.Join(project,"_task_mecca",".runtime","notification_events.json")
    journal:=notificationJournal{
        Version:1,
        Items:map[string]notificationObservation{},
        Events:[]map[string]any{},
    }
    if data,err:=os.ReadFile(path); err==nil {
        _=json.Unmarshal(data,&journal)
        if journal.Items==nil { journal.Items=map[string]notificationObservation{} }
        if journal.Events==nil { journal.Events=[]map[string]any{} }
        journal.Version=1
    }

    now:=time.Now().Format(time.RFC3339Nano)
    dirty:=false
    for id,item:=range items {
        fileState:=toString(item["file_state"])
        updatedAt:=toString(item["updated_at"])
        previous,seen:=journal.Items[id]

        // First observation establishes a baseline so existing backlogs are
        // never backfilled as "registered". Only genuinely new items observed
        // after the journal already exists produce a registration event.
        if !seen && len(journal.Items)>0 {
            at:=updatedAt
            if at=="" { at=now }
            eventID:=notificationEventID(id,"registered",at)
            if !notificationEventExists(journal.Events,eventID) {
                journal.Events=append(journal.Events,map[string]any{
                    "id":eventID,"task_id":id,"kind":"registered","at":at,
                    "title":toString(item["title"]),"task_updated_at":updatedAt,
                })
                dirty=true
            }
        }

        if seen && fileState=="done" && previous.FileState!="done" {
            at:=""
            if lifecycle,ok:=item["lifecycle"].(map[string]any); ok { at=toString(lifecycle["completed_at"]) }
            if at=="" { at=toString(item["completed_at"]) }
            if at=="" { at=updatedAt }
            if at=="" { at=now }
            eventID:=notificationEventID(id,"completed",at)
            if !notificationEventExists(journal.Events,eventID) {
                journal.Events=append(journal.Events,map[string]any{
                    "id":eventID,
                    "task_id":id,
                    "kind":"completed",
                    "at":at,
                    "title":toString(item["title"]),
                    "task_updated_at":updatedAt,
                })
                dirty=true
            }
        }

        // Notification is a consumer of the canonical lifecycle. It must not
        // independently infer "started" from backlog state or runtime health.
        startedAt:=""
        if lifecycle,ok:=item["lifecycle"].(map[string]any); ok {
            startedAt=toString(lifecycle["started_at"])
        }
        if startedAt!="" && (!seen || previous.StartedAt!=startedAt) {
            eventID:=notificationEventID(id,"started",startedAt)
            if !notificationEventExists(journal.Events,eventID) {
                journal.Events=append(journal.Events,map[string]any{
                    "id":eventID,"task_id":id,"kind":"started","at":startedAt,
                    "title":toString(item["title"]),"message":"작업이 착수 상태로 전환되었습니다.",
                    "task_updated_at":updatedAt,
                })
                dirty=true
            }
        }

        conditionKey:=""
        if condition,ok:=item["notification_condition"].(map[string]any); ok && len(condition)>0 {
            kind:=toString(condition["kind"])
            conditionKey=toString(condition["key"])
            if conditionKey=="" { conditionKey=kind+"\x00"+toString(condition["reason_type"]) }
            if kind!="" && (!seen || previous.ConditionKey!=conditionKey) {
                at:=now
                eventID:=notificationEventID(id,kind,at)
                if !notificationEventExists(journal.Events,eventID) {
                    journal.Events=append(journal.Events,map[string]any{
                        "id":eventID,"task_id":id,"kind":kind,"at":at,
                        "title":toString(item["title"]),
                        "reason_type":toString(condition["reason_type"]),
                        "message":toString(condition["message"]),
                        "resume_condition":toString(condition["resume_condition"]),
                        "attempt_id":toString(condition["attempt_id"]),
                        "runtime_state":toString(condition["runtime_state"]),
                        "task_updated_at":updatedAt,
                    })
                    dirty=true
                }
            }
        }

        next:=notificationObservation{FileState:fileState,UpdatedAt:updatedAt,ConditionKey:conditionKey,StartedAt:startedAt}
        if !seen || previous!=next {
            journal.Items[id]=next
            dirty=true
        }
    }

    if len(journal.Events)>250 {
        journal.Events=append([]map[string]any{},journal.Events[len(journal.Events)-250:]...)
        dirty=true
    }

    if !dirty {
        return append([]map[string]any{},journal.Events...),nil
    }
    if err:=os.MkdirAll(filepath.Dir(path),0755); err!=nil { return nil,err }
    data,err:=json.MarshalIndent(journal,"","  ")
    if err!=nil { return nil,err }
    tmp:=path+".tmp"
    if err:=os.WriteFile(tmp,append(data,'\n'),0644); err!=nil { return nil,err }
    if err:=os.Rename(tmp,path); err!=nil { return nil,err }

    return append([]map[string]any{},journal.Events...),nil
}
