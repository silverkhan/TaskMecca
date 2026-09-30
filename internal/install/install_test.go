package install

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
)

func TestInitAndPreserveData(t *testing.T) {
    root := t.TempDir()
    if err := Init(root, "0.2.1"); err != nil { t.Fatal(err) }
    target := filepath.Join(root, targetName)
    if _, err := os.Stat(filepath.Join(target, "data")); !os.IsNotExist(err) {
        t.Fatalf("init must not create project data: %v", err)
    }
    err := filepath.WalkDir(target, func(path string, entry os.DirEntry, walkErr error) error {
        if walkErr != nil { return walkErr }
        if !entry.IsDir() && filepath.Ext(path) == ".py" {
            t.Fatalf("standalone Go install must not contain Python runtime files: %s", path)
        }
        return nil
    })
    if err != nil { t.Fatal(err) }
    file := filepath.Join(target, "data", "backlog", "000001.A-1.task.todo.md")
    if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil { t.Fatal(err) }
    if err := os.WriteFile(file, []byte("# A-1\n"), 0644); err != nil { t.Fatal(err) }
    if err := Migrate(root, "0.2.1"); err != nil { t.Fatal(err) }
    if got, err := os.ReadFile(file); err != nil || string(got) != "# A-1\n" {
        t.Fatalf("project data changed: %q %v", got, err)
    }
}

func TestMigrateStopsOnModifiedFramework(t *testing.T) {
    root := t.TempDir()
    if err := Init(root, "0.2.1"); err != nil { t.Fatal(err) }
    path := filepath.Join(root, targetName, "framework", "web", "app.js")
    if err := os.WriteFile(path, []byte("local change"), 0644); err != nil { t.Fatal(err) }
    // Unchanged upstream managed files are preserved, matching the Python updater.
    if err := Migrate(root, "0.2.1"); err != nil { t.Fatal(err) }
    got, err := os.ReadFile(path)
    if err != nil || string(got) != "local change" { t.Fatalf("local customization lost: %q %v", got, err) }
}

func TestMigrateRestoresMissingManagedFile(t *testing.T) {
    project := t.TempDir()
    if err := Init(project, "0.2.0"); err != nil {
        t.Fatal(err)
    }
    target := filepath.Join(project, "_task_mecca", "framework", "_template.md")
    if err := os.Remove(target); err != nil {
        t.Fatal(err)
    }
    if err := Migrate(project, "0.2.1"); err != nil {
        t.Fatal(err)
    }
    if _, err := os.Stat(target); err != nil {
        t.Fatalf("missing managed file was not restored: %v", err)
    }
}


func TestMigrateReportsInstructionRefreshWhenSessionGuideChanges(t *testing.T) {
    project := t.TempDir()
    if err := Init(project, "0.2.0"); err != nil { t.Fatal(err) }

    target := filepath.Join(project, targetName)
    guidePath := filepath.Join(target, "framework", "SESSION_GUIDE.md")
    oldGuide := []byte("# Previous Session Guide\n")
    if err := os.WriteFile(guidePath, oldGuide, 0644); err != nil { t.Fatal(err) }

    manifestPath := filepath.Join(target, "manifest.json")
    raw, err := os.ReadFile(manifestPath)
    if err != nil { t.Fatal(err) }
    var m manifest
    if err = json.Unmarshal(raw, &m); err != nil { t.Fatal(err) }
    entry := m.Managed["framework/SESSION_GUIDE.md"]
    entry.BaselineSHA256 = hash(oldGuide)
    m.Managed["framework/SESSION_GUIDE.md"] = entry
    if err = saveManifest(target, m); err != nil { t.Fatal(err) }

    result, err := MigrateWithResult(project, "0.2.1")
    if err != nil { t.Fatal(err) }
    if !result.InstructionRefreshRequired {
        t.Fatal("instruction refresh should be required when SESSION_GUIDE changes")
    }
    found := false
    for _, path := range result.ChangedInstructions {
        if path == "framework/SESSION_GUIDE.md" { found = true }
    }
    if !found { t.Fatalf("changed instructions missing session guide: %#v", result.ChangedInstructions) }
}
