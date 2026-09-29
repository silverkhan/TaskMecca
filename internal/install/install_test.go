package install

import (
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
    file := filepath.Join(target, "data", "backlog", "000001.A-1.task.todo.md")
    if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil { t.Fatal(err) }
    if err := os.WriteFile(file, []byte("# A-1\n"), 0644); err != nil { t.Fatal(err) }
    if err := Update(root, "0.2.1"); err != nil { t.Fatal(err) }
    if got, err := os.ReadFile(file); err != nil || string(got) != "# A-1\n" {
        t.Fatalf("project data changed: %q %v", got, err)
    }
}

func TestUpdateStopsOnModifiedFramework(t *testing.T) {
    root := t.TempDir()
    if err := Init(root, "0.2.1"); err != nil { t.Fatal(err) }
    path := filepath.Join(root, targetName, "framework", "collab_tools.py")
    if err := os.WriteFile(path, []byte("local change"), 0644); err != nil { t.Fatal(err) }
    // Unchanged upstream managed files are preserved, matching the Python updater.
    if err := Update(root, "0.2.1"); err != nil { t.Fatal(err) }
    got, err := os.ReadFile(path)
    if err != nil || string(got) != "local change" { t.Fatalf("local customization lost: %q %v", got, err) }
}
