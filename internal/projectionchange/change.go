// Package projectionchange carries process-local invalidations, never evidence.
// Other processes and missed events are recovered by control-plane source scans.
package projectionchange

import (
	"path/filepath"
	"sync"
)

var revisions = struct {
	sync.Mutex
	values map[string]uint64
}{values: map[string]uint64{}}

func key(project string) string {
	if p, err := filepath.Abs(project); err == nil {
		return filepath.Clean(p)
	}
	return filepath.Clean(project)
}
func Mark(project string) { revisions.Lock(); revisions.values[key(project)]++; revisions.Unlock() }
func Version(project string) uint64 {
	revisions.Lock()
	defer revisions.Unlock()
	return revisions.values[key(project)]
}
