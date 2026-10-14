package maintenance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExecutionTarget describes the installed executable, not an extra Preview
// process. A PR target is an explicit, pinned child of the Dev release channel.
type ExecutionTarget struct {
	Kind string `json:"kind"` // stable | dev | pr
	Channel string `json:"channel"` // stable or dev
	PRNumber int `json:"pr_number,omitempty"`
	HeadSHA string `json:"head_sha,omitempty"`
	OriginalDevVersion string `json:"original_dev_version,omitempty"`
	OriginalDevSHA256 string `json:"original_dev_sha256,omitempty"`
	OriginalDevBackup string `json:"original_dev_backup,omitempty"`
	InstalledAt string `json:"installed_at,omitempty"`
}

// ExecutionTransition keeps the last incomplete transaction visible after a
// crash. It is not a claim that a restart, install or rollback succeeded.
type ExecutionTransition struct {
	Schema int `json:"schema"`
	Phase string `json:"phase"` // preparing | installing | restarting | verifying | rollback_required
	From ExecutionTarget `json:"from"`
	To ExecutionTarget `json:"to"`
	UpdatedAt string `json:"updated_at"`
}

const (
	executionTargetFile = "execution-target.json"
	executionTransitionFile = "execution-transition.json"
	maxExecutionRecord = 16 << 10
)

func executionTargetPath() string { return filepath.Join(homeDir(), executionTargetFile) }
func executionTransitionPath() string { return filepath.Join(homeDir(), executionTransitionFile) }

func validCommitSHA(value string) bool {
	if len(value) != 40 { return false }
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) { return false }
	}
	return true
}
func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 { return false }
	if strings.ToLower(value) != value { return false }
	bytes, err := hex.DecodeString(value)
	return err == nil && len(bytes) == sha256.Size
}

func (t ExecutionTarget) validate() error {
	if t.Kind != "stable" && t.Kind != "dev" && t.Kind != "pr" {
		return fmt.Errorf("unsupported execution target %q", t.Kind)
	}
	if t.Kind == "stable" || t.Kind == "dev" {
		if t.Channel != t.Kind || t.PRNumber != 0 || t.HeadSHA != "" ||
			t.OriginalDevVersion != "" || t.OriginalDevSHA256 != "" || t.OriginalDevBackup != "" {
			return errors.New("release target has invalid PR-specific metadata")
		}
		return nil
	}
	if t.Channel != "dev" || t.PRNumber <= 0 || !validCommitSHA(t.HeadSHA) ||
		strings.TrimSpace(t.OriginalDevVersion) == "" || !validSHA256(t.OriginalDevSHA256) ||
		t.OriginalDevBackup == "" || !filepath.IsAbs(t.OriginalDevBackup) {
		return errors.New("PR target missing pinned identity or exact Dev return baseline")
	}
	if _, err := time.Parse(time.RFC3339, t.InstalledAt); err != nil {
		return errors.New("PR target has invalid installed_at")
	}
	return nil
}

// readExecutionJSON refuses symlinks, nonregular files, oversized records and
// trailing JSON. A damaged record is an error, never silently treated as Dev.
func readExecutionJSON(path string, target any) error {
	info, err := os.Lstat(path)
	if err != nil { return err }
	if !info.Mode().IsRegular() || info.Size() > maxExecutionRecord {
		return errors.New("execution state must be a small regular file")
	}
	file, err := os.Open(path)
	if err != nil { return err }
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, maxExecutionRecord+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil { return err }
	var remaining any
	if err := decoder.Decode(&remaining); !errors.Is(err, io.EOF) {
		return errors.New("execution state has trailing JSON")
	}
	return nil
}

// InstalledExecutionTarget reports the selected runtime target. It preserves
// the existing stable/dev semantics when no PR record exists.
func InstalledExecutionTarget() (ExecutionTarget, error) {
	channel := CurrentChannel()
	if channel != devChannel {
		if _, err := os.Lstat(executionTargetPath()); err == nil {
			return ExecutionTarget{}, errors.New("stale PR execution marker conflicts with Stable channel")
		} else if !errors.Is(err, os.ErrNotExist) {
			return ExecutionTarget{}, err
		}
		return ExecutionTarget{Kind: stableChannel, Channel: stableChannel}, nil
	}
	var target ExecutionTarget
	if err := readExecutionJSON(executionTargetPath(), &target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ExecutionTarget{Kind: devChannel, Channel: devChannel}, nil
		}
		return ExecutionTarget{}, fmt.Errorf("PR execution state invalid: %w", err)
	}
	if err := target.validate(); err != nil { return ExecutionTarget{}, err }
	if target.Kind != "pr" { return ExecutionTarget{}, errors.New("unexpected release marker in PR execution state") }
	return target, nil
}

// PendingExecutionTransition exposes a crash recovery checkpoint without
// automatically modifying the operating service or restoring any user data.
func PendingExecutionTransition() (*ExecutionTransition, error) {
	var transition ExecutionTransition
	if err := readExecutionJSON(executionTransitionPath(), &transition); err != nil {
		if errors.Is(err, os.ErrNotExist) { return nil, nil }
		return nil, err
	}
	if transition.Schema != 1 || transition.UpdatedAt == "" {
		return nil, errors.New("execution transition schema is invalid")
	}
	switch transition.Phase {
	case "preparing", "installing", "restarting", "verifying", "rollback_required":
	default:
		return nil, errors.New("unknown execution transition phase")
	}
	if err := transition.From.validate(); err != nil { return nil, fmt.Errorf("invalid source target: %w", err) }
	if err := transition.To.validate(); err != nil { return nil, fmt.Errorf("invalid destination target: %w", err) }
	if _, err := time.Parse(time.RFC3339, transition.UpdatedAt); err != nil {
		return nil, errors.New("execution transition timestamp invalid")
	}
	return &transition, nil
}

// writeExecutionJSON writes a checkpoint atomically. Callers must serialize
// transitions and verify installer/health status before marking a target active.
func writeExecutionJSON(path string, value any) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() { return errors.New("execution state destination is not a regular file") }
	} else if !errors.Is(err, os.ErrNotExist) { return err }
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil { return err }
	if len(data) > maxExecutionRecord { return errors.New("execution state exceeds size limit") }
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil { return err }
	file, err := os.CreateTemp(filepath.Dir(path), ".execution-state-")
	if err != nil { return err }
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil { file.Close(); return err }
	if _, err := file.Write(append(data, '\n')); err != nil { file.Close(); return err }
	if err := file.Sync(); err != nil { file.Close(); return err }
	if err := file.Close(); err != nil { return err }
	return os.Rename(file.Name(), path)
}

// ensureReleaseTransitionAllowed is a shared fail-closed gate for Stable/Dev
// updates and channel changes. A later PR-switch executor will use its own
// journaled transition path instead of changing a PR process in-place.
func ensureReleaseTransitionAllowed() error {
	target, err := InstalledExecutionTarget()
	if err != nil { return fmt.Errorf("cannot change release while execution target is invalid: %w", err) }
	pending, err := PendingExecutionTransition()
	if err != nil { return fmt.Errorf("cannot change release with invalid transition journal: %w", err) }
	if pending != nil { return fmt.Errorf("environment transition is still %s; recover it first", pending.Phase) }
	if target.Kind == "pr" {
		return fmt.Errorf("PR #%d (%s) is active; return to the pinned Dev version before updating or switching channels", target.PRNumber, target.HeadSHA[:10])
	}
	return nil
}
