package webui

import (
	"testing"
	"time"
)

func TestAttentionFeedUnchangedRefreshDoesNotRebroadcast(t *testing.T) {
	project := t.TempDir()
	registerWebFixture(t, project)
	feed := &attentionFeed{project: project, subscribers: map[chan []byte]struct{}{}}
	feed.refresh()
	ch, initial, cancel := feed.subscribe()
	defer cancel()
	if len(initial) == 0 {
		t.Fatal("subscriber must receive current snapshot on connect")
	}

	feed.refresh()
	select {
	case <-ch:
		t.Fatal("unchanged attention revision must not rebroadcast after refresh/reconnect baseline")
	case <-time.After(100 * time.Millisecond):
	}
}
