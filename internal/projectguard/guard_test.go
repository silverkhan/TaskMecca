package projectguard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "management")
	t.Setenv("TASK_MECCA_HOME", home)
	project := filepath.Join(root, "holder", "project")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	return home, project
}
func paused(t *testing.T, home, project string) {
	t.Helper()
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"projects": []any{}, "paused_projects": []string{project}})
	if err := os.WriteFile(filepath.Join(home, "projects.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAliasAndDescendantSuppression(t *testing.T) {
	home, project := fixture(t)
	paused(t, home, project)
	if Allowed(project) || Allowed(filepath.Join(project, "nested")) {
		t.Fatal("suppressed project allowed")
	}
	alias := filepath.Join(filepath.Dir(filepath.Dir(project)), "alias")
	if err := os.Symlink(filepath.Dir(project), alias); err != nil {
		t.Skip(err)
	}
	if Allowed(filepath.Join(alias, "project")) {
		t.Fatal("parent alias bypassed suppression")
	}
	if err := os.Rename(project, project+"-recovery"); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireWrite(filepath.Join(alias, "project")); err == nil {
		t.Fatal("missing aliased project allowed")
	}
	if _, err := os.Stat(project); !os.IsNotExist(err) {
		t.Fatal("source recreated", err)
	}
}

func TestInvalidRegistryFailsClosed(t *testing.T) {
	home, project := fixture(t)
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "projects.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if Allowed(project) {
		t.Fatal("invalid registry allowed")
	}
}

func TestRemovedSourceSymlinkDoesNotExpandSuppression(t *testing.T) {
	home, project := fixture(t)
	canonical, err := CanonicalPath(project)
	if err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(t.TempDir(), "registered-root")
	if err := os.MkdirAll(unrelated, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"projects": []string{unrelated}, "paused_projects": []string{project, canonical}, "removal_history": []map[string]string{{"path": project}}})
	if err := os.WriteFile(filepath.Join(home, "projects.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(project, project+"-recovery"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(unrelated, project); err != nil {
		t.Skip(err)
	}
	if Allowed(project) {
		t.Fatal("removed source alias allowed")
	}
	if !Allowed(unrelated) {
		t.Fatal("historical boundary expanded to unrelated target")
	}
}

func TestCaseVariantSameFileSuppression(t *testing.T) {
	home, project := fixture(t)
	variant := filepath.Join(filepath.Dir(project), "PROJECT")
	one, err := os.Stat(project)
	if err != nil {
		t.Fatal(err)
	}
	two, err := os.Stat(variant)
	if err != nil || !os.SameFile(one, two) {
		t.Skip("fixture volume is case-sensitive")
	}
	paused(t, home, project)
	if Allowed(variant) {
		t.Fatal("same-file case variant bypassed suppression")
	}
}

func TestNestedSharedGuardDoesNotWaitBehindMutation(t *testing.T) {
	home, project := fixture(t)
	first, err := AcquireWrite(project)
	if err != nil {
		t.Fatal(err)
	}
	waiting := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(waiting)
		release, err := AcquireManagement()
		if err == nil {
			raw, _ := json.Marshal(map[string]any{"paused_projects": []string{project}})
			err = os.WriteFile(filepath.Join(home, "projects.json"), raw, 0600)
			release()
		}
		done <- err
	}()
	<-waiting
	for i := 0; i < 10; i++ {
		nested, err := AcquireWrite(project)
		if err != nil {
			first()
			t.Fatal(err)
		}
		nested()
	}
	first()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("management/nested guard deadlock")
	}
	if Allowed(project) {
		t.Fatal("mutation not observed")
	}
}

func TestBusyManagementFailsClosedWithoutProjectWrites(t *testing.T) {
	_, project := fixture(t)
	release, err := AcquireManagement()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := AcquireWrite(project); err == nil {
		t.Fatal("exclusive lease bypassed")
	}
	if _, err := os.Stat(filepath.Join(project, "_task_mecca")); !os.IsNotExist(err) {
		t.Fatal("guard created project data", err)
	}
}

func TestStaleReaderProcessAndRestart(t *testing.T) {
	if project := os.Getenv("TASK_MECCA_A25_GUARD_CHILD"); project != "" {
		cached := Allowed(project)
		fmt.Println("ready", cached)
		bufio.NewReader(os.Stdin).ReadString('\n')
		for i := 0; i < 5; i++ {
			release, err := AcquireWrite(project)
			if err == nil {
				_ = os.MkdirAll(filepath.Join(project, "_task_mecca", ".runtime"), 0700)
				release()
				fmt.Println("unsafe")
				os.Exit(3)
			}
		}
		fmt.Println("suppressed")
		return
	}
	home, project := fixture(t)
	launch := func() (*exec.Cmd, *bufio.Reader, *os.File) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestStaleReaderProcessAndRestart$")
		cmd.Env = append(os.Environ(), "TASK_MECCA_A25_GUARD_CHILD="+project)
		out, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stdin = reader
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		reader.Close()
		return cmd, bufio.NewReader(out), writer
	}
	cmd, out, in := launch()
	line, _ := out.ReadString('\n')
	if !strings.Contains(line, "ready true") {
		t.Fatal(line)
	}
	release, err := AcquireManagement()
	if err != nil {
		t.Fatal(err)
	}
	paused(t, home, project)
	if err := os.Rename(project, project+"-recovery"); err != nil {
		t.Fatal(err)
	}
	release()
	fmt.Fprintln(in, "scan")
	in.Close()
	line, _ = out.ReadString('\n')
	if !strings.Contains(line, "suppressed") {
		t.Fatal(line)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	cmd, out, in = launch()
	line, _ = out.ReadString('\n')
	if !strings.Contains(line, "ready false") {
		t.Fatal(line)
	}
	fmt.Fprintln(in, "scan")
	in.Close()
	line, _ = out.ReadString('\n')
	if !strings.Contains(line, "suppressed") {
		t.Fatal(line)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(project); !os.IsNotExist(err) {
		t.Fatal("source recreated", err)
	}
	if _, err := os.Stat(project + "-recovery"); err != nil {
		t.Fatal("recovery source lost", err)
	}
}
