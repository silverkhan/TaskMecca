package activity

import "testing"

func TestListCachedInvalidatesOnSameProcessAppend(t *testing.T){
 l:=Ledger{Project:t.TempDir()}
 initial,err:=l.ListCached("B-589")
 if err!=nil||len(initial)!=0{t.Fatalf("initial %v %v",initial,err)}
 e:=validEvent()
 if ok,err:=l.Append(e);err!=nil||!ok{t.Fatalf("append %v %v",ok,err)}
 all,err:=l.ListCached("B-589")
 if err!=nil||len(all)!=1{t.Fatalf("cache did not invalidate %v %v",all,err)}
 empty,err:=l.ListCached("B-999")
 if err!=nil||len(empty)!=0{t.Fatalf("cache task filter %v %v",empty,err)}
}
func BenchmarkListCached(b *testing.B){
 l:=Ledger{Project:b.TempDir()}
 e:=validEvent()
 if _,err:=l.Append(e);err!=nil{b.Fatal(err)}
 b.ResetTimer()
 for i:=0;i<b.N;i++{if _,err:=l.ListCached(e.TaskID);err!=nil{b.Fatal(err)}}
}
