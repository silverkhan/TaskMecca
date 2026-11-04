package activity

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strconv"
    "strings"
    "time"
)

// GitHubCIClient performs explicitly requested, bounded GitHub Actions reads.
// No polling, no background goroutines, and no token is written to the ledger.
type GitHubCIClient struct{
    HTTP *http.Client
    Token string
    BaseURL string // leave empty in production; injectable only for local tests
}
type githubRun struct{
    ID int64 `json:"id"`
    Name string `json:"name"`
    HeadSHA string `json:"head_sha"`
    RunAttempt int `json:"run_attempt"`
    Status string `json:"status"`
}
type githubJob struct{
    ID int64 `json:"id"`
    RunID int64 `json:"run_id"`
    Name string `json:"name"`
    Status string `json:"status"`
    Conclusion string `json:"conclusion"`
    StartedAt time.Time `json:"started_at"`
    CompletedAt time.Time `json:"completed_at"`
    HTMLURL string `json:"html_url"`
}
func (c GitHubCIClient) getJSON(ctx context.Context,path string,out any)error{
    base:=c.BaseURL
    if base==""{base="https://api.github.com"}
    client:=c.HTTP
    if client==nil{client=&http.Client{Timeout:12*time.Second}}
    u,err:=url.Parse(strings.TrimRight(base,"/")+path)
    if err!=nil{return err}
    req,err:=http.NewRequestWithContext(ctx,http.MethodGet,u.String(),nil)
    if err!=nil{return err}
    req.Header.Set("Accept","application/vnd.github+json")
    req.Header.Set("X-GitHub-Api-Version","2022-11-28")
    req.Header.Set("User-Agent","TaskMecca-Activity-CI")
    if c.Token!=""{req.Header.Set("Authorization","Bearer "+c.Token)}
    response,err:=client.Do(req)
    if err!=nil{return err}
    defer response.Body.Close()
    if response.StatusCode!=http.StatusOK {
        return fmt.Errorf("GitHub Actions read HTTP %d (check repository, run ID and Actions:read access)",response.StatusCode)
    }
    decoder:=json.NewDecoder(io.LimitReader(response.Body,4<<20))
    if err:=decoder.Decode(out);err!=nil{return fmt.Errorf("GitHub Actions response: %w",err)}
    return nil
}

// FetchGitHubCI binds CI evidence to a caller-specified task ONLY when the
// GitHub run HEAD SHA matches an exact expected commit. A workflow's jobs are
// independent wall-time signals, never inferred Worker/Controller activity.
func (c GitHubCIClient) FetchGitHubCI(ctx context.Context,task,repo string,runID int64,expectedSHA,jobFilter string,kind Type,scope string)([]CIJob,error){
    if !ciTaskRE.MatchString(task)||!ciRepoRE.MatchString(repo)||!ciSHA40.MatchString(expectedSHA)||runID<=0 {
        return nil,fmt.Errorf("%w: task, owner/repo, run ID and full SHA required",ErrInvalidEvent)
    }
    if kind!="" && (kind!=TargetedTest&&kind!=FullRegression&&kind!=BuildCheck||jobFilter==""||scope==""){
        return nil,fmt.Errorf("%w: CI classification requires exact job name and scope",ErrInvalidEvent)
    }
    prefix:="/repos/"+repo+"/actions/runs/"+strconv.FormatInt(runID,10)
    var run githubRun
    if err:=c.getJSON(ctx,prefix,&run);err!=nil{return nil,err}
    if run.ID!=runID||!strings.EqualFold(run.HeadSHA,expectedSHA){
        return nil,fmt.Errorf("%w: CI run/expected SHA mismatch",ErrUnverifiedActor)
    }
    if run.Status!="completed"||run.RunAttempt<=0{return nil,fmt.Errorf("CI run not completed")}
    workflow:=strings.TrimSpace(run.Name)
    if workflow==""{workflow="GitHub Actions"}
    result:=make([]CIJob,0,30)
    // Bound the synchronization to 500 jobs / 5 pages to avoid a run with
    // unexpected data causing unbounded memory, network or disk usage.
    for page:=1;page<=5;page++{
        var pageData struct {
            TotalCount int `json:"total_count"`
            Jobs []githubJob `json:"jobs"`
        }
        path:=prefix+"/jobs?per_page=100&page="+strconv.Itoa(page)
        if err:=c.getJSON(ctx,path,&pageData);err!=nil{return nil,err}
        for _,job:=range pageData.Jobs{
            if job.RunID!=0&&job.RunID!=runID{continue}
            if job.Status!="completed"||job.Conclusion==""||!job.CompletedAt.After(job.StartedAt){continue}
            if jobFilter!=""&&job.Name!=jobFilter{continue}
            label:=Type("")
            recordScope:=""
            if kind!=""{label=kind;recordScope=scope}
            entry:=CIJob{
                TaskID:task,Repository:repo,RunID:runID,RunAttempt:run.RunAttempt,
                JobID:job.ID,JobName:job.Name,Workflow:workflow,CommitSHA:strings.ToLower(run.HeadSHA),
                Scope:recordScope,Type:label,Conclusion:job.Conclusion,StartedAt:job.StartedAt,
                CompletedAt:job.CompletedAt,URL:job.HTMLURL,Source:"github_actions_api",
            }
            if err:=entry.Validate();err!=nil{return nil,err}
            result=append(result,entry)
        }
        if len(pageData.Jobs)<100{break}
        if page==5{return nil,fmt.Errorf("CI run exceeds bounded 500-job import limit; narrow the requested job")}
    }
    return result,nil
}
