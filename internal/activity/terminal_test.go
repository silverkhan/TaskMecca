package activity

import (
 "testing"
 "time"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestTerminalHookClosesUnknownWithoutInventingTime(t *testing.T) {
 project:=t.TempDir()
 base:=time.Now().UTC().Add(-5*time.Minute)
 assignment,err:=runtimeobs.RecordAssignment(project,"assignment-terminal","B-589","/root/controller/w",base)
 if err!=nil{t.Fatal(err)}
 start:=runtimeobs.ExecutionEvent{
  EventID:"hook-start",EventKind:"state",AttemptID:"run-terminal",ObservedAt:base.Format(time.RFC3339Nano),
  RuntimeAgentID:"agent-w",Provider:"codex",State:runtimeobs.StateRunning,
  EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved,
 }
 if err:=runtimeobs.AppendExecutionEvent(project,start);err!=nil{t.Fatal(err)}
 _,err=runtimeobs.BindAttempt(project,start.AttemptID,assignment.TaskID,assignment.AgentPath,"dispatch","",
  map[string]string{"assignment_id":assignment.AssignmentID},base.Add(time.Second))
 if err!=nil{t.Fatal(err)}
 observed:=validEvent()
 observed.EventID="reported-start"
 observed.AgentID="agent-w"
 observed.RuntimeAttemptID="run-terminal"
 observed.AssignmentID=assignment.AssignmentID
 observed.OccurredAt=base.Add(2*time.Minute)
 observed.RecordedAt=observed.OccurredAt
 observed.Confidence=Inferred
 observed.Evidence="reported analysis"
 if ok,err:=(Ledger{Project:project}).Apply(observed);err!=nil||!ok{t.Fatalf("activity start %v %v",ok,err)}
 end:=start
 end.EventID="hook-terminal"
 end.Terminal=true
 end.State=runtimeobs.StateCompleted
 end.ObservedAt=base.Add(4*time.Minute).Format(time.RFC3339Nano)
 if err:=runtimeobs.AppendExecutionEvent(project,end);err!=nil{t.Fatal(err)}
 if changed,err:=RecordTerminal(project,end);err!=nil||!changed{t.Fatalf("terminal close %v %v",changed,err)}
 if changed,err:=RecordTerminal(project,end);err!=nil||changed{t.Fatalf("terminal idempotency %v %v",changed,err)}
 rows,err:=(Ledger{Project:project}).List(assignment.TaskID)
 if err!=nil{t.Fatal(err)}
 summary:=Summarize(assignment.TaskID,rows)
 if summary.InterruptedActivities!=1||summary.AgentTimeMillis!=0||summary.OpenActivities!=0||
   summary.UnmeasuredUpperBoundMillis!=120000{
  t.Fatalf("terminal cannot invent active duration: %+v",summary)
 }
}

func TestNonTerminalHookHasZeroRecorderCost(t *testing.T){
 changed,err:=RecordTerminal("/missing",runtimeobs.ExecutionEvent{EventKind:"activity",AttemptID:"run-one"})
 if err!=nil||changed{t.Fatalf("tool hook should never touch recorder: %v %v",changed,err)}
}
