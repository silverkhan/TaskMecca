package webui

import (
	"encoding/json"
	"fmt"
	"sort"
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
	doneCh      chan struct{}
	mu          sync.Mutex
	latest      []byte
	revision    string
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

func (f *attentionFeed) refresh() {
	maintenance.WithProjectMonitoring(f.project, f.refreshActive)
}

func (f *attentionFeed) refreshActive() {
	payload, err := backlog.AttentionSnapshot(f.project, f.root, true)
	if err != nil {
		payload = map[string]any{"error": err.Error(), "attention": []map[string]any{}, "all_items": map[string]map[string]any{}, "notification_events": []map[string]any{}}
	}
	revision := attentionRevision(payload)
	if rows, ok := payload["notification_events"].([]map[string]any); ok {
		events := make([]notify.Event, 0, len(rows))
		for _, row := range rows {
			events = append(events, notify.Event{
				ID: notificationValue(row["id"]), TaskID: notificationValue(row["task_id"]), Kind: notificationValue(row["kind"]),
				Title: notificationValue(row["title"]), Message: notificationValue(row["message"]),
				ResumeCondition: notificationValue(row["resume_condition"]), At: notificationValue(row["at"]),
			})
		}
		_ = notify.Deliver(f.project, events)
	}
	data, _ := json.Marshal(payload)
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
	if changed {
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
	f.mu.Lock()
	f.subscribers[ch] = struct{}{}
	initial := append([]byte{}, f.latest...)
	f.mu.Unlock()
	cancel := func() {
		f.mu.Lock()
		if _, ok := f.subscribers[ch]; ok {
			delete(f.subscribers, ch)
			close(ch)
		}
		f.mu.Unlock()
	}
	return ch, initial, cancel
}
