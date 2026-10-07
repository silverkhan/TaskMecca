//go:build darwin

package maintenance

import (
	"fmt"
	"runtime"
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
