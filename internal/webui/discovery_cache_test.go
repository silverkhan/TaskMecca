package webui

import (
    "errors"
    "sync"
    "sync/atomic"
    "testing"
    "time"

    "github.com/silverkhan/TaskMecca/internal/backlog"
)

func clearTestDiscoveryKey(t *testing.T, key string) {
    t.Helper()
    discoveryCache.Lock()
    delete(discoveryCache.entries,key)
    discoveryCache.Unlock()
    t.Cleanup(func(){clearTestDiscoveryKeyNoCleanup(key)})
}

func clearTestDiscoveryKeyNoCleanup(key string) {
    discoveryCache.Lock()
    delete(discoveryCache.entries,key)
    discoveryCache.Unlock()
}

func expireTestDiscoveryKey(t *testing.T,key string) {
    t.Helper()
    discoveryCache.Lock()
    entry:=discoveryCache.entries[key]
    entry.at=time.Now().Add(-2*backlogDiscoveryRefreshInterval)
    discoveryCache.entries[key]=entry
    discoveryCache.Unlock()
}

func TestDiscoveryCacheServesStaleWhileRefreshing(t *testing.T) {
    project,root:="stale-test","stale-root"
    key:=project+"\x00"+root
    clearTestDiscoveryKey(t,key)
    entered:=make(chan struct{})
    release:=make(chan struct{})
    complete:=make(chan struct{})
    var calls atomic.Int32
    scan:=func(_, _ string) ([]backlog.Candidate,error) {
        n:=calls.Add(1)
        if n==1{return []backlog.Candidate{{Name:"backlog-original"}},nil}
        close(entered)
        <-release
        defer close(complete)
        return []backlog.Candidate{{Name:"backlog-refreshed"}},nil
    }
    first,err:=cachedBacklogDiscoverWith(project,root,scan)
    if err!=nil || len(first)!=1 {t.Fatalf("first=%v err=%v",first,err)}
    expireTestDiscoveryKey(t,key)
    stale,err:=cachedBacklogDiscoverWith(project,root,scan)
    if err!=nil || len(stale)!=1 || stale[0].Name!="backlog-original" {
        t.Fatalf("stale result=%v err=%v",stale,err)
    }
    select {
    case <-entered:
    case <-time.After(2*time.Second):t.Fatal("background refresh did not begin")
    }
    // A second HTTP request must never block on the still-running scan.
    fast:=make(chan []backlog.Candidate,1)
    go func(){rows,_:=cachedBacklogDiscoverWith(project,root,scan);fast<-rows}()
    select {
    case rows:=<-fast:
        if len(rows)!=1 || rows[0].Name!="backlog-original" {t.Fatalf("stale rows=%v",rows)}
    case <-time.After(300*time.Millisecond):
        t.Fatal("fast request waited on slow discovery")
    }
    if calls.Load()!=2 {t.Fatalf("duplicate discovery scans: %d",calls.Load())}
    // Callers cannot modify the globally cached candidate slice.
    stale[0].Name="corrupted"
    close(release)
    select {
    case <-complete:
    case <-time.After(2*time.Second):t.Fatal("background discovery did not finish")
    }
    deadline:=time.Now().Add(time.Second)
    for time.Now().Before(deadline) {
        latest,err:=cachedBacklogDiscoverWith(project,root,scan)
        if err==nil && len(latest)==1 && latest[0].Name=="backlog-refreshed" {return}
        time.Sleep(time.Millisecond)
    }
    t.Fatal("background discovery did not replace stale result")
}

func TestDiscoveryCacheColdRequestsShareOneScan(t *testing.T) {
    project,root:="singleflight-test","singleflight-root"
    key:=project+"\x00"+root
    clearTestDiscoveryKey(t,key)
    release:=make(chan struct{})
    entered:=make(chan struct{})
    var calls atomic.Int32
    scan:=func(_, _ string) ([]backlog.Candidate,error) {
        calls.Add(1)
        close(entered)
        <-release
        return []backlog.Candidate{{Name:"backlog"}},nil
    }
    var wg sync.WaitGroup
    errCh:=make(chan error,2)
    wg.Add(2)
    go func(){defer wg.Done();_,err:=cachedBacklogDiscoverWith(project,root,scan);errCh<-err}()
    select {
    case <-entered:
    case <-time.After(time.Second):t.Fatal("cold discover did not begin")
    }
    go func(){defer wg.Done();_,err:=cachedBacklogDiscoverWith(project,root,scan);errCh<-err}()
    time.Sleep(10*time.Millisecond)
    if calls.Load()!=1 {t.Errorf("duplicate concurrent cold discover %d",calls.Load())}
    close(release)
    wg.Wait()
    close(errCh)
    for err:=range errCh {if err!=nil {t.Fatal(err)}}
    if calls.Load()!=1 {t.Fatalf("cold discover called %d times",calls.Load())}
}

func TestDiscoveryCacheKeepsVerifiedSnapshotAfterRefreshFailure(t *testing.T) {
    project,root:="error-test","error-root"
    key:=project+"\x00"+root
    clearTestDiscoveryKey(t,key)
    var calls atomic.Int32
    scan:=func(_, _ string) ([]backlog.Candidate,error){
        if calls.Add(1)==1{return []backlog.Candidate{{Name:"backlog"}},nil}
        return nil,errors.New("scan failed")
    }
    if _,err:=cachedBacklogDiscoverWith(project,root,scan);err!=nil {t.Fatal(err)}
    expireTestDiscoveryKey(t,key)
    got,err:=cachedBacklogDiscoverWith(project,root,scan)
    if err!=nil || len(got)!=1 {t.Fatalf("stale=%v err=%v",got,err)}
    deadline:=time.Now().Add(time.Second)
    for time.Now().Before(deadline){
        discoveryCache.Lock()
        refreshed:=!discoveryCache.entries[key].refreshing
        discoveryCache.Unlock()
        if refreshed{break}
        time.Sleep(time.Millisecond)
    }
    for i:=0;i<5;i++{
        got,err=cachedBacklogDiscoverWith(project,root,scan)
        if err!=nil || len(got)!=1 || got[0].Name!="backlog" {t.Fatalf("snapshot lost after error: %v, %v",got,err)}
    }
    if calls.Load()!=2 {t.Fatalf("failure caused a discovery stampede: %d",calls.Load())}
}
