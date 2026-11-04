package activity

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "sync/atomic"
    "testing"
    "time"
)

func TestGitHubCIFetchBindsExactRevisionAndSkipsUnfinishedJobs(t *testing.T){
    const repo="silverkhan/TaskMecca"
    sha:=strings.Repeat("a",40)
    var calls atomic.Int32
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        calls.Add(1)
        if got:=r.Header.Get("Authorization");got!="Bearer test-token"{
            t.Errorf("missing auth header");w.WriteHeader(401);return
        }
        w.Header().Set("Content-Type","application/json")
        switch r.URL.Path{
        case "/repos/"+repo+"/actions/runs/100":
            _=json.NewEncoder(w).Encode(map[string]any{
                "id":100,"name":"CI","head_sha":sha,"run_attempt":2,"status":"completed",
            })
        case "/repos/"+repo+"/actions/runs/100/jobs":
            if r.URL.Query().Get("page")!="1"{t.Errorf("unexpected pagination: %s",r.URL.RawQuery)}
            _=json.NewEncoder(w).Encode(map[string]any{
                "total_count":2,"jobs":[]any{
                    map[string]any{
                        "id":991,"run_id":100,"name":"Go runtime","status":"completed",
                        "conclusion":"success","started_at":"2026-11-04T01:00:00Z",
                        "completed_at":"2026-11-04T01:01:20Z",
                        "html_url":"https://github.com/"+repo+"/actions/runs/100/job/991",
                    },
                    map[string]any{
                        "id":992,"run_id":100,"name":"Build pending","status":"in_progress",
                        "conclusion":nil,"started_at":"2026-11-04T01:00:00Z",
                        "completed_at":nil,
                        "html_url":"https://github.com/"+repo+"/actions/runs/100/job/992",
                    },
                },
            })
        default:
            t.Errorf("unexpected GitHub endpoint %s",r.URL.Path)
            http.NotFound(w,r)
        }
    }))
    defer server.Close()
    client:=GitHubCIClient{Token:"test-token",BaseURL:server.URL,HTTP:&http.Client{Timeout:time.Second}}
    records,err:=client.FetchGitHubCI(context.Background(),"B-589",repo,100,sha,"Go runtime",FullRegression,"all-go")
    if err!=nil{t.Fatal(err)}
    if len(records)!=1||records[0].Type!=FullRegression||records[0].RunAttempt!=2||
       records[0].CompletedAt.Sub(records[0].StartedAt)!=80*time.Second{
        t.Fatalf("wrong normalized evidence %+v",records)
    }
    if calls.Load()!=2{t.Fatalf("unexpected API calls: %d",calls.Load())}
    if _,err:=client.FetchGitHubCI(context.Background(),"B-589",repo,100,strings.Repeat("b",40),"Go runtime",FullRegression,"all-go");err==nil{
        t.Fatal("wrong commit SHA accepted")
    }
    if _,err:=client.FetchGitHubCI(context.Background(),"B-589",repo,100,sha,"",FullRegression,"all-go");err==nil{
        t.Fatal("classifying every job without exact name accepted")
    }
}
