package webui

import (
 "errors"
 "sync"
 "testing"
)

func TestUpgradeProgressTrackerSerializesAndReportsStages(t *testing.T){
 var tr upgradeProgressTracker
 if got:=tr.snapshot();got.Phase!="idle"||got.Active {t.Fatalf("initial=%+v",got)}
 if !tr.begin(){t.Fatal("first update should start")}
 if tr.begin(){t.Fatal("a second update was started")}
 id:=tr.snapshot().ID
 if id=="" {t.Fatal("missing transaction ID")}
 for _,phase:=range []string{"checking","downloading","verifying","installing"} {
  tr.step(phase)
  got:=tr.snapshot()
  if !got.Active||got.Phase!=phase||got.ID!=id {t.Fatalf("phase %s: %+v",phase,got)}
 }
 tr.step("unsafe")
 if tr.snapshot().Phase!="installing"{t.Fatal("unknown stage accepted")}
 tr.finish("restarting",nil)
 if got:=tr.snapshot();!got.Active||got.Phase!="restarting"{t.Fatalf("restart state=%+v",got)}
 if tr.begin(){t.Fatal("duplicate update admitted during restart")}
}

func TestUpgradeProgressTrackerRecoversAfterFailure(t *testing.T){
 var tr upgradeProgressTracker
 tr.begin()
 tr.step("downloading")
 tr.finish("failed",errors.New("network disconnected"))
 got:=tr.snapshot()
 if got.Phase!="failed"||got.Active||got.Error!="network disconnected" {t.Fatalf("failed status=%+v",got)}
 if !tr.begin(){t.Fatal("cannot retry after failure")}
 if tr.snapshot().Error!=""{t.Fatal("old error carried into retry")}
 tr.finish("completed",nil)
 if got=tr.snapshot();got.Phase!="completed"||got.Active{t.Fatalf("completed=%+v",got)}
}

func TestUpgradeProgressTrackerConcurrentBegin(t *testing.T){
 var tr upgradeProgressTracker
 var wg sync.WaitGroup
 successes:=make(chan bool,32)
 for i:=0;i<32;i++{wg.Add(1);go func(){defer wg.Done();successes<-tr.begin()}()}
 wg.Wait();close(successes)
 count:=0;for s:=range successes{if s{count++}}
 if count!=1{t.Fatalf("expected one accepted install, got %d",count)}
}
