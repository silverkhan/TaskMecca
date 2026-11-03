package activity

import (
    "testing"
    "time"
)

func TestSummarizeTestsIdentifiesRepeatCandidates(t *testing.T){
    at:=time.Date(2026,11,3,7,0,0,0,time.UTC)
    run:=&RunEvidence{Command:"go test ./...",GitSHA:"abcdef",Scope:"all-go",Purpose:"initial"}
    segments:=[]Segment{
        {TaskID:"B-1",Role:RoleWorker,Type:FullRegression,AgentID:"w1",StartedAt:at,EndedAt:at.Add(time.Minute),DurationMillis:60000,RunEvidence:run},
        {TaskID:"B-1",Role:RoleController,Type:FullRegression,AgentID:"c1",StartedAt:at.Add(2*time.Minute),EndedAt:at.Add(4*time.Minute),DurationMillis:120000,RunEvidence:run},
        {TaskID:"B-1",Role:RoleWorker,Type:TargetedTest,AgentID:"w1",StartedAt:at.Add(5*time.Minute),EndedAt:at.Add(6*time.Minute),DurationMillis:60000,RunEvidence:&RunEvidence{Command:"go test ./internal/activity",GitSHA:"abcdef",Scope:"activity"}},
        {TaskID:"B-1",Role:RoleWorker,Type:Implementation,AgentID:"w1",DurationMillis:500000},
    }
    out:=SummarizeTests(segments)
    if len(out.Runs)!=3||out.FullRegressionCount!=2||out.FullRegressionMillis!=180000||out.RepetitionCandidates!=1||out.TotalMillis!=240000 {
        t.Fatalf("wrong test statistics: %+v",out)
    }
    if !out.Runs[1].RepetitionCandidate{t.Fatal("same SHA/command/scope not detected")}
}

func TestRunEvidenceUnknownIsNotFabricated(t *testing.T){
    out:=SummarizeTests([]Segment{{Type:FullRegression,DurationMillis:5000}})
    if out.FullRegressionCount!=0||out.TotalMillis!=0 {t.Fatalf("missing evidence must remain uncounted: %+v",out)}
}
