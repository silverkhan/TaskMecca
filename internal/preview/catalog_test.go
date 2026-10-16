package preview

import (
    "context"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestCatalogExactHeadSHAAndTrustPolicy(t *testing.T) {
    const a="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    const b="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    calls:=map[string]int{}
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        calls[r.URL.Path]++
        w.Header().Set("Content-Type","application/json")
        switch r.URL.Path {
        case "/repos/silverkhan/TaskMecca/pulls":
            fmt.Fprintf(w,`[
                {"number":104,"title":"fork PR","updated_at":"2026-10-14T13:00:00Z","base":{"ref":"dev"},"head":{"ref":"outside","sha":"%s","repo":{"full_name":"attacker/TaskMecca"}}},
                {"number":103,"title":"stable only","base":{"ref":"main"},"head":{"ref":"main","sha":"%s","repo":{"full_name":"silverkhan/TaskMecca"}}},
                {"number":102,"title":"new running","updated_at":"2026-10-14T12:00:00Z","base":{"ref":"dev"},"head":{"ref":"develop","sha":"%s","repo":{"full_name":"silverkhan/TaskMecca"}}},
                {"number":101,"title":"verified artifact","updated_at":"2026-10-14T11:00:00Z","base":{"ref":"dev"},"head":{"ref":"feature","sha":"%s","repo":{"full_name":"silverkhan/TaskMecca"}}}
            ]`,a,a,b,a)
        case "/repos/silverkhan/TaskMecca/actions/workflows/go-artifacts.yml/runs":
            fmt.Fprint(w,`{"workflow_runs":[
                {"id":700,"status":"completed","conclusion":"success","event":"pull_request","updated_at":"2026-10-14T12:30:00Z","pull_requests":[{"number":101}]},
                {"id":710,"status":"in_progress","event":"pull_request","updated_at":"2026-10-14T12:40:00Z","pull_requests":[{"number":102}]}
            ]}`)
        case "/repos/silverkhan/TaskMecca/actions/runs/700/artifacts":
            fmt.Fprintf(w,`{"artifacts":[{"id":6,"name":"task-mecca-preview-pr-101-%s","expired":false},
                {"id":7,"name":"task-mecca-preview-pr-101-%s","expired":false}]}`,b,a)
        default:
            http.Error(w,"not found",http.StatusNotFound)
        }
    }))
    defer server.Close()
    client:=Client{HTTP:server.Client(),APIURL:server.URL}
    result,err:=client.ListBuilds(context.Background(),20)
    if err!=nil {t.Fatal(err)}
    if len(result)!=3 {t.Fatalf("want 3 same-target PRs, got %+v",result)}
    if result[0].Number!=101 || result[0].BuildStatus!="success"||result[0].ArtifactID!=7||
        result[0].HeadSHA!=a || result[0].ExecutionAllowed {
        t.Fatalf("head SHA build incorrectly approved: %+v",result[0])
    }
    if result[1].Number!=104||result[1].BuildStatus!="blocked"||result[1].ArtifactID!=0 {
        t.Fatalf("fork PR incorrectly allowed: %+v",result[1])
    }
    if result[2].Number!=102||result[2].BuildStatus!="in_progress"||result[2].ExecutionAllowed {
        t.Fatalf("running build incorrectly classified: %+v",result[2])
    }
    if calls["/repos/silverkhan/TaskMecca/actions/runs/700/artifacts"]!=1 {t.Fatalf("artifact calls %+v",calls)}
}

func TestCatalogNeverAcceptsStaleOrExpiredArtifact(t *testing.T) {
    sha:=strings.Repeat("c",40)
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        w.Header().Set("Content-Type","application/json")
        switch r.URL.Path {
        case "/repos/silverkhan/TaskMecca/pulls":
            fmt.Fprintf(w,`[{"number":88,"title":"test","base":{"ref":"dev"},"head":{"ref":"feature","sha":"%s","repo":{"full_name":"silverkhan/TaskMecca"}}}]`,sha)
        case "/repos/silverkhan/TaskMecca/actions/workflows/go-artifacts.yml/runs":
            fmt.Fprint(w,`{"workflow_runs":[{"id":900,"status":"completed","conclusion":"success","event":"pull_request","pull_requests":[{"number":88}]}]}`)
        case "/repos/silverkhan/TaskMecca/actions/runs/900/artifacts":
            fmt.Fprintf(w,`{"artifacts":[{"id":999,"name":"task-mecca-preview-pr-88-%s","expired":true}]}`,sha)
        default:
            http.Error(w,"not found",404)
        }
    }))
    defer server.Close()
    c:=Client{HTTP:server.Client(),APIURL:server.URL}
    result,err:=c.ListBuilds(context.Background(),20)
    if err!=nil{t.Fatal(err)}
    if len(result)!=1||result[0].BuildStatus!="expired"||result[0].ExecutionAllowed {
        t.Fatalf("expired preview must never be runnable: %+v",result)
    }
}

func TestCatalogHandlesNetworkAuthenticationAndBadSHA(t *testing.T) {
    for _,status:=range []int{http.StatusForbidden,http.StatusUnauthorized,http.StatusTooManyRequests} {
        server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
            w.WriteHeader(status)
        }))
        c:=Client{HTTP:server.Client(),APIURL:server.URL}
        _,err:=c.ListBuilds(context.Background(),20)
        server.Close()
        if err==nil||!strings.Contains(err.Error(),"GitHub") {t.Fatalf("status %d: %v",status,err)}
    }
    for _,sha:=range []string{"",strings.Repeat("z",40),strings.Repeat("a",39),strings.Repeat("a",40)} {
        want:=len(sha)==40&&sha[0]=='a'
        if validSHA(sha)!=want {t.Fatalf("validSHA(%q) unexpected",sha)}
    }
}
