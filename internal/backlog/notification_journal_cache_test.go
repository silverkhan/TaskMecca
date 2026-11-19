package backlog

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestNotificationJournalCacheInvalidateOnExternalAtomicReplacement(t *testing.T) {
	project := t.TempDir()
	items := map[string]map[string]any{"A-7": {"id": "A-7", "title": "Task", "file_state": "doing", "updated_at": "2026-11-04T00:00:00Z"}}
	if _, err := NotificationEvents(project, items); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "_task_mecca", ".runtime", "notification_events.json")
	notificationJournalMu.Lock()
	_, hit := cachedNotificationJournal(path)
	notificationJournalMu.Unlock()
	if !hit {
		t.Fatal("unchanged on-disk journal should hit cache")
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var updated notificationJournal
	if err = json.Unmarshal(raw, &updated); err != nil {
		t.Fatal(err)
	}
	updated.Events = append(updated.Events, map[string]any{"id": "external-event", "task_id": "A-7", "kind": "registered"})
	contents, err := json.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}
	replacement := path + ".external"
	if err = os.WriteFile(replacement, contents, 0644); err != nil {
		t.Fatal(err)
	}
	// Make the replacement deliberately look like the old file in mtime.
	if err = os.Chtimes(replacement, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	notificationJournalMu.Lock()
	_, hit = cachedNotificationJournal(path)
	notificationJournalMu.Unlock()
	if hit {
		t.Fatal("atomic external replacement must invalidate source identity")
	}
	events, err := ReadNotificationEvents(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0]["id"] != "external-event" {
		t.Fatalf("external event invisible: %#v", events)
	}
}

func TestNotificationJournalCacheIsolationAndCommit(t *testing.T) {
	project := t.TempDir()
	doing := map[string]map[string]any{"A-11": {"id": "A-11", "title": "Fresh", "file_state": "doing", "updated_at": "2026-11-04T00:00:00Z"}}
	if _, err := NotificationEvents(project, doing); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "_task_mecca", ".runtime", "notification_events.json")
	notificationJournalMu.Lock()
	cached, hit := cachedNotificationJournal(path)
	if !hit {
		notificationJournalMu.Unlock()
		t.Fatal("expected first cache hit")
	}
	cached.Items["A-11"] = notificationObservation{FileState: "done"}
	cached.Phases["A-11"] = 40
	cached.Events = append(cached.Events, map[string]any{"id": "injected"})
	notificationJournalMu.Unlock()
	done := map[string]map[string]any{"A-11": {"id": "A-11", "title": "Fresh", "file_state": "done", "updated_at": "2026-11-04T00:01:00Z", "completed_at": "2026-11-04T00:01:00Z"}}
	events, err := NotificationEvents(project, done)
	if err != nil || len(events) != 1 || events[0]["kind"] != "completed" {
		t.Fatalf("mutable cache leaked observation: %v %v", events, err)
	}
	notificationJournalMu.Lock()
	replacement, hit := cachedNotificationJournal(path)
	notificationJournalMu.Unlock()
	if !hit || replacement.Items["A-11"].FileState != "done" {
		t.Fatalf("new commit not published: hit=%v %#v", hit, replacement.Items)
	}
	again, err := NotificationEvents(project, done)
	if err != nil || len(again) != 1 || again[0]["id"] != events[0]["id"] {
		t.Fatalf("duplicate completion: %#v err=%v", again, err)
	}
}

func TestNotificationJournalCheckpointFallsBackToDisk(t *testing.T) {
	project := t.TempDir()
	items := map[string]map[string]any{"A-1": {"file_state": "doing", "updated_at": "2026-11-04T00:00:00Z"}}
	if _, err := NotificationEvents(project, items); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "_task_mecca", ".runtime", "notification_events.json")
	notificationJournalMu.Lock()
	entry := notificationJournalCache[path]
	entry.checked = time.Now().Add(-notificationJournalCheckpoint - time.Second)
	notificationJournalCache[path] = entry
	_, hit := cachedNotificationJournal(path)
	notificationJournalMu.Unlock()
	if hit {
		t.Fatal("checkpoint did not force fresh parse")
	}
	if _, err := NotificationEvents(project, items); err != nil {
		t.Fatal(err)
	}
	notificationJournalMu.Lock()
	_, hit = cachedNotificationJournal(path)
	notificationJournalMu.Unlock()
	if !hit {
		t.Fatal("checkpoint did not repopulate cache")
	}
}

func TestNotificationFastPathDoesNotHideLateStartOrConditionReset(t *testing.T) {
	project := t.TempDir()
	item := map[string]any{"id": "A-55", "title": "Active", "file_state": "doing", "updated_at": "2026-11-04T00:00:00Z"}
	rows := map[string]map[string]any{"A-55": item}
	if initial, err := NotificationEvents(project, rows); err != nil || len(initial) != 0 {
		t.Fatalf("baseline=%v err=%v", initial, err)
	}
	for i := 0; i < 3; i++ {
		if events, err := NotificationEvents(project, rows); err != nil || len(events) != 0 {
			t.Fatalf("unchanged repeat %d: %v %v", i, events, err)
		}
	}
	// A runtime-start observation can appear without a backlog file change.
	item["lifecycle"] = map[string]any{"started_at": "2026-11-04T00:00:30Z"}
	started, err := NotificationEvents(project, rows)
	if err != nil || len(started) != 1 || started[0]["kind"] != "started" {
		t.Fatalf("late Hook start invisible: %v %v", started, err)
	}
	// A first actionable episode must be emitted, then a cleared condition
	// may reappear with the same identifier as a separate notification episode.
	item["notification_condition"] = canonicalRequestConditionFixture("Owner access needed", "Owner: grant access")
	first, err := NotificationEvents(project, rows)
	if err != nil || len(first) != 2 {
		t.Fatalf("condition invisible: %v %v", first, err)
	}
	item["notification_condition"] = nil
	if _, err := NotificationEvents(project, rows); err != nil {
		t.Fatal(err)
	}
	item["notification_condition"] = canonicalRequestConditionFixture("Owner access needed", "Owner: grant access")
	again, err := NotificationEvents(project, rows)
	if err != nil || len(again) != 3 {
		t.Fatalf("new episode suppressed by consumed cache: %v %v", again, err)
	}
}

func TestNotificationJournalCacheFailsClosedForMalformedExternalEdit(t *testing.T) {
	project := t.TempDir()
	items := map[string]map[string]any{"A-44": {"id": "A-44", "file_state": "doing", "updated_at": "2026-11-04T00:00:00Z"}}
	if _, err := NotificationEvents(project, items); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "_task_mecca", ".runtime", "notification_events.json")
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// An external edit can replace the journal while this process is live.
	// Never return a successful stale read for invalid source evidence.
	if err := os.WriteFile(path, []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadNotificationEvents(project); err == nil {
		t.Fatal("invalid journal silently replaced by cached data")
	}
	if _, err := NotificationEvents(project, items); err == nil {
		t.Fatal("invalid journal silently reconciled")
	}
	restored := path + ".restore"
	if err := os.WriteFile(restored, saved, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(restored, path); err != nil {
		t.Fatal(err)
	}
	if _, err := NotificationEvents(project, items); err != nil {
		t.Fatalf("valid restored journal not accepted: %v", err)
	}
}

func TestA88Repair6SnapshotFactoryAndPublicationBinding(t *testing.T) {
	for _, kind := range []string{"same_inode_corrupt", "same_inode_valid", "atomic_valid", "whitespace", "truncated"} {
		t.Run(kind, func(t *testing.T) {
			p := t.TempDir()
			if _, err := NotificationEvents(p, map[string]map[string]any{}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(p, "_task_mecca/.runtime/notification_events.json")
			notificationJournalMu.Lock()
			snapshot, err := readNotificationJournalSnapshot(path, nil)
			notificationJournalMu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			original, _ := os.ReadFile(path)
			info, _ := os.Stat(path)
			changed := append([]byte(nil), original...)
			switch kind {
			case "same_inode_corrupt":
				changed[0] = '!'
			case "same_inode_valid", "atomic_valid":
				var j notificationJournal
				_ = json.Unmarshal(original, &j)
				j.Events = []map[string]any{{"id": "new-current", "task_id": "NEW", "kind": "registered"}}
				changed, _ = json.Marshal(j)
			case "whitespace":
				changed = []byte(" \n\t ")
			case "truncated":
				changed = []byte("{")
			}
			if kind == "atomic_valid" {
				if err = os.WriteFile(path+".replace", changed, 0600); err != nil {
					t.Fatal(err)
				}
				if err = os.Rename(path+".replace", path); err != nil {
					t.Fatal(err)
				}
			} else {
				if err = os.WriteFile(path, changed, 0600); err != nil {
					t.Fatal(err)
				}
				if err = os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			}
			// This is the same parse->publication boundary used by cold readers. It
			// must not attach fresh bytes/metadata to the earlier parsed snapshot.
			notificationJournalMu.Lock()
			err = publishNotificationJournalSnapshot(path, snapshot)
			_, published := notificationJournalCache[path]
			notificationJournalMu.Unlock()
			if err == nil || published {
				t.Fatal("changed snapshot published", err, published)
			}
			got, readErr := ReadNotificationEvents(p)
			var current notificationJournal
			parseErr := json.Unmarshal(changed, &current)
			if parseErr != nil {
				if readErr == nil || len(got) != 0 {
					t.Fatal("cold error hidden", got, readErr)
				}
			} else if readErr != nil || !reflect.DeepEqual(got, current.Events) {
				t.Fatal("valid replacement not read honestly", got, current.Events, readErr)
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(changed) {
				t.Fatal("raw evidence rewritten")
			}
		})
	}
}
func TestA88Repair6CacheFactoryOnlyPairsItsOwnParse(t *testing.T) {
	p := t.TempDir()
	if _, err := NotificationEvents(p, map[string]map[string]any{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(p, "_task_mecca/.runtime/notification_events.json")
	raw := []byte(`{"version":4,"items":{},"phases":{},"events":[{"id":"old","task_id":"A-88","kind":"registered","details":{"actors":[{"native":"original"}]}}]}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	first, err := ReadNotificationEvents(p)
	if err != nil || len(first) != 1 {
		t.Fatal(first, err)
	}
	first[0]["details"].(map[string]any)["actors"].([]any)[0].(map[string]any)["native"] = "caller-mutation"
	first[0]["kind"] = "intervention"
	again, err := ReadNotificationEvents(p)
	if err != nil || again[0]["kind"] != "registered" || again[0]["details"].(map[string]any)["actors"].([]any)[0].(map[string]any)["native"] != "original" {
		t.Fatal("caller mutated cached parse", again, err)
	}
	notificationJournalMu.Lock()
	entry, hit := notificationJournalCache[path]
	notificationJournalMu.Unlock()
	if !hit || entry.digest != sha256.Sum256(entry.raw) || string(entry.raw) != string(raw) {
		t.Fatal("cache raw/digest not bound")
	}
	var parsed notificationJournal
	if err = json.Unmarshal(entry.raw, &parsed); err != nil || !reflect.DeepEqual(parsed, entry.journal) {
		t.Fatal("cache parse not derived from same bytes", err)
	}
	// Once published, an external change must invalidate rather than return the
	// old nested parse, including same-inode edits with restored mtime.
	info, _ := os.Stat(path)
	corrupt := append([]byte(nil), raw...)
	corrupt[0] = '!'
	_ = os.WriteFile(path, corrupt, 0600)
	_ = os.Chtimes(path, info.ModTime(), info.ModTime())
	if got, err := ReadNotificationEvents(p); err == nil || len(got) != 0 {
		t.Fatal("after-publication corruption hidden", got, err)
	}
}
