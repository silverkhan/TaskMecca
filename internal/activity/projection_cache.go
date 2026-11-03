package activity

import (
    "sync"
    "time"
)

const projectionTTL=2*time.Second
const projectionMaxEvents=20000
const projectionMaxProjects=4

type cachedProjection struct {
    at time.Time
    events []Event
}
type projectionFlight struct {
    done chan struct{}
    events []Event
    err error
}
var projectionCache struct {
    sync.Mutex
    entries map[string]cachedProjection
    inflight map[string]*projectionFlight
    revision uint64
}

// ListCached is ONLY for read-only Web projections. Mutations always use
// Ledger.List beneath the canonical write gate. Concurrent read misses on the
// same project share one disk scan; reads on different projects do not block.
// A scan racing with a same-process write cannot repopulate stale cache data.
func (l Ledger) ListCached(taskID string)([]Event,error) {
    now:=time.Now()
    projectionCache.Lock()
    if entry,ok:=projectionCache.entries[l.Project];ok&&now.Sub(entry.at)<projectionTTL {
        projectionCache.Unlock()
        return filterProjectedEvents(entry.events,taskID),nil
    }
    if flight:=projectionCache.inflight[l.Project];flight!=nil {
        projectionCache.Unlock()
        <-flight.done
        if flight.err!=nil{return nil,flight.err}
        return filterProjectedEvents(flight.events,taskID),nil
    }
    if projectionCache.inflight==nil{projectionCache.inflight=make(map[string]*projectionFlight)}
    flight:=&projectionFlight{done:make(chan struct{})}
    revision:=projectionCache.revision
    projectionCache.inflight[l.Project]=flight
    projectionCache.Unlock()

    all,err:=l.List("")
    projectionCache.Lock()
    // Never publish an older disk snapshot over a concurrent in-process
    // append. Cross-process writers are bounded by the short TTL.
    if err==nil && revision==projectionCache.revision && len(all)<=projectionMaxEvents {
        if projectionCache.entries==nil{projectionCache.entries=make(map[string]cachedProjection)}
        if len(projectionCache.entries)>=projectionMaxProjects {
            var oldestKey string
            var oldestTime time.Time
            for key,value:=range projectionCache.entries {
                if oldestKey==""||value.at.Before(oldestTime){oldestKey,oldestTime=key,value.at}
            }
            delete(projectionCache.entries,oldestKey)
        }
        projectionCache.entries[l.Project]=cachedProjection{at:now,events:all}
    }
    flight.events=all
    flight.err=err
    delete(projectionCache.inflight,l.Project)
    close(flight.done)
    projectionCache.Unlock()
    if err!=nil{return nil,err}
    return filterProjectedEvents(all,taskID),nil
}

func filterProjectedEvents(all []Event, taskID string) []Event {
    if taskID==""{return all}
    out:=make([]Event,0)
    for _,e:=range all{if e.TaskID==taskID{out=append(out,e)}}
    return out
}
func invalidateProjection(project string) {
    projectionCache.Lock()
    projectionCache.revision++
    delete(projectionCache.entries,project)
    projectionCache.Unlock()
}
