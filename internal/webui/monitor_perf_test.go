package webui

import (
    "encoding/json"
    "strings"
    "testing"
    "time"
)

func TestMonitorPerfSnapshotAggregateAndSafeFields(t *testing.T) {
    // An isolated in-memory fixture: no monitor calls, filesystem scans or
    // notification delivery are needed.
    monitorPerfState.Lock()
    oldSince := monitorPerfState.since
    oldStarted := monitorPerfState.started
    oldBuckets := monitorPerfState.buckets
    oldHistory := monitorPerfState.history
    monitorPerfState.since = time.Now()
    monitorPerfState.started = time.Now()
    monitorPerfState.buckets = map[string]monitorPerfBucket{
        "attention_refresh": {Calls: 2, Changed: 1, Items: 5, TotalMS: 12.5, MaxMS: 8.0},
    }
    monitorPerfState.history = nil
    monitorPerfState.Unlock()
    defer func() {
        monitorPerfState.Lock()
        monitorPerfState.since = oldSince
        monitorPerfState.started = oldStarted
        monitorPerfState.buckets = oldBuckets
        monitorPerfState.history = oldHistory
        monitorPerfState.Unlock()
    }()
    snapshot := monitorPerfSnapshot("0.3.6-dev.test")
    data, err := json.Marshal(snapshot)
    if err != nil { t.Fatal(err) }
    if !strings.Contains(string(data), "\"attention_refresh\"") { t.Fatalf("missing metric: %s",data) }
    if !strings.Contains(string(data), "\"active_attention_feeds\"") { t.Fatalf("missing feed count: %s",data) }
    if strings.Contains(string(data), "project_path") || strings.Contains(string(data), "task_id") {
        t.Fatalf("monitor report unexpectedly includes identity fields")
    }
}

func TestMonitorPerfBoundedHistory(t *testing.T) {
    monitorPerfState.Lock()
    oldSince, oldBuckets, oldHistory := monitorPerfState.since, monitorPerfState.buckets, monitorPerfState.history
    monitorPerfState.history = make([]monitorPerfWindow, monitorPerfHistoryLimit)
    monitorPerfState.since = time.Now().Add(-time.Minute * 2)
    monitorPerfState.buckets = map[string]monitorPerfBucket{"operation_scan":{Calls:1}}
    flushMonitorPerfLocked(time.Now())
    count := len(monitorPerfState.history)
    monitorPerfState.since, monitorPerfState.buckets, monitorPerfState.history = oldSince, oldBuckets, oldHistory
    monitorPerfState.Unlock()
    if count != monitorPerfHistoryLimit { t.Fatalf("history retained %d items; want %d",count,monitorPerfHistoryLimit) }
}
