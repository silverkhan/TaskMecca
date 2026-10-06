package runtimeobs

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestGlobalHooksPreserveOtherSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	original := `{"permissions":{"allow":["Bash(git status)"]},"hooks":{"SubagentStart":[{"hooks":[{"type":"command","command":"echo user-hook"}]}]}}`
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := EnsureGlobalHooks("claude")
	if err != nil {
		t.Fatal(err)
	}
	if !first.Installed || !first.Changed || first.Path != path {
		t.Fatalf("first=%+v", first)
	}
	again, err := EnsureGlobalHooks("claude")
	if err != nil || again.Changed {
		t.Fatalf("idempotence=%+v err=%v", again, err)
	}
	removed, err := DisableGlobalHooks("claude")
	if err != nil || removed.Installed {
		t.Fatalf("remove=%+v err=%v", removed, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document["permissions"] == nil || !strings.Contains(string(data), "echo user-hook") || strings.Contains(string(data), "task-mecca runtime observe") {
		t.Fatalf("user settings changed: %s", data)
	}
}

func TestGlobalHookHomeUsesUserScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, provider := range []string{"codex", "claude"} {
		status, err := EnsureGlobalHooks(provider)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(status.Path, home+string(os.PathSeparator)) {
			t.Fatalf("path=%s", status.Path)
		}
	}
}

func TestGlobalHookPreservesSymlinkedSettings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, "dotfiles", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(`{"other":"preserved"}`), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureGlobalHooks("codex"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced: %v %+v", err, info)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"other": "preserved"`) {
		t.Fatalf("target settings lost: %s", data)
	}
}

func TestHookEvidenceRequiresActualEventAfterConfiguration(t *testing.T) {
	project := t.TempDir()
	at := time.Now().UTC().Add(-time.Minute)
	payload := `{"session_id":"s","turn_id":"t","hook_event_name":"SubagentStart","agent_id":"a"}`
	if _, err := ObserveHook(project, "codex", strings.NewReader(payload), at); err != nil {
		t.Fatal(err)
	}
	cutoff := at.Add(10 * time.Second).Format(time.RFC3339Nano)
	events, last, err := HookEvidenceSince(project, "codex", cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if last != "" || events["start"] {
		t.Fatalf("historical event counted: %+v %s", events, last)
	}
	second := `{"session_id":"s","turn_id":"t","hook_event_name":"PreToolUse","agent_id":"a","tool_use_id":"tool-1"}`
	if _, err := ObserveHook(project, "codex", strings.NewReader(second), at.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	events, last, err = HookEvidenceSince(project, "codex", cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if !events["activity"] || events["start"] || last == "" {
		t.Fatalf("evidence=%+v last=%s", events, last)
	}
}

func TestGlobalProjectHookDuplicateChild(t *testing.T) {
	if os.Getenv("TASK_MECCA_A4_CHILD") != "1" {
		return
	}
	project := os.Getenv("TASK_MECCA_A4_PROJECT")
	payload := `{"session_id":"shared","turn_id":"turn","hook_event_name":"SubagentStart","agent_id":"worker"}`
	if _, err := ObserveHook(project, "codex", strings.NewReader(payload), time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalProjectHookConcurrentInvocationRecordsOnce(t *testing.T) {
	project := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	commands := []*exec.Cmd{exec.Command(binary, "-test.run=^TestGlobalProjectHookDuplicateChild$"), exec.Command(binary, "-test.run=^TestGlobalProjectHookDuplicateChild$")}
	for _, command := range commands {
		command.Env = append(os.Environ(), "TASK_MECCA_A4_CHILD=1", "TASK_MECCA_A4_PROJECT="+project)
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range commands {
		if err := command.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	files, err := executionEventFiles(project)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, path := range files {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if strings.Contains(line, `"hook_event_name":"SubagentStart"`) {
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("raw Hook event count=%d, want 1", count)
	}
}

func TestGlobalHookInUninstalledProjectHasNoJournal(t *testing.T) {
	project := t.TempDir()
	if _, err := ResolveProject(project); err == nil {
		t.Fatal("uninstalled project resolved as Task Mecca project")
	}
	if _, err := os.Stat(ExecutionRootPath(project)); !os.IsNotExist(err) {
		t.Fatalf("execution journal created: %v", err)
	}
}
