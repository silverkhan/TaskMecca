package backlog

import (
    "encoding/json"
    "os"
    "path/filepath"
    "sync"
    "testing"
    "time"
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


func TestRuntimeUnknownNotificationRequiresStableDwell(t *testing.T) {
    project:=t.TempDir()
    base:=map[string]map[string]any{
        "B-610":{"file_state":"doing","updated_at":"2026-10-06T00:00:00Z","title":"Transient hook gap"},
    }
    if events,err:=NotificationEvents(project,base);err!=nil||len(events)!=0{
        t.Fatalf("baseline events=%+v err=%v",events,err)
    }

    unknown:=map[string]map[string]any{
        "B-610":{
            "file_state":"doing","updated_at":"2026-10-06T00:00:01Z","title":"Transient hook gap",
            "notification_condition":map[string]any{
                "kind":"runtime_unknown","key":"runtime:unknown\x00run-610",
                "reason_type":"runtime_unknown","message":"temporarily unknown",
                "attempt_id":"run-610","runtime_state":"runtime_unknown",
            },
        },
    }
    events,err:=NotificationEvents(project,unknown)
    if err!=nil{t.Fatal(err)}
    if len(events)!=0{t.Fatalf("runtime_unknown must not alert immediately: %+v",events)}

    path:=filepath.Join(project,"_task_mecca",".runtime","notification_events.json")
    data,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
    var journal notificationJournal
    if err:=json.Unmarshal(data,&journal);err!=nil{t.Fatal(err)}
    obs:=journal.Items["B-610"]
    // One minute is enough for a previously-started runtime loss, but a task
    // that has not established canonical start yet gets a longer startup grace.
    obs.ConditionSince=time.Now().Add(-runtimeUnknownNotificationGrace-time.Second).Format(time.RFC3339Nano)
    obs.ConditionConsumed=false
    journal.Items["B-610"]=obs
    encoded,err:=json.MarshalIndent(journal,"","  ");if err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(path,append(encoded,'\n'),0644);err!=nil{t.Fatal(err)}

    events,err=NotificationEvents(project,unknown)
    if err!=nil{t.Fatal(err)}
    for _,event:=range events{
        if toString(event["task_id"])=="B-610" && toString(event["kind"])=="runtime_unknown"{
            t.Fatalf("startup runtime_unknown must not alert after only the active grace: %+v",events)
        }
    }

    data,err=os.ReadFile(path);if err!=nil{t.Fatal(err)}
    if err:=json.Unmarshal(data,&journal);err!=nil{t.Fatal(err)}
    obs=journal.Items["B-610"]
    obs.ConditionSince=time.Now().Add(-runtimeUnknownStartupNotificationGrace-time.Second).Format(time.RFC3339Nano)
    obs.ConditionConsumed=false
    journal.Items["B-610"]=obs
    encoded,err=json.MarshalIndent(journal,"","  ");if err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(path,append(encoded,'\n'),0644);err!=nil{t.Fatal(err)}

    events,err=NotificationEvents(project,unknown)
    if err!=nil{t.Fatal(err)}
    count:=0
    for _,event:=range events{
        if toString(event["task_id"])=="B-610" && toString(event["kind"])=="runtime_unknown"{count++}
    }
    if count!=1{t.Fatalf("persistent startup runtime_unknown must alert once after startup dwell: %+v",events)}

    again,err:=NotificationEvents(project,unknown);if err!=nil{t.Fatal(err)}
    if len(again)!=len(events){t.Fatalf("runtime_unknown duplicated after consumption: %+v",again)}
}

func TestNotificationCompletedPhaseBlocksLateReplayAfterObservationLoss(t *testing.T) {
    project:=t.TempDir()
    base:=map[string]map[string]any{
        "B-920":{"file_state":"todo","updated_at":"2026-10-06T00:00:00Z","title":"Monotonic"},
    }
    if events,err:=NotificationEvents(project,base);err!=nil||len(events)!=0{
        t.Fatalf("baseline events=%+v err=%v",events,err)
    }

    running:=map[string]map[string]any{
        "B-920":{
            "file_state":"doing","updated_at":"2026-10-06T00:01:00Z","title":"Monotonic",
            "lifecycle":map[string]any{"started_at":"2026-10-06T00:01:00Z"},
        },
    }
    events,err:=NotificationEvents(project,running);if err!=nil{t.Fatal(err)}
    if len(events)!=1||toString(events[0]["kind"])!="started"{t.Fatalf("running events=%+v",events)}

    running["B-920"]["notification_condition"]=map[string]any{
        "kind":"finalize","key":"runtime:finalize\x00run-920",
        "reason_type":"completion_pending","message":"worker done",
    }
    events,err=NotificationEvents(project,running);if err!=nil{t.Fatal(err)}
    if len(events)!=2||toString(events[1]["kind"])!="finalize"{t.Fatalf("finalize events=%+v",events)}

    done:=map[string]map[string]any{
        "B-920":{
            "file_state":"done","updated_at":"2026-10-06T00:03:00Z","title":"Monotonic",
            "lifecycle":map[string]any{
                "started_at":"2026-10-06T00:01:00Z",
                "completed_at":"2026-10-06T00:03:00Z",
            },
        },
    }
    events,err=NotificationEvents(project,done);if err!=nil{t.Fatal(err)}
    if len(events)!=3||toString(events[2]["kind"])!="completed"{t.Fatalf("completed events=%+v",events)}

    // Simulate a partial journal/snapshot loss: the task observation disappears,
    // but durable phase/event history remains. Reappearance must not look new.
    path:=filepath.Join(project,"_task_mecca",".runtime","notification_events.json")
    data,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
    var journal notificationJournal
    if err:=json.Unmarshal(data,&journal);err!=nil{t.Fatal(err)}
    delete(journal.Items,"B-920")
    encoded,err:=json.MarshalIndent(journal,"","  ");if err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(path,append(encoded,'\n'),0644);err!=nil{t.Fatal(err)}

    replay:=map[string]map[string]any{
        "B-920":{
            "file_state":"done","updated_at":"2026-10-06T00:04:00Z","title":"Monotonic",
            "lifecycle":map[string]any{
                "created_at":"2026-10-06T00:00:00Z",
                "started_at":"2026-10-06T00:01:00Z",
                "completed_at":"2026-10-06T00:03:00Z",
            },
            "notification_condition":map[string]any{
                "kind":"runtime_unknown","key":"runtime:unknown\x00run-920",
                "reason_type":"runtime_unknown","message":"late stale signal",
            },
        },
    }
    again,err:=NotificationEvents(project,replay);if err!=nil{t.Fatal(err)}
    if len(again)!=len(events){t.Fatalf("terminal task replayed notifications: before=%+v after=%+v",events,again)}
    for _,event:=range again{
        if toString(event["task_id"])=="B-920" && (toString(event["kind"])=="registered"||toString(event["kind"])=="runtime_unknown"){
            t.Fatalf("terminal lifecycle regressed: %+v",event)
        }
    }
}

func TestNotificationFinalizeBlocksLateStartedRegression(t *testing.T) {
    project:=t.TempDir()
    base:=map[string]map[string]any{
        "B-921":{"file_state":"doing","updated_at":"2026-10-06T01:00:00Z","title":"Finalize first"},
    }
    if events,err:=NotificationEvents(project,base);err!=nil||len(events)!=0{
        t.Fatalf("baseline events=%+v err=%v",events,err)
    }

    base["B-921"]["notification_condition"]=map[string]any{
        "kind":"finalize","key":"runtime:finalize\x00run-921",
        "reason_type":"completion_pending","message":"worker done",
    }
    events,err:=NotificationEvents(project,base);if err!=nil{t.Fatal(err)}
    if len(events)!=1||toString(events[0]["kind"])!="finalize"{t.Fatalf("finalize events=%+v",events)}

    // If lifecycle start evidence is discovered late, it belongs in lifecycle
    // history but must not become a push notification after finalize.
    base["B-921"]["lifecycle"]=map[string]any{"started_at":"2026-10-06T00:59:00Z"}
    events,err=NotificationEvents(project,base);if err!=nil{t.Fatal(err)}
    if len(events)!=1{t.Fatalf("late started regressed notification stream: %+v",events)}
}

func TestControlTowerHoldUserProducesCanonicalIntervention(t *testing.T) {
    row:=Record{ID:"B-601",State:"hold",Location:"active"}
    reason,condition:=canonicalOperationalState(row,map[string]any{"wait_kind":"user","wait_note":"need answer","resume_condition":"reply"},nil)
    if toString(reason["type"])!="user_intervention" || toString(condition["kind"])!="intervention" {
        t.Fatalf("reason=%+v condition=%+v",reason,condition)
    }
}


func TestNotificationEventsLegacyJournalMigrationDoesNotBackfillCurrentState(t *testing.T) {
    project:=t.TempDir()
    path:=filepath.Join(project,"_task_mecca",".runtime","notification_events.json")
    if err:=os.MkdirAll(filepath.Dir(path),0755);err!=nil{t.Fatal(err)}
    legacy:=`{
  "version": 1,
  "items": {
    "B-800": {
      "file_state": "doing",
      "updated_at": "2026-10-05T11:20:00Z",
      "attention_key": "legacy-attention",
      "runtime_key": "legacy-runtime",
      "started_at": "2026-10-05T11:00:00Z"
    }
  },
  "events": []
}`
    if err:=os.WriteFile(path,[]byte(legacy),0644);err!=nil{t.Fatal(err)}

    items:=map[string]map[string]any{
        "B-800":{
            "file_state":"doing","updated_at":"2026-10-05T11:20:00Z","title":"Existing",
            "lifecycle":map[string]any{"started_at":"2026-10-05T11:00:00Z"},
            "notification_condition":map[string]any{
                "kind":"stalled","key":"runtime:stalled:run-old",
                "reason_type":"runtime_stalled","message":"existing condition",
            },
        },
    }
    events,err:=NotificationEvents(project,items)
    if err!=nil{t.Fatal(err)}
    if len(events)!=0{t.Fatalf("schema migration must baseline current state, got %+v",events)}

    data,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
    var journal notificationJournal
    if err:=json.Unmarshal(data,&journal);err!=nil{t.Fatal(err)}
    if journal.Version!=notificationJournalVersion{t.Fatalf("version=%d",journal.Version)}
    if journal.Items["B-800"].ConditionKey!="runtime:stalled:run-old"{
        t.Fatalf("condition key not migrated into baseline: %+v",journal.Items["B-800"])
    }
}

func TestNotificationEventsColdStartBaselinesHistoricalStartedAndCondition(t *testing.T) {
    project:=t.TempDir()
    items:=map[string]map[string]any{
        "B-801":{
            "file_state":"doing","updated_at":"2026-10-04T08:00:00Z","title":"Historical",
            "lifecycle":map[string]any{"started_at":"2026-10-04T07:30:00Z"},
            "notification_condition":map[string]any{
                "kind":"finalize","key":"runtime:finalize:run-old",
                "reason_type":"completion_pending","message":"historical",
            },
        },
    }
    events,err:=NotificationEvents(project,items)
    if err!=nil{t.Fatal(err)}
    if len(events)!=0{t.Fatalf("cold start must establish baseline without backfill: %+v",events)}
}


func TestNotificationEventsRecoveredBindingDoesNotEmitStarted(t *testing.T) {
    project:=t.TempDir()
    base:=map[string]map[string]any{
        "B-802":{"file_state":"doing","updated_at":"2026-10-05T12:00:00Z","title":"Recovered"},
    }
    if events,err:=NotificationEvents(project,base);err!=nil||len(events)!=0{
        t.Fatalf("baseline events=%+v err=%v",events,err)
    }

    recovered:=map[string]map[string]any{
        "B-802":{
            "file_state":"doing","updated_at":"2026-10-05T12:00:00Z","title":"Recovered",
            "lifecycle":map[string]any{
                "started_at":"2026-10-05T12:05:00Z",
                "started_notification_suppressed":true,
            },
        },
    }
    events,err:=NotificationEvents(project,recovered)
    if err!=nil{t.Fatal(err)}
    if len(events)!=0{t.Fatalf("recovery binding must not emit started: %+v",events)}

    recovered["B-802"]["lifecycle"]=map[string]any{
        "started_at":"2026-10-05T12:05:00Z",
        "started_notification_suppressed":false,
    }
    again,err:=NotificationEvents(project,recovered)
    if err!=nil{t.Fatal(err)}
    if len(again)!=0{t.Fatalf("same recovered start must remain consumed: %+v",again)}
}

func TestNotificationEventsDeletedJournalRebaselinesInsteadOfReplaying(t *testing.T) {
    project:=t.TempDir()
    base:=map[string]map[string]any{
        "B-803":{"file_state":"todo","updated_at":"2026-10-05T12:00:00Z","title":"Rebaseline"},
    }
    if _,err:=NotificationEvents(project,base);err!=nil{t.Fatal(err)}
    running:=map[string]map[string]any{
        "B-803":{
            "file_state":"doing","updated_at":"2026-10-05T12:01:00Z","title":"Rebaseline",
            "lifecycle":map[string]any{"started_at":"2026-10-05T12:01:00Z"},
        },
    }
    events,err:=NotificationEvents(project,running);if err!=nil{t.Fatal(err)}
    if len(events)!=1||events[0]["kind"]!="started"{t.Fatalf("expected live start before deletion: %+v",events)}

    journalPath:=filepath.Join(project,"_task_mecca",".runtime","notification_events.json")
    if err:=os.Remove(journalPath);err!=nil{t.Fatal(err)}
    events,err=NotificationEvents(project,running);if err!=nil{t.Fatal(err)}
    if len(events)!=0{t.Fatalf("journal recovery must baseline current state, got %+v",events)}
}


func TestNotificationEventsNewTaskAlreadyCompletedEmitsAllCanonicalStages(t *testing.T) {
    project:=t.TempDir()
    base:=map[string]map[string]any{
        "B-001":{"file_state":"todo","updated_at":"2026-10-05T01:00:00Z","title":"Baseline"},
    }
    if events,err:=NotificationEvents(project,base);err!=nil||len(events)!=0{
        t.Fatalf("baseline events=%+v err=%v",events,err)
    }

    items:=map[string]map[string]any{
        "B-001":{"file_state":"todo","updated_at":"2026-10-05T01:00:00Z","title":"Baseline"},
        "B-002":{
            "file_state":"done","updated_at":"2026-10-05T01:03:00Z","title":"Fast task",
            "lifecycle":map[string]any{
                "created_at":"2026-10-05T01:00:30Z",
                "started_at":"2026-10-05T01:01:00Z",
                "completed_at":"2026-10-05T01:02:00Z",
            },
        },
    }
    events,err:=NotificationEvents(project,items)
    if err!=nil{t.Fatal(err)}
    got:=[]map[string]any{}
    for _,event:=range events{
        if toString(event["task_id"])=="B-002"{got=append(got,event)}
    }
    if len(got)!=3{t.Fatalf("fast lifecycle events=%+v",got)}
    want:=[]string{"registered","started","completed"}
    for i,kind:=range want{
        if toString(got[i]["kind"])!=kind{t.Fatalf("event[%d]=%+v want kind=%s",i,got[i],kind)}
    }
    if toString(got[0]["at"])!="2026-10-05T01:00:30Z"{
        t.Fatalf("registration must use canonical created_at: %+v",got[0])
    }

    again,err:=NotificationEvents(project,items)
    if err!=nil{t.Fatal(err)}
    count:=0
    for _,event:=range again{if toString(event["task_id"])=="B-002"{count++}}
    if count!=3{t.Fatalf("fast lifecycle duplicated or lost: %+v",again)}
}

func TestNotificationEventsNewTaskCurrentConditionIsNotLost(t *testing.T) {
    project:=t.TempDir()
    if _,err:=NotificationEvents(project,map[string]map[string]any{
        "B-001":{"file_state":"todo","updated_at":"2026-10-05T02:00:00Z","title":"Baseline"},
    });err!=nil{t.Fatal(err)}

    items:=map[string]map[string]any{
        "B-001":{"file_state":"todo","updated_at":"2026-10-05T02:00:00Z","title":"Baseline"},
        "B-003":{
            "file_state":"doing","updated_at":"2026-10-05T02:02:00Z","title":"Approval fast path",
            "lifecycle":map[string]any{
                "created_at":"2026-10-05T02:00:30Z",
                "started_at":"2026-10-05T02:01:00Z",
            },
            "notification_condition":map[string]any{
                "kind":"approval","key":"runtime:approval:run-3","reason_type":"approval_required",
                "message":"approval needed","attempt_id":"run-3","runtime_state":"waiting_approval",
            },
        },
    }
    events,err:=NotificationEvents(project,items)
    if err!=nil{t.Fatal(err)}
    kinds:=[]string{}
    for _,event:=range events{
        if toString(event["task_id"])=="B-003"{kinds=append(kinds,toString(event["kind"]))}
    }
    if len(kinds)!=3 || kinds[0]!="registered" || kinds[1]!="started" || kinds[2]!="approval"{
        t.Fatalf("new task current condition was lost or reordered: %v events=%+v",kinds,events)
    }
}


func TestNotificationEventsEmptyBaselinePersistsFirstRegistrationBoundary(t *testing.T) {
    project:=t.TempDir()
    events,err:=NotificationEvents(project,map[string]map[string]any{})
    if err!=nil{t.Fatal(err)}
    if len(events)!=0{t.Fatalf("empty baseline events=%+v",events)}

    first:=map[string]map[string]any{
        "B-004":{
            "file_state":"todo","updated_at":"2026-10-05T03:00:00Z","title":"First real task",
            "lifecycle":map[string]any{"created_at":"2026-10-05T03:00:00Z"},
        },
    }
    events,err=NotificationEvents(project,first)
    if err!=nil{t.Fatal(err)}
    if len(events)!=1||toString(events[0]["kind"])!="registered"||toString(events[0]["task_id"])!="B-004"{
        t.Fatalf("first registration after observed empty baseline was lost: %+v",events)
    }
}
