package maintenance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRemovalProjectionPreservesOutcomeAndRestoreHolder(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "project")
	holder := filepath.Join(root, ".task-mecca-recycle-fixture")
	staged := filepath.Join(holder, "project")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(holder, 0700); err != nil {
		t.Fatal(err)
	}
	item := RemovalRecord{Path: source, FolderOutcome: "moved_to_trash", Presence: "present", StagingPath: staged}
	projectRemovalProjection(&item)
	if item.FolderOutcome != "moved_to_trash" || item.SourceState != "present_after_trash" || item.StagingHolderState != "retained_for_restore" || item.ParentHolderPath != root || item.StagingHolderPath != holder {
		t.Fatalf("projection %#v", item)
	}
	if err := os.MkdirAll(staged, 0700); err != nil {
		t.Fatal(err)
	}
	projectRemovalProjection(&item)
	if item.StagingHolderState != "contains_files" {
		t.Fatal(item)
	}
	item.StagingPath = filepath.Join(root, "foreign", "project")
	projectRemovalProjection(&item)
	if item.StagingHolderPath != "" || item.StagingHolderState != "unavailable" {
		t.Fatal("unvalidated holder read", item)
	}
}

func TestSafeOwnedEmptyStagingCleanup(t *testing.T) {
	root := t.TempDir()
	holder := filepath.Join(root, "holder")
	if err := os.Mkdir(holder, 0700); err != nil {
		t.Fatal(err)
	}
	identity, _ := os.Lstat(holder)
	if err := os.WriteFile(filepath.Join(holder, "foreign"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := removeOwnedEmptyStaging(holder, identity); err == nil {
		t.Fatal("nonempty removed")
	}
	if err := os.Remove(filepath.Join(holder, "foreign")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(holder, holder+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(holder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := removeOwnedEmptyStaging(holder, identity); err == nil {
		t.Fatal("replacement removed")
	}
	if _, err := os.Stat(holder); err != nil {
		t.Fatal(err)
	}
	current, _ := os.Lstat(holder)
	if err := removeOwnedEmptyStaging(holder, current); err != nil {
		t.Fatal(err)
	}
}

func TestTrashSuccessAndLaterSourcePresenceRemainSeparate(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", filepath.Join(root, "management"))
	source := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(source, "_task_mecca"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := RegisterProject(source); err != nil {
		t.Fatal(err)
	}
	removed, err := RemoveProject(source)
	if err != nil {
		t.Fatal(err)
	}
	_, err = moveRemovedProjectToTrash(removed.ID, source, func(path string) (string, error) {
		if err := os.Rename(path, path+"-recovery"); err != nil {
			return "", err
		}
		if err := os.Mkdir(path, 0700); err != nil {
			return "", err
		}
		return "system-trash:test-fixture", nil
	}, writeProjectRegistry)
	if err != nil {
		t.Fatal(err)
	}
	history, err := RemovalHistory()
	if err != nil {
		t.Fatal(err)
	}
	if history[0].FolderOutcome != "moved_to_trash" || history[0].Presence != "present" || history[0].SourceState != "present_after_trash" {
		t.Fatal(history)
	}
	if ProjectMonitoringAllowed(source) {
		t.Fatal("reappeared source allowed")
	}
	if _, err := MoveRemovedProjectToTrash(removed.ID, source); err == nil {
		t.Fatal("automatic repeat permitted")
	}
	if _, err := os.Stat(source + "-recovery"); errors.Is(err, os.ErrNotExist) {
		t.Fatal("recovery lost")
	}
}
