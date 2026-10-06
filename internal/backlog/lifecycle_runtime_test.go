package backlog

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestCompletedLifecyclePreservesObservedRegistrationAndStartWhenGitOnlyHasDone(t *testing.T) {
    project:=t.TempDir()
    gitRun(t,project,"init")
    gitRun(t,project,"config","user.email","ci@example.invalid")
    gitRun(t,project,"config","user.name","CI")

    backlogDir:=filepath.Join(project,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlogDir,0755);err!=nil{t.Fatal(err)}
    donePath:=filepath.Join(backlogDir,"0903.B-903.lifecycle.done.md")
    if err:=os.WriteFile(donePath,[]byte("# B-903 Lifecycle preservation\n"),0644);err!=nil{t.Fatal(err)}

    now:=time.Now().UTC()
    journalPath:=filepath.Join(project,"_task_mecca",".runtime","lifecycle_observations.json")
    if err:=os.MkdirAll(filepath.Dir(journalPath),0755);err!=nil{t.Fatal(err)}
    absLedger,err:=filepath.Abs(backlogDir);if err!=nil{t.Fatal(err)}
    journal:=map[string]any{
        "version":float64(1),
        "ledgers":map[string]any{
            absLedger:map[string]any{
                "items":map[string]any{
                    "B-903":[]any{
                        map[string]any{"state":"todo","at":now.Add(-2*time.Minute).Format(time.RFC3339)},
                        map[string]any{"state":"doing","at":now.Add(-time.Minute).Format(time.RFC3339)},
                    },
                },
            },
        },
    }
    data,err:=json.MarshalIndent(journal,"","  ");if err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(journalPath,append(data,'\n'),0644);err!=nil{t.Fatal(err)}

    gitRun(t,project,"add",".")
    gitRun(t,project,"commit","-m","complete task")

    rows,err:=Catalog(project,"");if err!=nil{t.Fatal(err)}
    timings,err:=lifecycleTimings(project,"",rows);if err!=nil{t.Fatal(err)}
    lifecycle:=timings["B-903"];if lifecycle==nil{t.Fatal("missing lifecycle")}
    events,ok:=lifecycle["events"].([]map[string]any);if !ok{t.Fatalf("events=%T %+v",lifecycle["events"],lifecycle["events"])}
    got:=[]string{}
    for _,event:=range events{got=append(got,toString(event["state"]))}
    want:=[]string{"todo","doing","done"}
    if len(got)!=len(want){t.Fatalf("lifecycle states=%v want=%v lifecycle=%+v",got,want,lifecycle)}
    for i:=range want{if got[i]!=want[i]{t.Fatalf("lifecycle states=%v want=%v",got,want)}}
    if incomplete,ok:=lifecycle["timing_incomplete"].(bool);!ok||incomplete{
        t.Fatalf("preserved observed start must keep completed lifecycle complete: %+v",lifecycle)
    }

    // A refresh after Git already matches the final state must not erase the
    // historical observations from the journal.
    again,err:=lifecycleTimings(project,"",rows);if err!=nil{t.Fatal(err)}
    againEvents:=again["B-903"]["events"].([]map[string]any)
    if len(againEvents)!=3{
        t.Fatalf("completed lifecycle history disappeared on refresh: %+v",again["B-903"])
    }
}


func TestLifecycleUsesPreservedRegistrationInsteadOfMutableFileCtime(t *testing.T) {
    project:=t.TempDir()
    if _,err:=runtimeobs.EnsureHooks(project,"codex");err!=nil{t.Fatal(err)}
    backlogDir:=filepath.Join(project,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlogDir,0755);err!=nil{t.Fatal(err)}
    task:=filepath.Join(backlogDir,"0931.B-931.mutable-ctime.doing.md")
    body:="# B-931 Mutable ctime\n- Agent: /root/controller/worker-931\n- RuntimeProvider: codex\n"
    if err:=os.WriteFile(task,[]byte(body),0644);err!=nil{t.Fatal(err)}

    base:=time.Now().UTC().Add(-2*time.Minute)
    journalPath:=filepath.Join(project,"_task_mecca",".runtime","lifecycle_observations.json")
    if err:=os.MkdirAll(filepath.Dir(journalPath),0755);err!=nil{t.Fatal(err)}
    absLedger,err:=filepath.Abs(backlogDir);if err!=nil{t.Fatal(err)}
    journal:=map[string]any{
        "version":float64(1),
        "ledgers":map[string]any{
            absLedger:map[string]any{
                "items":map[string]any{
                    "B-931":[]any{
                        map[string]any{"state":"todo","at":base.Format(time.RFC3339)},
                    },
                },
            },
        },
    }
    data,err:=json.MarshalIndent(journal,"","  ");if err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(journalPath,append(data,'\n'),0644);err!=nil{t.Fatal(err)}

    attemptID:="run-b931"
    runtimeStart:=base.Add(30*time.Second)
    start:=runtimeobs.ExecutionEvent{
        EventKind:"state",ObservedAt:runtimeStart.Format(time.RFC3339Nano),
        AttemptID:attemptID,Provider:"codex",RuntimeAgentID:"worker-931",
        State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,
        ObservationQuality:runtimeobs.QualityObserved,
    }
    if err:=runtimeobs.AppendExecutionEvent(project,start);err!=nil{t.Fatal(err)}
    bindingAt:=runtimeStart.Add(time.Second)
    if _,err:=runtimeobs.BindAttempt(project,attemptID,"B-931","/root/controller/worker-931","dispatch","",nil,bindingAt);err!=nil{t.Fatal(err)}

    rows,err:=Catalog(project,"");if err!=nil{t.Fatal(err)}
    // The live file was created/renamed much later than the preserved
    // registration observation. Old code compared start against this mutable
    // ctime and incorrectly discarded the canonical runtime start.
    row:=preferredRows(rows)["B-931"]
    if ctime,ok:=parseTime(row.Ctime);ok && !ctime.After(bindingAt){
        t.Fatalf("fixture requires mutable file ctime after runtime start: ctime=%s binding=%s",row.Ctime,bindingAt)
    }

    timings,err:=lifecycleTimings(project,"",rows);if err!=nil{t.Fatal(err)}
    lifecycle:=timings["B-931"]
    if lifecycle==nil||lifecycle["started_at"]==nil{
        t.Fatalf("canonical start was rejected despite preserved registration: %+v",lifecycle)
    }
    started,ok:=parseTime(toString(lifecycle["started_at"]));if !ok{t.Fatalf("invalid started_at: %+v",lifecycle)}
    if started.Before(bindingAt){
        t.Fatalf("task lifecycle backdated before binding: started=%s binding=%s",started,bindingAt)
    }
    events:=lifecycle["events"].([]map[string]any)
    if len(events)<2||toString(events[0]["state"])!="todo"||toString(events[1]["state"])!="doing"{
        t.Fatalf("want registered -> started lifecycle, got %+v",events)
    }
}


func TestLifecycleRejectsRuntimeStartBeforeRegistration(t *testing.T) {
    project:=t.TempDir()
    backlogDir:=filepath.Join(project,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlogDir,0755); err!=nil { t.Fatal(err) }
    task:=filepath.Join(backlogDir,"0001.B-436.test.doing.md")
    body:="---\nID: B-436\nTitle: Test\nAgent: /root/controller/pairi\nRuntimeProvider: codex\n---\n"
    if err:=os.WriteFile(task,[]byte(body),0644); err!=nil { t.Fatal(err) }

    now:=time.Now().UTC()
    oldStart:=now.Add(-2*time.Hour)
    attemptID:="run-lifecycle-old"
    start:=runtimeobs.ExecutionEvent{EventKind:"state",ObservedAt:oldStart.Format(time.RFC3339Nano),AttemptID:attemptID,Provider:"codex",RuntimeAgentID:"agent-old",State:runtimeobs.StateRunning}
    if err:=runtimeobs.AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    if _,err:=runtimeobs.BindAttempt(project,attemptID,"B-436","/root/controller/pairi","dispatch","",map[string]string{"test":"stale"},now); err!=nil { t.Fatal(err) }

    rows,err:=Catalog(project,"")
    if err!=nil { t.Fatal(err) }
    timings,err:=lifecycleTimings(project,"",rows)
    if err!=nil { t.Fatal(err) }
    lifecycle:=timings["B-436"]
    if lifecycle==nil { t.Fatal("missing lifecycle") }
    if started:=lifecycle["started_at"]; started!=nil {
        if parsed,ok:=parseTime(started.(string)); ok && parsed.Before(now.Add(-time.Minute)) {
            t.Fatalf("lifecycle was backdated before registration/binding: %v",started)
        }
    }
}


func TestCompletedLifecycleKeepsStartedAtFromImmutableEpisode(t *testing.T) {
    project:=t.TempDir()
    backlogDir:=filepath.Join(project,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlogDir,0755); err!=nil { t.Fatal(err) }
    task:=filepath.Join(backlogDir,"0441.B-441.lifecycle.doing.md")
    body:="---\nID: B-441\nTitle: Lifecycle preservation\nAgent: /root/controller/worker\nRuntimeProvider: codex\n---\n"
    if err:=os.WriteFile(task,[]byte(body),0644); err!=nil { t.Fatal(err) }
    started:=time.Now().UTC().Add(-4*time.Minute)
    completed:=started.Add(3*time.Minute)
    attemptID:="run-b441"
    start:=runtimeobs.ExecutionEvent{EventKind:"state",ObservedAt:started.Format(time.RFC3339Nano),AttemptID:attemptID,Provider:"codex",SessionID:"root-1",RuntimeAgentID:"worker-1",State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved}
    if err:=runtimeobs.AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    if _,err:=runtimeobs.BindAttempt(project,attemptID,"B-441","/root/controller/worker","dispatch","",nil,started); err!=nil { t.Fatal(err) }
    activeRows,err:=Catalog(project,""); if err!=nil { t.Fatal(err) }
    activeTimings,err:=lifecycleTimings(project,"",activeRows); if err!=nil { t.Fatal(err) }
    if activeTimings["B-441"]==nil || activeTimings["B-441"]["started_at"]==nil { t.Fatalf("active lifecycle missing start: %+v",activeTimings["B-441"]) }
    stop:=runtimeobs.ExecutionEvent{EventKind:"state",ObservedAt:completed.Format(time.RFC3339Nano),AttemptID:attemptID,Provider:"codex",SessionID:"root-1",RuntimeAgentID:"worker-1",State:runtimeobs.StateCompleted,Terminal:true,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved}
    if err:=runtimeobs.AppendExecutionEvent(project,stop); err!=nil { t.Fatal(err) }
    doneTask:=filepath.Join(backlogDir,"0441.B-441.lifecycle.done.md")
    if err:=os.Rename(task,doneTask); err!=nil { t.Fatal(err) }
    rows,err:=Catalog(project,""); if err!=nil { t.Fatal(err) }
    timings,err:=lifecycleTimings(project,"",rows); if err!=nil { t.Fatal(err) }
    lifecycle:=timings["B-441"]; if lifecycle==nil { t.Fatal("missing lifecycle") }
    if lifecycle["started_at"]==nil || lifecycle["completed_at"]==nil { t.Fatalf("completed lifecycle incomplete: %+v",lifecycle) }
    if incomplete,ok:=lifecycle["timing_incomplete"].(bool); ok && incomplete { t.Fatalf("completed lifecycle incorrectly incomplete: %+v",lifecycle) }
}


func TestControlTowerRecoveryBindingSuppressesStartedNotification(t *testing.T) {
    project:=t.TempDir()
    backlogDir:=filepath.Join(project,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlogDir,0755);err!=nil{t.Fatal(err)}
    task:=filepath.Join(backlogDir,"0900.B-900.recovery.doing.md")
    body:="# B-900 Recovery binding\n- Agent: /root/controller/worker-900\n- RuntimeProvider: codex\n"
    if err:=os.WriteFile(task,[]byte(body),0644);err!=nil{t.Fatal(err)}

    now:=time.Now().UTC()
    attemptID:="run-b900-recovery"
    episode:=runtimeobs.ExecutionEvent{
        EventKind:"episode",ObservedAt:now.Add(-6*time.Minute).Format(time.RFC3339Nano),
        AttemptID:attemptID,Provider:"codex",SessionID:"root-recovery",RuntimeAgentID:"worker-900",
        EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityAuthoritative,
    }
    if err:=runtimeobs.AppendExecutionEvent(project,episode);err!=nil{t.Fatal(err)}
    activity:=runtimeobs.ExecutionEvent{
        EventKind:"activity",ObservedAt:now.Add(-5*time.Minute).Format(time.RFC3339Nano),
        AttemptID:attemptID,Provider:"codex",SessionID:"root-recovery",RuntimeAgentID:"worker-900",
        EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
    }
    if err:=runtimeobs.AppendExecutionEvent(project,activity);err!=nil{t.Fatal(err)}

    // The notification journal already knows the doing task, but there is no
    // canonical lifecycle start until the control tower recovers the binding.
    baseline:=map[string]map[string]any{
        "B-900":{"file_state":"doing","updated_at":now.Add(-10*time.Minute).Format(time.RFC3339Nano),"title":"Recovery binding"},
    }
    if events,err:=NotificationEvents(project,baseline);err!=nil||len(events)!=0{
        t.Fatalf("baseline events=%+v err=%v",events,err)
    }

    rows,err:=Catalog(project,"");if err!=nil{t.Fatal(err)}
    control:=reconcileControlTower(project,"",rows)
    lifecycle:=control.Timings["B-900"]
    if lifecycle==nil||lifecycle["started_at"]==nil{
        t.Fatalf("recovery binding did not produce canonical start: %+v",lifecycle)
    }
    suppressed,ok:=lifecycle["started_notification_suppressed"].(bool)
    if !ok||!suppressed{
        t.Fatalf("recovery start must be marked notification-suppressed: %+v",lifecycle)
    }

    current:=map[string]map[string]any{
        "B-900":{
            "file_state":"doing","updated_at":now.Format(time.RFC3339Nano),"title":"Recovery binding",
            "lifecycle":lifecycle,
        },
    }
    events,err:=NotificationEvents(project,current);if err!=nil{t.Fatal(err)}
    if len(events)!=0{
        t.Fatalf("recovered historical start must not notify: %+v",events)
    }
}




func TestFreshRecoveryBindingEmitsStartedNotification(t *testing.T) {
    project:=t.TempDir()
    backlogDir:=filepath.Join(project,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlogDir,0755);err!=nil{t.Fatal(err)}
    task:=filepath.Join(backlogDir,"0902.B-902.fresh-recovery.doing.md")
    body:="# B-902 Fresh recovery\n- Agent: /root/controller/worker-902\n- RuntimeProvider: codex\n"
    if err:=os.WriteFile(task,[]byte(body),0644);err!=nil{t.Fatal(err)}

    if events,err:=NotificationEvents(project,map[string]map[string]any{
        "B-902":{"file_state":"doing","updated_at":time.Now().UTC().Format(time.RFC3339Nano),"title":"Fresh recovery"},
    });err!=nil||len(events)!=0{
        t.Fatalf("baseline events=%+v err=%v",events,err)
    }

    attemptID:="run-b902-fresh-recovery"
    activityAt:=time.Now().UTC().Add(-5*time.Second)
    activity:=runtimeobs.ExecutionEvent{
        EventKind:"activity",ObservedAt:activityAt.Format(time.RFC3339Nano),
        AttemptID:attemptID,Provider:"codex",SessionID:"root-fresh",RuntimeAgentID:"worker-902",
        EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
    }
    if err:=runtimeobs.AppendExecutionEvent(project,activity);err!=nil{t.Fatal(err)}

    rows,err:=Catalog(project,"");if err!=nil{t.Fatal(err)}
    control:=reconcileControlTower(project,"",rows)
    lifecycle:=control.Timings["B-902"]
    if lifecycle==nil||lifecycle["started_at"]==nil{
        t.Fatalf("fresh recovery did not produce canonical start: %+v",lifecycle)
    }
    if suppressed,ok:=lifecycle["started_notification_suppressed"].(bool);ok&&suppressed{
        t.Fatalf("fresh recovery start must remain notifyable: %+v",lifecycle)
    }

    events,err:=NotificationEvents(project,map[string]map[string]any{
        "B-902":{
            "file_state":"doing","updated_at":time.Now().UTC().Format(time.RFC3339Nano),"title":"Fresh recovery",
            "lifecycle":lifecycle,
        },
    })
    if err!=nil{t.Fatal(err)}
    if len(events)!=1||toString(events[0]["kind"])!="started"{
        t.Fatalf("fresh recovered start notification missing: %+v",events)
    }
}


func TestRecoveredBindingThatStartsLaterEmitsStartedAndDropsTransientUnknown(t *testing.T) {
    project:=t.TempDir()
    if _,err:=runtimeobs.EnsureHooks(project,"codex");err!=nil{t.Fatal(err)}
    backlogDir:=filepath.Join(project,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlogDir,0755);err!=nil{t.Fatal(err)}
    task:=filepath.Join(backlogDir,"0901.B-901.live-recovery.doing.md")
    body:="# B-901 Live recovery\n- Agent: /root/controller/worker-901\n- RuntimeProvider: codex\n"
    if err:=os.WriteFile(task,[]byte(body),0644);err!=nil{t.Fatal(err)}

    if events,err:=NotificationEvents(project,map[string]map[string]any{
        "B-901":{"file_state":"doing","updated_at":time.Now().UTC().Format(time.RFC3339Nano),"title":"Live recovery"},
    });err!=nil||len(events)!=0{
        t.Fatalf("baseline events=%+v err=%v",events,err)
    }

    attemptID:="run-b901-live-recovery"
    unknown:=runtimeobs.ExecutionEvent{
        EventKind:"state",ObservedAt:time.Now().UTC().Add(-2*time.Second).Format(time.RFC3339Nano),
        AttemptID:attemptID,Provider:"codex",SessionID:"root-live",RuntimeAgentID:"worker-901",
        State:runtimeobs.StateRuntimeUnknown,EvidenceSource:runtimeobs.EvidenceHook,
        ObservationQuality:runtimeobs.QualityObserved,
    }
    if err:=runtimeobs.AppendExecutionEvent(project,unknown);err!=nil{t.Fatal(err)}

    rows,err:=Catalog(project,"");if err!=nil{t.Fatal(err)}
    first:=reconcileControlTower(project,"",rows)
    firstLifecycle:=first.Timings["B-901"]
    if firstLifecycle!=nil && firstLifecycle["started_at"]!=nil{
        t.Fatalf("runtime_unknown recovery must not manufacture a start: %+v",firstLifecycle)
    }
    firstEvents,err:=NotificationEvents(project,map[string]map[string]any{
        "B-901":{
            "file_state":"doing","updated_at":time.Now().UTC().Format(time.RFC3339Nano),"title":"Live recovery",
            "lifecycle":firstLifecycle,"notification_condition":first.NotificationCondition["B-901"],
        },
    })
    if err!=nil{t.Fatal(err)}
    if len(firstEvents)!=0{
        t.Fatalf("transient runtime_unknown must stay out of push notifications: %+v",firstEvents)
    }

    activity:=runtimeobs.ExecutionEvent{
        EventKind:"activity",ObservedAt:time.Now().UTC().Format(time.RFC3339Nano),
        AttemptID:attemptID,Provider:"codex",SessionID:"root-live",RuntimeAgentID:"worker-901",
        EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
    }
    if err:=runtimeobs.AppendExecutionEvent(project,activity);err!=nil{t.Fatal(err)}

    rows,err=Catalog(project,"");if err!=nil{t.Fatal(err)}
    second:=reconcileControlTower(project,"",rows)
    lifecycle:=second.Timings["B-901"]
    if lifecycle==nil||lifecycle["started_at"]==nil{
        t.Fatalf("live recovery did not establish canonical start: %+v",lifecycle)
    }
    if suppressed,ok:=lifecycle["started_notification_suppressed"].(bool);ok&&suppressed{
        t.Fatalf("later live start must not inherit recovery suppression: %+v",lifecycle)
    }

    events,err:=NotificationEvents(project,map[string]map[string]any{
        "B-901":{
            "file_state":"doing","updated_at":time.Now().UTC().Format(time.RFC3339Nano),"title":"Live recovery",
            "lifecycle":lifecycle,
        },
    })
    if err!=nil{t.Fatal(err)}
    starts,unknowns:=0,0
    for _,event:=range events{
        if toString(event["task_id"])!="B-901"{continue}
        switch toString(event["kind"]){
        case "started": starts++
        case "runtime_unknown": unknowns++
        }
    }
    if starts!=1||unknowns!=0{
        t.Fatalf("want one started and no transient runtime_unknown, got events=%+v",events)
    }
}

func TestControlTowerDoesNotRecoverBindStaleUnboundAttempt(t *testing.T) {
    project:=t.TempDir()
    now:=time.Now().UTC()
    attemptID:="run-stale-unbound"
    start:=runtimeobs.ExecutionEvent{
        EventKind:"state",
        ObservedAt:now.Add(-2*time.Hour).Format(time.RFC3339Nano),
        AttemptID:attemptID,
        Provider:"codex",
        RuntimeAgentID:"worker-stale",
        State:runtimeobs.StateRunning,
        EvidenceSource:runtimeobs.EvidenceHook,
        ObservationQuality:runtimeobs.QualityObserved,
    }
    if err:=runtimeobs.AppendExecutionEvent(project,start);err!=nil{t.Fatal(err)}

    rows:=[]Record{{
        ID:"B-999",State:"doing",Location:"active",
        Fields:map[string]string{"Agent":"/root/controller/worker-stale"},
    }}
    recovered:=reconcileCanonicalBindings(project,rows,now)
    if recovered["B-999"]{
        t.Fatalf("stale unbound attempt must never be auto-bound: %+v",recovered)
    }

    ledger,err:=runtimeobs.ReconcileLedger(project,10,now)
    if err!=nil{t.Fatal(err)}
    found:=false
    for _,attempt:=range ledger.Attempts{
        if attempt.AttemptID!=attemptID{continue}
        found=true
        if attempt.BindingState!=runtimeobs.BindingUnbound{
            t.Fatalf("stale attempt binding was mutated: %+v",attempt)
        }
    }
    if !found{t.Fatal("stale attempt missing from ledger")}
}


func TestControlTowerDoesNotRecoverBindAmbiguousAttempt(t *testing.T) {
    project:=t.TempDir()
    now:=time.Now().UTC()
    attemptID:="run-ambiguous-unbound"
    start:=runtimeobs.ExecutionEvent{
        EventKind:"state",ObservedAt:now.Add(-time.Minute).Format(time.RFC3339Nano),
        AttemptID:attemptID,Provider:"codex",RuntimeAgentID:"worker-ambiguous",
        State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,
        ObservationQuality:runtimeobs.QualityObserved,
    }
    if err:=runtimeobs.AppendExecutionEvent(project,start);err!=nil{t.Fatal(err)}

    at:=now.Add(-30*time.Second).Format(time.RFC3339Nano)
    first:=runtimeobs.ExecutionEvent{
        EventKind:"binding",ObservedAt:at,AttemptID:attemptID,
        TaskID:"B-OLD-1",AgentPath:"/root/controller/worker-a",
        BindingSource:"explicit",EvidenceSource:runtimeobs.EvidenceManualBinding,
        ObservationQuality:runtimeobs.QualityAuthoritative,
    }
    second:=runtimeobs.ExecutionEvent{
        EventKind:"binding",ObservedAt:at,AttemptID:attemptID,
        TaskID:"B-OLD-2",AgentPath:"/root/controller/worker-b",
        BindingSource:"explicit",EvidenceSource:runtimeobs.EvidenceManualBinding,
        ObservationQuality:runtimeobs.QualityAuthoritative,
    }
    if err:=runtimeobs.AppendExecutionEvent(project,first);err!=nil{t.Fatal(err)}
    if err:=runtimeobs.AppendExecutionEvent(project,second);err!=nil{t.Fatal(err)}

    before,err:=runtimeobs.ReconcileLedger(project,10,now)
    if err!=nil{t.Fatal(err)}
    ambiguous:=false
    for _,attempt:=range before.Attempts{
        if attempt.AttemptID==attemptID && attempt.BindingState==runtimeobs.BindingAmbiguous{ambiguous=true}
    }
    if !ambiguous{t.Fatalf("test fixture did not produce ambiguous attempt: %+v",before.Attempts)}

    rows:=[]Record{{
        ID:"B-998",State:"doing",Location:"active",
        Fields:map[string]string{"Agent":"/root/controller/worker-ambiguous"},
    }}
    recovered:=reconcileCanonicalBindings(project,rows,now)
    if recovered["B-998"]{t.Fatalf("ambiguous attempt must never be auto-bound: %+v",recovered)}

    after,err:=runtimeobs.ReconcileLedger(project,10,now)
    if err!=nil{t.Fatal(err)}
    for _,attempt:=range after.Attempts{
        if attempt.AttemptID==attemptID && attempt.BindingState!=runtimeobs.BindingAmbiguous{
            t.Fatalf("ambiguous binding was overwritten by recovery: %+v",attempt)
        }
    }
}


func TestControlTowerRecoveryBindingRespectsRuntimeProvider(t *testing.T) {
    project:=t.TempDir()
    now:=time.Now().UTC()
    attemptID:="run-provider-mismatch"
    start:=runtimeobs.ExecutionEvent{
        EventKind:"state",ObservedAt:now.Add(-time.Minute).Format(time.RFC3339Nano),
        AttemptID:attemptID,Provider:"claude",RuntimeAgentID:"worker-provider",
        State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,
        ObservationQuality:runtimeobs.QualityObserved,
    }
    if err:=runtimeobs.AppendExecutionEvent(project,start);err!=nil{t.Fatal(err)}

    rows:=[]Record{{
        ID:"B-997",State:"doing",Location:"active",
        Fields:map[string]string{
            "Agent":"/root/controller/worker-provider",
            "RuntimeProvider":"codex",
        },
    }}
    recovered:=reconcileCanonicalBindings(project,rows,now)
    if recovered["B-997"]{
        t.Fatalf("provider-mismatched attempt must never be auto-bound: %+v",recovered)
    }

    ledger,err:=runtimeobs.ReconcileLedger(project,10,now)
    if err!=nil{t.Fatal(err)}
    for _,attempt:=range ledger.Attempts{
        if attempt.AttemptID==attemptID && attempt.BindingState!=runtimeobs.BindingUnbound{
            t.Fatalf("provider mismatch mutated binding: %+v",attempt)
        }
    }
}
