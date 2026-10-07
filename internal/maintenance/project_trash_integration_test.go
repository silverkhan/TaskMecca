//go:build darwin || windows

package maintenance

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Native CI MUST opt in; cross-compilation is not recycle/restore evidence.
// Only this unique temporary fixture is trashed and restored; never empty Bin.
func TestNativeSystemTrashAndRestoreIntegration(t *testing.T) {
	if os.Getenv("TASK_MECCA_RUN_NATIVE_TRASH") != "1" {
		t.Skip("native recycle/restore requires explicit fixture opt-in")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TASK_MECCA_HOME", filepath.Join(dir, "management"))
	name := fmt.Sprintf("TaskMecca recycle fixture 한글 space %d %s", time.Now().UnixNano(), strings.Repeat("long-", 12))
	source := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Join(source, "_task_mecca"), 0700); err != nil {
		t.Fatal(err)
	}
	payload := []byte("A22 unique recovery fixture: 한글, spaces, original content\n")
	if err := os.WriteFile(filepath.Join(source, "payload.txt"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	if err := RegisterProject(source); err != nil {
		t.Fatal(err)
	}
	record, err := RemoveProject(source)
	if err != nil {
		t.Fatal(err)
	}
	locator, err := MoveRemovedProjectToTrash(record.ID, source)
	if err != nil {
		t.Fatalf("actual native recycle failed (not a cross-build PASS): %v; recovery=%s", err, locator)
	}
	if _, err := os.Lstat(source); !os.IsNotExist(err) {
		t.Fatalf("original was not moved: %v", err)
	}
	values, err := url.ParseQuery(strings.TrimPrefix(locator, "system-trash:?"))
	if err != nil {
		t.Fatal(err)
	}
	stage := values.Get("staged")
	if err := restoreNativeTrashFixture(values.Get("location"), stage); err != nil {
		t.Fatalf("native restore failed; fixture locator=%s: %v", locator, err)
	}
	f, err := os.Open(stage)
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := moveProjectFolder(stage, source, info); err != nil {
		t.Fatal("no-overwrite original restoration", err)
	}
	restored, err := os.ReadFile(filepath.Join(source, "payload.txt"))
	if err != nil || sha256.Sum256(restored) != sha256.Sum256(payload) {
		t.Fatalf("restored payload hash mismatch: %v", err)
	}
	if err := RegisterWebProject(source); err != nil {
		t.Fatal(err)
	}
	reg, err := readProjectRegistry()
	if err != nil || len(reg.Projects) != 0 {
		t.Fatalf("restore automatically registered removed project: %+v %v", reg, err)
	}
	if _, err := MoveRemovedProjectToTrash(record.ID, source); err == nil {
		t.Fatal("duplicate cleanup accepted")
	}
	t.Logf("NATIVE_RECYCLE_RESTORE_PASS platform=%s original=%s staged=%s sha256=%x", runtime.GOOS, source, stage, sha256.Sum256(restored))
}
