package preview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetadataReusesExistingAuthenticationWithoutEnablingDispatch(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-private" {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer s.Close()
	c := Client{HTTP: s.Client(), APIURL: s.URL, TokenFallback: func(context.Context) (string, error) { calls++; return "fixture-private", nil }}
	for i := 0; i < 3; i++ {
		var out any
		if err := c.get(context.Background(), "/metadata", &out); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 || c.Token != "" {
		t.Fatal("existing authentication was not kept in the read-only client cache")
	}
	if err := c.RequestBuild(context.Background(), 348, strings.Repeat("a", 40)); err == nil || !strings.Contains(err.Error(), "Actions:write") {
		t.Fatal("read authentication widened dispatch gate")
	}
}
func TestMetadataErrorsAreClassifiedAndNeverReflectSecrets(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		status                 int
		remaining, retry, code string
	}{
		{"authentication", 401, "", "", "github_authentication"}, {"access", 403, "", "", "github_access_denied"},
		{"quota", 403, "0", "", "github_rate_limited"}, {"secondary", 403, "", "10", "github_rate_limited"},
		{"limited", 429, "", "", "github_rate_limited"}, {"missing", 404, "", "", "github_not_accessible"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-RateLimit-Remaining", tc.remaining)
				w.Header().Set("Retry-After", tc.retry)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, "fixture-private response account detail")
			}))
			defer s.Close()
			fallback := 0
			c := Client{HTTP: s.Client(), APIURL: s.URL, Token: "fixture-private", TokenFallback: func(context.Context) (string, error) { fallback++; return "other-credential", nil }}
			var out any
			err := c.get(context.Background(), "/metadata", &out)
			if err == nil || APIErrorCode(err) != tc.code || strings.Contains(err.Error(), "fixture-private") || strings.Contains(err.Error(), "account detail") || fallback != 0 {
				t.Fatalf("unsafe or misclassified error code=%s fallback=%d", APIErrorCode(err), fallback)
			}
		})
	}
	c := Client{TokenFallback: func(context.Context) (string, error) { return "", errors.New("sensitive callback detail") }}
	if c.readToken(context.Background()) != "" {
		t.Fatal("failed credential lookup returned a credential")
	}
}
func TestCatalogRecoversOnlyProvenanceBoundLegacyDispatch(t *testing.T) {
	sha := strings.Repeat("a", 40)
	dev := strings.Repeat("d", 40)
	for _, variant := range []string{"valid", "wrong-workflow", "wrong-path", "fork-run", "fork-head", "different-repo-id", "wrong-title", "wrong-branch", "invalid-dev-sha", "artifact-run", "artifact-sha", "artifact-repo", "artifact-head-repo", "missing-relation", "stale-head", "expired", "artifact-error", "workflow-error"} {
		t.Run(variant, func(t *testing.T) {
			run := ghRun{ID: 801, Event: "workflow_dispatch", DisplayTitle: "Preview PR", HeadBranch: "dev", HeadSHA: dev, WorkflowID: 42, Path: ".github/workflows/go-artifacts.yml", Status: "completed", Conclusion: "success", Repository: &ghRepository{ID: 7, FullName: Repository}, HeadRepository: &ghRepository{ID: 7, FullName: Repository}}
			switch variant {
			case "wrong-workflow":
				run.WorkflowID = 99
			case "wrong-path":
				run.Path = ".github/workflows/other.yml"
			case "fork-run":
				run.Repository.FullName = "outside/TaskMecca"
			case "fork-head":
				run.HeadRepository.FullName = "outside/TaskMecca"
			case "different-repo-id":
				run.HeadRepository.ID = 8
			case "wrong-title":
				run.DisplayTitle = "Preview PR anything"
			case "wrong-branch":
				run.HeadBranch = "main"
			case "invalid-dev-sha":
				run.HeadSHA = "invalid"
			}
			artifactCalls := 0
			workflowCalls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/repos/" + Repository + "/pulls":
					fmt.Fprintf(w, `[{"number":348,"base":{"ref":"dev"},"head":{"ref":"feature","sha":"%s","repo":{"full_name":"%s"}}},{"number":349,"base":{"ref":"dev"},"head":{"sha":"%s","repo":{"full_name":"%s"}}}]`, sha, Repository, sha, Repository)
				case "/repos/" + Repository + "/actions/workflows/go-artifacts.yml/runs":
					if r.URL.Query().Get("event") == "pull_request" {
						fmt.Fprint(w, `{"workflow_runs":[]}`)
					} else {
						_ = json.NewEncoder(w).Encode(ghRuns{Runs: []ghRun{run}})
					}
				case "/repos/" + Repository + "/actions/workflows/go-artifacts.yml":
					workflowCalls++
					if variant == "workflow-error" {
						w.Header().Set("X-RateLimit-Remaining", "0")
						w.WriteHeader(403)
						return
					}
					fmt.Fprint(w, `{"id":42,"path":".github/workflows/go-artifacts.yml","state":"active"}`)
				case "/repos/" + Repository + "/actions/runs/801/artifacts":
					artifactCalls++
					if variant == "artifact-error" {
						w.WriteHeader(403)
						return
					}
					relation := map[string]any{"id": 801, "head_sha": dev, "head_branch": "dev", "repository_id": 7, "head_repository_id": 7}
					switch variant {
					case "artifact-run":
						relation["id"] = 802
					case "artifact-sha":
						relation["head_sha"] = sha
					case "artifact-repo":
						relation["repository_id"] = 8
					case "artifact-head-repo":
						relation["head_repository_id"] = 8
					}
					name := "task-mecca-preview-pr-348-" + sha
					if variant == "stale-head" {
						name = "task-mecca-preview-pr-348-" + strings.Repeat("b", 40)
					}
					a := map[string]any{"id": 900, "name": name, "expired": variant == "expired", "workflow_run": relation}
					if variant == "missing-relation" {
						delete(a, "workflow_run")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"artifacts": []any{a}})
				default:
					http.NotFound(w, r)
				}
			}))
			defer s.Close()
			c := Client{HTTP: s.Client(), APIURL: s.URL}
			builds, err := c.ListBuilds(context.Background(), 20)
			if variant == "workflow-error" {
				if err == nil || APIErrorCode(err) != "github_rate_limited" {
					t.Fatal("workflow lookup failure became no builds")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var b Build
			for _, x := range builds {
				if x.Number == 348 {
					b = x
				}
			}
			if variant == "valid" {
				if b.BuildStatus != "success" || b.RunID != 801 || b.ArtifactID != 900 || b.ExecutionAllowed {
					t.Fatalf("valid provenance not recovered: %+v", b)
				}
				if artifactCalls != 1 || workflowCalls != 1 {
					t.Fatal("legacy metadata not cached across PRs")
				}
			} else if b.BuildStatus == "success" {
				t.Fatalf("unsafe %s accepted", variant)
			}
			if variant == "expired" && b.BuildStatus != "expired" {
				t.Fatal("expired artifact was hidden")
			}
			if variant == "artifact-error" && b.BuildStatus != "unavailable" {
				t.Fatal("lookup failure became no build")
			}
		})
	}
}

type recoveryRoundTrip func(*http.Request) (*http.Response, error)

func (f recoveryRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type recoveryReadFailure struct{}

func (recoveryReadFailure) Read([]byte) (int, error) {
	return 0, errors.New("signed-url?sig=fixture-sensitive")
}
func (recoveryReadFailure) Close() error { return nil }
func TestDownloadVerifiedRedactsNetworkAndTransferErrors(t *testing.T) {
	build := Build{Number: 348, HeadSHA: strings.Repeat("a", 40), RunID: 801, ArtifactID: 900, ArtifactName: "task-mecca-preview-pr-348-" + strings.Repeat("a", 40), BuildStatus: "success"}
	for _, kind := range []string{"network", "authenticated-network", "body"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			c := Client{APIURL: "https://api.github.com", TokenFallback: func(context.Context) (string, error) { return "fixture-sensitive", nil }}
			c.HTTP = &http.Client{Transport: recoveryRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				if kind == "authenticated-network" && calls == 1 {
					return &http.Response{StatusCode: 401, Body: recoveryReadFailure{}, Header: make(http.Header)}, nil
				}
				if kind == "body" {
					return &http.Response{StatusCode: 200, Body: recoveryReadFailure{}, Header: make(http.Header)}, nil
				}
				return nil, errors.New("https://signed-download/?sig=fixture-sensitive")
			})}
			_, err := c.DownloadVerified(context.Background(), build, t.TempDir(), "linux", "amd64")
			if err == nil || strings.Contains(err.Error(), "fixture-sensitive") || strings.Contains(err.Error(), "signed-download") {
				t.Fatal("artifact error exposed credential-bearing details")
			}
		})
	}
}
