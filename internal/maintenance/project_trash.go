package maintenance

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Stage with the platform's identity-bound no-replace rename before invoking
// a path-based native shell API. Never substitute permanent deletion.
func stageAndRecycle(source string, expected os.FileInfo, move func(string, string, os.FileInfo) error, recycle func(string, os.FileInfo) (string, error)) (string, error) {
	container, err := os.MkdirTemp(filepath.Dir(source), ".task-mecca-recycle-")
	if err != nil {
		return "", fmt.Errorf("cannot create private recycle staging; folder preserved: %w", err)
	}
	staged := filepath.Join(container, filepath.Base(source))
	if err := move(source, staged, expected); err != nil {
		_ = os.Remove(container) // Only our empty container.
		return "", err
	}
	location, err := recycle(staged, expected)
	if err != nil {
		if location == "" {
			if info, statErr := os.Lstat(staged); statErr == nil && os.SameFile(expected, info) {
				if rollbackErr := move(staged, source, info); rollbackErr == nil {
					_ = os.Remove(container)
					return "", fmt.Errorf("native Trash refused; folder restored at %s; fix permissions (macOS Files and Folders/Full Disk Access) or files in use before retrying: %w", source, err)
				}
			}
			if info, statErr := os.Lstat(staged); statErr == nil && os.SameFile(expected, info) {
				location = staged
			} else {
				location = systemTrashLocator("Outcome unknown; inspect system Trash and original path before any retry", staged)
			}
		}
		return location, fmt.Errorf("native Trash outcome incomplete; do not retry until checking original %s and recovery %s: %w", source, location, err)
	}
	if location == "" {
		return systemTrashLocator("Outcome unknown; inspect system Trash and original path before any retry", staged), fmt.Errorf("native Trash returned no recovery location; staged item may have moved; inspect system Trash and original path before retrying")
	}
	if _, err := os.Lstat(staged); !os.IsNotExist(err) {
		return location, fmt.Errorf("native Trash did not confirm source absence; verify staging %s and system Trash", staged)
	}
	// Keep this empty, uniquely owned parent for the OS's Put Back/Restore action.
	return systemTrashLocator(location, staged), nil
}

func systemTrashLocator(location, staged string) string {
	return "system-trash:?" + url.Values{"location": {location}, "staged": {staged}}.Encode()
}

func trashStagingPath(location string) string {
	if !strings.HasPrefix(location, "system-trash:?") {
		return ""
	}
	values, err := url.ParseQuery(strings.TrimPrefix(location, "system-trash:?"))
	if err != nil {
		return ""
	}
	return values.Get("staged")
}

func TrashStagingPath(location string) string { return trashStagingPath(location) }

func trashOutcomeUnknown(location string) bool {
	values, _ := url.ParseQuery(strings.TrimPrefix(location, "system-trash:?"))
	return strings.HasPrefix(values.Get("location"), "Outcome unknown;")
}

func rollbackTrashFolder(location, original string) error {
	if strings.HasPrefix(location, "system-trash:") {
		return fmt.Errorf("native Trash requires user restoration; no filesystem rollback is safe")
	}
	info, err := os.Lstat(location)
	if err != nil {
		return err
	}
	return moveProjectFolder(location, original, info)
}
