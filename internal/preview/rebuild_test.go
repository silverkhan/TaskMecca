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

func TestPRRebuildDispatchUsesTrustedDevAndPinnedHead(t *testing.T){
 const sha="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
 calls:=0
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("Content-Type","application/json")
  switch r.URL.Path{
  case "/repos/silverkhan/TaskMecca/pulls/218":
   fmt.Fprintf(w,`{"number":218,"state":"open","base":{"ref":"dev"},"head":{"sha":"%s","repo":{"full_name":"silverkhan/TaskMecca"}}}`,sha)
  case "/repos/silverkhan/TaskMecca/actions/workflows/go-artifacts.yml/dispatches":
   if r.Method!="POST"||r.Header.Get("Authorization")!="Bearer test-token"{t.Errorf("unexpected dispatch method/auth")}
   var body struct{Ref string `json:"ref"`; Inputs map[string]string `json:"inputs"`}
   if err:=json.NewDecoder(r.Body).Decode(&body);err!=nil{t.Error(err)}
   if body.Ref!="dev"||body.Inputs["preview_pr"]!="218"||body.Inputs["preview_sha"]!=sha{
    t.Errorf("untrusted checkout request: %+v",body)
   }
   calls++
   w.WriteHeader(http.StatusNoContent)
  default:
   http.Error(w,"invalid route",404)
  }
 }))
 defer server.Close()
 client:=Client{HTTP:server.Client(),APIURL:server.URL,Token:"test-token"}
 if err:=client.RequestBuild(context.Background(),218,sha);err!=nil{t.Fatal(err)}
 if calls!=1{t.Fatalf("want one dispatch, got %d",calls)}
 if err:=client.RequestBuild(context.Background(),218,strings.Repeat("b",40));err==nil{
  t.Fatal("stale PR SHA accepted")
 }
 if calls!=1{t.Fatal("stale SHA dispatched an action")}
}

func TestPRRebuildRejectsForkClosedPRAndMissingWriteToken(t *testing.T){
 const sha="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
 noToken:=Client{APIURL:"https://api.github.com"}
 if err:=noToken.RequestBuild(context.Background(),99,sha);err==nil||!strings.Contains(err.Error(),"Actions:write"){
  t.Fatalf("missing credentials did not give actionable help: %v",err)
 }
 for _,kind:=range []string{"fork","closed","stable"}{
  t.Run(kind,func(t *testing.T){
   dispatches:=0
   server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
    if strings.Contains(r.URL.Path,"/dispatches"){dispatches++;w.WriteHeader(204);return}
    state,base,headRepo:="open","dev","silverkhan/TaskMecca"
    switch kind{case "fork":headRepo="outside/TaskMecca";case "closed":state="closed";case "stable":base="main"}
    fmt.Fprintf(w,`{"number":100,"state":"%s","base":{"ref":"%s"},"head":{"sha":"%s","repo":{"full_name":"%s"}}}`,state,base,sha,headRepo)
   }))
   defer server.Close()
   client:=Client{HTTP:server.Client(),APIURL:server.URL,Token:"test-token"}
   if err:=client.RequestBuild(context.Background(),100,sha);err==nil {
    t.Fatal("untrusted PR accepted")
   }
   if dispatches!=0{t.Fatal("GitHub workflow dispatched despite failed PR trust verification")}
  })
 }
}

func TestCatalogIncludesOnlyExactTrustedDispatchRun(t *testing.T){
 const sha="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("Content-Type","application/json")
  switch r.URL.Path{
  case "/repos/silverkhan/TaskMecca/pulls":
   fmt.Fprintf(w,`[{"number":218,"title":"Smoke","updated_at":"2026-10-14T10:10:00Z","base":{"ref":"dev"},"head":{"sha":"%s","ref":"smoke","repo":{"full_name":"silverkhan/TaskMecca"}}}]`,sha)
  case "/repos/silverkhan/TaskMecca/actions/workflows/go-artifacts.yml/runs":
   if r.URL.Query().Get("event")=="pull_request"{fmt.Fprint(w,`{"workflow_runs":[]}`);return}
   fmt.Fprintf(w,`{"workflow_runs":[
      {"id":801,"status":"completed","conclusion":"success","event":"workflow_dispatch","head_branch":"main","display_title":"Preview PR #218 %s","created_at":"2026-10-14T10:12:00Z"},
      {"id":802,"status":"completed","conclusion":"success","event":"workflow_dispatch","head_branch":"dev","display_title":"Preview PR #218 %s","created_at":"2026-10-14T10:13:00Z"},
      {"id":803,"status":"completed","conclusion":"success","event":"workflow_dispatch","head_branch":"dev","display_title":"Preview PR #218 %s","created_at":"2026-10-14T10:14:00Z"}
    ]}`,sha,strings.Repeat("b",40),sha)
  case "/repos/silverkhan/TaskMecca/actions/runs/803/artifacts":
   fmt.Fprintf(w,`{"artifacts":[{"id":55,"name":"task-mecca-preview-pr-218-%s","expired":false}]}`,sha)
  default:
   http.Error(w,"wrong route",404)
  }
 }))
 defer server.Close()
 client:=Client{HTTP:server.Client(),APIURL:server.URL}
 builds,err:=client.ListBuilds(context.Background(),20)
 if err!=nil{t.Fatal(err)}
 if len(builds)!=1||builds[0].BuildStatus!="success"||builds[0].RunID!=803||
  builds[0].ArtifactID!=55||builds[0].HeadSHA!=sha{
  t.Fatalf("manual SHA provenance not linked: %+v",builds)
 }
}
