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
)

// RequestBuild starts an Actions job for an unchanged PR head. The build job
// independently validates the same PR/SHA with its own GitHub credentials
// before checking out code. No arbitrary SHA, fork or branch is accepted.
func (c *Client) RequestBuild(ctx context.Context, number int, sha string) error {
 if number<=0 || !validSHA(sha) {return errors.New("invalid Preview PR number or head SHA")}
 if strings.TrimSpace(c.Token)=="" {
  return errors.New("Preview rebuild requires TASK_MECCA_GITHUB_TOKEN with Actions:write; alternatively run the workflow manually on GitHub")
 }
 var pr struct{
  Number int `json:"number"`
  State string `json:"state"`
  Base struct{Ref string `json:"ref"`} `json:"base"`
  Head struct{
   SHA string `json:"sha"`
   Repo *ghRepository `json:"repo"`
  } `json:"head"`
 }
 if err:=c.get(ctx,"/repos/"+Repository+"/pulls/"+strconv.Itoa(number),&pr);err!=nil{return err}
 if pr.Number!=number || pr.State!="open" || pr.Base.Ref!="dev" || pr.Head.Repo==nil ||
  !strings.EqualFold(pr.Head.Repo.FullName,Repository) || pr.Head.SHA!=sha {
  return errors.New("Preview PR has changed, is a fork, is closed or no longer targets dev; refresh before rebuilding")
 }
 body,err:=json.Marshal(map[string]any{
  "ref":"dev",
  "inputs":map[string]string{"preview_pr":strconv.Itoa(number),"preview_sha":sha},
 })
 if err!=nil{return err}
 req,err:=http.NewRequestWithContext(ctx,http.MethodPost,
  strings.TrimRight(c.APIURL,"/")+"/repos/"+Repository+"/actions/workflows/"+workflow+"/dispatches",bytes.NewReader(body))
 if err!=nil{return err}
 req.Header.Set("Accept","application/vnd.github+json")
 req.Header.Set("Content-Type","application/json")
 req.Header.Set("X-GitHub-Api-Version","2022-11-28")
 req.Header.Set("Authorization","Bearer "+c.Token)
 client:=c.HTTP
 if client==nil{client=&http.Client{}}
 resp,err:=client.Do(req)
 if err!=nil{return fmt.Errorf("GitHub Preview workflow dispatch failed: %w",err)}
 defer resp.Body.Close()
 switch resp.StatusCode{
 case http.StatusNoContent, http.StatusCreated:return nil
 case http.StatusUnauthorized:return errors.New("GitHub token for Preview rebuild is unauthorized")
 case http.StatusForbidden:return errors.New("GitHub token lacks Actions:write or workflow permission")
 case http.StatusUnprocessableEntity:return errors.New("GitHub workflow dispatch rejected the PR/SHA or branch; check workflow availability on the default branch")
 default:return fmt.Errorf("GitHub Preview workflow returned HTTP %d",resp.StatusCode)
 }
}
