package webui

import (
    "encoding/json"
    "os"
    "path/filepath"
    "time"

    "github.com/silverkhan/TaskMecca/internal/projectguard"
)

// A no-change scan only needs to advance the durable observation heartbeat.
// Keep the immutable incident/stage history in session-monitor.json; never
// discard it or infer a runtime termination from a heartbeat.
type operationScanHeartbeat struct {
    LastScanAt string `json:"last_scan_at"`
    BaseScanAt string `json:"base_scan_at"`
    JournalSize int64 `json:"journal_size"`
    JournalModified int64 `json:"journal_modified_ns"`
}

func operationHeartbeatPath(project string) string {
    return filepath.Join(project,"_task_mecca",".runtime","operations","last-scan.json")
}

func operationHeartbeatTime(project string, base string) string {
    data,err:=os.ReadFile(operationHeartbeatPath(project))
    if err!=nil {return base}
    var h operationScanHeartbeat
    if json.Unmarshal(data,&h)!=nil || h.BaseScanAt!=base {return base}
    main,err:=os.Stat(operationJournalPath(project))
    if err!=nil || !main.Mode().IsRegular() || main.Size()!=h.JournalSize || main.ModTime().UnixNano()!=h.JournalModified {return base}
    heartbeatAt,err:=time.Parse(time.RFC3339Nano,h.LastScanAt)
    if err!=nil {return base}
    baseAt,_:=time.Parse(time.RFC3339Nano,base)
    if heartbeatAt.After(baseAt) {return h.LastScanAt}
    return base
}

func writeOperationHeartbeat(project,base,latest string) error {
    guard,err:=projectguard.AcquireWrite(project)
    if err!=nil {return err}
    defer guard()

    main,err:=os.Stat(operationJournalPath(project))
    if err!=nil {return err}
    h:=operationScanHeartbeat{
        LastScanAt:latest,BaseScanAt:base,
        JournalSize:main.Size(),JournalModified:main.ModTime().UnixNano(),
    }
    data,err:=json.Marshal(h)
    if err!=nil {return err}
    path:=operationHeartbeatPath(project)
    tmp,err:=os.CreateTemp(filepath.Dir(path),".last-scan-*")
    if err!=nil {return err}
    defer os.Remove(tmp.Name())
    if _,err:=tmp.Write(append(data,'\n'));err!=nil{tmp.Close();return err}
    if err:=tmp.Sync();err!=nil{tmp.Close();return err}
    if err:=tmp.Close();err!=nil{return err}
    return os.Rename(tmp.Name(),path)
}

// Clone before reconciling: the scan updates incident strings in place.
// The resolution pointers are also copied to prevent future mutation of
// nested evidence from falsely treating a changed journal as unchanged.
func snapshotOperationJournal(j operationJournal) operationJournal {
    out:=j
    out.Incidents=append([]operationIncident(nil),j.Incidents...)
    for i:=range out.Incidents {
        if p:=out.Incidents[i].Resolution;p!=nil {
            copy:=*p;out.Incidents[i].Resolution=&copy
        }
        if p:=out.Incidents[i].PolicyResolution;p!=nil {
            copy:=*p;out.Incidents[i].PolicyResolution=&copy
        }
    }
    out.Stages=append([]operationStage(nil),j.Stages...)
    return out
}
