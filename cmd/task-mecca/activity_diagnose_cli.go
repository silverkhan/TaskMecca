package main

import (
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "io"
 "os"
 "path/filepath"
 "runtime"
 "runtime/pprof"
 "sort"
 "time"

 "github.com/silverkhan/TaskMecca/internal/activity"
)

type activityDiagnosticReport struct {
 Project string `json:"project"`
 Task string `json:"task,omitempty"`
 Samples int `json:"samples"`
 Events int `json:"events"`
 StoredEventFiles int `json:"stored_event_files"`
 StoredBytes int64 `json:"stored_bytes"`
 ReadP50Millis float64 `json:"read_p50_ms"`
 ReadP95Millis float64 `json:"read_p95_ms"`
 ReadMinMillis float64 `json:"read_min_ms"`
 ReadMaxMillis float64 `json:"read_max_ms"`
 HeapBeforeBytes uint64 `json:"process_heap_before_bytes"`
 HeapAfterBytes uint64 `json:"process_heap_after_bytes"`
 CPUProfilePath string `json:"cpu_profile_path,omitempty"`
 Measurement string `json:"measurement"`
 Note string `json:"note"`
}
func diagnosticPercentile(samples []time.Duration,percent int)float64{
 if len(samples)==0{return 0}
 values:=append([]time.Duration(nil),samples...)
 sort.Slice(values,func(i,j int)bool{return values[i]<values[j]})
 rank:=(len(values)*percent+99)/100
 if rank<1{rank=1}
 if rank>len(values){rank=len(values)}
 return float64(values[rank-1].Microseconds())/1000
}
func runActivityDiagnose(args []string,out,errOut io.Writer)int {
 fs:=flag.NewFlagSet("activity diagnose",flag.ContinueOnError)
 fs.SetOutput(errOut)
 project:=fs.String("project",".","project root")
 task:=fs.String("task","","optional backlog ID")
 samples:=fs.Int("samples",5,"read samples (1-10), direct ledger reads only")
 cpuProfile:=fs.String("cpu-profile","","optional Go CPU profile output path")
 jsonOutput:=fs.Bool("json",true,"emit JSON report")
 if err:=fs.Parse(args);err!=nil{return 2}
 if fs.NArg()!=0 || *samples<1||*samples>10{
  fmt.Fprintln(errOut,"--samples must be 1..10 and no positional arguments are accepted")
  return 2
 }
 root,err:=filepath.Abs(*project)
 if err!=nil{fmt.Fprintln(errOut,err);return 2}
 store:=activity.Ledger{Project:root}
 report:=activityDiagnosticReport{
  Project:root,Task:*task,Samples:*samples,
  Measurement:"wall_clock_elapsed_not_cpu_or_battery",
  Note:"Opt-in direct ledger reads. Heap is the entire diagnostic process and not Activity-attributable. Samples are bounded to 10; results depend on filesystem cache and machine load.",
 }
 entries,err:=os.ReadDir(filepath.Join(root,"_task_mecca","data","activity","events"))
 if err!=nil && !errors.Is(err,os.ErrNotExist){fmt.Fprintln(errOut,err);return 1}
 for _,entry:=range entries{
  if entry.IsDir()||filepath.Ext(entry.Name())!=".json"{continue}
  info,statErr:=entry.Info()
  if statErr!=nil{fmt.Fprintln(errOut,statErr);return 1}
  report.StoredEventFiles++
  report.StoredBytes+=info.Size()
 }
 var profile *os.File
 if *cpuProfile!=""{
  profile,err=os.Create(*cpuProfile)
  if err!=nil{fmt.Fprintln(errOut,err);return 1}
  defer profile.Close()
  if err:=pprof.StartCPUProfile(profile);err!=nil{fmt.Fprintln(errOut,err);return 1}
  defer pprof.StopCPUProfile()
  report.CPUProfilePath=*cpuProfile
 }
 var before,after runtime.MemStats
 runtime.ReadMemStats(&before)
 observations:=make([]time.Duration,0,*samples)
 for i:=0;i<*samples;i++{
  started:=time.Now()
  rows,readErr:=store.List(*task)
  observations=append(observations,time.Since(started))
  if readErr!=nil{fmt.Fprintln(errOut,readErr);return 1}
  if i==0{report.Events=len(rows)}
 }
 runtime.ReadMemStats(&after)
 report.HeapBeforeBytes=before.HeapAlloc
 report.HeapAfterBytes=after.HeapAlloc
 report.ReadP50Millis=diagnosticPercentile(observations,50)
 report.ReadP95Millis=diagnosticPercentile(observations,95)
 report.ReadMinMillis=diagnosticPercentile(observations,0)
 report.ReadMaxMillis=diagnosticPercentile(observations,100)
 if *jsonOutput{
  if err:=json.NewEncoder(out).Encode(report);err!=nil{fmt.Fprintln(errOut,err);return 1}
 }else{
  fmt.Fprintf(out,"Activity: %d events, %d files, %d bytes, read p50 %.2f ms, p95 %.2f ms\n",
   report.Events,report.StoredEventFiles,report.StoredBytes,report.ReadP50Millis,report.ReadP95Millis)
 }
 return 0
}
