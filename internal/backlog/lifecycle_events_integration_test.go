package backlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGitlessLifecycleSurvivesRefreshWithoutProvisionalEvents(t *testing.T) {
	project := t.TempDir() // No Git repository, Web server or Hook setup.
	backlogDir := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(backlogDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backlogDir, "000001.B-1.direct.done.md"), []byte("# B-1 Direct lifecycle\n- Agent: /root/controller/worker\n"), 0644); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	inputs := []LifecycleTransition{
		{EventID: "b1-register", TaskID: "B-1", Kind: "registered", Actor: "/root/registrar", EvidenceSource: "registrar_report"},
		{EventID: "b1-assign", TaskID: "B-1", Kind: "assigned", Actor: "/root/controller", EvidenceSource: "controller_report", AssignmentID: "assignment-b1"},
		{EventID: "b1-start", TaskID: "B-1", Kind: "started", Actor: "/root/controller/worker", EvidenceSource: "worker_report", AssignmentID: "assignment-b1"},
		{EventID: "b1-wait", TaskID: "B-1", Kind: "waiting", Actor: "/root/controller/worker", EvidenceSource: "worker_report"},
		{EventID: "b1-resume", TaskID: "B-1", Kind: "resumed", Actor: "/root/controller", EvidenceSource: "controller_report"},
		{EventID: "b1-complete", TaskID: "B-1", Kind: "completed", Actor: "/root/controller", EvidenceSource: "controller_report"},
	}
	for i, input := range inputs {
		input.OccurredAt = base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339Nano)
		inputs[i].OccurredAt = input.OccurredAt
		if _, err := RecordLifecycleTransition(project, input, base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	read := func() map[string]any {
		t.Helper()
		timings, err := LifecycleTimings(project, "")
		if err != nil {
			t.Fatal(err)
		}
		row := timings["B-1"]
		if row == nil {
			t.Fatal("missing B-1 lifecycle")
		}
		return row
	}
	first := read()
	second := read() // Simulate a fresh consumer reading durable project data.
	for _, row := range []map[string]any{first, second} {
		events, ok := row["events"].([]map[string]any)
		if !ok {
			t.Fatalf("missing lifecycle events: %+v", row)
		}
		if len(events) != len(inputs) {
			t.Fatalf("lost or duplicated lifecycle phases: %+v", events)
		}
		for i, event := range events {
			if event["event_id"] != inputs[i].EventID || event["provisional"] != false || event["source"] != "durable_lifecycle" {
				t.Fatalf("canonical event changed across refresh: %+v", events)
			}
		}
		if row["lifecycle_inferred"] != false || row["started_at"] != inputs[2].OccurredAt || row["completed_at"] != inputs[5].OccurredAt {
			t.Fatalf("durable completion was made provisional or retimed: %+v", row)
		}
	}
}

func TestAssignedWithoutWorkerHasNoStartWithoutGitOrHook(t *testing.T) {
	project := t.TempDir()
	backlogDir := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(backlogDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backlogDir, "000001.B-1.assigned.doing.md"), []byte("# B-1 Assignment only\n- Agent: /root/controller/worker\n"), 0644); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Minute)
	for _, event := range []LifecycleTransition{
		{EventID: "b1-register", TaskID: "B-1", Kind: "registered", Actor: "/root/registrar", EvidenceSource: "registrar_report", OccurredAt: at.Format(time.RFC3339Nano)},
		{EventID: "b1-assign", TaskID: "B-1", Kind: "assigned", Actor: "/root/controller", EvidenceSource: "controller_report", OccurredAt: at.Add(time.Second).Format(time.RFC3339Nano)},
	} {
		if _, err := RecordLifecycleTransition(project, event, at); err != nil {
			t.Fatal(err)
		}
	}
	timings, err := LifecycleTimings(project, "")
	if err != nil {
		t.Fatal(err)
	}
	row := timings["B-1"]
	if row["started_at"] != nil {
		t.Fatalf("doing file became fake Worker start: %+v", row)
	}
	events := row["events"].([]map[string]any)
	if len(events) != 2 || events[1]["kind"] != "assigned" {
		t.Fatalf("assignment evidence lost: %+v", events)
	}
}

func TestDurableLifecycleDrivesNotificationIDsWithoutWebOrHook(t *testing.T) {
	project := t.TempDir()
	if _, err := NotificationEvents(project, map[string]map[string]any{}); err != nil {
		t.Fatal(err)
	} // Establish delivery baseline without Web.
	base := time.Now().UTC().Add(-time.Minute)
	phases := []struct{ kind, state string }{
		{"registered", "todo"}, {"assigned", "doing"}, {"started", "doing"}, {"waiting", "hold"}, {"resumed", "doing"}, {"completed", "done"},
	}
	var last []map[string]any
	for i, phase := range phases {
		event := LifecycleTransition{EventID: "event-" + phase.kind, TaskID: "B-1", Kind: phase.kind,
			OccurredAt: base.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano), Actor: "/root/controller", EvidenceSource: "controller_report"}
		if phase.kind == "started" {
			event.Actor = "/root/controller/worker"
			event.EvidenceSource = "worker_report"
		}
		if _, err := RecordLifecycleTransition(project, event, base.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
		lifecycle := map[string]any{"created_at": base.Format(time.RFC3339Nano), "events": []map[string]any{}}
		events := lifecycle["events"].([]map[string]any)
		for _, prior := range phases[:i+1] {
			events = append(events, map[string]any{"kind": prior.kind, "source": "durable_lifecycle", "event_id": "event-" + prior.kind})
		}
		lifecycle["events"] = events
		if i >= 2 {
			lifecycle["started_at"] = base.Add(2 * time.Second).Format(time.RFC3339Nano)
		}
		if phase.kind == "completed" {
			lifecycle["completed_at"] = event.OccurredAt
		}
		items := map[string]map[string]any{"B-1": {"file_state": phase.state, "updated_at": event.OccurredAt, "title": "B-1", "lifecycle": lifecycle}}
		var err error
		last, err = NotificationEvents(project, items)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(last) != 3 {
		t.Fatalf("want registered, started, completed only once: %+v", last)
	}
	for _, event := range last {
		kind := toString(event["kind"])
		if toString(event["lifecycle_event_id"]) != "event-"+kind || toString(event["id"]) != notificationEventID("B-1", kind, "event-"+kind) {
			t.Fatalf("notification diverged from canonical event: %+v", event)
		}
	}
	repeated, err := ReadNotificationEvents(project)
	if err != nil || len(repeated) != len(last) {
		t.Fatalf("notification journal changed after restart read: %+v %v", repeated, err)
	}
}

func TestUncommittedGitTransitionsAndDoneOnlyCommitDoNotDuplicateCanonicalEvents(t *testing.T) {
	project := t.TempDir()
	gitRun(t, project, "init")
	gitRun(t, project, "config", "user.email", "ci@example.invalid")
	gitRun(t, project, "config", "user.name", "CI")
	backlogDir := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(backlogDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(backlogDir, "000001.B-1.only-done.done.md")
	if err := os.WriteFile(path, []byte("# B-1 Only done in Git\n"), 0644); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(-3 * time.Minute)
	kinds := []string{"registered", "assigned", "started", "completed"}
	for i, kind := range kinds {
		event := LifecycleTransition{EventID: "direct-" + kind, TaskID: "B-1", Kind: kind,
			OccurredAt: base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339Nano),
			Actor:      "/root/controller", EvidenceSource: "controller_report"}
		if kind == "started" {
			event.Actor = "/root/controller/worker"
			event.EvidenceSource = "worker_report"
		}
		if _, err := RecordLifecycleTransition(project, event, base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	before, err := LifecycleTimings(project, "")
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, project, "add", path)
	gitRun(t, project, "commit", "-m", "Only final done recorded")
	after, err := LifecycleTimings(project, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []map[string]any{before["B-1"], after["B-1"]} {
		events := row["events"].([]map[string]any)
		if len(events) != len(kinds) {
			t.Fatalf("Git introduced duplicate or missing canonical phase: %+v", events)
		}
		for i, event := range events {
			if event["event_id"] != "direct-"+kinds[i] || event["provisional"] != false {
				t.Fatalf("Git replaced canonical evidence: %+v", events)
			}
		}
		if row["started_at"] != base.Add(2*time.Minute).Format(time.RFC3339Nano) {
			t.Fatalf("Git done-only history retimed start: %+v", row)
		}
	}
}

func TestInterruptedFileTransitionRemainsExplicitlyUncertain(t *testing.T) {
	project := t.TempDir()
	backlogDir := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(backlogDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backlogDir, "000001.B-1.not-yet-renamed.doing.md"), []byte("# B-1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	event := LifecycleTransition{EventID: "completion-before-rename", TaskID: "B-1", Kind: "completed", Actor: "/root/controller", EvidenceSource: "controller_report"}
	if _, err := RecordLifecycleTransition(project, event, now); err != nil {
		t.Fatal(err)
	}
	timings, err := LifecycleTimings(project, "")
	if err != nil {
		t.Fatal(err)
	}
	row := timings["B-1"]
	if row["consistency_status"] != "event_file_mismatch" || row["file_observation"] != "doing" || row["last_lifecycle_event_id"] != event.EventID {
		t.Fatalf("interrupted transition was silently reconciled: %+v", row)
	}
}
