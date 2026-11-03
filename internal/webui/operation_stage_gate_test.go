package webui

import (
    "fmt"
    "testing"
    "time"

    "github.com/silverkhan/TaskMecca/internal/handoff"
    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestOperationStageGateTransitions(t *testing.T) {
    now:=time.Date(2026,11,4,1,0,0,0,time.UTC)
    future:=now.Add(time.Minute).Format(time.RFC3339Nano)
    old:=now.Add(-time.Second).Format(time.RFC3339Nano)
    cases:=[]struct{name,state string; failed bool; grace string; want bool}{
        {"running","worker_running",false,"",false},
        {"unverified_start","runtime_observation_pending",false,"",false},
        {"handoff_applied","handoff_applied",false,"",false},
        {"confirmed_done","controller_verified_complete",false,"",false},
        {"assignment_dwell","assignment_pending",false,future,false},
        {"assignment_expired","assignment_pending",false,old,true},
        {"handoff_wait","handoff_pending",false,future,false},
        {"handoff_overdue","handoff_pending",false,old,true},
        {"review_wait","controller_review",false,future,false},
        {"review_overdue","controller_review",false,old,true},
        {"hold_unresolved","handoff_evidence_unverified",false,"",true},
        {"ownerless","assignment_pending",false,"",true},
        {"reported_end","worker_report_pending",false,"",true},
        {"handoff_failure","handoff_failed",true,future,true},
        {"contradictory_settled","worker_running",true,"",false},
    }
    for _,tc:=range cases {t.Run(tc.name,func(t *testing.T){
        got:=operationStageNeedsAttention(operationStage{Stage:tc.state,Failed:tc.failed,GraceUntil:tc.grace},now)
        if got!=tc.want {t.Fatalf("got=%v want=%v",got,tc.want)}
    })}
}

func TestOperationStageOnlyCurrentAssignmentAndMatchingAttempt(t *testing.T) {
    now:=time.Date(2026,11,4,1,0,0,0,time.UTC)
    previous:=now.Add(-time.Hour).Format(time.RFC3339Nano)
    current:=now.Add(-time.Minute).Format(time.RFC3339Nano)
    assignments:=[]runtimeobs.Assignment{
        {TaskID:"A-42",AssignmentID:"old",AgentPath:"/root/old",AssignedAt:previous},
        {TaskID:"A-42",AssignmentID:"current",AgentPath:"/root/new",AssignedAt:current},
    }
    attempts:=[]runtimeobs.Attempt{
        {TaskID:"A-42",AttemptID:"old-worker",AgentPath:"/root/old",BindingState:runtimeobs.BindingBound,BindingEvidence:map[string]string{"assignment_id":"old"}},
        {TaskID:"A-42",AttemptID:"new-worker",AgentPath:"/root/new",BindingState:runtimeobs.BindingBound,BindingEvidence:map[string]string{"assignment_id":"current"},CurrentState:runtimeobs.StateRunning,StateEvidenceSource:runtimeobs.EvidenceHook,StateObservationQuality:runtimeobs.QualityObserved},
        {TaskID:"A-42",AttemptID:"other-worker",AgentPath:"/root/other",BindingState:runtimeobs.BindingBound,BindingEvidence:map[string]string{"assignment_id":"current"}},
    }
    stages:=projectOperationStages("p",assignments,attempts,handoff.Ledger{},now)
    if len(stages)!=1 || stages[0].AssignmentID!="current" || stages[0].AttemptID!="new-worker" || stages[0].Stage!="worker_running" {t.Fatalf("stages=%+v",stages)}
    // A subsequent confirmed failure remains visible even when the task and
    // previous attempt have a retained done/archived history.
    attempts[1].CurrentState=runtimeobs.StateErrored
    attempts[1].Terminal=true
    stages=projectOperationStages("p",assignments,attempts,handoff.Ledger{},now)
    if len(stages)!=1 || stages[0].Stage=="worker_running" {t.Fatalf("late failure hidden: %+v",stages)}
    attempts=append(attempts,runtimeobs.Attempt{TaskID:"A-42",AttemptID:"duplicate",AgentPath:"/root/new",BindingState:runtimeobs.BindingBound,BindingEvidence:map[string]string{"assignment_id":"current"}})
    stages=projectOperationStages("p",assignments,attempts,handoff.Ledger{},now)
    if len(stages)!=0 {t.Fatalf("ambiguous exact assignment must not yield synthetic stage: %+v",stages)}
}

func BenchmarkOperationStageIndexWithHistoricalAttempts(b *testing.B) {
    now:=time.Now().UTC()
    assignments:=make([]runtimeobs.Assignment,0,300)
    attempts:=make([]runtimeobs.Attempt,0,5000)
    for i:=0;i<5000;i++ {
        id:=fmt.Sprintf("A-%d",i)
        attempts=append(attempts,runtimeobs.Attempt{TaskID:id,AttemptID:fmt.Sprint(i),AgentPath:"/root/worker",BindingState:runtimeobs.BindingBound,BindingEvidence:map[string]string{"assignment_id":fmt.Sprint(i)},Terminal:true})
        if i%20==0 {
            assignments=append(assignments,runtimeobs.Assignment{TaskID:id,AssignmentID:fmt.Sprint(i),AgentPath:"/root/worker",AssignedAt:now.Add(-time.Hour).Format(time.RFC3339Nano)})
        }
    }
    b.ResetTimer()
    for i:=0;i<b.N;i++ {projectOperationStages("p",assignments,attempts,handoff.Ledger{},now)}
}
