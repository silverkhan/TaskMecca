package activity

import (
    "fmt"
    "strings"
)

type RunEvidence struct {
    RunID string `json:"run_id,omitempty"`
    Command string `json:"command,omitempty"`
    Scope string `json:"scope,omitempty"`
    GitSHA string `json:"git_sha,omitempty"`
    Worktree string `json:"worktree,omitempty"`
    Purpose string `json:"purpose,omitempty"`
    Result string `json:"result,omitempty"`
    CIURL string `json:"ci_url,omitempty"`
}

func (r RunEvidence) Validate()error {
    if strings.ContainsAny(r.RunID+r.Command+r.Scope+r.GitSHA+r.Purpose,"\x00"){return fmt.Errorf("%w: null byte in run evidence",ErrInvalidEvent)}
    if r.Result!="" && r.Result!="passed" && r.Result!="failed" && r.Result!="cancelled" && r.Result!="unknown" {
        return fmt.Errorf("%w: invalid run result",ErrInvalidEvent)
    }
    return nil
}
type TestRun struct {
    Type Type `json:"type"`
    Round int `json:"round"`
    AgentID string `json:"agent_id"`
    RunEvidence RunEvidence `json:"run_evidence"`
    DurationMillis int64 `json:"duration_ms"`
    RepetitionCandidate bool `json:"repetition_candidate"`
}
type TestReport struct {
    Runs []TestRun `json:"runs"`
    TotalMillis int64 `json:"total_ms"`
    FullRegressionMillis int64 `json:"full_regression_ms"`
    FullRegressionCount int `json:"full_regression_count"`
    RepetitionCandidates int `json:"repetition_candidates"`
}
// SummarizeTests only attributes explicit test/build activity segments with
// run evidence. Same SHA, command and scope are repetition CANDIDATES, not
// automatic proof of waste (flaky CI and explicit retest can be necessary).
func SummarizeTests(segments []Segment) TestReport {
    out:=TestReport{Runs:[]TestRun{}}
    seen:=map[string]bool{}
    for _,s:=range segments{
        if s.Type!=TargetedTest && s.Type!=FullRegression && s.Type!=BuildCheck{continue}
        if s.RunEvidence==nil{continue}
        run:=*s.RunEvidence
        item:=TestRun{Type:s.Type,Round:s.Round,AgentID:s.AgentID,RunEvidence:run,DurationMillis:s.DurationMillis}
        if run.GitSHA!=""&&run.Command!=""&&run.Scope!=""{
            key:=string(s.Type)+"\x00"+run.GitSHA+"\x00"+run.Command+"\x00"+run.Scope
            if seen[key]{item.RepetitionCandidate=true;out.RepetitionCandidates++}
            seen[key]=true
        }
        out.TotalMillis+=s.DurationMillis
        if s.Type==FullRegression{out.FullRegressionCount++;out.FullRegressionMillis+=s.DurationMillis}
        out.Runs=append(out.Runs,item)
    }
    return out
}
