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
	// Register after TempDir/environment cleanup so the fixture's feed is
	// stopped and joined before its registry or project directory is removed.
	stopProjectAttentionFeedsForTest(t, project)
}

func TestRegisteredWebFixtureOwnsAttentionFeedTeardown(t *testing.T) {
	var project string
	var feed *attentionFeed
	t.Run("registered project", func(t *testing.T) {
		project = t.TempDir()
		registerWebFixture(t, project)
		feed = ensureDurableAttentionFeed(project, "")
	})
	// Subtest cleanups have completed: stopping without joining the refresh
	// would still permit writes racing with TempDir removal.
	select {
	case <-feed.doneCh:
	default:
		t.Fatal("fixture returned before its attention feed stopped")
	}
	attentionFeeds.Lock()
	_, retained := attentionFeeds.feeds[project+"\x00"]
	attentionFeeds.Unlock()
	if retained {
		t.Fatal("fixture retained a background feed")
	}
	if _, err := os.Stat(project); !os.IsNotExist(err) {
		t.Fatalf("fixture project not removed after feed shutdown: %v", err)
	}
}
