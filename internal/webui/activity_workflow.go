package webui

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
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
		return prior.value
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
	return out
}
