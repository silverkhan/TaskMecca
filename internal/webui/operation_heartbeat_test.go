package webui

import (
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestOperationNoChangeScanUsesDurableHeartbeat(t *testing.T) {
    project:=testOperationProject(t)
    firstTime:=time.Now().UTC().Truncate(time.Second)
    first,err:=scanOperationProject(project,firstTime)
    if err!=nil{t.Fatal(err)}
    path:=operationJournalPath(project)
    original,err:=os.Stat(path)
    if err!=nil{t.Fatal(err)}
    secondTime:=firstTime.Add(15*time.Second)
    second,err:=scanOperationProject(project,secondTime)
    if err!=nil{t.Fatal(err)}
    same,err:=os.Stat(path)
    if err!=nil{t.Fatal(err)}
    if !os.SameFile(original,same) || original.Size()!=same.Size() || !original.ModTime().Equal(same.ModTime()){
        t.Fatal("no-change scan rewrote large historical journal")
    }
    if _,err:=os.Stat(operationHeartbeatPath(project));err!=nil{t.Fatalf("heartbeat not persisted: %v",err)}
    persisted,err:=readOperationJournal(project)
    if err!=nil{t.Fatal(err)}
    if persisted.LastScanAt!=second.LastScanAt || persisted.LastScanAt==first.LastScanAt{
        t.Fatalf("scan time lost across read: first=%q second=%q persisted=%q",first.LastScanAt,second.LastScanAt,persisted.LastScanAt)
    }
    // New monitoring-gap evidence is not a no-op, and must rewrite the
    // historical journal without letting an older heartbeat override it.
    thirdTime:=secondTime.Add(operationGapAfter+time.Second)
    third,err:=scanOperationProject(project,thirdTime)
    if err!=nil{t.Fatal(err)}
    after,err:=os.Stat(path);if err!=nil{t.Fatal(err)}
    if os.SameFile(same,after){t.Fatal("evidence change was not durably persisted")}
    reloaded,err:=readOperationJournal(project);if err!=nil{t.Fatal(err)}
    if reloaded.LastScanAt!=third.LastScanAt || len(reloaded.Incidents)==0{
        t.Fatalf("gap evidence or last scan lost after rewrite: %+v",reloaded)
    }
}

func TestOperationHeartbeatFailsClosedOnJournalReplacement(t *testing.T) {
    project:=testOperationProject(t)
    first:=time.Now().UTC().Truncate(time.Second)
    if _,err:=scanOperationProject(project,first);err!=nil{t.Fatal(err)}
    if _,err:=scanOperationProject(project,first.Add(15*time.Second));err!=nil{t.Fatal(err)}
    path:=operationJournalPath(project)
    // External atomic replacement with an older source cannot trust the
    // heartbeat associated with the original journal.
    raw,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
    tmp:=path+".external"
    if err:=os.WriteFile(tmp,append(raw, ' '),0644);err!=nil{t.Fatal(err)}
    if err:=os.Rename(tmp,path);err!=nil{t.Fatal(err)}
    observed,err:=readOperationJournal(project);if err!=nil{t.Fatal(err)}
    if observed.LastScanAt!=first.Format(time.RFC3339Nano){
        t.Fatalf("heartbeat applied to external journal: %q",observed.LastScanAt)
    }
    if err:=os.WriteFile(operationHeartbeatPath(project),[]byte("invalid json"),0644);err!=nil{t.Fatal(err)}
    reloaded,err:=readOperationJournal(project);if err!=nil{t.Fatal(err)}
    if reloaded.LastScanAt!=observed.LastScanAt{t.Fatal("corrupt heartbeat affected operation evidence")}
    if _,err:=os.Stat(filepath.Dir(path));err!=nil{t.Fatal(err)}
}
