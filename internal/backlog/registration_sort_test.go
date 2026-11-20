package backlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// These reports exist only inside TempDir. Writing an event as an external
// producer also tests metadata invalidation without process-local notifications.
func writeA95Registration(t *testing.T, project, eventID, taskID, kind, at string) {
	t.Helper()
	event := LifecycleTransition{EventID: eventID, TaskID: taskID, Kind: kind, OccurredAt: at, Actor: "/fixture/registrar", EvidenceSource: "registrar_report"}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	path := lifecycleTransitionPath(project, eventID)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func a95RegistrationFixture(t *testing.T) (string, string) {
	t.Helper()
	project := t.TempDir()
	root := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(filepath.Join(root, "archive", "2026-10"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, task := range []struct{ name, title string }{
		{"000900.A-900.old.done.md", "A-900"},
		{"archive/2026-10/000001.B-1.new.done.md", "B-1"},
		{"000002.A-2.tie.done.md", "A-2"},
		{"000003.C-2.tie.done.md", "C-2"},
		{"000008.D-8.legacy.done.md", "D-8"},
		{"000009.E-3.invalid.done.md", "E-3"},
	} {
		path := writeWebTask(t, root, task.name, "# "+task.title+" fixture\n- Tags: area:backend\n")
		stamp := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		if task.title == "A-900" {
			stamp = stamp.Add(24 * time.Hour)
		}
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	writeA95Registration(t, project, "old", "A-900", "registered", "2026-10-01T00:00:00.000000100Z")
	writeA95Registration(t, project, "new", "B-1", "registered", "2026-10-01T00:00:00.000000300Z")
	writeA95Registration(t, project, "tie-offset", "A-2", "registered", "2026-10-01T09:00:00.000000200+09:00")
	writeA95Registration(t, project, "tie-utc", "C-2", "registered", "2026-10-01T00:00:00.000000200Z")
	writeA95Registration(t, project, "invalid", "E-3", "registered", "invalid")
	writeA95Registration(t, project, "assigned-only", "D-8", "assigned", "2026-10-02T00:00:00Z")
	writeA95Registration(t, project, "foreign", "Z-404", "registered", "2026-10-03T00:00:00Z")
	writeA95Registration(t, project, "repeat", "A-900", "registered", "2026-10-04T00:00:00Z")
	if _, err := EnsureTagRegistry(project); err != nil {
		t.Fatal(err)
	}
	return project, root
}

func a95IDs(items []map[string]any) []string {
	ids := []string{}
	for _, item := range items {
		ids = append(ids, toString(item["id"]))
	}
	return ids
}

func a95IndexBuilds(project string) uint64 {
	registrationIndexes.Lock()
	defer registrationIndexes.Unlock()
	return registrationIndexes.entries[project].builds
}

func TestA95RegistrationPagesAndExplicitSorts(t *testing.T) {
	project, root := a95RegistrationFixture(t)
	want := []string{"B-1", "A-2", "C-2", "A-900", "D-8", "E-3"}
	for _, key := range []string{"", "invalid", "recently_registered"} {
		got := []string{}
		for page := 1; page <= 3; page++ {
			data, err := BacklogPage(project, root, page, 2, nil, nil, "", key)
			if err != nil {
				t.Fatal(err)
			}
			items := data["items"].([]map[string]any)
			if len(items) != 2 || data["total"] != 6 {
				t.Fatalf("pagination: %+v", data)
			}
			got = append(got, a95IDs(items)...)
			for _, item := range items {
				if item["id"] == "D-8" || item["id"] == "E-3" {
					if _, ok := item["registration_sort_key"]; ok {
						t.Fatalf("invented registration: %+v", item)
					}
				}
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("sort %q: %v want %v", key, got, want)
		}
	}
	for key, first := range map[string]string{"id_desc": "A-900", "id_asc": "B-1", "updated_desc": "A-900", "updated_asc": "B-1"} {
		data, err := BacklogPage(project, root, 1, 1, nil, nil, "", key)
		if err != nil {
			t.Fatal(err)
		}
		if got := data["items"].([]map[string]any)[0]["id"]; got != first {
			t.Fatalf("manual %s first=%v", key, got)
		}
	}
	// Filter and search a complete scope before paging, including archived rows.
	data, err := BacklogPage(project, root, 1, 1, []string{"done"}, []string{"area:backend"}, "fixture", "recently_registered")
	if err != nil {
		t.Fatal(err)
	}
	item := data["items"].([]map[string]any)[0]
	if data["total"] != 6 || item["id"] != "B-1" || item["location"] != "archive" {
		t.Fatalf("scope: %+v", data)
	}
	keys, err := registrationIndex(project)
	if err != nil {
		t.Fatal(err)
	}
	if keys["A-2"] != keys["C-2"] || keys["B-1"] != "2026-10-01T00:00:00.000000300Z" || keys["A-900"] != "2026-10-01T00:00:00.000000100Z" {
		t.Fatalf("exact timestamps: %v", keys)
	}
	if builds := a95IndexBuilds(project); builds != 1 {
		t.Fatalf("per-page event rescans: %d", builds)
	}
}

func TestA95RegistrationExternalAddAndInterruptedPublication(t *testing.T) {
	project, root := a95RegistrationFixture(t)
	prior, err := registrationIndex(project)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an external writer after a warm snapshot, without invalidation API.
	path := lifecycleTransitionPath(project, "external")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	partial, err := registrationIndex(project)
	if err != nil {
		t.Fatal(err)
	}
	if partial["D-8"] != "" {
		t.Fatal("incomplete publication became a registration")
	}
	writeA95Registration(t, project, "external", "D-8", "registered", "2026-10-05T00:00:00Z")
	data, err := BacklogPage(project, root, 1, 1, nil, nil, "", "recently_registered")
	if err != nil {
		t.Fatal(err)
	}
	if data["items"].([]map[string]any)[0]["id"] != "D-8" {
		t.Fatalf("external addition ignored: %+v", data)
	}
	if prior["D-8"] != "" {
		t.Fatal("published index was mutated")
	}
	if builds := a95IndexBuilds(project); builds != 2 {
		t.Fatalf("cache rebuilds=%d", builds)
	}
	other := t.TempDir()
	foreign, err := registrationIndex(other)
	if err != nil || len(foreign) != 0 {
		t.Fatalf("project leak: %v %v", foreign, err)
	}
}
