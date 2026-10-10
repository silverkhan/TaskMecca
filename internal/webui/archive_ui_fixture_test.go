package webui

import (
	"github.com/silverkhan/TaskMecca/internal/install"
	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Opt-in synthetic fixture: no production registry, OS Trash or transport use.
func TestServeA26ArchiveUIFixture(t *testing.T) {
	root := os.Getenv("TASK_MECCA_A26_UI_FIXTURE_ROOT")
	if root == "" {
		t.Skip("isolated UI fixture only")
	}
	if !strings.HasPrefix(root, "/private/tmp/a26-kkobugi-ui-") {
		t.Fatal("unexpected fixture boundary")
	}
	t.Setenv("TASK_MECCA_HOME", filepath.Join(root, "management"))
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "disabled")
	project := filepath.Join(root, "금융_프로젝트_긴_이름과_전체_경로_확인용_수정된_framework")
	archived := filepath.Join(root, "보관한_프로젝트_전체_경로를_확인하는_사용자_자료_보존_예제")
	for _, path := range []string{project, archived} {
		if err := install.Init(path, "0.2.0"); err != nil {
			t.Fatal(err)
		}
		if err := maintenance.RegisterProject(path); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := maintenance.ArchiveProject(archived); err != nil {
		t.Fatal(err)
	}
	if err := maintenance.SetProjectMonitoring(project, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "_task_mecca", "framework", "web", "app.js"), []byte("// Synthetic local modification for migration choice QA\n"), 0600); err != nil {
		t.Fatal(err)
	}
	h, err := Handler(project, "", "A26-fixture")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:18926")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Log("A26 isolated http://127.0.0.1:18926 · synthetic monitoring stopped")
	if err := http.Serve(listener, h); err != nil {
		t.Fatal(err)
	}
}
