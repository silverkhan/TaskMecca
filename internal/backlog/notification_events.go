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
    AttentionKey string `json:"attention_key,omitempty"`
    RuntimeKey string `json:"runtime_key,omitempty"`
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

    now:=time.Now().Format(time.RFC3339Nano)
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

        runtimeKey:=""
        if activity,ok:=item["activity"].(map[string]any); ok && toString(activity["source"])=="execution_ledger" {
            attemptID:=toString(activity["attempt_id"])
            runtimeState:=toString(activity["runtime_state"])
            health:=toString(activity["health"])
            runtimeKey=attemptID+"\x00"+runtimeState+"\x00"+health
            if attemptID!="" && (!seen || previous.RuntimeKey!=runtimeKey) {
                kind:=""
                message:=""
                switch {
                case health=="active" && (runtimeState=="starting" || runtimeState=="running"):
                    kind="started"; message="Worker가 실제 실행을 시작했습니다."
                case runtimeState=="waiting_approval":
                    kind="approval"; message="Worker가 승인을 기다리고 있습니다."
                case health=="needs_user":
                    kind="intervention"; message="Worker가 사용자 입력을 기다리고 있습니다."
                case health=="stale":
                    kind="stalled"; message="실행 활동이 일정 시간 관측되지 않았습니다."
                case health=="execution_interrupted":
                    kind="interrupted"; message="실행이 정상 완료되지 않고 종료되었습니다."
                case health=="runtime_unknown":
                    kind="runtime_unknown"; message="현재 실행 상태를 신뢰성 있게 확인할 수 없습니다."
                case health=="awaiting_finalize":
                    kind="finalize"; message="Worker 실행은 끝났지만 백로그가 아직 doing 상태입니다."
                }
                if kind!="" {
                    at:=now
                    eventID:=notificationEventID(id,kind,at)
                    if !notificationEventExists(journal.Events,eventID) {
                        journal.Events=append(journal.Events,map[string]any{
                            "id":eventID,"task_id":id,"kind":kind,"at":at,"title":toString(item["title"]),
                            "message":message,"attempt_id":attemptID,"runtime_state":runtimeState,
                            "task_updated_at":updatedAt,
                        })
                        dirty=true
                    }
                }
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
                eventID:=notificationEventID(id,kind,at)
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

        next:=notificationObservation{FileState:fileState,UpdatedAt:updatedAt,AttentionKey:attentionKey,RuntimeKey:runtimeKey}
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
