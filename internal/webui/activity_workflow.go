package webui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/silverkhan/TaskMecca/goassets"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/contextlife"
	"github.com/silverkhan/TaskMecca/internal/contractscope"
	"github.com/silverkhan/TaskMecca/internal/handoff"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/reviewcontinuity"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// This exact payload constructor is shared by the ordinary product Activity
// endpoint and the source-proven isolated read-only endpoint. Neither route
// invents a fixture workflow or substitutes a custom timeline renderer.
func activityReadPayload(project, task string, events []activity.Event, summary activity.Summary, facts []activity.LifecycleFact, findings int) map[string]any {
	return map[string]any{"task_id": task, "events": events, "summary": summary, "test_report": activity.SummarizeTests(summary.Segments), "review_report": activity.SummarizeReviews(events, summary.Segments), "lifecycle_coverage": activity.SummarizeLifecycle(task, facts, summary.Segments), "lifecycle_findings": findings, "workflow": readActivityWorkflow(project, task, events, summary, facts)}
}

func collectionReadRequest(project, backlogPath string, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	for _, value := range r.URL.Query()["project"] {
		if value != project {
			return false
		}
	}
	for _, value := range r.URL.Query()["backlog"] {
		if value != backlogPath {
			return false
		}
	}
	for _, key := range []string{"root", "path", "source_project", "runtime_diagnostic"} {
		if _, ok := r.URL.Query()[key]; ok {
			return false
		}
	}
	p := r.URL.Path
	if !strings.HasPrefix(p, "/api/") {
		return p != "/sw.js" && p != "/manifest.webmanifest"
	}
	switch p {
	case "/api/activity", "/api/backlog/tasks", "/api/hub", "/api/hub/projects", "/api/health", "/api/version":
		return true
	}
	if strings.HasPrefix(p, "/api/tasks/") {
		projection := r.URL.Query().Get("projection")
		return projection == "content" || projection == "lifecycle"
	}
	return false
}

func collectionReadTree(project string) error {
	count := 0
	return filepath.WalkDir(filepath.Join(project, "_task_mecca"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		count++
		if count > 5000 {
			return fmt.Errorf("isolated collection read tree exceeds limit")
		}
		if entry.Type()&os.ModeSymlink != 0 || (!entry.IsDir() && !entry.Type().IsRegular()) {
			return fmt.Errorf("isolated collection read refuses symlink/special file: %s", path)
		}
		return nil
	})
}

func collectionReadHandler(project, root, version string) (http.Handler, error) {
	source := os.Getenv("TASK_MECCA_COLLECTION_SOURCE_PROJECT")
	if !PreviewMode() || source == "" || root != "" {
		return nil, fmt.Errorf("isolated collection requires explicit source-proven Preview mode and its fixed backlog")
	}
	if _, err := runtimeobs.ReadCollectionProject(source, project, time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := collectionReadTree(project); err != nil {
		return nil, err
	}
	backlogPath := filepath.Join(project, "_task_mecca", "data", "backlog")
	assets, err := fs.Sub(goassets.Template, embeddedRoot+"/web")
	if err != nil {
		return nil, err
	}
	static := http.FileServer(http.FS(assets))
	write := func(w http.ResponseWriter, v any, status int) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !collectionReadRequest(project, backlogPath, r) {
			write(w, map[string]any{"error": "route or scope disabled in isolated collection read-only view"}, http.StatusForbidden)
			return
		}
		if _, err := runtimeobs.ReadCollectionProject(source, project, time.Now().UTC()); err != nil {
			write(w, map[string]any{"error": err.Error()}, http.StatusForbidden)
			return
		}
		if err := collectionReadTree(project); err != nil {
			write(w, map[string]any{"error": err.Error()}, http.StatusForbidden)
			return
		}
		w.Header().Set("X-Task-Mecca-Collection", "isolated-readonly")
		switch r.URL.Path {
		case "/api/activity":
			task := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("task")))
			if task == "" {
				write(w, map[string]any{"error": "task query required"}, http.StatusBadRequest)
				return
			}
			events, err := (activity.Ledger{Project: project}).ListCached(task)
			if err != nil {
				write(w, map[string]any{"error": err.Error()}, 500)
				return
			}
			facts, findings, err := readActivityLifecycle(project, task)
			if err != nil {
				write(w, map[string]any{"error": err.Error()}, 500)
				return
			}
			write(w, activityReadPayload(project, task, events, activity.Summarize(task, events), facts, findings), 200)
		case "/api/backlog/tasks":
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
			payload, err := backlog.BacklogInitialPage(project, backlogPath, page, size, nil, r.URL.Query().Get("q"), r.URL.Query().Get("sort"))
			if err != nil {
				write(w, map[string]any{"error": err.Error()}, 500)
				return
			}
			payload["project_path"] = project
			write(w, payload, 200)
		case "/api/hub", "/api/hub/projects":
			counts, err := backlog.QuickCounts(project, backlogPath)
			if err != nil {
				write(w, map[string]any{"error": err.Error()}, 500)
				return
			}
			// Ephemeral read context, explicitly NOT a registry/monitor entry.
			item := map[string]any{"path": project, "name": filepath.Base(project), "monitoring": false, "status": "isolated", "presence": "present", "isolated_collection": true, "counts": counts}
			write(w, map[string]any{"projects": []any{item}, "archives": []any{}, "current_project": project, "cli": map[string]any{"current": version}}, 200)
		case "/api/health":
			write(w, map[string]any{"ok": true, "version": version, "preview_mode": true, "collection_readonly": true, "telegram_transport_disabled": true, "pid": os.Getpid()}, 200)
		case "/api/version":
			write(w, map[string]any{"cli": map[string]any{"current": version, "error": "Release lookup disabled in isolated collection view"}, "project": map[string]any{"path": project, "migration_available": false}}, 200)
		default:
			if strings.HasPrefix(r.URL.Path, "/api/tasks/") {
				id := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
				if r.URL.Query().Get("projection") == "lifecycle" {
					value, available := backlog.ReadLifecycleProjection(project, backlogPath, id)
					if !available {
						write(w, map[string]any{"snapshot": map[string]any{"trust": "pending"}, "isolated_collection": true}, http.StatusAccepted)
					} else {
						write(w, value, 200)
					}
					return
				}
				value, err := backlog.TaskContent(project, backlogPath, id)
				if err != nil {
					write(w, map[string]any{"error": err.Error()}, 404)
					return
				}
				write(w, value, 200)
				return
			}
			if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/tasks/") {
				copy := r.Clone(r.Context())
				u := *r.URL
				u.Path = "/"
				copy.URL = &u
				static.ServeHTTP(w, copy)
				return
			}
			static.ServeHTTP(w, r)
		}
	}), nil
}

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
	records   []handoffjournal.Record
	err       error
}

func readWorkflowClaimRecords(project string) []handoffjournal.Record {
	path := handoffjournal.JournalPath(project)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 32<<20 {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil
	}
	scanner := bufio.NewScanner(io.LimitReader(f, (32<<20)+1))
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	var records []handoffjournal.Record
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var row handoffjournal.Record
		if json.Unmarshal(scanner.Bytes(), &row) != nil || row.HandoffID == "" {
			return nil
		}
		records = append(records, row)
	}
	if scanner.Err() != nil {
		return nil
	}
	return records
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
	next := workflowHandoffCacheEntry{at: now, checkedAt: now, ledger: ledger, records: readWorkflowClaimRecords(project), err: err}
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
// Bounded exact local host reads supply missing historical project provenance.
// No observation event, provider scan, role repair, or authorization is performed.
func readWorkflowNativeProjectProof(project, task, operationalNative string, observations []runtimeobs.Attempt, history workflowHandoffCacheEntry, now time.Time) map[string]runtimeobs.T3Receipt {
	out := map[string]runtimeobs.T3Receipt{}
	if operationalNative == "" {
		return out
	}
	ids := map[string]bool{}
	for _, r := range history.records {
		h, ok := history.ledger.Handoffs[r.HandoffID]
		if !ok || h.TaskID != task {
			continue
		}
		if r.RecordKind == "claimed" || r.RecordKind == "claim_transferred" {
			ids[r.ClaimedAttemptID] = true
			if r.PreviousClaimedAttemptID != "" {
				ids[r.PreviousClaimedAttemptID] = true
			}
		}
	}
	if len(ids) > 64 {
		return out
	}
	for _, a := range observations {
		r := a.NativeReceipt
		if !ids[a.AttemptID] || a.TaskID != "" || a.AgentPath != "/root/controller" || a.BindingState != runtimeobs.BindingBound || a.RuntimeAgentID != operationalNative || a.SessionID != operationalNative || r == nil || r.CanonicalProject != "" || r.ThreadID == "" || r.RunID == "" || r.NativeAttemptID == "" {
			continue
		}
		sel := runtimeobs.T3Selector{CanonicalProject: project, ThreadID: r.ThreadID, SessionID: a.SessionID, RuntimeAgentID: a.RuntimeAgentID, RunID: r.RunID, NativeAttemptID: r.NativeAttemptID}
		var proof runtimeobs.T3Receipt
		var err error
		if a.Terminal {
			proof, err = runtimeobs.ReadT3HistoricalExecution(sel, now)
		} else {
			proof, err = runtimeobs.ReadT3Execution(sel, now)
		}
		if err == nil {
			out[a.AttemptID] = proof
		}
	}
	return out
}

func readActivityWorkflow(project, task string, events []activity.Event, summary activity.Summary, facts []activity.LifecycleFact) activity.Workflow {
	now := time.Now().UTC()
	cached := loadWorkflowHandoffs(project, now)
	control, _ := backlog.ReadLifecycleProjection(project, "", task)
	// Read the exact task's persisted policy. No role/name inference and no
	// permission or current Activity association is changed by this display input.
	operationalNative := ""
	if rows, valid := operationCanonicalRows(project); valid && len(rows[task]) == 1 {
		if p, _, e := rolepolicy.Load(project, task, rows[task][0].RawMarkdown); e == nil && p != nil && p.Operational.Role == "controller" && p.Operational.AgentPath == "/root/controller" {
			operationalNative = p.Operational.NativeID
		}
	}
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
	key := project + "\x00" + task + "\x00" + operationalNative
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
	hostProject := project
	if canonical := os.Getenv("TASK_MECCA_COLLECTION_SOURCE_PROJECT"); canonical != "" {
		if scope, e := runtimeobs.ReadCollectionProject(canonical, project, now); e == nil {
			hostProject = scope.Canonical
		}
	}
	nativeProjects := readWorkflowNativeProjectProof(hostProject, task, operationalNative, observations, cached, now)
	out := activity.SummarizeWorkflow(activity.WorkflowInput{ExecutionObservations: observations, TaskID: task, Events: events, Summary: summary, Facts: facts, Handoffs: cached.ledger.Handoffs, HandoffRecords: cached.records, OperationalControllerNative: operationalNative, SourceProject: hostProject, NativeProjectReceipts: nativeProjects, Control: control, Now: now})
	out.Snapshot["execution_evidence_source"] = "runtime_ledger_read_only"
	if observationErr != nil {
		out.Snapshot["execution_evidence_error"] = observationErr.Error()
		out.Coverage.Findings = append(out.Coverage.Findings, activity.WorkflowRecoveryFinding{TaskID: task, Start: now, End: now, Status: "collection_failed", Cause: observationErr.Error(), Owner: "controller", ResumeCondition: "Restore persisted execution evidence read; do not fabricate historical Activity", Evidence: []activity.WorkflowGapEvidence{}})
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
	presentation := readReviewerPresentation(project, task, nil)
	cloned["reviewer_presentation"] = presentation
	w.Snapshot = cloned
	return activity.ApplyReviewPurpose(w, presentation)
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
	assignments, assignmentErr := runtimeobs.ListAssignments(project)
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
			workerPurpose := p.Reviewer.Role == "worker" && rolepolicy.WorkerPath(a.AgentPath)
			if !formal && !legacy && !workerPurpose {
				continue
			}
			v := activity.ReviewerPresentationEntry{AttemptID: a.AttemptID, NativeID: a.SessionID, TaskID: id, Purpose: "review", PurposeVerified: true, Necessity: "unknown", ActualState: "unknown", ContextStatus: "unknown", SourceDelta: []json.RawMessage{}, Errors: []string{}}
			// Current exact assignment and source linkage are independent from the
			// legacy policy-purpose display. No old Activity is upgraded by keywords.
			assignmentID := a.BindingEvidence["assignment_id"]
			matches := 0
			for _, assigned := range assignments {
				if assigned.AssignmentID == assignmentID && assigned.TaskID == id && assigned.AgentPath == a.AgentPath {
					matches++
				}
			}
			if assignmentErr == nil && assignmentID != "" && matches == 1 && rolepolicy.ValidateRequirementScope(p) == nil {
				contract := contractscope.Fingerprint(records[0].RawMarkdown)
				ac, acErr := handoff.AcceptanceFingerprint(records[0].RawMarkdown)
				if contract != "" && acErr == nil {
					v.AssignmentID = assignmentID
					v.ContractSHA256 = contract
					v.AcceptanceSHA256 = ac
					copy := source
					v.Source = &copy
					v.PolicySHA256 = rolepolicy.PolicyDigest(p)
					v.LinkageVerified = true
				}
			}
			if workerPurpose && !legacy && !v.LinkageVerified {
				continue
			}
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
				if s, e := contextlife.Read(project, role); e == nil && s.OwnerAttempt == a.AttemptID && s.Observation != nil && s.Observation.AttemptID == a.AttemptID && a.Provider != "" && s.Observation.Provider == a.Provider {
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
