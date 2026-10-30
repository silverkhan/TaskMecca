package backlog

import (
    "github.com/silverkhan/TaskMecca/internal/projectguard"
    "encoding/json"
    "crypto/sha256"
    "encoding/hex"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
    "github.com/silverkhan/TaskMecca/internal/handoffjournal"
)

type lifecycleEvent struct {
    State string
    At string
    Source string
	ID     string
	Kind   string
	Actor  string
	AssignedWorker string
	EvidenceSource string
	AssignmentID string
	AttemptID string
}

type lifecycleGitCacheEntry struct {
    head string
    output string
}

var lifecycleGitCache = struct {
    sync.Mutex
    entries map[string]lifecycleGitCacheEntry
}{entries: map[string]lifecycleGitCacheEntry{}}

// Keep the full Git rename/lifecycle evidence; cache the expensive historical
// log across Web process restarts, keyed to the last Git commit touching the backlog paths and
// pathspecs. No canonical evidence or notification journal is modified.
const lifecycleLogCacheMax = 64 << 20

type persistedLifecycleGitLog struct {
    Head string `json:"head"`
    Output string `json:"output"`
}

func lifecycleGitLogCachePath(repo string,pathspecs []string) (string,error) {
    directory,err:=os.UserCacheDir()
    if err!=nil {return "",err}
    digest:=sha256.Sum256([]byte(repo+"\\x00"+strings.Join(pathspecs,"\\x00")))
    return filepath.Join(directory,"task-mecca","lifecycle-git",hex.EncodeToString(digest[:])+".json"),nil
}

func loadPersistedLifecycleGitLog(path,head string) (string,bool) {
    info,err:=os.Lstat(path)
    if err!=nil||!info.Mode().IsRegular()||info.Size()<1||info.Size()>lifecycleLogCacheMax {return "",false}
    file,err:=os.Open(path)
    if err!=nil{return "",false}
    defer file.Close()
    var cached persistedLifecycleGitLog
    decoder:=json.NewDecoder(io.LimitReader(file,lifecycleLogCacheMax+1))
    if decoder.Decode(&cached)!=nil||cached.Head!=head{return "",false}
    return cached.Output,true
}

func persistLifecycleGitLog(path,head,output string) {
    if len(output)==0||len(output)>lifecycleLogCacheMax-1024{return}
    directory:=filepath.Dir(path)
    if err:=os.MkdirAll(directory,0700);err!=nil{return}
    if info,err:=os.Lstat(path);err==nil&&(!info.Mode().IsRegular()){return}
    data,err:=json.Marshal(persistedLifecycleGitLog{Head:head,Output:output})
    if err!=nil||len(data)>lifecycleLogCacheMax{return}
    temp,err:=os.CreateTemp(directory,".lifecycle-")
    if err!=nil{return}
    defer os.Remove(temp.Name())
    if err=temp.Chmod(0600);err!=nil{temp.Close();return}
    if _,err=temp.Write(data);err!=nil{temp.Close();return}
    if err=temp.Sync();err!=nil{temp.Close();return}
    if err=temp.Close();err!=nil{return}
    _=os.Rename(temp.Name(),path)
}

func lifecycleGitLog(repo string,pathspecs []string) string {
    // The latest commit touching either backlog path is a stronger cache
    // identity than repository HEAD: Dev/PR commits elsewhere do not force a
    // 400+ backlog Git history replay, while path updates still invalidate.
    revisionArgs:=append([]string{"log","-1","--format=%H","--"},pathspecs...)
    head:=strings.TrimSpace(gitOutput(repo,revisionArgs...))
    key:=repo+"\\x00"+strings.Join(pathspecs,"\\x00")
    if head!="" {
        lifecycleGitCache.Lock()
        cached,ok:=lifecycleGitCache.entries[key]
        lifecycleGitCache.Unlock()
        if ok && cached.head==head {return cached.output}
        // Loading a persisted cache avoids running git log -M across the
        // entire historical backlog after each Web/update restart or unrelated PR.
        if path,err:=lifecycleGitLogCachePath(repo,pathspecs);err==nil{
            if out,ok:=loadPersistedLifecycleGitLog(path,head);ok{
                lifecycleGitCache.Lock()
                lifecycleGitCache.entries[key]=lifecycleGitCacheEntry{head:head,output:out}
                lifecycleGitCache.Unlock()
                return out
            }
        }
    }
    args:=[]string{"log","--reverse","-M","--format=@@COLLAB@@%cI","--name-status","--"}
    args=append(args,pathspecs...)
    out:=gitOutput(repo,args...)
    if head!="" && out!="" {
        lifecycleGitCache.Lock()
        lifecycleGitCache.entries[key]=lifecycleGitCacheEntry{head:head,output:out}
        lifecycleGitCache.Unlock()
        if path,err:=lifecycleGitLogCachePath(repo,pathspecs);err==nil{
            persistLifecycleGitLog(path,head,out)
        }
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

func observedLifecycleAt(items map[string]any,id,state string) string {
    raw,ok:=items[id].([]any)
    if !ok { return "" }
    best:=""
    var bestTime time.Time
    for _,entryRaw:=range raw {
        entry,ok:=entryRaw.(map[string]any)
        if !ok || toString(entry["state"])!=state { continue }
        at:=toString(entry["at"])
        parsed,valid:=parseTime(at)
        if !valid { continue }
        if best=="" || parsed.Before(bestTime) {
            best=at
            bestTime=parsed
        }
    }
    return best
}

func LifecycleTimings(project,root string) (map[string]map[string]any,error) {
    return lifecycleTimings(project,root,nil)
}

func lifecycleTimings(project,root string,rows []Record,shared ...requestRuntime) (map[string]map[string]any,error) {
    releaseGuard, guardErr := projectguard.AcquireWrite(project)
    if guardErr != nil { return nil, guardErr }
    defer releaseGuard()

	durableScan,scanErr:=ReadLifecycleTransitions(project)
	if scanErr!=nil { return nil,scanErr }
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
    if !ok { ledgers=map[string]any{}
		journal["ledgers"]=ledgers }
    absLedger,_:=filepath.Abs(ledger)
    ledgerRow,ok:=ledgers[absLedger].(map[string]any)
    if !ok { ledgerRow=map[string]any{"items":map[string]any{}}
		ledgers[absLedger]=ledgerRow }
    items,ok:=ledgerRow["items"].(map[string]any)
    if !ok { items=map[string]any{}
		ledgerRow["items"]=items }

    now:=time.Now()

    // Canonical execution-start evidence comes from the provider-neutral
    // Execution Attempt Ledger. A backlog "doing" state is dispatch/workflow
    // state; it is not, by itself, proof that the assigned worker executed.
    runtimeStarts:=map[string]lifecycleEvent{}
    if runtimeLedger,ledgerErr:=readRequestRuntime(project,10,now,shared...); ledgerErr==nil {
        for _,attempt:=range runtimeLedger.Attempts {
            id:=strings.ToUpper(strings.TrimSpace(attempt.TaskID))
            if id=="" || attempt.BindingState!=runtimeobs.BindingBound || attempt.StartedAt=="" { continue }

            // A binding can be written after an attempt has already existed for
            // some time. Never backdate the task lifecycle to runtime activity
            // that predates that task binding. BindingAt is a canonical folded
            // field and does not disappear when RecentTransitions is truncated.
            effectiveStart:=attempt.StartedAt
            if bindingAt,ok:=parseTime(attempt.BindingAt); ok {
                if startAt,startOK:=parseTime(effectiveStart); !startOK || bindingAt.After(startAt) {
                    effectiveStart=attempt.BindingAt
                }
            }

            // Registration is an invariant boundary: a task cannot start before
            // it exists. Prefer the immutable/provisional registration observation
            // already captured in lifecycle_observations.json. Using the current
            // file ctime here is unsafe because todo -> doing rename/edit advances
            // ctime and can make a valid runtime start look older than registration.
            registeredAt:=""
            if durable:=events[id]; len(durable)>0 {
                for _,event:=range durable {
                    if event.State=="todo" { registeredAt=event.At
						break }
                }
            }
            if registeredAt=="" {
                registeredAt=observedLifecycleAt(items,id,"todo")
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
            oldAt,oldOK:=parseTime(existing.At)
			newAt,newOK:=parseTime(candidate.At)
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
            if parsed,ok:=parseTime(durable[len(durable)-1].At); ok { durableLastTime=parsed
				hasDurableLast=true }
        }

        cached:=[]map[string]string{}
        cacheFiltered:=false
        if raw,ok:=items[id].([]any); ok {
            for _,entryRaw:=range raw {
                entry,ok:=entryRaw.(map[string]any)
				if !ok { cacheFiltered=true
					continue }
                state,_:=entry["state"].(string)
				at,_:=entry["at"].(string)
                dt,valid:=parseTime(at)
                if !valid || (state!="todo"&&state!="doing"&&state!="hold"&&state!="done") { cacheFiltered=true
					continue }
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

	// Explicit role reports are the canonical source. Git and the legacy
	// observation journal remain readable fallback evidence, never a reason to
	// duplicate a phase or promote a late file observation to a real start.
	directByID:=map[string][]LifecycleTransition{}
	needsAssignmentLookup:=false
	for _,event:=range durableScan.Events {
        directByID[event.TaskID]=append(directByID[event.TaskID],event)
        if event.AssignmentID!="" {needsAssignmentLookup=true}
    }
    // Event Actor is the recorded actor, not necessarily the assigned Worker.
    // Resolve only exact TaskID + assignment ID; never substitute today's Agent.
    historicalAssigned:=map[string]string{}
    ambiguousAssignment:=map[string]bool{}
    if needsAssignmentLookup {
        if assignments,assignmentErr:=runtimeobs.ListAssignments(project);assignmentErr==nil {
            for _,a:=range assignments {
                key:=strings.ToUpper(a.TaskID)+"\x00"+a.AssignmentID
                if existing,ok:=historicalAssigned[key];ok && existing!=a.AgentPath {
                    historicalAssigned[key]=""
                    ambiguousAssignment[key]=true
                }else if !ok {historicalAssigned[key]=a.AgentPath}
            }
        }
    }
	for id,direct:=range directByID {
		stateFor:=func(kind string) string { switch kind { case "registered","assigned":return "todo"; case "started","resumed":return "doing"; case "waiting":return "hold"; case "completed":return "done" }; return "" }
		directState:=map[string]bool{}
		hasStart:=false
		hasAssignment:=false
		for _,event:=range direct { directState[stateFor(event.Kind)]=true; if event.Kind=="started" { hasStart=true }; if event.Kind=="assigned" { hasAssignment=true } }
		filtered:=[]lifecycleEvent{}
		for _,old:=range combinedEvents[id] {
			if directState[old.State] { continue }
			if hasAssignment && !hasStart && old.State=="doing" { continue }
			filtered=append(filtered,old)
		}
		for _,event:=range direct {
			workerKey:=event.TaskID+"\x00"+event.AssignmentID
            worker:=""
            if event.AssignmentID!="" && !ambiguousAssignment[workerKey] { worker=historicalAssigned[workerKey] }
            filtered=append(filtered,lifecycleEvent{State:stateFor(event.Kind),At:event.OccurredAt,Source:"durable_lifecycle",ID:event.EventID,Kind:event.Kind,Actor:event.Actor,AssignedWorker:worker,EvidenceSource:event.EvidenceSource,AssignmentID:event.AssignmentID,AttemptID:event.AttemptID})
		}
		combinedEvents[id]=filtered
	}
	if handoffs, handoffErr := handoffjournal.BuildLedger(project, now); handoffErr == nil && len(handoffs.Findings) == 0 {
		attempts, attemptErr := readRequestRuntime(project, 20, now, shared...)
		assignments, assignmentErr := runtimeobs.ListAssignments(project)
		if attemptErr == nil && assignmentErr == nil {
			for _, h := range handoffs.Handoffs {
				if h.EventType != handoffjournal.EventWorkerDone || h.Target.AgentPath != "/root/controller" || h.ClaimedBy != h.Target.AgentPath || h.Target.AttemptID == "" || h.ClaimedAttemptID != h.Target.AttemptID || h.Target.RuntimeAgentID == "" || h.Target.SessionID == "" || h.Target.Provider == "" || h.Applied { continue }
				if _, accepted := h.Steps["acceptance"]; accepted { continue }
				claimedAt, valid := parseTime(h.ClaimedAt); if !valid || claimedAt.After(now) { continue }
				id := strings.ToUpper(h.TaskID); row, exists := currentRows[id]; if !exists || row.Location != "active" || row.State != "doing" { continue }
				assignmentID, matches := "", 0
				for _, a := range assignments {
					if !strings.EqualFold(a.TaskID, id) || a.AgentPath != h.SourceAgentPath { continue }
					for _, attempt := range attempts.Attempts { if attempt.AttemptID == h.SourceAttemptID && attempt.BindingState == runtimeobs.BindingBound && strings.EqualFold(attempt.TaskID, id) && attempt.AgentPath == h.SourceAgentPath && attempt.BindingEvidence["assignment_id"] == a.AssignmentID { assignmentID, matches = a.AssignmentID, matches+1 } }
				}
				if matches != 1 { continue }
				live := 0
				for _, attempt := range attempts.Attempts {
					if attempt.AttemptID != h.Target.AttemptID || attempt.AgentPath != h.Target.AgentPath || attempt.RuntimeAgentID != h.Target.RuntimeAgentID || attempt.SessionID != h.Target.SessionID || attempt.Provider != h.Target.Provider || attempt.BindingState != runtimeobs.BindingBound || attempt.StateEvidenceSource != runtimeobs.EvidenceHook || (attempt.StateObservationQuality != runtimeobs.QualityObserved && attempt.StateObservationQuality != runtimeobs.QualityAuthoritative) || attempt.CurrentState != runtimeobs.StateRunning || attempt.Terminal { continue }
					last, ok := parseTime(attempt.LastActivityAt); if ok && !last.Before(claimedAt.Add(-15*time.Minute)) && !last.After(now) { live++ }
				}
				if live == 1 { combinedEvents[id] = append(combinedEvents[id], lifecycleEvent{State:"doing", At:h.ClaimedAt, Source:"handoff_projection", ID:h.HandoffID, Kind:"review_started", Actor:h.ClaimedBy, EvidenceSource:"exact_controller_claim", AssignmentID:assignmentID, AttemptID:h.Target.AttemptID}) }
			}
		}
	}
	result:=map[string]map[string]any{}
    labels:=map[string]string{"todo":"Registered","doing":"Started","hold":"Hold","done":"Completed"}
    for id,seq:=range combinedEvents {
        sort.SliceStable(seq,func(i,j int)bool {
            ti,_:=parseTime(seq[i].At)
			tj,_:=parseTime(seq[j].At)
			if ti.Equal(tj) {
				order:=map[string]int{"registered":10,"assigned":20,"started":30,"review_started":35,"waiting":40,"resumed":50,"completed":60}
				return order[seq[i].Kind]<order[seq[j].Kind]
			}
			return ti.Before(tj)
        })
        seq=collapseLifecycleEvents(seq)
        reviewEvents:=[]lifecycleEvent{}
        lifecycleEvents:=seq[:0]
        for _,event:=range seq { if event.Kind=="review_started" { reviewEvents=append(reviewEvents,event) } else { lifecycleEvents=append(lifecycleEvents,event) } }
        seq=lifecycleEvents
        if len(seq)==0 { continue }
        type parsedEvent struct{ event lifecycleEvent
			at time.Time }
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
			label:=labels[entry.event.State]
			if entry.event.Kind!="" { label=map[string]string{"registered":"Registered","assigned":"Assigned","started":"Started","review_started":"Controller review started","waiting":"Waiting","resumed":"Resumed","completed":"Completed"}[entry.event.Kind] }
			eventRows=append(eventRows,map[string]any{
                "state":entry.event.State,
                "label":            label,
				"event_id":         entry.event.ID,
				"kind":             entry.event.Kind,
				"actor":            entry.event.Actor,
                "assigned_worker":  entry.event.AssignedWorker,
				"evidence_source":  entry.event.EvidenceSource,
                "assignment_id": entry.event.AssignmentID,
                "attempt_id": entry.event.AttemptID,
                "at":entry.event.At,
                "interval_seconds":interval,
                "interval":intervalValue,
                "source":entry.event.Source,
                "provisional":entry.event.Source == "runtime_observed",
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
        for _,entry:=range parsed { if entry.event.State=="doing" { v:=entry.at
				started=&v
				break } }
        var completed *time.Time
        for i:=len(parsed)-1;i>=0;i-- { if parsed[i].event.State=="done" { v:=parsed[i].at
				completed=&v
				break } }
        var queueSeconds any
        if started!=nil {
            q:=started.Sub(created).Seconds()
			if q<0 { q=0 }
			queueSeconds=q
        } else if completed==nil {
            q:=now.Sub(created).Seconds()
			if q<0 { q=0 }
			queueSeconds=q
        } else {
            queueSeconds=nil
        }
        leadEnd:=now
        if completed!=nil { leadEnd=*completed }
        leadSeconds:=leadEnd.Sub(created).Seconds()
		if leadSeconds<0 { leadSeconds=0 }
        var activeValue any
        if hasDoing { activeValue=activeSeconds }
        var waitValue any
        if hasHold || hasDoing || current.event.State=="todo" { waitValue=waitSeconds }
        var elapsed any
        if current.event.State=="doing" { elapsed=currentSegment }
        var duration any
        if current.event.State=="done" { duration=activeValue }

        for _, review := range reviewEvents {
            eventRows=append(eventRows,map[string]any{"state":"doing","label":"Controller review started","event_id":review.ID,"kind":review.Kind,"actor":review.Actor,"evidence_source":review.EvidenceSource,"assignment_id":review.AssignmentID,"attempt_id":review.AttemptID,"at":review.At,"interval_seconds":0.0,"interval":"-","source":review.Source,"provisional":false})
        }
        sort.SliceStable(eventRows,func(i,j int)bool{return toString(eventRows[i]["at"])<toString(eventRows[j]["at"])})
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
            "duration":func()string{ if current.event.State=="done" { return formatDuration(activeValue) }
				return "-" }(),
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
	for id,direct:=range directByID {
		row:=result[id]
		if row==nil || len(direct)==0 { continue }
		if _,ok:=currentRows[id];!ok {
			row["consistency_status"]="event_without_backlog_file"
			row["consistency_note"]="영속 전환은 기록됐지만 백로그 파일을 찾을 수 없어 확인이 필요합니다."
			continue
		}
		fileState:=currentRows[id].State
		last:=direct[len(direct)-1]
		if last.Kind=="assigned" && fileState=="doing" {
			row["start_evidence_status"]="assigned_not_started"
			row["file_observation"]="doing"
			continue
		}
		expected:=map[string]string{"registered":"todo","assigned":"doing","started":"doing","waiting":"hold","resumed":"doing","completed":"done"}[last.Kind]
		if expected!="" && fileState!=expected {
			row["consistency_status"]="event_file_mismatch"
			row["consistency_note"]="영속 전환과 현재 백로그 파일 상태가 달라 중단 또는 미반영 여부를 확인해야 합니다."
			row["file_observation"]=fileState
			row["last_lifecycle_event_id"]=last.EventID
		}
	}
	return result,nil
}

func lifecycleSourceRank(source string) int {
    switch source {
	case "durable_lifecycle":
		return 40
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
        if len(out)>0 && out[len(out)-1].State==event.State && out[len(out)-1].Kind == event.Kind {
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
