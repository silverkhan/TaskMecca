package runtimeobs

import (
	"fmt"
	"sort"
	"time"
)

const sessionRecentWindow = 48 * time.Hour

// Usage is a read-only presentation projection. Observation, binding, naming,
// liveness and retention clocks are deliberately not inputs to this decision.
func attemptUsage(a Attempt, now time.Time) (string, string) {
	latest, source := "", ""
	accept := func(value, provenance string) {
		at, err := time.Parse(time.RFC3339Nano, value)
		if err != nil || at.After(now) || at.IsZero() {
			return
		}
		if latest == "" || valueTime(latest).Before(at) {
			latest, source = at.UTC().Format(time.RFC3339Nano), provenance
		}
	}
	if a.HookActivityCount > 0 {
		accept(a.LastHookActivityAt, "observed_hook_activity")
	}
	// Receipt timestamps are provider execution boundaries, never ObservedAt.
	if r := a.NativeReceipt; r != nil && a.StateEvidenceSource == EvidenceT3 && HasObservedExecution(a) && r.ThreadID != "" && r.RunID != "" && r.NativeAttemptID != "" {
		observed, err := time.Parse(time.RFC3339Nano, r.ObservedAt)
		if err == nil && !observed.After(now) {
			for _, value := range []string{r.StartedAt, r.CompletedAt} {
				if at, err := time.Parse(time.RFC3339Nano, value); err == nil && !at.After(observed) {
					accept(value, "native_execution_boundary")
				}
			}
		}
	}
	return latest, source
}

func valueTime(value string) time.Time { at, _ := time.Parse(time.RFC3339Nano, value); return at }

func applySessionUsage(roots []RootSession, attempts []Attempt, now time.Time) {
	type usage struct{ at, source string }
	byRoot := map[string]usage{}
	for _, a := range attempts {
		at, source := attemptUsage(a, now)
		id := RootSessionIDForAttempt(a)
		if at != "" && (byRoot[id].at == "" || valueTime(byRoot[id].at).Before(valueTime(at))) {
			byRoot[id] = usage{at, source}
		}
	}
	for i := range roots {
		u := byRoot[roots[i].RootSessionID]
		roots[i].LastUsedAt, roots[i].UsageSource = u.at, u.source
		roots[i].UsageGroup = "historical"
		if u.at != "" && !valueTime(u.at).Before(now.Add(-sessionRecentWindow)) {
			roots[i].UsageGroup = "active"
		}
	}
}

type SessionWorkload struct {
	Project         string          `json:"project"`
	GeneratedAt     string          `json:"generated_at"`
	Active          RootSessionPage `json:"active"`
	Historical      RootSessionPage `json:"historical"`
	LatestSessionID string          `json:"latest_session_id,omitempty"`
	UnknownUsage    int             `json:"unknown_usage"`
	RuntimeCounts   map[string]int  `json:"runtime_counts"`
}

func usagePage(rows []RootSession, page, size int) RootSessionPage {
	if size < 1 {
		size = 10
	}
	if size > 50 {
		size = 50
	}
	pages := (len(rows) + size - 1) / size
	if page < 1 {
		page = 1
	}
	if pages > 0 && page > pages {
		page = pages
	}
	start := (page - 1) * size
	if start > len(rows) {
		start = len(rows)
	}
	end := start + size
	if end > len(rows) {
		end = len(rows)
	}
	return RootSessionPage{Items: append([]RootSession{}, rows[start:end]...), Page: page, PageSize: size, Total: len(rows), TotalPages: pages}
}

// QuerySessionWorkload groups the complete project before pagination. One clock
// and snapshot drive both counts and pages, including the exact 48-hour boundary.
func QuerySessionWorkload(project string, ledger Ledger, activePage, historicalPage, size int, now time.Time) (SessionWorkload, error) {
	roots, err := BuildRootSessions(project, ledger, now)
	if err != nil {
		return SessionWorkload{}, err
	}
	sort.Slice(roots.Items, func(i, j int) bool {
		a, b := roots.Items[i], roots.Items[j]
		if !valueTime(a.LastUsedAt).Equal(valueTime(b.LastUsedAt)) {
			return valueTime(a.LastUsedAt).After(valueTime(b.LastUsedAt))
		}
		return a.RootSessionID < b.RootSessionID
	})
	out := SessionWorkload{Project: project, GeneratedAt: now.UTC().Format(time.RFC3339Nano)}
	out.RuntimeCounts = map[string]int{"active": roots.Active, "needs_check": roots.NeedsCheck, "terminal": roots.Terminal, "cleanup_eligible": roots.CleanupEligible, "total": roots.Total}
	active, historical := []RootSession{}, []RootSession{}
	for _, root := range roots.Items {
		if root.LastUsedAt == "" {
			out.UnknownUsage++
		}
		if root.UsageGroup == "active" {
			active = append(active, root)
		} else {
			historical = append(historical, root)
		}
	}
	if len(active) > 0 {
		out.LatestSessionID = active[0].RootSessionID
	}
	out.Active, out.Historical = usagePage(active, activePage, size), usagePage(historical, historicalPage, size)
	return out, nil
}

type SessionWorkloadDetail struct {
	Project     string          `json:"project"`
	RootSession RootSession     `json:"root_session"`
	Attempts    []Attempt       `json:"attempts"`
	Findings    []LedgerFinding `json:"findings"`
	Page        int             `json:"page"`
	TotalPages  int             `json:"total_pages"`
	Total       int             `json:"total"`
}

func QuerySessionWorkloadDetail(project string, ledger Ledger, id string, page int, now time.Time) (SessionWorkloadDetail, error) {
	root, err := FindRootSession(project, ledger, id, now)
	if err != nil {
		return SessionWorkloadDetail{}, err
	}
	if root.ProviderSessionID != "" {
		storage, storageErr := RootSessionStorageUsage(project, root.Provider, root.ProviderSessionID)
		if storageErr != nil {
			return SessionWorkloadDetail{}, storageErr
		}
		root.StorageBytes = storage.TotalBytes
		if root.CleanupEligible {
			root.CleanupBytes = storage.TotalBytes
		}
	}
	attempts, err := rootSessionAttempts(project, ledger, now)
	if err != nil {
		return SessionWorkloadDetail{}, err
	}
	rows := []Attempt{}
	for _, a := range attempts {
		if RootSessionIDForAttempt(a) == id {
			rows = append(rows, a)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].AttemptID < rows[j].AttemptID })
	pages := (len(rows) + 19) / 20
	if page < 1 {
		page = 1
	}
	if pages > 0 && page > pages {
		page = pages
	}
	start := (page - 1) * 20
	if start > len(rows) {
		start = len(rows)
	}
	end := start + 20
	if end > len(rows) {
		end = len(rows)
	}
	if len(rows) == 0 {
		return SessionWorkloadDetail{}, fmt.Errorf("session has no attempts")
	}
	items := append([]Attempt{}, rows[start:end]...)
	return SessionWorkloadDetail{Project: project, RootSession: root, Attempts: items, Findings: FilterFindingsForAttempts(ledger.Findings, items), Page: page, TotalPages: pages, Total: len(rows)}, nil
}
