package backlog

import (
    "encoding/json"
    "os"
    "os/exec"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type lifecycleEvent struct {
    State string
    At string
    Source string
}

type lifecycleGitCacheEntry struct {
    head string
    output string
}

var lifecycleGitCache = struct {
    sync.Mutex
    entries map[string]lifecycleGitCacheEntry
}{entries: map[string]lifecycleGitCacheEntry{}}

func lifecycleGitLog(repo string,pathspecs []string) string {
    head:=strings.TrimSpace(gitOutput(repo,"rev-parse","HEAD"))
    key:=repo+"\x00"+strings.Join(pathspecs,"\x00")
    if head!="" {
        lifecycleGitCache.Lock()
        cached,ok:=lifecycleGitCache.entries[key]
        lifecycleGitCache.Unlock()
        if ok && cached.head==head { return cached.output }
    }
    args:=[]string{"log","--reverse","-M","--format=@@COLLAB@@%cI","--name-status","--"}
    args=append(args,pathspecs...)
    out:=gitOutput(repo,args...)
    if head!="" {
        lifecycleGitCache.Lock()
        lifecycleGitCache.entries[key]=lifecycleGitCacheEntry{head:head,output:out}
        lifecycleGitCache.Unlock()
    }
    return out
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
    if absolute,err:=filepath.Abs(probe); err==nil { probe=absolute }
    for {
        if _,err:=os.Stat(probe); err==nil { break }
        parent:=filepath.Dir(probe)
        if parent==probe { break }
        probe=parent
    }
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
    return lifecycleTimings(project,root,nil)
}

func lifecycleTimings(project,root string,rows []Record) (map[string]map[string]any,error) {
    ledger,err:=Select(project,root)
    if err!=nil { return nil,err }
    if ledger=="" {
        if root!="" { ledger=root } else { ledger=filepath.Join(project,"_task_mecca","data","backlog") }
    }
    repo:=repoRoot(ledger)
    rel,relErr:=filepath.Rel(repo,ledger)
    if relErr!=nil { rel=filepath.Base(ledger) }

    pathspecs:=[]string{rel}
    dataDir:=filepath.Join(project,"_task_mecca","data")
    if cleanLedger,cleanData:=filepath.Clean(ledger),filepath.Clean(dataDir); filepath.Dir(cleanLedger)==cleanData {
        legacy:=filepath.Join(project,"_task_mecca",filepath.Base(cleanLedger))
        if legacyRel,legacyErr:=filepath.Rel(repo,legacy); legacyErr==nil && legacyRel!=rel {
            pathspecs=append(pathspecs,legacyRel)
        }
    }
    out:=lifecycleGitLog(repo,pathspecs)
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

    if rows==nil {
        rows,err=Catalog(project,root)
        if err!=nil { return nil,err }
    }
    currentRows:=map[string]Record{}
    for _,row:=range rows {
        existing,ok:=currentRows[row.ID]
        if !ok || (row.Location=="active" && existing.Location!="active") { currentRows[row.ID]=row }
    }

    journalPath:=filepath.Join(project,"_task_mecca",".runtime","lifecycle_observations.json")
    journal:=map[string]any{"version":float64(1),"ledgers":map[string]any{}}
    if data,readErr:=os.ReadFile(journalPath); readErr==nil {
        _=json.Unmarshal(data,&journal)
    }
    ledgers,ok:=journal["ledgers"].(map[string]any)
    if !ok { ledgers=map[string]any{}; journal["ledgers"]=ledgers }
    absLedger,_:=filepath.Abs(ledger)
    ledgerRow,ok:=ledgers[absLedger].(map[string]any)
    if !ok { ledgerRow=map[string]any{"items":map[string]any{}}; ledgers[absLedger]=ledgerRow }
    items,ok:=ledgerRow["items"].(map[string]any)
    if !ok { items=map[string]any{}; ledgerRow["items"]=items }

    now:=time.Now()

    // Canonical execution-start evidence comes from the provider-neutral
    // Execution Attempt Ledger. A backlog "doing" state is dispatch/workflow
    // state; it is not, by itself, proof that the assigned worker executed.
    runtimeStarts:=map[string]lifecycleEvent{}
    if runtimeLedger,ledgerErr:=runtimeobs.ReconcileLedger(project,10,now); ledgerErr==nil {
        for _,attempt:=range runtimeLedger.Attempts {
            id:=strings.ToUpper(strings.TrimSpace(attempt.TaskID))
            if id=="" || attempt.BindingState!=runtimeobs.BindingBound || attempt.StartedAt=="" { continue }

            // A binding can be written after an attempt has already existed for
            // some time. Never backdate the task lifecycle to runtime activity
            // that predates that task binding. The effective start is the later
            // of runtime start and binding time.
            effectiveStart:=attempt.StartedAt
            for _,transition:=range attempt.RecentTransitions {
                if transition.Kind!="binding" { continue }
                if bindingAt,ok:=parseTime(transition.At); ok {
                    if startAt,startOK:=parseTime(effectiveStart); !startOK || bindingAt.After(startAt) {
                        effectiveStart=transition.At
                    }
                }
            }

            // Registration is an invariant boundary: a task cannot start before
            // it exists. Reject stale/reused attempt evidence that would invert
            // Registered -> Started ordering.
            registeredAt:=""
            if durable:=events[id]; len(durable)>0 {
                for _,event:=range durable {
                    if event.State=="todo" { registeredAt=event.At; break }
                }
            }
            if registeredAt=="" {
                if row,ok:=currentRows[id]; ok { registeredAt=firstNonEmpty(row.Ctime,row.Mtime) }
            }
            if regAt,regOK:=parseTime(registeredAt); regOK {
                startAt,startOK:=parseTime(effectiveStart)
                if !startOK || startAt.Before(regAt) { continue }
            }

            candidate:=lifecycleEvent{State:"doing",At:effectiveStart,Source:"execution_ledger"}
            existing,ok:=runtimeStarts[id]
            if !ok {
                runtimeStarts[id]=candidate
                continue
            }
            oldAt,oldOK:=parseTime(existing.At); newAt,newOK:=parseTime(candidate.At)
            if newOK && (!oldOK || newAt.Before(oldAt)) { runtimeStarts[id]=candidate }
        }
    }

    // If the task explicitly targets a provider whose Task Mecca hook is
    // installed, absence of runtime start evidence must remain "not started".
    // This prevents a mere todo -> doing file move from becoming a false start.
    runtimeExpected:=map[string]bool{}
    hookInstalled:=map[string]bool{}
    for _,provider:=range []string{"codex","claude"} {
        if setup,hookErr:=runtimeobs.HookStatus(project,provider); hookErr==nil { hookInstalled[provider]=setup.Installed }
    }
    anyHookInstalled:=hookInstalled["codex"] || hookInstalled["claude"]
    for id,row:=range currentRows {
        metadata:=runtimeFromFields(row.Fields)
        provider:=strings.ToLower(strings.TrimSpace(toString(metadata["runtime_provider"])))
        assigned:=strings.TrimSpace(row.Fields["Agent"])!=""
        if (provider!="" && hookInstalled[provider]) || (provider=="" && assigned && anyHookInstalled) {
            runtimeExpected[id]=true
        }
    }

    journalChanged:=false
    combinedEvents:=map[string][]lifecycleEvent{}
    for id,seq:=range events { combinedEvents[id]=append([]lifecycleEvent{},seq...) }
    for id,row:=range currentRows {
        durable:=events[id]
        durableLastState:=""
        var durableLastTime time.Time
        hasDurableLast:=false
        if len(durable)>0 {
            durableLastState=durable[len(durable)-1].State
            if parsed,ok:=parseTime(durable[len(durable)-1].At); ok { durableLastTime=parsed; hasDurableLast=true }
        }

        cached:=[]map[string]string{}
        cacheFiltered:=false
        if raw,ok:=items[id].([]any); ok {
            for _,entryRaw:=range raw {
                entry,ok:=entryRaw.(map[string]any); if !ok { cacheFiltered=true; continue }
                state,_:=entry["state"].(string); at,_:=entry["at"].(string)
                dt,valid:=parseTime(at)
                if !valid || (state!="todo"&&state!="doing"&&state!="hold"&&state!="done") { cacheFiltered=true; continue }
                // Once a durable completion exists, no earlier-state observation
                // at or after that completion can be part of the canonical
                // history. Drop only those impossible tail entries; observations
                // before completion remain valuable historical evidence.
                if row.State=="done" && durableLastState=="done" && hasDurableLast && !dt.Before(durableLastTime) && state!="done" {
                    cacheFiltered=true
                    continue
                }
                // A provisional observation is historical evidence once seen.
                // Do not discard it merely because Git later records a newer
                // terminal/current state; Git may never contain the earlier
                // registration/start transition that this observation proves.
                cached=append(cached,map[string]string{"state":state,"at":at})
            }
        }
        if cacheFiltered {
            raw:=[]any{}
            for _,entry:=range cached { raw=append(raw,map[string]any{"state":entry["state"],"at":entry["at"]}) }
            items[id]=raw
            journalChanged=true
        }

        // Determine the latest state by timestamp across durable Git evidence
        // and retained observations. Using cached[len-1] alone is unsafe after
        // Git catches up because retained history can be older than Git.
        known:=durableLastState
        previous:=durableLastTime
        hasPrevious:=hasDurableLast
        for _,entry:=range cached {
            if p,ok:=parseTime(entry["at"]); ok && (!hasPrevious || p.After(previous)) {
                known=entry["state"]
                previous=p
                hasPrevious=true
            }
        }
        if durableLastState!=row.State && known!=row.State {
            observedRaw:=row.Ctime
            if observedRaw=="" { observedRaw=row.Mtime }
            observed,valid:=parseTime(observedRaw)
            if !valid { observed=now }
            if hasPrevious && observed.Before(previous) { observed=now }
            cached=append(cached,map[string]string{
                "state":row.State,
                "at":observed.Format(time.RFC3339),
            })
            raw:=[]any{}
            for _,entry:=range cached { raw=append(raw,map[string]any{"state":entry["state"],"at":entry["at"]}) }
            items[id]=raw
            journalChanged=true
        }
        combined:=append([]lifecycleEvent{},durable...)

        // For active work with observable runtime, remove workflow-state doing
        // transitions and replace them with the first bound runtime start.
        // Completed/history rows retain Git evidence for backward compatibility.
        if row.Location=="active" && runtimeExpected[id] {
            filtered:=make([]lifecycleEvent,0,len(combined))
            for _,event:=range combined { if event.State!="doing" { filtered=append(filtered,event) } }
            combined=filtered
            filteredCached:=cached[:0]
            for _,entry:=range cached { if entry["state"]!="doing" { filteredCached=append(filteredCached,entry) } }
            cached=filteredCached
            if start,ok:=runtimeStarts[id]; ok { combined=append(combined,start) }
        } else if start,ok:=runtimeStarts[id]; ok {
            // Runtime evidence outranks any inferred doing timestamp even when
            // a durable Git transition also exists.
            filtered:=make([]lifecycleEvent,0,len(combined))
            for _,event:=range combined { if event.State!="doing" { filtered=append(filtered,event) } }
            combined=append(filtered,start)
            filteredCached:=cached[:0]
            for _,entry:=range cached { if entry["state"]!="doing" { filteredCached=append(filteredCached,entry) } }
            cached=filteredCached
        }

        for _,entry:=range cached {
            combined=append(combined,lifecycleEvent{State:entry["state"],At:entry["at"],Source:"runtime_observed"})
        }
        combinedEvents[id]=combined
    }
    if journalChanged {
        if err:=os.MkdirAll(filepath.Dir(journalPath),0755); err==nil {
            if data,err:=json.MarshalIndent(journal,"","  "); err==nil {
                _=os.WriteFile(journalPath,append(data,'\n'),0644)
            }
        }
    }

    result:=map[string]map[string]any{}
    labels:=map[string]string{"todo":"Registered","doing":"Started","hold":"Hold","done":"Completed"}
    for id,seq:=range combinedEvents {
        sort.SliceStable(seq,func(i,j int)bool {
            ti,_:=parseTime(seq[i].At); tj,_:=parseTime(seq[j].At); return ti.Before(tj)
        })
        seq=collapseLifecycleEvents(seq)
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
                "provisional":entry.event.Source!="git",
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
            "lifecycle_inferred":func()bool{
                for _,event:=range eventRows { if value,ok:=event["provisional"].(bool); ok && value { return true } }
                return false
            }(),
            "lifecycle_inference_note":func()string{
                for _,event:=range eventRows { if value,ok:=event["provisional"].(bool); ok && value {
                    return "Current state includes a provisional runtime observation that is retained across refreshes until Git records the transition."
                } }
                return ""
            }(),
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

func lifecycleSourceRank(source string) int {
    switch source {
    case "execution_ledger": return 30
    case "git": return 20
    case "runtime_observed": return 10
    default: return 0
    }
}

func collapseLifecycleEvents(seq []lifecycleEvent) []lifecycleEvent {
    if len(seq)==0 { return seq }
    out:=make([]lifecycleEvent,0,len(seq))
    for _,event:=range seq {
        if len(out)>0 && out[len(out)-1].State==event.State {
            // The same state observed twice without an intervening state is one
            // transition. Prefer the stronger source, preserving runtime start
            // over Git doing and Git over provisional filesystem observation.
            if lifecycleSourceRank(event.Source)>lifecycleSourceRank(out[len(out)-1].Source) {
                out[len(out)-1]=event
            }
            continue
        }
        out=append(out,event)
    }
    return out
}

func nilIfEmpty(value string) any {
    if value=="" { return nil }
    return value
}
