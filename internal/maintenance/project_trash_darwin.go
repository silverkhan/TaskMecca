//go:build darwin

package maintenance

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

func validateTrashPlatform(source string) error { return nil }

var foundationOnce sync.Once
var foundationErr error
var objectiveMessage uintptr
var objectiveClass func(string) uintptr
var objectiveSelector func(string) uintptr

func loadFoundation() error {
	foundationOnce.Do(func() {
		_, foundationErr = purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if foundationErr != nil {
			return
		}
		library, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			foundationErr = err
			return
		}
		purego.RegisterLibFunc(&objectiveClass, library, "objc_getClass")
		purego.RegisterLibFunc(&objectiveSelector, library, "sel_registerName")
		objectiveMessage, foundationErr = purego.Dlsym(library, "objc_msgSend")
	})
	return foundationErr
}

func foundationMessage(object uintptr, selector string, args ...uintptr) uintptr {
	arguments := append([]uintptr{object, objectiveSelector(selector)}, args...)
	result, _, _ := purego.SyscallN(objectiveMessage, arguments...)
	return result
}

func foundationString(value string) uintptr {
	data := append([]byte(value), 0)
	result := foundationMessage(objectiveClass("NSString"), "stringWithUTF8String:", uintptr(unsafe.Pointer(&data[0])))
	runtime.KeepAlive(data)
	return result
}

func foundationText(value uintptr) string {
	if value == 0 {
		return ""
	}
	address := foundationMessage(value, "UTF8String")
	if address == 0 {
		return ""
	}
	var data []byte
	for n := uintptr(0); n < 1024*1024; n++ {
		character := *(*byte)(unsafe.Pointer(address + n))
		if character == 0 {
			break
		}
		data = append(data, character)
	}
	return string(data)
}

// Native Foundation API: no compiler, Finder Automation, ~/.Trash enumeration
// or permanent-delete fallback. Correct NSURL**/NSError** output pointers.
func recycleStagedFolder(source string, expected os.FileInfo) (string, error) {
	current, err := os.Lstat(source)
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(expected, current) {
		return "", fmt.Errorf("staged folder identity changed; folder preserved")
	}
	parent, err := openCleanupParent(filepath.Dir(source))
	if err != nil {
		return "", err
	}
	defer unix.Close(parent)
	var anchored unix.Stat_t
	if err := unix.Fstatat(parent, filepath.Base(source), &anchored, unix.AT_SYMLINK_NOFOLLOW); err != nil || anchored.Mode&unix.S_IFMT != unix.S_IFDIR {
		return "", fmt.Errorf("staging path changed; native Trash refused")
	}
	if err := loadFoundation(); err != nil {
		return "", err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := foundationMessage(foundationMessage(objectiveClass("NSAutoreleasePool"), "alloc"), "init")
	defer foundationMessage(pool, "drain")
	manager := foundationMessage(objectiveClass("NSFileManager"), "defaultManager")
	itemURL := foundationMessage(objectiveClass("NSURL"), "fileURLWithPath:", foundationString(source))
	// File-reference URL follows the staged inode, not a substituted name.
	reference := foundationMessage(itemURL, "fileReferenceURL")
	if reference == 0 {
		return "", fmt.Errorf("native file identity reference unavailable; folder preserved")
	}
	resolved := foundationText(foundationMessage(reference, "path"))
	info, err := os.Lstat(resolved)
	if err != nil || !os.SameFile(expected, info) {
		return "", fmt.Errorf("staging reference identity changed; native Trash refused")
	}
	var resultURL, nativeError uintptr
	success := foundationMessage(manager, "trashItemAtURL:resultingItemURL:error:", reference, uintptr(unsafe.Pointer(&resultURL)), uintptr(unsafe.Pointer(&nativeError)))
	if success == 0 {
		return "", fmt.Errorf("macOS native Trash denied: %s", foundationText(foundationMessage(nativeError, "localizedDescription")))
	}
	location := foundationText(foundationMessage(resultURL, "path"))
	if location == "" {
		return "", fmt.Errorf("native Trash moved item but recovery URL unavailable")
	}
	return location, nil
}
