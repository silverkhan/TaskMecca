package activity

import (
 "sync"
 "strconv"
 "testing"
)

func TestReadProjectionConcurrentReaders(t *testing.T){
 l:=Ledger{Project:t.TempDir()}
 for i:=0;i<100;i++{
  e:=validEvent()
  e.EventID="concurrent-"+fmtIndex(i)
  e.RuntimeAttemptID="attempt-"+fmtIndex(i)
  if _,err:=l.Append(e);err!=nil{t.Fatal(err)}
 }
 var group sync.WaitGroup
 for i:=0;i<24;i++{
  group.Add(1)
  go func(){
    defer group.Done()
    result,err:=l.ListCached("B-589")
    if err!=nil{t.Errorf("cache read: %v",err);return}
    if len(result)!=100{t.Errorf("expected 100 events, got %d",len(result))}
  }()
 }
 group.Wait()
}

func TestReadProjectionDoesNotRetainStaleSnapshotAfterWrite(t *testing.T){
 l:=Ledger{Project:t.TempDir()}
 before,err:=l.ListCached("B-589")
 if err!=nil||len(before)!=0{t.Fatalf("before %d %v",len(before),err)}
 e:=validEvent()
 if _,err:=l.Append(e);err!=nil{t.Fatal(err)}
 after,err:=l.ListCached("B-589")
 if err!=nil||len(after)!=1{t.Fatalf("stale after append %d %v",len(after),err)}
}
func fmtIndex(i int)string {return strconv.Itoa(i)}
