package maintenance

import (
	"encoding/json"
	"reflect"
	"github.com/silverkhan/TaskMecca/internal/projectguard"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryWritersPreserveLegacyRemovalFactsAndUnknownFields(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	legacy := filepath.Join(t.TempDir(), "legacy-removed")
	project := filepath.Join(t.TempDir(), "active")
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca"), 0700); err != nil { t.Fatal(err) }
	seed := map[string]any{
		"projects": []any{},
		"paused_projects": []string{legacy},
		"removal_history": []any{map[string]any{"id":"A25-evidence", "path":legacy, "trash_path":"historic-trash-fact", "source_present":false, "user_extension":map[string]any{"keep":true}}},
		"user_unknown": map[string]any{"nested":[]any{"keep", float64(42)}},
	}
	data, _ := json.Marshal(seed)
	if err := os.WriteFile(registryPath(), data, 0600); err != nil { t.Fatal(err) }
	verify := func() {
		t.Helper()
		data, err := os.ReadFile(registryPath()); if err != nil { t.Fatal(err) }
		var got map[string]any; if err := json.Unmarshal(data, &got); err != nil { t.Fatal(err) }
		for _, key := range []string{"removal_history", "user_unknown"} { if !reflect.DeepEqual(got[key], seed[key]) { t.Fatalf("%s evidence lost: %#v", key, got[key]) } }
		if projectguard.Allowed(legacy) { t.Fatal("legacy suppression lost") }
		paused := got["paused_projects"].([]any); found := false
		for _, path := range paused { if path == legacy { found = true } }
		if !found { t.Fatal("unrelated paused boundary lost") }
	}
	if err := RegisterProject(project); err != nil { t.Fatal(err) }; verify()
	if err := SetProjectMonitoring(project, false); err != nil { t.Fatal(err) }; verify()
	if err := SetProjectMonitoring(project, true); err != nil { t.Fatal(err) }; verify()
	item, err := ArchiveProject(project); if err != nil { t.Fatal(err) }; verify()
	if err := RestoreArchivedProject(item.ID, project); err != nil { t.Fatal(err) }; verify()
	item, err = ArchiveProject(project); if err != nil { t.Fatal(err) }; verify()
	if err := ForgetArchivedProject(item.ID, project); err != nil { t.Fatal(err) }; verify()
	if err := RegisterProject(project); err != nil { t.Fatal(err) }; verify()
}

func TestArchiveRestoreForgetAndExplicitAddPreserveFiles(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	root := t.TempDir()
	project := filepath.Join(root, "project")
	os.MkdirAll(filepath.Join(project, "_task_mecca"), 0700)
	file := filepath.Join(project, "private.txt")
	os.WriteFile(file, []byte("keep"), 0600)
	if ProjectMonitoringAllowed(project) {
		t.Fatal("unregistered project monitored")
	}
	RegisterWebProject(project)
	if ProjectMonitoringAllowed(project) || len(ListProjects()) != 0 {
		t.Fatal("Web startup registered or monitored project")
	}
	if err := RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	if !ProjectMonitoringAllowed(project) {
		t.Fatal("explicit add not monitored")
	}
	item, err := ArchiveProject(project)
	if err != nil {
		t.Fatal(err)
	}
	if ProjectMonitoringAllowed(project) || projectguard.Allowed(project) {
		t.Fatal("archive not suppressed")
	}
	RegisterWebProject(project)
	if len(ListProjects()) != 0 {
		t.Fatal("restart restored archive")
	}
	if err := RestoreArchivedProject(item.ID, project); err != nil {
		t.Fatal(err)
	}
	if !ProjectMonitoringAllowed(project) {
		t.Fatal("restore did not reactivate")
	}
	RegisterProject(project)
	if len(ListProjects()) != 1 {
		t.Fatal("duplicate registration")
	}
	item, err = ArchiveProject(project)
	if err != nil {
		t.Fatal(err)
	}
	if err := ForgetArchivedProject(item.ID, project); err != nil {
		t.Fatal(err)
	}
	RegisterWebProject(project)
	if ProjectMonitoringAllowed(project) || projectguard.Allowed(project) || len(ListProjects()) != 0 {
		t.Fatal("forget/restart lost suppression")
	}
	if err := RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	if !ProjectMonitoringAllowed(project) {
		t.Fatal("explicit add cannot recover forgotten entry")
	}
	bytes, err := os.ReadFile(file)
	if err != nil || string(bytes) != "keep" {
		t.Fatal("management changed project files", err)
	}
}

func TestArchiveRestoreRejectsSubstitutedAncestor(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	root := t.TempDir()
	holder := filepath.Join(root, "holder")
	project := filepath.Join(holder, "project")
	os.MkdirAll(filepath.Join(project, "_task_mecca"), 0700)
	RegisterProject(project)
	item, err := ArchiveProject(project)
	if err != nil {
		t.Fatal(err)
	}
	preserved := filepath.Join(root, "preserved")
	if err := os.Rename(holder, preserved); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	os.MkdirAll(filepath.Join(outside, "project", "_task_mecca"), 0700)
	if err := os.Symlink(outside, holder); err != nil {
		t.Skip(err)
	}
	if err := RestoreArchivedProject(item.ID, project); err == nil {
		t.Fatal("substituted ancestor restored unrelated project")
	}
	if !projectguard.Allowed(filepath.Join(outside, "project")) {
		t.Fatal("historical tombstone retargeted unrelated project")
	}
}
