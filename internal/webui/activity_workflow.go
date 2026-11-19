package webui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/contextlife"
	"github.com/silverkhan/TaskMecca/internal/contractscope"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/reviewcontinuity"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

type workflowResponseEntry struct {
	at       time.Time
	revision string
	value    activity.Workflow
}

var workflowResponseCache = struct {
	sync.Mutex
	items map[string]workflowResponseEntry
}{items: map[string]workflowResponseEntry{}}

type workflowHandoffCacheEntry struct {
	at        time.Time
	checkedAt time.Time
	ledger    handoffjournal.Ledger
	err       error
}

var workflowHandoffCache = struct {
	sync.Mutex
	projects map[string]workflowHandoffCacheEntry
	flights  map[string]chan struct{}
}{projects: map[string]workflowHandoffCacheEntry{}, flights: map[string]chan struct{}{}}

func loadWorkflowHandoffs(project string, now time.Time) workflowHandoffCacheEntry {
	workflowHandoffCache.Lock()
	cached, ok := workflowHandoffCache.projects[project]
	if ok && now.Sub(cached.checkedAt) < activityLifecycleTTL {
		workflowHandoffCache.Unlock()
		return cached
	}
	if done := workflowHandoffCache.flights[project]; done != nil {
		workflowHandoffCache.Unlock()
		<-done
		workflowHandoffCache.Lock()
		out := workflowHandoffCache.projects[project]
		workflowHandoffCache.Unlock()
		return out
	}
	done := make(chan struct{})
	workflowHandoffCache.flights[project] = done
	workflowHandoffCache.Unlock()
	ledger, err := handoffjournal.BuildLedger(project, now)
	next := workflowHandoffCacheEntry{at: now, checkedAt: now, ledger: ledger, err: err}
	if err != nil && ok {
		next.ledger = cached.ledger
		next.at = cached.at
	}
	workflowHandoffCache.Lock()
	if len(workflowHandoffCache.projects) >= activityLifecycleMaxProjects {
		for id := range workflowHandoffCache.projects {
			if id != project {
				delete(workflowHandoffCache.projects, id)
				break
			}
		}
	}
	workflowHandoffCache.projects[project] = next
	delete(workflowHandoffCache.flights, project)
	close(done)
	workflowHandoffCache.Unlock()
	return next
}

// This is an on-demand, bounded read cache, not a Control Tower reconciliation.
// Only persisted Runtime evidence is read; no provider scan or monitor lock runs.
func readActivityWorkflow(project, task string, events []activity.Event, summary activity.Summary, facts []activity.LifecycleFact) activity.Workflow {
	now := time.Now().UTC()
	cached := loadWorkflowHandoffs(project, now)
	control, _ := backlog.ReadLifecycleProjection(project, "", task)
	observations, observationErr := activity.ReadWorkflowExecutions(project, task, now)
	// Compare persisted evidence, not elapsed/waiting durations recomputed at
	// read time. Terminal updates/resumes must still invalidate the cache even
	// while Control Tower is pending. Keep the actual response input intact.
	revisionEvidence := make([]runtimeobs.Attempt, len(observations))
	copy(revisionEvidence, observations)
	for i := range revisionEvidence {
		revisionEvidence[i].ElapsedMillis = 0
		revisionEvidence[i].WaitingMillis = 0
	}
	observationRevision, _ := json.Marshal(revisionEvidence)
	key := project + "\x00" + task
	lastEvent, lastFact := "", ""
	if len(events) > 0 {
		lastEvent = events[len(events)-1].EventID
	}
	if len(facts) > 0 {
		lastFact = facts[len(facts)-1].EventID
	}
	snapshot, _ := control["snapshot"].(map[string]any)
	revision := fmt.Sprintf("%d:%s|%d:%s|%s|%v|%v|%v", len(events), lastEvent, len(facts), lastFact, cached.checkedAt.Format(time.RFC3339Nano), snapshot["version"], snapshot["trust"], snapshot["source_revision"]) + "|" + string(observationRevision) + fmt.Sprint(observationErr)
	workflowResponseCache.Lock()
	prior, found := workflowResponseCache.items[key]
	workflowResponseCache.Unlock()
	if found && prior.revision == revision && now.Sub(prior.at) < 2*time.Second {
		return attachReviewerPresentation(project, task, activity.AttachExecutionCoverage(project, task, prior.value))
	}
	out := activity.SummarizeWorkflow(activity.WorkflowInput{ExecutionObservations: observations, TaskID: task, Events: events, Summary: summary, Facts: facts, Handoffs: cached.ledger.Handoffs, Control: control, Now: now})
	out.Snapshot["execution_evidence_source"] = "runtime_ledger_read_only"
	if observationErr != nil {
		out.Snapshot["execution_evidence_error"] = observationErr.Error()
		out.Measurement.Status = "conflict"
	}
	out.Snapshot["handoff_checked_at"] = cached.checkedAt
	out.Snapshot["handoff_computed_at"] = cached.at
	if cached.err != nil {
		out.Snapshot["handoff_error"] = cached.err.Error()
		out.Measurement.Status = "conflict"
	}
	if len(cached.ledger.Findings) > 0 {
		out.Snapshot["handoff_findings"] = len(cached.ledger.Findings)
		out.Measurement.Status = "conflict"
	}
	workflowResponseCache.Lock()
	if len(workflowResponseCache.items) >= 64 {
		oldestKey := ""
		var oldest time.Time
		for id, v := range workflowResponseCache.items {
			if oldestKey == "" || v.at.Before(oldest) {
				oldestKey = id
				oldest = v.at
			}
		}
		delete(workflowResponseCache.items, oldestKey)
	}
	workflowResponseCache.items[key] = workflowResponseEntry{at: now, revision: revision, value: out}
	workflowResponseCache.Unlock()
	return attachReviewerPresentation(project, task, activity.AttachExecutionCoverage(project, task, out))
}

func attachReviewerPresentation(project, task string, w activity.Workflow) activity.Workflow {
	cloned := map[string]any{}
	for k, v := range w.Snapshot {
		cloned[k] = v
	}
	cloned["reviewer_presentation"] = readReviewerPresentation(project, task, nil)
	w.Snapshot = cloned
	return w
}

// Persisted reads only: no provider refresh, monitor, mutation or timing edits.
func readReviewerPresentation(project, task string, existing *runtimeobs.Ledger) activity.ReviewerPresentation {
	var ledger runtimeobs.Ledger
	var err error
	if existing != nil {
		ledger = *existing
	} else {
		ledger, err = runtimeobs.BuildLedger(project, 0, time.Now().UTC())
	}
	policies := map[string]*rolepolicy.Policy{}
	verified := map[string]activity.ReviewerPresentationEntry{}
	if err != nil {
		return activity.NewReviewerPresentation(project, task, nil, nil, policies, verified)
	}
	for _, f := range ledger.Findings {
		if f.Code == "invalid_event_line" {
			return activity.NewReviewerPresentation(project, task, nil, nil, policies, verified)
		}
	}
	rows, _ := operationCanonicalRows(project)
	identities := []rolepolicy.Identity{}
	for _, a := range ledger.Attempts {
		identities = append(identities, rolepolicy.Identity{AttemptID: a.AttemptID, NativeID: a.SessionID, RuntimeAgentID: a.RuntimeAgentID, Provider: a.Provider, AgentPath: a.AgentPath, TaskID: a.TaskID, Bound: a.BindingState == runtimeobs.BindingBound, Observed: runtimeobs.HasObservedExecution(a)})
	}
	for id, records := range rows {
		if task != "" && id != task || len(records) != 1 {
			continue
		}
		p, _, e := rolepolicy.Load(project, id, records[0].RawMarkdown)
		if e != nil || p == nil {
			continue
		}
		policies[id] = p
		history, e := runtimeobs.ProductImplementationHistory(project, id, p, time.Now().UTC())
		if e != nil {
			continue
		}
		if p.Reviewer.NativeID == "" || p.Implementation.NativeID == "" {
			continue
		}
		for _, a := range ledger.Attempts {
			if a.TaskID != id || (p.Review != nil && p.Review.AttemptID != "" && p.Review.AttemptID != a.AttemptID) || a.SessionID != p.Reviewer.NativeID || a.AgentPath != p.Reviewer.AgentPath || a.BindingState != runtimeobs.BindingBound || !runtimeobs.HasObservedExecution(a) {
				continue
			}
			var reviewer, implementation rolepolicy.Identity
			for _, h := range identities {
				if h.AttemptID == a.AttemptID {
					reviewer = h
				}
				if h.TaskID == id && h.NativeID == p.Implementation.NativeID && h.AgentPath == p.Implementation.AgentPath && h.Bound && h.Observed {
					implementation = h
				}
			}
			if implementation.NativeID == "" || rolepolicy.ValidateImplementationHistory(id, reviewer, history) != nil {
				continue
			}
			source, e := rolepolicy.Snapshot(p.Workspace, p.Scope)
			if e != nil || len(source.Files) == 0 {
				continue
			}
			formal := rolepolicy.ReviewerPath(a.AgentPath) && p.Reviewer.Role == "reviewer"
			// Historical Worker purposes require an explicit actual review record;
			// their stored role/name remains unchanged.
			legacy := p.Reviewer.Role == "worker" && p.Review != nil && p.Review.AttemptID == a.AttemptID
			if !formal && !legacy {
				continue
			}
			v := activity.ReviewerPresentationEntry{AttemptID: a.AttemptID, NativeID: a.SessionID, TaskID: id, Purpose: "review", PurposeVerified: true, Necessity: "unknown", ActualState: "unknown", ContextStatus: "unknown", SourceDelta: []json.RawMessage{}, Errors: []string{}}
			if p.ReviewRequirement != nil {
				if d, e := p.ReviewRequirement.Decide(); e == nil {
					v.Necessity = string(d.Level)
					if d.Status == "omitted" {
						v.Purpose = "unknown"
						v.PurposeVerified = false
						v.ActualState = "omitted"
					}
				}
			}
			if formal {
				role := "reviewer:" + strings.TrimPrefix(a.AgentPath, "/root/controller/reviewer/")
				if s, e := contextlife.Read(project, role); e == nil && s.OwnerAttempt == a.AttemptID && s.Observation != nil && s.Observation.AttemptID == a.AttemptID {
					v.ContextStatus = contextlife.Decide(role, s.Observation, 0, "").ContextStatus
				}
			}
			// An optional persisted input is useful only when bound to the current
			// policy, original contract, exact runtime identity and complete source.
			if strings.HasPrefix(a.AttemptID, "run-") && filepath.Base(a.AttemptID) == a.AttemptID {
				path := filepath.Join(project, "_task_mecca", ".runtime", "reviewer_continuity", a.AttemptID+".json")
				var saved struct {
					PolicySHA256 string                 `json:"policy_sha256"`
					Input        reviewcontinuity.Input `json:"input"`
				}
				if b, e := os.ReadFile(path); e == nil && len(b) <= 1<<20 && json.Unmarshal(b, &saved) == nil && saved.PolicySHA256 == rolepolicy.PolicyDigest(p) {
					r := saved.Input.Current.Request
					contractOK, _ := contractscope.MatchesFingerprint(records[0].RawMarkdown, r.ContractSHA256)
					if contractOK && r.TaskID == id && a.BindingEvidence["assignment_id"] != "" && r.AssignmentID == a.BindingEvidence["assignment_id"] && reflect.DeepEqual(r.Reviewer, reviewer) && reflect.DeepEqual(r.Implementation, implementation) && reflect.DeepEqual(r.Source, source) && p.ReviewRequirement != nil && reflect.DeepEqual(r.Requirement, *p.ReviewRequirement) {
						saved.Input.ImplementationHistory = history
						if result, e := reviewcontinuity.Assess(p.Workspace, saved.Input); e == nil {
							b, _ := json.Marshal(result)
							_ = json.Unmarshal(b, &v)
							v.TaskID = id
						}
					}
				}
			}
			verified[a.AttemptID] = v
		}
	}
	events, _ := (activity.Ledger{Project: project}).ListCached(task)
	return activity.NewReviewerPresentation(project, task, ledger.Attempts, events, policies, verified)
}

// Keep the original response object's keys and JSON values unchanged.
func reviewerPresentationEnvelope(value any, p activity.ReviewerPresentation) map[string]any {
	b, e := json.Marshal(value)
	if e != nil {
		return map[string]any{"reviewer_presentation": p}
	}
	var original map[string]json.RawMessage
	if json.Unmarshal(b, &original) != nil {
		return map[string]any{"reviewer_presentation": p}
	}
	out := map[string]any{}
	for k, v := range original {
		out[k] = v
	}
	out["reviewer_presentation"] = p
	return out
}
