package webui

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"github.com/silverkhan/TaskMecca/internal/notify"
)

type attentionFeed struct {
	project     string
	root        string
	stopCh      chan struct{}
    // Governed by attentionFeeds.Mutex. UI subscribers do not own a durable loop.
    durable bool
	doneCh      chan struct{}
	mu          sync.Mutex
	latest      []byte
	revision    string
	lastDeliveryWarning time.Time
	subscribers map[chan []byte]struct{}
}

var attentionFeeds = struct {
	sync.Mutex
	feeds map[string]*attentionFeed
}{feeds: map[string]*attentionFeed{}}

func notificationValue(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func attentionRevision(payload map[string]any) string {
	parts := []string{}
	if rows, ok := payload["attention"].([]map[string]any); ok {
		for _, row := range rows {
			parts = append(parts, strings.Join([]string{
				fmt.Sprint(row["id"]), fmt.Sprint(row["type"]), fmt.Sprint(row["health"]),
				fmt.Sprint(row["runtime_state"]), fmt.Sprint(row["last_activity_at"]),
				fmt.Sprint(row["title"]), fmt.Sprint(row["message"]), fmt.Sprint(row["resume_condition"]),
			}, "|"))
		}
	}
	if events, ok := payload["notification_events"].([]map[string]any); ok {
		for _, event := range events {
			parts = append(parts, "event:"+fmt.Sprint(event["id"]))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n")
}

func ensureAttentionFeed(project, root string) *attentionFeed {
	key := project + "\x00" + root
	attentionFeeds.Lock()
	if feed := attentionFeeds.feeds[key]; feed != nil {
		attentionFeeds.Unlock()
		return feed
	}
	feed := &attentionFeed{project: project, root: root, stopCh: make(chan struct{}), doneCh: make(chan struct{}), subscribers: map[chan []byte]struct{}{}}
	attentionFeeds.feeds[key] = feed
	attentionFeeds.Unlock()
	go feed.run()
	return feed
}

// A browser-only feed is deleted as soon as its last subscriber leaves.
// Exactly one canonical root per monitored project owns persistent Telegram
// notification reconciliation, independently of any browser subscription.
func ensureDurableAttentionFeed(project, root string) *attentionFeed {
    key := project + "\x00" + root
    attentionFeeds.Lock()
    defer attentionFeeds.Unlock()
    for oldKey, old := range attentionFeeds.feeds {
        if old.project != project || oldKey == key || !old.durable { continue }
        old.durable = false
        old.mu.Lock()
        unused := len(old.subscribers) == 0
        old.mu.Unlock()
        if unused { stopAttentionFeedLocked(oldKey, old) }
    }
    feed := attentionFeeds.feeds[key]
    if feed == nil {
        feed = &attentionFeed{project:project, root:root,stopCh:make(chan struct{}),doneCh:make(chan struct{}),subscribers:map[chan []byte]struct{}{}}
        attentionFeeds.feeds[key] = feed
        go feed.run()
    }
    feed.durable = true
    return feed
}

// Must be called with attentionFeeds locked. Closing subscriptions wakes
// stale SSE requests so a removed project cannot keep a client loop alive.
func stopAttentionFeedLocked(key string, feed *attentionFeed) {
    if attentionFeeds.feeds[key] != feed { return }
    delete(attentionFeeds.feeds, key)
    close(feed.stopCh)
    feed.mu.Lock()
    for ch := range feed.subscribers {
        delete(feed.subscribers, ch)
        close(ch)
    }
    feed.mu.Unlock()
}

func stopProjectAttentionFeeds(project string) {
    attentionFeeds.Lock()
    defer attentionFeeds.Unlock()
    for key,feed := range attentionFeeds.feeds {
        if feed.project == project { stopAttentionFeedLocked(key, feed) }
    }
}

// Registry changes are reconciled even when no UI is connected. A project
// with monitoring paused/removed has no background attention goroutines.
func reconcileAttentionFeedProjects(active []string) {
    // A temporary registry read failure is not evidence of removal. Avoid
    // stopping healthy project feeds when management metadata is unavailable.
    if _, err := maintenance.ManagedProjects(); err != nil { return }
    allowed := make(map[string]bool,len(active))
    for _, project := range active { allowed[project] = true }
    attentionFeeds.Lock()
    defer attentionFeeds.Unlock()
    for key,feed := range attentionFeeds.feeds {
        if !allowed[feed.project] { stopAttentionFeedLocked(key,feed) }
    }
}

func (f *attentionFeed) refresh() {
    start := time.Now()
    executed := false
    maintenance.TryWithProjectMonitoring(f.project, func() {
        executed = true
        f.refreshActive()
    })
    if !executed {
        recordMonitorPerf("attention_skipped", time.Since(start), 0, 0, false, true)
    }
}

func (f *attentionFeed) refreshActive() {
    start := time.Now()
    payload, profile, err := backlog.AttentionSnapshotWithProfile(f.project, f.root, true)
    recordMonitorPerf("attention_snapshot", time.Since(start), 0, 0, false, false)
    // These are subphases of attention_snapshot; never add to its total.
    recordMonitorPerf("attention_control_runtime", time.Duration(profile["runtime"]*float64(time.Millisecond)), 0, 0, false, false)
    recordMonitorPerf("attention_control_lifecycle", time.Duration(profile["lifecycle_git"]*float64(time.Millisecond)), 0, 0, false, false)
    recordMonitorPerf("attention_control_activity", time.Duration(profile["activity_handoff"]*float64(time.Millisecond)), 0, 0, false, false)
	if err != nil {
		payload = map[string]any{"error": err.Error(), "attention": []map[string]any{}, "all_items": map[string]map[string]any{}, "notification_events": []map[string]any{}}
	}
	revision := attentionRevision(payload)
    deliveryStarted := time.Now()
	if rows, ok := payload["notification_events"].([]map[string]any); ok {
		events := make([]notify.Event, 0, len(rows))
		for _, row := range rows {
			events = append(events, notify.Event{
				ID: notificationValue(row["id"]), TaskID: notificationValue(row["task_id"]), Kind: notificationValue(row["kind"]),
				Title: notificationValue(row["title"]), Message: notificationValue(row["message"]),
				ResumeCondition: notificationValue(row["resume_condition"]), At: notificationValue(row["at"]),
			})
		}
		if errs := notify.Deliver(f.project, events); len(errs) > 0 {
			// HTTP errors can include credentials in request URLs. Never log
			// the raw error, bot token, destination or message contents.
			f.mu.Lock()
			now := time.Now()
			loggable := now.Sub(f.lastDeliveryWarning) >= time.Minute
			if loggable {
				f.lastDeliveryWarning = now
			}
			f.mu.Unlock()
			if loggable {
				log.Printf("Task Mecca automatic Telegram delivery issue in project %q: %d events, %d failures; inspect notification delivery ledger", filepath.Base(f.project), len(events), len(errs))
			}
		}
	}
    recordMonitorPerf("attention_delivery", time.Since(deliveryStarted), 0, 0, false, false)
    encodingStarted := time.Now()
	data, _ := json.Marshal(payload)
    recordMonitorPerf("attention_encoding", time.Since(encodingStarted), 0, 0, false, false)
	f.mu.Lock()
	changed := revision != f.revision || f.latest == nil
	f.revision = revision
	f.latest = append([]byte{}, data...)
	subscribers := make([]chan []byte, 0, len(f.subscribers))
	if changed {
		for ch := range f.subscribers {
			subscribers = append(subscribers, ch)
		}
	}
	f.mu.Unlock()
    attentionCount := 0
    notificationCount := 0
    if rows, ok := payload["attention"].([]map[string]any); ok { attentionCount = len(rows) }
    if rows, ok := payload["notification_events"].([]map[string]any); ok { notificationCount = len(rows) }
    recordMonitorPerf("attention_refresh", time.Since(start), attentionCount, notificationCount, changed, false)
	if changed {
		// Let foreground SSE consume a fresh canonical event first; background
		// Push shares exactly the same AID-119 claim ledger.
		if rows, ok := payload["notification_events"].([]map[string]any); ok && len(rows)>0 {
			current, _ := payload["all_items"].(map[string]map[string]any)
			go func(events []map[string]any,items map[string]map[string]any){
				time.Sleep(800*time.Millisecond)
				deliverWebPush(f.project,events,items)
			}(rows,current)
		}
		for _, ch := range subscribers {
			select {
			case ch <- append([]byte{}, data...):
			default:
			}
		}
	}
}

func (f *attentionFeed) run() {
	defer close(f.doneCh)
    select { case <-f.stopCh: return; default: }
	f.refresh()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			f.refresh()
		case <-f.stopCh:
			return
		}
	}
}

func (f *attentionFeed) subscribe() (chan []byte, []byte, func()) {
    ch := make(chan []byte, 4)
    // A project may be removed between ensureAttentionFeed and subscribe.
    // Serialize with registry teardown so a stopped feed cannot gain clients.
    attentionFeeds.Lock()
    if attentionFeeds.feeds[f.project+"\x00"+f.root] != f {
        close(ch)
        attentionFeeds.Unlock()
        return ch, nil, func(){}
    }
	f.mu.Lock()
	f.subscribers[ch] = struct{}{}
	initial := append([]byte{}, f.latest...)
	f.mu.Unlock()
    attentionFeeds.Unlock()
	cancel := func() {
		f.mu.Lock()
		if _, ok := f.subscribers[ch]; ok {
			delete(f.subscribers, ch)
			close(ch)
		}
        f.mu.Unlock()
        // Serialise deletion with feed creation. The unsubscribe callback
        // may race pause/archival, where the channel was already closed.
        attentionFeeds.Lock()
        f.mu.Lock()
        unused := len(f.subscribers)==0
        f.mu.Unlock()
        if unused && !f.durable {
            stopAttentionFeedLocked(f.project+"\x00"+f.root,f)
        }
        attentionFeeds.Unlock()
	}
	return ch, initial, cancel
}
