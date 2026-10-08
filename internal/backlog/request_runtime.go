package backlog

import (
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"time"
)

// requestRuntime is never retained across requests. Every request replays current
// runtime evidence, including bindings, terminal events and recovery tombstones.
type requestRuntime struct {
	ledger runtimeobs.Ledger
	err    error
}

func readRequestRuntime(project string, limit int, now time.Time, shared ...requestRuntime) (runtimeobs.Ledger, error) {
	if len(shared) > 0 {
		ledger := shared[0].ledger
		ledger.Attempts = append([]runtimeobs.Attempt{}, ledger.Attempts...)
		for i := range ledger.Attempts {
			transitions := ledger.Attempts[i].RecentTransitions
			if limit > 0 && len(transitions) > limit {
				ledger.Attempts[i].RecentTransitions = transitions[len(transitions)-limit:]
			}
		}
		return ledger, shared[0].err
	}
	return runtimeobs.ReconcileLedger(project, limit, now)
}
