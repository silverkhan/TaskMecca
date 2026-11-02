package webui

import (
    "log"
    "os"
    "sort"
    "sync"
    "sync/atomic"
    "time"
)

// Diagnostic collection is opt-in for stable, but automatically enabled for
// Dev at Web startup. No project names, task IDs, paths or event contents are
// retained. All counters are aggregates in bounded process memory.
var monitorPerfEnabled atomic.Bool
var monitorPerfVerbose = os.Getenv("TASK_MECCA_MONITOR_PERF") == "1"

func init() {
    monitorPerfEnabled.Store(monitorPerfVerbose)
}

func enableDevMonitorPerf() {
    monitorPerfEnabled.Store(true)
}

type monitorPerfBucket struct {
    Calls   int64 `json:"calls"`
    Skipped int64 `json:"skipped"`
    Changed int64 `json:"changed"`
    Items   int64 `json:"items"`
    Events  int64 `json:"events"`
    TotalMS float64 `json:"total_ms"`
    MaxMS   float64 `json:"max_ms"`
}

type monitorPerfWindow struct {
    From string `json:"from"`
    To string `json:"to"`
    Metrics map[string]monitorPerfBucket `json:"metrics"`
}

var monitorPerfState = struct {
    sync.Mutex
    started time.Time
    since time.Time
    buckets map[string]monitorPerfBucket
    history []monitorPerfWindow
}{started: time.Now(), since: time.Now(), buckets: make(map[string]monitorPerfBucket)}

const monitorPerfHistoryLimit = 15

func flushMonitorPerfLocked(now time.Time) {
    if now.Sub(monitorPerfState.since) < time.Minute {
        return
    }
    if len(monitorPerfState.buckets) > 0 {
        copied := make(map[string]monitorPerfBucket, len(monitorPerfState.buckets))
        for kind, bucket := range monitorPerfState.buckets { copied[kind] = bucket }
        monitorPerfState.history = append(monitorPerfState.history, monitorPerfWindow{
            From: monitorPerfState.since.UTC().Format(time.RFC3339),
            To: now.UTC().Format(time.RFC3339),
            Metrics: copied,
        })
        if len(monitorPerfState.history) > monitorPerfHistoryLimit {
            monitorPerfState.history = append([]monitorPerfWindow(nil), monitorPerfState.history[len(monitorPerfState.history)-monitorPerfHistoryLimit:]...)
        }
        if monitorPerfVerbose {
            kinds := make([]string, 0, len(copied))
            for kind := range copied { kinds = append(kinds, kind) }
            sort.Strings(kinds)
            for _, kind := range kinds {
                b := copied[kind]
                log.Printf("Task Mecca monitor perf: kind=%s calls=%d skipped=%d changed=%d observed_items=%d events=%d total_ms=%.1f max_ms=%.1f",
                    kind, b.Calls, b.Skipped, b.Changed, b.Items, b.Events, b.TotalMS, b.MaxMS)
            }
        }
    }
    monitorPerfState.buckets = make(map[string]monitorPerfBucket)
    monitorPerfState.since = now
}

func recordMonitorPerf(kind string, elapsed time.Duration, items, events int, changed, skipped bool) {
    if !monitorPerfEnabled.Load() {
        return
    }
    now := time.Now()
    monitorPerfState.Lock()
    defer monitorPerfState.Unlock()
    flushMonitorPerfLocked(now)
    b := monitorPerfState.buckets[kind]
    b.Calls++
    if skipped { b.Skipped++ }
    if changed { b.Changed++ }
    b.Items += int64(items)
    b.Events += int64(events)
    ms := float64(elapsed) / float64(time.Millisecond)
    b.TotalMS += ms
    if ms > b.MaxMS { b.MaxMS = ms }
    monitorPerfState.buckets[kind] = b
}

// Snapshot does no filesystem or monitoring work, and is safe during active
// notification delivery. The API exports only aggregate statistics.
func monitorPerfSnapshot(version string) map[string]any {
    now := time.Now()
    monitorPerfState.Lock()
    defer monitorPerfState.Unlock()
    flushMonitorPerfLocked(now)
    history := append([]monitorPerfWindow{}, monitorPerfState.history...)
    current := make(map[string]monitorPerfBucket,len(monitorPerfState.buckets))
    for k,v := range monitorPerfState.buckets { current[k] = v }
    return map[string]any{
        "schema": 1,
        "version": version,
        "captured_at": now.UTC().Format(time.RFC3339),
        "started_at": monitorPerfState.started.UTC().Format(time.RFC3339),
        "enabled": monitorPerfEnabled.Load(),
        "window_seconds": 60,
        "measurement": "wall_clock_not_cpu",
        "observed_items_definition": "attention rows or operation stages, not full catalog size",
        "current": monitorPerfWindow{From: monitorPerfState.since.UTC().Format(time.RFC3339), To: now.UTC().Format(time.RFC3339), Metrics: current},
        "history": history,
    }
}
