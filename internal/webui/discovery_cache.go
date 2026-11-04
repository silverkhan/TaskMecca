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
    token uint64
    pending chan struct{}
    err error
}

var discoveryCache = struct {
    sync.Mutex
    entries map[string]discoveryCacheEntry
    nextToken uint64
}{entries:map[string]discoveryCacheEntry{}}

const discoveryCacheLimit = 64

// Discard only completed entries. In-flight cold requests must finish for
// their waiting readers; in-flight background work may not resurrect an
// explicitly invalidated project or obsolete root.
func invalidateProjectDiscovery(project string) {
    discoveryCache.Lock()
    defer discoveryCache.Unlock()
    prefix:=project+"\x00"
    for key,entry:=range discoveryCache.entries {
        if len(key)<len(prefix) || key[:len(prefix)]!=prefix { continue }
        if entry.refreshing && !entry.valid {
            entry.at=time.Time{}
            entry.token=0
            discoveryCache.entries[key]=entry
        } else {
            delete(discoveryCache.entries,key)
        }
    }
}

func pruneDiscoveryCacheLocked() {
    for len(discoveryCache.entries)>=discoveryCacheLimit {
        oldestKey:=""
        var oldest time.Time
        for key,entry:=range discoveryCache.entries {
            if entry.refreshing { continue }
            if oldestKey=="" || entry.at.Before(oldest) {
                oldestKey,oldest=key,entry.at
            }
        }
        if oldestKey=="" { return }
        delete(discoveryCache.entries,oldestKey)
    }
}


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
        recordMonitorPerf("discovery_cache_hit", 0, len(rows), 0, false, true)
        if time.Since(entry.at)>=backlogDiscoveryRefreshInterval && !entry.refreshing {
            entry.refreshing=true
            discoveryCache.nextToken++
            entry.token=discoveryCache.nextToken
            token:=entry.token
            discoveryCache.entries[key]=entry
            go func() {
                started:=time.Now()
                updated,err:=discover(project,root)
                recordMonitorPerf("discovery_background_scan",time.Since(started),len(updated),0,false,false)
                discoveryCache.Lock()
                current,still:=discoveryCache.entries[key]
                if still && current.refreshing && current.token==token {
                    if err==nil {
                        current.candidates=copyCandidates(updated)
                        current.valid=true
                    }
                    // Back off even on failure; a failing source must not
                    // spawn an unbounded number of background file walks.
                    current.at=time.Now()
                    current.refreshing=false
                    discoveryCache.entries[key]=current
                }
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
        completed,still:=discoveryCache.entries[key]
        discoveryCache.Unlock()
        if still && completed.valid {return copyCandidates(completed.candidates),nil}
        // Invalidation during cold scan: the completed scan cannot be
        // trusted; perform a new scan on the next request.
        return cachedBacklogDiscoverWith(project,root,discover)
    }
    // Single-flight cold scans. Explicit invalidation never revives an
    // obsolete background result.
    pruneDiscoveryCacheLocked()
    pending:=make(chan struct{})
    discoveryCache.nextToken++
    token:=discoveryCache.nextToken
    discoveryCache.entries[key]=discoveryCacheEntry{refreshing:true,pending:pending,token:token}
    discoveryCache.Unlock()
    started:=time.Now()
    rows,err:=discover(project,root)
    recordMonitorPerf("discovery_cold_scan",time.Since(started),len(rows),0,false,false)
    discoveryCache.Lock()
    completed,still:=discoveryCache.entries[key]
    if still && completed.refreshing && completed.token==token {
        if err==nil {
            completed.candidates=copyCandidates(rows)
            completed.valid=true
        }
        completed.err=err
        completed.at=time.Now()
        completed.refreshing=false
        completed.pending=nil
        discoveryCache.entries[key]=completed
    } else if still && completed.pending==pending {
        // Project invalidated while the cold filesystem scan was in flight.
        // Wake old waiters and force a fresh scan after this one.
        delete(discoveryCache.entries,key)
    }
    close(pending)
    discoveryCache.Unlock()
    if err!=nil{return nil,err}
    return copyCandidates(rows),nil
}
