// Package projectguard fences project-local background writes against durable
// Hub pause/removal state. Its lock is outside the project, so checking an absent
// or removed project never recreates that project's runtime directory.
package projectguard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrSuppressed = errors.New("project writes suppressed: monitoring stopped, folder unavailable, or management busy")

func Home() string {
	if value := strings.TrimSpace(os.Getenv("TASK_MECCA_HOME")); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".task-mecca")
}

// CanonicalPath resolves existing ancestors even after the project was moved.
// This keeps /tmp and /private/tmp, relative paths and symlink-parent aliases
// equivalent without inspecting the user's Trash inventory.
func CanonicalPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", ErrSuppressed
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	probe := filepath.Clean(absolute)
	parts := []string{}
	for {
		resolved, resolveErr := filepath.EvalSymlinks(probe)
		if resolveErr == nil {
			for i := len(parts) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, parts[i])
			}
			resolved = filepath.Clean(resolved)
			if runtime.GOOS == "windows" {
				resolved = strings.ToLower(resolved)
			}
			return resolved, nil
		}
		if !os.IsNotExist(resolveErr) {
			return "", resolveErr
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return "", resolveErr
		}
		parts = append(parts, filepath.Base(probe))
		probe = parent
	}
}

func sameOrDescendant(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

// Stored boundaries must not follow newly substituted project/ancestor symlinks.
// Only normalize the platform's fixed root aliases; removal persists its resolved
// canonical boundary separately in paused_projects at mutation time.
func storedBoundary(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	if runtime.GOOS == "darwin" {
		for _, prefix := range []string{"/tmp", "/var", "/etc"} {
			if sameOrDescendant(absolute, prefix) {
				resolved, err := filepath.EvalSymlinks(prefix)
				if err != nil {
					return "", err
				}
				absolute = resolved + strings.TrimPrefix(absolute, prefix)
				break
			}
		}
	}
	if runtime.GOOS == "windows" {
		absolute = strings.ToLower(absolute)
	}
	return absolute, nil
}

// Same-file matching covers case-insensitive volumes without merging distinct
// case-sensitive directories. Never follow a tombstone's substituted symlink.
func sameFileBoundary(project, root string) bool {
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return false
	}
	for parent := filepath.Dir(root); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if filepath.Dir(parent) == parent {
			break
		}
	}
	for probe := project; ; probe = filepath.Dir(probe) {
		if info, err := os.Stat(probe); err == nil && os.SameFile(info, rootInfo) {
			return true
		}
		if filepath.Dir(probe) == probe {
			return false
		}
	}
}

func Allowed(project string) bool {
	canonical, err := CanonicalPath(project)
	if err != nil {
		return false
	}
	lexical, err := storedBoundary(project)
	if err != nil {
		return false
	}
	info, err := os.Stat(project)
	if err != nil || !info.IsDir() {
		return false
	}
	home := Home()
	if home == "" {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(home, "projects.json"))
	if os.IsNotExist(err) {
		return true
	}
	if err != nil {
		return false
	}
	var reg struct {
		Paused  []string `json:"paused_projects"`
		History []struct {
			Path string `json:"path"`
		} `json:"removal_history"`
	}
	if json.Unmarshal(raw, &reg) != nil {
		return false
	}
	paths := append([]string(nil), reg.Paused...)
	for _, item := range reg.History {
		paths = append(paths, item.Path)
	}
	for _, path := range paths {
		root, err := storedBoundary(path)
		if err != nil {
			return false
		}
		if sameOrDescendant(canonical, root) || sameOrDescendant(lexical, root) || sameFileBoundary(canonical, root) {
			return false
		}
	}
	return true
}

func AcquireWrite(project string) (func(), error) {
	// Check before creating even the shared-home lock for unavailable projects.
	if !Allowed(project) {
		return nil, ErrSuppressed
	}
	release, err := lock(false)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSuppressed, err)
	}
	if !Allowed(project) {
		release()
		return nil, ErrSuppressed
	}
	return release, nil
}

func AcquireManagement() (func(), error) { return lock(true) }

// AcquireWrites fences a shared-recipient operation before any network request
// or partial settings update. A suppressed member rejects the entire operation.
func AcquireWrites(projects []string) (func(), error) {
	releases := []func(){}
	releaseAll := func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}
	for _, project := range projects {
		release, err := AcquireWrite(project)
		if err != nil {
			releaseAll()
			return nil, err
		}
		releases = append(releases, release)
	}
	return releaseAll, nil
}

func lockPath() (string, error) {
	home := Home()
	if home == "" {
		return "", ErrSuppressed
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return "", err
	}
	return filepath.Join(home, "projects.lock"), nil
}
