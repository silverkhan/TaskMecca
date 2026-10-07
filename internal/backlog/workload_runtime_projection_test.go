package backlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestWorkloadLinksBoundAttemptToObservedSessionName(t *testing.T) {
	project := t.TempDir()
	backlogDir := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(backlogDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backlogDir, "000455.B-455.observe.doing.md"), []byte("# B-455 Observe\n- Agent: /root/controller/pikachyu\n"), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 7, 1, 0, 0, 0, time.UTC)
	attempt := runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: "run-b455", Provider: "codex", SessionID: "root-b455", SessionName: "EMPFUND B-455", RuntimeAgentID: "agent-b455", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}
	if err := runtimeobs.AppendExecutionEvent(project, attempt); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.BindAttempt(project, attempt.AttemptID, "B-455", "/root/controller/pikachyu", "dispatch", "", map[string]string{"assignment_id": "assignment-b455"}, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	snapshot, err := WorkloadSnapshot(project, "")
	if err != nil {
		t.Fatal(err)
	}
	item := snapshot["all_items"].(map[string]map[string]any)["B-455"]
	runtime := item["runtime_attempt"].(map[string]any)
	if runtime["attempt_id"] != "run-b455" || runtime["session_name"] != "EMPFUND B-455" {
		t.Fatalf("runtime projection=%+v", runtime)
	}
}

func TestWorkloadDoesNotInventSessionForDispatchAwaitingFirstHook(t *testing.T) {
	project := t.TempDir()
	backlogDir := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(backlogDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backlogDir, "000455.B-455.dispatch.doing.md"), []byte("# B-455 Dispatch\n- Agent: /root/controller/pikachyu\n"), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 7, 1, 0, 0, 0, time.UTC)
	if _, err := runtimeobs.BindRuntimeAgent(project, "/root/controller/pikachyu", "B-455", "/root/controller/pikachyu", "", now); err != nil {
		t.Fatal(err)
	}

	snapshot, err := WorkloadSnapshot(project, "")
	if err != nil {
		t.Fatal(err)
	}
	item := snapshot["all_items"].(map[string]map[string]any)["B-455"]
	if _, found := item["runtime_attempt"]; !found {
		t.Fatalf("bound dispatch attempt disappeared: %+v", item)
	}
	if name := item["runtime_attempt"].(map[string]any)["session_name"]; name != nil && name != "" {
		t.Fatalf("unobserved dispatch invented session name: %+v", item["runtime_attempt"])
	}
	if health := item["activity"].(map[string]any)["health"]; health != "awaiting_start" {
		t.Fatalf("health=%v want awaiting_start", health)
	}
}
