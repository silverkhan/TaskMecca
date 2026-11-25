package backlog

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentNotificationsCanonicalStatesPreserveHistory(t *testing.T) {
	project := t.TempDir()
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	writeWebTask(t, folder, "000001.A-1.done.done.md", "# A-1 Completed\n")
	writeWebTask(t, filepath.Join(folder, "archive", "2026-11"), "000002.A-2.archived.done.md", "# A-2 Archived\n")
	writeWebTask(t, folder, "000003.A-3.todo.todo.md", "# A-3 Todo\n")
	writeWebTask(t, folder, "000004.A-4.hold.hold.md", "# A-4 Hold\n")
	writeWebTask(t, folder, "000005.A-5.worker.doing.md", "# A-5 Worker\n- Agent: /root/controller/worker\n")
	runtime := filepath.Join(project, "_task_mecca", ".runtime")
	if err := os.MkdirAll(filepath.Join(runtime, "agents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime, "agents", "worker.json"), []byte(`{"agent":"/root/controller/worker","task_id":"A-5","state":"completed","heartbeat_at":"2026-11-25T00:00:00Z"}`), 0644); err != nil {
		t.Fatal(err)
	}
	events := []map[string]any{}
	for _, kind := range []string{"registered", "started", "completed", "intervention", "approval", "stalled", "runtime_unknown"} {
		events = append(events, map[string]any{"id": "retained-" + kind, "task_id": "A-1", "kind": kind, "at": "2026-11-24T00:00:00Z", "reason_type": "user_intervention"})
	}
	events = append(events, map[string]any{"id": "archive-started", "task_id": "A-2", "kind": "started", "at": "2026-11-24T00:00:00Z"}, map[string]any{"id": "unknown-task", "task_id": "missing", "kind": "started"}, map[string]any{"id": "system", "kind": "system"})
	journal, _ := json.Marshal(notificationJournal{Version: notificationJournalVersion, Items: map[string]notificationObservation{}, Phases: map[string]int{}, Events: events})
	journalPath := filepath.Join(runtime, "notification_events.json")
	if err := os.WriteFile(journalPath, journal, 0644); err != nil {
		t.Fatal(err)
	}
	deliveryPath := filepath.Join(runtime, "notification_delivery_evidence.json")
	delivery := []byte("{\"retained-completed\":{\"telegram\":\"sent\",\"web\":\"display_requested\"}}\n")
	if err := os.WriteFile(deliveryPath, delivery, 0644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := AttentionSnapshot(project, folder, false)
	if err != nil {
		t.Fatal(err)
	}
	states := snapshot["current_task_states"].(map[string]string)
	for id, want := range map[string]string{"A-1": "done", "A-2": "done", "A-3": "todo", "A-4": "hold", "A-5": "doing"} {
		if states[id] != want {
			t.Errorf("canonical %s=%q want %q", id, states[id], want)
		}
	}
	if _, ok := states["missing"]; ok {
		t.Fatal("unknown task acquired a canonical state")
	}
	gotEvents, _ := json.Marshal(snapshot["notification_events"])
	wantEvents, _ := json.Marshal(events)
	if !bytes.Equal(gotEvents, wantEvents) {
		t.Fatalf("raw delivery events changed: %s", gotEvents)
	}
	for path, want := range map[string][]byte{journalPath: journal, deliveryPath: delivery} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("history bytes changed: %s", path)
		}
	}
	other := t.TempDir()
	otherFolder := filepath.Join(other, "_task_mecca", "data", "backlog")
	writeWebTask(t, otherFolder, "000001.A-1.same.doing.md", "# A-1 Other project\n")
	otherSnapshot, err := AttentionSnapshot(other, otherFolder, false)
	if err != nil {
		t.Fatal(err)
	}
	if otherSnapshot["current_task_states"].(map[string]string)["A-1"] != "doing" {
		t.Fatal("same ID inherited another project's completion")
	}
}

func TestCurrentNotificationsPagedListIncludesOffPageArchiveState(t *testing.T) {
	project := t.TempDir()
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	writeWebTask(t, folder, "000002.A-2.active.doing.md", "# A-2 Active\n")
	writeWebTask(t, filepath.Join(folder, "archive", "2026-11"), "000001.A-1.archived.done.md", "# A-1 Archived\n")
	page, err := BacklogPage(project, folder, 1, 1, []string{"doing"}, nil, "", "id_desc")
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if len(items) != 1 || items[0]["id"] != "A-2" {
		t.Fatalf("paged items: %+v", items)
	}
	states := page["current_task_states"].(map[string]string)
	if states["A-1"] != "done" || states["A-2"] != "doing" {
		t.Fatalf("incomplete canonical projection: %+v", states)
	}
}
