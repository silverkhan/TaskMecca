package activity

import (
 "testing"
 "time"
)
func TestSummarizeSuggestionsDoesNotPromoteType(t *testing.T){
 t0:=time.Date(2026,11,3,0,0,0,0,time.UTC)
 start:=validEvent();start.Type=Other;start.ProposedType="dependency_resolution";start.Explanation="external package compatibility";start.OccurredAt=t0
 stop:=start;stop.EventID="evt-stop";stop.Action=Stop;stop.OccurredAt=t0.Add(3*time.Minute)
 result:=SummarizeSuggestions([]Event{start,stop})
 if len(result)!=1||result[0].ProposedType!="dependency_resolution"||result[0].EventCount!=1||result[0].ObservedMillis!=180000{
  t.Fatalf("wrong suggestion projection: %+v",result)
 }
 if Type("dependency_resolution").Valid(){t.Fatal("proposal automatically became official type")}
}
