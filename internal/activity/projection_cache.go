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
var projectionCache struct {
    sync.Mutex
    entries map[string]cachedProjection
}

// ListCached is for READ-ONLY web projections. Mutations always use Ledger.List
// beneath the canonical write guard. Data from other processes is refreshed
// within a bounded TTL; same-process writes invalidate it immediately.
// Large projects are deliberately not cached to cap retained memory.
func(l Ledger) ListCached(taskID string)([]Event,error){
    now:=time.Now()
    projectionCache.Lock()
    if entry,ok:=projectionCache.entries[l.Project];ok&&now.Sub(entry.at)<projectionTTL{
        out:=filterProjectedEvents(entry.events,taskID)
        projectionCache.Unlock()
        return out,nil
    }
    projectionCache.Unlock()

    all,err:=l.List("")
    if err!=nil{return nil,err}
    if len(all)<=projectionMaxEvents{
        projectionCache.Lock()
        if projectionCache.entries==nil{projectionCache.entries=map[string]cachedProjection{}}
        if len(projectionCache.entries)>=projectionMaxProjects{
            var oldestKey string
            var oldestTime time.Time
            for key,value:=range projectionCache.entries{
                if oldestKey==""||value.at.Before(oldestTime){oldestKey,oldestTime=key,value.at}
            }
            delete(projectionCache.entries,oldestKey)
        }
        projectionCache.entries[l.Project]=cachedProjection{at:now,events:all}
        projectionCache.Unlock()
    }
    return filterProjectedEvents(all,taskID),nil
}

func filterProjectedEvents(all []Event, taskID string)[]Event {
    if taskID==""{return all}
    out:=make([]Event,0)
    for _,e:=range all{if e.TaskID==taskID{out=append(out,e)}}
    return out
}
func invalidateProjection(project string){
    projectionCache.Lock()
    delete(projectionCache.entries,project)
    projectionCache.Unlock()
}
