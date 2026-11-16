//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package rolepolicy

import (
	"errors"
	"os"
)

func openSnapshotPath(root, path string) (*os.File, error) {
	return nil, errors.New("safe_source_snapshot_unsupported_platform")
}
