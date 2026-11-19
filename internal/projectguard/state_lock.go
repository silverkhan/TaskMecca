package projectguard

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
)

var stateLockName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// AcquireStateLock fences management first, then serializes one named project's
// state across processes. Keep the stable lock inode; never unlink a held lock.
func AcquireStateLock(project, name string) (func(), error) {
	if !stateLockName.MatchString(name) {
		return nil, errors.New("invalid project state lock name")
	}
	releaseGuard, err := AcquireWrite(project)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (func(), error) { releaseGuard(); return nil, err }
	root, err := CanonicalPath(project)
	if err != nil {
		return fail(err)
	}
	dir := filepath.Join(root, "_task_mecca", ".runtime")
	// Reject substituted runtime/lock symlinks rather than locking another file.
	for _, path := range []string{filepath.Join(root, "_task_mecca"), dir, filepath.Join(dir, name+".lock")} {
		info, e := os.Lstat(path)
		if e == nil && info.Mode()&os.ModeSymlink != 0 {
			return fail(errors.New("project state lock symlink rejected"))
		}
		if e != nil && !os.IsNotExist(e) {
			return fail(e)
		}
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return fail(err)
	}
	file, err := os.OpenFile(filepath.Join(dir, name+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fail(err)
	}
	unlock, err := lockStateFile(file)
	if err != nil {
		file.Close()
		return fail(err)
	}
	return func() { unlock(); _ = file.Close(); releaseGuard() }, nil
}
