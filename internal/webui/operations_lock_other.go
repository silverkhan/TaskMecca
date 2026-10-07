//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd && !windows

package webui

import "fmt"

func lockOperationJournal(string) (func(), error) {
	return nil, fmt.Errorf("operation reconciliation lock unsupported on this platform")
}
