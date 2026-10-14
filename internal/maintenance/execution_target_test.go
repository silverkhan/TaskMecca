package maintenance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecutionTargetDefaultsAndPinnedPR(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	t.Setenv("TASK_MECCA_CHANNEL", "stable")
	target, err := InstalledExecutionTarget()
	if err != nil || target.Kind != "stable" || target.Channel != "stable" {
		t.Fatalf("stable target = %+v, %v", target, err)
	}
	t.Setenv("TASK_MECCA_CHANNEL", "dev")
	target, err = InstalledExecutionTarget()
	if err != nil || target.Kind != "dev" || target.Channel != "dev" {
		t.Fatalf("dev target = %+v, %v", target, err)
	}
	pinned := ExecutionTarget{
		Kind: "pr", Channel: "dev", PRNumber: 213,
		HeadSHA: strings.Repeat("a", 40),
		OriginalDevVersion: "0.3.1-dev.16",
		OriginalDevSHA256: strings.Repeat("b", 64),
		OriginalDevBackup: filepath.Join(t.TempDir(), "dev-unchanged"),
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := pinned.validate(); err != nil { t.Fatal(err) }
	if err := writeExecutionJSON(executionTargetPath(), pinned); err != nil { t.Fatal(err) }
	target, err = InstalledExecutionTarget()
	if err != nil || target != pinned { t.Fatalf("PR target = %+v, %v", target, err) }

	t.Setenv("TASK_MECCA_CHANNEL", "stable")
	if _, err := InstalledExecutionTarget(); err == nil {
		t.Fatal("stale PR marker must block reporting Stable")
	}
}

func TestExecutionTargetRejectsInvalidMetadata(t *testing.T) {
	valid := ExecutionTarget{
		Kind: "pr", Channel: "dev", PRNumber: 1,
		HeadSHA: strings.Repeat("a", 40),
		OriginalDevVersion: "dev.1",
		OriginalDevSHA256: strings.Repeat("b", 64),
		OriginalDevBackup: filepath.Join(t.TempDir(), "baseline"),
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}
	cases := map[string]func(*ExecutionTarget){
		"forked channel": func(p *ExecutionTarget) { p.Channel = "stable" },
		"missing PR": func(p *ExecutionTarget) { p.PRNumber = 0 },
		"uppercase SHA": func(p *ExecutionTarget) { p.HeadSHA = strings.Repeat("A",40) },
		"short SHA": func(p *ExecutionTarget) { p.HeadSHA = "aaaa" },
		"missing baseline": func(p *ExecutionTarget) { p.OriginalDevBackup = "" },
		"bad digest": func(p *ExecutionTarget) { p.OriginalDevSHA256 = strings.Repeat("x",64) },
		"relative baseline": func(p *ExecutionTarget) { p.OriginalDevBackup = "./dev" },
		"missing timestamp": func(p *ExecutionTarget) { p.InstalledAt = "" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			check := valid
			change(&check)
			if err := check.validate(); err == nil { t.Fatal("invalid PR target accepted") }
		})
	}
}

func TestExecutionStateCorruptAndSymlinkFailsClosed(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	t.Setenv("TASK_MECCA_CHANNEL", "dev")
	if err := os.WriteFile(executionTargetPath(), []byte("{broken"), 0600); err != nil { t.Fatal(err) }
	if _, err := InstalledExecutionTarget(); err == nil { t.Fatal("malformed JSON must not become Dev") }
	if err := os.Remove(executionTargetPath()); err != nil { t.Fatal(err) }
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("{}"), 0600); err != nil { t.Fatal(err) }
	if err := os.Symlink(outside, executionTargetPath()); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := InstalledExecutionTarget(); err == nil { t.Fatal("symlinked record accepted") }
	if err := writeExecutionJSON(executionTargetPath(), map[string]any{"bad":true}); err == nil {
		t.Fatal("atomic writer replaced a symlink")
	}
}

func TestExecutionTransitionCheckpoint(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	if got, err := PendingExecutionTransition(); err != nil || got != nil {
		t.Fatalf("unexpected checkpoint: %v %v", got, err)
	}
	from := ExecutionTarget{Kind: "dev", Channel: "dev"}
	to := ExecutionTarget{
		Kind: "pr", Channel: "dev", PRNumber: 99,
		HeadSHA: strings.Repeat("c",40), OriginalDevVersion: "dev",
		OriginalDevSHA256: strings.Repeat("d",64),
		OriginalDevBackup: filepath.Join(t.TempDir(), "original-dev"),
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}
	cp := ExecutionTransition{Schema:1, Phase:"preparing", From:from, To:to,
		UpdatedAt:time.Now().UTC().Format(time.RFC3339)}
	if err := writeExecutionJSON(executionTransitionPath(), cp); err != nil { t.Fatal(err) }
	read, err := PendingExecutionTransition()
	if err != nil || read == nil || *read != cp { t.Fatalf("read checkpoint: %v %v", read, err) }
	cp.Phase = "unknown"
	if err := writeExecutionJSON(executionTransitionPath(), cp); err != nil { t.Fatal(err) }
	if _, err := PendingExecutionTransition(); err == nil { t.Fatal("unknown phase accepted") }
}
