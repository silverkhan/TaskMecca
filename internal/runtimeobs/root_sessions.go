package runtimeobs

import (
    "bufio"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "time"
)

const rootSessionCleanupAge = 7 * 24 * time.Hour

type RootSessionStatus string

const (
    RootSessionActive           RootSessionStatus = "active"
    RootSessionNeedsCheck       RootSessionStatus = "needs_check"
    RootSessionTerminal         RootSessionStatus = "terminal"
    RootSessionInactiveTerminal RootSessionStatus = "inactive_terminal"
)

type RootSession struct {
    RootSessionID       string            `json:"root_session_id"`
    Provider            string            `json:"provider"`
    ProviderSessionID   string            `json:"provider_session_id,omitempty"`
    DisplayName         string            `json:"display_name"`
    DisplayNameSource   string            `json:"display_name_source"`
    CreatedAt           string            `json:"created_at,omitempty"`
    CreatedAtSource     string            `json:"created_at_source,omitempty"`
    LastActivityAt      string            `json:"last_activity_at,omitempty"`
    Status              RootSessionStatus `json:"status"`
    InactiveMillis      int64             `json:"inactive_ms"`
    CleanupEligible     bool              `json:"cleanup_eligible"`
    CleanupAfter        string            `json:"cleanup_after,omitempty"`
    AttemptCount        int               `json:"attempt_count"`
    AgentCount          int               `json:"agent_count"`
    CurrentCount        int               `json:"current_count"`
    NeedsCheckCount     int               `json:"needs_check_count"`
    TerminalCount       int               `json:"terminal_count"`
    AttemptIDs          []string          `json:"attempt_ids,omitempty"`
}

type RootSessionCollection struct {
    Items           []RootSession `json:"items"`
    Active          int           `json:"active"`
    NeedsCheck      int           `json:"needs_check"`
    Terminal        int           `json:"terminal"`
    CleanupEligible int           `json:"cleanup_eligible"`
    Total           int           `json:"total"`
    GeneratedAt     string        `json:"generated_at"`
}

type RootSessionStorage struct {
    Raw        StorageBucket `json:"raw"`
    History    StorageBucket `json:"history"`
    Legacy     StorageBucket `json:"legacy"`
    TotalBytes int64         `json:"total_bytes"`
}

type RootSessionCleanupPreview struct {
    RootSessionID    string             `json:"root_session_id"`
    Eligible         bool               `json:"eligible"`
    Reason           string             `json:"reason,omitempty"`
    Storage          RootSessionStorage `json:"storage"`
    ReclaimableBytes int64              `json:"reclaimable_bytes"`
    LastActivityAt   string             `json:"last_activity_at,omitempty"`
    CleanupAfter     string             `json:"cleanup_after,omitempty"`
    AttemptCount     int                `json:"attempt_count"`
}

type RootSessionCleanupResult struct {
    RootSessionID         string `json:"root_session_id"`
    RawFilesRewritten     int    `json:"raw_files_rewritten"`
    RawFilesRemoved       int    `json:"raw_files_removed"`
    HistoryFilesRewritten int    `json:"history_files_rewritten"`
    HistoryFilesRemoved   int    `json:"history_files_removed"`
    LegacyRewritten       bool   `json:"legacy_rewritten"`
    LegacyRemoved         bool   `json:"legacy_removed"`
    ReclaimedBytes        int64  `json:"reclaimed_bytes"`
}

type mutableRootSession struct {
    session        RootSession
    agents         map[string]bool
    attemptIDs     map[string]bool
    explicitName   string
    explicitNameAt string
    title          string
    titleAt        string
}

func rootSessionIDFor(provider, sessionID string) string {
    key:=strings.ToLower(strings.TrimSpace(provider))+"\x00"+strings.TrimSpace(sessionID)
    sum:=sha256.Sum256([]byte(key))
    return "rs-"+hex.EncodeToString(sum[:])[:16]
}

func unresolvedRootSessionID(provider, attemptID string) string {
    key:=strings.ToLower(strings.TrimSpace(provider))+"\x00unresolved\x00"+strings.TrimSpace(attemptID)
    sum:=sha256.Sum256([]byte(key))
    return "rs-unresolved-"+hex.EncodeToString(sum[:])[:12]
}

func rootAttemptCreatedAt(attempt Attempt) string {
    return firstNonEmptyRuntime(attempt.FirstObservedAt,attempt.StartedAt,attempt.LastObservedAt,attempt.EndedAt)
}

func rootAttemptLastActivity(attempt Attempt) string {
    latest:=""
    for _,value:=range []string{attempt.LastActivityAt,attempt.LastObservedAt,attempt.EndedAt,attempt.StartedAt,attempt.FirstObservedAt} {
        if value>latest { latest=value }
    }
    return latest
}

func rootFallbackName(createdAt string) string {
    if parsed,err:=time.Parse(time.RFC3339Nano,createdAt); err==nil {
        return "Root · "+parsed.Local().Format("2006-01-02 15:04")
    }
    return "Root Session"
}

func rootSessionAttempts(project string,ledger Ledger,now time.Time) ([]Attempt,error) {
    terminal,err:=terminalHistoryAttempts(project,ledger,now)
    if err!=nil { return nil,err }
    merged:=map[string]Attempt{}
    for _,attempt:=range terminal { merged[attempt.AttemptID]=attempt }
    for _,attempt:=range ledger.Attempts { merged[attempt.AttemptID]=attempt }
    out:=make([]Attempt,0,len(merged))
    for _,attempt:=range merged { out=append(out,attempt) }
    sort.Slice(out,func(i,j int)bool {
        left:=rootAttemptLastActivity(out[i])
        right:=rootAttemptLastActivity(out[j])
        if left!=right { return left>right }
        return out[i].AttemptID<out[j].AttemptID
    })
    return out,nil
}

func BuildRootSessions(project string,ledger Ledger,now time.Time) (RootSessionCollection,error) {
    attempts,err:=rootSessionAttempts(project,ledger,now)
    if err!=nil { return RootSessionCollection{},err }

    findingsByAttempt:=map[string][]LedgerFinding{}
    for _,finding:=range ledger.Findings {
        if finding.AttemptID!="" {
            findingsByAttempt[finding.AttemptID]=append(findingsByAttempt[finding.AttemptID],finding)
        }
    }

    groups:=map[string]*mutableRootSession{}
    order:=[]string{}
    for _,attempt:=range attempts {
        provider:=strings.ToLower(strings.TrimSpace(attempt.Provider))
        providerSessionID:=strings.TrimSpace(attempt.SessionID)
        rootID:=""
        groupKey:=""
        if providerSessionID!="" {
            rootID=rootSessionIDFor(provider,providerSessionID)
            groupKey=provider+"\x00"+providerSessionID
        } else {
            rootID=unresolvedRootSessionID(provider,attempt.AttemptID)
            groupKey=provider+"\x00unresolved\x00"+attempt.AttemptID
        }
        group:=groups[groupKey]
        if group==nil {
            group=&mutableRootSession{
                session:RootSession{
                    RootSessionID:rootID,
                    Provider:provider,
                    ProviderSessionID:providerSessionID,
                    DisplayNameSource:"task_mecca_fallback",
                    CreatedAtSource:"first_observed",
                    AttemptIDs:[]string{},
                },
                agents:map[string]bool{},
                attemptIDs:map[string]bool{},
            }
            groups[groupKey]=group
            order=append(order,groupKey)
        }

        if !group.attemptIDs[attempt.AttemptID] {
            group.attemptIDs[attempt.AttemptID]=true
            group.session.AttemptIDs=append(group.session.AttemptIDs,attempt.AttemptID)
            group.session.AttemptCount++
        }
        agentKey:=strings.TrimSpace(attempt.AgentPath)
        if agentKey=="" { agentKey=strings.TrimSpace(attempt.RuntimeAgentID) }
        if agentKey=="" { agentKey=attempt.AttemptID }
        group.agents[agentKey]=true

        created:=rootAttemptCreatedAt(attempt)
        if created!="" && (group.session.CreatedAt=="" || created<group.session.CreatedAt) {
            group.session.CreatedAt=created
        }
        last:=rootAttemptLastActivity(attempt)
        if last>group.session.LastActivityAt { group.session.LastActivityAt=last }

        if attempt.SessionName!="" && (group.explicitNameAt=="" || last>=group.explicitNameAt) {
            group.explicitName=attempt.SessionName
            group.explicitNameAt=last
        }
        if attempt.SessionTitle!="" && (group.titleAt=="" || last>=group.titleAt) {
            group.title=attempt.SessionTitle
            group.titleAt=last
        }

        switch SessionClassForAttempt(attempt,findingsByAttempt[attempt.AttemptID]) {
        case SessionClassCurrent:
            group.session.CurrentCount++
        case SessionClassNeedsCheck:
            group.session.NeedsCheckCount++
        default:
            group.session.TerminalCount++
        }
    }

    collection:=RootSessionCollection{
        Items:[]RootSession{},
        GeneratedAt:now.UTC().Format(time.RFC3339Nano),
    }
    for _,key:=range order {
        group:=groups[key]
        root:=group.session
        root.AgentCount=len(group.agents)
        sort.Strings(root.AttemptIDs)

        switch {
        case group.explicitName!="":
            root.DisplayName=group.explicitName
            root.DisplayNameSource="provider_name"
        case group.title!="":
            root.DisplayName=group.title
            root.DisplayNameSource="provider_title"
        default:
            root.DisplayName=rootFallbackName(root.CreatedAt)
            root.DisplayNameSource="task_mecca_fallback"
        }

        if root.LastActivityAt!="" {
            if parsed,parseErr:=time.Parse(time.RFC3339Nano,root.LastActivityAt); parseErr==nil && now.After(parsed) {
                root.InactiveMillis=now.Sub(parsed).Milliseconds()
                cleanupAt:=parsed.Add(rootSessionCleanupAge)
                root.CleanupAfter=cleanupAt.UTC().Format(time.RFC3339Nano)
            }
        }

        switch {
        case root.CurrentCount>0:
            root.Status=RootSessionActive
            collection.Active++
        case root.NeedsCheckCount>0:
            root.Status=RootSessionNeedsCheck
            collection.NeedsCheck++
        default:
            root.Status=RootSessionTerminal
            collection.Terminal++
            if root.ProviderSessionID!="" && root.InactiveMillis>=rootSessionCleanupAge.Milliseconds() {
                root.Status=RootSessionInactiveTerminal
                root.CleanupEligible=true
                collection.CleanupEligible++
            }
        }
        collection.Items=append(collection.Items,root)
    }

    sort.SliceStable(collection.Items,func(i,j int)bool {
        rank:=func(status RootSessionStatus) int {
            switch status {
            case RootSessionActive: return 0
            case RootSessionNeedsCheck: return 1
            case RootSessionTerminal: return 2
            default: return 3
            }
        }
        leftRank,rightRank:=rank(collection.Items[i].Status),rank(collection.Items[j].Status)
        if leftRank!=rightRank { return leftRank<rightRank }
        if collection.Items[i].LastActivityAt!=collection.Items[j].LastActivityAt {
            return collection.Items[i].LastActivityAt>collection.Items[j].LastActivityAt
        }
        return collection.Items[i].RootSessionID<collection.Items[j].RootSessionID
    })
    collection.Total=len(collection.Items)
    return collection,nil
}

func FindRootSession(project string,ledger Ledger,rootSessionID string,now time.Time) (RootSession,error) {
    collection,err:=BuildRootSessions(project,ledger,now)
    if err!=nil { return RootSession{},err }
    for _,root:=range collection.Items {
        if root.RootSessionID==rootSessionID { return root,nil }
    }
    return RootSession{},fmt.Errorf("root session not found: %s",rootSessionID)
}

func lineMatchesRootEvent(line []byte,provider,sessionID string) bool {
    var event ExecutionEvent
    if err:=json.Unmarshal(line,&event); err!=nil { return false }
    return strings.EqualFold(event.Provider,provider) && event.SessionID==sessionID
}

func lineMatchesRootHistory(line []byte,provider,sessionID string) bool {
    var record HistoryRecord
    if err:=json.Unmarshal(line,&record); err!=nil { return false }
    return strings.EqualFold(record.Attempt.Provider,provider) && record.Attempt.SessionID==sessionID
}

func rootSessionBucketFromFiles(paths []string,matcher func([]byte) bool) (StorageBucket,error) {
    bucket:=StorageBucket{}
    for _,path:=range paths {
        file,err:=os.Open(path)
        if errors.Is(err,os.ErrNotExist) { continue }
        if err!=nil { return bucket,err }
        scanner:=bufio.NewScanner(file)
        scanner.Buffer(make([]byte,64*1024),2*1024*1024)
        matched:=false
        for scanner.Scan() {
            raw:=append([]byte{},scanner.Bytes()...)
            if matcher(raw) {
                matched=true
                bucket.Bytes+=int64(len(raw)+1)
            }
        }
        scanErr:=scanner.Err()
        closeErr:=file.Close()
        if scanErr!=nil { return bucket,scanErr }
        if closeErr!=nil { return bucket,closeErr }
        if matched { bucket.Files++ }
    }
    return bucket,nil
}

func RootSessionStorageUsage(project,provider,sessionID string) (RootSessionStorage,error) {
    eventFiles,err:=executionEventFiles(project)
    if err!=nil { return RootSessionStorage{},err }
    rawPaths:=[]string{}
    legacyPaths:=[]string{}
    legacy:=ExecutionLegacyJournalPath(project)
    for _,path:=range eventFiles {
        if path==legacy { legacyPaths=append(legacyPaths,path) } else { rawPaths=append(rawPaths,path) }
    }
    historyPaths,err:=historyFiles(project)
    if err!=nil { return RootSessionStorage{},err }

    raw,err:=rootSessionBucketFromFiles(rawPaths,func(line []byte)bool{
        return lineMatchesRootEvent(line,provider,sessionID)
    })
    if err!=nil { return RootSessionStorage{},err }
    legacyBucket,err:=rootSessionBucketFromFiles(legacyPaths,func(line []byte)bool{
        return lineMatchesRootEvent(line,provider,sessionID)
    })
    if err!=nil { return RootSessionStorage{},err }
    history,err:=rootSessionBucketFromFiles(historyPaths,func(line []byte)bool{
        return lineMatchesRootHistory(line,provider,sessionID)
    })
    if err!=nil { return RootSessionStorage{},err }
    return RootSessionStorage{
        Raw:raw,History:history,Legacy:legacyBucket,
        TotalBytes:raw.Bytes+history.Bytes+legacyBucket.Bytes,
    },nil
}

func PreviewRootSessionCleanup(project string,ledger Ledger,rootSessionID string,now time.Time) (RootSessionCleanupPreview,error) {
    root,err:=FindRootSession(project,ledger,rootSessionID,now)
    if err!=nil { return RootSessionCleanupPreview{},err }
    preview:=RootSessionCleanupPreview{
        RootSessionID:root.RootSessionID,
        Eligible:root.CleanupEligible,
        LastActivityAt:root.LastActivityAt,
        CleanupAfter:root.CleanupAfter,
        AttemptCount:root.AttemptCount,
    }
    if root.ProviderSessionID=="" {
        preview.Eligible=false
        preview.Reason="provider session id is unavailable"
        return preview,nil
    }
    storage,err:=RootSessionStorageUsage(project,root.Provider,root.ProviderSessionID)
    if err!=nil { return preview,err }
    preview.Storage=storage
    if preview.Eligible { preview.ReclaimableBytes=storage.TotalBytes }
    if !preview.Eligible {
        switch root.Status {
        case RootSessionActive:
            preview.Reason="root session still has current executions"
        case RootSessionNeedsCheck:
            preview.Reason="root session has unresolved runtime executions"
        default:
            preview.Reason="root session has not been inactive for 7 days"
        }
    }
    return preview,nil
}

func rewriteJSONLForRoot(path string,matcher func([]byte)bool) (rewritten,removed bool,reclaimed int64,err error) {
    data,readErr:=os.ReadFile(path)
    if errors.Is(readErr,os.ErrNotExist) { return false,false,0,nil }
    if readErr!=nil { return false,false,0,readErr }
    originalSize:=int64(len(data))
    scanner:=bufio.NewScanner(strings.NewReader(string(data)))
    scanner.Buffer(make([]byte,64*1024),2*1024*1024)
    kept:=[]byte{}
    removedAny:=false
    for scanner.Scan() {
        raw:=append([]byte{},scanner.Bytes()...)
        if matcher(raw) {
            removedAny=true
            continue
        }
        kept=append(kept,raw...)
        kept=append(kept,'\n')
    }
    if scanErr:=scanner.Err(); scanErr!=nil { return false,false,0,scanErr }
    if !removedAny { return false,false,0,nil }
    if len(kept)==0 {
        if removeErr:=os.Remove(path); removeErr!=nil && !errors.Is(removeErr,os.ErrNotExist) {
            return false,false,0,removeErr
        }
        return false,true,originalSize,nil
    }
    info,statErr:=os.Stat(path)
    mode:=os.FileMode(0600)
    if statErr==nil { mode=info.Mode().Perm() }
    temp:=path+".root-cleanup.tmp"
    if writeErr:=os.WriteFile(temp,kept,mode); writeErr!=nil { return false,false,0,writeErr }
    if renameErr:=os.Rename(temp,path); renameErr!=nil {
        _=os.Remove(temp)
        return false,false,0,renameErr
    }
    reclaimed=originalSize-int64(len(kept))
    if reclaimed<0 { reclaimed=0 }
    return true,false,reclaimed,nil
}

func CleanupRootSession(project string,ledger Ledger,rootSessionID string,now time.Time) (RootSessionCleanupResult,error) {
    executionStorageMu.Lock()
    defer executionStorageMu.Unlock()

    root,err:=FindRootSession(project,ledger,rootSessionID,now)
    if err!=nil { return RootSessionCleanupResult{},err }
    if !root.CleanupEligible || root.ProviderSessionID=="" {
        return RootSessionCleanupResult{},fmt.Errorf("root session is not eligible for cleanup")
    }

    result:=RootSessionCleanupResult{RootSessionID:root.RootSessionID}
    eventFiles,err:=executionEventFiles(project)
    if err!=nil { return result,err }
    legacy:=ExecutionLegacyJournalPath(project)
    for _,path:=range eventFiles {
        rewritten,removed,reclaimed,rewriteErr:=rewriteJSONLForRoot(path,func(line []byte)bool{
            return lineMatchesRootEvent(line,root.Provider,root.ProviderSessionID)
        })
        if rewriteErr!=nil { return result,rewriteErr }
        result.ReclaimedBytes+=reclaimed
        if path==legacy {
            result.LegacyRewritten=result.LegacyRewritten||rewritten
            result.LegacyRemoved=result.LegacyRemoved||removed
        } else {
            if rewritten { result.RawFilesRewritten++ }
            if removed { result.RawFilesRemoved++ }
        }
    }

    historyPaths,err:=historyFiles(project)
    if err!=nil { return result,err }
    for _,path:=range historyPaths {
        rewritten,removed,reclaimed,rewriteErr:=rewriteJSONLForRoot(path,func(line []byte)bool{
            return lineMatchesRootHistory(line,root.Provider,root.ProviderSessionID)
        })
        if rewriteErr!=nil { return result,rewriteErr }
        result.ReclaimedBytes+=reclaimed
        if rewritten { result.HistoryFilesRewritten++ }
        if removed { result.HistoryFilesRemoved++ }
    }
    return result,nil
}
