// Package rolepolicy is the common, read-only role and review policy used by
// contract projections and supported execution/acceptance gates. It is not a
// second approval manager and cannot fence manual or legacy editor writes.
package rolepolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const Fence = "```task-mecca-role-policy\n"

type Owner struct {
	Role      string `json:"role"`
	AgentPath string `json:"agent_path,omitempty"`
	NativeID  string `json:"native_id,omitempty"`
}
type Exception struct {
	Reason          string   `json:"reason"`
	Evidence        string   `json:"evidence"`
	Scope           []string `json:"scope"`
	ApprovalEventID string   `json:"approval_event_id"`
	ResumeWhen      string   `json:"resume_when"`
}
type Review struct {
	AttemptID       string `json:"attempt_id"`
	ReportFile      string `json:"report_file"`
	SourceWorkspace string `json:"source_workspace"`
}
type Policy struct {
	SchemaVersion  int        `json:"schema_version"`
	Implementation Owner      `json:"implementation_owner"`
	Operational    Owner      `json:"operational_owner"`
	Reviewer       Owner      `json:"review_owner"`
	Workspace      string     `json:"workspace"`
	Method         string     `json:"method"`
	Scope          []string   `json:"scope"`
	Exception      *Exception `json:"exception,omitempty"`
	Review         *Review    `json:"review,omitempty"`
}

// Identity is populated from the runtime ledger, never from a caller's role,
// display name, independent:true flag, or changed turn identifier.
type Identity struct {
	AttemptID      string `json:"attempt_id"`
	NativeID       string `json:"native_id"`
	RuntimeAgentID string `json:"runtime_agent_id,omitempty"`
	Provider       string `json:"provider"`
	AgentPath      string `json:"agent_path"`
	TaskID         string `json:"task_id"`
	Bound          bool   `json:"bound"`
	Observed       bool   `json:"observed"`
}
type View struct {
	Policy            *Policy `json:"policy,omitempty"`
	Implementation    string  `json:"implementation"`
	Review            string  `json:"review"`
	Allowed           bool    `json:"allowed"`
	Reason            string  `json:"reason"`
	ResumeWhen        string  `json:"resume_when"`
	ExceptionApproved bool    `json:"exception_approved"`
	Legacy            bool    `json:"legacy"`
}

func Parse(text string) (*Policy, error) {
	i := strings.Index(text, Fence)
	if i < 0 {
		return nil, nil
	}
	body := text[i+len(Fence):]
	j := strings.Index(body, "\n```")
	if j < 0 || strings.Contains(body[j+4:], Fence) {
		return nil, errors.New("role_policy_invalid_fence")
	}
	d := json.NewDecoder(strings.NewReader(body[:j]))
	d.DisallowUnknownFields()
	var p Policy
	if err := d.Decode(&p); err != nil {
		return nil, fmt.Errorf("role_policy_invalid: %w", err)
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, errors.New("role_policy_trailing_json")
	}
	if p.SchemaVersion != 1 {
		return nil, errors.New("role_policy_schema_unsupported")
	}
	return &p, nil
}
func WorkerPath(path string) bool {
	return strings.HasPrefix(path, "/root/controller/") && !strings.Contains(strings.TrimPrefix(path, "/root/controller/"), "/") && !strings.HasPrefix(path, "/root/controller/sub-") && path != "/root/controller/"
}
func Role(path string) string {
	if path == "/root/controller" {
		return "controller"
	}
	if WorkerPath(path) {
		return "worker"
	}
	return "unknown"
}
func validScope(scope []string) bool {
	if len(scope) == 0 {
		return false
	}
	for _, s := range scope {
		if s == "" || s == "." || filepath.IsAbs(s) || filepath.ToSlash(filepath.Clean(s)) != s || s == ".." || strings.HasPrefix(s, "../") || strings.ContainsAny(s, "*?\\") {
			return false
		}
	}
	return true
}
func Covers(scope []string, path string) bool {
	for _, s := range scope {
		if path == s || strings.HasPrefix(path, s+"/") {
			return true
		}
	}
	return false
}
func Evaluate(p *Policy, actual Identity, workspace string, approved bool) View {
	v := View{Policy: p, Implementation: "worker", Review: "unknown", Legacy: p == nil, ExceptionApproved: approved, ResumeWhen: "Bind the exact task/native recipient and verify its implementation workspace."}
	deny := func(reason string) View { v.Reason = reason; return v }
	if !actual.Bound || !actual.Observed || actual.NativeID == "" || actual.Provider == "" || actual.TaskID == "" {
		return deny("recipient_identity_unknown")
	}
	if p == nil {
		if Role(actual.AgentPath) != "worker" {
			return deny("worker_default_requires_worker")
		}
		v.Allowed = true
		v.Reason = "existing_worker_authorization_applies"
		v.ResumeWhen = ""
		return v
	}
	if p.Exception != nil && p.Exception.ResumeWhen != "" {
		v.ResumeWhen = p.Exception.ResumeWhen
	}
	if p.Method == "controller_exception" {
		v.Implementation = "controller_exception"
	}
	if p.Operational.Role != "controller" || p.Operational.AgentPath != "/root/controller" || !validScope(p.Scope) || !filepath.IsAbs(p.Workspace) || filepath.Clean(p.Workspace) != filepath.Clean(workspace) {
		return deny("owners_scope_or_workspace_unverified")
	}
	if p.Implementation.Role != Role(actual.AgentPath) || p.Implementation.AgentPath != actual.AgentPath || (p.Implementation.NativeID != "" && p.Implementation.NativeID != actual.NativeID) {
		return deny("recipient_role_or_identity_mismatch")
	}
	if p.Method == "worker" && p.Implementation.Role == "worker" {
		v.Allowed = true
		v.Reason = "existing_worker_authorization_applies"
		v.ResumeWhen = ""
		return v
	}
	if p.Method != "controller_exception" || p.Implementation.Role != "controller" {
		return deny("implementation_method_mismatch")
	}
	x := p.Exception
	if x != nil && x.ResumeWhen != "" {
		v.ResumeWhen = x.ResumeWhen
	}
	if x == nil || strings.TrimSpace(x.Reason) == "" || strings.TrimSpace(x.Evidence) == "" || !validScope(x.Scope) || x.ApprovalEventID == "" || !approved || p.Reviewer.NativeID == "" || p.Reviewer.NativeID == actual.NativeID || p.Reviewer.AgentPath == "" || p.Reviewer.Role != Role(p.Reviewer.AgentPath) || p.Reviewer.Role == "unknown" {
		return deny("restricted_exception_pending")
	}
	for _, s := range p.Scope {
		if !Covers(x.Scope, s) {
			return deny("exception_scope_mismatch")
		}
	}
	v.Allowed = true
	v.Reason = "approved_restricted_controller_exception"
	v.ResumeWhen = ""
	return v
}

// Source is a bounded exact file hash set. Deletions are represented by "absent".
// Metadata/merge commits do not invalidate it; a covered product change does.
type Source struct {
	Files map[string]string `json:"files"`
}

func PolicyDigest(p *Policy) string {
	if p == nil {
		return ""
	}
	copy := *p
	copy.Review = nil
	if p.Exception != nil {
		x := *p.Exception
		x.ApprovalEventID = ""
		copy.Exception = &x
	}
	b, _ := json.Marshal(copy)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (s Source) Digest() string {
	b, _ := json.Marshal(s)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type ReviewStatement struct {
	TaskID                 string `json:"task_id"`
	ReviewerAttemptID      string `json:"reviewer_attempt_id"`
	ImplementationNativeID string `json:"implementation_native_id"`
	Source                 Source `json:"source"`
	ReportSHA256           string `json:"report_sha256"`
	Result                 string `json:"result"`
}

const ReviewMarker = "TASK_MECCA_REVIEW "

func ValidateReview(task string, implementation, reviewer Identity, p *Policy, statement ReviewStatement, reportSHA string, current Source) (string, error) {
	if (implementation.NativeID != "" && implementation.NativeID == reviewer.NativeID) || (implementation.RuntimeAgentID != "" && implementation.RuntimeAgentID == reviewer.RuntimeAgentID) {
		return "self", errors.New("same_native_self_review")
	}
	if p == nil || !implementation.Bound || !implementation.Observed || !reviewer.Bound || !reviewer.Observed || implementation.NativeID == "" || reviewer.NativeID == "" || implementation.Provider == "" || reviewer.Provider == "" || implementation.TaskID != task || reviewer.TaskID != task || statement.TaskID != task || statement.ReviewerAttemptID != reviewer.AttemptID || reviewer.AttemptID == "" || statement.ImplementationNativeID != implementation.NativeID {
		return "unknown", errors.New("review_identity_or_task_unverified")
	}
	if p.Reviewer.NativeID != "" && p.Reviewer.NativeID != reviewer.NativeID {
		return "unknown", errors.New("designated_reviewer_mismatch")
	}
	if p.Reviewer.AgentPath != "" && p.Reviewer.AgentPath != reviewer.AgentPath {
		return "unknown", errors.New("reviewer_path_mismatch")
	}
	if p.Reviewer.Role != Role(reviewer.AgentPath) || p.Reviewer.Role == "unknown" {
		return "unknown", errors.New("reviewer_role_mismatch")
	}
	if statement.Result != "accepted" || reportSHA == "" || reportSHA != statement.ReportSHA256 || len(statement.Source.Files) == 0 || len(current.Files) == 0 || statement.Source.Digest() != current.Digest() {
		return "unknown", errors.New("review_result_report_or_source_changed")
	}
	for path := range current.Files {
		if !Covers(p.Scope, path) {
			return "unknown", errors.New("review_source_outside_scope")
		}
	}
	for _, path := range p.Scope {
		found := false
		for f := range current.Files {
			if Covers([]string{path}, f) {
				found = true
			}
		}
		if !found {
			return "unknown", errors.New("review_scope_incomplete")
		}
	}
	return "independent", nil
}
