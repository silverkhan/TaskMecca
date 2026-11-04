package preview

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// readToken caches existing CLI authentication in memory, without widening the
// explicit Actions:write credential requirement for RequestBuild.
func (c *Client) readToken(ctx context.Context) string {
	if c.Token != "" {
		return c.Token
	}
	c.authMu.Lock()
	defer c.authMu.Unlock()
	if !c.fallbackResolved {
		c.fallbackResolved = true
		if c.TokenFallback != nil {
			token, err := c.TokenFallback(ctx)
			if err == nil && token != "" && !strings.ContainsAny(token, "\r\n\t ") {
				c.fallbackToken = token
			}
		}
	}
	return c.fallbackToken
}
func (c *Client) cachedReadToken() string {
	if c.Token != "" {
		return c.Token
	}
	c.authMu.Lock()
	defer c.authMu.Unlock()
	return c.fallbackToken
}

type APIError struct {
	Code   string
	Status int
}

func (e *APIError) Error() string {
	switch e.Code {
	case "github_authentication":
		return "GitHub authentication failed; check the existing gh login or TASK_MECCA_GITHUB_TOKEN"
	case "github_rate_limited":
		return "GitHub API request limit reached; retry after the limit resets"
	case "github_access_denied":
		return "GitHub API access denied; check repository and Actions read permissions"
	case "github_not_accessible":
		return "GitHub repository or workflow is not accessible"
	default:
		return "GitHub API request failed; retry the build lookup"
	}
}
func APIErrorCode(err error) string {
	var api *APIError
	if errors.As(err, &api) {
		return api.Code
	}
	return "preview_lookup_failed"
}

func githubAPIError(resp *http.Response) error {
	code := "github_unavailable"
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		code = "github_authentication"
	case http.StatusForbidden:
		code = "github_access_denied"
		if resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != "" {
			code = "github_rate_limited"
		}
	case http.StatusTooManyRequests:
		code = "github_rate_limited"
	case http.StatusNotFound:
		code = "github_not_accessible"
	}
	return &APIError{Code: code, Status: resp.StatusCode}
}

// Only the historical YAML-comment title is eligible for recovery. The title
// carries no PR identity: trusted workflow/repositories/run relation and exact
// current-head artifact supply that binding. DownloadVerified still checks ZIP
// provenance, checksum and return protocol; discovery never grants execution.
type ghWorkflow struct {
	ID    int64  `json:"id"`
	Path  string `json:"path"`
	State string `json:"state"`
}
type catalogArtifacts struct {
	items ghArtifacts
	err   error
}

func legacyDispatchCandidate(run ghRun) bool {
	return run.Event == "workflow_dispatch" && run.HeadBranch == "dev" && run.DisplayTitle == "Preview PR"
}
func legacyDispatchTrusted(run ghRun, w *ghWorkflow) bool {
	return w != nil && legacyDispatchCandidate(run) && run.ID > 0 && run.Status == "completed" && run.Conclusion == "success" && validSHA(run.HeadSHA) &&
		run.WorkflowID == w.ID && run.Path == w.Path && run.Repository != nil && run.HeadRepository != nil && run.Repository.ID > 0 && run.Repository.ID == run.HeadRepository.ID &&
		strings.EqualFold(run.Repository.FullName, Repository) && strings.EqualFold(run.HeadRepository.FullName, Repository)
}
func legacyArtifactMatches(a ghArtifact, run ghRun) bool {
	w := a.WorkflowRun
	return w != nil && w.ID == run.ID && w.HeadBranch == "dev" && w.HeadSHA == run.HeadSHA && w.RepositoryID == run.Repository.ID && w.HeadRepositoryID == run.HeadRepository.ID
}
