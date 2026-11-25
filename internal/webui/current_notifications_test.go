package webui

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/silverkhan/TaskMecca/internal/backlog"
)

func TestCurrentNotificationsExistingRefreshPublishesCanonicalDone(t *testing.T) {
	project := t.TempDir()
	registerWebFixture(t, project)
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatal(err)
	}
	doing := filepath.Join(folder, "000001.A-1.current.hold.md")
	if err := os.WriteFile(doing, []byte("# A-1 Current\n"), 0644); err != nil {
		t.Fatal(err)
	}
	feed := &attentionFeed{project: project, root: folder, subscribers: map[chan []byte]struct{}{}}
	feed.refresh()
	ch, initial, cancel := feed.subscribe()
	defer cancel()
	var before map[string]any
	if err := json.Unmarshal(initial, &before); err != nil {
		t.Fatal(err)
	}
	if before["current_task_states"].(map[string]any)["A-1"] != "hold" {
		t.Fatalf("initial=%s", initial)
	}
	if err := os.Rename(doing, filepath.Join(folder, "000001.A-1.current.done.md")); err != nil {
		t.Fatal(err)
	}
	feed.refresh()
	select {
	case data := <-ch:
		var after map[string]any
		if err := json.Unmarshal(data, &after); err != nil {
			t.Fatal(err)
		}
		if after["current_task_states"].(map[string]any)["A-1"] != "done" {
			t.Fatalf("refreshed=%s", data)
		}
	default:
		t.Fatal("existing refresh did not publish canonical completion")
	}
}

func TestCurrentNotificationsRevisionPreservesDeliveryEvents(t *testing.T) {
	events := []map[string]any{{"id": "completed", "task_id": "A-1", "kind": "completed"}, {"id": "old-started", "task_id": "A-1", "kind": "started"}, {"id": "unknown", "task_id": "missing", "kind": "intervention"}, {"id": "system", "kind": "system"}}
	raw, _ := json.Marshal(events)
	payload := map[string]any{"notification_events": events, "current_task_states": map[string]string{"A-1": "doing"}}
	before := attentionRevision(payload)
	payload["current_task_states"] = map[string]string{"A-1": "done"}
	after := attentionRevision(payload)
	if before == after {
		t.Fatal("unchanged event IDs concealed canonical completion")
	}
	got, _ := json.Marshal(payload["notification_events"])
	if !bytes.Equal(raw, got) {
		t.Fatal("current revision changed delivery event data")
	}
	// The actual backlog projection retains completed transport events as well.
	project := t.TempDir()
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "000001.A-1.done.done.md"), []byte("# A-1 Done\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runtime := filepath.Join(project, "_task_mecca", ".runtime")
	if err := os.MkdirAll(runtime, 0755); err != nil {
		t.Fatal(err)
	}
	journal, _ := json.Marshal(map[string]any{"version": 4, "items": map[string]any{}, "phases": map[string]int{}, "events": events})
	path := filepath.Join(runtime, "notification_events.json")
	if err := os.WriteFile(path, journal, 0644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := backlog.AttentionSnapshot(project, folder, false)
	if err != nil {
		t.Fatal(err)
	}
	got, _ = json.Marshal(snapshot["notification_events"])
	if !bytes.Equal(raw, got) {
		t.Fatalf("canonical done removed transport events: %s", got)
	}
	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(journal, persisted) {
		t.Fatal("read projection rewrote history journal")
	}
}
