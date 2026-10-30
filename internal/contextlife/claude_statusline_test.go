package contextlife

import (
 "strings"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)
func TestClaudeStatuslineExactSessionAndNullableUsage(t *testing.T){
 project:=seedProject(t);now:=time.Date(2026,10,30,4,0,0,0,time.UTC)
 start:=runtimeobs.ExecutionEvent{EventID:"claude-event",EventKind:"state",ObservedAt:now.Format(time.RFC3339Nano),AttemptID:"run-claude",Provider:"claude",SessionID:"session-claude",RuntimeAgentID:"native-agent",State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved}
 if err:=runtimeobs.AppendExecutionEvent(project,start);err!=nil{t.Fatal(err)}
 if _,err:=runtimeobs.BindAttempt(project,"run-claude","","/root/controller","explicit","",map[string]string{"source":"test"},now.Add(time.Second));err!=nil{t.Fatal(err)}
 for _,payload:=range []string{
  `{"session_id":"wrong","context_window":{"used_percentage":82,"context_window_size":200000,"total_input_tokens":164000}}`,
  `{"session_id":"session-claude","context_window":{"used_percentage":null,"context_window_size":200000}}`,
  `{"session_id":"session-claude","context_window":{"used_percentage":150,"context_window_size":200000}}`,
 }{
  if _,err:=ObserveClaudeStatusline(project,"controller","run-claude",strings.NewReader(payload),now.Add(2*time.Second));err==nil{t.Fatal("invalid structured provider reading accepted")}
 }
 s,err:=ObserveClaudeStatusline(project,"controller","run-claude",strings.NewReader(`{"session_id":"session-claude","context_window":{"used_percentage":83,"context_window_size":200000,"total_input_tokens":166000}}`),now.Add(3*time.Second))
 if err!=nil{t.Fatal(err)}
 if s.Observation.Trust!="provider_structured"{t.Fatalf("%+v",s)}
 st,err:=Status(project,"controller","controller_resume",now.Add(4*time.Second));if err!=nil{t.Fatal(err)}
 if st.Decision.Action!="prepare"||st.Decision.ContextStatus!="provider_structured"{t.Fatalf("unverified structured JSON must only prepare, got %+v",st.Decision)}
}
