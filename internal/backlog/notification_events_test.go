package backlog

import (
    "testing"
)

func TestNotificationEventsPersistCompletionTransition(t *testing.T) {
    project:=t.TempDir()

    doing:=map[string]map[string]any{
        "A-23":{
            "id":"A-23",
            "title":"Task",
            "file_state":"doing",
            "updated_at":"2026-09-30T10:00:00Z",
        },
    }
    events,err:=NotificationEvents(project,doing)
    if err!=nil { t.Fatal(err) }
    if len(events)!=0 { t.Fatalf("baseline events=%+v",events) }

    done:=map[string]map[string]any{
        "A-23":{
            "id":"A-23",
            "title":"Task",
            "file_state":"done",
            "updated_at":"2026-09-30T10:05:00Z",
            "completed_at":"2026-09-30T10:05:00Z",
        },
    }
    events,err=NotificationEvents(project,done)
    if err!=nil { t.Fatal(err) }
    if len(events)!=1 { t.Fatalf("events=%+v",events) }
    if events[0]["task_id"]!="A-23" || events[0]["kind"]!="completed" {
        t.Fatalf("event=%+v",events[0])
    }

    again,err:=NotificationEvents(project,done)
    if err!=nil { t.Fatal(err) }
    if len(again)!=1 { t.Fatalf("duplicate events=%+v",again) }
    if again[0]["id"]!=events[0]["id"] { t.Fatalf("event id changed: %v != %v",again[0]["id"],events[0]["id"]) }
}

func TestNotificationEventsDoNotNotifyHistoricalDoneOnFirstObservation(t *testing.T) {
    project:=t.TempDir()
    done:=map[string]map[string]any{
        "A-22":{
            "id":"A-22",
            "title":"Already done",
            "file_state":"done",
            "updated_at":"2026-09-29T10:00:00Z",
            "completed_at":"2026-09-29T10:00:00Z",
        },
    }
    events,err:=NotificationEvents(project,done)
    if err!=nil { t.Fatal(err) }
    if len(events)!=0 { t.Fatalf("historical completion should establish baseline: %+v",events) }
}

func TestNotificationEventsPersistAttentionEpisode(t *testing.T) {
    project:=t.TempDir()
    items:=map[string]map[string]any{
        "AID-39":{"file_state":"doing","updated_at":"2026-10-03T00:00:00Z","title":"Runtime sensing","attention_reason":map[string]any{"type":"runtime_stalled","message":"no activity","resume_condition":"check worker"}},
    }
    events,err:=NotificationEvents(project,items); if err!=nil { t.Fatal(err) }
    if len(events)!=1 { t.Fatalf("events=%v",events) }
    if events[0]["kind"]!="stalled" || events[0]["task_id"]!="AID-39" { t.Fatalf("event=%v",events[0]) }
    again,err:=NotificationEvents(project,items); if err!=nil { t.Fatal(err) }
    if len(again)!=1 { t.Fatalf("duplicate attention event: %v",again) }

    items["AID-39"]["attention_reason"]=nil
    if _,err=NotificationEvents(project,items); err!=nil { t.Fatal(err) }
    items["AID-39"]["attention_reason"]=map[string]any{"type":"runtime_stalled","message":"no activity","resume_condition":"check worker"}
    resumed,err:=NotificationEvents(project,items); if err!=nil { t.Fatal(err) }
    if len(resumed)!=2 { t.Fatalf("reappearing attention should create a new episode, got %v",resumed) }
}

func TestNotificationEventsPersistExecutionLifecycle(t *testing.T) {
 project:=t.TempDir()
 items:=map[string]map[string]any{
  "AID-39":{"file_state":"doing","updated_at":"2026-10-03T03:00:00Z","title":"Runtime sensing",
   "activity":map[string]any{"source":"execution_ledger","attempt_id":"run-1","runtime_state":"running","health":"active"}},
 }
 events,err:=NotificationEvents(project,items);if err!=nil{t.Fatal(err)}
 if len(events)!=1||events[0]["kind"]!="started"{t.Fatalf("start events=%v",events)}
 items["AID-39"]["activity"]=map[string]any{"source":"execution_ledger","attempt_id":"run-1","runtime_state":"completed","health":"awaiting_finalize"}
 events,err=NotificationEvents(project,items);if err!=nil{t.Fatal(err)}
 if len(events)!=2||events[1]["kind"]!="finalize"{t.Fatalf("finalize events=%v",events)}
 items["AID-39"]["file_state"]="done";items["AID-39"]["completed_at"]="2026-10-03T03:10:00Z"
 events,err=NotificationEvents(project,items);if err!=nil{t.Fatal(err)}
 if len(events)!=3||events[2]["kind"]!="completed"{t.Fatalf("completed events=%v",events)}
}
