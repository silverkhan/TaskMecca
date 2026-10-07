//go:build darwin

package maintenance

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unsafe"
)

func restoreNativeTrashFixture(location, stage string) error {
	if err := loadFoundation(); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := foundationMessage(foundationMessage(objectiveClass("NSAutoreleasePool"), "alloc"), "init")
	defer foundationMessage(pool, "drain")
	manager := foundationMessage(objectiveClass("NSFileManager"), "defaultManager")
	var nativeError uintptr
	if foundationMessage(manager, "moveItemAtPath:toPath:error:", foundationString(location), foundationString(stage), uintptr(unsafe.Pointer(&nativeError))) == 0 {
		return fmt.Errorf("native restore: %s", foundationText(foundationMessage(nativeError, "localizedDescription")))
	}
	return nil
}

func TestRecoverA22NativeUIFixture(t *testing.T) {
	root := os.Getenv("TASK_MECCA_A22_UI_FIXTURE_ROOT")
	if root == "" {
		t.Skip("explicit isolated UI fixture recovery only")
	}
	if !strings.HasPrefix(root, "/private/tmp/a22-raichyu-") {
		t.Fatal("not an A22 isolated fixture")
	}
	data, err := os.ReadFile(filepath.Join(root, "home", "projects.json"))
	if err != nil {
		t.Fatal(err)
	}
	var registry projectRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatal(err)
	}
	for _, record := range registry.History {
		if !strings.Contains(record.Path, "/Alpha/Same Name") || !strings.HasPrefix(record.Path, root+"/") {
			continue
		}
		values, err := url.ParseQuery(strings.TrimPrefix(record.TrashPath, "system-trash:?"))
		if err != nil {
			t.Fatal(err)
		}
		location, stage := values.Get("location"), values.Get("staged")
		if !strings.HasPrefix(stage, root+"/") || !strings.HasPrefix(location, "/Users/") {
			t.Fatal("unexpected recovery binding")
		}
		if _, err := os.Lstat(record.Path); !os.IsNotExist(err) {
			t.Fatal("original collision; recovery not attempted")
		}
		if err := loadFoundation(); err != nil {
			t.Fatal(err)
		}
		runtime.LockOSThread()
		pool := foundationMessage(foundationMessage(objectiveClass("NSAutoreleasePool"), "alloc"), "init")
		var nativeError uintptr
		text := foundationMessage(foundationMessage(objectiveClass("NSString"), "alloc"), "initWithContentsOfFile:encoding:error:", foundationString(filepath.Join(location, "payload.txt")), 4, uintptr(unsafe.Pointer(&nativeError)))
		payload := foundationText(text)
		foundationMessage(pool, "drain")
		runtime.UnlockOSThread()
		expected := "A22 isolated native recoverable folder payload. 한글 and spaces.\n"
		if sha256.Sum256([]byte(payload)) != sha256.Sum256([]byte(expected)) {
			t.Fatal("fixture payload mismatch; native bin untouched")
		}
		if err := restoreNativeTrashFixture(location, stage); err != nil {
			t.Fatal(err)
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
		if err := moveProjectFolder(stage, record.Path, info); err != nil {
			t.Fatal(err)
		}
		restored, err := os.ReadFile(filepath.Join(record.Path, "payload.txt"))
		if err != nil || sha256.Sum256(restored) != sha256.Sum256([]byte(expected)) {
			t.Fatal("UI fixture restore hash mismatch", err)
		}
		t.Logf("A22_UI_NATIVE_RECOVERY_PASS original=%s staged=%s sha256=%x", record.Path, stage, sha256.Sum256(restored))
		return
	}
	t.Fatal("exact A22 moved UI fixture record not found")
}

func TestA22ReadOnlyBridgeTrialTrashInventory(t *testing.T) {
	if os.Getenv("TASK_MECCA_A22_BRIDGE_INVENTORY") != "1" {
		t.Skip("read-only bridge trial inventory opt-in")
	}
	if err := loadFoundation(); err != nil {
		t.Fatal(err)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := foundationMessage(foundationMessage(objectiveClass("NSAutoreleasePool"), "alloc"), "init")
	defer foundationMessage(pool, "drain")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	var nativeError uintptr
	names := foundationMessage(foundationMessage(objectiveClass("NSFileManager"), "defaultManager"), "contentsOfDirectoryAtPath:error:", foundationString(filepath.Join(home, ".Trash")), uintptr(unsafe.Pointer(&nativeError)))
	if names == 0 {
		t.Log("BRIDGE_TRIAL_INVENTORY_DENIED", foundationText(foundationMessage(nativeError, "localizedDescription")))
		return
	}
	count := foundationMessage(names, "count")
	for i := uintptr(0); i < count; i++ {
		name := foundationText(foundationMessage(names, "objectAtIndex:", i))
		if strings.HasPrefix(name, "TaskMecca recycle fixture 한글 space ") {
			t.Log("PRESERVED_BRIDGE_TRIAL_ITEM", name)
		}
	}
}
