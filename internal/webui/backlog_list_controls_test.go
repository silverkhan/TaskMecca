package webui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedA24ListControlsUIFixture(t *testing.T) {
	root := os.Getenv("TASK_MECCA_A24_UI_FIXTURE_ROOT")
	if root == "" {
		t.Skip("explicit isolated UI fixture only")
	}
	if !strings.HasPrefix(root, "/private/tmp/a24-raichyu-") {
		t.Fatal("unexpected UI fixture path")
	}
	for _, folder := range []string{"backlog", "backlog_long_localized_folder_name_for_mobile_검증", "backlog_empty"} {
		dir := filepath.Join(root, "_task_mecca", "data", folder)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if folder == "backlog_empty" {
			continue
		}
		count := 31
		if folder != "backlog" {
			count = 2
		}
		for i := 1; i <= count; i++ {
			body := fmt.Sprintf("# A-%d Fixture needle task %d\n\n## 작업 개요\n- Agent: -\n- 변경범위: isolated UI fixture\n\n- Tags: area:frontend\n\n## 핵심 요약\n- 목적: Search needle and folder interaction\n- 핵심 변경: Synthetic task, no runtime events\n", i, i)
			file := filepath.Join(dir, fmt.Sprintf("%06d.A-%d.fixture.todo.md", i, i))
			if err := os.WriteFile(file, []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
