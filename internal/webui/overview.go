package webui

import (
	stdcontext "context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/maintenance"
)

const overviewTTL = 5 * time.Second
const overviewConcurrency = 4
const overviewProjectBatch = 8
const overviewCacheBatches = 8
const overviewResponseMaxBytes = 2 << 20

type overviewCoverage struct {
	Complete        bool `json:"complete"`
	ScannedProjects int  `json:"scanned_projects"`
	TotalProjects   int  `json:"total_projects"`
}

type overviewSection struct {
	Items      []map[string]any `json:"items"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	Total      *int             `json:"total"`
	Pages      *int             `json:"pages"`
	HasMore    bool             `json:"has_more"`
	Status     string           `json:"status"`
	Coverage   overviewCoverage `json:"coverage"`
	ObservedAt *string          `json:"observed_at"`
	NextCursor string           `json:"next_cursor,omitempty"`
	Pending    bool             `json:"pending"`
	Loading    bool             `json:"loading"`
	Source     string           `json:"source,omitempty"`
}

type overviewError struct {
	Project string `json:"project"`
	Message string `json:"message"`
}

type overviewProject struct {
	row      map[string]any
	items    map[string][]map[string]any
	complete map[string]bool
	observed string
	stale    bool
	pending  bool
	errors   []overviewError
}

type overviewSnapshot struct {
	items    map[string][]map[string]any
	coverage map[string]overviewCoverage
	observed map[string]*string
	stale    bool
	pending  bool
	errors   []overviewError
	at       time.Time
	batch    int
	registry string
	loading  bool
}

// A service belongs to its HTTP handler, not a process-global registry. One
// bounded cold aggregation is shared by all section/page requests. It reads
// existing catalog/event caches and canonical projections, never reconciles.
type overviewService struct {
	mu       sync.Mutex
	cached   map[int]*overviewSnapshot
	cacheKey string
	pending  bool
	projects func() ([]maintenance.ProjectState, error)
	load     func(stdcontext.Context, maintenance.ProjectState) overviewProject
	now      func() time.Time
	revision func() string
}

var overviewSections = []string{"attention", "working", "projects", "activity"}

func newOverviewService() *overviewService {
	return &overviewService{cached: map[int]*overviewSnapshot{}, projects: maintenance.ManagedProjects, load: loadOverviewProject, now: time.Now, revision: func() string { return overviewEvents.current().Revision }}
}

func registerOverviewRoute(mux *http.ServeMux, writeJSON func(http.ResponseWriter, any, int)) {
	service := newOverviewService()
	mux.HandleFunc("/api/overview", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, map[string]any{"error": "GET required"}, http.StatusMethodNotAllowed)
			return
		}
		responseRevision := overviewEvents.current().Revision
		pages, size, err := overviewPagination(r.URL.Query())
		if err != nil {
			writeJSON(w, map[string]any{"error": err.Error()}, http.StatusBadRequest)
			return
		}
		projects, err := service.projects()
		if err != nil {
			writeJSON(w, map[string]any{"error": "managed project registry: " + err.Error()}, http.StatusServiceUnavailable)
			return
		}
		key := overviewRegistryKey(projects)
		requests := map[string]overviewCursor{}
		for _, name := range overviewSections {
			cursor := overviewCursor{Page: pages[name], Registry: key}
			if encoded := r.URL.Query().Get(name + "_cursor"); encoded != "" {
				cursor, err = decodeOverviewCursor(encoded)
				if err != nil || cursor.Batch < 0 || cursor.Batch%overviewProjectBatch != 0 || cursor.Page < 1 || (cursor.Batch >= len(projects) && cursor.Batch != 0) {
					writeJSON(w, map[string]any{"error": "invalid overview cursor"}, http.StatusBadRequest)
					return
				}
				if cursor.Registry != key {
					writeJSON(w, map[string]any{"error": "overview registry changed; restart pagination"}, http.StatusConflict)
					return
				}
			}
			requests[name] = cursor
		}
		snapshots := map[string]*overviewSnapshot{}
		for _, name := range overviewSections {
			snapshots[name] = service.read(projects, key, requests[name].Batch)
		}
		payload := overviewResponse(service.now(), requests, size, snapshots, len(projects))
		payload["revision"] = responseRevision
		writeOverviewResponse(w, payload, writeJSON)
	})
}

func writeOverviewResponse(w http.ResponseWriter, payload map[string]any, writeJSON func(http.ResponseWriter, any, int)) {
	encoded, err := json.Marshal(payload)
	if err != nil || len(encoded) > overviewResponseMaxBytes {
		writeJSON(w, map[string]any{"error": "overview response budget exceeded; retry with a smaller page_size"}, http.StatusRequestEntityTooLarge)
		return
	}
	writeJSON(w, payload, http.StatusOK)
}

func overviewPagination(query url.Values) (map[string]int, int, error) {
	parse := func(key string, fallback int) (int, error) {
		if query.Get(key) == "" {
			return fallback, nil
		}
		n, err := strconv.Atoi(query.Get(key))
		if err != nil || n < 1 {
			return 0, fmt.Errorf("%s must be a positive integer", key)
		}
		return n, nil
	}
	size, err := parse("page_size", 8)
	if err != nil {
		return nil, 0, err
	}
	if size > 50 {
		size = 50
	}
	pages := make(map[string]int, len(overviewSections))
	for _, section := range overviewSections {
		page, err := parse(section+"_page", 1)
		if err != nil {
			return nil, 0, err
		}
		pages[section] = page
	}
	return pages, size, nil
}

func overviewRegistryKey(projects []maintenance.ProjectState) string {
	parts := make([]string, 0, len(projects))
	for _, p := range projects {
		parts = append(parts, fmt.Sprintf("%s\x00%s\x00%t\x00%s", p.Path, p.Presence, p.Monitoring, p.Name))
	}
	sort.Strings(parts)
	data, _ := json.Marshal(parts)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

type overviewCursor struct {
	Batch    int    `json:"b"`
	Page     int    `json:"p"`
	Registry string `json:"r"`
}

func (c overviewCursor) encode() string {
	data, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeOverviewCursor(encoded string) (overviewCursor, error) {
	var cursor overviewCursor
	if len(encoded) > 256 {
		return cursor, fmt.Errorf("cursor too long")
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return cursor, err
	}
	err = json.Unmarshal(data, &cursor)
	return cursor, err
}

// A cold request starts at most one eight-project read job, then returns
// immediately. All in-flight batches share the same four-worker ceiling.
// No catalog walk, ledger scan or canonical reconciliation runs on the request.
func (s *overviewService) read(projects []maintenance.ProjectState, key string, batch int) *overviewSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	generation := s.revision()
	dataKey := key + "\x00" + generation
	if s.cacheKey != dataKey {
		s.cached = map[int]*overviewSnapshot{}
		s.cacheKey = dataKey
	}
	cached := s.cached[batch]
	if cached != nil && s.now().Sub(cached.at) < overviewTTL {
		return cached
	}
	if !s.pending {
		s.pending = true
		end := batch + overviewProjectBatch
		if end > len(projects) {
			end = len(projects)
		}
		targets := append([]maintenance.ProjectState{}, projects[batch:end]...)
		go func() {
			result := s.collect(stdcontext.Background(), targets)
			result.batch, result.registry = batch, key
			for name, coverage := range result.coverage {
				coverage.TotalProjects = len(projects)
				coverage.Complete = coverage.Complete && len(targets) == len(projects)
				result.coverage[name] = coverage
			}
			s.mu.Lock()
			if s.cacheKey == dataKey && s.revision() == generation {
				if len(s.cached) >= overviewCacheBatches && s.cached[batch] == nil {
					oldest := -1
					for offset, item := range s.cached {
						if oldest < 0 || item.at.Before(s.cached[oldest].at) {
							oldest = offset
						}
					}
					delete(s.cached, oldest)
				}
				s.cached[batch] = result
			}
			s.pending = false
			s.mu.Unlock()
		}()
	}
	if cached != nil {
		refreshing := *cached
		refreshing.loading = true
		return &refreshing
	} // stale-while-revalidate, with original observation time
	pending := &overviewSnapshot{items: map[string][]map[string]any{}, coverage: map[string]overviewCoverage{}, observed: map[string]*string{}, errors: []overviewError{}, at: s.now(), batch: batch, registry: key, loading: true}
	for _, name := range overviewSections {
		pending.items[name] = []map[string]any{}
		pending.coverage[name] = overviewCoverage{TotalProjects: len(projects)}
	}
	return pending
}

func (s *overviewService) collect(ctx stdcontext.Context, projects []maintenance.ProjectState) *overviewSnapshot {
	results := make([]overviewProject, len(projects))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for i := 0; i < overviewConcurrency && i < len(projects); i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				if ctx.Err() == nil {
					results[index] = s.load(ctx, projects[index])
				}
			}
		}()
	}
	for i := range projects {
		if ctx.Err() != nil {
			break
		}
		jobs <- i
	}
	close(jobs)
	workers.Wait()
	result := &overviewSnapshot{items: map[string][]map[string]any{}, coverage: map[string]overviewCoverage{}, observed: map[string]*string{}, errors: []overviewError{}, at: s.now()}
	for _, section := range overviewSections {
		result.items[section] = []map[string]any{}
		result.coverage[section] = overviewCoverage{Complete: true, TotalProjects: len(projects)}
	}
	for _, p := range results {
		if p.row != nil {
			result.items["projects"] = append(result.items["projects"], p.row)
		}
		result.errors = append(result.errors, p.errors...)
		result.stale = result.stale || p.stale
		result.pending = result.pending || p.pending
		for _, section := range overviewSections {
			if section != "projects" {
				result.items[section] = append(result.items[section], p.items[section]...)
			}
			coverage := result.coverage[section]
			if p.complete[section] {
				coverage.ScannedProjects++
			} else {
				coverage.Complete = false
			}
			result.coverage[section] = coverage
			if p.observed != "" && (result.observed[section] == nil || p.observed < *result.observed[section]) {
				observed := p.observed
				result.observed[section] = &observed
			}
		}
	}
	for _, section := range overviewSections {
		items := result.items[section]
		sort.SliceStable(items, func(i, j int) bool {
			if section == "projects" {
				return overviewString(items[i]["path"]) < overviewString(items[j]["path"])
			}
			key := "updated_at"
			if section == "activity" {
				key = "at"
			}
			if left, right := overviewString(items[i][key]), overviewString(items[j][key]); left != right {
				return left > right
			}
			return overviewString(items[i]["href"]) < overviewString(items[j]["href"])
		})
	}
	return result
}

func overviewResponse(now time.Time, requests map[string]overviewCursor, size int, snapshots map[string]*overviewSnapshot, totalProjects int) map[string]any {
	sections := map[string]overviewSection{}
	status, stale, loading, pending := "ok", false, false, false
	errors := []overviewError{}
	seenErrors := map[overviewError]bool{}
	var observed *string
	for _, name := range overviewSections {
		s, cursor := snapshots[name], requests[name]
		items, coverage := s.items[name], s.coverage[name]
		old := s.stale || now.Sub(s.at) >= overviewTTL
		stale, loading, pending = stale || old, loading || s.loading, pending || s.pending
		knownPages := (len(items) + size - 1) / size
		if knownPages < 1 {
			knownPages = 1
		}
		page := cursor.Page
		if page > knownPages {
			page = knownPages
		}
		begin := (page - 1) * size
		end := begin + size
		if end > len(items) {
			end = len(items)
		}
		part := overviewSection{Items: items[begin:end], Page: page, PageSize: size, Status: "ok", Coverage: coverage, ObservedAt: s.observed[name], Loading: s.loading}
		if old {
			part.Status = "stale"
		}
		if coverage.Complete {
			total := len(items)
			part.Total, part.Pages = &total, &knownPages
		} else {
			part.Status = "partial"
		}
		if !s.loading {
			next := cursor
			if end < len(items) {
				next.Page = page + 1
				part.HasMore = true
			} else if cursor.Batch+overviewProjectBatch < totalProjects {
				next.Batch += overviewProjectBatch
				next.Page = 1
				part.HasMore = true
			}
			if part.HasMore {
				part.NextCursor = next.encode()
			}
		}
		if name == "attention" {
			part.Pending = s.pending
		}
		if name == "activity" {
			part.Source = "notification_events"
		}
		sections[name] = part
		for _, item := range s.errors {
			if !seenErrors[item] {
				errors = append(errors, item)
				seenErrors[item] = true
			}
		}
		if s.observed[name] != nil && (observed == nil || *s.observed[name] < *observed) {
			observed = s.observed[name]
		}
	}
	if stale {
		status = "stale"
	}
	for _, section := range sections {
		if section.Status == "partial" {
			status = "partial"
		}
	}
	counts := map[string]*int{"projects": &totalProjects, "attention": sections["attention"].Total, "working": sections["working"].Total}
	return map[string]any{"schema_version": 1, "generated_at": now.UTC().Format(time.RFC3339Nano), "status": status, "freshness": map[string]any{"observed_at": observed, "stale": stale}, "counts": counts, "sections": sections, "errors": errors, "loading": loading, "pending": pending, "retry_after_ms": overviewRetryDelay(loading, pending), "limits": map[string]int{"project_batch": overviewProjectBatch, "concurrency": overviewConcurrency, "page_size_max": 50, "cache_batches": overviewCacheBatches, "response_max_bytes": overviewResponseMaxBytes}}
}

func overviewRetryDelay(loading, pending bool) int {
	if loading {
		return 500
	}
	if pending {
		return 5000
	}
	return 0
}

func overviewString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func overviewText(value any) string {
	text := []rune(overviewString(value))
	if len(text) > 512 {
		return string(text[:512]) + "…"
	}
	return string(text)
}

func overviewFields(source map[string]any, keys []string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, exists := source[key]; exists {
			switch value.(type) {
			case string:
				out[key] = overviewText(value)
			case nil, bool, float64, int, int64:
				out[key] = value
			}
		}
	}
	return out
}

func overviewSummaryFields(kind string, source map[string]any) map[string]any {
	switch kind {
	case "attention_reason":
		return overviewFields(source, []string{"type", "audience", "title", "message", "action", "resume_condition", "severity", "wait_kind", "evidence"})
	case "activity":
		return overviewFields(source, []string{"health", "runtime_state", "agent_state", "last_activity_at", "last_runtime_at", "message", "attempt_id", "assignment_id"})
	default:
		return overviewFields(source, []string{"lifecycle_inferred", "lifecycle_inference_note", "timing_incomplete", "timing_incomplete_note", "registered_at", "created_at", "started_at", "claimed_at", "completed_at", "current_state", "current_state_at", "current_state_source", "current_segment_seconds", "queue_seconds", "active_seconds", "wait_seconds", "work_seconds", "lead_seconds", "elapsed_seconds", "duration_seconds", "queue", "active", "wait", "work", "lead", "elapsed", "duration", "total_elapsed_ms", "work_elapsed_ms", "running_ms", "review_ms", "rework_ms", "waiting_ms", "total_work_ms", "total_review_ms", "registration_to_start_ms", "first_started_at", "last_started_at", "measured", "source", "state", "status"})
	}
}

func overviewHref(project, root, id string) string {
	query := url.Values{"project": {project}, "backlog": {root}}
	if id != "" {
		return "/tasks/" + url.PathEscape(id) + "?" + query.Encode()
	}
	return "/?" + query.Encode()
}

// No DashboardSnapshot/AttentionSnapshot/WorkloadSnapshot calls: even the
// non-notifying attention path reconciles runtime and lifecycle ledgers.
// ReadLifecycleProjection returns a detached canonical summary; pending or
// missing summaries remain provisional and cannot establish zero attention.
func loadOverviewProject(ctx stdcontext.Context, project maintenance.ProjectState) overviewProject {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result := overviewProject{row: map[string]any{"name": project.Name, "path": project.Path, "counts": nil, "status": "ok", "observed_at": now, "error": nil}, items: map[string][]map[string]any{}, complete: map[string]bool{"projects": true}, observed: now}
	fail := func(status, message string) overviewProject {
		result.row["status"], result.row["error"] = status, message
		result.errors = append(result.errors, overviewError{Project: project.Path, Message: message})
		return result
	}
	if !project.Monitoring {
		return fail("paused", "project monitoring is paused")
	}
	if project.Presence != "present" {
		return fail(project.Presence, "project is not available")
	}
	if ctx.Err() != nil {
		return fail("error", ctx.Err().Error())
	}
	if !maintenance.ProjectMonitoringAllowed(project.Path) {
		return fail("paused", "project monitoring is not allowed")
	}
	projectCtx, err := webContext(project.Path, "")
	if err != nil {
		return fail("error", err.Error())
	}
	root, _, err := resolveBacklog(project.Path, projectCtx, nil)
	if err != nil {
		return fail("error", err.Error())
	}
	rows := []backlog.Record{}
	if root != "" {
		rows, err = backlog.CachedCatalog(project.Path, root)
		if err != nil {
			return fail("error", err.Error())
		}
	}
	// Match the canonical catalog's active-over-archive ID preference, without
	// a second filesystem walk or calling a reconciling list endpoint per page.
	byID := map[string]backlog.Record{}
	for _, row := range rows {
		old, exists := byID[row.ID]
		if !exists || (row.Location == "active" && old.Location != "active") {
			byID[row.ID] = row
		}
	}
	attentionKnown := true
	attentionPending := false
	attentionError := ""
	counts := map[string]any{"all": len(byID), "working": 0, "hold": 0, "done": 0, "attention": nil}
	for _, row := range byID {
		if row.State == "done" {
			counts["done"] = counts["done"].(int) + 1
		}
		if row.Location != "active" {
			continue
		}
		if row.State == "doing" {
			counts["working"] = counts["working"].(int) + 1
		}
		if row.State == "hold" {
			counts["hold"] = counts["hold"].(int) + 1
		}
		projection, available := backlog.ReadLifecycleProjection(project.Path, root, row.ID)
		meta, _ := projection["snapshot"].(map[string]any)
		trust := overviewString(meta["trust"])
		computed, computedErr := time.Parse(time.RFC3339Nano, overviewString(meta["computed_at"]))
		modified, modifiedErr := time.Parse(time.RFC3339Nano, row.Mtime)
		canonical := available && projection["not_found"] != true && (trust == "verified" || trust == "stale") && projection["file_state"] == row.State && computedErr == nil && modifiedErr == nil && !computed.Before(modified)
		item := map[string]any{"id": row.ID, "title": row.Title, "state": row.State, "file_state": row.State, "updated_at": row.Mtime, "agent": row.Fields["Agent"], "lifecycle": nil, "activity": nil, "snapshot": meta, "state_provisional": !canonical}
		if row.State == "hold" || row.State == "todo" {
			item["agent"] = "" // Same released/unassigned semantics as backlog summaries.
		}
		tags := []string{}
		seenTags := map[string]bool{}
		for _, raw := range strings.Split(strings.ReplaceAll(row.Fields["Tags"], "\n", ","), ",") {
			if tag, err := backlog.NormalizeTag(raw); err == nil && tag != "" && !seenTags[tag] {
				tags = append(tags, tag)
				seenTags[tag] = true
			}
		}
		sort.Strings(tags)
		item["tags"] = tags
		if !canonical {
			attentionKnown = false
			if trust == "error" {
				attentionError = overviewText(meta["diagnostic"])
			} else {
				attentionPending = true
			}
		} else {
			for _, key := range []string{"state", "lifecycle", "activity", "attention_reason"} {
				if value, exists := projection[key]; exists {
					if fields, ok := value.(map[string]any); ok {
						item[key] = overviewSummaryFields(key, fields)
					} else {
						item[key] = value
					}
				}
			}
			if trust != "verified" {
				result.stale = true
			}
			observed := overviewString(meta["validated_at"])
			if observed == "" {
				observed = overviewString(meta["computed_at"])
			}
			if observed != "" && observed < result.observed {
				result.observed = observed
			}
		}
		item["project"], item["project_name"], item["backlog"] = project.Path, project.Name, root
		item["href"] = overviewHref(project.Path, root, row.ID)
		item["title"] = overviewText(item["title"])
		item["reason"] = item["attention_reason"]
		item["evidence"] = map[string]any{"source": "backlog_file", "path": row.Path}
		if canonical {
			item["evidence"] = map[string]any{"source": "lifecycle_projection", "path": row.Path, "snapshot": meta}
		}
		if reason, ok := item["attention_reason"].(map[string]any); ok && len(reason) > 0 && overviewString(reason["audience"]) != "controller" {
			result.items["attention"] = append(result.items["attention"], item)
		}
		if row.State == "doing" {
			result.items["working"] = append(result.items["working"], item)
		}
	}
	if attentionKnown {
		counts["attention"] = len(result.items["attention"])
	} else {
		result.row["status"] = "partial"
		result.pending = attentionPending
		message := "canonical attention projection is pending; file states are provisional"
		if attentionError != "" {
			message = "canonical attention projection unavailable: " + attentionError
		}
		result.errors = append(result.errors, overviewError{Project: project.Path, Message: message})
	}
	result.complete["attention"], result.complete["working"] = attentionKnown, true
	result.row["counts"], result.row["backlog"] = counts, root
	result.row["href"] = overviewHref(project.Path, root, "")
	events, eventErr := backlog.ReadNotificationEvents(project.Path)
	if eventErr != nil {
		result.row["status"] = "partial"
		result.errors = append(result.errors, overviewError{Project: project.Path, Message: "notification events: " + eventErr.Error()})
	} else {
		result.complete["activity"] = true
		for _, event := range events {
			id := overviewString(event["task_id"])
			result.items["activity"] = append(result.items["activity"], map[string]any{"id": event["id"], "task_id": id, "title": overviewText(event["title"]), "project": project.Path, "project_name": project.Name, "backlog": root, "kind": event["kind"], "at": event["at"], "evidence": overviewFields(event, []string{"id", "kind", "at", "message", "resume_condition"}), "href": overviewHref(project.Path, root, id)})
		}
	}
	if result.stale && result.row["status"] == "ok" {
		result.row["status"] = "stale"
	}
	result.row["observed_at"] = result.observed
	if !maintenance.ProjectMonitoringAllowed(project.Path) {
		result.items = map[string][]map[string]any{}
		result.complete = map[string]bool{"projects": true}
		result.row["counts"] = nil
		return fail("paused", "project monitoring changed during overview read")
	}
	return result
}

// A process-local invalidation bus observes the existing control-plane output.
// It never opens a project, refreshes a feed, scans a ledger or starts a timer.
// Its revision invalidates all handler-local aggregate caches lazily on read.
const overviewEventSubscriberLimit = 256
const overviewEventHeartbeat = 15 * time.Second

type overviewHint struct {
	SchemaVersion int    `json:"schema_version"`
	Revision      string `json:"revision"`
	Reason        string `json:"reason"`
}

type overviewNotifier struct {
	mu          sync.Mutex
	epoch       string
	sequence    uint64
	registry    string
	subscribers map[chan overviewHint]struct{}
}

func newOverviewNotifier() *overviewNotifier {
	epoch, err := randomHex(12)
	if err != nil {
		epoch = strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return &overviewNotifier{epoch: epoch, subscribers: map[chan overviewHint]struct{}{}}
}

var overviewEvents = newOverviewNotifier()

func (n *overviewNotifier) hintLocked(reason string) overviewHint {
	return overviewHint{SchemaVersion: 1, Revision: n.epoch + ":" + strconv.FormatUint(n.sequence, 10), Reason: reason}
}

func (n *overviewNotifier) current() overviewHint {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.hintLocked("ready")
}

func (n *overviewNotifier) publishLocked(reason string) {
	n.sequence++
	hint := n.hintLocked(reason)
	for ch := range n.subscribers {
		// Retain only the newest hint. Every send, drain and close is under
		// this mutex; cancel/shutdown cannot race a send to a closed channel.
		select {
		case ch <- hint:
		default:
			select {
			case <-ch:
			default:
			}
			ch <- hint
		}
	}
}

func (n *overviewNotifier) publish(reason string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.publishLocked(reason)
}

func (n *overviewNotifier) observeRegistry(projects []maintenance.ProjectState) {
	key := overviewRegistryKey(projects)
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.registry == key {
		return
	}
	n.registry = key
	n.publishLocked("membership")
}

func (n *overviewNotifier) subscribe() (chan overviewHint, overviewHint, func(), error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.subscribers) >= overviewEventSubscriberLimit {
		return nil, overviewHint{}, nil, fmt.Errorf("overview event subscriber limit reached")
	}
	ch := make(chan overviewHint, 1)
	n.subscribers[ch] = struct{}{}
	cancel := func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		if _, ok := n.subscribers[ch]; ok {
			delete(n.subscribers, ch)
			close(ch)
		}
	}
	return ch, n.hintLocked("ready"), cancel, nil
}

// Called by the HTTP server shutdown hook. Preserve epoch/sequence so another
// handler in this process can resume; only the current connections are closed.
func (n *overviewNotifier) closeSubscribers() {
	n.mu.Lock()
	defer n.mu.Unlock()
	for ch := range n.subscribers {
		delete(n.subscribers, ch)
		close(ch)
	}
}

func (n *overviewNotifier) serveHTTP(w http.ResponseWriter, r *http.Request) {
	n.serve(w, r, overviewEventHeartbeat)
}

func (n *overviewNotifier) serve(w http.ResponseWriter, r *http.Request, heartbeat time.Duration) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := w.(http.Flusher); !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	ch, ready, cancel, err := n.subscribe()
	if err != nil {
		w.Header().Set("Retry-After", "3")
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	write := func(hint *overviewHint) bool {
		_ = controller.SetWriteDeadline(time.Now().Add(time.Second))
		var err error
		if hint == nil {
			_, err = fmt.Fprint(w, ": keepalive\n\n")
		} else {
			data, _ := json.Marshal(hint)
			_, err = fmt.Fprintf(w, "id: %s\nevent: overview\ndata: %s\n\n", hint.Revision, data)
		}
		return err == nil && controller.Flush() == nil
	}
	// Every connection/reconnection gets one current hint. The HTTP snapshot
	// revision lets clients skip an identical baseline without another fetch.
	if !write(&ready) {
		return
	}
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case hint, ok := <-ch:
			if !ok {
				return
			}
			if !write(&hint) {
				return
			}
		case <-ticker.C:
			if !write(nil) {
				return
			}
		}
	}
}

// Stable semantic fields only: age, elapsed durations, snapshot_at and
// computed_at advance on idle refreshes and must never trigger invalidation.
func overviewPayloadRevision(payload map[string]any) string {
	items := map[string]any{}
	add := func(id string, item map[string]any) {
		row := map[string]any{}
		for _, key := range []string{"id", "title", "state", "file_state", "updated_at", "agent", "tags", "attention_reason"} {
			if value, ok := item[key]; ok {
				row[key] = value
			}
		}
		if signal, ok := item["activity"].(map[string]any); ok {
			row["activity"] = overviewSummaryFields("activity", signal)
		}
		if lifecycle, ok := item["lifecycle"].(map[string]any); ok {
			row["lifecycle"] = overviewFields(lifecycle, []string{"current_state", "current_state_at", "created_at", "started_at", "claimed_at", "completed_at"})
		}
		items[id] = row
	}
	switch rows := payload["all_items"].(type) {
	case map[string]map[string]any:
		for id, item := range rows {
			add(id, item)
		}
	case map[string]any:
		for id, raw := range rows {
			if item, ok := raw.(map[string]any); ok {
				add(id, item)
			}
		}
	}
	data, _ := json.Marshal(map[string]any{"items": items, "attention_revision": attentionRevision(payload), "error": payload["error"]})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func publishOverviewFeedHint(feed *attentionFeed, payload map[string]any) {
	revision := overviewPayloadRevision(payload)
	// Existing feed lifecycle ordering is attentionFeeds -> feed.mu. The
	// notifier never takes either lock, and no aggregate mutex is acquired.
	attentionFeeds.Lock()
	defer attentionFeeds.Unlock()
	if attentionFeeds.feeds[feed.project+"\x00"+feed.root] != feed {
		return
	} // retired source cannot invalidate a replacement
	feed.mu.Lock()
	changed := feed.overviewRevision != revision
	feed.overviewRevision = revision
	feed.mu.Unlock()
	if changed {
		overviewEvents.publish("changed")
	}
}
