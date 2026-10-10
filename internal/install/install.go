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
	Policy         string `json:"policy"`
	BaselineSHA256 string `json:"baseline_sha256"`
}

type manifest struct {
	Version string             `json:"task_mecca_version"`
	Schema  int                `json:"schema_version"`
	Managed map[string]managed `json:"managed_files"`
	Owned   []string           `json:"project_owned_patterns"`
}

type MigrationResult struct {
	PlanDigest                 string   `json:"plan_digest,omitempty"`
	Status                     string   `json:"status"`
	ModifiedFiles              []string `json:"modified_files,omitempty"`
	Choices                    []string `json:"choices,omitempty"`
	FromVersion                string   `json:"from_version"`
	ToVersion                  string   `json:"to_version"`
	InstructionRefreshRequired bool     `json:"instruction_refresh_required"`
	ChangedInstructions        []string `json:"changed_instructions,omitempty"`
	LegacyBootstrap            bool     `json:"legacy_bootstrap,omitempty"`
	BackupPath                 string   `json:"backup_path,omitempty"`
}

var owned = []string{"data/**", "backlog/**", "backlog_*/**", ".runtime/**", "backups/**", "config.toml"}

var customizable = map[string]bool{
	"ROOT_PROMPT.md": true, "framework/README.md": true,
	"framework/README.en.md": true, "framework/SESSION_GUIDE.md": true,
	"framework/SESSION_GUIDE.en.md": true, "framework/collab.md": true,
	"framework/EXECUTION_PROTOCOL.md": true, "framework/EXECUTION_PROTOCOL.en.md": true,
	"framework/_template.md": true,
}

func policy(path string) string {
	if customizable[path] || strings.HasPrefix(path, "framework/roles/") {
		return "customizable"
	}
	if path == "VERSION" {
		return "updater"
	}
	return "framework"
}

func cleanVersion(value string) string {
	value = strings.TrimSpace(value)
	for {
		previous := value
		value = strings.TrimSpace(strings.TrimSuffix(value, `\n`))
		value = strings.TrimSpace(strings.TrimSuffix(value, `\r`))
		if value == previous {
			return value
		}
	}
}

func hash(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func bundled() (map[string][]byte, error) {
	files := map[string][]byte{}
	err := fs.WalkDir(goassets.Template, sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, sourceRoot+"/")
		if rel == "manifest.json" {
			return nil
		}
		data, err := goassets.Template.ReadFile(path)
		if err == nil {
			files[rel] = data
		}
		return err
	})
	return files, err
}

func newManifest(files map[string][]byte, version string) manifest {
	m := manifest{Version: version, Schema: 2, Managed: map[string]managed{}, Owned: owned}
	for path, data := range files {
		m.Managed[path] = managed{Policy: policy(path), BaselineSHA256: hash(data)}
	}
	return m
}

func saveManifest(target string, m manifest) error {
	if err := safeTargetPath(target, "manifest.json"); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(target, "manifest.json"), append(data, '\n'), 0644)
}

func write(target, rel string, data []byte) error {
	path := filepath.Join(target, filepath.FromSlash(rel))
	if err := safeTargetPath(target, rel); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Reject symlinked managed files and ancestors before either reads or writes.
func safeTargetPath(target, rel string) error {
	root := filepath.Clean(target)
	path := filepath.Join(root, filepath.FromSlash(rel))
	boundary, err := filepath.Rel(root, path)
	if err != nil || boundary == ".." || strings.HasPrefix(boundary, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path outside framework boundary")
	}
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic-link framework boundary: %s", current)
		}
		if current == root {
			break
		}
	}
	return nil
}

func fileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return hash(data)
}

func Init(project, version string) error {
	target := filepath.Join(project, targetName)
	if entries, err := os.ReadDir(target); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s already exists; use task-mecca migrate", target)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	files, err := bundled()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(target, 0755); err != nil {
		return err
	}
	for path, data := range files {
		if err = write(target, path, data); err != nil {
			return err
		}
	}
	return saveManifest(target, newManifest(files, version))
}

func sessionInstructionPath(path string) bool {
	switch path {
	case "ROOT_PROMPT.md", "framework/SESSION_GUIDE.md", "framework/SESSION_GUIDE.en.md", "framework/collab.md", "framework/EXECUTION_PROTOCOL.md", "framework/EXECUTION_PROTOCOL.en.md":
		return true
	}
	return strings.HasPrefix(path, "framework/roles/") && strings.HasSuffix(path, ".md")
}

func copyTree(source, destination string) error {
	info, err := os.Stat(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		data, readErr := os.ReadFile(source)
		if readErr != nil {
			return readErr
		}
		return write(destination, ".", data)
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(source, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("legacy framework backup does not support symlink: %s", path)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}

func legacyBootstrapMigration(target, version string, files map[string][]byte, result MigrationResult, choice string, snapshot map[string]string) (MigrationResult, error) {
	result.LegacyBootstrap = true
	if raw, err := os.ReadFile(filepath.Join(target, "VERSION")); err == nil {
		if value := cleanVersion(string(raw)); value != "" {
			result.FromVersion = value
		}
	}
	if result.FromVersion == "" {
		result.FromVersion = "legacy"
	}

	changedInstructions := []string{}
	for path, data := range files {
		if sessionInstructionPath(path) && fileHash(filepath.Join(target, filepath.FromSlash(path))) != hash(data) {
			changedInstructions = append(changedInstructions, path)
		}
	}
	sort.Strings(changedInstructions)
	framework := filepath.Join(target, "framework")
	if choice == "backup" && len(result.ModifiedFiles) > 0 {
		if err := safeTargetPath(target, "backups"); err != nil {
			return result, err
		}
		stamp := time.Now().Format("2006-01-02_150405")
		backupRoot := filepath.Join(target, "backups", stamp, "legacy-bootstrap")
		backedUp := false
		if _, err := os.Stat(framework); err == nil {
			if err := copyTree(framework, filepath.Join(backupRoot, "framework")); err != nil {
				return result, err
			}
			backedUp = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return result, err
		}
		for _, name := range []string{"ROOT_PROMPT.md", "VERSION"} {
			source := filepath.Join(target, name)
			if _, err := os.Stat(source); err == nil {
				data, readErr := os.ReadFile(source)
				if readErr != nil {
					return result, readErr
				}
				if err := write(backupRoot, name, data); err != nil {
					return result, err
				}
				backedUp = true
			} else if !errors.Is(err, os.ErrNotExist) {
				return result, err
			}
		}
		if backedUp {
			result.BackupPath = backupRoot
		}
	}
	// No manifest means unknown legacy files cannot safely be retired. Preserve
	// them; only named incoming framework files are refreshed.
	if err := verifyMigrationSnapshot(target, nil, snapshot); err != nil {
		return result, err
	}
	for path, data := range files {
		if err := write(target, path, data); err != nil {
			return result, err
		}
	}
	incoming := newManifest(files, version)
	if err := saveManifest(target, incoming); err != nil {
		return result, err
	}
	result.ChangedInstructions = changedInstructions
	result.InstructionRefreshRequired = len(changedInstructions) > 0
	return result, nil
}

func Migrate(project, version string) error {
	_, err := MigrateWithResult(project, version)
	return err
}

func MigrateWithResult(project, version string) (MigrationResult, error) {
	return MigrateWithChoice(project, version, "")
}

// ChoiceRequiredError means that planning completed without changing any file.
type ChoiceRequiredError struct{}

func (*ChoiceRequiredError) Error() string {
	return "local modifications require a choice: overwrite, backup, or cancel"
}

func managedPath(path string) bool {
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, "../") {
		return false
	}
	return path == "ROOT_PROMPT.md" || path == "VERSION" || path == ".gitignore" || path == "collab_tools.py" || path == "AGENTS_TASK_MECCA_SNIPPET.md" || strings.HasPrefix(path, "framework/")
}

func MigrateWithChoice(project, version, choice string) (MigrationResult, error) {
	return MigrateWithPlan(project, version, choice, "")
}

func migrationPlanDigest(version string, raw []byte, snapshot map[string]string) string {
	values := []string{version, hash(raw)}
	keys := make([]string, 0, len(snapshot))
	for key := range snapshot {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		values = append(values, key+"="+snapshot[key])
	}
	return hash([]byte(strings.Join(values, "\n")))
}

func verifyMigrationSnapshot(target string, raw []byte, snapshot map[string]string) error {
	current, err := os.ReadFile(filepath.Join(target, "manifest.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if hash(current) != hash(raw) {
		return fmt.Errorf("manifest changed during migration; no framework files updated")
	}
	for rel, expected := range snapshot {
		if err := safeTargetPath(target, rel); err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(rel)))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		actual := ""
		if err == nil {
			actual = hash(data)
		}
		if actual != expected {
			return fmt.Errorf("framework file changed during migration: %s; no framework files updated", rel)
		}
	}
	return nil
}

func MigrateWithPlan(project, version, choice, expectedPlan string) (MigrationResult, error) {
	result := MigrationResult{ToVersion: version, Status: "migrated"}
	if choice != "" && choice != "overwrite" && choice != "backup" && choice != "cancel" {
		return result, fmt.Errorf("invalid migration choice")
	}
	if choice == "cancel" {
		result.Status = "cancelled"
		return result, nil
	}
	target := filepath.Join(project, targetName)
	if err := safeTargetPath(target, "manifest.json"); err != nil {
		return result, err
	}
	raw, err := os.ReadFile(filepath.Join(target, "manifest.json"))
	if errors.Is(err, os.ErrNotExist) {
		for _, rel := range []string{"framework", "ROOT_PROMPT.md", "VERSION", ".gitignore"} {
			if err := safeTargetPath(target, rel); err != nil {
				return result, err
			}
		}
		files, bundleErr := bundled()
		if bundleErr != nil {
			return result, bundleErr
		}
		// Without a baseline, existing framework files are user modifications.
		snapshot := map[string]string{}
		walkErr := filepath.WalkDir(target, func(path string, entry fs.DirEntry, err error) error {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(target, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if managedPath(rel) {
				if err := safeTargetPath(target, rel); err != nil {
					return err
				}
				if data, err := os.ReadFile(path); err != nil {
					return err
				} else {
					snapshot[rel] = hash(data)
				}
				result.ModifiedFiles = append(result.ModifiedFiles, rel)
			}
			return nil
		})
		if walkErr != nil {
			return result, walkErr
		}
		result.PlanDigest = migrationPlanDigest(version, nil, snapshot)
		if (len(result.ModifiedFiles) > 0 && choice == "") || (expectedPlan != "" && expectedPlan != result.PlanDigest) {
			result.Status = "choice_required"
			result.Choices = []string{"overwrite", "backup", "cancel"}
			return result, &ChoiceRequiredError{}
		}
		if err := verifyMigrationSnapshot(target, nil, snapshot); err != nil {
			return result, err
		}
		return legacyBootstrapMigration(target, version, files, result, choice, snapshot)
	}
	if err != nil {
		return result, err
	}
	var previous manifest
	if err = json.Unmarshal(raw, &previous); err != nil {
		return result, err
	}
	result.FromVersion = previous.Version
	if previous.Managed == nil {
		return result, errors.New("invalid Task Mecca manifest")
	}
	files, err := bundled()
	if err != nil {
		return result, err
	}
	incoming := newManifest(files, version)
	paths := make([]string, 0, len(previous.Managed)+len(files))
	seen := map[string]bool{}
	for path := range previous.Managed {
		paths = append(paths, path)
		seen[path] = true
	}
	for path := range files {
		if !seen[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	conflicts := []string{}
	retired := []string{}
	changedInstructions := []string{}
	snapshot := map[string]string{}
	for _, path := range paths {
		if !managedPath(path) {
			return result, fmt.Errorf("manifest contains non-framework path: %s", path)
		}
		old, existed := previous.Managed[path]
		data, exists := files[path]
		if err := safeTargetPath(target, path); err != nil {
			return result, err
		}
		diskBytes, readErr := os.ReadFile(filepath.Join(target, filepath.FromSlash(path)))
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return result, readErr
		}
		disk := ""
		if readErr == nil {
			disk = hash(diskBytes)
		}
		snapshot[path] = disk
		info, statErr := os.Lstat(filepath.Join(target, filepath.FromSlash(path)))
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return result, statErr
		}
		if statErr == nil && (!info.Mode().IsRegular()) {
			return result, fmt.Errorf("managed file is not a regular file: %s", path)
		}
		modified := existed && disk != "" && disk != old.BaselineSHA256
		if !existed && disk != "" && disk != hash(data) {
			modified = true
		}
		changed := !existed || !exists || old.BaselineSHA256 != hash(data)
		if existed && !exists {
			retired = append(retired, path)
		}
		if changed && sessionInstructionPath(path) {
			changedInstructions = append(changedInstructions, path)
		}
		if modified {
			conflicts = append(conflicts, path)
		}
	}
	result.PlanDigest = migrationPlanDigest(version, raw, snapshot)
	result.ModifiedFiles = conflicts
	if (len(conflicts) > 0 && choice == "") || (expectedPlan != "" && expectedPlan != result.PlanDigest) {
		result.Status = "choice_required"
		result.Choices = []string{"overwrite", "backup", "cancel"}
		return result, &ChoiceRequiredError{}
	}
	if len(conflicts) > 0 {
		if choice == "backup" {
			if err := safeTargetPath(target, "backups"); err != nil {
				return result, err
			}
			backup, backupErr := os.MkdirTemp(filepath.Join(target, "backups"), "migration-")
			if errors.Is(backupErr, os.ErrNotExist) {
				if err = os.MkdirAll(filepath.Join(target, "backups"), 0755); err != nil {
					return result, err
				}
				backup, backupErr = os.MkdirTemp(filepath.Join(target, "backups"), "migration-")
			}
			if backupErr != nil {
				return result, backupErr
			}
			copied := []string{}
			for _, path := range conflicts {
				data, readErr := os.ReadFile(filepath.Join(target, filepath.FromSlash(path)))
				if readErr == nil {
					if err = write(backup, path, data); err != nil {
						return result, err
					}
					copied = append(copied, path)
				} else {
					return result, readErr
				}
			}
			meta, _ := json.MarshalIndent(map[string]any{"created_at": time.Now().Format(time.RFC3339), "from_version": previous.Version, "to_version": version, "files": copied}, "", "  ")
			if err = os.WriteFile(filepath.Join(backup, "backup.json"), append(meta, '\n'), 0644); err != nil {
				return result, err
			}
			result.BackupPath = backup
		}
	}
	if err := verifyMigrationSnapshot(target, raw, snapshot); err != nil {
		return result, err
	}
	for _, path := range retired {
		if err = os.Remove(filepath.Join(target, filepath.FromSlash(path))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return result, err
		}
	}
	for path, data := range files {
		old, existed := previous.Managed[path]
		disk := fileHash(filepath.Join(target, filepath.FromSlash(path)))
		if choice == "" && existed && disk != "" && disk != old.BaselineSHA256 && old.BaselineSHA256 == hash(data) {
			continue
		}
		if err = write(target, path, data); err != nil {
			return result, err
		}
	}
	if err = saveManifest(target, incoming); err != nil {
		return result, err
	}
	result.ChangedInstructions = changedInstructions
	result.InstructionRefreshRequired = len(changedInstructions) > 0
	return result, nil
}
