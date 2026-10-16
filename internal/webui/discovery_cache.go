package webui

import (
    "sync"
    "time"

    "github.com/silverkhan/TaskMecca/internal/backlog"
)

const backlogDiscoveryRefreshInterval = 5 * time.Second

type discoveryCacheEntry struct {
    at time.Time
    candidates []backlog.Candidate
    valid bool
    refreshing bool
    pending chan struct{}
    err error
}

var discoveryCache = struct {
    sync.Mutex
    entries map[string]discoveryCacheEntry
}{entries:map[string]discoveryCacheEntry{}}

func copyCandidates(rows []backlog.Candidate) []backlog.Candidate {
    return append([]backlog.Candidate{}, rows...)
}

// Folder discovery can walk thousands of archived documents. Never make a
// previously successful content/initial-list request synchronously repeat that
// walk merely because the 5-second discovery TTL expired. Refresh a verified
// candidate snapshot in the background, at most once per project/scan root.
//
// Cold starts still need a real discovery; simultaneous cold requests share
// one scan rather than stampeding the filesystem. Failures never replace an
// existing valid snapshot with an untrusted/empty list.
func cachedBacklogDiscover(project, root string) ([]backlog.Candidate,error) {
    return cachedBacklogDiscoverWith(project,root,backlog.Discover)
}

func cachedBacklogDiscoverWith(project,root string,discover func(string,string)([]backlog.Candidate,error)) ([]backlog.Candidate,error) {
    key:=project+"\x00"+root
    discoveryCache.Lock()
    entry,exists:=discoveryCache.entries[key]
    if exists && entry.valid {
        rows:=copyCandidates(entry.candidates)
        if time.Since(entry.at)>=backlogDiscoveryRefreshInterval && !entry.refreshing {
            entry.refreshing=true
            discoveryCache.entries[key]=entry
            go func() {
                updated,err:=discover(project,root)
                discoveryCache.Lock()
                current:=discoveryCache.entries[key]
                if err==nil {
                    current.candidates=copyCandidates(updated)
                    current.valid=true
                }
                // Apply a backoff even on errors, otherwise every HTTP read
                // would spawn a new expensive background discover scan.
                current.at=time.Now()
                current.refreshing=false
                discoveryCache.entries[key]=current
                discoveryCache.Unlock()
            }()
        }
        discoveryCache.Unlock()
        return rows,nil
    }
    if exists && entry.refreshing && entry.pending!=nil {
        pending:=entry.pending
        discoveryCache.Unlock()
        <-pending
        discoveryCache.Lock()
        completed:=discoveryCache.entries[key]
        discoveryCache.Unlock()
        if completed.valid {return copyCandidates(completed.candidates),nil}
        return nil,completed.err
    }
    // No successful prior result: only the first cold caller scans.
    pending:=make(chan struct{})
    discoveryCache.entries[key]=discoveryCacheEntry{refreshing:true,pending:pending}
    discoveryCache.Unlock()

    rows,err:=discover(project,root)
    discoveryCache.Lock()
    completed:=discoveryCache.entries[key]
    if err==nil {
        completed.candidates=copyCandidates(rows)
        completed.valid=true
    }
    completed.err=err
    completed.at=time.Now()
    completed.refreshing=false
    completed.pending=nil
    discoveryCache.entries[key]=completed
    close(pending)
    discoveryCache.Unlock()
    if err!=nil{return nil,err}
    return copyCandidates(rows),nil
}
