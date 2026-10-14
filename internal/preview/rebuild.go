package preview

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "net/http"
    "strconv"
    "strings"
    "time"
)

// RequestBuild asks the trusted Dev workflow to recompile the exact CURRENT
// same-repository PR head. It never changes the PR branch and never launches
// a second Task Mecca process. Only an explicit user action may call this.
func (c *Client) RequestBuild(ctx context.Context, number int, sha string) error {
    if number<1 || !validSHA(sha) {
        return errors.New("PR rebuild requires a positive PR number and full lowercase head SHA")
    }
    if strings.TrimSpace(c.Token)=="" {
        return errors.New("PR build creation requires TASK_MECCA_GITHUB_TOKEN with Actions:write and Pull requests:read; otherwise trigger the Go standalone artifacts workflow manually on GitHub")
    }
    var pr struct {
        Number int `json:"number"`
        State string `json:"state"`
        Base struct{Ref string `json:"ref"`} `json:"base"`
        Head struct {
            SHA string `json:"sha"`
            Repo *ghRepository `json:"repo"`
        } `json:"head"`
    }
    endpoint:="/repos/"+Repository+"/pulls/"+strconv.Itoa(number)
    if err:=c.get(ctx,endpoint,&pr);err!=nil {return fmt.Errorf("PR head verification failed: %w",err)}
    if pr.Number!=number || pr.State!="open" || pr.Base.Ref!="dev" ||
        pr.Head.Repo==nil || !strings.EqualFold(pr.Head.Repo.FullName,Repository) ||
        pr.Head.SHA!=sha {
        return errors.New("PR has a new head, is closed, is a fork, or does not target Dev; refresh the list")
    }

    body,err:=json.Marshal(map[string]any{
        "ref":"dev",
        "inputs":map[string]string{
            "preview_pr":strconv.Itoa(number),
            "preview_sha":sha,
        },
    })
    if err!=nil{return err}
    endpoint="/repos/"+Repository+"/actions/workflows/"+workflow+"/dispatches"
    req,err:=http.NewRequestWithContext(ctx,http.MethodPost,strings.TrimRight(c.APIURL,"/")+endpoint,bytes.NewReader(body))
    if err!=nil{return err}
    req.Header.Set("Authorization","Bearer "+c.Token)
    req.Header.Set("Accept","application/vnd.github+json")
    req.Header.Set("X-GitHub-Api-Version","2022-11-28")
    req.Header.Set("Content-Type","application/json")
    client:=c.HTTP
    if client==nil {client=&http.Client{Timeout:20*time.Second}}
    response,err:=client.Do(req)
    if err!=nil{return fmt.Errorf("GitHub Actions dispatch failed: %w",err)}
    defer response.Body.Close()
    switch response.StatusCode{
    case http.StatusNoContent:
        return nil
    case http.StatusUnauthorized:
        return errors.New("GitHub token is invalid (HTTP 401); check TASK_MECCA_GITHUB_TOKEN")
    case http.StatusForbidden:
        return errors.New("GitHub denied Actions:write permission (HTTP 403)")
    case http.StatusNotFound:
        return errors.New("trusted Dev workflow does not support manual PR builds or the token cannot access it (HTTP 404)")
    case http.StatusUnprocessableEntity:
        return errors.New("GitHub rejected PR build inputs or Dev workflow dispatch is unavailable (HTTP 422)")
    default:
        return fmt.Errorf("PR build dispatch failed (HTTP %d)",response.StatusCode)
    }
}
