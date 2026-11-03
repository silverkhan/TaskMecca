package activity

import (
 "errors"
 "sort"
 "strings"
 "time"
)

// SegmentFilter filters completed, observed Activity intervals. This is a
// diagnostic view, not a filter on canonical lifecycle/approval evidence.
type SegmentFilter struct {
 Role Role `json:"role,omitempty"`
 Type Type `json:"type,omitempty"`
 AgentID string `json:"agent_id,omitempty"`
 Round *int `json:"round,omitempty"`
 From *time.Time `json:"from,omitempty"`
 Until *time.Time `json:"until,omitempty"`
}
var ErrInvalidSegmentFilter=errors.New("invalid activity segment filter")
func(f SegmentFilter)Validate()error{
 if f.Role!=""&&!f.Role.Valid(){return ErrInvalidSegmentFilter}
 if f.Type!=""&&!f.Type.Valid(){return ErrInvalidSegmentFilter}
 if f.Round!=nil&&*f.Round<0{return ErrInvalidSegmentFilter}
 if strings.ContainsAny(f.AgentID,"\r\n\x00"){return ErrInvalidSegmentFilter}
 if f.From!=nil&&f.Until!=nil&&!f.From.Before(*f.Until){return ErrInvalidSegmentFilter}
 return nil
}
func(f SegmentFilter)matches(s Segment)bool{
 if f.Role!=""&&f.Role!=s.Role{return false}
 if f.Type!=""&&f.Type!=s.Type{return false}
 if f.AgentID!=""&&f.AgentID!=s.AgentID{return false}
 if f.Round!=nil&&*f.Round!=s.Round{return false}
 return true
}
func(f SegmentFilter)clip(s Segment)(Segment,bool){
 if !f.matches(s){return Segment{},false}
 if f.From!=nil && s.StartedAt.Before(*f.From){s.StartedAt=*f.From}
 if f.Until!=nil && s.EndedAt.After(*f.Until){s.EndedAt=*f.Until}
 if !s.EndedAt.After(s.StartedAt){return Segment{},false}
 s.DurationMillis=s.EndedAt.Sub(s.StartedAt).Milliseconds()
 return s,true
}
// FilteredProjectReport intentionally excludes lifecycle elapsed and
// waiting values. Filtering activities must not silently rewrite a complete
// task's registered-to-completed duration.
type FilteredProjectReport struct{
 Filter SegmentFilter `json:"filter"`
 Basis string `json:"duration_basis"`
 MatchedTasks int `json:"matched_tasks"`
 MatchedSegments int `json:"matched_segments"`
 AgentTimeMillis int64 `json:"agent_time_ms"`
 SumTaskObservedWallMillis int64 `json:"sum_task_observed_wall_ms"`
 ByType map[Type]int64 `json:"by_type_ms"`
 TopTasks []TaskTime `json:"top_tasks"`
}
func SummarizeFilteredProject(events []Event,f SegmentFilter)(FilteredProjectReport,error){
 if err:=f.Validate();err!=nil{return FilteredProjectReport{},err}
 grouped:=map[string][]Event{}
 for _,e:=range events{grouped[e.TaskID]=append(grouped[e.TaskID],e)}
 result:=FilteredProjectReport{Filter:f,Basis:"activity_observed_segments",ByType:map[Type]int64{},TopTasks:[]TaskTime{}}
 for id,rows:=range grouped{
  summary:=Summarize(id,rows)
  selected:=make([]Segment,0)
  var agentMillis int64
  for _,s:=range summary.Segments{
   part,ok:=f.clip(s)
   if !ok{continue}
   result.MatchedSegments++
   agentMillis+=part.DurationMillis
   result.AgentTimeMillis+=part.DurationMillis
   result.ByType[part.Type]+=part.DurationMillis
   selected=append(selected,part)
  }
  if len(selected)==0{continue}
  result.MatchedTasks++
  union:=unionSegmentMillis(selected)
  result.SumTaskObservedWallMillis+=union
  result.TopTasks=append(result.TopTasks,TaskTime{
   TaskID:id,DurationMillis:union,DurationBasis:"activity_observed_segments",
   AgentMillis:agentMillis,
  })
 }
 sort.Slice(result.TopTasks,func(i,j int)bool{
  a,b:=result.TopTasks[i],result.TopTasks[j]
  if a.DurationMillis==b.DurationMillis{return a.TaskID<b.TaskID}
  return a.DurationMillis>b.DurationMillis
 })
 if len(result.TopTasks)>10{result.TopTasks=result.TopTasks[:10]}
 return result,nil
}
func unionSegmentMillis(segments []Segment)int64{
 if len(segments)==0{return 0}
 sort.Slice(segments,func(i,j int)bool{return segments[i].StartedAt.Before(segments[j].StartedAt)})
 begin,end:=segments[0].StartedAt,segments[0].EndedAt
 var total int64
 for _,s:=range segments[1:]{
  if s.StartedAt.After(end){
   total+=end.Sub(begin).Milliseconds()
   begin,end=s.StartedAt,s.EndedAt
  } else if s.EndedAt.After(end){end=s.EndedAt}
 }
 return total+end.Sub(begin).Milliseconds()
}
