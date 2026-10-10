package webui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHistoryDeltaUnchangedUpdateDeletionCorruptionAndExpiredBase(t *testing.T) {
	project := t.TempDir()
	directory := filepath.Join(project, "_task_mecca", ".runtime")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "notification_events.json")
	write := func(ids ...string) {
		events := []map[string]any{}
		for _, id := range ids {
			events = append(events, map[string]any{"id": id, "task_id": id, "kind": "started", "at": "2026-10-08T00:00:00Z"})
		}
		body, _ := json.Marshal(map[string]any{"version": 4, "events": events})
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cache := &notificationHistoryCache{}
	write("same-b", "same-a")
	first, err := cache.read(project, "")
	if err != nil {
		t.Fatal(err)
	}
	revision := first["revision"].(string)
	if len(first["history"].([]map[string]any)) != 2 {
		t.Fatal(first)
	}
	unchanged, err := cache.read(project, revision)
	if err != nil || unchanged["unchanged"] != true || unchanged["history"] != nil {
		t.Fatalf("unchanged %v %v", unchanged, err)
	}
	write("same-b", "same-a", "new")
	delta, err := cache.read(project, revision)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(first)
	unchangedJSON, _ := json.Marshal(unchanged)
	deltaJSON, _ := json.Marshal(delta)
	t.Logf("initial_rows=2 initial_bytes=%d unchanged_rows=0 unchanged_bytes=%d delta_rows=1 delta_bytes=%d", len(firstJSON), len(unchangedJSON), len(deltaJSON))
	if delta["reset"] != false || len(delta["history"].([]map[string]any)) != 1 {
		t.Fatal(delta)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	deleted, err := cache.read(project, delta["revision"].(string))
	if err != nil || len(deleted["removed"].([]string)) != 3 {
		t.Fatalf("deletion %v %v", deleted, err)
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.read(project, deleted["revision"].(string)); err == nil {
		t.Fatal("corruption treated as success")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.read(project, deleted["revision"].(string)); err == nil {
		t.Fatal("read failure treated as unchanged")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	write("restored")
	reset, err := cache.read(project, "expired-unknown")
	if err != nil || reset["reset"] != true {
		t.Fatalf("reset %v %v", reset, err)
	}
}
