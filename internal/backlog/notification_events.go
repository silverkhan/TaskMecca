package backlog

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "os"
    "path/filepath"
    "time"
)

type notificationObservation struct {
    FileState string `json:"file_state"`
    UpdatedAt string `json:"updated_at,omitempty"`
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

    now:=time.Now().Format(time.RFC3339)
    dirty:=false
    for id,item:=range items {
        fileState:=toString(item["file_state"])
        updatedAt:=toString(item["updated_at"])
        previous,seen:=journal.Items[id]

        if seen && fileState=="done" && previous.FileState!="done" {
            at:=toString(item["completed_at"])
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

        attentionKey:=""
        if reason,ok:=item["attention_reason"].(map[string]any); ok && len(reason)>0 {
            reasonType:=toString(reason["type"])
            kind:="intervention"
            if reasonType=="runtime_stalled" { kind="stalled" }
            attentionKey=kind+"\x00"+reasonType+"\x00"+toString(reason["message"])+"\x00"+toString(reason["resume_condition"])
            if !seen || previous.AttentionKey!=attentionKey {
                at:=now
                eventID:=notificationEventID(id,kind,attentionKey)
                if !notificationEventExists(journal.Events,eventID) {
                    journal.Events=append(journal.Events,map[string]any{
                        "id":eventID,"task_id":id,"kind":kind,"at":at,
                        "title":toString(item["title"]),"reason_type":reasonType,
                        "message":toString(reason["message"]),"resume_condition":toString(reason["resume_condition"]),
                        "task_updated_at":updatedAt,
                    })
                    dirty=true
                }
            }
        }

        next:=notificationObservation{FileState:fileState,UpdatedAt:updatedAt,AttentionKey:attentionKey}
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
