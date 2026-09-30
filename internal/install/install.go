package install

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/goassets"
)

const targetName = "_task_mecca"
const sourceRoot = "template/_task_mecca"

type managed struct {
    Policy string `json:"policy"`
    BaselineSHA256 string `json:"baseline_sha256"`
}

type manifest struct {
    Version string `json:"task_mecca_version"`
    Schema int `json:"schema_version"`
    Managed map[string]managed `json:"managed_files"`
    Owned []string `json:"project_owned_patterns"`
}

type MigrationResult struct {
    FromVersion string `json:"from_version"`
    ToVersion string `json:"to_version"`
    InstructionRefreshRequired bool `json:"instruction_refresh_required"`
    ChangedInstructions []string `json:"changed_instructions,omitempty"`
}

var owned = []string{"data/**", "backlog/**", "backlog_*/**", ".runtime/**", "backups/**", "config.toml"}

var customizable = map[string]bool{
    "ROOT_PROMPT.md": true, "framework/README.md": true,
    "framework/README.en.md": true, "framework/SESSION_GUIDE.md": true,
    "framework/SESSION_GUIDE.en.md": true, "framework/collab.md": true,
    "framework/_template.md": true,
}

func policy(path string) string {
    if customizable[path] || strings.HasPrefix(path, "framework/roles/") { return "customizable" }
    if path == "VERSION" { return "updater" }
    return "framework"
}

func hash(data []byte) string {
    digest := sha256.Sum256(data)
    return hex.EncodeToString(digest[:])
}

func bundled() (map[string][]byte, error) {
    files := map[string][]byte{}
    err := fs.WalkDir(goassets.Template, sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
        if walkErr != nil { return walkErr }
        if entry.IsDir() { return nil }
        rel := strings.TrimPrefix(path, sourceRoot + "/")
        if rel == "manifest.json" { return nil }
        data, err := goassets.Template.ReadFile(path)
        if err == nil { files[rel] = data }
        return err
    })
    return files, err
}

func newManifest(files map[string][]byte, version string) manifest {
    m := manifest{Version: version, Schema: 2, Managed: map[string]managed{}, Owned: owned}
    for path, data := range files { m.Managed[path] = managed{Policy: policy(path), BaselineSHA256: hash(data)} }
    return m
}

func saveManifest(target string, m manifest) error {
    data, err := json.MarshalIndent(m, "", "  ")
    if err != nil { return err }
    return os.WriteFile(filepath.Join(target, "manifest.json"), append(data, '\n'), 0644)
}

func write(target, rel string, data []byte) error {
    path := filepath.Join(target, filepath.FromSlash(rel))
    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil { return err }
    return os.WriteFile(path, data, 0644)
}

func fileHash(path string) string {
    data, err := os.ReadFile(path)
    if err != nil { return "" }
    return hash(data)
}

func Init(project, version string) error {
    target := filepath.Join(project, targetName)
    if entries, err := os.ReadDir(target); err == nil && len(entries) > 0 {
        return fmt.Errorf("%s already exists; use task-mecca migrate", target)
    } else if err != nil && !errors.Is(err, os.ErrNotExist) { return err }
    files, err := bundled()
    if err != nil { return result, err }
    if err = os.MkdirAll(target, 0755); err != nil { return err }
    for path, data := range files {
        if err = write(target, path, data); err != nil { return err }
    }
    return saveManifest(target, newManifest(files, version))
}

func sessionInstructionPath(path string) bool {
    switch path {
    case "ROOT_PROMPT.md", "framework/SESSION_GUIDE.md", "framework/SESSION_GUIDE.en.md", "framework/collab.md":
        return true
    }
    return strings.HasPrefix(path, "framework/roles/") && strings.HasSuffix(path, ".md")
}

func Migrate(project, version string) error {
    _, err := MigrateWithResult(project, version)
    return err
}

func MigrateWithResult(project, version string) (MigrationResult, error) {
    result := MigrationResult{ToVersion: version}
    target := filepath.Join(project, targetName)
    raw, err := os.ReadFile(filepath.Join(target, "manifest.json"))
    if err != nil { return result, fmt.Errorf("manifest not found; run task-mecca init first: %w", err) }
    var previous manifest
    if err = json.Unmarshal(raw, &previous); err != nil { return result, err }
    result.FromVersion = previous.Version
    if previous.Managed == nil { return result, errors.New("invalid Task Mecca manifest") }
    files, err := bundled()
    if err != nil { return result, err }
    incoming := newManifest(files, version)
    paths := make([]string, 0, len(previous.Managed)+len(files))
    seen := map[string]bool{}
    for path := range previous.Managed { paths = append(paths, path); seen[path] = true }
    for path := range files { if !seen[path] { paths = append(paths, path) } }
    sort.Strings(paths)
    conflicts := []string{}
    retired := []string{}
    changedInstructions := []string{}
    for _, path := range paths {
        old, existed := previous.Managed[path]
        data, exists := files[path]
        disk := fileHash(filepath.Join(target, filepath.FromSlash(path)))
        modified := existed && disk != old.BaselineSHA256
        changed := !existed || !exists || old.BaselineSHA256 != hash(data)
        if existed && !exists && !modified { retired = append(retired, path) }
        if changed && sessionInstructionPath(path) { changedInstructions = append(changedInstructions, path) }
        if modified && changed { conflicts = append(conflicts, path) }
    }
    if len(conflicts) > 0 {
        stamp := time.Now().Format("2006-01-02_150405")
        backup := filepath.Join(target, "backups", stamp)
        if err = os.MkdirAll(backup, 0755); err != nil { return result, err }
        copied := []string{}
        for _, path := range conflicts {
            data, readErr := os.ReadFile(filepath.Join(target, filepath.FromSlash(path)))
            if readErr == nil {
                if err = write(backup, path, data); err != nil { return result, err }
                copied = append(copied, path)
            }
        }
        meta, _ := json.MarshalIndent(map[string]any{"created_at": time.Now().Format(time.RFC3339), "from_version": previous.Version, "to_version": version, "files": copied}, "", "  ")
        if err = os.WriteFile(filepath.Join(backup, "backup.json"), append(meta, '\n'), 0644); err != nil { return result, err }
        return result, fmt.Errorf("local changes conflict with upstream: %s; backup: %s; migration stopped", strings.Join(conflicts, ", "), backup)
    }
    for _, path := range retired {
        if err = os.Remove(filepath.Join(target, filepath.FromSlash(path))); err != nil { return result, err }
    }
    for path, data := range files {
        old, existed := previous.Managed[path]
        disk := fileHash(filepath.Join(target, filepath.FromSlash(path)))
        if existed && disk != "" && disk != old.BaselineSHA256 && old.BaselineSHA256 == hash(data) { continue }
        if err = write(target, path, data); err != nil { return result, err }
    }
    if err = saveManifest(target, incoming); err != nil { return result, err }
    result.ChangedInstructions = changedInstructions
    result.InstructionRefreshRequired = len(changedInstructions) > 0
    return result, nil
}
