//go:build windows

package maintenance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsCleanupRejectsUNCDeviceAndDriveRoots(t *testing.T) {
	for _, path := range []string{`\\server\share\project`, `\\?\C:\project`, `C:\`} {
		if path == `C:\` {
			if _, err := safeCleanupPath(path); err == nil {
				t.Fatal("drive root accepted")
			}
		} else if err := validateTrashPlatform(path); err == nil {
			t.Fatal("unsupported path accepted", path)
		}
	}
}

func TestWindowsCleanupIdentityAndDestinationCollision(t *testing.T) {
	for _, swap := range []bool{false, true} {
		t.Run(map[bool]string{false: "collision", true: "identity-swap"}[swap], func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			destination := filepath.Join(dir, "destination")
			if err := os.Mkdir(source, 0700); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(source)
			if err != nil {
				t.Fatal(err)
			}
			info, err := f.Stat()
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			before := func() {
				if err := os.Rename(source, filepath.Join(dir, "preserved")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(source, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if !swap {
				if err := os.WriteFile(destination, []byte("do not overwrite"), 0600); err != nil {
					t.Fatal(err)
				}
				before = nil
			}
			if err := moveProjectFolderBeforeRename(source, destination, info, before); err == nil {
				t.Fatal("unsafe move accepted")
			}
			if swap {
				if _, err := os.Stat(filepath.Join(dir, "preserved")); err != nil {
					t.Fatal(err)
				}
			} else {
				data, err := os.ReadFile(destination)
				if err != nil || string(data) != "do not overwrite" {
					t.Fatal("collision changed", err)
				}
			}
		})
	}
}

func TestWindowsCleanupLockedFolderPreservesPayload(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "project")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(source, "locked.txt")
	if err := os.WriteFile(file, []byte("preserve locked payload"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := MoveProjectToTrash(source); err == nil {
		t.Fatal("locked folder recycle accepted")
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "preserve locked payload" {
		t.Fatal("locked payload lost", err)
	}
}
