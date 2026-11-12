package runtimeobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// T3Selector contains identities only. State and timestamps are never accepted
// from a caller-supplied receipt; they are read from the local live T3 store.
type T3Selector struct {
	ThreadID        string `json:"thread_id"`
	SessionID       string `json:"session_id"`
	RuntimeAgentID  string `json:"runtime_agent_id"`
	RunID           string `json:"run_id,omitempty"`
	NativeAttemptID string `json:"native_attempt_id,omitempty"`
}

type T3Receipt struct {
	ThreadID        string         `json:"thread_id"`
	RunID           string         `json:"run_id"`
	NativeAttemptID string         `json:"native_attempt_id"`
	SessionID       string         `json:"session_id"`
	Provider        string         `json:"provider"`
	State           CanonicalState `json:"state"`
	StartedAt       string         `json:"started_at"`
	CompletedAt     string         `json:"completed_at,omitempty"`
	ObservedAt      string         `json:"observed_at"`
	Origin          string         `json:"origin"`
}

// Schema-bound read-only adapter for the installed T3 v2 projection store.
// No T3 database path, SQL or state is supplied by the CLI caller. The live
// server's exclusive owner lock is required so an abandoned running projection
// cannot authorize execution. Unsupported stores fail closed.
type t3Store struct{ dir string }

func localT3Store() (t3Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return t3Store{}, err
	}
	return t3Store{filepath.Join(home, ".t3", "userdata")}, nil
}
func (s t3Store) query(sql string) ([]byte, error) {
	for _, name := range []string{"statev2.sqlite", "server-owner.sqlite"} {
		p := filepath.Join(s.dir, name)
		info, err := os.Lstat(p)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("T3 source unavailable or unsafe: %s", name)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// T3 holds this database exclusively for the lifetime of its server.
	owner := exec.CommandContext(ctx, "sqlite3", "-readonly", filepath.Join(s.dir, "server-owner.sqlite"), "SELECT count(*) FROM sqlite_master;")
	output, err := owner.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "database is locked") {
		return nil, errors.New("T3 live server ownership lock not verified")
	}
	return exec.CommandContext(ctx, "sqlite3", "-readonly", "-json", filepath.Join(s.dir, "statev2.sqlite"), sql).Output()
}
func sqlLiteral(v string) string { return "'" + strings.ReplaceAll(v, "'", "''") + "'" }
func (s t3Store) read(sel T3Selector, historical bool, now time.Time) (T3Receipt, error) {
	if sel.ThreadID == "" || sel.SessionID == "" || sel.RuntimeAgentID != sel.SessionID {
		return T3Receipt{}, errors.New("exact T3 thread and native session/runtime identity required")
	}
	// One transaction/snapshot, with latest run and latest retry selected by
	// ordinal. Native identity comes from T3's run-attempt, not its shared
	// provider-session ID or an unverified user report.
	query := `SELECT r.thread_id,r.run_id,r.provider,r.status AS run_status,r.completed_at AS run_completed,
 a.attempt_id,a.status AS attempt_status,json_extract(a.payload_json,'$.nativeThreadId') AS session_id,
 json_extract(a.payload_json,'$.startedAt') AS started_at,json_extract(a.payload_json,'$.completedAt') AS completed_at,
 p.driver,p.thread_id AS owner_thread,p.status AS provider_status,t.deleted_at,t.archived_at
 FROM orchestration_v2_projection_runs r
 JOIN orchestration_v2_projection_run_attempts a ON a.run_id=r.run_id AND a.thread_id=r.thread_id AND a.provider=r.provider
 JOIN orchestration_v2_projection_provider_threads p ON p.provider_thread_id=a.provider_thread_id AND p.provider_thread_id=r.provider_thread_id
 JOIN orchestration_v2_projection_threads t ON t.thread_id=r.thread_id
 WHERE r.thread_id=` + sqlLiteral(sel.ThreadID)
	if historical {
		query += ` AND r.run_id=` + sqlLiteral(sel.RunID)
	} else {
		query += ` AND r.ordinal=(SELECT max(ordinal) FROM orchestration_v2_projection_runs WHERE thread_id=r.thread_id)`
	}
	if historical && sel.NativeAttemptID != "" {
		query += ` AND a.attempt_id=` + sqlLiteral(sel.NativeAttemptID) + `;`
	} else {
		query += ` AND a.attempt_ordinal=(SELECT max(attempt_ordinal) FROM orchestration_v2_projection_run_attempts WHERE run_id=r.run_id);`
	}
	raw, err := s.query(query)
	if err != nil {
		return T3Receipt{}, fmt.Errorf("T3 structured observation unavailable: %w", err)
	}
	var rows []struct {
		Thread         string  `json:"thread_id"`
		Run            string  `json:"run_id"`
		Provider       string  `json:"provider"`
		RunStatus      string  `json:"run_status"`
		RunCompleted   *string `json:"run_completed"`
		Attempt        string  `json:"attempt_id"`
		Status         string  `json:"attempt_status"`
		Session        string  `json:"session_id"`
		Started        string  `json:"started_at"`
		Completed      *string `json:"completed_at"`
		Driver         string  `json:"driver"`
		Owner          string  `json:"owner_thread"`
		ProviderStatus string  `json:"provider_status"`
		Deleted        *string `json:"deleted_at"`
		Archived       *string `json:"archived_at"`
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		return T3Receipt{}, err
	}
	if len(rows) != 1 {
		return T3Receipt{}, errors.New("T3 receipt missing or ambiguous")
	}
	r := rows[0]
	if r.Thread != sel.ThreadID || r.Owner != sel.ThreadID || r.Provider != "codex" || r.Driver != "codex" || r.Session != sel.SessionID || r.Deleted != nil || r.Archived != nil || r.Attempt == "" || (sel.RunID != "" && r.Run != sel.RunID) {
		return T3Receipt{}, errors.New("T3 receipt identity/current run conflict")
	}
	started, e := time.Parse(time.RFC3339Nano, r.Started)
	if e != nil || started.After(now) {
		return T3Receipt{}, errors.New("T3 start timestamp invalid or future")
	}
	result := T3Receipt{ThreadID: r.Thread, RunID: r.Run, NativeAttemptID: r.Attempt, SessionID: r.Session, Provider: r.Provider, StartedAt: r.Started, ObservedAt: now.UTC().Format(time.RFC3339Nano), Origin: "t3_local_projection_v2"}
	switch r.Status {
	case "running":
		if historical || r.RunStatus != "running" || r.Completed != nil || r.RunCompleted != nil || r.ProviderStatus != "active" {
			return T3Receipt{}, errors.New("T3 running state conflict")
		}
		result.State = StateRunning
	case "completed", "interrupted", "errored":
		if (!historical && (r.Status != r.RunStatus || r.RunCompleted == nil)) || r.Completed == nil {
			return T3Receipt{}, errors.New("T3 terminal state conflict")
		}
		ended, e := time.Parse(time.RFC3339Nano, *r.Completed)
		if e != nil || ended.Before(started) || ended.After(now) {
			return T3Receipt{}, errors.New("T3 terminal timestamp invalid")
		}
		result.CompletedAt = *r.Completed
		switch r.Status {
		case "completed":
			result.State = StateCompleted
		case "interrupted":
			result.State = StateInterrupted
		default:
			result.State = StateErrored
		}
	default:
		return T3Receipt{}, errors.New("T3 state not observable as actual execution")
	}
	return result, nil
}

func ObserveT3Execution(project string, sel T3Selector, now time.Time) (Attempt, T3Receipt, error) {
	release, err := lockObservationControl()
	if err != nil {
		return Attempt{}, T3Receipt{}, err
	}
	defer release()
	control, err := ObservationStatus("codex")
	if err != nil {
		return Attempt{}, T3Receipt{}, err
	}
	if !control.Enabled {
		return Attempt{}, T3Receipt{}, errors.New("Codex runtime observation disabled")
	}
	store, err := localT3Store()
	if err != nil {
		return Attempt{}, T3Receipt{}, err
	}
	return observeT3Execution(project, sel, now, store.read)
}

type t3Reader func(T3Selector, bool, time.Time) (T3Receipt, error)

func observeT3Execution(project string, sel T3Selector, now time.Time, read t3Reader) (Attempt, T3Receipt, error) {
	release, err := lockHookEvents(project)
	if err != nil {
		return Attempt{}, T3Receipt{}, err
	}
	defer release()
	receipt, err := read(sel, false, now)
	if err != nil {
		return Attempt{}, receipt, err
	}
	ledger, err := BuildLedger(project, 20, now)
	if err != nil {
		return Attempt{}, receipt, err
	}
	episode := executionEpisodeID(receipt.Provider, receipt.SessionID, sel.RuntimeAgentID, receipt.NativeAttemptID)
	existingEpisode := false
	var pending *Attempt
	var prior *Attempt
	type closure struct {
		id      string
		receipt T3Receipt
	}
	var closures []closure
	for _, a := range ledger.Attempts {
		if a.RuntimeAgentID != sel.RuntimeAgentID {
			continue
		}
		if (a.Provider != "" && a.Provider != receipt.Provider) || (a.SessionID != "" && a.SessionID != receipt.SessionID) {
			return Attempt{}, receipt, errors.New("conflicting runtime provider/session")
		}
		if a.NativeReceipt != nil && a.NativeReceipt.NativeAttemptID == receipt.NativeAttemptID {
			episode = a.AttemptID
			existingEpisode = true
			if a.Terminal && a.CurrentState != receipt.State {
				return Attempt{}, receipt, errors.New("terminal attempt cannot reopen")
			}
		} else if a.Terminal && a.NativeReceipt != nil {
			if prior == nil || a.LastObservedAt > prior.LastObservedAt {
				copy := a
				prior = &copy
			}
		} else if !a.Terminal {
			if a.NativeReceipt != nil {
				oldSel := sel
				oldSel.RunID = a.NativeReceipt.RunID
				oldSel.NativeAttemptID = a.NativeReceipt.NativeAttemptID
				old, e := read(oldSel, true, now)
				if e != nil || old.NativeAttemptID != a.NativeReceipt.NativeAttemptID || old.State == StateRunning {
					return Attempt{}, receipt, errors.New("previous execution episode not verified terminal")
				}
				closures = append(closures, closure{a.AttemptID, old})
				copy := a
				prior = &copy
			} else if a.StartedAt == "" && a.CurrentState == StateRuntimeUnknown && a.BindingState == BindingBound && a.BindingSource == "dispatch" {
				if pending != nil {
					return Attempt{}, receipt, errors.New("ambiguous pending dispatch")
				}
				copy := a
				pending = &copy
			} else {
				return Attempt{}, receipt, errors.New("another live attempt requires explicit terminal observation")
			}
		}
	}
	if pending != nil {
		if existingEpisode && pending.AttemptID != episode {
			return Attempt{}, receipt, errors.New("pending dispatch has no fresh observed execution episode")
		}
		episode = pending.AttemptID
	}
	// Re-read after identity validation, under the same observation lock, to
	// reject concurrent resumes/retries/completions before committing evidence.
	check, err := read(sel, false, now)
	if err != nil || check != receipt {
		return Attempt{}, receipt, errors.New("T3 execution changed during observation; retry")
	}
	for _, a := range ledger.Attempts {
		if a.AttemptID == episode && a.NativeReceipt != nil && *a.NativeReceipt == receipt {
			return a, receipt, nil
		}
	}
	for _, c := range closures {
		if err = appendT3State(project, c.id, sel.RuntimeAgentID, c.receipt); err != nil {
			return Attempt{}, receipt, err
		}
	}
	if err = appendT3State(project, episode, sel.RuntimeAgentID, receipt); err != nil {
		return Attempt{}, receipt, err
	}
	if prior != nil && prior.BindingState == BindingBound && !existingEpisode && pending == nil {
		_, err = bindAttempt(project, episode, prior.TaskID, prior.AgentPath, "dispatch", prior.ParentAttemptID, prior.BindingEvidence, now)
		if err != nil {
			return Attempt{}, receipt, err
		}
	}
	ledger, err = BuildLedger(project, 20, now)
	if err != nil {
		return Attempt{}, receipt, err
	}
	for _, a := range ledger.Attempts {
		if a.AttemptID == episode {
			return a, receipt, nil
		}
	}
	return Attempt{}, receipt, errors.New("observed episode disappeared")
}
func appendT3State(project, episode, runtimeID string, r T3Receipt) error {
	e := ExecutionEvent{EventKind: "state", AttemptID: episode, Provider: r.Provider, SessionID: r.SessionID, RuntimeAgentID: runtimeID, TurnID: r.NativeAttemptID, ObservedAt: r.ObservedAt, State: r.State, Terminal: r.State != StateRunning, EvidenceSource: EvidenceT3, ObservationQuality: QualityObserved, NativeReceipt: &r, Reason: r.Origin}
	// Re-observation updates freshness, while retries at the same observation
	// boundary remain idempotent in the fold (event IDs include receipt).
	return AppendExecutionEvent(project, e)
}

// VerifyT3Attempt checks the *current* source again at authorization boundaries.
// Stored JSON and an old running snapshot are never sufficient to claim work.
func VerifyT3Attempt(a Attempt, now time.Time) error {
	if a.NativeReceipt == nil || a.StateEvidenceSource != EvidenceT3 {
		return errors.New("missing T3 execution provenance")
	}
	observed, err := time.Parse(time.RFC3339Nano, a.NativeReceipt.ObservedAt)
	if err != nil || now.Before(observed) || now.Sub(observed) > 2*time.Minute {
		return errors.New("T3 observation stale; observe current episode again")
	}
	store, err := localT3Store()
	if err != nil {
		return err
	}
	r, err := store.read(T3Selector{ThreadID: a.NativeReceipt.ThreadID, SessionID: a.SessionID, RuntimeAgentID: a.RuntimeAgentID, RunID: a.NativeReceipt.RunID}, false, now)
	if err != nil {
		return err
	}
	if r.NativeAttemptID != a.NativeReceipt.NativeAttemptID || r.State != a.CurrentState {
		return errors.New("T3 current execution no longer matches stored attempt")
	}
	return nil
}

// ReadT3Execution verifies the local source without touching a project.
func ReadT3Execution(sel T3Selector, now time.Time) (T3Receipt, error) {
	store, err := localT3Store()
	if err != nil {
		return T3Receipt{}, err
	}
	return store.read(sel, false, now)
}
