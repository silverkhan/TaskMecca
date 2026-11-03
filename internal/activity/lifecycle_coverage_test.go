package activity

import (
 "testing"
 "time"
)
func TestLifecycleCoverageSeparatesWaitFromObservedWork(t *testing.T){
 base:=time.Date(2026,11,4,0,0,0,0,time.UTC)
 mk:=func(id,kind string,min int)LifecycleFact{
  return LifecycleFact{TaskID:"B-1",EventID:id,Kind:kind,OccurredAt:base.Add(time.Duration(min)*time.Minute),EvidenceSource:"controller_report"}
 }
 facts:=[]LifecycleFact{
  mk("completed","completed",60),mk("waiting","waiting",25),mk("registered","registered",0),
  mk("assigned","assigned",3),mk("started","started",10),mk("resumed","resumed",40),
  mk("assigned","assigned",3), // identical replay must not inflate duration
 }
 segments:=[]Segment{
  {TaskID:"B-1",Type:Implementation,StartedAt:base.Add(10*time.Minute),EndedAt:base.Add(25*time.Minute)},
  {TaskID:"B-1",Type:Implementation,StartedAt:base.Add(40*time.Minute),EndedAt:base.Add(55*time.Minute)},
 }
 out:=SummarizeLifecycle("B-1",facts,segments)
 if out.TotalElapsedMillis==nil||*out.TotalElapsedMillis!=3600000{t.Fatalf("total %+v",out)}
 if out.RegistrationQueueMillis!=180000||out.DispatchWaitMillis!=420000||out.ExplicitWaitMillis!=900000 {
  t.Fatalf("waiting classification %+v",out)
 }
 if out.WaitMillis!=25*60000 || out.CoveredElapsedMillis!=55*60000 ||
    out.UnattributedElapsedMillis==nil||*out.UnattributedElapsedMillis!=5*60000||out.OverlapMillis!=0 {
  t.Fatalf("coverage %+v",out)
 }
}
func TestLifecycleOverlapDoesNotDoubleCount(t *testing.T){
 base:=time.Date(2026,11,4,0,0,0,0,time.UTC)
 makeFact:=func(kind string,mins int)LifecycleFact{return LifecycleFact{TaskID:"B-2",EventID:kind,Kind:kind,OccurredAt:base.Add(time.Duration(mins)*time.Minute)}}
 facts:=[]LifecycleFact{makeFact("registered",0),makeFact("assigned",5),makeFact("started",10),makeFact("completed",20)}
 seg:=[]Segment{{TaskID:"B-2",StartedAt:base.Add(3*time.Minute),EndedAt:base.Add(8*time.Minute)}}
 out:=SummarizeLifecycle("B-2",facts,seg)
 if out.OverlapMillis!=5*60000 || out.CoveredElapsedMillis!=10*60000 {
  t.Fatalf("expected overlap=5min and coverage=10min: %+v",out)
 }
 if out.UnattributedElapsedMillis==nil||*out.UnattributedElapsedMillis!=10*60000{t.Fatalf("unknown coverage %+v",out)}
}
func TestLifecycleUnfinishedKeepsUnknownBoundary(t *testing.T){
 base:=time.Date(2026,11,4,0,0,0,0,time.UTC)
 out:=SummarizeLifecycle("B-3",[]LifecycleFact{
  {TaskID:"B-3",EventID:"waiting",Kind:"waiting",OccurredAt:base},
 },nil)
 if out.TotalElapsedMillis!=nil||out.UnattributedElapsedMillis!=nil||out.OpenWaitCount!=1{
  t.Fatalf("open lifecycle must not invent elapsed time: %+v",out)
 }
}
