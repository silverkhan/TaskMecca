package webui

import (
    "sync"
    "time"

    "github.com/silverkhan/TaskMecca/internal/activity"
    "github.com/silverkhan/TaskMecca/internal/backlog"
)

type activityLifecycleEvidence struct {
    at time.Time
    byTask map[string][]activity.LifecycleFact
    findings int
}
const activityLifecycleTTL=3*time.Second
const activityLifecycleMaxProjects=4
var activityLifeCache=struct{
    sync.Mutex
    projects map[string]activityLifecycleEvidence
}{projects:map[string]activityLifecycleEvidence{}}

// Lifecycle facts are loaded only for the separately requested Activity panel,
// not on the critical backlog-detail HTML path. Repeated panel reads within the
// TTL avoid reopening every lifecycle event file.
func readActivityLifecycle(project,task string)([]activity.LifecycleFact,int,error){
    now:=time.Now()
    activityLifeCache.Lock()
    if item,ok:=activityLifeCache.projects[project];ok&&now.Sub(item.at)<activityLifecycleTTL{
        activityLifeCache.Unlock()
        return item.byTask[task],item.findings,nil
    }
    activityLifeCache.Unlock()
    scan,err:=backlog.ReadLifecycleTransitions(project)
    if err!=nil{return nil,0,err}
    byTask:=map[string][]activity.LifecycleFact{}
    for _,e:=range scan.Events{
        when,parseErr:=time.Parse(time.RFC3339Nano,e.OccurredAt)
        if parseErr!=nil{continue}
        byTask[e.TaskID]=append(byTask[e.TaskID],activity.LifecycleFact{
            EventID:e.EventID,TaskID:e.TaskID,Kind:e.Kind,OccurredAt:when,EvidenceSource:e.EvidenceSource,
        })
    }
    snapshot:=activityLifecycleEvidence{at:now,byTask:byTask,findings:len(scan.Findings)}
    activityLifeCache.Lock()
    if len(activityLifeCache.projects)>=activityLifecycleMaxProjects{
        var victim string
        var oldest time.Time
        for id,item:=range activityLifeCache.projects{
            if victim==""||item.at.Before(oldest){victim,oldest=id,item.at}
        }
        delete(activityLifeCache.projects,victim)
    }
    activityLifeCache.projects[project]=snapshot
    activityLifeCache.Unlock()
    return byTask[task],snapshot.findings,nil
}
