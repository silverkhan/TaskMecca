//go:build darwin || linux || freebsd || openbsd || netbsd

package rolepolicy

import (
	"errors"
	"os"
	"syscall"
	"time"
)

func decisionReadLease(path string) (func(), error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if e != nil {
		return nil, e
	}
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("decision_lock_nonregular")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		e = syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
		if e == nil {
			return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
		}
		if (!errors.Is(e, syscall.EWOULDBLOCK) && !errors.Is(e, syscall.EAGAIN)) || time.Now().After(deadline) {
			f.Close()
			return nil, e
		}
		time.Sleep(10 * time.Millisecond)
	}
}
