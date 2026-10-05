package backlog

import (
    "encoding/json"
    "os"
    "path/filepath"
    "sync"
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
        "AID-39":{"file_state":"doing","updated_at":"2026-10-03T00:00:00Z","title":"Runtime sensing"},
    }
    events,err:=NotificationEvents(project,items); if err!=nil { t.Fatal(err) }
    if len(events)!=0 { t.Fatalf("baseline events=%v",events) }

    items["AID-39"]["notification_condition"]=map[string]any{"kind":"stalled","key":"runtime:stalled","reason_type":"runtime_stalled","message":"no activity","resume_condition":"check worker"}
    events,err=NotificationEvents(project,items); if err!=nil { t.Fatal(err) }
    if len(events)!=1 { t.Fatalf("events=%v",events) }
    if events[0]["kind"]!="stalled" || events[0]["task_id"]!="AID-39" { t.Fatalf("event=%v",events[0]) }
    again,err:=NotificationEvents(project,items); if err!=nil { t.Fatal(err) }
    if len(again)!=1 { t.Fatalf("duplicate attention event: %v",again) }

    items["AID-39"]["notification_condition"]=nil
    if _,err=NotificationEvents(project,items); err!=nil { t.Fatal(err) }
    items["AID-39"]["notification_condition"]=map[string]any{"kind":"stalled","key":"runtime:stalled","reason_type":"runtime_stalled","message":"no activity","resume_condition":"check worker"}
    resumed,err:=NotificationEvents(project,items); if err!=nil { t.Fatal(err) }
    if len(resumed)!=2 { t.Fatalf("reappearing attention should create a new episode, got %v",resumed) }
}

func TestNotificationEventsConsumeCanonicalLifecycle(t *testing.T) {
 project:=t.TempDir()
 items:=map[string]map[string]any{
  "AID-39":{"file_state":"doing","updated_at":"2026-10-03T03:00:00Z","title":"Runtime sensing",
   "activity":map[string]any{"source":"execution_ledger","attempt_id":"run-1","runtime_state":"running","health":"active"}},
 }
 // Runtime health alone must not independently manufacture a start event.
 events,err:=NotificationEvents(project,items);if err!=nil{t.Fatal(err)}
 if len(events)!=0{t.Fatalf("runtime-only start must wait for canonical lifecycle: %v",events)}

 items["AID-39"]["lifecycle"]=map[string]any{"started_at":"2026-10-03T03:00:01Z"}
 events,err=NotificationEvents(project,items);if err!=nil{t.Fatal(err)}
 if len(events)!=1||events[0]["kind"]!="started"{t.Fatalf("canonical start events=%v",events)}

 // Refreshing the same canonical event must not duplicate the notification.
 events,err=NotificationEvents(project,items);if err!=nil{t.Fatal(err)}
 if len(events)!=1{t.Fatalf("duplicate canonical start=%v",events)}

 items["AID-39"]["notification_condition"]=map[string]any{"kind":"finalize","key":"runtime:finalize\\x00run-1","reason_type":"completion_pending","message":"done"}
 events,err=NotificationEvents(project,items);if err!=nil{t.Fatal(err)}
 if len(events)!=2||events[1]["kind"]!="finalize"{t.Fatalf("finalize events=%v",events)}
 items["AID-39"]["file_state"]="done"
 items["AID-39"]["lifecycle"]=map[string]any{"started_at":"2026-10-03T03:00:01Z","completed_at":"2026-10-03T03:10:00Z"}
 events,err=NotificationEvents(project,items);if err!=nil{t.Fatal(err)}
 if len(events)!=3||events[2]["kind"]!="completed"{t.Fatalf("completed events=%v",events)}
}

func TestNotificationEventsRegistrationBaselinesExistingItems(t *testing.T) {
 project:=t.TempDir()
 initial:=map[string]map[string]any{
  "B-1":{"file_state":"backlog","updated_at":"2026-10-03T01:00:00Z","title":"existing"},
 }
 events,err:=NotificationEvents(project,initial);if err!=nil{t.Fatal(err)}
 if len(events)!=0 { t.Fatalf("initial baseline must not backfill registration: %v",events) }
 initial["B-2"]=map[string]any{"file_state":"backlog","updated_at":"2026-10-03T02:00:00Z","title":"new"}
 events,err=NotificationEvents(project,initial);if err!=nil{t.Fatal(err)}
 if len(events)!=1||events[0]["kind"]!="registered"||events[0]["task_id"]!="B-2"{t.Fatalf("registration events=%v",events)}
}


func TestCanonicalLifecycleEmitsStartedThenCompletedOnce(t *testing.T) {
    project:=t.TempDir()
    base:=map[string]map[string]any{"B-442":{"file_state":"backlog","updated_at":"2026-10-05T01:31:00Z","title":"Task"}}
    if _,err:=NotificationEvents(project,base); err!=nil { t.Fatal(err) }
    running:=map[string]map[string]any{"B-442":{"file_state":"doing","updated_at":"2026-10-05T01:32:00Z","title":"Task","lifecycle":map[string]any{"started_at":"2026-10-05T01:32:00Z"}}}
    events,err:=NotificationEvents(project,running); if err!=nil { t.Fatal(err) }
    if len(events)!=1 || events[0]["kind"]!="started" { t.Fatalf("started event missing: %+v",events) }
    done:=map[string]map[string]any{"B-442":{"file_state":"done","updated_at":"2026-10-05T01:35:00Z","title":"Task","completed_at":"2026-10-05T01:35:00Z","lifecycle":map[string]any{"started_at":"2026-10-05T01:32:00Z","completed_at":"2026-10-05T01:35:00Z"}}}
    events,err=NotificationEvents(project,done); if err!=nil { t.Fatal(err) }
    if len(events)!=2 || events[0]["kind"]!="started" || events[1]["kind"]!="completed" { t.Fatalf("lifecycle notification sequence=%+v",events) }
    again,err:=NotificationEvents(project,done); if err!=nil { t.Fatal(err) }
    if len(again)!=2 { t.Fatalf("lifecycle notifications duplicated: %+v",again) }
}


func TestNotificationEventsConcurrentReconcileKeepsStartedEventAndValidJournal(t *testing.T) {
    project:=t.TempDir()
    base:=map[string]map[string]any{
        "B-500":{"file_state":"todo","updated_at":"2026-10-05T10:00:00Z","title":"Concurrent"},
    }
    if _,err:=NotificationEvents(project,base); err!=nil { t.Fatal(err) }

    running:=map[string]map[string]any{
        "B-500":{"file_state":"doing","updated_at":"2026-10-05T10:01:00Z","title":"Concurrent",
            "lifecycle":map[string]any{"started_at":"2026-10-05T10:01:00Z"}},
    }
    var wg sync.WaitGroup
    errs:=make(chan error,24)
    for i:=0;i<24;i++ {
        wg.Add(1)
        go func(){
            defer wg.Done()
            _,err:=NotificationEvents(project,running)
            errs<-err
        }()
    }
    wg.Wait()
    close(errs)
    for err:=range errs { if err!=nil { t.Fatalf("concurrent reconcile failed: %v",err) } }

    events,err:=ReadNotificationEvents(project)
    if err!=nil { t.Fatalf("journal unreadable after concurrent reconcile: %v",err) }
    starts:=0
    for _,event:=range events {
        if toString(event["task_id"])=="B-500" && toString(event["kind"])=="started" { starts++ }
    }
    if starts!=1 { t.Fatalf("started event count=%d events=%+v",starts,events) }
}


func TestControlTowerAllOperationalKindsJournalExactlyOnce(t *testing.T) {
    cases:=[]struct{health,state,kind string}{
        {"","waiting_approval","approval"},
        {"needs_user","waiting_user","intervention"},
        {"stale","running","stalled"},
        {"execution_interrupted","failed","interrupted"},
        {"runtime_unknown","runtime_unknown","runtime_unknown"},
        {"awaiting_finalize","completed","finalize"},
    }
    for _,tc:=range cases {
        t.Run(tc.kind,func(t *testing.T){
            project:=t.TempDir()
            base:=map[string]map[string]any{"B-600":{"file_state":"doing","updated_at":"2026-10-05T11:00:00Z","title":"Kinds"}}
            if _,err:=NotificationEvents(project,base);err!=nil{t.Fatal(err)}
            row:=Record{ID:"B-600",State:"doing",Location:"active"}
            _,condition:=canonicalOperationalState(row,nil,map[string]any{"source":"execution_ledger","attempt_id":"run-600","runtime_state":tc.state,"health":tc.health})
            item:=map[string]any{"file_state":"doing","updated_at":"2026-10-05T11:01:00Z","title":"Kinds","notification_condition":condition}
            events,err:=NotificationEvents(project,map[string]map[string]any{"B-600":item})
            if err!=nil{t.Fatal(err)}
            count:=0
            for _,event:=range events { if toString(event["kind"])==tc.kind { count++ } }
            if count!=1 { t.Fatalf("kind %s count=%d events=%+v",tc.kind,count,events) }
            again,err:=NotificationEvents(project,map[string]map[string]any{"B-600":item});if err!=nil{t.Fatal(err)}
            if len(again)!=len(events){t.Fatalf("kind %s duplicated: %+v",tc.kind,again)}
        })
    }
}

func TestControlTowerHoldUserProducesCanonicalIntervention(t *testing.T) {
    row:=Record{ID:"B-601",State:"hold",Location:"active"}
    reason,condition:=canonicalOperationalState(row,map[string]any{"wait_kind":"user","wait_note":"need answer","resume_condition":"reply"},nil)
    if toString(reason["type"])!="user_intervention" || toString(condition["kind"])!="intervention" {
        t.Fatalf("reason=%+v condition=%+v",reason,condition)
    }
}
