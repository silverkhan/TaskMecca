package webui

import (
    "fmt"
    "sync/atomic"
    "testing"
    "time"

    "github.com/silverkhan/TaskMecca/internal/backlog"
)

func TestDiscoveryInvalidationCannotResurrectBackgroundSnapshot(t *testing.T) {
    project,root:="cache-invalidation-project","root"
    key:=project+"\x00"+root
    clearTestDiscoveryKey(t,key)
    entered:=make(chan struct{})
    release:=make(chan struct{})
    done:=make(chan struct{})
    var count atomic.Int32
    scan:=func(_, _ string)([]backlog.Candidate,error){
        switch count.Add(1) {
        case 1: return []backlog.Candidate{{Name:"original"}},nil
        case 2:
            close(entered)
            <-release
            defer close(done)
            return []backlog.Candidate{{Name:"obsolete"}},nil
        default: return []backlog.Candidate{{Name:"current"}},nil
        }
    }
    if _,err:=cachedBacklogDiscoverWith(project,root,scan);err!=nil{t.Fatal(err)}
    expireTestDiscoveryKey(t,key)
    if _,err:=cachedBacklogDiscoverWith(project,root,scan);err!=nil{t.Fatal(err)}
    select{case <-entered:case <-time.After(2*time.Second):t.Fatal("refresh never started")}
    invalidateProjectDiscovery(project)
    if _,err:=cachedBacklogDiscoverWith(project,root,scan);err!=nil{t.Fatal(err)}
    close(release)
    select{case <-done:case <-time.After(2*time.Second):t.Fatal("obsolete refresh blocked")}
    // Ensure asynchronous completion had time to acquire the shared mutex.
    deadline:=time.Now().Add(time.Second)
    for time.Now().Before(deadline) {
        discoveryCache.Lock()
        value,exists:=discoveryCache.entries[key]
        discoveryCache.Unlock()
        if exists && value.valid && len(value.candidates)==1 && value.candidates[0].Name=="current" {return}
        time.Sleep(time.Millisecond)
    }
    t.Fatal("invalidated background result overwrote new canonical discovery")
}

func TestDiscoveryCacheBoundedAtColdReadBoundary(t *testing.T) {
    project:="bounded-discovery"
    defer invalidateProjectDiscovery(project)
    scan:=func(_,root string)([]backlog.Candidate,error){
        return []backlog.Candidate{{Name:root}},nil
    }
    for i:=0;i<discoveryCacheLimit+8;i++ {
        root:=fmt.Sprintf("root-%03d",i)
        if _,err:=cachedBacklogDiscoverWith(project,root,scan);err!=nil{t.Fatal(err)}
    }
    discoveryCache.Lock()
    size:=len(discoveryCache.entries)
    discoveryCache.Unlock()
    if size>discoveryCacheLimit {t.Fatalf("cache grew to %d, limit %d",size,discoveryCacheLimit)}
}
