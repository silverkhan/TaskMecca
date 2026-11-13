package runtimeobs

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// WorkerStart connects a real START identity envelope to the existing observer
// and assignment binder. It is a Controller operation, never a Worker write.
// SourceWorktree is context only; all reads and writes use CanonicalProject.
type WorkerStart struct {
	CanonicalProject string     `json:"canonical_project"`
	SourceWorktree   string     `json:"source_worktree"`
	TaskID           string     `json:"task_id"`
	AssignmentID     string     `json:"assignment_id"`
	WorkerPath       string     `json:"worker_path"`
	Selector         T3Selector `json:"selector"`
}

type WorkerStartResult struct {
	Status     string      `json:"status"`
	Stage      string      `json:"stage"`
	Context    WorkerStart `json:"context"`
	ReceivedAt string      `json:"collection_received_at"`
	Attempt    Attempt     `json:"attempt"`
	Receipt    T3Receipt   `json:"receipt"`
	Diagnostic string      `json:"diagnostic,omitempty"`
}

func ObserveT3WorkerStart(in WorkerStart, now time.Time) (WorkerStartResult, error) {
	return observeWorkerStart(in, now, ObserveT3Execution)
}

func observeWorkerStart(in WorkerStart, now time.Time, observe func(string, T3Selector, time.Time) (Attempt, T3Receipt, error)) (WorkerStartResult, error) {
	out := WorkerStartResult{Status: "collection_gap", Stage: "identity", Context: in, ReceivedAt: now.UTC().Format(time.RFC3339Nano)}
	fail := func(err error) (WorkerStartResult, error) { out.Diagnostic = err.Error(); return out, err }
	if in.CanonicalProject == "" || in.SourceWorktree == "" || in.TaskID == "" || in.AssignmentID == "" || !strings.HasPrefix(in.WorkerPath, "/root/controller/") || strings.HasPrefix(in.WorkerPath, "/root/controller/sub-") || in.Selector.ThreadID == "" || in.Selector.SessionID == "" || in.Selector.RuntimeAgentID != in.Selector.SessionID || in.Selector.ThreadID == in.Selector.SessionID {
		return fail(fmt.Errorf("exact canonical project, source worktree, task, assignment, worker path and distinct native/T3 identities required"))
	}
	project, err := ResolveProject(in.CanonicalProject)
	if err != nil {
		return fail(err)
	}
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		return fail(err)
	}
	requested, err := filepath.Abs(in.CanonicalProject)
	if err != nil {
		return fail(err)
	}
	requested, err = filepath.EvalSymlinks(requested)
	if err != nil || requested != project {
		return fail(fmt.Errorf("--project must name the canonical project root explicitly"))
	}
	source, err := filepath.Abs(in.SourceWorktree)
	if err != nil {
		return fail(err)
	}
	out.Context.CanonicalProject = project
	out.Context.SourceWorktree = source
	out.Stage = "assignment"
	assignment, err := FindAssignment(project, in.AssignmentID)
	if err != nil {
		return fail(err)
	}
	if assignment.TaskID != in.TaskID || assignment.AgentPath != in.WorkerPath {
		return fail(fmt.Errorf("START task/worker does not match exact canonical assignment"))
	}
	out.Stage = "observation"
	out.Attempt, out.Receipt, err = observe(project, in.Selector, now)
	if err != nil {
		return fail(err)
	}
	// Verify observer output before binding. No titles, caller-supplied state or
	// dispatch timestamps can substitute for the official structured receipt.
	if out.Receipt.ThreadID != in.Selector.ThreadID || out.Receipt.SessionID != in.Selector.SessionID || out.Attempt.RuntimeAgentID != in.Selector.RuntimeAgentID || out.Attempt.NativeReceipt == nil || *out.Attempt.NativeReceipt != out.Receipt || out.Receipt.Provider != "codex" || out.Receipt.RunID == "" || out.Receipt.NativeAttemptID == "" || (in.Selector.RunID != "" && in.Selector.RunID != out.Receipt.RunID) || out.Receipt.State != StateRunning || out.Attempt.Terminal {
		return fail(fmt.Errorf("observed native/T3 execution does not match current START"))
	}
	out.Stage = "binding"
	bound, err := BindRuntimeAgentForAssignment(project, in.AssignmentID, in.Selector.RuntimeAgentID, "", now)
	if err != nil {
		return fail(err)
	}
	if bound.AttemptID != out.Attempt.AttemptID || bound.BindingState != BindingBound || bound.TaskID != in.TaskID || bound.AgentPath != in.WorkerPath || bound.BindingEvidence["assignment_id"] != in.AssignmentID {
		return fail(fmt.Errorf("exact observed attempt was not bound; retry current START after diagnosis"))
	}
	out.Attempt = bound
	out.Status = "bound"
	out.Stage = "activity_reporting"
	return out, nil
}
