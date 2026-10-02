package runtimeobs

import (
    "bufio"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "time"
)

const (
    SessionClassCurrent    = "current"
    SessionClassNeedsCheck = "needs_check"
    SessionClassTerminal   = "terminal"
)

type SessionGroups struct {
    CurrentIDs    []string `json:"current_ids"`
    NeedsCheckIDs []string `json:"needs_check_ids"`
    TerminalIDs   []string `json:"terminal_ids"`
    Current       int      `json:"current"`
    NeedsCheck    int      `json:"needs_check"`
    Terminal      int      `json:"terminal"`
}

type StorageBucket struct {
    Files int   `json:"files"`
    Bytes int64 `json:"bytes"`
}

type CleanupPreview struct {
    CandidateFiles       int    `json:"candidate_files"`
    CandidateAttempts    int    `json:"candidate_attempts"`
    ReclaimableBytes     int64  `json:"reclaimable_bytes"`
    RawFiles             int    `json:"raw_files"`
    RawBytes             int64  `json:"raw_bytes"`
    LegacyFiles          int    `json:"legacy_files"`
    LegacyBytes          int64  `json:"legacy_bytes"`
    HistoryRecords       int    `json:"history_records"`
    HistoryBytes         int64  `json:"history_bytes"`
    OldestCandidateAt    string `json:"oldest_candidate_at,omitempty"`
    ProtectedAttempts    int    `json:"protected_attempts"`
}

type StorageReport struct {
    TotalBytes        int64          `json:"total_bytes"`
    Raw               StorageBucket  `json:"raw"`
    History           StorageBucket  `json:"history"`
    Legacy            StorageBucket  `json:"legacy"`
    ProtectedRaw      StorageBucket  `json:"protected_raw"`
    HistoryAttempts   int            `json:"history_attempts"`
    Retention         map[string]int `json:"retention"`
    Cleanup           CleanupPreview `json:"cleanup"`
    GeneratedAt       string         `json:"generated_at"`
}

type CleanupResult struct {
    PreviewBefore         CleanupPreview `json:"preview_before"`
    RawFilesRemoved       int            `json:"raw_files_removed"`
    LegacyFilesRemoved    int            `json:"legacy_files_removed"`
    HistoryFilesRewritten int            `json:"history_files_rewritten"`
    ReclaimedBytes        int64          `json:"reclaimed_bytes"`
    RemainingBytes        int64          `json:"remaining_bytes"`
}

type detailedHistoryRecord struct {
    Record HistoryRecord
    Bytes  int64
}

type cleanupPlan struct {
    Preview CleanupPreview
    RawPaths []string
    LegacyPaths []string
    RetainedHistory map[string]HistoryRecord
    RewriteHistory bool
}

var executionStorageMu sync.Mutex

func SessionClassForAttempt(attempt Attempt, findings []LedgerFinding) string {
    if attempt.Terminal {
        return SessionClassTerminal
    }
    if attempt.CurrentState == StateRuntimeUnknown {
        return SessionClassNeedsCheck
    }
    for _,finding:=range findings {
        if finding.AttemptID!=attempt.AttemptID { continue }
        switch finding.Code {
        case "stale","runtime_unknown":
            return SessionClassNeedsCheck
        }
    }
    switch attempt.CurrentState {
    case StateStarting,StateRunning,StateWaitingUser,StateWaitingApproval:
        return SessionClassCurrent
    default:
        return SessionClassNeedsCheck
    }
}

func ClassifySessions(attempts []Attempt, findings []LedgerFinding) SessionGroups {
    groups:=SessionGroups{CurrentIDs:[]string{},NeedsCheckIDs:[]string{},TerminalIDs:[]string{}}
    for _,attempt:=range attempts {
        switch SessionClassForAttempt(attempt,findings) {
        case SessionClassCurrent:
            groups.Current++
            groups.CurrentIDs=append(groups.CurrentIDs,attempt.AttemptID)
        case SessionClassNeedsCheck:
            groups.NeedsCheck++
            groups.NeedsCheckIDs=append(groups.NeedsCheckIDs,attempt.AttemptID)
        default:
            groups.Terminal++
            groups.TerminalIDs=append(groups.TerminalIDs,attempt.AttemptID)
        }
    }
    return groups
}

func directoryBucket(path string) (StorageBucket,error) {
    bucket:=StorageBucket{}
    err:=filepath.WalkDir(path,func(current string,entry os.DirEntry,walkErr error) error {
        if walkErr!=nil {
            if errors.Is(walkErr,os.ErrNotExist) { return nil }
            return walkErr
        }
        if entry.IsDir() { return nil }
        info,err:=entry.Info(); if err!=nil { return err }
        bucket.Files++
        bucket.Bytes+=info.Size()
        return nil
    })
    if errors.Is(err,os.ErrNotExist) { return StorageBucket{},nil }
    return bucket,err
}

func fileBucket(path string) (StorageBucket,error) {
    info,err:=os.Stat(path)
    if errors.Is(err,os.ErrNotExist) { return StorageBucket{},nil }
    if err!=nil { return StorageBucket{},err }
    if info.IsDir() { return StorageBucket{},nil }
    return StorageBucket{Files:1,Bytes:info.Size()},nil
}

func readDetailedHistory(project string) (map[string]detailedHistoryRecord,error) {
    files,err:=historyFiles(project); if err!=nil { return nil,err }
    out:=map[string]detailedHistoryRecord{}
    for _,path:=range files {
        file,err:=os.Open(path); if err!=nil { return nil,err }
        scanner:=bufio.NewScanner(file)
        scanner.Buffer(make([]byte,64*1024),2*1024*1024)
        for scanner.Scan() {
            raw:=append([]byte{},scanner.Bytes()...)
            var record HistoryRecord
            if err:=json.Unmarshal(raw,&record); err!=nil { continue }
            attempt:=record.Attempt
            if attempt.AttemptID=="" || !attempt.Terminal { continue }
            candidate:=detailedHistoryRecord{Record:record,Bytes:int64(len(raw)+1)}
            previous,ok:=out[attempt.AttemptID]
            if !ok || terminalSortTime(attempt)>terminalSortTime(previous.Record.Attempt) {
                out[attempt.AttemptID]=candidate
            }
        }
        scanErr:=scanner.Err()
        closeErr:=file.Close()
        if scanErr!=nil { return nil,scanErr }
        if closeErr!=nil { return nil,closeErr }
    }
    return out,nil
}

func historyRetentionRecords(project string,ledger Ledger,now time.Time) (map[string]HistoryRecord,int,error) {
    detailed,err:=readDetailedHistory(project); if err!=nil { return nil,0,err }
    records:=map[string]HistoryRecord{}
    for id,item:=range detailed { records[id]=item.Record }

    for _,attempt:=range ledger.Attempts {
        if attempt.Terminal {
            existing,ok:=records[attempt.AttemptID]
            if !ok || terminalSortTime(attempt)>terminalSortTime(existing.Attempt) {
                records[attempt.AttemptID]=HistoryRecord{
                    Version:1,
                    ArchivedAt:now.UTC().Format(time.RFC3339Nano),
                    Attempt:attempt,
                }
            }
        } else {
            delete(records,attempt.AttemptID)
        }
    }

    type row struct { id string; record HistoryRecord }
    rows:=make([]row,0,len(records))
    cutoff:=now.UTC().AddDate(0,0,-historyRetentionDays)
    for id,record:=range records {
        at:=eventObservedTime(terminalSortTime(record.Attempt),time.Time{})
        if !at.IsZero() && at.Before(cutoff) { continue }
        rows=append(rows,row{id:id,record:record})
    }
    sort.Slice(rows,func(i,j int)bool {
        left:=terminalSortTime(rows[i].record.Attempt)
        right:=terminalSortTime(rows[j].record.Attempt)
        if left!=right { return left>right }
        return rows[i].id<rows[j].id
    })
    if len(rows)>historyMaxAttempts { rows=rows[:historyMaxAttempts] }
    retained:=map[string]HistoryRecord{}
    for _,row:=range rows { retained[row.id]=row.record }
    removed:=len(records)-len(retained)
    if removed<0 { removed=0 }
    return retained,removed,nil
}

func rawFileMetadata(path string) (map[string]bool,time.Time,error) {
    file,err:=os.Open(path); if err!=nil { return nil,time.Time{},err }
    defer file.Close()
    ids:=map[string]bool{}
    latest:=time.Time{}
    scanner:=bufio.NewScanner(file)
    scanner.Buffer(make([]byte,64*1024),2*1024*1024)
    for scanner.Scan() {
        var event ExecutionEvent
        if err:=json.Unmarshal(scanner.Bytes(),&event); err!=nil { continue }
        if event.AttemptID!="" { ids[event.AttemptID]=true }
        at:=eventObservedTime(event.ObservedAt,time.Time{})
        if at.After(latest) { latest=at }
    }
    return ids,latest,scanner.Err()
}

func buildCleanupPlan(project string,ledger Ledger,now time.Time) (cleanupPlan,error) {
    plan:=cleanupPlan{RawPaths:[]string{},LegacyPaths:[]string{},RetainedHistory:map[string]HistoryRecord{}}
    cleanupAttemptIDs:=map[string]bool{}
    currentNonTerminal:=map[string]bool{}
    for _,attempt:=range ledger.Attempts {
        if !attempt.Terminal { currentNonTerminal[attempt.AttemptID]=true }
    }
    plan.Preview.ProtectedAttempts=len(currentNonTerminal)

    detailed,err:=readDetailedHistory(project); if err!=nil { return plan,err }
    historyKnown:=map[string]bool{}
    for id:=range detailed { historyKnown[id]=true }
    for _,attempt:=range ledger.Attempts {
        if attempt.Terminal { historyKnown[attempt.AttemptID]=true }
    }

    cutoff:=now.UTC().AddDate(0,0,-rawRetentionDays)
    rawEntries,err:=os.ReadDir(ExecutionRawDir(project))
    if err!=nil && !errors.Is(err,os.ErrNotExist) { return plan,err }
    for _,entry:=range rawEntries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(),".jsonl") { continue }
        path:=filepath.Join(ExecutionRawDir(project),entry.Name())
        ids,latest,scanErr:=rawFileMetadata(path)
        if scanErr!=nil { continue }
        safe:=!latest.IsZero() && latest.Before(cutoff)
        for id:=range ids {
            if currentNonTerminal[id] || !historyKnown[id] { safe=false; break }
        }
        if !safe { continue }
        info,statErr:=entry.Info(); if statErr!=nil { continue }
        plan.RawPaths=append(plan.RawPaths,path)
        plan.Preview.RawFiles++
        plan.Preview.RawBytes+=info.Size()
        for id:=range ids { cleanupAttemptIDs[id]=true }
        updateOldestCandidate(&plan.Preview,latest)
    }

    legacy:=ExecutionLegacyJournalPath(project)
    if info,statErr:=os.Stat(legacy); statErr==nil && !info.IsDir() {
        ids,latest,scanErr:=rawFileMetadata(legacy)
        safe:=scanErr==nil && !latest.IsZero() && latest.Before(cutoff)
        for id:=range ids {
            if currentNonTerminal[id] || !historyKnown[id] { safe=false; break }
        }
        if safe {
            plan.LegacyPaths=append(plan.LegacyPaths,legacy)
            plan.Preview.LegacyFiles=1
            plan.Preview.LegacyBytes=info.Size()
            for id:=range ids { cleanupAttemptIDs[id]=true }
            updateOldestCandidate(&plan.Preview,latest)
        }
    }

    retained,removed,err:=historyRetentionRecords(project,ledger,now)
    if err!=nil { return plan,err }
    plan.RetainedHistory=retained
    plan.Preview.HistoryRecords=removed
    historyNeedsRewrite:=removed>0
    for id,record:=range retained {
        existing,ok:=detailed[id]
        if !ok || terminalSortTime(record.Attempt)>terminalSortTime(existing.Record.Attempt) {
            historyNeedsRewrite=true
        }
    }
    currentHistoryBucket,err:=directoryBucket(ExecutionHistoryDir(project)); if err!=nil { return plan,err }
    targetBytes:=int64(0)
    for _,record:=range retained {
        payload,marshalErr:=json.Marshal(record)
        if marshalErr!=nil { return plan,marshalErr }
        targetBytes+=int64(len(payload)+1)
    }
    if currentHistoryBucket.Bytes>targetBytes {
        plan.Preview.HistoryBytes=currentHistoryBucket.Bytes-targetBytes
    }
    if historyNeedsRewrite || plan.Preview.HistoryBytes>0 {
        plan.RewriteHistory=true
    }

    plan.Preview.CandidateFiles=plan.Preview.RawFiles+plan.Preview.LegacyFiles
    for id:=range cleanupAttemptIDs { _=id; plan.Preview.CandidateAttempts++ }
    plan.Preview.CandidateAttempts+=plan.Preview.HistoryRecords
    plan.Preview.ReclaimableBytes=plan.Preview.RawBytes+plan.Preview.LegacyBytes+plan.Preview.HistoryBytes
    return plan,nil
}

func updateOldestCandidate(preview *CleanupPreview,at time.Time) {
    if at.IsZero() { return }
    current:=eventObservedTime(preview.OldestCandidateAt,time.Time{})
    if current.IsZero() || at.Before(current) {
        preview.OldestCandidateAt=at.UTC().Format(time.RFC3339)
    }
}

func protectedRawBucket(project string,ledger Ledger) (StorageBucket,error) {
    active:=map[string]bool{}
    for _,attempt:=range ledger.Attempts {
        if !attempt.Terminal { active[attempt.AttemptID]=true }
    }
    bucket:=StorageBucket{}
    files,err:=executionEventFiles(project); if err!=nil { return bucket,err }
    for _,path:=range files {
        ids,_,scanErr:=rawFileMetadata(path)
        if scanErr!=nil { continue }
        protected:=false
        for id:=range ids {
            if active[id] { protected=true; break }
        }
        if !protected { continue }
        info,statErr:=os.Stat(path); if statErr!=nil { continue }
        bucket.Files++
        bucket.Bytes+=info.Size()
    }
    return bucket,nil
}

func RuntimeStorageReport(project string,ledger Ledger,now time.Time) (StorageReport,error) {
    rawBucket,err:=directoryBucket(ExecutionRawDir(project)); if err!=nil { return StorageReport{},err }
    historyBucket,err:=directoryBucket(ExecutionHistoryDir(project)); if err!=nil { return StorageReport{},err }
    legacyBucket,err:=fileBucket(ExecutionLegacyJournalPath(project)); if err!=nil { return StorageReport{},err }
    protectedBucket,err:=protectedRawBucket(project,ledger); if err!=nil { return StorageReport{},err }
    history,err:=readHistoryAttempts(project); if err!=nil { return StorageReport{},err }
    plan,err:=buildCleanupPlan(project,ledger,now); if err!=nil { return StorageReport{},err }
    return StorageReport{
        TotalBytes:rawBucket.Bytes+historyBucket.Bytes+legacyBucket.Bytes,
        Raw:rawBucket,History:historyBucket,Legacy:legacyBucket,ProtectedRaw:protectedBucket,
        HistoryAttempts:len(history),Retention:RetentionPolicy(),Cleanup:plan.Preview,
        GeneratedAt:now.UTC().Format(time.RFC3339Nano),
    },nil
}

func writeHistoryRecords(project string,records map[string]HistoryRecord) (int,error) {
    grouped:=map[string][]HistoryRecord{}
    for _,record:=range records {
        at:=eventObservedTime(terminalSortTime(record.Attempt),time.Now())
        key:=at.UTC().Format("2006-01")
        grouped[key]=append(grouped[key],record)
    }
    for key:=range grouped {
        sort.Slice(grouped[key],func(i,j int)bool {
            return terminalSortTime(grouped[key][i].Attempt)>terminalSortTime(grouped[key][j].Attempt)
        })
    }

    dir:=ExecutionHistoryDir(project)
    if err:=os.MkdirAll(dir,0700); err!=nil { return 0,err }
    existing,err:=historyFiles(project); if err!=nil { return 0,err }
    written:=0
    keep:=map[string]bool{}
    for month,rows:=range grouped {
        path:=filepath.Join(dir,month+".jsonl")
        keep[path]=true
        payload:=[]byte{}
        for _,record:=range rows {
            line,marshalErr:=json.Marshal(record); if marshalErr!=nil { return written,marshalErr }
            payload=append(payload,line...)
            payload=append(payload,'\n')
        }
        temp:=path+".tmp"
        if err:=os.WriteFile(temp,payload,0600); err!=nil { return written,err }
        _=os.Remove(path)
        if err:=os.Rename(temp,path); err!=nil { return written,err }
        written++
    }
    for _,path:=range existing {
        if keep[path] { continue }
        if err:=os.Remove(path); err!=nil && !errors.Is(err,os.ErrNotExist) { return written,err }
    }
    return written,nil
}

func CleanupRuntimeStorage(project string,ledger Ledger,now time.Time) (CleanupResult,error) {
    executionStorageMu.Lock()
    defer executionStorageMu.Unlock()

    before,err:=RuntimeStorageReport(project,ledger,now)
    if err!=nil { return CleanupResult{},err }
    plan,err:=buildCleanupPlan(project,ledger,now)
    if err!=nil { return CleanupResult{},err }
    result:=CleanupResult{PreviewBefore:plan.Preview}

    // Persist/normalize terminal summaries first. Raw evidence is only removed
    // after the compact history is safely written.
    if plan.RewriteHistory {
        rewritten,writeErr:=writeHistoryRecords(project,plan.RetainedHistory)
        if writeErr!=nil { return result,writeErr }
        result.HistoryFilesRewritten=rewritten
    }

    for _,path:=range plan.RawPaths {
        if err:=os.Remove(path); err!=nil && !errors.Is(err,os.ErrNotExist) { return result,err }
        result.RawFilesRemoved++
    }
    for _,path:=range plan.LegacyPaths {
        if err:=os.Remove(path); err!=nil && !errors.Is(err,os.ErrNotExist) { return result,err }
        result.LegacyFilesRemoved++
    }
    after,err:=RuntimeStorageReport(project,ledger,now)
    if err!=nil { return result,err }
    result.RemainingBytes=after.TotalBytes
    if before.TotalBytes>after.TotalBytes {
        result.ReclaimedBytes=before.TotalBytes-after.TotalBytes
    }
    return result,nil
}

func StorageSummaryText(report StorageReport) string {
    return fmt.Sprintf("runtime %.1f KiB · reclaimable %.1f KiB",
        float64(report.TotalBytes)/1024,float64(report.Cleanup.ReclaimableBytes)/1024)
}
