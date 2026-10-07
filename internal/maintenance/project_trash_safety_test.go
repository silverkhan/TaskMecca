package maintenance

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureNoReplace(from, to string, expected os.FileInfo) error {
	info, err := os.Lstat(from)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !os.SameFile(expected, info) {
		return errors.New("identity changed")
	}
	if _, err := os.Lstat(to); !os.IsNotExist(err) {
		return errors.New("destination exists")
	}
	return os.Rename(from, to)
}

func TestNativeRefusalRestoresAndCollisionPreservesBothFolders(t *testing.T) {
	for _, collision := range []bool{false, true} {
		t.Run(map[bool]string{false: "restored", true: "collision"}[collision], func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "original")
			if err := os.Mkdir(source, 0700); err != nil {
				t.Fatal(err)
			}
			info, _ := os.Lstat(source)
			location, err := stageAndRecycle(source, info, fixtureNoReplace, func(stage string, _ os.FileInfo) (string, error) {
				if collision {
					if err := os.Mkdir(source, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(source, "new-owner.txt"), []byte("must remain"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				return "", errors.New("native permission or lock refusal")
			})
			if err == nil {
				t.Fatal("refusal falsely succeeded")
			}
			if collision {
				if location == "" {
					t.Fatal("missing partial recovery path")
				}
				if _, err := os.Stat(filepath.Join(source, "new-owner.txt")); err != nil {
					t.Fatal("collision overwritten", err)
				}
				if _, err := os.Stat(location); err != nil {
					t.Fatal("staged original lost", err)
				}
			} else {
				if location != "" || !strings.Contains(err.Error(), "folder restored") {
					t.Fatal(location, err)
				}
				if _, err := os.Stat(source); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestNativeUnknownOutcomeDoesNotClaimAbsentStagingAsRecovery(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "original")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Lstat(source)
	recycled := filepath.Join(dir, "simulated-system-bin")
	location, err := stageAndRecycle(source, info, fixtureNoReplace, func(stage string, _ os.FileInfo) (string, error) {
		if err := os.Rename(stage, recycled); err != nil {
			t.Fatal(err)
		}
		return "", errors.New("native output unavailable after move")
	})
	if err == nil || !trashOutcomeUnknown(location) || trashStagingPath(location) == "" {
		t.Fatal(location, err)
	}
	if _, err := os.Stat(recycled); err != nil {
		t.Fatal("recoverable content lost", err)
	}
	if _, err := os.Stat(trashStagingPath(location)); !os.IsNotExist(err) {
		t.Fatal("expected absent staging", err)
	}
}

func TestNativeUnknownHistoryIsPersistedAndCannotBeRetried(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", filepath.Join(dir, "management"))
	source := filepath.Join(dir, "project")
	if err := os.MkdirAll(filepath.Join(source, "_task_mecca"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := RegisterProject(source); err != nil {
		t.Fatal(err)
	}
	record, err := RemoveProject(source)
	if err != nil {
		t.Fatal(err)
	}
	location := systemTrashLocator("Outcome unknown; inspect system Trash", filepath.Join(dir, "stage"))
	_, err = moveRemovedProjectToTrash(record.ID, source, func(string) (string, error) { return location, errors.New("native callback unavailable") }, writeProjectRegistry)
	if err == nil {
		t.Fatal("unknown reported success")
	}
	history, err := RemovalHistory()
	if err != nil || history[0].FolderOutcome != "cleanup_unknown" || history[0].CleanupError == "" || history[0].StagingPath == "" {
		t.Fatal(history, err)
	}
	called := false
	_, err = moveRemovedProjectToTrash(record.ID, source, func(string) (string, error) { called = true; return "", nil }, writeProjectRegistry)
	if err == nil || called {
		t.Fatal("unknown cleanup retried")
	}
}

func TestStagingIdentityChangeDoesNotRecycleReplacement(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "project")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	expected, _ := os.Lstat(source)
	location, err := stageAndRecycle(source, expected, fixtureNoReplace, func(stage string, info os.FileInfo) (string, error) {
		if err := os.Rename(stage, filepath.Join(dir, "preserved-original")); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(stage, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stage, "replacement"), []byte("not ours"), 0600); err != nil {
			t.Fatal(err)
		}
		return "", errors.New("staging identity changed")
	})
	if err == nil || !trashOutcomeUnknown(location) {
		t.Fatal(location, err)
	}
	if _, err := os.Stat(filepath.Join(trashStagingPath(location), "replacement")); err != nil {
		t.Fatal("replacement changed", err)
	}
}
