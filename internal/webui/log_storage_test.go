package webui

import (
    "os"
    "path/filepath"
    "testing"
)

func TestLogStorageReportAndSelectiveClear(t *testing.T) {
    t.Setenv("TASK_MECCA_HOME",t.TempDir())
    project:=t.TempDir()
    runtime:=filepath.Join(project,"_task_mecca",".runtime")
    files:=map[string]string{
        "web-service":WebLogPath(),
        "hook-diagnostics":filepath.Join(runtime,"observability-spike","events.jsonl"),
        "execution-ledger":filepath.Join(runtime,"executions","events.jsonl"),
        "notification-events":filepath.Join(runtime,"notification_events.json"),
        "lifecycle-observations":filepath.Join(runtime,"lifecycle_observations.json"),
    }
    for id,path:=range files {
        if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil { t.Fatal(err) }
        if err:=os.WriteFile(path,[]byte(id+" evidence"),0600);err!=nil { t.Fatal(err) }
    }
    report,err:=LogStorageReport(project)
    if err!=nil { t.Fatal(err) }
    items:=report["items"].([]LogStorageItem)
    if len(items)!=5 { t.Fatalf("got %d types",len(items)) }
    sizes:=int64(0)
    protected:=0
    for _,item:=range items {
        if !item.Exists||item.SizeBytes<=0 { t.Fatalf("bad item: %+v",item) }
        sizes+=item.SizeBytes
        if !item.CanClear { protected++ }
    }
    if protected!=3||report["total_bytes"]!=sizes { t.Fatalf("report=%+v",report) }
    if _,err=ClearLogStorage(project,"execution-ledger");err==nil { t.Fatal("protected ledger cleared") }
    if _,err=ClearLogStorage(project,"../../private");err==nil { t.Fatal("arbitrary path accepted") }
    before,err:=ClearLogStorage(project,"web-service")
    if err!=nil||before<=0 { t.Fatalf("web clear: %d %v",before,err) }
    after,err:=os.Stat(files["web-service"])
    if err!=nil||after.Size()!=0 { t.Fatalf("web log not truncated: %v %v",after,err) }
    if _,err=ClearLogStorage(project,"hook-diagnostics");err!=nil { t.Fatal(err) }
    for _,id:=range []string{"execution-ledger","notification-events","lifecycle-observations"} {
        data,err:=os.ReadFile(files[id])
        if err!=nil||string(data)!=id+" evidence" { t.Fatalf("protected %s changed: %s %v",id,data,err) }
    }
    report,err=LogStorageReport(project)
    if err!=nil||report["reclaimable_bytes"]!=int64(0) { t.Fatalf("remaining reclaimable bytes: %+v %v",report,err) }
}

func TestLogStorageMissingAndSymlinkAreSafe(t *testing.T) {
    t.Setenv("TASK_MECCA_HOME",t.TempDir())
    project:=t.TempDir()
    report,err:=LogStorageReport(project)
    if err!=nil { t.Fatal(err) }
    if report["total_bytes"]!=int64(0) { t.Fatalf("missing logs: %+v",report) }
    if n,err:=ClearLogStorage(project,"hook-diagnostics");err!=nil||n!=0 {
        t.Fatalf("missing clear: %d %v",n,err)
    }
    outside:=filepath.Join(t.TempDir(),"outside-data")
    if err:=os.WriteFile(outside,[]byte("keep me"),0600);err!=nil { t.Fatal(err) }
    log:=WebLogPath()
    if err:=os.MkdirAll(filepath.Dir(log),0700);err!=nil { t.Fatal(err) }
    if err:=os.Symlink(outside,log);err!=nil { t.Skipf("symlink unavailable: %v",err) }
    if _,err:=ClearLogStorage(project,"web-service");err==nil { t.Fatal("symlink was cleared") }
    raw,err:=os.ReadFile(outside)
    if err!=nil||string(raw)!="keep me" { t.Fatalf("outside altered: %q %v",raw,err) }
}

func TestLogStorageTruncateKeepsWriterDescriptor(t *testing.T) {
    t.Setenv("TASK_MECCA_HOME",t.TempDir())
    project:=t.TempDir()
    path:=WebLogPath()
    if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil { t.Fatal(err) }
    writer,err:=os.OpenFile(path,os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600)
    if err!=nil { t.Fatal(err) }
    defer writer.Close()
    if _,err=writer.WriteString("first log\n");err!=nil { t.Fatal(err) }
    if _,err=ClearLogStorage(project,"web-service");err!=nil { t.Fatal(err) }
    if _,err=writer.WriteString("new log\n");err!=nil { t.Fatal(err) }
    data,err:=os.ReadFile(path)
    if err!=nil||string(data)!="new log\n" { t.Fatalf("append fd invalid: %q %v",data,err) }
}
