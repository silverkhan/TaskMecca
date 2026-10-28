package webui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/maintenance"
)

func TestLifecycleProjectionBypassesMonitoringAndReportsFreshness(t *testing.T) {
	project := t.TempDir()
	registerWebFixture(t, project)
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "000001.A-1.read.todo.md"), []byte("# A-1 Read only\n"), 0600); err != nil {
		t.Fatal(err)
	}
	handler, err := Handler(project, "", "test")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := backlog.RefreshLifecycleProjection(project, folder, i == 0); err != nil {
			t.Fatal(err)
		}
	}
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan bool, 1)
	go func() { finished <- maintenance.WithProjectMonitoring(project, func() { close(entered); <-release }) }()
	<-entered
	response := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/tasks/A-1?projection=lifecycle", nil))
		response <- r
	}()
	select {
	case rec := <-response:
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d %s", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("projection may be cached by browser")
		}
		timing := rec.Header().Get("Server-Timing")
		for _, phase := range []string{"monitor_wait;dur=0", "catalog;dur=0", "control;dur=0", "selection;dur=", "projection;dur="} {
			if !strings.Contains(timing, phase) {
				t.Fatalf("missing %s: %s", phase, timing)
			}
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		meta := payload["snapshot"].(map[string]any)
		if meta["version"] == "" || meta["computed_at"] == nil || meta["validated_at"] == nil || meta["trust"] != "verified" {
			t.Fatalf("freshness missing: %+v", meta)
		}
		if _, ok := payload["raw_markdown"]; ok {
			t.Fatal("projection contains body")
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("lifecycle read waited for monitoring mutex")
	}
	close(release)
	if !<-finished {
		t.Fatal("monitor unexpectedly suppressed")
	}
	// Wait for the scheduled control job before removing fixture files.
	deadline := time.Now().Add(10 * time.Second)
	for {
		item, _ := backlog.ReadLifecycleProjection(project, folder, "A-1")
		if item["snapshot"].(map[string]any)["refreshing"] != true {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("refresh did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	if err := maintenance.SetProjectMonitoring(project, false); err != nil {
		t.Fatal(err)
	}
	paused := httptest.NewRecorder()
	handler.ServeHTTP(paused, httptest.NewRequest(http.MethodGet, "/api/tasks/A-1?projection=lifecycle", nil))
	if paused.Code != http.StatusConflict {
		t.Fatalf("paused read status=%d", paused.Code)
	}
}

// A real local TCP/HTTP measurement complements in-process work counters.
// Browser paint and remote Tailscale are deliberately not inferred from it.
func TestLifecycleProjectionLocalhostTiming(t *testing.T) {
	project := t.TempDir()
	registerWebFixture(t, project)
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "000001.A-1.http.todo.md"), []byte("# A-1 HTTP timing\n"), 0600); err != nil {
		t.Fatal(err)
	}
	handler, err := Handler(project, "", "test")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := backlog.RefreshLifecycleProjection(project, folder, i == 0); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	samples := []map[string]any{}
	for i, projection := range []string{"content", "lifecycle", "lifecycle"} {
		phase := []string{"content:first", "lifecycle:first", "lifecycle:refresh"}[i]
		request, err := http.NewRequest(http.MethodGet, server.URL+"/api/tasks/A-1?projection="+projection, nil)
		if err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		var ttfb time.Duration
		request = request.WithContext(httptrace.WithClientTrace(request.Context(), &httptrace.ClientTrace{GotFirstResponseByte: func() { ttfb = time.Since(started) }}))
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		raw, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read=%v close=%v", readErr, closeErr)
		}
		if response.StatusCode != 200 {
			t.Fatalf("%s status=%d %s", phase, response.StatusCode, raw)
		}
		samples = append(samples, map[string]any{"phase": phase, "ttfb_ms": float64(ttfb.Nanoseconds()) / 1e6, "http_total_ms": float64(time.Since(started).Nanoseconds()) / 1e6, "server_timing": response.Header.Get("Server-Timing")})
	}
	data, err := json.MarshalIndent(map[string]any{"route": "localhost TCP/HTTP", "snapshot_prebuilt": true, "samples": samples, "browser_paint": "not measured by HTTP client", "remote_tailscale": "not measured; record separately in Dev"}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(data))
	if path := os.Getenv("TASK_MECCA_LIFECYCLE_HTTP_REPORT"); path != "" {
		if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		item, _ := backlog.ReadLifecycleProjection(project, folder, "A-1")
		if item["snapshot"].(map[string]any)["refreshing"] != true {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("refresh did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}
