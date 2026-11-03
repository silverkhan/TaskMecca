package activity

import (
    "errors"
    "os"
    "strings"
    "time"
)

// Diagnostics is a metadata-only, explicitly requested inventory. It does
// not parse the ledger, execute Git, inspect process CPU, or mutate evidence.
type Diagnostics struct {
    EventFiles int `json:"event_files"`
    EventBytes int64 `json:"event_bytes"`
    OtherFiles int `json:"other_files"`
    AverageEventBytes int64 `json:"avg_event_bytes"`
    MetadataScanWallMillis float64 `json:"metadata_scan_wall_ms"`
    ReadCacheEligible bool `json:"read_cache_eligible"`
    Measurement string `json:"measurement"`
}
func(l Ledger) Diagnostics()(Diagnostics,error){
    started:=time.Now()
    out:=Diagnostics{Measurement:"filesystem metadata and wall-clock scan time, not CPU or battery drain"}
    if strings.TrimSpace(l.Project)==""{return out,errors.New("missing canonical project")}
    entries,err:=os.ReadDir(l.dir())
    if errors.Is(err,os.ErrNotExist){return out,nil}
    if err!=nil{return out,err}
    for _,entry:=range entries{
        if entry.IsDir() {out.OtherFiles++;continue}
        if !strings.HasSuffix(entry.Name(),".json")||!validEventID(strings.TrimSuffix(entry.Name(),".json")){
            out.OtherFiles++;continue
        }
        info,infoErr:=entry.Info()
        if infoErr!=nil{return out,infoErr}
        if !info.Mode().IsRegular(){out.OtherFiles++;continue}
        out.EventFiles++
        out.EventBytes+=info.Size()
    }
    if out.EventFiles>0{out.AverageEventBytes=out.EventBytes/int64(out.EventFiles)}
    out.ReadCacheEligible=out.EventFiles<=projectionMaxEvents
    out.MetadataScanWallMillis=float64(time.Since(started).Microseconds())/1000
    return out,nil
}
