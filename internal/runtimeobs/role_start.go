package runtimeobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

// RoleStart is an explicit formal collection request, not creation, delivery,
// claim, or implementation permission. Parent identity is independently checked
// against the live host store; caller metadata alone is never a receipt.
type RoleStart struct {
	CanonicalProject string     `json:"canonical_project"`
	SourceWorktree   string     `json:"source_worktree"`
	TaskID           string     `json:"task_id"`
	AssignmentID     string     `json:"assignment_id"`
	Role             string     `json:"role"`
	AgentPath        string     `json:"agent_path"`
	ParentThreadID   string     `json:"parent_thread_id"`
	Selector         T3Selector `json:"selector"`
}

func ObserveT3RoleStart(in RoleStart, now time.Time) (WorkerStartResult, error) {
	store, err := localT3Store()
	if err != nil {
		return WorkerStartResult{Status: "collection_gap", Stage: "scope", Diagnostic: err.Error()}, err
	}
	return observeRoleCollection(in, now, store.verifyRoleScope, ObserveT3Execution)
}

func observeRoleCollection(in RoleStart, now time.Time, scope func(RoleStart) error, observe func(string, T3Selector, time.Time) (Attempt, T3Receipt, error)) (WorkerStartResult, error) {
	legacy := WorkerStart{in.CanonicalProject, in.SourceWorktree, in.TaskID, in.AssignmentID, in.AgentPath, in.Selector}
	fail := func(stage, reason string) (WorkerStartResult, error) {
		return WorkerStartResult{Status: "collection_gap", Stage: stage, Context: legacy, ReceivedAt: now.UTC().Format(time.RFC3339Nano), Diagnostic: reason}, errors.New(reason)
	}
	validRolePath := rolepolicy.Role(in.AgentPath) == in.Role || (in.Role == "registrar" && in.AgentPath == "/root/registrar")
	if (in.Role != "controller" && in.Role != "worker" && in.Role != "registrar") || !validRolePath {
		return fail("role", "role_path_mismatch: controller requires /root/controller; registrar requires /root/registrar; worker requires a direct Worker child path")
	}
	if in.ParentThreadID == "" || in.ParentThreadID == in.Selector.ThreadID {
		return fail("scope", "parent_identity_required: use exact host parent thread, never native ID or display name")
	}
	if in.CanonicalProject == "" || in.SourceWorktree == "" || in.TaskID == "" || in.AssignmentID == "" || in.Selector.ThreadID == "" || in.Selector.SessionID == "" || in.Selector.RuntimeAgentID != in.Selector.SessionID || in.Selector.ThreadID == in.Selector.SessionID || strings.HasPrefix(in.Selector.SessionID, "thread:") || strings.HasPrefix(in.Selector.ThreadID, "thread:provider:") {
		return fail("identity", "identity_kind_mismatch_or_missing: exact managed T3 and distinct provider-native identities required; provider wrappers are not managed creation proof")
	}
	if err := scope(in); err != nil {
		return fail("scope", err.Error())
	}
	verifiedObserve := func(p string, s T3Selector, n time.Time) (Attempt, T3Receipt, error) {
		if err := scope(in); err != nil {
			return Attempt{}, T3Receipt{}, err
		}
		a, r, err := observe(p, s, n)
		if err == nil {
			err = scope(in) // changed host scope cannot authorize binding
		}
		return a, r, err
	}
	if in.Role == "registrar" {
		return observeRegistrarCollection(legacy, now, verifiedObserve)
	}
	return observeRoleStart(legacy, now, verifiedObserve, in.Role == "controller")
}

// Registration is operational attribution, not Worker implementation consent.
// Require an existing exact task/Registrar assignment, managed parent/project/
// repository scope, and the current structured receipt. Never bind by a title,
// registration timestamp, an old run, or a caller-supplied execution object.
func observeRegistrarCollection(in WorkerStart, now time.Time, observe func(string, T3Selector, time.Time) (Attempt, T3Receipt, error)) (WorkerStartResult, error) {
	out := WorkerStartResult{Status: "collection_gap", Stage: "assignment", Context: in, ReceivedAt: now.UTC().Format(time.RFC3339Nano)}
	fail := func(err error) (WorkerStartResult, error) { out.Diagnostic = err.Error(); return out, err }
	project, err := ResolveProject(in.CanonicalProject)
	if err != nil {
		return fail(err)
	}
	project, err = exactDirectory(project)
	if err != nil {
		return fail(err)
	}
	requested, err := exactDirectory(in.CanonicalProject)
	if err != nil || requested != project {
		return fail(errors.New("--project must name the canonical project root explicitly"))
	}
	a, err := FindAssignment(project, in.AssignmentID)
	if err != nil {
		return fail(err)
	}
	if a.TaskID != in.TaskID || a.AgentPath != "/root/registrar" || in.WorkerPath != a.AgentPath {
		return fail(errors.New("Registrar task/path does not match exact canonical assignment"))
	}
	out.Stage = "observation"
	out.Attempt, out.Receipt, err = observe(project, in.Selector, now)
	if err != nil {
		return fail(err)
	}
	r := out.Receipt
	if r.ThreadID != in.Selector.ThreadID || r.SessionID != in.Selector.SessionID || out.Attempt.RuntimeAgentID != in.Selector.RuntimeAgentID || out.Attempt.NativeReceipt == nil || *out.Attempt.NativeReceipt != r || r.Provider != "codex" || r.RunID == "" || r.NativeAttemptID == "" || (in.Selector.RunID != "" && r.RunID != in.Selector.RunID) || r.State != StateRunning || out.Attempt.Terminal {
		return fail(errors.New("observed native/T3 execution does not match current Registrar START"))
	}
	out.Stage = "binding"
	bound, err := BindRuntimeAgentForAssignment(project, in.AssignmentID, in.Selector.RuntimeAgentID, "", now)
	if err != nil {
		return fail(err)
	}
	if bound.AttemptID != out.Attempt.AttemptID || bound.BindingState != BindingBound || bound.TaskID != in.TaskID || bound.AgentPath != "/root/registrar" || bound.BindingEvidence["assignment_id"] != in.AssignmentID {
		return fail(errors.New("exact observed Registrar attempt was not bound"))
	}
	out.Attempt, out.Status, out.Stage = bound, "bound", "activity_reporting"
	return out, nil
}

// Scope comes from the same schema-bound, ownership-checked read-only adapter
// as execution. No caller JSON, DB path, title or inferred UUID is accepted.
func (s t3Store) verifyRoleScope(in RoleStart) error {
	query := `SELECT t.project_id,json_extract(t.payload_json,'$.lineage.parentThreadId') AS parent_id,
json_extract(t.payload_json,'$.lineage.relationshipToParent') AS relationship,
json_extract(t.payload_json,'$.worktreePath') AS worktree,p.workspace_root,
parent.project_id AS parent_project,parent.deleted_at AS parent_deleted,parent.archived_at AS parent_archived
FROM orchestration_v2_projection_threads t JOIN projection_projects p ON p.project_id=t.project_id
JOIN orchestration_v2_projection_threads parent ON parent.thread_id=json_extract(t.payload_json,'$.lineage.parentThreadId')
WHERE t.deleted_at IS NULL AND t.archived_at IS NULL AND p.deleted_at IS NULL AND p.archived_at IS NULL AND t.thread_id=` + sqlLiteral(in.Selector.ThreadID) + `;`
	raw, err := s.query(query)
	if err != nil {
		return fmt.Errorf("host_scope_unavailable: %w; update the supported host bridge, never inject receipt JSON", err)
	}
	var rows []struct {
		Project       string  `json:"project_id"`
		Parent        string  `json:"parent_id"`
		Relationship  string  `json:"relationship"`
		Worktree      string  `json:"worktree"`
		WorkspaceRoot string  `json:"workspace_root"`
		ParentProject string  `json:"parent_project"`
		ParentDeleted *string `json:"parent_deleted"`
		ParentArchive *string `json:"parent_archived"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return fmt.Errorf("host_scope_invalid: %w", err)
	}
	if len(rows) != 1 || rows[0].Project == "" || rows[0].Parent != in.ParentThreadID || rows[0].Relationship != "subagent" || rows[0].ParentProject != rows[0].Project || rows[0].ParentDeleted != nil || rows[0].ParentArchive != nil {
		return errors.New("host_parent_or_project_mismatch: re-read exact managed child and parent in the intended project")
	}
	r := rows[0]
	canonical, err := exactDirectory(in.CanonicalProject)
	if err != nil {
		return fmt.Errorf("canonical_project_invalid: %w", err)
	}
	hostRoot, err := exactDirectory(r.WorkspaceRoot)
	if err != nil || hostRoot != canonical {
		return errors.New("host_project_mismatch: --project must be the host project's canonical workspace root")
	}
	source, err := exactDirectory(in.SourceWorktree)
	if err != nil {
		return fmt.Errorf("source_worktree_invalid: %w", err)
	}
	worktree := r.Worktree
	if worktree == "" { // supported shared thread uses project workspace
		worktree = r.WorkspaceRoot
	}
	hostSource, err := exactDirectory(worktree)
	if err != nil || hostSource != source {
		return errors.New("host_worktree_mismatch: use actual source cwd, never the canonical ledger as a substitute")
	}
	// A path matching a host payload is insufficient if it points to another
	// repository. Read-only Git identity preserves both separate and shared mode.
	common := func(dir string) (string, error) {
		cmd := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
		b, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return exactDirectory(strings.TrimSpace(string(b)))
	}
	a, err := common(canonical)
	if err != nil {
		return fmt.Errorf("canonical_repository_unverified: %w", err)
	}
	b, err := common(source)
	if err != nil || a != b {
		return errors.New("source_repository_mismatch: source and canonical project must share Git identity")
	}
	return nil
}

func exactDirectory(p string) (string, error) {
	if !filepath.IsAbs(p) {
		return "", errors.New("absolute path required")
	}
	return filepath.EvalSymlinks(filepath.Clean(p))
}
