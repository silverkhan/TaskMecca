package webui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Bounded, process-local read cache. No ledger or settings are written.
type notificationHistoryCache struct {
	mu       sync.Mutex
	projects map[string]*notificationHistoryVersions
}
type notificationHistoryVersions struct {
	fingerprint string
	current     string
	versions    map[string][]map[string]any
	order       []string
}

func notificationHistoryFingerprint(project string) (string, error) {
	result := ""
	for _, name := range []string{"notification_events.json", "notifications/delivery_ledger.json"} {
		info, err := os.Stat(filepath.Join(project, "_task_mecca", ".runtime", name))
		if os.IsNotExist(err) {
			result += name + ":missing;"
			continue
		}
		if err != nil {
			return "", err
		}
		result += fmt.Sprintf("%s:%d:%d:%v;", name, info.Size(), info.ModTime().UnixNano(), info.Mode())
	}
	return result, nil
}
func historyRowJSON(row map[string]any) string { data, _ := json.Marshal(row); return string(data) }
func historyEventID(row map[string]any) string { id, _ := row["event_id"].(string); return id }
func (cache *notificationHistoryCache) read(project, since string) (map[string]any, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.projects == nil {
		cache.projects = map[string]*notificationHistoryVersions{}
	}
	fingerprint, err := notificationHistoryFingerprint(project)
	if err != nil {
		return nil, err
	}
	entry := cache.projects[project]
	if entry == nil || entry.fingerprint != fingerprint {
		rows, err := notificationHistory(project)
		if err != nil {
			return nil, err
		}
		serialized, _ := json.Marshal(rows)
		hash := sha256.Sum256(serialized)
		revision := hex.EncodeToString(hash[:])
		if entry == nil {
			if len(cache.projects) >= 128 {
				cache.projects = map[string]*notificationHistoryVersions{}
			}
			entry = &notificationHistoryVersions{versions: map[string][]map[string]any{}}
			cache.projects[project] = entry
		}
		entry.fingerprint = fingerprint
		entry.current = revision
		if _, ok := entry.versions[revision]; !ok {
			entry.versions[revision] = rows
			entry.order = append(entry.order, revision)
			if len(entry.order) > 8 {
				delete(entry.versions, entry.order[0])
				entry.order = entry.order[1:]
			}
		}
	}
	current := entry.versions[entry.current]
	if since == entry.current {
		return map[string]any{"revision": entry.current, "unchanged": true}, nil
	}
	previous, known := entry.versions[since]
	if since == "" || !known {
		return map[string]any{"revision": entry.current, "history": current, "reset": true}, nil
	}
	old := map[string]string{}
	for _, row := range previous {
		old[historyEventID(row)] = historyRowJSON(row)
	}
	changed := []map[string]any{}
	for _, row := range current {
		id := historyEventID(row)
		if old[id] != historyRowJSON(row) {
			changed = append(changed, row)
		}
		delete(old, id)
	}
	removed := []string{}
	for id := range old {
		removed = append(removed, id)
	}
	return map[string]any{"revision": entry.current, "history": changed, "removed": removed, "reset": false}, nil
}
