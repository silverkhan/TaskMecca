package activity

import (
 "testing"
 "time"
)
func TestActivityFilterClipsActualIntervalsWithoutDoubleCounting(t *testing.T){
 at:=time.Date(2026,11,4,0,0,0,0,time.UTC)
 seg:=func(role Role,kind Type,round,start,end int)Segment{
  return Segment{TaskID:"B-1",Role:role,Type:kind,Round:round,
   StartedAt:at.Add(time.Duration(start)*time.Minute),EndedAt:at.Add(time.Duration(end)*time.Minute),
   DurationMillis:int64(end-start)*60000}
 }
 full:=Summary{TaskID:"B-1",Segments:[]Segment{
   seg(RoleWorker,Implementation,0,0,10),
   seg(RoleController,ControllerReview,0,5,15),
   seg(RoleWorker,Implementation,1,10,25),
 }}
 result:=FilterSummary(full,ActivityFilter{
    Role:RoleWorker,Type:Implementation,
    Since:at.Add(8*time.Minute),Until:at.Add(20*time.Minute),
 })
 if len(result.Segments)!=2||result.AgentTimeMillis!=12*60000||result.ObservedWallMillis!=12*60000||
    result.InitialWorkerMillis!=2*60000||result.ReworkWorkerMillis!=10*60000{
     t.Fatalf("filtered contributions %+v",result)
 }
 round:=1
 only:=FilterSummary(full,ActivityFilter{Role:RoleWorker,Round:&round})
 if len(only.Segments)!=1||only.AgentTimeMillis!=15*60000{t.Fatalf("round filter %+v",only)}
}
func TestProjectFilteringDoesNotRelabelLifecycleElapsedAsAgentTime(t *testing.T){
 at:=time.Date(2026,11,4,0,0,0,0,time.UTC)
 e:=validEvent();e.TaskID="B-1";e.EventID="filtered-start";e.Type=Implementation;e.OccurredAt=at;e.RecordedAt=at
 end:=e;end.EventID="filtered-stop";end.Action=Stop;end.OccurredAt=at.Add(3*time.Minute);end.RecordedAt=end.OccurredAt
 facts:=map[string][]LifecycleFact{"B-1":{
  {TaskID:"B-1",EventID:"registered",Kind:"registered",OccurredAt:at},
  {TaskID:"B-1",EventID:"completed",Kind:"completed",OccurredAt:at.Add(60*time.Minute)},
 }}
 report:=SummarizeProjectWithFilter([]Event{e,end},facts,ActivityFilter{Role:RoleWorker})
 if !report.FilteredActivity||report.AgentTimeMillis!=3*60000||report.LifecycleTotalElapsedMillis!=60*60000{
  t.Fatalf("filtered project %+v",report)
 }
 if len(report.TopTasks)!=1||report.TopTasks[0].DurationMillis!=3*60000||
  report.TopTasks[0].DurationBasis!="filtered_activity_wall"{
  t.Fatalf("filtered task basis %+v",report.TopTasks)
 }
}
