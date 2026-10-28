package backlog

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/projectionchange"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func projectionFixture(t *testing.T, n int) (string, string) {
	t.Helper()
	project := t.TempDir()
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	for i := 1; i <= n; i++ {
		writeWebTask(t, folder, fmt.Sprintf("%06d.A-%d.task.todo.md", i, i), fmt.Sprintf("# A-%d Task\n- Agent: -\n", i))
	}
	return project, folder
}
func projectionRefresh(t *testing.T, project, folder string, force bool) {
	t.Helper()
	if err := RefreshLifecycleProjection(project, folder, force); err != nil {
		t.Fatal(err)
	}
	// First control calculation may itself recover lifecycle observations.
	if err := RefreshLifecycleProjection(project, folder, false); err != nil {
		t.Fatal(err)
	}
}
func projectionMeta(t *testing.T, item map[string]any) map[string]any {
	t.Helper()
	return item["snapshot"].(map[string]any)
}
func projectionCounts(project, folder string) (uint64, uint64) {
	lifecycleProjections.Lock()
	defer lifecycleProjections.Unlock()
	e := lifecycleProjectionEntryFor(lifecycleProjectionKey(project, folder))
	return e.builds, e.scans
}

func TestLifecycleProjectionReadIsImmutableAndDoesNoControlWork(t *testing.T) {
	project, folder := projectionFixture(t, 10)
	projectionRefresh(t, project, folder, true)
	before, scans := projectionCounts(project, folder)
	// Holding the catalog refresh lock proves reads do not visit the catalog.
	lock := catalogRefreshLock(folder)
	lock.Lock()
	defer lock.Unlock()
	for i := 0; i < 100; i++ {
		item, ok := ReadLifecycleProjection(project, folder, "A-1")
		if !ok || projectionMeta(t, item)["trust"] != "verified" {
			t.Fatalf("snapshot: %+v", item)
		}
		item["lifecycle"].(map[string]any)["events"] = "caller mutation"
	}
	after, afterScans := projectionCounts(project, folder)
	if before != after || scans != afterScans {
		t.Fatalf("read caused work: builds %d/%d scans %d/%d", before, after, scans, afterScans)
	}
	item, _ := ReadLifecycleProjection(project, folder, "A-1")
	if item["state"] != "ready" {
		t.Fatalf("canonical readiness lost: %+v", item)
	}
	if item["lifecycle"].(map[string]any)["events"] == "caller mutation" {
		t.Fatal("mutable snapshot escaped")
	}
}

func TestLifecycleProjectionDurableChangeMissedEventRestartAndIsolation(t *testing.T) {
	project, folder := projectionFixture(t, 1)
	other, otherFolder := projectionFixture(t, 1)
	projectionRefresh(t, project, folder, true)
	projectionRefresh(t, other, otherFolder, true)
	initial, _ := ReadLifecycleProjection(project, folder, "A-1")
	unrelated, _ := ReadLifecycleProjection(other, otherFolder, "A-1")
	event := LifecycleTransition{EventID: "started-test", TaskID: "A-1", Kind: "started", Actor: "/root/controller/kkobugi", EvidenceSource: "worker_report"}
	if _, err := RecordLifecycleTransition(project, event, time.Now()); err != nil {
		t.Fatal(err)
	}
	stale, _ := ReadLifecycleProjection(project, folder, "A-1")
	if projectionMeta(t, stale)["trust"] != "stale" {
		t.Fatal("event did not invalidate")
	}
	old := filepath.Join(folder, "000001.A-1.task.todo.md")
	next := filepath.Join(folder, "000001.A-1.task.doing.md")
	if err := os.Rename(old, next); err != nil {
		t.Fatal(err)
	}
	projectionRefresh(t, project, folder, false)
	changed, _ := ReadLifecycleProjection(project, folder, "A-1")
	raw, _ := json.Marshal(changed)
	if changed["file_state"] != "doing" || !strings.Contains(string(raw), "started-test") {
		t.Fatalf("lost evidence: %s", raw)
	}
	if projectionMeta(t, changed)["version"] == projectionMeta(t, initial)["version"] {
		t.Fatal("version unchanged")
	}
	isolated, _ := ReadLifecycleProjection(other, otherFolder, "A-1")
	if projectionMeta(t, isolated)["version"] != projectionMeta(t, unrelated)["version"] {
		t.Fatal("unrelated project rebuilt")
	}
	// Simulate a new Web process: no snapshot is accepted until rebuilt.
	lifecycleProjections.Lock()
	delete(lifecycleProjections.entries, lifecycleProjectionKey(project, folder))
	lifecycleProjections.Unlock()
	pending, ok := ReadLifecycleProjection(project, folder, "A-1")
	if ok || projectionMeta(t, pending)["trust"] != "pending" {
		t.Fatal("restart invented evidence")
	}
	projectionRefresh(t, project, folder, false)
	restored, _ := ReadLifecycleProjection(project, folder, "A-1")
	raw, _ = json.Marshal(restored)
	if !strings.Contains(string(raw), "started-test") {
		t.Fatal("restart lost durable event")
	}
	// No in-process invalidation: metadata scanner still recovers external edits.
	renamed := filepath.Join(folder, "000001.A-1.task.hold.md")
	if err := os.Rename(next, renamed); err != nil {
		t.Fatal(err)
	}
	projectionRefresh(t, project, folder, false)
	external, _ := ReadLifecycleProjection(project, folder, "A-1")
	if external["file_state"] != "hold" {
		t.Fatal("missed event not recovered")
	}
}

func TestLifecycleProjectionInputVersionsAndErrorRecovery(t *testing.T) {
	project, folder := projectionFixture(t, 1)
	projectionRefresh(t, project, folder, true)
	before, err := lifecycleProjectionSourceRevision(project, folder)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventID: "execution-test", AttemptID: "runtime-test", EventKind: "start", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	after, _ := lifecycleProjectionSourceRevision(project, folder)
	if before == after {
		t.Fatal("runtime not an input")
	}
	projectionRefresh(t, project, folder, false)
	path := filepath.Join(project, "_task_mecca", ".runtime", "handoffs", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{corrupt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := RefreshLifecycleProjection(project, folder, false); err == nil {
		t.Fatal("corrupt handoff accepted")
	}
	failed, ok := ReadLifecycleProjection(project, folder, "A-1")
	if !ok || projectionMeta(t, failed)["trust"] != "error" {
		t.Fatalf("failure hidden: %+v", failed)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	projectionRefresh(t, project, folder, false)
	recovered, _ := ReadLifecycleProjection(project, folder, "A-1")
	if projectionMeta(t, recovered)["trust"] != "verified" {
		t.Fatal("error did not recover")
	}
}

func TestLifecycleProjectionForcedRecoveryDetectsPreservedMetadata(t *testing.T) {
	project, folder := projectionFixture(t, 1)
	projectionRefresh(t, project, folder, true)
	path := filepath.Join(folder, "000001.A-1.task.todo.md")
	info, _ := os.Stat(path)
	raw, _ := os.ReadFile(path)
	// Same size and mtime bypass ordinary metadata caches. Forced recovery must
	// invalidate parsed records as well as the rendered snapshot.
	edited := strings.Replace(string(raw), "Agent: -", "Agent: x", 1)
	if len(edited) != len(raw) {
		t.Fatal("fixture size changed")
	}
	if err := os.WriteFile(path, []byte(edited), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	lifecycleProjections.Lock()
	e := lifecycleProjectionEntryFor(lifecycleProjectionKey(project, folder))
	e.computedAt = time.Now().Add(-2 * lifecycleProjectionRebuildInterval)
	e.validatedAt = time.Now()
	lifecycleProjections.Unlock()
	stale, _ := ReadLifecycleProjection(project, folder, "A-1")
	if projectionMeta(t, stale)["trust"] != "stale" {
		t.Fatal("expired computation reported current")
	}
	projectionRefresh(t, project, folder, false)
	row, ok, err := CachedTaskRecord(project, folder, "A-1")
	if err != nil || !ok || row.Fields["Agent"] != "x" {
		t.Fatalf("forced recovery retained stale record: %+v %v", row, err)
	}
}

func TestLifecycleProjectionConcurrentRequestsCoalesceAndRemainReadable(t *testing.T) {
	project, folder := projectionFixture(t, 1)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	guard := func(work func()) bool { close(entered); <-release; work(); close(done); return true }
	ScheduleLifecycleProjectionRefresh(project, folder, guard)
	<-entered
	var readers sync.WaitGroup
	for i := 0; i < 50; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			ScheduleLifecycleProjectionRefresh(project, folder, guard)
			ReadLifecycleProjection(project, folder, "A-1")
		}()
	}
	readers.Wait()
	close(release)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("refresh stalled")
	}
	projectionRefresh(t, project, folder, false)
	before, scans := projectionCounts(project, folder)
	projectionchange.Mark(project)
	item, _ := ReadLifecycleProjection(project, folder, "A-1")
	if projectionMeta(t, item)["trust"] != "stale" {
		t.Fatal("invalidation missing")
	}
	after, afterScans := projectionCounts(project, folder)
	if before != after || scans != afterScans {
		t.Fatal("read refreshed synchronously")
	}
}

func TestLifecycleProjectionLifecycleCorruptionNeverBecomesVerifiedEmptySnapshot(t *testing.T) {
	project, folder := projectionFixture(t, 1)
	path := filepath.Join(lifecycleEventsDir(project), "broken", "event.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := RefreshLifecycleProjection(project, folder, true); err == nil {
		t.Fatal("corrupt lifecycle accepted")
	}
	failed, ok := ReadLifecycleProjection(project, folder, "A-1")
	if ok || projectionMeta(t, failed)["trust"] != "error" {
		t.Fatalf("failed cold snapshot invented facts: %+v", failed)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	projectionRefresh(t, project, folder, false)
	recovered, ok := ReadLifecycleProjection(project, folder, "A-1")
	if !ok || projectionMeta(t, recovered)["trust"] != "verified" {
		t.Fatal("could not recover")
	}
}

func TestLifecycleProjectionHandoffClaimAndMarkInvalidateVersion(t *testing.T) {
	project, folder := projectionFixture(t, 1)
	projectionRefresh(t, project, folder, true)
	first, _ := ReadLifecycleProjection(project, folder, "A-1")
	version := projectionMeta(t, first)["version"]
	path := filepath.Join(project, "_task_mecca", ".runtime", "handoffs", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	prepared := `{"handoff_id":"test-handoff","record_kind":"prepared","event_type":"registration_ready","task_id":"A-1","target_agent_path":"/root/controller","observed_at":"2026-10-28T00:00:00Z"}` + "\n"
	if err := os.WriteFile(path, []byte(prepared), 0600); err != nil {
		t.Fatal(err)
	}
	projectionRefresh(t, project, folder, false)
	next, _ := ReadLifecycleProjection(project, folder, "A-1")
	if projectionMeta(t, next)["version"] == version {
		t.Fatal("prepare not detected")
	}
	version = projectionMeta(t, next)["version"]
	claimed := `{"handoff_id":"test-handoff","record_kind":"claimed","claimed_by":"/root/controller","claimed_attempt_id":"test-attempt","observed_at":"2026-10-28T00:00:01Z"}` + "\n"
	if err := os.WriteFile(path, []byte(prepared+claimed), 0600); err != nil {
		t.Fatal(err)
	}
	projectionRefresh(t, project, folder, false)
	next, _ = ReadLifecycleProjection(project, folder, "A-1")
	if projectionMeta(t, next)["version"] == version {
		t.Fatal("claim not detected")
	}
	version = projectionMeta(t, next)["version"]
	marked := `{"handoff_id":"test-handoff","record_kind":"step_succeeded","step":"acceptance","result":"ok","observed_at":"2026-10-28T00:00:02Z"}` + "\n"
	if err := os.WriteFile(path, []byte(prepared+claimed+marked), 0600); err != nil {
		t.Fatal(err)
	}
	projectionRefresh(t, project, folder, false)
	next, _ = ReadLifecycleProjection(project, folder, "A-1")
	if projectionMeta(t, next)["version"] == version {
		t.Fatal("mark not detected")
	}
}

func TestLifecycleProjectionGitHistoryChangeRebuildsCanonicalSnapshot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	project, folder := projectionFixture(t, 1)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = project
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q")
	git("config", "user.email", "fixture@example.invalid")
	git("config", "user.name", "Fixture")
	git("add", "_task_mecca/data/backlog")
	git("-c", "commit.gpgsign=false", "commit", "-qm", "registered task")
	projectionRefresh(t, project, folder, true)
	before, _ := ReadLifecycleProjection(project, folder, "A-1")
	old := filepath.Join(folder, "000001.A-1.task.todo.md")
	next := filepath.Join(folder, "000001.A-1.task.hold.md")
	if err := os.Rename(old, next); err != nil {
		t.Fatal(err)
	}
	git("add", "_task_mecca/data/backlog")
	git("-c", "commit.gpgsign=false", "commit", "-qm", "task waiting")
	projectionRefresh(t, project, folder, false)
	after, _ := ReadLifecycleProjection(project, folder, "A-1")
	if projectionMeta(t, before)["source_revision"] == projectionMeta(t, after)["source_revision"] {
		t.Fatal("Git change did not invalidate source revision")
	}
	if after["file_state"] != "hold" {
		t.Fatal("Git/rename change not rebuilt")
	}
	// Snapshot remains exactly the output of the existing canonical engine.
	full, _, err := TaskDetailWithTimings(project, folder, "A-1")
	if err != nil {
		t.Fatal(err)
	}
	latest, _ := ReadLifecycleProjection(project, folder, "A-1")
	a, _ := json.Marshal(latest["lifecycle"])
	b, _ := json.Marshal(full["lifecycle"])
	if string(a) != string(b) {
		t.Fatalf("read engine drifted: %s != %s", a, b)
	}
}
