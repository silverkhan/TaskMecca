package activity

import (
    "errors"
    "fmt"
    "sync"
    "testing"
    "time"
)

func TestApplySerializesConcurrentStarts(t *testing.T) {
    l:=Ledger{Project:t.TempDir()}
    const total=12
    var wg sync.WaitGroup
    results:=make(chan bool,total)
    for i:=0;i<total;i++ {
        wg.Add(1)
        go func(i int){
            defer wg.Done()
            e:=validEvent()
            e.EventID=fmt.Sprintf("start-%d",i)
            inserted,err:=l.Apply(e)
            if err!=nil && !errors.Is(err,ErrInvalidEvent){t.Errorf("apply: %v",err)}
            results<-inserted
        }(i)
    }
    wg.Wait();close(results)
    count:=0
    for x:=range results{if x{count++}}
    if count!=1{t.Fatalf("concurrent starts published %d entries",count)}
}
func TestApplyMaintainsTransitionAndIdempotency(t *testing.T) {
    l:=Ledger{Project:t.TempDir()}
    start:=validEvent()
    if ok,err:=l.Apply(start);err!=nil||!ok{t.Fatalf("start %v %v",ok,err)}
    if ok,err:=l.Apply(start);err!=nil||ok{t.Fatalf("retry %v %v",ok,err)}
    changed:=start
    changed.Evidence="different"
    if _,err:=l.Apply(changed);!errors.Is(err,ErrEventConflict){t.Fatalf("conflicting retry: %v",err)}
    stop:=start
    stop.EventID="stop-one";stop.Action=Stop;stop.OccurredAt=stop.OccurredAt.Add(time.Second);stop.RecordedAt=stop.OccurredAt
    if ok,err:=l.Apply(stop);err!=nil||!ok{t.Fatalf("stop %v %v",ok,err)}
    if _,err:=l.Apply(stop);err!=nil{t.Fatal(err)}
    stop.EventID="second-stop"
    if _,err:=l.Apply(stop);!errors.Is(err,ErrInvalidEvent){t.Fatalf("double stop: %v",err)}
}
func BenchmarkApplyNewEvent(b *testing.B){
    l:=Ledger{Project:b.TempDir()}
    for i:=0;i<b.N;i++{
       e:=validEvent()
       e.EventID=fmt.Sprintf("event-%d",i)
       e.RuntimeAttemptID=fmt.Sprintf("attempt-%d",i)
       if _,err:=l.Apply(e);err!=nil{b.Fatal(err)}
    }
}
