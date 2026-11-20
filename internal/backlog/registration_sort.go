package backlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Registration keys come only from explicit canonical registration reports.
// Lifecycle created_at can include file/observation fallbacks and is deliberately
// not used here. Fixed-width UTC preserves nanoseconds in the browser as well.
const registrationTimeLayout = "2006-01-02T15:04:05.000000000Z"

type registrationIndexEntry struct {
	revision string
	keys     map[string]string
	builds   uint64
}

var registrationIndexes = struct {
	sync.Mutex
	entries map[string]registrationIndexEntry
}{entries: map[string]registrationIndexEntry{}}

// Canonical event directories are append-only. Reuse their existing metadata
// change boundary, not task projections, runtime queries or lifecycle inference.
// Published maps are immutable; callers only read keys for their selected scope.
func registrationIndex(project string) (map[string]string, error) {
	project, err := filepath.Abs(project)
	if err != nil {
		return nil, err
	}
	revision := "absent"
	info, err := os.Stat(lifecycleEventsDir(project))
	if err == nil {
		revision = fmt.Sprintf("%d:%d", info.ModTime().UnixNano(), info.Size())
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	registrationIndexes.Lock()
	defer registrationIndexes.Unlock()
	old, exists := registrationIndexes.entries[project]
	if exists && old.revision == revision {
		return old.keys, nil
	}
	scan, err := ReadLifecycleTransitions(project)
	if err != nil {
		return nil, err
	}
	keys := map[string]string{}
	for _, event := range scan.Events {
		if event.Kind != "registered" {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, event.OccurredAt)
		if err != nil {
			continue
		}
		key := at.UTC().Format(registrationTimeLayout)
		// Preserve the original registration if a later report repeats it.
		if prior := keys[event.TaskID]; prior == "" || key < prior {
			keys[event.TaskID] = key
		}
	}
	// An externally created event directory may still be awaiting its atomic
	// event.json publication. Never freeze that interrupted read in the cache.
	for _, finding := range scan.Findings {
		if finding.Code == "incomplete_event" {
			return keys, nil
		}
	}
	registrationIndexes.entries[project] = registrationIndexEntry{revision: revision, keys: keys, builds: old.builds + 1}
	return keys, nil
}

func normalizedBacklogSort(key string) string {
	switch key {
	case "id_desc", "id_asc", "updated_desc", "updated_asc":
		return key
	default:
		return "recently_registered"
	}
}

// Apply to the whole filtered scope before taking a page. Unknown registrations
// are last; ties and unknown legacy items use exact ascending ID order.
func backlogRecordLess(left, right Record, sortKey string, registered map[string]string) bool {
	switch normalizedBacklogSort(sortKey) {
	case "updated_asc":
		if left.Mtime != right.Mtime {
			return left.Mtime < right.Mtime
		}
		if left.SortKey != right.SortKey {
			return left.SortKey < right.SortKey
		}
		return left.ID < right.ID
	case "updated_desc":
		if left.Mtime != right.Mtime {
			return left.Mtime > right.Mtime
		}
		if left.SortKey != right.SortKey {
			return left.SortKey > right.SortKey
		}
		return left.ID > right.ID
	case "id_asc":
		if left.SortKey != right.SortKey {
			return left.SortKey < right.SortKey
		}
		return left.ID < right.ID
	case "id_desc":
		if left.SortKey != right.SortKey {
			return left.SortKey > right.SortKey
		}
		return left.ID > right.ID
	default:
		a, b := registered[left.ID], registered[right.ID]
		if a != b {
			if a == "" {
				return false
			}
			if b == "" {
				return true
			}
			return a > b
		}
		return strings.Compare(left.ID, right.ID) < 0
	}
}

func addRegistrationKey(item map[string]any, registered map[string]string, id string) {
	if key := registered[id]; key != "" {
		item["registration_sort_key"] = key
	}
}
