package activity

import (
    "errors"
    "os"
    "path/filepath"
    "sync"
    "testing"
)

func TestLedgerAppendReadAndConflict(t *testing.T){
    l:=Ledger{Project:t.TempDir()}
    e:=validEvent()
    created,err:=l.Append(e);if err!=nil||!created{t.Fatalf("append %v %v",created,err)}
    created,err=l.Append(e);if err!=nil||created{t.Fatalf("duplicate %v %v",created,err)}
    got,err:=l.Read(e.EventID);if err!=nil||got.TaskID!=e.TaskID{t.Fatalf("read %v %v",got,err)}
    e.Explanation="changed source"
    if _,err=l.Append(e);!errors.Is(err,ErrEventConflict){t.Fatalf("expected conflict, got %v",err)}
    events,err:=l.List("B-589");if err!=nil||len(events)!=1{t.Fatalf("list %v %v",events,err)}
    none,err:=l.List("B-999");if err!=nil||len(none)!=0{t.Fatalf("filter %v %v",none,err)}
}

func TestLedgerRejectsTraversalAndPartialFile(t *testing.T){
    l:=Ledger{Project:t.TempDir()}
    e:=validEvent();e.EventID="../../attack"
    if _,err:=l.Append(e);err==nil{t.Fatal("unsafe ID accepted")}
    e.EventID="evt-corrupt"
    target:=filepath.Join(l.dir(),"evt-corrupt.json")
    if err:=os.MkdirAll(filepath.Dir(target),0700);err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(target,[]byte("{"),0600);err!=nil{t.Fatal(err)}
    if _,err:=l.Read(e.EventID);err==nil{t.Fatal("corrupt event accepted")}
}

func TestLedgerConcurrentIdempotency(t *testing.T){
    l:=Ledger{Project:t.TempDir()};e:=validEvent()
    const n=16
    var wg sync.WaitGroup
    results:=make(chan bool,n)
    for i:=0;i<n;i++ {wg.Add(1);go func(){defer wg.Done();created,err:=l.Append(e);if err!=nil{t.Errorf("append: %v",err)};results<-created}()}
    wg.Wait();close(results)
    count:=0;for created:=range results{if created{count++}}
    if count!=1{t.Fatalf("expected 1 publication, got %d",count)}
}
