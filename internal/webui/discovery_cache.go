package webui

import (
    "sync"
    "time"

    "github.com/silverkhan/TaskMecca/internal/backlog"
)

type discoveryCacheEntry struct {
    at time.Time
    candidates []backlog.Candidate
}

var discoveryCache = struct {
    sync.Mutex
    entries map[string]discoveryCacheEntry
}{entries:map[string]discoveryCacheEntry{}}

func cachedBacklogDiscover(project,root string) ([]backlog.Candidate,error) {
    key:=project+"\x00"+root
    now:=time.Now()
    discoveryCache.Lock()
    if entry,ok:=discoveryCache.entries[key]; ok && now.Sub(entry.at)<5*time.Second {
        rows:=append([]backlog.Candidate{},entry.candidates...)
        discoveryCache.Unlock()
        return rows,nil
    }
    discoveryCache.Unlock()

    rows,err:=backlog.Discover(project,root)
    if err!=nil { return nil,err }
    discoveryCache.Lock()
    discoveryCache.entries[key]=discoveryCacheEntry{at:now,candidates:append([]backlog.Candidate{},rows...)}
    discoveryCache.Unlock()
    return rows,nil
}
