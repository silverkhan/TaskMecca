package backlog

import (
    "os/exec"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "time"
)

type lifecycleEvent struct {
    State string
    At string
    Source string
}

func gitOutput(repo string,args ...string) string {
    cmd:=exec.Command("git",args...)
    cmd.Dir=repo
    out,err:=cmd.Output()
    if err!=nil { return "" }
    return string(out)
}

func repoRoot(path string) string {
    probe:=path
    if probe=="" { probe="." }
    out:=strings.TrimSpace(gitOutput(probe,"rev-parse","--show-toplevel"))
    if out!="" { return out }
    return filepath.Dir(probe)
}

func formatDuration(seconds any) string {
    if seconds==nil { return "-" }
    var value float64
    switch v:=seconds.(type) {
    case float64: value=v
    case int: value=float64(v)
    case int64: value=float64(v)
    default: return "-"
    }
    if value<0 { return "-" }
    total:=int64(value)
    minutes:=total/60
    sec:=total%60
    if minutes<60 {
        if minutes>0 { return strconv.FormatInt(minutes,10)+"m "+pad2(sec)+"s" }
        return strconv.FormatInt(sec,10)+"s"
    }
    hours:=minutes/60
    minutes%=60
    if hours<24 {
        return strconv.FormatInt(hours,10)+"h "+pad2(minutes)+"m "+pad2(sec)+"s"
    }
    days:=hours/24
    hours%=24
    return strconv.FormatInt(days,10)+"d "+pad2(hours)+"h "+pad2(minutes)+"m "+pad2(sec)+"s"
}

func pad2(v int64) string {
    if v<10 { return "0"+strconv.FormatInt(v,10) }
    return strconv.FormatInt(v,10)
}

func parseTime(value string) (time.Time,bool) {
    parsed,err:=time.Parse(time.RFC3339,value)
    if err==nil { return parsed,true }
    parsed,err=time.Parse(time.RFC3339Nano,value)
    return parsed,err==nil
}

func LifecycleTimings(project,root string) (map[string]map[string]any,error) {
    ledger,err:=Select(project,root)
    if err!=nil { return nil,err }
    if ledger=="" {
        if root!="" { ledger=root } else { ledger=filepath.Join(project,"_task_mecca","data","backlog") }
    }
    repo:=repoRoot(ledger)
    rel,relErr:=filepath.Rel(repo,ledger)
    if relErr!=nil { rel=filepath.Base(ledger) }

    args:=[]string{"log","--reverse","-M","--format=@@COLLAB@@%cI","--name-status","--",rel}
    out:=gitOutput(repo,args...)
    events:=map[string][]lifecycleEvent{}
    lastState:=map[string]string{}
    stamp:=""
    for _,line:=range strings.Split(out,"\n") {
        if strings.HasPrefix(line,"@@COLLAB@@") {
            stamp=strings.TrimSpace(strings.TrimPrefix(line,"@@COLLAB@@"))
            continue
        }
        if stamp=="" || strings.TrimSpace(line)=="" { continue }
        parts:=strings.Split(line,"\t")
        if len(parts)<2 { continue }
        status:=parts[0]
        pathText:=""
        if (strings.HasPrefix(status,"R")||strings.HasPrefix(status,"C")) && len(parts)>=3 {
            pathText=parts[2]
        } else if (strings.HasPrefix(status,"A")||strings.HasPrefix(status,"M")) && len(parts)>=2 {
            pathText=parts[1]
        } else {
            continue
        }
        match:=itemName.FindStringSubmatch(filepath.Base(pathText))
        if match==nil { continue }
        id:=strings.ToUpper(match[2])
        state:=match[4]
        if lastState[id]==state { continue }
        events[id]=append(events[id],lifecycleEvent{State:state,At:stamp,Source:"git"})
        lastState[id]=state
    }

    now:=time.Now()
    result:=map[string]map[string]any{}
    labels:=map[string]string{"todo":"Registered","doing":"Started","hold":"Hold","done":"Completed"}
    for id,seq:=range events {
        sort.SliceStable(seq,func(i,j int)bool {
            ti,_:=parseTime(seq[i].At); tj,_:=parseTime(seq[j].At); return ti.Before(tj)
        })
        if len(seq)==0 { continue }
        type parsedEvent struct{ event lifecycleEvent; at time.Time }
        parsed:=[]parsedEvent{}
        for _,entry:=range seq {
            if dt,ok:=parseTime(entry.At); ok { parsed=append(parsed,parsedEvent{entry,dt}) }
        }
        if len(parsed)==0 { continue }
        createdAt:=parsed[0].event.At
        firstDoing:=""
        latestDoing:=""
        completedAt:=""
        hasDoing:=false
        hasHold:=false
        activeSeconds:=0.0
        waitSeconds:=0.0
        eventRows:=[]map[string]any{}
        for i,entry:=range parsed {
            if entry.event.State=="doing" {
                hasDoing=true
                if firstDoing=="" { firstDoing=entry.event.At }
                latestDoing=entry.event.At
            }
            if entry.event.State=="hold" { hasHold=true }
            if entry.event.State=="done" { completedAt=entry.event.At }
            next:=now
            if i+1<len(parsed) { next=parsed[i+1].at }
            interval:=next.Sub(entry.at).Seconds()
            if interval<0 { interval=0 }
            if entry.event.State=="done" { interval=0 }
            if entry.event.State=="doing" { activeSeconds+=interval }
            if entry.event.State=="hold" { waitSeconds+=interval }
            intervalValue:="-"
            if interval>0 { intervalValue=formatDuration(interval) }
            eventRows=append(eventRows,map[string]any{
                "state":entry.event.State,
                "label":labels[entry.event.State],
                "at":entry.event.At,
                "interval_seconds":interval,
                "interval":intervalValue,
                "source":entry.event.Source,
                "provisional":false,
            })
        }
        current:=parsed[len(parsed)-1]
        currentSegment:=0.0
        if current.event.State!="done" {
            currentSegment=now.Sub(current.at).Seconds()
            if currentSegment<0 { currentSegment=0 }
        }
        created:=parsed[0].at
        var started *time.Time
        for _,entry:=range parsed { if entry.event.State=="doing" { v:=entry.at; started=&v; break } }
        var completed *time.Time
        for i:=len(parsed)-1;i>=0;i-- { if parsed[i].event.State=="done" { v:=parsed[i].at; completed=&v; break } }
        var queueSeconds any
        if started!=nil {
            q:=started.Sub(created).Seconds(); if q<0 { q=0 }; queueSeconds=q
        } else if completed==nil {
            q:=now.Sub(created).Seconds(); if q<0 { q=0 }; queueSeconds=q
        } else {
            queueSeconds=nil
        }
        leadEnd:=now
        if completed!=nil { leadEnd=*completed }
        leadSeconds:=leadEnd.Sub(created).Seconds(); if leadSeconds<0 { leadSeconds=0 }
        var activeValue any
        if hasDoing { activeValue=activeSeconds }
        var waitValue any
        if hasHold || hasDoing || current.event.State=="todo" { waitValue=waitSeconds }
        var elapsed any
        if current.event.State=="doing" { elapsed=currentSegment }
        var duration any
        if current.event.State=="done" { duration=activeValue }

        result[id]=map[string]any{
            "current_state":current.event.State,
            "current_state_at":current.event.At,
            "current_state_source":current.event.Source,
            "current_segment_seconds":currentSegment,
            "created_at":createdAt,
            "started_at":nilIfEmpty(firstDoing),
            "claimed_at":nilIfEmpty(latestDoing),
            "completed_at":nilIfEmpty(completedAt),
            "queue_seconds":queueSeconds,
            "active_seconds":activeValue,
            "wait_seconds":waitValue,
            "work_seconds":activeValue,
            "lead_seconds":leadSeconds,
            "elapsed_seconds":elapsed,
            "duration_seconds":duration,
            "queue":formatDuration(queueSeconds),
            "active":formatDuration(activeValue),
            "wait":formatDuration(waitValue),
            "work":formatDuration(activeValue),
            "lead":formatDuration(leadSeconds),
            "elapsed":formatDuration(elapsed),
            "duration":func()string{ if current.event.State=="done" { return formatDuration(activeValue) }; return "-" }(),
            "events":eventRows,
            "lifecycle_inferred":false,
            "lifecycle_inference_note":"",
            "timing_incomplete":completed!=nil && !hasDoing,
            "timing_incomplete_note":func()string{
                if completed!=nil && !hasDoing {
                    return "No doing transition was committed or observed for this completed task, so Active/Queue timing is unknown rather than zero."
                }
                return ""
            }(),
        }
    }
    return result,nil
}

func nilIfEmpty(value string) any {
    if value=="" { return nil }
    return value
}
