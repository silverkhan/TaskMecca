package webui

import (
    "log"
    "os"
    "sync"
    "time"
)

// AID-246: opt-in, process-local diagnostics. No names, paths, task IDs or
// event contents are written to the logs. Disabled by default.
var monitorPerfEnabled = os.Getenv("TASK_MECCA_MONITOR_PERF") == "1"

type monitorPerfBucket struct {
    calls int64
    skipped int64
    changed int64
    items int64
    events int64
    elapsed time.Duration
    max time.Duration
}

var monitorPerfState = struct {
    sync.Mutex
    since time.Time
    buckets map[string]monitorPerfBucket
}{since: time.Now(), buckets: make(map[string]monitorPerfBucket)}

func recordMonitorPerf(kind string, elapsed time.Duration, items, events int, changed, skipped bool) {
    if !monitorPerfEnabled {
        return
    }
    monitorPerfState.Lock()
    defer monitorPerfState.Unlock()
    b := monitorPerfState.buckets[kind]
    b.calls++
    if skipped { b.skipped++ }
    if changed { b.changed++ }
    b.items += int64(items)
    b.events += int64(events)
    b.elapsed += elapsed
    if elapsed > b.max { b.max = elapsed }
    monitorPerfState.buckets[kind] = b
    now := time.Now()
    if now.Sub(monitorPerfState.since) < time.Minute {
        return
    }
    for category, sample := range monitorPerfState.buckets {
        avg := time.Duration(0)
        if sample.calls > 0 { avg = sample.elapsed / time.Duration(sample.calls) }
        log.Printf("Task Mecca monitor perf: kind=%s calls=%d skipped=%d changed=%d items=%d events=%d avg=%s max=%s total=%s window=%s",
            category, sample.calls, sample.skipped, sample.changed, sample.items, sample.events,
            avg, sample.max, sample.elapsed, now.Sub(monitorPerfState.since).Round(time.Second))
    }
    clear(monitorPerfState.buckets)
    monitorPerfState.since = now
}
