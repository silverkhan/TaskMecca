package backlog

import (
    "sync"
    "testing"
    "time"
)

func TestCatalogRefreshLockIsPerBacklog(t *testing.T) {
    a:=catalogRefreshLock(t.TempDir()+"/first")
    same:=catalogRefreshLock(t.TempDir()+"/same")
    if a==same { t.Fatal("unrelated backlogs unexpectedly share refresh mutex") }
    a.Lock()
    defer a.Unlock()
    done:=make(chan struct{})
    go func(){ same.Lock(); same.Unlock(); close(done) }()
    select {
    case <-done:
    case <-time.After(time.Second):
        t.Fatal("another backlog was blocked by a different project's refresh")
    }
}

func TestCatalogRefreshLockSerializesSameBacklog(t *testing.T) {
    key:=t.TempDir()+"/backlog"
    a,b:=catalogRefreshLock(key),catalogRefreshLock(key)
    if a!=b { t.Fatal("same backlog must share one refresh lock") }
    a.Lock()
    var entered sync.WaitGroup
    entered.Add(1)
    done:=make(chan struct{})
    go func(){
        entered.Done()
        b.Lock()
        b.Unlock()
        close(done)
    }()
    entered.Wait()
    select {
    case <-done:
        a.Unlock()
        t.Fatal("same backlog refreshed concurrently")
    case <-time.After(30*time.Millisecond):
    }
    a.Unlock()
    select {
    case <-done:
    case <-time.After(time.Second):
        t.Fatal("same-backlog refresh failed to resume")
    }
}
