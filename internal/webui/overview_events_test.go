package webui

import (
	"bufio"
	"bytes"
	stdcontext "context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/goassets"
	"github.com/silverkhan/TaskMecca/internal/maintenance"
)

func TestOverviewEventsBoundedCoalescingIdleAndCancel(t *testing.T) {
	n := newOverviewNotifier()
	ch, ready, cancel, err := n.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	if ready.Reason != "ready" || cap(ch) != 1 {
		t.Fatal("bad bootstrap/bounded queue")
	}
	for i := 0; i < 1000; i++ {
		n.publish("changed")
	}
	if len(ch) != 1 {
		t.Fatal("unbounded hint backlog")
	}
	hint := <-ch
	if hint.Revision != n.current().Revision || hint.Reason != "changed" {
		t.Fatal("coalescing lost newest hint")
	}
	encoded, _ := json.Marshal(hint)
	if len(encoded) > 160 {
		t.Fatal("hint contains source data instead of revision")
	}
	before := n.current().Revision
	for i := 0; i < 100; i++ {
		n.current()
	}
	if n.current().Revision != before || len(ch) != 0 {
		t.Fatal("idle reads generated changes")
	}
	cancel()
	cancel()
	if _, ok := <-ch; ok {
		t.Fatal("cancel did not close subscription")
	}
	_, _, again, err := n.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	n.closeSubscribers()
	again()
	if n.current().Revision != before {
		t.Fatal("shutdown changed process revision")
	}
	newCh, newReady, done, err := n.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	if newReady.Revision != before {
		t.Fatal("same-process reconnect lost revision")
	}
	n.publish("membership")
	if (<-newCh).Reason != "membership" {
		t.Fatal("reconnected subscription unusable")
	}
	// Exercise publication against simultaneous cancel and shutdown under race.
	var workers sync.WaitGroup
	for i := 0; i < 50; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 20; j++ {
				c, _, stop, e := n.subscribe()
				if e == nil {
					n.publish("changed")
					select {
					case <-c:
					default:
					}
					stop()
				}
			}
		}()
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		for i := 0; i < 20; i++ {
			n.closeSubscribers()
		}
	}()
	workers.Wait()
	done()
	n.closeSubscribers()
	n.mu.Lock()
	remaining := len(n.subscribers)
	n.mu.Unlock()
	if remaining != 0 {
		t.Fatal("subscriber leak")
	}
}

func TestOverviewEventsSubscriberLimitAndRegistryChanges(t *testing.T) {
	n := newOverviewNotifier()
	methodRec := httptest.NewRecorder()
	n.serveHTTP(methodRec, httptest.NewRequest(http.MethodPost, "/api/overview/events", nil))
	if methodRec.Code != 405 || methodRec.Header().Get("Allow") != "GET" {
		t.Fatal("SSE route method guard changed")
	}
	stops := []func(){}
	for i := 0; i < overviewEventSubscriberLimit; i++ {
		_, _, stop, err := n.subscribe()
		if err != nil {
			t.Fatal(err)
		}
		stops = append(stops, stop)
	}
	defer func() {
		for _, stop := range stops {
			stop()
		}
	}()
	if _, _, _, err := n.subscribe(); err == nil {
		t.Fatal("subscriber ceiling ignored")
	}
	rec := httptest.NewRecorder()
	n.serveHTTP(rec, httptest.NewRequest("GET", "/api/overview/events", nil))
	if rec.Code != 503 || rec.Header().Get("Retry-After") != "3" {
		t.Fatal("capacity failure is not explicit/retryable")
	}
	projects := overviewTestProjects(2)
	n.observeRegistry(projects)
	first := n.current().Revision
	projects[0].CheckedAt = time.Now().Add(time.Second).Format(time.RFC3339Nano)
	n.observeRegistry(projects)
	if n.current().Revision != first {
		t.Fatal("registry observation clock caused idle invalidation")
	}
	projects[0].Monitoring = false
	n.observeRegistry(projects)
	paused := n.current().Revision
	if paused == first {
		t.Fatal("pause membership did not invalidate")
	}
	n.observeRegistry(projects[:1])
	if n.current().Revision == paused {
		t.Fatal("removed registry member did not invalidate")
	}
}

func overviewReadFrame(t *testing.T, reader *bufio.Reader) (string, *overviewHint) {
	t.Helper()
	var frame strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("SSE stream: %v", err)
		}
		frame.WriteString(line)
		if line == "\n" {
			break
		}
	}
	for _, line := range strings.Split(frame.String(), "\n") {
		if strings.HasPrefix(line, "data: ") {
			var hint overviewHint
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &hint); err != nil {
				t.Fatal(err)
			}
			return frame.String(), &hint
		}
	}
	return frame.String(), nil
}

func TestOverviewEventsHeartbeatReconnectDisconnectAndGracefulShutdown(t *testing.T) {
	n := newOverviewNotifier()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { n.serve(w, r, 5*time.Millisecond) }))
	server.Config.RegisterOnShutdown(n.closeSubscribers)
	server.Start()
	defer server.Close()
	client := &http.Client{Timeout: 2 * time.Second}
	connect := func(last string) (*http.Response, *bufio.Reader) {
		t.Helper()
		req, _ := http.NewRequest("GET", server.URL+"/api/overview/events", nil)
		if last != "" {
			req.Header.Set("Last-Event-ID", last)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 200 || !strings.Contains(response.Header.Get("Content-Type"), "text/event-stream") || response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("not a read-only SSE response")
		}
		return response, bufio.NewReader(response.Body)
	}
	response, reader := connect("")
	frame, ready := overviewReadFrame(t, reader)
	if ready == nil || ready.Reason != "ready" || !strings.Contains(frame, "event: overview") {
		t.Fatal("missing current revision baseline")
	}
	baseline := ready.Revision
	for i := 0; i < 2; i++ {
		frame, hint := overviewReadFrame(t, reader)
		if hint != nil || !strings.HasPrefix(frame, ": keepalive") {
			t.Fatal("heartbeat caused a data event")
		}
	}
	if n.current().Revision != baseline {
		t.Fatal("heartbeat changed revision")
	}
	response.Body.Close()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		n.mu.Lock()
		count := len(n.subscribers)
		n.mu.Unlock()
		if count == 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	n.mu.Lock()
	count := len(n.subscribers)
	n.mu.Unlock()
	if count != 0 {
		t.Fatal("disconnect leaked subscription")
	}
	// A missed change while offline is carried by the next current revision.
	n.publish("changed")
	reconnected, rereader := connect(baseline)
	defer reconnected.Body.Close()
	_, current := overviewReadFrame(t, rereader)
	if current == nil || current.Revision == baseline || current.Revision != n.current().Revision {
		t.Fatal("reconnect lost missed source invalidation")
	}
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), time.Second)
	defer cancel()
	if err := server.Config.Shutdown(ctx); err != nil {
		t.Fatalf("SSE prevented graceful shutdown: %v", err)
	}
	_, err := io.Copy(io.Discard, rereader)
	if err != nil {
		t.Fatalf("shutdown did not end stream cleanly: %v", err)
	}
	n.mu.Lock()
	count = len(n.subscribers)
	n.mu.Unlock()
	if count != 0 {
		t.Fatal("shutdown leaked subscriptions")
	}
	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), 100*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("isolated fixture listener remained open")
	}
}

func TestOverviewEventsInvalidateTTLAndDiscardOldInflightResult(t *testing.T) {
	n := newOverviewNotifier()
	service := newOverviewService()
	service.revision = func() string { return n.current().Revision }
	projects := overviewTestProjects(1)
	key := overviewRegistryKey(projects)
	started, release := make(chan struct{}), make(chan struct{})
	var reads atomic.Int32
	service.load = func(_ stdcontext.Context, p maintenance.ProjectState) overviewProject {
		call := reads.Add(1)
		result := overviewTestProject(p, 1)
		result.items["working"][0]["title"] = fmt.Sprintf("generation-read-%d", call)
		if call == 1 {
			close(started)
			<-release
		}
		return result
	}
	service.read(projects, key, 0)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("source did not start")
	}
	n.publish("changed")
	close(release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		service.mu.Lock()
		pending := service.pending
		service.mu.Unlock()
		if !pending {
			break
		}
		time.Sleep(time.Millisecond)
	}
	service.mu.Lock()
	remaining, pending := len(service.cached), service.pending
	service.mu.Unlock()
	if remaining != 0 || pending {
		t.Fatal("old in-flight generation was published after invalidation")
	}
	fresh := overviewAwait(t, service, projects, 0)
	if fresh.items["working"][0]["title"] != "generation-read-2" || time.Since(fresh.at) >= overviewTTL {
		t.Fatal("bad fresh cache fixture")
	}
	n.publish("changed") // well before the five-second TTL expires
	invalidated := service.read(projects, key, 0)
	if !invalidated.loading || len(invalidated.items["working"]) != 0 {
		t.Fatal("old TTL cache leaked into a new generation")
	}
	newest := overviewAwait(t, service, projects, 0)
	if newest.items["working"][0]["title"] != "generation-read-3" {
		t.Fatal("changed aggregate did not recover")
	}
	for i := 0; i < 20; i++ {
		service.read(projects, key, 0)
	}
	if reads.Load() != 3 {
		t.Fatal("unchanged revision caused extra aggregate scans")
	}

}

func TestOverviewEventsStableSemanticFeedAndLateRetiredSource(t *testing.T) {
	feed := &attentionFeed{project: "/synthetic/overview-events", root: "/synthetic/overview-events/backlog", stopCh: make(chan struct{}), subscribers: map[chan []byte]struct{}{}}
	key := feed.project + "\x00" + feed.root
	attentionFeeds.Lock()
	attentionFeeds.feeds[key] = feed
	attentionFeeds.Unlock()
	t.Cleanup(func() {
		attentionFeeds.Lock()
		if attentionFeeds.feeds[key] == feed {
			stopAttentionFeedLocked(key, feed)
		}
		attentionFeeds.Unlock()
	})
	payload := map[string]any{"all_items": map[string]map[string]any{"A-1": {"id": "A-1", "title": "Original", "state": "doing", "updated_at": "2026-11-08T00:00:00Z", "lifecycle": map[string]any{"current_state": "doing", "started_at": "2026-11-08T00:00:00Z", "active_seconds": 1.0}}}, "attention": []map[string]any{}, "notification_events": []map[string]any{}}
	publishOverviewFeedHint(feed, payload)
	initial := overviewEvents.current().Revision
	item := payload["all_items"].(map[string]map[string]any)["A-1"]
	life := item["lifecycle"].(map[string]any)
	life["active_seconds"] = 99.0
	life["elapsed_seconds"] = 99.0
	payload["snapshot_at"] = "later clock only"
	item["snapshot"] = map[string]any{"age_ms": 999, "computed_at": "later"}
	publishOverviewFeedHint(feed, payload)
	if overviewEvents.current().Revision != initial {
		t.Fatal("wall-clock-only refresh invalidated overview")
	}
	oldAttention := attentionRevision(payload)
	item["title"] = "Real working task title edit"
	if attentionRevision(payload) != oldAttention {
		t.Fatal("fixture no longer isolates the old attention-only fingerprint gap")
	}
	publishOverviewFeedHint(feed, payload)
	working := overviewEvents.current().Revision
	if working == initial {
		t.Fatal("actual working summary change did not invalidate")
	}
	life["completed_at"] = "2026-11-08T00:05:00Z"
	life["current_state"] = "done"
	publishOverviewFeedHint(feed, payload)
	if overviewEvents.current().Revision == working {
		t.Fatal("actual lifecycle transition did not invalidate")
	}
	attentionFeeds.Lock()
	stopAttentionFeedLocked(key, feed)
	attentionFeeds.Unlock()
	stopped := overviewEvents.current().Revision
	item["title"] = "Late result of retired feed"
	publishOverviewFeedHint(feed, payload)
	if overviewEvents.current().Revision != stopped {
		t.Fatal("retired source invalidated replacement data")
	}
}

func TestOverviewUIAssetsNestedMIMEBytesAndTraversalReadOnly(t *testing.T) {
	project, _ := overviewFixture(t)
	t.Setenv("TASK_MECCA_PREVIEW_MODE", "1")
	handler, err := Handler(project, "", "test")
	if err != nil {
		t.Fatal(err)
	}
	before := overviewTreeHashes(t, project)
	cases := []struct{ path, mime string }{{"ui/main.js", "javascript"}, {"ui/screens/home.js", "javascript"}, {"ui/smoke.css", "text/css"}, {"ui/fonts/manrope.ttf", "font"}}
	for _, c := range cases {
		expected, err := fs.ReadFile(goassets.Template, embeddedRoot+"/web/"+c.path)
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/"+c.path, nil))
		if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), c.mime) || rec.Header().Get("X-Content-Type-Options") != "nosniff" || !bytes.Equal(rec.Body.Bytes(), expected) {
			t.Fatalf("asset served as HTML/wrong bytes/MIME: %s HTTP%d %s", c.path, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
	for _, path := range []string{"/ui/", "/ui/screens/", "/ui/does-not-exist.js", "/ui/%2e%2e/app.js", "/ui/%5c..%5capp.js", "/ui/../../_task_mecca/data/backlog"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code == 200 {
			t.Fatalf("invalid static path became successful HTML: %s", path)
		}
	}
	for _, path := range []string{"/ui/main.js", "/api/overview/events"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("POST", path, nil))
		if rec.Code != http.StatusForbidden && rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("mutation method accepted: %s HTTP%d", path, rec.Code)
		}
	}
	if fmt.Sprint(before) != fmt.Sprint(overviewTreeHashes(t, project)) {
		t.Fatal("read-only static/event routes mutated fixture source")
	}
}

func TestOverviewEventsObserveExistingFeedRegistrationAndStop(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	project := t.TempDir()
	root := project + "/unregistered-synthetic-backlog"
	before := overviewEvents.current().Revision
	feed := ensureAttentionFeed(project, root)
	defer func() {
		stopProjectAttentionFeeds(project)
		select {
		case <-feed.doneCh:
		case <-time.After(time.Second):
			t.Error("fixture feed leaked")
		}
	}()
	registered := overviewEvents.current().Revision
	if registered == before {
		t.Fatal("existing feed registration was not observed")
	}
	if ensureAttentionFeed(project, root) != feed || overviewEvents.current().Revision != registered {
		t.Fatal("duplicate feed lookup generated invalidation")
	}
	stopProjectAttentionFeeds(project)
	stopped := overviewEvents.current().Revision
	if stopped == registered {
		t.Fatal("existing feed stop was not observed")
	}
	stopProjectAttentionFeeds(project)
	if overviewEvents.current().Revision != stopped {
		t.Fatal("duplicate stop generated invalidation")
	}
	if files := overviewTreeHashes(t, project); len(files) != 0 {
		t.Fatal("notifier created project evidence")
	}
}
