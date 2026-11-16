package rolepolicy

import (
	"os"
	"path/filepath"
)

// WithDecisionReadLease shares the existing contract CLI registration/decision
// writer lock. It adds no ledger or manager. Keep the final local revalidation
// and journal append in one lease so supported metadata/consent writers cannot
// replace their inputs between the final check and receipt publication.
func WithDecisionReadLease(project string, fn func() error) error {
	path := filepath.Join(project, "_task_mecca", "data", "cli_batches", ".registration.lock")
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	release, e := decisionReadLease(path)
	if e != nil {
		return e
	}
	defer release()
	return fn()
}
