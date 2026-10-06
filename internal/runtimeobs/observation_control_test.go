package runtimeobs

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type heldHookReader struct {
	started chan struct{}
	release chan struct{}
	payload string
}

func (r *heldHookReader) Read(p []byte) (int, error) {
	select {
	case <-r.started:
	default:
		close(r.started)
	}
	<-r.release
	if r.payload == "" {
		return 0, io.EOF
	}
	n := copy(p, r.payload)
	r.payload = r.payload[n:]
	return n, nil
}

func TestObservationDisableWaitsForInFlightAppend(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	reader := &heldHookReader{started: make(chan struct{}), release: make(chan struct{}), payload: `{"session_id":"s","turn_id":"t","hook_event_name":"SubagentStart","agent_id":"a"}`}
	observed := make(chan error, 1)
	go func() { _, err := ObserveHook(project, "codex", reader, time.Now()); observed <- err }()
	<-reader.started
	disabled := make(chan error, 1)
	go func() { _, err := SetObservationEnabled("codex", false); disabled <- err }()
	select {
	case err := <-disabled:
		t.Fatalf("disable returned while append in flight: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(reader.release)
	if err := <-observed; err != nil {
		t.Fatal(err)
	}
	if err := <-disabled; err != nil {
		t.Fatal(err)
	}
	blocked, err := ObserveHook(project, "codex", strings.NewReader(`{"session_id":"later","hook_event_name":"SubagentStart","agent_id":"a"}`), time.Now())
	if err != nil || blocked.EventID != "" {
		t.Fatalf("record after disable: %+v %v", blocked, err)
	}
}

func TestObservationControlPreservesProvidersHooksAndHistory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := t.TempDir()
	path := filepath.Join(project, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"hooks":{"SubagentStart":[{"hooks":[{"type":"command","command":"echo other"}]}]},"custom":"keep"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureHooks(project, "codex"); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureGlobalHooks("codex"); err != nil {
		t.Fatal(err)
	}
	payload := `{"session_id":"s","turn_id":"t","hook_event_name":"SubagentStart","agent_id":"a"}`
	before, err := ObserveHook(project, "codex", strings.NewReader(payload), time.Now())
	if err != nil || before.EventID == "" {
		t.Fatalf("initial event=%+v err=%v", before, err)
	}
	if _, err := SetObservationEnabled("codex", false); err != nil {
		t.Fatal(err)
	}
	if _, err := DisableGlobalHooks("codex"); err != nil {
		t.Fatal(err)
	}
	still, err := HookStatus(project, "codex")
	if err != nil || !still.Installed {
		t.Fatalf("project hook lost: %+v %v", still, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `"custom": "keep"`) || !strings.Contains(string(data), "echo other") {
		t.Fatalf("other settings lost: %s %v", data, err)
	}
	blocked, err := ObserveHook(project, "codex", strings.NewReader(payload), time.Now())
	if err != nil || blocked.EventID != "" {
		t.Fatalf("disabled event=%+v err=%v", blocked, err)
	}
	legacy, err := Observe(project, "codex", strings.NewReader(payload), time.Now())
	if err != nil || legacy.ObservedAt != "" {
		t.Fatalf("legacy spike recorded while disabled: %+v %v", legacy, err)
	}
	claude, err := ObserveHook(project, "claude", strings.NewReader(payload), time.Now())
	if err != nil || claude.EventID == "" {
		t.Fatalf("other provider blocked: %+v %v", claude, err)
	}
	if _, err := EnsureGlobalHooks("codex"); err != nil {
		t.Fatal(err)
	}
	resumed, err := SetObservationEnabled("codex", true)
	if err != nil {
		t.Fatal(err)
	}
	resumeAt, err := time.Parse(time.RFC3339Nano, resumed.ChangedAt)
	if err != nil {
		t.Fatal(err)
	}
	late, err := ObserveHook(project, "codex", strings.NewReader(payload), resumeAt.Add(-time.Nanosecond))
	if err != nil || late.EventID != "" {
		t.Fatalf("pre-resume event recorded: %+v %v", late, err)
	}
	fresh := `{"session_id":"new","turn_id":"new","hook_event_name":"SubagentStart","agent_id":"a"}`
	newEvent, err := ObserveHook(project, "codex", strings.NewReader(fresh), resumeAt.Add(time.Second))
	if err != nil || newEvent.EventID == "" {
		t.Fatalf("new event not recorded: %+v %v", newEvent, err)
	}
	ledger, err := BuildLedger(project, 10, resumeAt.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Attempts) != 3 {
		t.Fatalf("expected prior Codex, Claude and new Codex; got %d", len(ledger.Attempts))
	}
}
