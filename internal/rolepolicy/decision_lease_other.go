//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd && !windows

package rolepolicy

import "errors"

func decisionReadLease(string) (func(), error) {
	return nil, errors.New("decision_read_lease_unsupported_platform")
}
