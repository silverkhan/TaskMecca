package maintenance

// Project management changes registry state only; it never moves or deletes files.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/silverkhan/TaskMecca/internal/projectguard"
)

// Serialize management mutations so repeated Hub actions cannot move a folder
// twice or overwrite another action's removal record.
var projectManagementMu sync.Mutex

// Background monitoring requires active registration. Forgetting an archive
// entry retains durable suppression; Web opening never opts into monitoring.
func projectMonitoringAllowed(path string) bool {
	if !projectguard.Allowed(path) {
		return false
	}
	reg, err := readProjectRegistry()
	if err != nil || projectPresence(path) != "present" {
		return false
	}
	if containsCleanPath(reg.Paused, path) {
		return false
	}
	for _, item := range reg.History {
		if filepath.Clean(item.Path) == filepath.Clean(path) {
			return false
		}
	}
	for _, item := range reg.Projects {
		if sameProjectIdentity(item.Path, path) {
			return true
		}
	}
	return false
}

func ProjectMonitoringAllowed(path string) bool {
	projectManagementMu.Lock()
	defer projectManagementMu.Unlock()
	return projectMonitoringAllowed(path)
}

// Serialize the complete background write with pause/remove, rather than
// check once and allow an already-running delivery to recreate a removed path.
func WithProjectMonitoring(path string, work func()) bool {
	projectManagementMu.Lock()
	defer projectManagementMu.Unlock()
	if !projectMonitoringAllowed(path) {
		return false
	}
	work()
	return true
}

// Web opening/restarting never changes registration. Only explicit add/init does.
func RegisterWebProject(path string) error { return nil }

// Linked worktrees have a .git file pointing to an administrative directory
// with commondir. Ordinary repositories use a .git directory; submodules may
// use a .git file but do not have the linked-worktree commondir marker.
func linkedGitWorktree(path string) bool {
	marker, err := os.ReadFile(filepath.Join(path, ".git"))
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(marker)), "gitdir:") {
		return false
	}
	gitdir := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(marker)), "gitdir:"))
	if gitdir == "" {
		return false
	}
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(path, gitdir)
	}
	common, err := os.ReadFile(filepath.Join(gitdir, "commondir"))
	return err == nil && strings.TrimSpace(string(common)) != ""
}

type RemovalRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	RemovedAt string `json:"archived_at"`
	Boundary  string `json:"canonical_path,omitempty"`
}

type projectRegistry struct {
	Projects []Project       `json:"projects"`
	History  []RemovalRecord `json:"archived_projects,omitempty"`
	Paused   []string        `json:"paused_projects,omitempty"`
	Preserved map[string]json.RawMessage `json:"-"`
}

// Legacy removal facts and unknown registry fields are immutable evidence.
// Keep them opaque: no old folder actions or display are reintroduced.
func (value *projectRegistry) UnmarshalJSON(data []byte) error {
	type fields projectRegistry
	var decoded fields
	if err := json.Unmarshal(data, &decoded); err != nil { return err }
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil { return err }
	for _, key := range []string{"projects", "archived_projects", "paused_projects"} { delete(raw, key) }
	decoded.Preserved = raw
	*value = projectRegistry(decoded)
	return nil
}

func (value projectRegistry) MarshalJSON() ([]byte, error) {
	type fields projectRegistry
	data, err := json.Marshal(fields(value))
	if err != nil { return nil, err }
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil { return nil, err }
	for key, data := range value.Preserved { raw[key] = data }
	return json.Marshal(raw)
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
	release, lockErr := projectguard.AcquireManagement()
	if lockErr != nil {
		return lockErr
	}
	defer release()
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	reg, err := readProjectRegistry()
	if err != nil {
		return err
	}
	for i := range reg.Projects {
		if sameProjectIdentity(reg.Projects[i].Path, abs) {
			canonical, canonicalErr := projectguard.CanonicalPath(abs)
			if canonicalErr != nil {
				return canonicalErr
			}
			if enabled {
				filtered := reg.Paused[:0]
				for _, item := range reg.Paused {
					if !projectguard.MatchesBoundary(item, abs) {
						filtered = append(filtered, item)
					}
				}
				reg.Paused = filtered
			} else if !containsCleanPath(reg.Paused, abs) {
				reg.Paused = append(reg.Paused, abs)
			}
			if !enabled && !containsCleanPath(reg.Paused, canonical) {
				reg.Paused = append(reg.Paused, canonical)
			}
			return writeProjectRegistry(reg)
		}
	}
	return fmt.Errorf("registered project not found")
}

func RemoveProject(path string) (RemovalRecord, error) {
	projectManagementMu.Lock()
	defer projectManagementMu.Unlock()
	release, lockErr := projectguard.AcquireManagement()
	if lockErr != nil {
		return RemovalRecord{}, lockErr
	}
	defer release()
	abs, err := filepath.Abs(path)
	if err != nil {
		return RemovalRecord{}, err
	}
	reg, err := readProjectRegistry()
	if err != nil {
		return RemovalRecord{}, err
	}
	for i, p := range reg.Projects {
		if !sameProjectIdentity(p.Path, abs) {
			continue
		}
		record := RemovalRecord{ID: fmt.Sprintf("archive-%d", time.Now().UnixNano()), Name: p.Name, Path: p.Path, RemovedAt: time.Now().Format(time.RFC3339)}
		reg.Projects = append(reg.Projects[:i], reg.Projects[i+1:]...)
		if !containsCleanPath(reg.Paused, abs) {
			reg.Paused = append(reg.Paused, abs)
		}
		canonical, canonicalErr := projectguard.CanonicalPath(abs)
		if canonicalErr != nil {
			return RemovalRecord{}, canonicalErr
		}
		if !containsCleanPath(reg.Paused, canonical) {
			reg.Paused = append(reg.Paused, canonical)
		}
		record.Boundary = canonical
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
	return out, nil
}

func DeleteRemovalHistory(id string) error {
	projectManagementMu.Lock()
	defer projectManagementMu.Unlock()
	release, lockErr := projectguard.AcquireManagement()
	if lockErr != nil {
		return lockErr
	}
	defer release()
	reg, err := readProjectRegistry()
	if err != nil {
		return err
	}
	for i, h := range reg.History {
		if h.ID == id {
			if !containsCleanPath(reg.Paused, h.Path) {
				reg.Paused = append(reg.Paused, h.Path)
			}
			reg.History = append(reg.History[:i], reg.History[i+1:]...)
			return writeProjectRegistry(reg)
		}
	}
	return fmt.Errorf("removal history not found")
}

func ArchiveProject(path string) (RemovalRecord, error) { return RemoveProject(path) }
func ArchivedProjects() ([]RemovalRecord, error)        { return RemovalHistory() }
func IsManagedProject(path string) bool {
	if IsRegisteredProject(path) {
		return true
	}
	archives, err := ArchivedProjects()
	if err != nil {
		return false
	}
	for _, item := range archives {
		if filepath.Clean(item.Path) == filepath.Clean(path) {
			return true
		}
	}
	return false
}
func ForgetArchivedProject(id, path string) error {
	items, err := ArchivedProjects()
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ID == id && filepath.Clean(item.Path) == filepath.Clean(path) {
			return DeleteRemovalHistory(id)
		}
	}
	return fmt.Errorf("exact archive ID and path required")
}
func RestoreArchivedProject(id, path string) error {
	projectManagementMu.Lock()
	defer projectManagementMu.Unlock()
	release, err := projectguard.AcquireManagement()
	if err != nil {
		return err
	}
	defer release()
	reg, err := readProjectRegistry()
	if err != nil {
		return err
	}
	for i, item := range reg.History {
		if item.ID != id || filepath.Clean(item.Path) != filepath.Clean(path) {
			continue
		}
		if projectPresence(path) != "present" {
			return fmt.Errorf("project path is unavailable")
		}
		if _, err = os.Stat(filepath.Join(path, "_task_mecca")); err != nil {
			return err
		}
		canonical, canonicalErr := projectguard.CanonicalPath(path)
		if canonicalErr != nil {
			return canonicalErr
		}
		if item.Boundary != "" && !projectguard.MatchesBoundary(item.Boundary, canonical) {
			return fmt.Errorf("archived project identity changed; add the intended path explicitly")
		}
		filtered := reg.Paused[:0]
		for _, paused := range reg.Paused {
			if !projectguard.MatchesBoundary(paused, path) {
				filtered = append(filtered, paused)
			}
		}
		reg.Paused = filtered
		reg.History = append(reg.History[:i], reg.History[i+1:]...)
		found := false
		for _, p := range reg.Projects {
			if filepath.Clean(p.Path) == filepath.Clean(path) {
				found = true
			}
		}
		if !found {
			reg.Projects = append(reg.Projects, Project{Name: item.Name, Path: path, FrameworkVersion: frameworkVersion(path), LastSeen: time.Now().Format(time.RFC3339)})
		}
		return writeProjectRegistry(reg)
	}
	return fmt.Errorf("archive not found")
}

func sameProjectIdentity(left, right string) bool {
	if filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	a, err := os.Lstat(left)
	if err != nil || a.Mode()&os.ModeSymlink != 0 {
		return false
	}
	b, err := os.Lstat(right)
	return err == nil && b.Mode()&os.ModeSymlink == 0 && os.SameFile(a, b)
}
