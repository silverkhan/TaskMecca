// Package preview discovers provenance-bound, read-only GitHub PR build metadata.
// It intentionally provides no executable-download or process-launch primitive.
package preview

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
    "sort"
    "strconv"
    "strings"
    "time"
)

const (
    Repository = "silverkhan/TaskMecca"
    workflow = "go-artifacts.yml"
)

type Build struct {
    Number int `json:"number"`
    Title string `json:"title"`
    URL string `json:"url"`
    Branch string `json:"branch"`
    HeadSHA string `json:"head_sha"`
    UpdatedAt string `json:"updated_at"`
    BuildStatus string `json:"build_status"`
    BuildFinishedAt string `json:"build_finished_at,omitempty"`
    RunID int64 `json:"run_id,omitempty"`
    RunURL string `json:"run_url,omitempty"`
    ArtifactID int64 `json:"artifact_id,omitempty"`
    ArtifactName string `json:"artifact_name,omitempty"`
    ExecutionAllowed bool `json:"execution_allowed"`
    Reason string `json:"reason,omitempty"`
}

type Client struct {
    HTTP *http.Client
    APIURL string
    Token string
    // TokenFallback is used only if artifact downloading returns 401 without
    // an explicit token. It is not used to authenticate the Preview process.
    TokenFallback func(context.Context)(string,error)
}

func NewClient() *Client {
    return &Client{
        HTTP:&http.Client{Timeout:12*time.Second},
        APIURL:"https://api.github.com",
        Token:strings.TrimSpace(os.Getenv("TASK_MECCA_GITHUB_TOKEN")),
        TokenFallback:githubCLIToken,
    }
}

type ghRepository struct { FullName string `json:"full_name"` }
type ghPull struct {
    Number int `json:"number"`
    Title string `json:"title"`
    HTMLURL string `json:"html_url"`
    UpdatedAt string `json:"updated_at"`
    Base struct { Ref string `json:"ref"` } `json:"base"`
    Head struct {
        Ref string `json:"ref"`
        SHA string `json:"sha"`
        Repo *ghRepository `json:"repo"`
    } `json:"head"`
}
type ghRun struct {
    ID int64 `json:"id"`
    HTMLURL string `json:"html_url"`
    Status string `json:"status"`
    Conclusion string `json:"conclusion"`
    Event string `json:"event"`
    CreatedAt string `json:"created_at"`
    UpdatedAt string `json:"updated_at"`
    RunAttempt int `json:"run_attempt"`
    PullRequests []struct { Number int `json:"number"` } `json:"pull_requests"`
}
type ghRuns struct { Runs []ghRun `json:"workflow_runs"` }
type ghArtifact struct {
    ID int64 `json:"id"`
    Name string `json:"name"`
    Expired bool `json:"expired"`
}
type ghArtifacts struct { Artifacts []ghArtifact `json:"artifacts"` }

func (c *Client) get(ctx context.Context, path string, out any) error {
    base:=strings.TrimRight(c.APIURL,"/")
    if base=="" {return errors.New("GitHub API URL is missing")}
    req,err:=http.NewRequestWithContext(ctx,http.MethodGet,base+path,nil)
    if err!=nil{return err}
    req.Header.Set("Accept","application/vnd.github+json")
    req.Header.Set("X-GitHub-Api-Version","2022-11-28")
    if c.Token!="" {req.Header.Set("Authorization","Bearer "+c.Token)}
    client:=c.HTTP
    if client==nil {client=&http.Client{Timeout:12*time.Second}}
    resp,err:=client.Do(req)
    if err!=nil{return fmt.Errorf("GitHub API unavailable: %w",err)}
    defer resp.Body.Close()
    if resp.StatusCode!=http.StatusOK {
        // Never reflect GitHub response bodies: they may contain user/account data.
        switch resp.StatusCode {
        case 401: return errors.New("GitHub authentication failed")
        case 403,429: return errors.New("GitHub API rate limited or access denied")
        case 404: return errors.New("GitHub repository or workflow not accessible")
        default: return fmt.Errorf("GitHub API status %d",resp.StatusCode)
        }
    }
    dec:=json.NewDecoder(io.LimitReader(resp.Body,4<<20))
    if err=dec.Decode(out);err!=nil{return fmt.Errorf("GitHub API invalid response: %w",err)}
    return nil
}

func validSHA(sha string) bool {
    if len(sha)!=40 {return false}
    for _,v:=range sha {
        if !((v>='0'&&v<='9')||(v>='a'&&v<='f')) {return false}
    }
    return true
}

func runMatchesPR(run ghRun, number int) bool {
    if run.Event!="pull_request" {return false}
    for _,pr:=range run.PullRequests {if pr.Number==number{return true}}
    return false
}

func dateKey(value string) time.Time {
    at,_:=time.Parse(time.RFC3339,value)
    return at
}

// ListBuilds returns at most 30 same-repository PRs targeting dev.
// An unverified run is never advertised as executable. Build discovery does
// not download or run the PR's binaries.
func (c *Client) ListBuilds(ctx context.Context, limit int) ([]Build,error) {
    if limit<1 || limit>30 {limit=20}
    var prs []ghPull
    path:="/repos/"+Repository+"/pulls?state=open&sort=updated&direction=desc&per_page=100"
    if err:=c.get(ctx,path,&prs);err!=nil{return nil,err}
    var runs ghRuns
    runPath:="/repos/"+Repository+"/actions/workflows/"+url.PathEscape(workflow)+"/runs?event=pull_request&per_page=100"
    if err:=c.get(ctx,runPath,&runs);err!=nil{return nil,err}

    result:=make([]Build,0,limit)
    for _,pr:=range prs {
        if pr.Base.Ref!="dev" {continue}
        item:=Build{
            Number:pr.Number,Title:pr.Title,URL:pr.HTMLURL,Branch:pr.Head.Ref,
            HeadSHA:pr.Head.SHA,UpdatedAt:pr.UpdatedAt,BuildStatus:"none",
            ExecutionAllowed:false,
        }
        if pr.Head.Repo==nil||!strings.EqualFold(pr.Head.Repo.FullName,Repository) {
            item.BuildStatus="blocked"
            item.Reason="fork or unknown head repository"
        } else if !validSHA(pr.Head.SHA) {
            item.BuildStatus="blocked"
            item.Reason="invalid PR head SHA"
        } else {
            item=c.findBuild(ctx,pr.Number,pr.Head.SHA,runs.Runs)
        }
        item.Number=pr.Number
        item.Title=pr.Title
        item.URL=pr.HTMLURL
        item.Branch=pr.Head.Ref
        item.HeadSHA=pr.Head.SHA
        item.UpdatedAt=pr.UpdatedAt
        result=append(result,item)
        if len(result)==limit {break}
    }
    // Prefer newest completed-build time; no-completed-build entries follow
    // in PR updated order. Ties have a deterministic PR number fallback.
    sort.SliceStable(result,func(i,j int)bool {
        a,b:=dateKey(result[i].BuildFinishedAt),dateKey(result[j].BuildFinishedAt)
        if !a.Equal(b){return a.After(b)}
        ua,ub:=dateKey(result[i].UpdatedAt),dateKey(result[j].UpdatedAt)
        if !ua.Equal(ub){return ua.After(ub)}
        return result[i].Number>result[j].Number
    })
    return result,nil
}

func (c *Client) findBuild(ctx context.Context, number int, sha string, runs []ghRun) Build {
    item:=Build{BuildStatus:"none",ExecutionAllowed:false}
    expected:="task-mecca-preview-pr-"+strconv.Itoa(number)+"-"+sha
    for _,run:=range runs {
        if !runMatchesPR(run,number) {continue}
        if run.Status!="completed" {
            // A still-running build is informative, but never usable.
            if item.BuildStatus=="none" {
                item.BuildStatus=run.Status
                item.RunID=run.ID
                item.RunURL=run.HTMLURL
            }
            continue
        }
        // Only a successful workflow with an exact head-SHA artifact can be
        // advertised as a build of this PR revision.
        if run.Conclusion!="success" {
            if item.BuildStatus=="none" {
                item.BuildStatus="failure"
                item.RunID=run.ID
                item.RunURL=run.HTMLURL
                item.BuildFinishedAt=run.UpdatedAt
            }
            continue
        }
        var artifacts ghArtifacts
        path:=fmt.Sprintf("/repos/%s/actions/runs/%d/artifacts?per_page=100",Repository,run.ID)
        if err:=c.get(ctx,path,&artifacts);err!=nil{
            if item.BuildStatus=="none" {
                item.BuildStatus="unavailable"
                item.Reason=err.Error()
            }
            continue
        }
        for _,a:=range artifacts.Artifacts {
            if a.Name!=expected {continue}
            item.RunID=run.ID
            item.RunURL=run.HTMLURL
            item.BuildFinishedAt=run.UpdatedAt
            item.ArtifactID=a.ID
            item.ArtifactName=a.Name
            if a.Expired {
                item.BuildStatus="expired"
                item.Reason="GitHub artifact retention expired"
            }else{
                item.BuildStatus="success"
                // This is only discovery. M2 enables execution after
                // manifest/checksum, runtime sandbox and consent checks.
                item.ExecutionAllowed=false
                item.Reason="execution is disabled pending sandbox and integrity verification"
            }
            return item
        }
    }
    if item.BuildStatus=="none" {item.Reason="no successful artifact matches the current PR head SHA"}
    return item
}
