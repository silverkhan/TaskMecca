package main

import (
    "bytes"
    "encoding/json"
    "os"
    "path/filepath"
    "testing"

    "github.com/silverkhan/TaskMecca/internal/maintenance"
    "github.com/silverkhan/TaskMecca/internal/webui"
    "net/http"
    "net/http/httptest"
)

// Fresh installs must become visible in an already-running Hub without
// requiring task-mecca web to register the active directory itself.
func TestFirstInitRegistersInWebHub(t *testing.T) {
    base := t.TempDir()
    t.Setenv("TASK_MECCA_HOME", filepath.Join(base, "management"))
    project := filepath.Join(base, "first project")
    if err := os.MkdirAll(project, 0755); err != nil { t.Fatal(err) }

    // README onboarding starts Web before init. Opening the Web must not
    // create _task_mecca/.runtime or otherwise block first installation.
    handler, err := webui.Handler(project, "", "test")
    if err != nil { t.Fatal(err) }
    if _, err := os.Stat(filepath.Join(project, "_task_mecca")); !os.IsNotExist(err) {
        t.Fatalf("Web startup created an uninitialized framework: %v", err)
    }

    if code := run([]string{"init", "--project", project}); code != 0 {
        t.Fatalf("fresh init after Web startup returned %d", code)
    }
    states, err := maintenance.ManagedProjects()
    if err != nil || len(states) != 1 || states[0].Path != project || !states[0].Monitoring {
        t.Fatalf("initial registration not monitoring: %#v, %v", states, err)
    }
    // Query the already-created handler to prove the running Web can
    // discover the newly initialized project without a restart.
    for _, path := range []string{"/api/hub", "/api/hub/projects"} {
        rec := httptest.NewRecorder()
        handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
        if rec.Code != http.StatusOK { t.Fatalf("%s status=%d: %s", path, rec.Code, rec.Body.String()) }
        var body struct {
            Projects []struct { Path string `json:"path"` } `json:"projects"`
        }
        if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil { t.Fatal(err) }
        if len(body.Projects) != 1 || body.Projects[0].Path != project {
            t.Fatalf("%s did not discover new project: %s", path, rec.Body.String())
        }
    }
}

func TestInitRegistrationFailureLeavesFrameworkAndSupportsExplicitRepair(t *testing.T) {
    base := t.TempDir()
    home := filepath.Join(base, "management")
    t.Setenv("TASK_MECCA_HOME", home)
    if err := os.MkdirAll(home, 0755); err != nil { t.Fatal(err) }
    registry := filepath.Join(home, "projects.json")
    broken := []byte("{broken")
    if err := os.WriteFile(registry, broken, 0600); err != nil { t.Fatal(err) }
    project := filepath.Join(base, "recovery project")
    if err := os.MkdirAll(project, 0755); err != nil { t.Fatal(err) }

    if code := run([]string{"init", "--project", project}); code == 0 {
        t.Fatal("initialization must not report success if registry registration failed")
    }
    manifest := filepath.Join(project, "_task_mecca", "manifest.json")
    if _, err := os.Stat(manifest); err != nil {
        t.Fatalf("failed registration must preserve installed framework: %v", err)
    }
    before, err := os.ReadFile(registry)
    if err != nil || !bytes.Equal(before, broken) {
        t.Fatalf("failed registration must not overwrite invalid registry: %q %v", before, err)
    }

    // A second init must not overwrite an existing framework, including
    // customized user files. Repair is an explicit projects add.
    if code := run([]string{"init", "--project", project}); code == 0 {
        t.Fatal("second init must not reinstall over existing files")
    }
    if err := os.WriteFile(registry, []byte("{}\n"), 0600); err != nil { t.Fatal(err) }
    var output, errors bytes.Buffer
    if code := runProjectCLI([]string{"add", "--path", project}, &output, &errors); code != 0 {
        t.Fatalf("explicit safe registration recovery failed: code=%d err=%s", code, errors.String())
    }
    states, err := maintenance.ManagedProjects()
    if err != nil || len(states) != 1 || !states[0].Monitoring {
        t.Fatalf("recovery did not restore Hub monitoring: %#v %v", states, err)
    }
}

func TestRepeatedInitCannotResumePausedProject(t *testing.T) {
    base := t.TempDir()
    t.Setenv("TASK_MECCA_HOME", filepath.Join(base, "management"))
    project := filepath.Join(base, "paused")
    if err := os.MkdirAll(project, 0755); err != nil { t.Fatal(err) }
    if code := run([]string{"init", "--project", project}); code != 0 { t.Fatal(code) }
    if err := maintenance.SetProjectMonitoring(project, false); err != nil { t.Fatal(err) }
    if code := run([]string{"init", "--project", project}); code == 0 {
        t.Fatal("repeated init unexpectedly succeeded")
    }
    states, err := maintenance.ManagedProjects()
    if err != nil || len(states) != 1 || states[0].Monitoring {
        t.Fatalf("repeated init resumed deliberately paused project: %#v %v", states, err)
    }
}
