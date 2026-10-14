package preview

import (
 "context"
 "encoding/json"
 "fmt"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
)

func TestRequestBuildVerifiesPRAndDispatchesExactSHA(t *testing.T){
 sha:=strings.Repeat("a",40)
 postCount:=0
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  switch r.URL.Path {
  case "/repos/silverkhan/TaskMecca/pulls/100":
   fmt.Fprintf(w,`{"number":100,"state":"open","base":{"ref":"dev"},"head":{"sha":"%s","repo":{"full_name":"silverkhan/TaskMecca"}}}`,sha)
  case "/repos/silverkhan/TaskMecca/actions/workflows/go-artifacts.yml/dispatches":
   if r.Method!="POST"||r.Header.Get("Authorization")!="Bearer test-token"{t.Error("missing auth/method")}
   var body struct{
    Ref string `json:"ref"`
    Inputs map[string]string `json:"inputs"`
   }
   if err:=json.NewDecoder(r.Body).Decode(&body);err!=nil{t.Error(err)}
   if body.Ref!="dev"||body.Inputs["preview_pr"]!="100"||body.Inputs["preview_sha"]!=sha{t.Errorf("wrong dispatch: %+v",body)}
   postCount++
   w.WriteHeader(http.StatusNoContent)
  default: http.Error(w,"not found",404)
  }
 }))
 defer srv.Close()
 client:=Client{APIURL:srv.URL,HTTP:srv.Client(),Token:"test-token"}
 if err:=client.RequestBuild(context.Background(),100,sha);err!=nil{t.Fatal(err)}
 if postCount!=1{t.Fatalf("want one workflow dispatch, got %d",postCount)}
 if err:=client.RequestBuild(context.Background(),100,strings.Repeat("b",40));err==nil {
  t.Fatal("stale PR SHA accepted")
 }
 if postCount!=1{t.Fatal("stale request dispatched")}
 client.Token=""
 if err:=client.RequestBuild(context.Background(),100,sha);err==nil||!strings.Contains(err.Error(),"TASK_MECCA_GITHUB_TOKEN"){
  t.Fatalf("missing token should be explained: %v",err)
 }
}
func TestRequestBuildBlocksForkOrWrongTarget(t *testing.T) {
 sha:=strings.Repeat("c",40)
 for _,tc:=range []struct{name,repo,target,state string}{
  {"fork","outsider/fork","dev","open"},
  {"base","silverkhan/TaskMecca","main","open"},
  {"closed","silverkhan/TaskMecca","dev","closed"},
 }{
  t.Run(tc.name,func(t *testing.T){
   posts:=0
   srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
    if r.Method=="POST"{posts++;w.WriteHeader(204);return}
    fmt.Fprintf(w,`{"number":100,"state":"%s","base":{"ref":"%s"},"head":{"sha":"%s","repo":{"full_name":"%s"}}}`,tc.state,tc.target,sha,tc.repo)
   }))
   defer srv.Close()
   client:=Client{APIURL:srv.URL,HTTP:srv.Client(),Token:"test-token"}
   if err:=client.RequestBuild(context.Background(),100,sha);err==nil{t.Fatal("untrusted PR build allowed")}
   if posts>0{t.Fatal("untrusted PR submitted to Actions")}
  })
 }
}
func TestManualWorkflowMatchingRequiresDevAndExactPRSHA(t *testing.T){
 sha:=strings.Repeat("d",40)
 run:=ghRun{Event:"workflow_dispatch",HeadBranch:"dev",DisplayTitle:"Preview PR #60 "+sha}
 if !runMatchesPR(run,60,sha){t.Fatal("matching dispatched run not found")}
 for _,bad:=range []ghRun{
  {Event:"workflow_dispatch",HeadBranch:"main",DisplayTitle:run.DisplayTitle},
  {Event:"workflow_dispatch",HeadBranch:"dev",DisplayTitle:"Preview PR #60 "+strings.Repeat("e",40)},
  {Event:"workflow_dispatch",HeadBranch:"dev",DisplayTitle:"Preview PR #61 "+sha},
  {Event:"push",HeadBranch:"dev",DisplayTitle:run.DisplayTitle},
 } {
  if runMatchesPR(bad,60,sha){t.Errorf("invalid manual run matched: %+v",bad)}
 }
}
