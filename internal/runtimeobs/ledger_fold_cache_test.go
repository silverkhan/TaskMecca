package runtimeobs

import (
    "os"
    "testing"
    "time"
)

func TestExecutionFoldCacheRefreshAndClockProjection(t *testing.T) {
    project:=t.TempDir()
    start:=time.Date(2026,time.October,15,1,0,0,0,time.UTC)
    event:=ExecutionEvent{
        EventID:"fold-start",EventKind:"state",AttemptID:"run-fold",
        ObservedAt:start.Format(time.RFC3339Nano),State:StateRunning,
        EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
    }
    if err:=AppendExecutionEvent(project,event);err!=nil {t.Fatal(err)}
    initial,err:=BuildLedger(project,20,start.Add(time.Second))
    if err!=nil {t.Fatal(err)}
    if len(initial.Attempts)!=1 || initial.Attempts[0].CurrentState!=StateRunning {t.Fatalf("first fold: %+v",initial.Attempts)}
    files,err:=executionEventFiles(project)
    if err!=nil {t.Fatal(err)}
    stamps,ok:=executionFileFingerprints(files)
    if !ok {t.Fatal("could not fingerprint execution files")}
    if _,_,hit:=cachedExecutionAccumulators(project,stamps,true);!hit {t.Fatal("event fold was not cached")}

    later,err:=ReconcileLedger(project,1,start.Add(45*time.Minute))
    if err!=nil {t.Fatal(err)}
    if later.Attempts[0].ElapsedMillis<=initial.Attempts[0].ElapsedMillis {t.Fatal("cached fold froze current elapsed time")}
    stale:=false
    for _,finding:=range later.Findings {if finding.Code=="stale" {stale=true}}
    if !stale {t.Fatal("cached fold froze stale detection")}

    event.EventID="fold-end"
    event.ObservedAt=start.Add(46*time.Minute).Format(time.RFC3339Nano)
    event.State=StateCompleted
    event.Terminal=true
    if err:=AppendExecutionEvent(project,event);err!=nil {t.Fatal(err)}
    updated,err:=ReconcileLedger(project,20,start.Add(47*time.Minute))
    if err!=nil {t.Fatal(err)}
    if len(updated.Attempts)!=1 || updated.Attempts[0].CurrentState!=StateCompleted || !updated.Attempts[0].Terminal {
        t.Fatalf("appended terminal event was not applied: %+v",updated.Attempts)
    }
    if err:=os.Remove(files[0]);err!=nil {t.Fatal(err)}
    empty,err:=BuildLedger(project,20,start.Add(48*time.Minute))
    if err!=nil {t.Fatal(err)}
    if len(empty.Attempts)!=0 {t.Fatal("deleted source journal retained stale cached attempts")}
}

func TestExecutionFoldCacheHonorsTransitionLimitAndCopyIsolation(t *testing.T) {
    project:=t.TempDir()
    start:=time.Date(2026,time.October,15,1,0,0,0,time.UTC)
    for i,eventState:=range []CanonicalState{StateRunning,StateWaitingUser,StateRunning} {
        e:=ExecutionEvent{EventID:[]string{"a","b","c"}[i],EventKind:"state",AttemptID:"run-limit",
            ObservedAt:start.Add(time.Duration(i)*time.Minute).Format(time.RFC3339Nano),
            State:eventState,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved}
        if err:=AppendExecutionEvent(project,e);err!=nil {t.Fatal(err)}
    }
    one,err:=BuildLedger(project,1,start.Add(4*time.Minute))
    if err!=nil {t.Fatal(err)}
    if len(one.Attempts[0].RecentTransitions)!=1 {t.Fatal("first limit was ignored")}
    one.Attempts[0].RecentTransitions[0].Reason="caller mutation"
    many,err:=BuildLedger(project,10,start.Add(5*time.Minute))
    if err!=nil {t.Fatal(err)}
    if len(many.Attempts[0].RecentTransitions)!=3 {t.Fatalf("cached transition limit: %+v",many.Attempts[0].RecentTransitions)}
    for _,transition:=range many.Attempts[0].RecentTransitions {
        if transition.Reason=="caller mutation" {t.Fatal("caller corrupted reusable transition evidence")}
    }
}
