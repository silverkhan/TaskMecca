//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd && !windows
package activity
import "errors"
func acquireActivityLock(string)(func(),error){return nil,errors.New("Activity writes unsupported on this platform without an exclusive file lock")}
