package maintenance

// Project management deliberately keeps registry changes and filesystem changes
// separate.  Removing a Hub entry never removes its directory; a caller must
// explicitly request the second, recoverable action.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Serialize management mutations so repeated Hub actions cannot move a folder
// twice or overwrite another action's removal record.
var projectManagementMu sync.Mutex

type RemovalRecord struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	RemovedAt     string `json:"removed_at"`
	FolderOutcome string `json:"folder_outcome"`
	TrashPath     string `json:"trash_path,omitempty"`
	LastCheckedAt string `json:"last_checked_at,omitempty"`
	Presence      string `json:"presence,omitempty"`
}

type projectRegistry struct {
	Projects []Project       `json:"projects"`
	History  []RemovalRecord `json:"removal_history,omitempty"`
	Paused   []string        `json:"paused_projects,omitempty"`
}

func readProjectRegistry() (projectRegistry, error) {
	var value projectRegistry
	data, err := os.ReadFile(registryPath())
	if os.IsNotExist(err) {
		return value, nil
	}
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, fmt.Errorf("project registry is invalid: %w", err)
	}
	return value, nil
}

func writeProjectRegistry(value projectRegistry) error {
	if err := os.MkdirAll(homeDir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(homeDir(), ".projects-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), registryPath())
}

// ProjectState is read-only status for the Hub.  It never creates a runtime
// directory and never probes a missing project beyond lstat.
type ProjectState struct {
	Project
	Presence   string `json:"presence"`
	CheckedAt  string `json:"checked_at"`
	Monitoring bool   `json:"monitoring"`
}

func projectPresence(path string) string {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "missing"
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "unavailable"
	}
	return "present"
}

func ManagedProjects() ([]ProjectState, error) {
	reg, err := readProjectRegistry()
	if err != nil {
		return nil, err
	}
	now := time.Now().Format(time.RFC3339)
	out := make([]ProjectState, 0, len(reg.Projects))
	for _, p := range reg.Projects {
		out = append(out, ProjectState{Project: p, Presence: projectPresence(p.Path), CheckedAt: now, Monitoring: !containsCleanPath(reg.Paused, p.Path)})
	}
	return out, nil
}

func containsCleanPath(paths []string, path string) bool {
	for _, candidate := range paths {
		if filepath.Clean(candidate) == filepath.Clean(path) {
			return true
		}
	}
	return false
}

func SetProjectMonitoring(path string, enabled bool) error {
	projectManagementMu.Lock()
	defer projectManagementMu.Unlock()
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	reg, err := readProjectRegistry()
	if err != nil {
		return err
	}
	for i := range reg.Projects {
		if filepath.Clean(reg.Projects[i].Path) == filepath.Clean(abs) {
			if enabled {
				filtered := reg.Paused[:0]
				for _, item := range reg.Paused {
					if filepath.Clean(item) != filepath.Clean(abs) {
						filtered = append(filtered, item)
					}
				}
				reg.Paused = filtered
			} else if !containsCleanPath(reg.Paused, abs) {
				reg.Paused = append(reg.Paused, abs)
			}
			return writeProjectRegistry(reg)
		}
	}
	return fmt.Errorf("registered project not found")
}

func RemoveProject(path string) (RemovalRecord, error) {
	projectManagementMu.Lock()
	defer projectManagementMu.Unlock()
	abs, err := filepath.Abs(path)
	if err != nil {
		return RemovalRecord{}, err
	}
	reg, err := readProjectRegistry()
	if err != nil {
		return RemovalRecord{}, err
	}
	for i, p := range reg.Projects {
		if filepath.Clean(p.Path) != filepath.Clean(abs) {
			continue
		}
		record := RemovalRecord{ID: fmt.Sprintf("removed-%d", time.Now().UnixNano()), Name: p.Name, Path: p.Path, RemovedAt: time.Now().Format(time.RFC3339), FolderOutcome: "preserved"}
		record.Presence = projectPresence(p.Path)
		record.LastCheckedAt = time.Now().Format(time.RFC3339)
		reg.Projects = append(reg.Projects[:i], reg.Projects[i+1:]...)
		filtered := reg.Paused[:0]
		for _, item := range reg.Paused {
			if filepath.Clean(item) != filepath.Clean(abs) {
				filtered = append(filtered, item)
			}
		}
		reg.Paused = filtered
		reg.History = append([]RemovalRecord{record}, reg.History...)
		return record, writeProjectRegistry(reg)
	}
	return RemovalRecord{}, fmt.Errorf("registered project not found")
}

func RemovalHistory() ([]RemovalRecord, error) {
	reg, err := readProjectRegistry()
	if err != nil {
		return nil, err
	}
	out := append([]RemovalRecord(nil), reg.History...)
	for i := range out {
		out[i].Presence = projectPresence(out[i].Path)
		out[i].LastCheckedAt = time.Now().Format(time.RFC3339)
	}
	return out, nil
}

func DeleteRemovalHistory(id string) error {
	projectManagementMu.Lock()
	defer projectManagementMu.Unlock()
	reg, err := readProjectRegistry()
	if err != nil {
		return err
	}
	for i, h := range reg.History {
		if h.ID == id {
			reg.History = append(reg.History[:i], reg.History[i+1:]...)
			return writeProjectRegistry(reg)
		}
	}
	return fmt.Errorf("removal history not found")
}

func safeCleanupPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(abs)
	home, _ := os.UserHomeDir()
	for _, protected := range []string{homeDir(), filepath.Join(home, ".Trash")} {
		if clean == filepath.Clean(protected) || strings.HasPrefix(clean, filepath.Clean(protected)+string(filepath.Separator)) {
			return "", fmt.Errorf("management data or Trash cannot be cleaned")
		}
	}
	for _, component := range strings.Split(clean, string(filepath.Separator)) {
		if component == "_task_mecca" || component == ".Trash" {
			return "", fmt.Errorf("management data or Trash cannot be cleaned")
		}
	}
	for _, protected := range []string{homeDir(), os.TempDir(), "/Users", "/Users/Shared", "/Applications", "/System", "/Library", "/private", "/private/tmp", "/var", "/etc", "/usr", "/bin", "/sbin"} {
		if clean == filepath.Clean(protected) || strings.HasPrefix(filepath.Clean(protected), clean+string(filepath.Separator)) {
			return "", fmt.Errorf("protected shared or management path cannot be cleaned")
		}
	}
	if clean == string(filepath.Separator) || clean == filepath.Clean(home) || clean == filepath.Clean(filepath.Dir(clean)) {
		return "", fmt.Errorf("protected path cannot be cleaned")
	}
	info, err := os.Lstat(clean)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("symbolic-link project cannot be cleaned")
	}
	if !info.IsDir() {
		return "", fmt.Errorf("only directories can be cleaned")
	}
	parent := filepath.Dir(clean)
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil || filepath.Clean(resolvedParent) != filepath.Clean(parent) {
		return "", fmt.Errorf("project under symbolic-link parent cannot be cleaned")
	}
	if _, err := os.Lstat(filepath.Join(clean, ".git")); err == nil {
		return "", fmt.Errorf("repository root cannot be cleaned automatically")
	}
	for ancestor := filepath.Dir(clean); ancestor != filepath.Dir(ancestor); ancestor = filepath.Dir(ancestor) {
		if _, err := os.Lstat(filepath.Join(ancestor, ".git")); err == nil {
			return "", fmt.Errorf("folder inside an original repository cannot be cleaned automatically")
		}
	}
	return clean, nil
}

// MoveProjectToTrash moves only an explicitly named, non-repository directory.
// It is intentionally recoverable and never performs permanent deletion.
func MoveProjectToTrash(path string) (string, error) {
	source, err := safeCleanupPath(path)
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("recoverable folder cleanup is supported only by macOS Trash; folder preserved")
	}
	expected, err := os.Lstat(source)
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	trash := filepath.Join(home, ".Trash")
	if info, err := os.Lstat(trash); err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return "", fmt.Errorf("Trash must be a real directory; folder preserved")
	}
	if err := os.MkdirAll(trash, 0700); err != nil {
		return "", err
	}
	container, err := os.MkdirTemp(trash, "task-mecca-")
	if err != nil {
		return "", err
	}
	destination := filepath.Join(container, filepath.Base(source))
	if err := moveProjectFolder(source, destination, expected); err != nil {
		_ = os.Remove(container) // Only remove our empty private container.
		return "", err
	}
	return destination, nil
}

func RecordTrashResult(path, trashPath string) error {
	reg, err := readProjectRegistry()
	if err != nil {
		return err
	}
	for i := range reg.History {
		if filepath.Clean(reg.History[i].Path) == filepath.Clean(path) {
			reg.History[i].FolderOutcome = "moved_to_trash"
			reg.History[i].TrashPath = trashPath
			reg.History[i].Presence = "missing"
			reg.History[i].LastCheckedAt = time.Now().Format(time.RFC3339)
			return writeProjectRegistry(reg)
		}
	}
	return fmt.Errorf("removal history not found")
}

// MoveRemovedProjectToTrash only cleans a folder represented by the exact
// removal-history record selected in the Hub. This keeps the destructive
// follow-up action bound to a prior, explicit "Remove from Hub" operation
// instead of accepting an arbitrary path from a browser request.
func MoveRemovedProjectToTrash(historyID, path string) (string, error) {
	projectManagementMu.Lock()
	defer projectManagementMu.Unlock()
	return moveRemovedProjectToTrash(historyID, path, MoveProjectToTrash, writeProjectRegistry)
}

func moveRemovedProjectToTrash(historyID, path string, move func(string) (string, error), write func(projectRegistry) error) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	reg, err := readProjectRegistry()
	if err != nil {
		return "", err
	}
	for i := range reg.History {
		item := &reg.History[i]
		if item.ID != historyID || filepath.Clean(item.Path) != filepath.Clean(abs) {
			continue
		}
		if item.FolderOutcome == "moved_to_trash" {
			return "", fmt.Errorf("folder cleanup already recorded")
		}
		trashPath, err := move(item.Path)
		if err != nil {
			item.FolderOutcome = "cleanup_failed"
			item.Presence = projectPresence(item.Path)
			item.LastCheckedAt = time.Now().Format(time.RFC3339)
			if writeErr := write(reg); writeErr != nil {
				return "", fmt.Errorf("folder cleanup failed (%v); failure history could not be updated: %w", err, writeErr)
			}
			return "", err
		}
		item.FolderOutcome = "moved_to_trash"
		item.TrashPath = trashPath
		item.Presence = "missing"
		item.LastCheckedAt = time.Now().Format(time.RFC3339)
		if err := write(reg); err != nil {
			if _, sourceErr := os.Lstat(item.Path); os.IsNotExist(sourceErr) {
				if rollbackErr := os.Rename(trashPath, item.Path); rollbackErr == nil {
					return "", fmt.Errorf("removal history update failed; folder restored: %w", err)
				}
			}
			return trashPath, fmt.Errorf("folder moved but removal history could not be updated; restore from %s: %w", trashPath, err)
		}
		return trashPath, nil
	}
	return "", fmt.Errorf("removal history not found for folder cleanup")
}
