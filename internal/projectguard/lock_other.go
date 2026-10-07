//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd && !windows

package projectguard

func lock(bool) (func(), error) { return nil, ErrSuppressed }
