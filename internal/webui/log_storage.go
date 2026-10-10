package webui

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
)

// LogStorageItem represents a fixed, application-owned log or event ledger.
// An explicit allowlist prevents a web request from becoming an arbitrary
// filesystem path selector.
type LogStorageItem struct {
    ID string `json:"id"`
    Scope string `json:"scope"`
    Label string `json:"label"`
    SizeBytes int64 `json:"size_bytes"`
    Exists bool `json:"exists"`
    CanClear bool `json:"can_clear"`
    Note string `json:"note"`
    ReleaseCondition string `json:"release_condition"`
    ReclaimableNowBytes int64 `json:"reclaimable_now_bytes"`
    ConditionalReclaimBytes int64 `json:"conditional_reclaim_bytes"`
}

type logStorageTarget struct {
    item LogStorageItem
    path string
}

func logStorageTargets(project string) []logStorageTarget {
    runtimeDir:=filepath.Join(project,"_task_mecca",".runtime")
    return []logStorageTarget{
        {LogStorageItem{ID:"web-service",Scope:"global",Label:"Web service log",CanClear:true,Note:"Diagnostic stdout/stderr; currently running service can write new lines."},WebLogPath()},
        {LogStorageItem{ID:"hook-diagnostics",Scope:"project",Label:"Hook diagnostic events",CanClear:true,Note:"Diagnostic hook capture. Clearing it removes previous observations."},filepath.Join(runtimeDir,"observability-spike","events.jsonl")},
        {LogStorageItem{ID:"execution-ledger",Scope:"project",Label:"Execution event ledger",CanClear:false,Note:"Protected: required for execution history and lifecycle reconstruction."},filepath.Join(runtimeDir,"executions","events.jsonl")},
        {LogStorageItem{ID:"notification-events",Scope:"project",Label:"Notification history",CanClear:false,Note:"Protected: notification deduplication and delivery history."},filepath.Join(runtimeDir,"notification_events.json")},
        {LogStorageItem{ID:"lifecycle-observations",Scope:"project",Label:"Lifecycle observations",CanClear:false,Note:"Protected: lifecycle evidence and backlog state history."},filepath.Join(runtimeDir,"lifecycle_observations.json")},
    }
}

// inspectLogFile uses Lstat so symlinks and nonregular files are never
// followed or truncated. Missing files are represented as zero bytes.
func inspectLogFile(path string) (int64,bool,error) {
    info,err:=os.Lstat(path)
    if errors.Is(err,os.ErrNotExist) { return 0,false,nil }
    if err!=nil { return 0,false,err }
    if !info.Mode().IsRegular() { return 0,true,fmt.Errorf("not a regular log file") }
    return info.Size(),true,nil
}

func LogStorageReport(project string) (map[string]any,error) {
    items:=make([]LogStorageItem,0,5)
    var total int64
    var reclaimable int64
    for _,target:=range logStorageTargets(project) {
        size,exists,err:=inspectLogFile(target.path)
        if err!=nil { return nil,fmt.Errorf("%s: %w",target.item.ID,err) }
        item:=target.item
        item.SizeBytes=size
        item.Exists=exists
        // Protected canonical ledgers have no approved release condition. No
        // portion of their bytes may be advertised as hypothetically reclaimable.
        if item.CanClear {
            item.ReleaseCondition="explicit_manual_clear"
            item.ReclaimableNowBytes=size
        } else {
            item.ReleaseCondition="no_approved_deletion_policy"
        }
        items=append(items,item)
        total+=size
        if item.CanClear { reclaimable+=size }
    }
    return map[string]any{"items":items,"total_bytes":total,"reclaimable_bytes":reclaimable},nil
}

// ClearLogStorage only accepts IDs from the allowlist above. Open without
// O_TRUNC, verify that the file opened is the same regular file discovered by
// Lstat, then truncate the open descriptor. Never remove or replace a file:
// appenders (including launchd) must retain their original descriptor.
func ClearLogStorage(project,id string) (int64,error) {
    var target *logStorageTarget
    for _,candidate:=range logStorageTargets(project) {
        if candidate.item.ID==id {
            copy:=candidate
            target=&copy
            break
        }
    }
    if target==nil { return 0,fmt.Errorf("unknown log category") }
    if !target.item.CanClear { return 0,fmt.Errorf("this event history is protected") }
    before,exists,err:=inspectLogFile(target.path)
    if err!=nil { return 0,err }
    if !exists { return 0,nil }
    file,err:=os.OpenFile(target.path,os.O_WRONLY,0)
    if err!=nil { return 0,err }
    defer file.Close()
    opened,err:=file.Stat()
    if err!=nil { return 0,err }
    onDisk,err:=os.Lstat(target.path)
    if err!=nil { return 0,err }
    if !onDisk.Mode().IsRegular() || !opened.Mode().IsRegular() || !os.SameFile(opened,onDisk) {
        return 0,fmt.Errorf("log file changed during cleanup")
    }
    if err=file.Truncate(0);err!=nil { return 0,err }
    return before,nil
}
