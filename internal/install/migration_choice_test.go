package install

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationRejectsStaleHumanChoiceBeforeWrites(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "old"); err != nil { t.Fatal(err) }
	target := filepath.Join(root, targetName)
	file := filepath.Join(target, "framework", "web", "app.js")
	if err := os.WriteFile(file, []byte("first customization"), 0600); err != nil { t.Fatal(err) }
	plan, _ := MigrateWithChoice(root, "new", "")
	if plan.PlanDigest == "" { t.Fatal("missing plan digest") }
	if err := os.WriteFile(file, []byte("later customization"), 0600); err != nil { t.Fatal(err) }
	result, err := MigrateWithPlan(root, "new", "backup", plan.PlanDigest)
	var required *ChoiceRequiredError
	if !errors.As(err, &required) || result.Status != "choice_required" { t.Fatal(result, err) }
	data, _ := os.ReadFile(file)
	if string(data) != "later customization" { t.Fatal("stale choice overwrote file") }
	if _, err := os.Stat(filepath.Join(target, "backups")); !os.IsNotExist(err) { t.Fatal("stale choice created backup") }
}

func TestLegacyMigrationRequiresChoiceWithoutBackupOrWrites(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "old"); err != nil { t.Fatal(err) }
	target := filepath.Join(root, targetName)
	if err := os.Remove(filepath.Join(target, "manifest.json")); err != nil { t.Fatal(err) }
	file := filepath.Join(target, "framework", "web", "app.js")
	if err := os.WriteFile(file, []byte("legacy customization"), 0600); err != nil { t.Fatal(err) }
	result, err := MigrateWithChoice(root, "new", "")
	var required *ChoiceRequiredError
	if !errors.As(err, &required) || result.Status != "choice_required" { t.Fatal(result, err) }
	data, _ := os.ReadFile(file)
	if string(data) != "legacy customization" { t.Fatal("legacy changed before choice") }
	if _, err := os.Stat(filepath.Join(target, "backups")); !os.IsNotExist(err) { t.Fatal("legacy backup before choice") }
	if _, err := os.Stat(filepath.Join(target, "manifest.json")); !os.IsNotExist(err) { t.Fatal("legacy manifest before choice") }
}

func TestMigrationChoiceNoWritesAndThreeChoices(t *testing.T) {
	for _, choice := range []string{"", "cancel", "overwrite", "backup"} {
		t.Run(choice, func(t *testing.T) {
			root := t.TempDir()
			if err := Init(root, "old"); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, targetName)
			file := filepath.Join(target, "framework", "web", "app.js")
			if err := os.WriteFile(file, []byte("user customization"), 0600); err != nil {
				t.Fatal(err)
			}
			manifestBefore, _ := os.ReadFile(filepath.Join(target, "manifest.json"))
			result, err := MigrateWithChoice(root, "new", choice)
			if choice == "" {
				var required *ChoiceRequiredError
				if !errors.As(err, &required) || result.Status != "choice_required" || len(result.ModifiedFiles) != 1 || len(result.Choices) != 3 {
					t.Fatal(result, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(file)
			if choice == "" || choice == "cancel" {
				if string(got) != "user customization" {
					t.Fatal("modified before choice")
				}
				after, _ := os.ReadFile(filepath.Join(target, "manifest.json"))
				if string(after) != string(manifestBefore) {
					t.Fatal("manifest changed")
				}
				if _, err := os.Stat(filepath.Join(target, "backups")); !os.IsNotExist(err) {
					t.Fatal("backup before choice")
				}
			}
			if choice == "overwrite" || choice == "backup" {
				if string(got) == "user customization" {
					t.Fatal("explicit choice did not migrate")
				}
			}
			if choice == "backup" {
				saved, err := os.ReadFile(filepath.Join(result.BackupPath, "framework", "web", "app.js"))
				if err != nil || string(saved) != "user customization" {
					t.Fatal("backup failed", err)
				}
			}
		})
	}
}

func TestMigrationBackupFailureAndReadErrorStopBeforeWrites(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "old"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, targetName)
	file := filepath.Join(target, "framework", "web", "app.js")
	os.WriteFile(file, []byte("custom"), 0600)
	os.WriteFile(filepath.Join(target, "backups"), []byte("blocked"), 0600)
	if _, err := MigrateWithChoice(root, "new", "backup"); err == nil {
		t.Fatal("backup failure ignored")
	}
	data, _ := os.ReadFile(file)
	if string(data) != "custom" {
		t.Fatal("backup failure overwrote framework")
	}
	if os.Geteuid() != 0 {
		if err := os.Chmod(file, 0200); err != nil {
			t.Skipf("cannot simulate unreadable file with chmod: %v", err)
		}
		defer os.Chmod(file, 0600)
		// On Windows the chmod write-bit setting does not revoke read
		// permission. Do not treat a readable file as an unreadable fixture.
		// The backup-failure and no-data-loss assertions above still run.
		if _, err := os.ReadFile(file); err == nil {
			t.Log("filesystem does not enforce read denial through chmod; unreadable-file subcase not applicable")
			return
		}
		if _, err := MigrateWithChoice(root, "new", "overwrite"); err == nil {
			t.Fatal("unreadable existing file treated as missing")
		}
	}
}

func TestMigrationRejectsSymlinkAncestorsAndUserOwnedManifestPaths(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		root := t.TempDir()
		Init(root, "old")
		target := filepath.Join(root, targetName)
		outside := t.TempDir()
		os.WriteFile(filepath.Join(outside, "app.js"), []byte("credentials"), 0600)
		os.Rename(filepath.Join(target, "framework", "web"), filepath.Join(target, "framework", "web-preserved"))
		if err := os.Symlink(outside, filepath.Join(target, "framework", "web")); err != nil {
			t.Skip(err)
		}
		if _, err := MigrateWithChoice(root, "new", "overwrite"); err == nil {
			t.Fatal("symlink ancestor accepted")
		}
		data, _ := os.ReadFile(filepath.Join(outside, "app.js"))
		if string(data) != "credentials" {
			t.Fatal("outside data changed")
		}
	})
	for _, rel := range []string{"data/backlog/private.md", ".runtime/notifications/telegram.json", "config.toml", "backups/keep.txt", "../credentials"} {
		t.Run(rel, func(t *testing.T) {
			root := t.TempDir()
			Init(root, "old")
			target := filepath.Join(root, targetName)
			raw, _ := os.ReadFile(filepath.Join(target, "manifest.json"))
			var m manifest
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatal(err)
			}
			m.Managed[rel] = managed{BaselineSHA256: "old"}
			saveManifest(target, m)
			if _, err := MigrateWithChoice(root, "new", "overwrite"); err == nil {
				t.Fatal("user-owned manifest path accepted", rel)
			}
		})
	}
}
