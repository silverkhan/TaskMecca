package webui

import (
	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"os"
	"path/filepath"
	"testing"
)

// Read/projection tests explicitly opt their synthetic project into monitoring.
// Each gets an isolated registry; no fixture can register into the user's home.
func registerWebFixture(t *testing.T, project string) {
	t.Helper()
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := maintenance.RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	// Registered last, so this runs before Setenv and either TempDir cleanup.
	// Handler owns durable attention writers even for unrelated API tests.
	t.Cleanup(func() {
		var stopping []*attentionFeed
		attentionFeeds.Lock()
		for key, feed := range attentionFeeds.feeds {
			if feed.project == project {
				stopping = append(stopping, feed)
				stopAttentionFeedLocked(key, feed)
			}
		}
		attentionFeeds.Unlock()
		// Do not hold the registry mutex while the worker completes refresh.
		for _, feed := range stopping {
			<-feed.doneCh
		}
	})
}

// Handler starts durable monitoring even when a test only exercises a different
// endpoint. Fixture teardown must join that writer before TempDir/registry cleanup.
func TestRegisteredWebFixtureJoinsAttentionFeedBeforeTempCleanup(t *testing.T) {
	var feed *attentionFeed
	var project, root string
	defer func() {
		if feed != nil {
			stopProjectAttentionFeeds(project)
			<-feed.doneCh
		}
	}()
	t.Run("handler owner", func(t *testing.T) {
		project = t.TempDir()
		root = filepath.Join(project, "_task_mecca", "data", "backlog")
		if err := os.MkdirAll(root, 0755); err != nil {
			t.Fatal(err)
		}
		registerWebFixture(t, project)
		if _, err := Handler(project, "", "0.3.6-dev.test"); err != nil {
			t.Fatal(err)
		}
		attentionFeeds.Lock()
		feed = attentionFeeds.feeds[project+"\x00"+root]
		attentionFeeds.Unlock()
		if feed == nil {
			t.Fatal("registered Handler did not start its durable attention feed")
		}
	})
	if feed == nil {
		t.Fatal("fixture did not produce a feed")
	}
	select {
	case <-feed.doneCh:
	default:
		t.Fatal("fixture returned before its background writer finished")
	}
	attentionFeeds.Lock()
	_, retained := attentionFeeds.feeds[project+"\x00"+root]
	attentionFeeds.Unlock()
	if retained {
		t.Fatal("fixture feed retained after owner teardown")
	}
}
