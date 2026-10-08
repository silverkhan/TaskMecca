package backlog

import (
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPageAggregateScopeExcludesStatusSelectionAndRefreshesChanges(t *testing.T) {
	project := t.TempDir()
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	paths := []string{}
	for i, state := range []string{"todo", "doing", "hold", "done"} {
		id := string(rune('1' + i))
		paths = append(paths, writeWebTask(t, folder, "00000"+id+".A-"+id+".match."+state+".md", "# A-"+id+" Match\n- Tags: area:web\n- 대기유형: user\n- 대기: 정책 판단\n"))
	}
	writeWebTask(t, folder, "000005.A-5.other.done.md", "# A-5 Other\n- Tags: area:other\n")
	if _, err := EnsureTagRegistry(project); err != nil {
		t.Fatal(err)
	}
	query := func() map[string]any {
		value, err := BacklogPage(project, folder, 1, 1, []string{"done"}, []string{"area:web"}, "Match", "id_desc", "summary")
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	value := query()
	counts := value["counts"].(map[string]int)
	if counts["all"] != 4 || counts["working"] != 1 || counts["hold"] != 1 || counts["done"] != 1 || value["total"] != 1 || len(value["items"].([]map[string]any)) != 0 {
		t.Fatalf("scope=%+v", value)
	}
	if err := os.Remove(paths[3]); err != nil {
		t.Fatal(err)
	}
	if query()["counts"].(map[string]int)["done"] != 0 {
		t.Fatal("deleted file retained")
	}
	writeWebTask(t, folder, "000006.A-6.match.done.md", "# A-6 Match\n- Tags: area:web\n")
	if query()["counts"].(map[string]int)["done"] != 1 {
		t.Fatal("new file missing")
	}
}
func TestPagedSummaryOmitsFullRequirementsButDetailPreservesThem(t *testing.T) {
	project := t.TempDir()
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	body := "# A-1 Preview\n## 작업 정의\n### 목표\n" + strings.Repeat("목표", 1000) + "\n### 수용 기준\n" + strings.Repeat("유지", 1000) + "\n"
	writeWebTask(t, folder, "000001.A-1.preview.todo.md", body)
	page, err := BacklogPage(project, folder, 1, 20, nil, nil, "", "id_desc")
	if err != nil {
		t.Fatal(err)
	}
	req := page["items"].([]map[string]any)[0]["document"].(map[string]any)["requirements"].(map[string]any)
	if len([]rune(req["goal"].(string))) > 601 || len(req) != 1 {
		t.Fatalf("unbounded list document: %v", req)
	}
	detail, err := TaskDetail(project, folder, "A-1")
	if err != nil {
		t.Fatal(err)
	}
	if len([]rune(detail["document"].(map[string]any)["requirements"].(map[string]any)["goal"].(string))) != 2000 {
		t.Fatal("detail goal changed")
	}
}

func TestRequestRuntimePreservesConsumerTransitionLimit(t *testing.T) {
	now := time.Now()
	transitions := make([]runtimeobs.Transition, 25)
	for i := range transitions {
		transitions[i] = runtimeobs.Transition{At: now.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano), State: runtimeobs.StateRunning}
	}
	source := requestRuntime{ledger: runtimeobs.Ledger{Attempts: []runtimeobs.Attempt{{AttemptID: "exact", CurrentState: runtimeobs.StateWaitingUser, StartedAt: "start", EndedAt: "end", BindingState: runtimeobs.BindingBound, RecentTransitions: transitions}}}}
	got, err := readRequestRuntime("unused", 10, now, source)
	if err != nil || len(got.Attempts[0].RecentTransitions) != 10 || got.Attempts[0].RecentTransitions[0].At != transitions[15].At || len(source.ledger.Attempts[0].RecentTransitions) != 25 || got.Attempts[0].CurrentState != runtimeobs.StateWaitingUser || got.Attempts[0].StartedAt != "start" {
		t.Fatalf("limit changed identity/state: %+v", got)
	}
}
