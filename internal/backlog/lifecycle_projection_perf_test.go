package backlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// This gate uses work counts as a deterministic invariant and same-run,
// same-machine timings as a secondary regression signal. It is opt-in locally
// and runs in CI; no developer laptop milliseconds are imposed on CI hardware.
func TestLifecycleProjectionPerformanceRegression(t *testing.T) {
	if os.Getenv("TASK_MECCA_LIFECYCLE_PERF") != "1" {
		t.Skip("set TASK_MECCA_LIFECYCLE_PERF=1 to run scale measurements")
	}
	type measurement struct {
		Tasks           int     `json:"tasks"`
		RuntimeEvents   int     `json:"runtime_events"`
		HandoffEvents   int     `json:"handoff_events"`
		ColdMS          float64 `json:"cold_ms"`
		LegacyMS        float64 `json:"legacy_ms"`
		RefreshMS       float64 `json:"refresh_ms"`
		AutomaticMS     float64 `json:"automatic_refresh_ms"`
		WarmP50NS       int64   `json:"warm_p50_ns"`
		WarmP95NS       int64   `json:"warm_p95_ns"`
		ConcurrentNS    int64   `json:"concurrent_ns_per_op"`
		ReadControlRuns uint64  `json:"read_control_runs"`
		ReadSourceScans uint64  `json:"read_source_scans"`
	}
	measurements := []measurement{}
	for _, n := range []int{100, 500, 1000} {
		project, folder := projectionFixture(t, n)
		now := time.Now().UTC()
		rawDir := runtimeobs.ExecutionRawDir(project)
		if err := os.MkdirAll(rawDir, 0700); err != nil {
			t.Fatal(err)
		}
		rf, err := os.Create(filepath.Join(rawDir, now.Format("2006-01-02")+".jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		hp := handoffjournal.JournalPath(project)
		if err := os.MkdirAll(filepath.Dir(hp), 0700); err != nil {
			t.Fatal(err)
		}
		hf, err := os.Create(hp)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < n; i++ {
			event := runtimeobs.ExecutionEvent{EventID: fmt.Sprintf("event-%d", i), AttemptID: fmt.Sprintf("run-%d", i), EventKind: "state", Provider: "codex", RuntimeAgentID: fmt.Sprintf("agent-%d", i), SessionID: "perf-session", ObservedAt: now.Add(time.Duration(i-n) * time.Millisecond).Format(time.RFC3339Nano), State: runtimeobs.StateCompleted, Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}
			if err := json.NewEncoder(rf).Encode(event); err != nil {
				t.Fatal(err)
			}
			h := handoffjournal.Record{HandoffID: fmt.Sprintf("handoff-%d", i), RecordKind: "prepared", TaskID: fmt.Sprintf("A-%d", i+1), EventType: handoffjournal.EventRegistrationReady, TargetAgentPath: "/root/controller", ObservedAt: now.Format(time.RFC3339Nano)}
			if err := json.NewEncoder(hf).Encode(h); err != nil {
				t.Fatal(err)
			}
		}
		if err := rf.Close(); err != nil {
			t.Fatal(err)
		}
		if err := hf.Close(); err != nil {
			t.Fatal(err)
		}
		m := measurement{Tasks: n, RuntimeEvents: n, HandoffEvents: n}
		start := time.Now()
		projectionRefresh(t, project, folder, true)
		m.ColdMS = float64(time.Since(start).Nanoseconds()) / 1e6
		start = time.Now()
		if _, _, err := TaskDetailWithTimings(project, folder, "A-1"); err != nil {
			t.Fatal(err)
		}
		m.LegacyMS = float64(time.Since(start).Nanoseconds()) / 1e6
		// An independently published canonical result must be verified before
		// it is called current. Measure an unchanged source check separately.
		projectionRefresh(t, project, folder, false)
		start = time.Now()
		if err := RefreshLifecycleProjection(project, folder, false); err != nil {
			t.Fatal(err)
		}
		m.RefreshMS = float64(time.Since(start).Nanoseconds()) / 1e6
		if _, err := RecordLifecycleTransition(project, LifecycleTransition{EventID: "performance-refresh", TaskID: "A-1", Kind: "registered", Actor: "/root/registrar", EvidenceSource: "registrar_report"}, time.Now()); err != nil {
			t.Fatal(err)
		}
		automaticDone := make(chan struct{})
		start = time.Now()
		ScheduleLifecycleProjectionRefresh(project, folder, func(work func()) bool { work(); close(automaticDone); return true })
		select {
		case <-automaticDone:
		case <-time.After(10 * time.Second):
			t.Fatal("automatic refresh stalled")
		}
		m.AutomaticMS = float64(time.Since(start).Nanoseconds()) / 1e6
		projectionRefresh(t, project, folder, false)
		builds, scans := projectionCounts(project, folder)
		samples := make([]int64, 300)
		for i := range samples {
			start = time.Now()
			if _, ok := ReadLifecycleProjection(project, folder, "A-1"); !ok {
				t.Fatal("snapshot unavailable")
			}
			samples[i] = time.Since(start).Nanoseconds()
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		m.WarmP50NS = samples[len(samples)/2]
		m.WarmP95NS = samples[len(samples)*95/100]
		start = time.Now()
		var calls sync.WaitGroup
		for i := 0; i < 16; i++ {
			calls.Add(1)
			go func() {
				defer calls.Done()
				for j := 0; j < 50; j++ {
					ReadLifecycleProjection(project, folder, "A-1")
				}
			}()
		}
		calls.Wait()
		m.ConcurrentNS = time.Since(start).Nanoseconds() / 800
		after, afterScans := projectionCounts(project, folder)
		m.ReadControlRuns = after - builds
		m.ReadSourceScans = afterScans - scans
		if m.ReadControlRuns != 0 || m.ReadSourceScans != 0 {
			t.Fatalf("%d tasks: read triggered global work: %+v", n, m)
		}
		// Baseline is the same fixture's former canonical full-detail path, measured
		// on this exact machine/run. A projection must remain cheaper than that path.
		if float64(m.WarmP95NS)/1e6 >= m.LegacyMS {
			t.Fatalf("read no longer cheaper than same-run control baseline: %+v", m)
		}
		measurements = append(measurements, m)
	}
	// The 100-task projection is a same-run baseline for larger inputs. A broad
	// noise allowance avoids machine-speed constants; deterministic zero-work and
	// catalog-lock tests are the primary gates, not a fragile wall-clock budget.
	if measurements[2].WarmP50NS > measurements[0].WarmP95NS*4 {
		t.Fatalf("read latency scales with global history: %+v", measurements)
	}
	report := map[string]any{"go_version": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "cpus": runtime.NumCPU(), "measured_at": time.Now().UTC().Format(time.RFC3339Nano), "route": "in_process", "network": "not measured; localhost HTTP and remote Tailscale are separate diagnostics", "baseline": "same fixture legacy full-detail path and same-run 100-task read distribution", "measurements": measurements}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(data))
	if path := os.Getenv("TASK_MECCA_LIFECYCLE_PERF_REPORT"); path != "" {
		if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
