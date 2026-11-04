package activity

import (
  "errors"
  "strings"
  "testing"
  "time"
)

func sampleCIJob()CIJob{
    start:=time.Date(2026,11,4,1,0,0,0,time.UTC)
    return CIJob{
      TaskID:"B-589",Repository:"silverkhan/TaskMecca",RunID:100,RunAttempt:1,
      JobID:991,JobName:"Go runtime",Workflow:"CI",
      CommitSHA:strings.Repeat("a",40),Type:FullRegression,Scope:"all-go",
      Conclusion:"success",StartedAt:start,CompletedAt:start.Add(80*time.Second),
      URL:"https://github.com/silverkhan/TaskMecca/actions/runs/100/job/991",
      Source:"github_actions_api",
    }
}
func TestCIJournalAppendIdempotentAndDistinctFromAgentTime(t *testing.T){
    store:=CIJournal{Project:t.TempDir()}
    job:=sampleCIJob()
    if inserted,err:=store.AppendBatch([]CIJob{job});err!=nil||inserted!=1{t.Fatalf("first import %d %v",inserted,err)}
    if inserted,err:=store.AppendBatch([]CIJob{job});err!=nil||inserted!=0{t.Fatalf("idempotent import %d %v",inserted,err)}
    records,err:=store.List(job.TaskID)
    if err!=nil||len(records)!=1{t.Fatalf("read records %d %v",len(records),err)}
    local:=[]TestRun{{Type:FullRegression,RunEvidence:RunEvidence{
        RunID:"test-go-1",GitSHA:strings.Repeat("a",40),Scope:"all-go",
    },DurationMillis:40000}}
    report:=SummarizeCI(records,local)
    if report.JobCount!=1||report.SumJobMillis!=80000||report.FullRegressionCount!=1||
       report.MatchedJobs!=1||len(report.Links)!=1||report.Links[0].LocalRunIDs[0]!="test-go-1"{
       t.Fatalf("CI report mismatch: %+v",report)
    }
    // CI duration is only in the separate CI summary, not Activity summary.
    if got:=Summarize(job.TaskID,nil);got.AgentTimeMillis!=0{
       t.Fatalf("CI evidence inflated agent-time: %+v",got)
    }
    changed:=job;changed.Conclusion="failure"
    if _,err:=store.AppendBatch([]CIJob{changed});!errors.Is(err,ErrCIConflict){t.Fatalf("conflicting provider evidence accepted: %v",err)}
    bad:=job;bad.URL="https://evil.example/job"
    if err:=bad.Validate();err==nil{t.Fatal("untrusted CI link accepted")}
}
func TestCINeverMatchesWithoutFullRevisionScopeAndType(t *testing.T){
    job:=sampleCIJob()
    local:=[]TestRun{{Type:FullRegression,RunEvidence:RunEvidence{
        RunID:"test-other-commit",GitSHA:strings.Repeat("b",40),Scope:"all-go",
    }},{Type:TargetedTest,RunEvidence:RunEvidence{
        RunID:"test-other-scope",GitSHA:strings.Repeat("a",40),Scope:"all-go",
    }}}
    report:=SummarizeCI([]CIJob{job},local)
    if report.MatchedJobs!=0{t.Fatalf("false CI/local test match: %+v",report)}
}
