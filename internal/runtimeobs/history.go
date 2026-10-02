package runtimeobs

import (
    "bufio"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"
)

const (
    rawRetentionDays     = 7
    historyRetentionDays = 90
    historyMaxAttempts   = 2000
    defaultHistoryPage   = 20
    maxHistoryPage       = 100
)

type HistoryRecord struct {
    Version    int     `json:"version"`
    ArchivedAt string  `json:"archived_at"`
    Attempt    Attempt `json:"attempt"`
}

type HistoryPage struct {
    Items      []Attempt `json:"items"`
    Page       int       `json:"page"`
    PageSize   int       `json:"page_size"`
    Total      int       `json:"total"`
    TotalPages int       `json:"total_pages"`
    Provider   string    `json:"provider,omitempty"`
    State      string    `json:"state,omitempty"`
    Retention  map[string]any `json:"retention"`
}

type RetentionStats struct {
    SummariesAdded int `json:"summaries_added"`
    RawFilesRemoved int `json:"raw_files_removed"`
    HistoryFilesRemoved int `json:"history_files_removed"`
}

func ExecutionRootPath(project string) string {
    return filepath.Join(project, "_task_mecca", ".runtime", "executions")
}

func ExecutionRawDir(project string) string {
    return filepath.Join(ExecutionRootPath(project), "raw")
}

func ExecutionHistoryDir(project string) string {
    return filepath.Join(ExecutionRootPath(project), "history")
}

func ExecutionLegacyJournalPath(project string) string {
    return filepath.Join(ExecutionRootPath(project), "events.jsonl")
}

func executionRawPathAt(project string, at time.Time) string {
    return filepath.Join(ExecutionRawDir(project), at.UTC().Format("2006-01-02")+".jsonl")
}

func executionHistoryPathAt(project string, at time.Time) string {
    return filepath.Join(ExecutionHistoryDir(project), at.UTC().Format("2006-01")+".jsonl")
}

func eventObservedTime(value string, fallback time.Time) time.Time {
    if parsed,err:=time.Parse(time.RFC3339Nano,strings.TrimSpace(value)); err==nil {
        return parsed
    }
    return fallback
}

func executionEventFiles(project string) ([]string,error) {
    files:=[]string{}
    legacy:=ExecutionLegacyJournalPath(project)
    if info,err:=os.Stat(legacy); err==nil && !info.IsDir() {
        files=append(files,legacy)
    } else if err!=nil && !errors.Is(err,os.ErrNotExist) {
        return nil,err
    }

    entries,err:=os.ReadDir(ExecutionRawDir(project))
    if errors.Is(err,os.ErrNotExist) { return files,nil }
    if err!=nil { return nil,err }
    for _,entry:=range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(),".jsonl") { continue }
        files=append(files,filepath.Join(ExecutionRawDir(project),entry.Name()))
    }
    sort.Strings(files)
    return files,nil
}

func historyFiles(project string) ([]string,error) {
    entries,err:=os.ReadDir(ExecutionHistoryDir(project))
    if errors.Is(err,os.ErrNotExist) { return []string{},nil }
    if err!=nil { return nil,err }
    files:=[]string{}
    for _,entry:=range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(),".jsonl") { continue }
        files=append(files,filepath.Join(ExecutionHistoryDir(project),entry.Name()))
    }
    sort.Strings(files)
    return files,nil
}

func appendHistoryAttempt(project string, attempt Attempt, now time.Time) error {
    if !attempt.Terminal || strings.TrimSpace(attempt.AttemptID)=="" { return nil }
    at:=eventObservedTime(firstNonEmptyRuntime(attempt.EndedAt,attempt.LastObservedAt),now)
    path:=executionHistoryPathAt(project,at)
    if err:=os.MkdirAll(filepath.Dir(path),0700); err!=nil { return err }
    record:=HistoryRecord{Version:1,ArchivedAt:now.UTC().Format(time.RFC3339Nano),Attempt:attempt}
    payload,err:=json.Marshal(record); if err!=nil { return err }
    payload=append(payload,'\n')
    file,err:=os.OpenFile(path,os.O_CREATE|os.O_WRONLY|os.O_APPEND,0600); if err!=nil { return err }
    _,writeErr:=file.Write(payload); closeErr:=file.Close()
    if writeErr!=nil { return writeErr }
    return closeErr
}

func readHistoryAttempts(project string) (map[string]Attempt,error) {
    files,err:=historyFiles(project); if err!=nil { return nil,err }
    out:=map[string]Attempt{}
    for _,path:=range files {
        file,err:=os.Open(path); if err!=nil { return nil,err }
        scanner:=bufio.NewScanner(file)
        scanner.Buffer(make([]byte,64*1024),2*1024*1024)
        for scanner.Scan() {
            line:=strings.TrimSpace(scanner.Text()); if line=="" { continue }
            var record HistoryRecord
            if err:=json.Unmarshal([]byte(line),&record); err!=nil { continue }
            attempt:=record.Attempt
            if strings.TrimSpace(attempt.AttemptID)=="" || !attempt.Terminal { continue }
            previous,ok:=out[attempt.AttemptID]
            if !ok || firstNonEmptyRuntime(attempt.LastObservedAt,attempt.EndedAt)>firstNonEmptyRuntime(previous.LastObservedAt,previous.EndedAt) {
                out[attempt.AttemptID]=attempt
            }
        }
        scanErr:=scanner.Err()
        closeErr:=file.Close()
        if scanErr!=nil { return nil,scanErr }
        if closeErr!=nil { return nil,closeErr }
    }
    return out,nil
}

func mergeTerminalHistory(history map[string]Attempt, current Ledger) map[string]Attempt {
    merged:=map[string]Attempt{}
    for id,attempt:=range history { merged[id]=attempt }
    for _,attempt:=range current.Attempts {
        if attempt.Terminal {
            merged[attempt.AttemptID]=attempt
        } else {
            // A runtime identity may resume after a terminal event. A stale
            // historical terminal summary must not make a resumed attempt
            // appear completed.
            delete(merged,attempt.AttemptID)
        }
    }
    return merged
}

func terminalSortTime(attempt Attempt) string {
    return firstNonEmptyRuntime(attempt.EndedAt,attempt.LastObservedAt,attempt.LastActivityAt,attempt.StartedAt)
}

func terminalHistoryAttempts(project string,current Ledger,now time.Time) ([]Attempt,error) {
    history,err:=readHistoryAttempts(project); if err!=nil { return nil,err }
    merged:=mergeTerminalHistory(history,current)
    cutoff:=now.AddDate(0,0,-historyRetentionDays)
    rows:=make([]Attempt,0,len(merged))
    for _,attempt:=range merged {
        at:=eventObservedTime(terminalSortTime(attempt),time.Time{})
        if !at.IsZero() && at.Before(cutoff) { continue }
        rows=append(rows,attempt)
    }
    sort.Slice(rows,func(i,j int)bool {
        if terminalSortTime(rows[i])!=terminalSortTime(rows[j]) {
            return terminalSortTime(rows[i])>terminalSortTime(rows[j])
        }
        return rows[i].AttemptID<rows[j].AttemptID
    })
    if len(rows)>historyMaxAttempts { rows=rows[:historyMaxAttempts] }
    return rows,nil
}

type ProviderObservation struct {
    Activity bool `json:"activity"`
    Start bool `json:"start"`
    Stop bool `json:"stop"`
    LastObservedAt string `json:"last_observed_at,omitempty"`
}

func ExecutionHistoryStats(project string,current Ledger,now time.Time) (int,map[string]ProviderObservation,error) {
    rows,err:=terminalHistoryAttempts(project,current,now)
    if err!=nil { return 0,nil,err }
    observations:=map[string]ProviderObservation{}
    for _,attempt:=range rows {
        provider:=strings.ToLower(strings.TrimSpace(attempt.Provider))
        if provider=="" { continue }
        item:=observations[provider]
        if attempt.ActivityCount>0 { item.Activity=true }
        if attempt.StartedAt!="" { item.Start=true }
        if attempt.EndedAt!="" { item.Stop=true }
        if attempt.LastObservedAt>item.LastObservedAt { item.LastObservedAt=attempt.LastObservedAt }
        observations[provider]=item
    }
    for _,attempt:=range current.Attempts {
        provider:=strings.ToLower(strings.TrimSpace(attempt.Provider))
        if provider=="" { continue }
        item:=observations[provider]
        if attempt.ActivityCount>0 { item.Activity=true }
        if attempt.StartedAt!="" { item.Start=true }
        if attempt.EndedAt!="" { item.Stop=true }
        if attempt.LastObservedAt>item.LastObservedAt { item.LastObservedAt=attempt.LastObservedAt }
        observations[provider]=item
    }
    return len(rows),observations,nil
}

func QueryExecutionHistory(project string,current Ledger,page,pageSize int,provider,state string,now time.Time) (HistoryPage,error) {
    rows,err:=terminalHistoryAttempts(project,current,now); if err!=nil { return HistoryPage{},err }
    provider=strings.ToLower(strings.TrimSpace(provider))
    state=strings.ToLower(strings.TrimSpace(state))
    filtered:=make([]Attempt,0,len(rows))
    for _,attempt:=range rows {
        if provider!="" && strings.ToLower(attempt.Provider)!=provider { continue }
        if state!="" && strings.ToLower(string(attempt.CurrentState))!=state { continue }
        filtered=append(filtered,attempt)
    }

    if page<1 { page=1 }
    if pageSize<1 { pageSize=defaultHistoryPage }
    if pageSize>maxHistoryPage { pageSize=maxHistoryPage }
    total:=len(filtered)
    pages:=0
    if total>0 { pages=(total+pageSize-1)/pageSize }
    if pages>0 && page>pages { page=pages }
    start:=(page-1)*pageSize
    if start<0 { start=0 }
    if start>total { start=total }
    end:=start+pageSize
    if end>total { end=total }
    items:=append([]Attempt{},filtered[start:end]...)
    return HistoryPage{
        Items:items,Page:page,PageSize:pageSize,Total:total,TotalPages:pages,
        Provider:provider,State:state,
        Retention:map[string]any{
            "raw_days":rawRetentionDays,
            "history_days":historyRetentionDays,
            "history_max_attempts":historyMaxAttempts,
        },
    },nil
}

func VisibleWorkloadAttempts(ledger Ledger,recentTerminalLimit int) []Attempt {
    if recentTerminalLimit<0 { recentTerminalLimit=0 }
    active:=[]Attempt{}
    terminal:=[]Attempt{}
    for _,attempt:=range ledger.Attempts {
        if attempt.Terminal { terminal=append(terminal,attempt) } else { active=append(active,attempt) }
    }
    if len(terminal)>recentTerminalLimit { terminal=terminal[:recentTerminalLimit] }
    return append(active,terminal...)
}

func FilterFindingsForAttempts(findings []LedgerFinding,attempts []Attempt) []LedgerFinding {
    allowed:=map[string]bool{}
    for _,attempt:=range attempts { allowed[attempt.AttemptID]=true }
    out:=[]LedgerFinding{}
    for _,finding:=range findings {
        if finding.AttemptID=="" || allowed[finding.AttemptID] { out=append(out,finding) }
    }
    return out
}

func MaintainExecutionHistory(project string,ledger Ledger,now time.Time) (RetentionStats,error) {
    executionStorageMu.Lock()
    defer executionStorageMu.Unlock()

    stats:=RetentionStats{}
    history,err:=readHistoryAttempts(project); if err!=nil { return stats,err }

    for _,attempt:=range ledger.Attempts {
        if !attempt.Terminal { continue }
        previous,exists:=history[attempt.AttemptID]
        if exists && terminalSortTime(previous)>=terminalSortTime(attempt) { continue }
        if err:=appendHistoryAttempt(project,attempt,now); err!=nil { return stats,err }
        history[attempt.AttemptID]=attempt
        stats.SummariesAdded++
    }

    active:=map[string]bool{}
    for _,attempt:=range ledger.Attempts {
        if !attempt.Terminal { active[attempt.AttemptID]=true }
    }

    entries,err:=os.ReadDir(ExecutionRawDir(project))
    if err!=nil && !errors.Is(err,os.ErrNotExist) { return stats,err }
    rawCutoff:=now.UTC().AddDate(0,0,-rawRetentionDays)
    for _,entry:=range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(),".jsonl") { continue }
        dayText:=strings.TrimSuffix(entry.Name(),".jsonl")
        day,parseErr:=time.Parse("2006-01-02",dayText)
        if parseErr!=nil || !day.Add(24*time.Hour).Before(rawCutoff) { continue }
        path:=filepath.Join(ExecutionRawDir(project),entry.Name())
        ids,scanErr:=attemptIDsInEventFile(path)
        if scanErr!=nil { continue }
        safe:=true
        for id:=range ids {
            if active[id] { safe=false; break }
            if _,ok:=history[id]; !ok { safe=false; break }
        }
        if safe {
            if err:=os.Remove(path); err==nil { stats.RawFilesRemoved++ }
        }
    }

    retained,removed,err:=historyRetentionRecords(project,ledger,now)
    if err!=nil { return stats,err }
    if removed>0 {
        existing,filesErr:=historyFiles(project)
        if filesErr!=nil { return stats,filesErr }
        rewritten,writeErr:=writeHistoryRecords(project,retained)
        if writeErr!=nil { return stats,writeErr }
        stats.HistoryFilesRemoved=len(existing)-rewritten
        if stats.HistoryFilesRemoved<0 { stats.HistoryFilesRemoved=0 }
    }
    return stats,nil
}

func attemptIDsInEventFile(path string) (map[string]bool,error) {
    file,err:=os.Open(path); if err!=nil { return nil,err }
    defer file.Close()
    ids:=map[string]bool{}
    scanner:=bufio.NewScanner(file)
    scanner.Buffer(make([]byte,64*1024),2*1024*1024)
    for scanner.Scan() {
        var event ExecutionEvent
        if err:=json.Unmarshal(scanner.Bytes(),&event); err!=nil { continue }
        if event.AttemptID!="" { ids[event.AttemptID]=true }
    }
    return ids,scanner.Err()
}

var retentionMu sync.Mutex
var retentionLast=map[string]time.Time{}

func MaybeMaintainExecutionHistory(project string,ledger Ledger,now time.Time) (RetentionStats,error) {
    retentionMu.Lock()
    last:=retentionLast[project]
    if !last.IsZero() && now.Sub(last)<time.Minute {
        retentionMu.Unlock()
        return RetentionStats{},nil
    }
    retentionLast[project]=now
    retentionMu.Unlock()
    stats,err:=MaintainExecutionHistory(project,ledger,now)
    if err!=nil {
        retentionMu.Lock()
        delete(retentionLast,project)
        retentionMu.Unlock()
    }
    return stats,err
}

func RetentionPolicy() map[string]int {
    return map[string]int{
        "raw_days":rawRetentionDays,
        "history_days":historyRetentionDays,
        "history_max_attempts":historyMaxAttempts,
        "default_page_size":defaultHistoryPage,
        "max_page_size":maxHistoryPage,
    }
}

func parseHistoryPageValue(value string,fallback int) int {
    value=strings.TrimSpace(value)
    if value=="" { return fallback }
    parsed,err:=strconv.Atoi(value)
    if err!=nil || parsed<1 { return fallback }
    return parsed
}

func FormatRetentionPolicy() string {
    return fmt.Sprintf("raw %dd · history %dd · max %d attempts",rawRetentionDays,historyRetentionDays,historyMaxAttempts)
}
