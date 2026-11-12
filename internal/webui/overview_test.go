package webui

import (
	stdcontext "context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/maintenance"
)

func overviewTestProjects(n int) []maintenance.ProjectState {
	projects := make([]maintenance.ProjectState, n)
	for i := range projects {
		projects[i] = maintenance.ProjectState{Project: maintenance.Project{Name: fmt.Sprintf("project-%03d", i), Path: fmt.Sprintf("/synthetic/project-%03d", i)}, Monitoring: true, Presence: "present"}
	}
	return projects
}

func overviewTestProject(project maintenance.ProjectState, count int) overviewProject {
	p := overviewProject{row: map[string]any{"path": project.Path, "name": project.Name}, items: map[string][]map[string]any{}, complete: map[string]bool{}, observed: time.Now().UTC().Format(time.RFC3339Nano)}
	for _, section := range overviewSections {
		p.complete[section] = true
		if section == "projects" {
			continue
		}
		for i := 0; i < count; i++ {
			p.items[section] = append(p.items[section], map[string]any{"id": fmt.Sprintf("%s-task-%d", project.Name, i), "href": fmt.Sprintf("%s/%d", project.Path, i), "title": "Synthetic task", "updated_at": "2026-11-07T16:00:00Z", "at": "2026-11-07T16:00:00Z"})
		}
	}
	return p
}

func overviewAwait(t *testing.T, s *overviewService, projects []maintenance.ProjectState, batch int) *overviewSnapshot {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := s.read(projects, overviewRegistryKey(projects), batch)
		if !snapshot.loading {
			return snapshot
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("bounded synthetic overview did not complete")
	return nil
}

func overviewTestResponse(s *overviewSnapshot, requests map[string]overviewCursor, size, total int) map[string]any {
	snapshots := map[string]*overviewSnapshot{}
	for _, section := range overviewSections {
		snapshots[section] = s
	}
	return overviewResponse(time.Now(), requests, size, snapshots, total)
}

func TestOverviewColdRequestReturnsBeforeBoundedReadAndSharesFourWorkers(t *testing.T) {
	projects := overviewTestProjects(10000)
	s := newOverviewService()
	var calls, active, maximum atomic.Int32
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	s.load = func(_ stdcontext.Context, p maintenance.ProjectState) overviewProject {
		calls.Add(1)
		n := active.Add(1)
		for {
			old := maximum.Load()
			if n <= old || maximum.CompareAndSwap(old, n) {
				break
			}
		}
		<-release
		active.Add(-1)
		return overviewTestProject(p, 2)
	}
	started := time.Now()
	returned := make(chan *overviewSnapshot, 1)
	go func() { returned <- s.read(projects, overviewRegistryKey(projects), 0) }()
	select {
	case first := <-returned:
		t.Logf("OVERVIEW_COLD_BOUND simulated_projects=%d blocked_source=true request_ms=%.3f", len(projects), float64(time.Since(started).Microseconds())/1000)
		if !first.loading || first.coverage["working"].ScannedProjects != 0 {
			t.Fatalf("cold result claims data: %+v", first)
		}
	case <-time.After(time.Second):
		t.Fatal("HTTP-facing read waited for blocked source")
	}
	for i := 0; i < 100; i++ {
		s.read(projects, overviewRegistryKey(projects), 0)
		s.read(projects, overviewRegistryKey(projects), 8)
	}
	if maximum.Load() > overviewConcurrency || calls.Load() > overviewConcurrency {
		t.Fatalf("unbounded cold fanout active=%d calls=%d", maximum.Load(), calls.Load())
	}
	once.Do(func() { close(release) })
	snapshot := overviewAwait(t, s, projects, 0)
	t.Logf("OVERVIEW_WORK_BOUND calls=%d max_concurrency=%d", calls.Load(), maximum.Load())
	if calls.Load() != overviewProjectBatch || maximum.Load() > 4 {
		t.Fatalf("expected one batch8 / workers4, calls=%d max=%d", calls.Load(), maximum.Load())
	}
	for _, section := range overviewSections {
		c := snapshot.coverage[section]
		if c.Complete || c.TotalProjects != 10000 || c.ScannedProjects != 8 {
			t.Fatalf("false global coverage: %s %+v", section, c)
		}
	}
	before := calls.Load()
	for i := 0; i < 20; i++ {
		s.read(projects, overviewRegistryKey(projects), 0)
	}
	if calls.Load() != before {
		t.Fatal("warm section/page requests repeated catalog reads")
	}
}

func TestOverviewCursorReachesEveryKnownRowAcrossProjectBatches(t *testing.T) {
	projects := overviewTestProjects(19)
	s := newOverviewService()
	s.load = func(_ stdcontext.Context, p maintenance.ProjectState) overviewProject {
		return overviewTestProject(p, 23)
	}
	requests := map[string]overviewCursor{}
	for _, section := range overviewSections {
		requests[section] = overviewCursor{Page: 1, Registry: overviewRegistryKey(projects)}
	}
	seen := map[string]bool{}
	for turn := 0; turn < 100; turn++ {
		cursor := requests["working"]
		snapshot := overviewAwait(t, s, projects, cursor.Batch)
		response := overviewTestResponse(snapshot, requests, 50, len(projects))
		part := response["sections"].(map[string]overviewSection)["working"]
		if part.Total != nil || part.Pages != nil || len(part.Items) > 50 {
			t.Fatalf("partial batch pretends to be global total: %+v", part)
		}
		if *response["counts"].(map[string]*int)["projects"] != 19 || response["counts"].(map[string]*int)["working"] != nil {
			t.Fatal("unknown working count became a false zero")
		}
		for _, item := range part.Items {
			id := item["id"].(string)
			if seen[id] {
				t.Fatalf("duplicate cursor row %s", id)
			}
			seen[id] = true
		}
		if !part.HasMore {
			break
		}
		next, err := decodeOverviewCursor(part.NextCursor)
		if err != nil || next == cursor {
			t.Fatalf("non-advancing cursor: %+v %v", next, err)
		}
		requests["working"] = next
	}
	if len(seen) != 19*23 {
		t.Fatalf("cursor lost records: %d want %d", len(seen), 19*23)
	}
}

func TestOverviewIndependentPagesUnknownAndEmptySemantics(t *testing.T) {
	projects := overviewTestProjects(1)
	s := newOverviewService()
	s.load = func(_ stdcontext.Context, p maintenance.ProjectState) overviewProject {
		return overviewTestProject(p, 25)
	}
	snapshot := overviewAwait(t, s, projects, 0)
	requests := map[string]overviewCursor{}
	for _, section := range overviewSections {
		requests[section] = overviewCursor{Page: 1, Registry: overviewRegistryKey(projects)}
	}
	requests["working"] = overviewCursor{Page: 2, Registry: overviewRegistryKey(projects)}
	parts := overviewTestResponse(snapshot, requests, 8, 1)["sections"].(map[string]overviewSection)
	if parts["working"].Page != 2 || parts["attention"].Page != 1 || *parts["working"].Total != 25 || *parts["working"].Pages != 4 {
		t.Fatal("sections do not paginate independently")
	}
	pages, size, err := overviewPagination(url.Values{"page_size": {"500"}, "attention_page": {"3"}})
	if err != nil || size != 50 || pages["attention"] != 3 || pages["working"] != 1 {
		t.Fatal("pagination contract changed")
	}
	for _, query := range []url.Values{{"page_size": {"0"}}, {"working_page": {"-1"}}, {"attention_page": {"999999999999999999999999"}}} {
		if _, _, err := overviewPagination(query); err == nil {
			t.Fatal("invalid pagination accepted")
		}
	}
	// Actual empty registry is a known zero; unavailable projects are not.
	empty := overviewAwait(t, newOverviewService(), nil, 0)
	response := overviewTestResponse(empty, requests, 8, 0)
	for _, part := range response["sections"].(map[string]overviewSection) {
		if part.Total == nil || *part.Total != 0 || part.HasMore {
			t.Fatal("true empty registry is not represented as empty")
		}
	}
	snapshot.coverage["attention"] = overviewCoverage{TotalProjects: 1}
	if overviewTestResponse(snapshot, requests, 8, 1)["counts"].(map[string]*int)["attention"] != nil {
		t.Fatal("incomplete attention was coerced to zero")
	}
}

func TestOverviewReadFailureStaleAndCacheInvalidation(t *testing.T) {
	projects := overviewTestProjects(1)
	s := newOverviewService()
	s.load = func(_ stdcontext.Context, p maintenance.ProjectState) overviewProject {
		result := overviewTestProject(p, 1)
		result.stale = true
		result.complete["attention"] = false
		result.errors = []overviewError{{Project: p.Path, Message: "synthetic read failure"}}
		return result
	}
	first := overviewAwait(t, s, projects, 0)
	requests := map[string]overviewCursor{}
	for _, name := range overviewSections {
		requests[name] = overviewCursor{Page: 1, Registry: overviewRegistryKey(projects)}
	}
	response := overviewTestResponse(first, requests, 8, 1)
	if response["status"] != "partial" || !response["freshness"].(map[string]any)["stale"].(bool) || len(response["errors"].([]overviewError)) != 1 {
		t.Fatal("stale data/error/coverage were hidden")
	}
	projects[0].Monitoring = false
	if !s.read(projects, overviewRegistryKey(projects), 0).loading {
		t.Fatal("paused registry reused an old successful snapshot")
	}
	overviewAwait(t, s, projects, 0)
}

func overviewFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	project := filepath.Join(root, "synthetic-project")
	t.Setenv("TASK_MECCA_HOME", home)
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "disabled")
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := maintenance.RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	return project, folder
}

func overviewTreeHashes(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err == nil {
			out[path] = fmt.Sprintf("%x", sha256.Sum256(data))
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestOverviewHTTPColdToDataReadOnlyAndRealTaskLinks(t *testing.T) {
	project, folder := overviewFixture(t)
	for i := 1; i <= 73; i++ {
		name := filepath.Join(folder, fmt.Sprintf("%06d.A-%d.synthetic.doing.md", i, i))
		if err := os.WriteFile(name, []byte(fmt.Sprintf("# A-%d Synthetic task\n- Agent: /root/synthetic\n- Tags: area:backend\n", i)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := overviewTreeHashes(t, filepath.Dir(project))
	mux := http.NewServeMux()
	registerOverviewRoute(mux, func(w http.ResponseWriter, payload any, status int) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(payload)
	})
	get := func(target string) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != 200 {
			t.Fatalf("HTTP%d %s", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("browser caching enabled")
		}
		if rec.Body.Len() > overviewResponseMaxBytes {
			t.Fatal("payload exceeded budget")
		}
		var data map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		return data
	}
	var response map[string]any
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		response = get("/api/overview")
		if response["loading"] == false {
			break
		}
		time.Sleep(time.Millisecond)
	}
	sections := response["sections"].(map[string]any)
	working := sections["working"].(map[string]any)
	if response["counts"].(map[string]any)["attention"] != nil || response["status"] != "partial" {
		t.Fatal("cold canonical attention became zero/healthy")
	}
	if working["total"] != float64(73) || len(working["items"].([]any)) != 8 {
		t.Fatalf("cold index did not become useful: %+v", working)
	}
	item := working["items"].([]any)[0].(map[string]any)
	if item["state_provisional"] != true || item["lifecycle"] != nil {
		t.Fatal("file doing was promoted to measured execution")
	}
	link, err := url.Parse(item["href"].(string))
	if err != nil || !strings.HasPrefix(link.Path, "/tasks/A-") || link.Query().Get("project") != project || link.Query().Get("backlog") != folder {
		t.Fatalf("broken existing task deep link: %v %+v", err, link)
	}
	seen := 0
	for {
		seen += len(working["items"].([]any))
		if !working["has_more"].(bool) {
			break
		}
		cursor := url.QueryEscape(working["next_cursor"].(string))
		response = get("/api/overview?working_cursor=" + cursor)
		working = response["sections"].(map[string]any)["working"].(map[string]any)
	}
	if seen != 73 {
		t.Fatalf("large backlog pagination lost records: %d", seen)
	}
	after := overviewTreeHashes(t, filepath.Dir(project))
	t.Logf("OVERVIEW_READONLY_HTTP files=%d before=%x after=%x", len(before), sha256.Sum256([]byte(fmt.Sprint(before))), sha256.Sum256([]byte(fmt.Sprint(after))))
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatal("overview mutated registry/backlog/runtime/notification/lifecycle files")
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, "/api/overview", nil))
		if rec.Code != 405 || rec.Header().Get("Allow") != "GET" {
			t.Fatal("overview accepted a mutation method")
		}
	}
	for _, target := range []string{"/api/overview?page_size=0", "/api/overview?working_cursor=bad", "/api/overview?activity_page=-2"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != 400 {
			t.Fatal("invalid query accepted")
		}
	}
}

func TestOverviewRegistryFailureAndMonitoringSuppression(t *testing.T) {
	project, _ := overviewFixture(t)
	if err := maintenance.SetProjectMonitoring(project, false); err != nil {
		t.Fatal(err)
	}
	projects, err := maintenance.ManagedProjects()
	if err != nil {
		t.Fatal(err)
	}
	p := loadOverviewProject(stdcontext.Background(), projects[0])
	if p.row["status"] != "paused" || p.row["counts"] != nil || p.complete["working"] || p.complete["attention"] {
		t.Fatal("paused project became healthy zero or was read")
	}
	registry := filepath.Join(os.Getenv("TASK_MECCA_HOME"), "projects.json")
	if err := os.WriteFile(registry, []byte("{invalid registry"), 0600); err != nil {
		t.Fatal(err)
	}
	before := overviewTreeHashes(t, filepath.Dir(project))
	mux := http.NewServeMux()
	registerOverviewRoute(mux, func(w http.ResponseWriter, p any, status int) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(p)
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/overview", nil))
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "registry") {
		t.Fatal("registry failure was silently an empty portfolio")
	}
	if fmt.Sprint(before) != fmt.Sprint(overviewTreeHashes(t, filepath.Dir(project))) {
		t.Fatal("registry failure rewrote source")
	}
}

func TestOverviewAutomaticMonitorWarmsCanonicalAttentionWithoutUIReadWrites(t *testing.T) {
	project, folder := overviewFixture(t)
	name := filepath.Join(folder, "000001.A-1.synthetic.hold.md")
	if err := os.WriteFile(name, []byte("# A-1 Synthetic judgment needed\n- 대기유형: user\n- 대기: Choose a prototype direction\n- 재개조건: User decision recorded\n- 대기근거: Synthetic review fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Seed only a declared synthetic user-wait file. Run the real service
	// monitor entry point, with no operations/attention/SSE/browser request.
	// The monitor, not overview, owns canonical projection publication.
	monitorCtx, cancel := stdcontext.WithCancel(stdcontext.Background())
	done := make(chan struct{})
	go func() { runOperationMonitor(monitorCtx, ""); close(done) }()
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			cancel()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("fixture monitor did not stop")
			}
			attentionFeeds.Lock()
			closing := []*attentionFeed{}
			for key, feed := range attentionFeeds.feeds {
				if feed.project == project {
					closing = append(closing, feed)
					stopAttentionFeedLocked(key, feed)
				}
			}
			attentionFeeds.Unlock()
			for _, feed := range closing {
				select {
				case <-feed.doneCh:
				case <-time.After(5 * time.Second):
					t.Error("fixture attention feed did not stop")
				}
			}
		})
	}
	t.Cleanup(stop)
	deadline := time.Now().Add(5 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		projection, available := backlog.ReadLifecycleProjection(project, folder, "A-1")
		meta, _ := projection["snapshot"].(map[string]any)
		if available && (meta["trust"] == "verified" || meta["trust"] == "stale") && projection["attention_reason"] != nil {
			ready = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	stop()
	if !ready {
		managed, _ := maintenance.ManagedProjects()
		t.Logf("paths fixture=%s root=%s managed=%+v preview=%v", project, folder, managed, PreviewMode())
		for _, p := range managed {
			v, a := backlog.ReadLifecycleProjection(p.Path, filepath.Join(p.Path, "_task_mecca", "data", "backlog"), "A-1")
			t.Logf("managed projection available=%v value=%+v", a, v)
		}
		t.Fatal("production monitor did not warm canonical projection without UI")
	}
	before := overviewTreeHashes(t, filepath.Dir(project))
	projects, err := maintenance.ManagedProjects()
	if err != nil {
		t.Fatal(err)
	}
	result := loadOverviewProject(stdcontext.Background(), projects[0])
	if !result.complete["attention"] || len(result.items["attention"]) != 1 {
		t.Fatalf("canonical user decision missing: %+v errors=%+v", result.items, result.errors)
	}
	item := result.items["attention"][0]
	if item["state_provisional"] != false || item["reason"].(map[string]any)["type"] != "user_intervention" || !strings.Contains(item["href"].(string), "/tasks/A-1?") {
		t.Fatal("canonical attention lost meaning or task link")
	}
	if fmt.Sprint(before) != fmt.Sprint(overviewTreeHashes(t, filepath.Dir(project))) {
		t.Fatal("canonical overview read changed ledgers")
	}
	t.Logf("OVERVIEW_READONLY_CANONICAL files=%d before=%x after=%x", len(before), sha256.Sum256([]byte(fmt.Sprint(before))), sha256.Sum256([]byte(fmt.Sprint(overviewTreeHashes(t, filepath.Dir(project))))))
	service := newOverviewService()
	snapshot := overviewAwait(t, service, projects, 0)
	requests := map[string]overviewCursor{}
	for _, name := range overviewSections {
		requests[name] = overviewCursor{Page: 1, Registry: overviewRegistryKey(projects)}
	}
	encoded, _ := json.Marshal(overviewTestResponse(snapshot, requests, 8, len(projects)))
	t.Logf("OVERVIEW_SYNTHETIC_CANONICAL_RESPONSE %s", encoded)

}

func TestOverviewCacheBoundAndStaleWhileRevalidate(t *testing.T) {
	projects := overviewTestProjects(80)
	s := newOverviewService()
	s.load = func(_ stdcontext.Context, p maintenance.ProjectState) overviewProject {
		return overviewTestProject(p, 1)
	}
	for offset := 0; offset < len(projects); offset += 8 {
		overviewAwait(t, s, projects, offset)
		s.mu.Lock()
		n := len(s.cached)
		s.mu.Unlock()
		if n > overviewCacheBatches {
			t.Fatalf("unbounded batch cache: %d", n)
		}
	}
	snapshot := overviewAwait(t, s, projects, 72)
	s.mu.Lock()
	snapshot.at = time.Now().Add(-overviewTTL - time.Second)
	s.mu.Unlock()
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	var loads atomic.Int32
	s.load = func(_ stdcontext.Context, p maintenance.ProjectState) overviewProject {
		loads.Add(1)
		<-release
		return overviewTestProject(p, 1)
	}
	stale := s.read(projects, overviewRegistryKey(projects), 72)
	if !stale.loading || len(stale.items["working"]) == 0 || stale.at.After(time.Now().Add(-overviewTTL)) {
		t.Fatal("stale cache blocked or pretended fresh")
	}
	for i := 0; i < 20; i++ {
		s.read(projects, overviewRegistryKey(projects), 72)
	}
	if loads.Load() > 4 {
		t.Fatal("stale refresh stampede")
	}
	once.Do(func() { close(release) })
	overviewAwait(t, s, projects, 72)
}

func TestOverviewResponseBudgetPreservesUnknownAndRejectsOversize(t *testing.T) {
	if got := overviewText(strings.Repeat("한", 50000)); len([]rune(got)) != 513 {
		t.Fatal("summary string limit failed")
	}
	payload := map[string]any{"untrusted_oversize_summary": strings.Repeat("x", overviewResponseMaxBytes+1)}
	rec := httptest.NewRecorder()
	writeOverviewResponse(rec, payload, func(w http.ResponseWriter, p any, status int) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(p)
	})
	if rec.Code != http.StatusRequestEntityTooLarge || rec.Body.Len() > 1024 || !strings.Contains(rec.Body.String(), "smaller page_size") {
		t.Fatal("oversize response escaped the hard budget or lost recovery guidance")
	}
	if overviewRetryDelay(true, false) != 500 || overviewRetryDelay(false, true) != 5000 || overviewRetryDelay(false, false) != 0 {
		t.Fatal("bounded loading/projection polling hints changed")
	}
}
