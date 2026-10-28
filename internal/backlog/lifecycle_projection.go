package backlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/projectionchange"
)

const lifecycleProjectionCheckInterval = time.Second
const lifecycleProjectionRebuildInterval = 30 * time.Second

// A snapshot contains only canonical Control Tower output. Reads do not scan
// files, reconcile evidence, acquire monitoring locks or write any ledger.
// Its version is process-local: a restart always rebuilds from durable inputs.
type lifecycleProjectionEntry struct {
	items                              map[string][]byte
	computedAt, validatedAt, checkedAt time.Time
	revision, version                  string
	generation                         uint64
	refreshing                         bool
	dirty                              bool
	err                                string
	timings                            map[string]float64
	builds, scans                      uint64
}

var lifecycleProjections = struct {
	sync.Mutex
	entries map[string]*lifecycleProjectionEntry
}{entries: map[string]*lifecycleProjectionEntry{}}

func lifecycleProjectionKey(project, root string) string {
	project, _ = filepath.Abs(project)
	if root == "" {
		root = "@default"
	} else if !filepath.IsAbs(root) {
		root = filepath.Join(project, root)
	}
	return filepath.Clean(project) + "\x00" + filepath.Clean(root)
}
func lifecycleProjectionEntryFor(key string) *lifecycleProjectionEntry {
	e := lifecycleProjections.entries[key]
	if e == nil {
		e = &lifecycleProjectionEntry{}
		lifecycleProjections.entries[key] = e
	}
	return e
}

// publishLifecycleProjection is called ONLY by the canonical reconciliation
// engine. Consumers never duplicate its runtime/attention/review decisions.
func publishLifecycleProjection(project, root string, rows []Record, control controlTowerSnapshot) {
	if control.Err != nil {
		return
	}
	items := make(map[string][]byte, len(rows))
	readyIDs, blocked := webWorkflowStateMaps(project, root, rows)
	for id, row := range preferredRows(rows) {
		state := row.State
		if row.State == "todo" {
			if readyIDs[id] {
				state = "ready"
			}
			if _, exists := blocked[id]; exists {
				state = "blocked"
			}
		}
		item := map[string]any{"id": id, "lifecycle": control.Timings[id], "activity": control.Activity[id], "state": effectiveStateFromControl(state, control.Attention[id]), "file_state": row.State}
		if item["lifecycle"] == nil {
			item["lifecycle"] = map[string]any{}
		}
		if item["activity"] == nil {
			item["activity"] = map[string]any{"health": "n/a"}
		}
		if len(control.Attention[id]) > 0 {
			item["attention_reason"] = control.Attention[id]
		}
		data, err := json.Marshal(item)
		if err != nil {
			return
		}
		items[id] = data
	}
	now := time.Now().UTC()
	lifecycleProjections.Lock()
	defer lifecycleProjections.Unlock()
	e := lifecycleProjectionEntryFor(lifecycleProjectionKey(project, root))
	e.timings = control.Profile
	e.items = items
	e.computedAt = now
	e.validatedAt = time.Time{}
	e.generation = projectionchange.Version(project)
	e.err = ""
	e.dirty = true
	e.builds++
	e.version = fmt.Sprintf("%d-%d", now.UnixNano(), e.builds)
}

// ReadLifecycleProjection has work bounded by one selected task's payload,
// regardless of total backlog/runtime/handoff history. Missing, stale and failed
// snapshots are explicit; no filename is promoted to a started/completed fact.
func ReadLifecycleProjection(project, root, id string) (map[string]any, bool) {
	now := time.Now().UTC()
	generation := projectionchange.Version(project)
	lifecycleProjections.Lock()
	e := lifecycleProjectionEntryFor(lifecycleProjectionKey(project, root))
	data, found := e.items[strings.ToUpper(strings.TrimSpace(id))]
	trust := "verified"
	if e.err != "" {
		trust = "error"
	} else if e.items == nil {
		trust = "pending"
	} else if e.dirty || e.generation != generation || now.Sub(e.validatedAt) > lifecycleProjectionRebuildInterval || now.Sub(e.computedAt) >= lifecycleProjectionRebuildInterval {
		trust = "stale"
	}
	meta := map[string]any{"trust": trust, "version": e.version, "computed_at": nil, "validated_at": nil, "source_revision": e.revision, "refreshing": e.refreshing, "age_ms": now.Sub(e.computedAt).Milliseconds(), "check_interval_ms": lifecycleProjectionCheckInterval.Milliseconds(), "rebuild_interval_ms": lifecycleProjectionRebuildInterval.Milliseconds(), "refresh_timings_ms": e.timings}
	if e.computedAt.IsZero() {
		meta["age_ms"] = nil
	}
	if !e.computedAt.IsZero() {
		meta["computed_at"] = e.computedAt.Format(time.RFC3339Nano)
	}
	if !e.validatedAt.IsZero() {
		meta["validated_at"] = e.validatedAt.Format(time.RFC3339Nano)
	}
	if e.err != "" {
		meta["diagnostic"] = "라이프사이클 근거를 다시 확인하지 못했습니다. 마지막 확인 결과를 표시합니다."
	}
	if e.timings != nil {
		timings := map[string]float64{}
		for k, v := range e.timings {
			timings[k] = v
		}
		meta["refresh_timings_ms"] = timings
	}
	available := e.items != nil
	lifecycleProjections.Unlock()
	item := map[string]any{"id": strings.ToUpper(strings.TrimSpace(id)), "lifecycle": map[string]any{}, "activity": map[string]any{"health": "runtime_unknown"}}
	if found {
		if err := json.Unmarshal(data, &item); err != nil {
			meta["trust"] = "error"
			available = false
		}
	}
	item["snapshot"] = meta
	if available && !found {
		item["not_found"] = true
	}
	return item, available
}

// ScheduleLifecycleProjectionRefresh coalesces readers. All I/O and canonical
// reconciliation run in a separate control job under the caller's monitoring
// guard. A slow/paused project never blocks snapshot reads or another project.
func ScheduleLifecycleProjectionRefresh(project, root string, guard func(func()) bool) {
	key := lifecycleProjectionKey(project, root)
	now := time.Now()
	lifecycleProjections.Lock()
	e := lifecycleProjectionEntryFor(key)
	changed := e.dirty || e.generation != projectionchange.Version(project)
	if e.refreshing || (!changed && now.Sub(e.checkedAt) < lifecycleProjectionCheckInterval) {
		lifecycleProjections.Unlock()
		return
	}
	e.refreshing = true
	e.checkedAt = now
	lifecycleProjections.Unlock()
	go func() {
		queuedAt := time.Now()
		allowed := guard(func() {
			waitMS := time.Since(queuedAt).Seconds() * 1000
			_ = RefreshLifecycleProjection(project, root, false)
			lifecycleProjections.Lock()
			e := lifecycleProjectionEntryFor(key)
			if e.timings == nil {
				e.timings = map[string]float64{}
			}
			e.timings["monitor_wait"] = waitMS
			lifecycleProjections.Unlock()
		})
		lifecycleProjections.Lock()
		defer lifecycleProjections.Unlock()
		e := lifecycleProjectionEntryFor(key)
		e.refreshing = false
		if !allowed {
			e.err = "monitoring unavailable"
		}
	}()
}

// RefreshLifecycleProjection is a control-plane operation, not a read API.
// Metadata scans recover cross-process/missed events; periodic forced rebuilds
// also recover edits preserving mtime/size. No persistent snapshot is trusted
// after restart. Atomic publication and before/after checks handle racing input.
func RefreshLifecycleProjection(project, root string, force bool) error {
	started := time.Now()
	key := lifecycleProjectionKey(project, root)
	before, err := lifecycleProjectionSourceRevision(project, root)
	if err != nil {
		return lifecycleProjectionFailure(key, err)
	}
	generation := projectionchange.Version(project)
	lifecycleProjections.Lock()
	e := lifecycleProjectionEntryFor(key)
	e.scans++
	fullRecovery := force || e.items == nil || time.Since(e.computedAt) >= lifecycleProjectionRebuildInterval
	rebuild := fullRecovery || e.dirty || e.err != "" || before != e.revision || generation != e.generation
	lifecycleProjections.Unlock()
	if rebuild {
		catalogStarted := time.Now()
		if fullRecovery {
			folder, selectErr := Select(project, root)
			if selectErr != nil {
				return lifecycleProjectionFailure(key, selectErr)
			}
			if folder != "" {
				InvalidateCachedCatalog(folder)
			}
		}
		rows, readErr := CachedCatalog(project, root)
		if readErr != nil {
			return lifecycleProjectionFailure(key, readErr)
		}
		catalogMS := time.Since(catalogStarted).Seconds() * 1000
		controlStarted := time.Now()
		// Corrupt handoff input must not be concealed by a successful cached read.
		h, hErr := handoffjournal.BuildLedger(project, time.Now())
		if hErr != nil {
			return lifecycleProjectionFailure(key, hErr)
		}
		if len(h.Findings) > 0 {
			return lifecycleProjectionFailure(key, fmt.Errorf("handoff evidence has %d findings", len(h.Findings)))
		}
		control := reconcileControlTower(project, root, rows)
		if control.Err != nil {
			return lifecycleProjectionFailure(key, control.Err)
		}
		lifecycleProjections.Lock()
		e := lifecycleProjectionEntryFor(key)
		e.timings["catalog"] = catalogMS
		e.timings["control"] = time.Since(controlStarted).Seconds() * 1000
		lifecycleProjections.Unlock()
	}
	after, err := lifecycleProjectionSourceRevision(project, root)
	if err != nil {
		return lifecycleProjectionFailure(key, err)
	}
	lifecycleProjections.Lock()
	defer lifecycleProjections.Unlock()
	e = lifecycleProjectionEntryFor(key)
	e.scans++
	e.checkedAt = time.Now().UTC()
	if e.timings == nil {
		e.timings = map[string]float64{}
	}
	e.timings["total"] = time.Since(started).Seconds() * 1000
	e.revision = after
	if before != after || generation != projectionchange.Version(project) {
		// Reconciliation may itself recover/write evidence. Recheck on the next
		// control pass instead of claiming that a concurrent snapshot is current.
		e.validatedAt = time.Time{}
		e.dirty = true
		e.generation = generation
	} else {
		e.validatedAt = time.Now().UTC()
		e.dirty = false
		e.generation = generation
		e.err = ""
	}
	return nil
}
func lifecycleProjectionFailure(key string, err error) error {
	lifecycleProjections.Lock()
	e := lifecycleProjectionEntryFor(key)
	e.err = err.Error()
	e.checkedAt = time.Now().UTC()
	lifecycleProjections.Unlock()
	return err
}

func lifecycleProjectionSourceRevision(project, root string) (string, error) {
	folder, err := Select(project, root)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	// Input collection belongs to the control path. Include every source used by
	// canonical lifecycle/runtime/review decisions, never unrelated project data.
	paths := []string{folder, filepath.Join(project, "_task_mecca", "data", "lifecycle"), filepath.Join(project, "_task_mecca", ".runtime", "executions"), filepath.Join(project, "_task_mecca", "data", "assignments"), filepath.Join(project, "_task_mecca", ".runtime", "handoffs"), filepath.Join(project, "_task_mecca", ".runtime", "agents"), filepath.Join(project, "_task_mecca", ".runtime", "lifecycle_observations.json")}
	for _, path := range paths {
		if path == "" {
			continue
		}
		walkErr := filepath.WalkDir(path, func(path string, d fs.DirEntry, walkErr error) error {
			if os.IsNotExist(walkErr) {
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			if strings.HasSuffix(d.Name(), ".lock") || strings.HasSuffix(d.Name(), ".tmp") {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			fmt.Fprintf(h, "%s\x00%d\x00%d\x00%d\n", path, info.Size(), info.ModTime().UnixNano(), info.Mode())
			return nil
		})
		if walkErr != nil {
			return "", walkErr
		}
	}
	// HEAD includes worktree/symbolic refs and packed refs without assuming .git
	// is a directory. This command is outside the read request and history replay.
	fmt.Fprint(h, gitOutput(project, "rev-parse", "HEAD"))
	return hex.EncodeToString(h.Sum(nil)), nil
}
