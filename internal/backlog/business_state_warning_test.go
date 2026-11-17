package backlog

import "testing"

// Pure projection checks: no runtime files, services, or canonical project writes.
func TestA82BusinessStateWarnings(t *testing.T) {
	for _, file := range []string{"todo", "doing", "hold", "done"} {
		for _, kind := range []string{"", "runtime_unknown", "runtime_stalled", "execution_interrupted", "approval_required", "controller_completion_review"} {
			reason := map[string]any{}
			if kind != "" {
				reason = map[string]any{"type": kind, "state": "controller_review", "message": "synthetic evidence"}
			}
			row := Record{ID: "B-643", State: file, Fields: map[string]string{}, Document: map[string]any{}}
			got := webSummaryItem(row, file, nil, reason, nil)
			snapshot := dashboardItem(row, file, nil, nil, nil)
			snapshot["state"] = effectiveStateFromControl(file, reason)
			if snapshot["business_state"] != got["business_state"] {
				t.Fatalf("snapshot/list mismatch: %#v / %#v", snapshot, got)
			}
			if got["business_state"] != file || got["file_state"] != file {
				t.Fatalf("%s/%s lost business state: %#v", file, kind, got)
			}
			if kind != "" && got["state"] != effectiveStateFromControl(file, reason) {
				t.Fatalf("legacy consumer state changed: %#v", got)
			}
			if kind != "" && got["attention_reason"] == nil {
				t.Fatal("warning evidence lost")
			}
			for _, filter := range []string{"doing", "hold", "done", "ready", "blocked"} {
				if stateMatches(got, []string{filter}) != (filter == file) {
					t.Fatalf("%s/%s filter %s: %#v", file, kind, filter, got)
				}
			}
		}
	}
	for _, workflow := range []string{"ready", "blocked"} {
		got := webSummaryItem(Record{State: "todo", Fields: map[string]string{}}, workflow, nil, map[string]any{"type": "runtime_unknown"}, nil)
		if got["business_state"] != workflow || !stateMatches(got, []string{workflow}) {
			t.Fatalf("validated workflow hidden: %#v", got)
		}
		if listBusinessState("doing", workflow) != "doing" {
			t.Fatal("workflow overrode source doing")
		}
	}
}

func TestA82BusinessStateListCounts(t *testing.T) {
	project := t.TempDir()
	folder := project + "/_task_mecca/data/backlog"
	writeWebTask(t, folder, "000643.B-643.fixture.doing.md", "# B-643 Synthetic fixture\n- Agent: -\n")
	writeWebTask(t, folder, "000644.B-644.fixture.todo.md", "# B-644 Ready\n")
	writeWebTask(t, folder, "000645.B-645.fixture.hold.md", "# B-645 Hold\n")
	writeWebTask(t, folder, "000646.B-646.fixture.done.md", "# B-646 Done\n")
	// An unlinked doing task produces the real warning projection in temp data.
	for _, tc := range []struct{ filter, key, business string }{{"doing", "working", "doing"}, {"ready", "ready", "ready"}, {"hold", "hold", "hold"}, {"done", "done", "done"}} {
		page, err := BacklogPage(project, folder, 1, 20, []string{tc.filter}, nil, "", "id_desc")
		if err != nil {
			t.Fatal(err)
		}
		items := page["items"].([]map[string]any)
		if len(items) != 1 || items[0]["business_state"] != tc.business {
			t.Fatalf("%s: %#v", tc.filter, page)
		}
		if page["counts"].(map[string]int)[tc.key] != 1 {
			t.Fatalf("count differs from filter: %#v", page["counts"])
		}
	}
}
