package maintenance

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAutomaticRegistrationExcludesLinkedWorktreeButExplicitRegistrationAllowsIt(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	base := t.TempDir()
	repo := filepath.Join(base, "repository")
	if err := os.MkdirAll(repo, 0700); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git("init")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "fixture")
	worktree := filepath.Join(base, "linked worktree")
	git("worktree", "add", "-b", "fixture-linked", worktree)
	for _, project := range []string{repo, worktree} {
		if err := os.MkdirAll(filepath.Join(project, "_task_mecca"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if !linkedGitWorktree(worktree) || linkedGitWorktree(repo) {
		t.Fatal("Git worktree classification failed")
	}
	if err := RegisterWebProject(worktree); err != nil {
		t.Fatal(err)
	}
	reg, err := readProjectRegistry()
	if err != nil || len(reg.Projects) != 0 {
		t.Fatalf("automatic linked registration: %+v %v", reg, err)
	}
	if err := RegisterProject(worktree); err != nil {
		t.Fatal(err)
	}
	if err := RegisterWebProject(repo); err != nil {
		t.Fatal(err)
	}
	reg, err = readProjectRegistry()
	if err != nil || len(reg.Projects) != 2 {
		t.Fatalf("explicit/ordinary registration: %+v %v", reg, err)
	}
}

func TestGitFileWithoutCommonDirIsNotExcluded(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, "module-admin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".git"), []byte("gitdir: module-admin\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if linkedGitWorktree(project) {
		t.Fatal("submodule-style gitdir falsely excluded")
	}
	if err := RegisterWebProject(project); err != nil {
		t.Fatal(err)
	}
	reg, err := readProjectRegistry()
	if err != nil || len(reg.Projects) != 1 {
		t.Fatalf("gitdir registration: %+v %v", reg, err)
	}
}
