package webui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/notify"
)

type lifecycleFakeTelegramTransport struct {
	telegram func(*http.Request) (*http.Response, error)
	fallback http.RoundTripper
}

func (f lifecycleFakeTelegramTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Version checks can already be in flight when this fixture starts. Only
	// intercept Telegram; redirects for unrelated background HTTP retain their
	// original transport and must not enter this test's delivery assertions.
	if r.URL.Host != "api.telegram.org" {
		return f.fallback.RoundTrip(r)
	}
	return f.telegram(r)
}

type lifecycleRoundTripFunc func(*http.Request) (*http.Response, error)

func (f lifecycleRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestLifecycleTelegramTransportIsolatesBackgroundVersionRequests(t *testing.T) {
	var telegram, background int
	response := func(r *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: r}
	}
	transport := lifecycleFakeTelegramTransport{
		telegram: func(r *http.Request) (*http.Response, error) {
			telegram++
			return response(r), nil
		},
		fallback: lifecycleRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			background++
			return response(r), nil
		}),
	}
	for _, target := range []string{
		"https://github.com/silverkhan/TaskMecca/releases/latest/download/VERSION.txt",
		"https://release-assets.githubusercontent.com/fixture/VERSION.txt",
		"https://api.telegram.org/botlocal-test/sendMessage",
	} {
		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if telegram != 1 || background != 2 {
		t.Fatalf("transport isolation: Telegram=%d background=%d", telegram, background)
	}
}

func TestDurableLifecycleHTTPProjectionWithAndWithoutGit(t *testing.T) {
	for _, fixture := range []struct {
		name            string
		withGit, legacy bool
	}{
		{name: "gitless"},
		{name: "git_uncommitted", withGit: true},
		{name: "legacy_backlog_b", legacy: true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			project := t.TempDir()
			if fixture.withGit {
				for _, args := range [][]string{{"init"}, {"config", "user.email", "ci@example.invalid"}, {"config", "user.name", "CI"}} {
					cmd := exec.Command("git", args...)
					cmd.Dir = project
					if output, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("git %v: %v: %s", args, err, output)
					}
				}
			}
			backlogDir := filepath.Join(project, "_task_mecca", "data", "backlog")
			if fixture.legacy {
				backlogDir = filepath.Join(project, "_task_mecca", "backlog_b")
			}
			if err := os.MkdirAll(backlogDir, 0755); err != nil {
				t.Fatal(err)
			}
			if _, err := backlog.AttentionSnapshot(project, "", true); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(backlogDir, "000001.B-1.http.doing.md")
			if err := os.WriteFile(file, []byte("# B-1 HTTP lifecycle\n- Agent: /root/controller/worker\n"), 0644); err != nil {
				t.Fatal(err)
			}
			base := time.Now().UTC().Add(-time.Minute)
			// Feed the exact HTTP event to the real Telegram delivery ledger. A
			// local transport substitutes only the external Telegram endpoint.
			configDir := filepath.Join(project, "_task_mecca", ".runtime", "notifications")
			if err := os.MkdirAll(configDir, 0755); err != nil {
				t.Fatal(err)
			}
			config := map[string]any{"token": "local-test", "chat_id": 7, "enabled": true,
				"kinds": map[string]bool{"started": true}, "activated_at": base.Add(-time.Second).Format(time.RFC3339Nano)}
			configBytes, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(configDir, "telegram.json"), configBytes, 0600); err != nil {
				t.Fatal(err)
			}
			var sent atomic.Int32
			previousTransport := http.DefaultTransport
			http.DefaultTransport = lifecycleFakeTelegramTransport{fallback: previousTransport, telegram: func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "api.telegram.org" || !strings.HasSuffix(r.URL.Path, "/sendMessage") {
					t.Errorf("unexpected Telegram request: %s", r.URL)
					return nil, io.EOF
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				text, _ := payload["text"].(string)
				if !strings.Contains(text, "["+filepath.Base(project)+"] ") {
					t.Errorf("wrong project identity: %q", text)
				}
				if strings.Contains(text, "B-1 · B-1") {
					t.Errorf("redundant task label: %q", text)
				}
				sent.Add(1)
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1}}`)), Request: r}, nil
			}}
			defer func() { http.DefaultTransport = previousTransport }()
			for i, event := range []backlog.LifecycleTransition{
				{EventID: "http-register", TaskID: "B-1", Kind: "registered", Actor: "/root/registrar", EvidenceSource: "registrar_report"},
				{EventID: "http-assign", TaskID: "B-1", Kind: "assigned", Actor: "/root/controller", EvidenceSource: "controller_report"},
				{EventID: "http-start", TaskID: "B-1", Kind: "started", Actor: "/root/controller/worker", EvidenceSource: "worker_report"},
				{EventID: "http-wait", TaskID: "B-1", Kind: "waiting", Actor: "/root/controller/worker", EvidenceSource: "worker_report"},
				{EventID: "http-resume", TaskID: "B-1", Kind: "resumed", Actor: "/root/controller/worker", EvidenceSource: "worker_report"},
			} {
				event.OccurredAt = base.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano)
				if _, err := backlog.RecordLifecycleTransition(project, event, base); err != nil {
					t.Fatal(err)
				}
			}
			registerWebFixture(t, project)
			handler, err := Handler(project, "", "test")
			if err != nil {
				t.Fatal(err)
			}
			request := func(path string) map[string]any {
				t.Helper()
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
				if rec.Code != http.StatusOK {
					t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
				}
				var payload map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				return payload
			}
			check := func(path string, lifecycle map[string]any) {
				t.Helper()
				events, ok := lifecycle["events"].([]any)
				if !ok || len(events) != 5 {
					t.Fatalf("%s events: %+v", path, lifecycle)
				}
				for i, want := range []string{"http-register", "http-assign", "http-start", "http-wait", "http-resume"} {
					event := events[i].(map[string]any)
					if event["event_id"] != want || event["provisional"] != false {
						t.Fatalf("%s event %d: %+v", path, i, event)
					}
				}
				if lifecycle["started_at"] != events[2].(map[string]any)["at"] {
					t.Fatalf("%s start timing changed: %+v", path, lifecycle)
				}
			}
			check("detail", request("/api/tasks/B-1")["lifecycle"].(map[string]any))
			check("workload", request("/api/workload")["all_items"].(map[string]any)["B-1"].(map[string]any)["lifecycle"].(map[string]any))
			attention := request("/api/attention")
			check("attention", attention["all_items"].(map[string]any)["B-1"].(map[string]any)["lifecycle"].(map[string]any))
			found := false
			for _, raw := range attention["notification_events"].([]any) {
				event := raw.(map[string]any)
				if event["kind"] == "started" && event["task_id"] == "B-1" {
					found = true
					if event["lifecycle_event_id"] != "http-start" {
						t.Fatalf("HTTP notification changed ID: %+v", event)
					}
				}
			}
			if !found {
				t.Fatalf("HTTP notification missing Worker start: %+v", attention["notification_events"])
			}
			for _, raw := range attention["notification_events"].([]any) {
				event := raw.(map[string]any)
				if event["kind"] != "started" || event["task_id"] != "B-1" {
					continue
				}
				telegramEvent := notify.Event{ID: event["id"].(string), TaskID: "B-1", Kind: "started", Title: "HTTP lifecycle", At: event["at"].(string)}
				if failures := notify.Deliver(project, []notify.Event{telegramEvent}); len(failures) > 0 {
					t.Fatal(failures)
				}
				if failures := notify.Deliver(project, []notify.Event{telegramEvent}); len(failures) > 0 {
					t.Fatal(failures)
				}
			}
			if sent.Load() != 2 {
				records, _ := notify.DeliveryRecords(project, "B-1", "")
				t.Fatalf("registered and started events sent %d times, want two; records=%+v", sent.Load(), records)
			}
			records, err := notify.DeliveryRecords(project, "B-1", "")
			if err != nil || len(records) != 2 {
				t.Fatalf("delivery ledger: %+v %v", records, err)
			}
			for _, record := range records {
				if record.State != "sent" || record.Attempts != 1 {
					t.Fatalf("canonical event delivered more than once: %+v", record)
				}
			}
		})
	}
}
