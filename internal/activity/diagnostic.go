package activity

import (
    "errors"
    "io"
    "os"
    "strings"
    "time"
)

const diagnosticSampleFiles=128

// Diagnostic describes bounded metadata inspection of Activity storage.
// It never parses event bodies, runs tests, or measures CPU/battery drain.
// Event count is a LOWER BOUND when Truncated is true.
type Diagnostic struct {
    EventFilesObserved int `json:"event_files_observed"`
    ScanLimit int `json:"scan_limit"`
    Truncated bool `json:"truncated"`
    SampledFiles int `json:"sampled_files"`
    SampledBytes int64 `json:"sampled_bytes"`
    ScanElapsedMS float64 `json:"scan_elapsed_ms"`
    Measurement string `json:"measurement"`
}

// Diagnose limits directory walk and per-file size-stat calls. It is
// explicitly invoked by an operator, never run on every Web refresh or Hook.
func (l Ledger) Diagnose(limit int)(Diagnostic,error){
    if limit<=0 || limit>50000{limit=5000}
    report:=Diagnostic{ScanLimit:limit,Measurement:"bounded directory metadata and wall-clock scan; not CPU, battery or total disk size"}
    started:=time.Now()
    defer func(){report.ScanElapsedMS=float64(time.Since(started).Microseconds())/1000}()
    file,err:=os.Open(l.dir())
    if errors.Is(err,os.ErrNotExist){return report,nil}
    if err!=nil{return report,err}
    defer file.Close()
    for report.EventFilesObserved<limit{
        batch,readErr:=file.ReadDir(256)
        if readErr!=nil && !errors.Is(readErr,io.EOF){return report,readErr}
        for _,entry:=range batch{
            if entry.IsDir()||!strings.HasSuffix(entry.Name(),".json"){continue}
            report.EventFilesObserved++
            if report.SampledFiles<diagnosticSampleFiles{
                info,statErr:=entry.Info()
                if statErr!=nil{return report,statErr}
                report.SampledFiles++
                report.SampledBytes+=info.Size()
            }
            if report.EventFilesObserved>=limit{report.Truncated=true;break}
        }
        if errors.Is(readErr,io.EOF){break}
    }
    return report,nil
}
