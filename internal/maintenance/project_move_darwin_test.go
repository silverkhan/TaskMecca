//go:build darwin

package maintenance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnchoredCleanupNeverOverwritesRacingTrashDestination(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "project")
	destination := filepath.Join(dir, "fixture-trash", "project")
	for _, path := range []string{source, filepath.Dir(destination)} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	err = moveProjectFolderBeforeRename(source, destination, expected, func() {
		if err := os.WriteFile(destination, []byte("must survive"), 0600); err != nil {
			t.Fatal(err)
		}
	})
	if err == nil {
		t.Fatal("racing destination accepted")
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "must survive" {
		t.Fatal("racing destination overwritten", err)
	}
	unchanged, err := os.Stat(source)
	if err != nil || !os.SameFile(expected, unchanged) {
		t.Fatal("source changed", err)
	}
}

func TestAnchoredCleanupDoesNotFollowReplacedParent(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(dir, "parent")
	source := filepath.Join(parent, "project")
	outside := filepath.Join(dir, "outside")
	outsideProject := filepath.Join(outside, "project")
	destination := filepath.Join(dir, "fixture-trash", "project")
	for _, path := range []string{source, outsideProject, filepath.Dir(destination)} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	external, err := os.Stat(outsideProject)
	if err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(dir, "saved-parent")
	err = moveProjectFolderBeforeRename(source, destination, expected, func() {
		if err := os.Rename(parent, saved); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, parent); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	moved, err := os.Stat(destination)
	if err != nil || !os.SameFile(expected, moved) {
		t.Fatal("confirmed directory not moved", err)
	}
	unchanged, err := os.Stat(outsideProject)
	if err != nil || !os.SameFile(external, unchanged) {
		t.Fatal("outside directory touched", err)
	}
}

func TestAnchoredCleanupRestoresLeafSwapWithoutFollowingLink(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "project")
	outside := filepath.Join(dir, "outside")
	destination := filepath.Join(dir, "fixture-trash", "project")
	for _, path := range []string{source, outside, filepath.Dir(destination)} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	external, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(dir, "confirmed-original")
	err = moveProjectFolderBeforeRename(source, destination, expected, func() {
		if err := os.Rename(source, saved); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, source); err != nil {
			t.Fatal(err)
		}
	})
	if err == nil {
		t.Fatal("leaf swap accepted")
	}
	if info, err := os.Lstat(source); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("swapped link not restored", err)
	}
	unchanged, err := os.Stat(outside)
	if err != nil || !os.SameFile(external, unchanged) {
		t.Fatal("outside link target touched", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("swapped link left in Trash", err)
	}
}
