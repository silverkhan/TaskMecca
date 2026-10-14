package runtimeobs

import (
    "os"
    "sync"
)

// Event folds are immutable once published. Only time-dependent projections
// (elapsed time, waiting time, stale findings and recent-transition limits)
// are calculated per request. This avoids replaying an ever-growing JSONL
// history whenever the backlog list or a lifecycle panel is refreshed.
type executionFileFingerprint struct {
    path string
    size int64
    modified int64
}

type executionFoldCacheEntry struct {
    files []executionFileFingerprint
    accumulators map[string]*accumulator
    findings []LedgerFinding
}

var executionFoldCache = struct {
    sync.Mutex
    entries map[string]executionFoldCacheEntry
}{entries:map[string]executionFoldCacheEntry{}}

const executionFoldCacheMaxProjects = 8

func executionFileFingerprints(files []string) ([]executionFileFingerprint, bool) {
    result:=make([]executionFileFingerprint,0,len(files))
    for _,path:=range files {
        info,err:=os.Stat(path)
        if err!=nil || !info.Mode().IsRegular() { return nil,false }
        result=append(result,executionFileFingerprint{path:path,size:info.Size(),modified:info.ModTime().UnixNano()})
    }
    return result,true
}

func sameExecutionFileFingerprints(a,b []executionFileFingerprint) bool {
    if len(a)!=len(b) { return false }
    for i:=range a { if a[i]!=b[i] { return false } }
    return true
}

func cachedExecutionAccumulators(project string, files []executionFileFingerprint, valid bool) (map[string]*accumulator,[]LedgerFinding,bool) {
    if !valid { return nil,nil,false }
    executionFoldCache.Lock()
    defer executionFoldCache.Unlock()
    snapshot,ok:=executionFoldCache.entries[project]
    if !ok || !sameExecutionFileFingerprints(snapshot.files,files) { return nil,nil,false }
    return snapshot.accumulators,append([]LedgerFinding(nil),snapshot.findings...),true
}

func rememberExecutionAccumulators(project string, files []executionFileFingerprint, accs map[string]*accumulator, findings []LedgerFinding) {
    executionFoldCache.Lock()
    defer executionFoldCache.Unlock()
    if len(executionFoldCache.entries)>=executionFoldCacheMaxProjects {
        if _,exists:=executionFoldCache.entries[project];!exists {
            for key:=range executionFoldCache.entries { delete(executionFoldCache.entries,key);break }
        }
    }
    executionFoldCache.entries[project]=executionFoldCacheEntry{
        files:append([]executionFileFingerprint(nil),files...),
        accumulators:accs,
        findings:append([]LedgerFinding(nil),findings...),
    }
}
