package webui

import (
	stdcontext "context"
	"sort"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

const currentExecutionBudget = 250 * time.Millisecond
const currentExecutionReadLimit = 16

// One shared policy/assignment loader for workload and notification callers.
// It reads the requested canonical project only, never global recent-use rows.
// Metadata roles are presentation hints from the same typed policy read; they
// cannot authorize work or substitute a reviewer for the implementation owner.
func currentExecutionEvidence(project string, ledger runtimeobs.Ledger, now time.Time, refresh bool) map[string]runtimeobs.CurrentExecutionSnapshot {
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), currentExecutionBudget)
	defer cancel()
	rows, available := operationCanonicalRows(project)
	assignments, err := runtimeobs.ListAssignments(project)
	return loadCurrentExecutionEvidence(ctx, project, ledger, rows, available, assignments, err, now, refresh, rolepolicy.Load, runtimeobs.RefreshCurrentExecutionContext)
}

func loadCurrentExecutionEvidence(ctx stdcontext.Context, project string, ledger runtimeobs.Ledger, rows map[string][]backlog.Record, available bool, assignments []runtimeobs.Assignment, assignmentErr error, now time.Time, refresh bool,
	load func(string, string, string) (*rolepolicy.Policy, bool, error),
	read func(stdcontext.Context, runtimeobs.CurrentExecutionSnapshot, time.Time) runtimeobs.CurrentExecutionSnapshot) map[string]runtimeobs.CurrentExecutionSnapshot {
	out := map[string]runtimeobs.CurrentExecutionSnapshot{}
	tasks := map[string]bool{}
	for task := range rows {
		tasks[strings.ToUpper(task)] = true
	}
	for _, a := range ledger.Attempts {
		if a.TaskID != "" {
			tasks[strings.ToUpper(a.TaskID)] = true
		}
	}
	keys := make([]string, 0, len(tasks))
	for task := range tasks {
		keys = append(keys, task)
	}
	sort.Strings(keys)
	// Spend the bounded official-read budget on explicit human causes first.
	// This ordering never chooses an owner or converts task state to liveness.
	sort.SliceStable(keys, func(i, j int) bool {
		priority := func(task string) int {
			r := rows[task]
			if len(r) != 1 {
				return 3
			}
			if r[0].Fields["대기유형"] == "user" || r[0].Fields["대기유형"] == "external" {
				return 0
			}
			if r[0].State == "doing" {
				return 1
			}
			return 2
		}
		return priority(keys[i]) < priority(keys[j])
	})
	reads := 0
	for _, task := range keys {
		s := runtimeobs.CurrentExecutionSnapshot{Identity: runtimeobs.CurrentExecutionIdentity{Project: project, TaskID: task}, State: runtimeobs.StateRuntimeUnknown}
		fail := func(stage, reason string) {
			s.Fresh = false
			s.OfficialCurrent = false
			s.FailedStage = stage
			s.ActionOwner = "/root/controller"
			s.Limitation = reason
			out[task] = s
		}
		matches := rows[task]
		if !available || len(matches) != 1 || matches[0].Location != "active" {
			fail("canonical_task", "unique active canonical task unavailable")
			continue
		}
		if ctx.Err() != nil {
			fail("official_observation_budget", "current evidence deferred at shared request deadline")
			continue
		}
		if matches[0].State != "doing" && matches[0].State != "hold" {
			fail("canonical_task", "task has no current doing/hold implementation; historical execution remains separate")
			continue
		}
		p, _, e := load(project, task, matches[0].RawMarkdown)
		if e != nil || p == nil || p.Implementation.NativeID == "" || (p.Implementation.Role != "worker" && p.Implementation.Role != "controller") {
			fail("implementation_policy", "current typed implementation owner/native unavailable; legacy Agent and reviewer cannot supply it")
			continue
		}
		s.ImplementationAgentPath = p.Implementation.AgentPath
		s.ReviewAgentPath, s.ReviewNativeID = p.Reviewer.AgentPath, p.Reviewer.NativeID
		s.Identity.AgentPath, s.Identity.NativeID = p.Implementation.AgentPath, p.Implementation.NativeID
		if assignmentErr != nil {
			fail("assignment", "durable assignments unavailable")
			continue
		}
		var latest *runtimeobs.Assignment
		ambiguous := false
		for i := range assignments {
			a := &assignments[i]
			if !strings.EqualFold(a.TaskID, task) || a.AgentPath != p.Implementation.AgentPath {
				continue
			}
			at, e := time.Parse(time.RFC3339Nano, a.AssignedAt)
			if e != nil || at.IsZero() || at.After(now) {
				ambiguous = true
				continue
			}
			if latest == nil {
				latest = a
				continue
			}
			prior, _ := time.Parse(time.RFC3339Nano, latest.AssignedAt)
			if at.Equal(prior) && a.AssignmentID != latest.AssignmentID {
				ambiguous = true
			}
			if at.After(prior) {
				latest = a
			}
		}
		if latest == nil || ambiguous {
			fail("assignment", "latest exact implementation assignment missing or ambiguous")
			continue
		}
		s.Identity.AssignmentID = latest.AssignmentID
		providers := map[string]bool{}
		for _, a := range ledger.Attempts {
			if strings.EqualFold(a.TaskID, task) && a.AgentPath == s.Identity.AgentPath && a.BindingState == runtimeobs.BindingBound && a.BindingEvidence["assignment_id"] == latest.AssignmentID && a.SessionID == s.Identity.NativeID && a.RuntimeAgentID == s.Identity.NativeID && a.Provider != "" {
				providers[a.Provider] = true
			}
		}
		if len(providers) != 1 || operationTaskBlocked(ledger, task) {
			fail("execution_identity", "exact native provider ambiguous/unavailable or runtime findings conflict")
			continue
		}
		for provider := range providers {
			s.Identity.Provider = provider
		}
		selected := runtimeobs.EvaluateCurrentExecution(s.Identity, ledger.Attempts, now)
		selected.ImplementationAgentPath = s.ImplementationAgentPath
		selected.ReviewAgentPath, selected.ReviewNativeID = s.ReviewAgentPath, s.ReviewNativeID
		s = selected
		if refresh && s.Matched {
			if ctx.Err() != nil || reads >= currentExecutionReadLimit {
				fail("official_observation_budget", "official observation deferred: shared request deadline or read cap exhausted")
				continue
			}
			reads++
			s = read(ctx, s, now)
			if ctx.Err() != nil {
				fail("official_observation_budget", "official observation cancelled at shared request deadline")
				continue
			}
		}
		out[task] = s
	}
	return out
}
