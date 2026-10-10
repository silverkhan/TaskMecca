package notify

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/projectguard"
)

const resumeReason = "maintenance_resume_no_replay"

var persistResumeLedger = saveLedger

// ResumeBoundary is channel evidence only, never recipient configuration.
type ResumeBoundary struct {
	Version       int    `json:"version"`
	ID            string `json:"id"`
	CutoffAt      string `json:"cutoff_at"`
	EstablishedAt string `json:"established_at"`
	Reason        string `json:"reason"`
	BackupPath    string `json:"backup_path,omitempty"`
	BackupSHA256  string `json:"backup_sha256,omitempty"`
}

type DeliverySuppression struct {
	Reason        string `json:"reason"`
	EvidenceID    string `json:"evidence_id"`
	CutoffAt      string `json:"cutoff_at"`
	ClassifiedAt  string `json:"classified_at"`
	PreviousState string `json:"previous_state"`
}

type ResumeResult struct {
	Boundary      *ResumeBoundary `json:"boundary,omitempty"`
	Suppressed    int             `json:"suppressed"`
	SentPreserved int             `json:"sent_preserved"`
	Unchanged     bool            `json:"unchanged"`
}

func validateResumeBoundary(boundary *ResumeBoundary) error {
	if boundary == nil {
		return nil
	}
	if boundary.Version != 1 || boundary.ID == "" || boundary.Reason != resumeReason {
		return errors.New("invalid Telegram resume boundary")
	}
	cutoff, err := time.Parse(time.RFC3339Nano, boundary.CutoffAt)
	if err != nil || cutoff.IsZero() {
		return errors.New("invalid Telegram resume cutoff")
	}
	established, err := time.Parse(time.RFC3339Nano, boundary.EstablishedAt)
	if err != nil || established.Before(cutoff) {
		return errors.New("invalid Telegram resume evidence timestamp")
	}
	return nil
}

func terminalDelivery(record DeliveryRecord) bool {
	return record.State == "sent" || record.State == "consumed_legacy" || strings.HasPrefix(record.State, "suppressed")
}

func resumeSuppressionReason(event Event, record DeliveryRecord, boundary ResumeBoundary) string {
	cutoff, _ := time.Parse(time.RFC3339Nano, boundary.CutoffAt)
	at, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(event.At))
	if err != nil || at.IsZero() {
		return "invalid_or_missing_event_time"
	}
	if !at.After(cutoff) {
		return "event_at_or_before_resume_cutoff"
	}
	if at.After(time.Now().UTC()) {
		return "future_event_time"
	}
	if strings.TrimSpace(record.EventAt) == "" || strings.TrimSpace(record.CreatedAt) == "" || (record.Attempts > 0 && strings.TrimSpace(record.LastAttemptAt) == "") {
		return "missing_delivery_evidence_time"
	}
	// Changing a known event's timestamp must not resurrect an old attempt.
	for _, raw := range []string{record.EventAt, record.CreatedAt, record.LastAttemptAt} {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		timestamp, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return "invalid_delivery_evidence_time"
		}
		if !timestamp.After(cutoff) {
			return "delivery_evidence_at_or_before_resume_cutoff"
		}
	}
	switch record.State {
	case "pending", "failed", "sending", "uncertain", "sent", "consumed_legacy":
	default:
		if !strings.HasPrefix(record.State, "suppressed") {
			return "unrecognized_delivery_state"
		}
	}
	return ""
}

func suppressForResume(record DeliveryRecord, boundary ResumeBoundary, now, reason string) DeliveryRecord {
	previous := record.State
	record.State = "suppressed_before_resume"
	record.Suppression = &DeliverySuppression{Reason: reason, EvidenceID: boundary.ID, CutoffAt: boundary.CutoffAt, ClassifiedAt: now, PreviousState: previous}
	// Attempts, uncertainty, error, retry time and history remain truthful.
	return record
}

// safeResumePath refuses symlinked ledger/backup ancestors before any writes.
func safeResumePath(project, path string) error {
	root, err := filepath.Abs(project)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("notification path escapes project")
	}
	current := root
	for _, component := range append([]string{""}, strings.Split(rel, string(filepath.Separator))...) {
		if component != "" {
			current = filepath.Join(current, component)
		}
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("notification ledger symlink boundary rejected")
		}
	}
	return nil
}

// SuppressBeforeResume commits a monotonic boundary while transport is blocked.
// The caller supplies a read-only event snapshot; late discoveries are covered by
// the persistent cutoff. Every pre-existing unsettled attempt is suppressed on
// first/advanced cutover, including malformed/future-dated/unknown records.
func SuppressBeforeResume(project, cutoffAt string, events []Event) (ResumeResult, error) {
	var result ResumeResult
	if !filepath.IsAbs(project) {
		return result, errors.New("an absolute project path is required")
	}
	if !TelegramTransportDisabled() {
		return result, errors.New("Telegram transport must remain disabled while committing resume suppression")
	}
	cutoff, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(cutoffAt))
	nowTime := time.Now().UTC()
	if err != nil || cutoff.IsZero() || cutoff.After(nowTime) {
		return result, errors.New("cutoff must be a valid non-future RFC3339 timestamp")
	}
	cutoff = cutoff.UTC()
	cutoffAt = cutoff.Format(time.RFC3339Nano)
	releaseGuard, err := projectguard.AcquireWrite(project)
	if err != nil {
		return result, err
	}
	defer releaseGuard()
	telegramMu.Lock()
	defer telegramMu.Unlock()
	if err := safeResumePath(project, ledgerPath(project)); err != nil {
		return result, err
	}
	release, err := acquireDeliveryLock(project)
	if err != nil {
		return result, err
	}
	defer release()
	ledger, err := readLedger(project)
	if err != nil {
		return result, err
	}
	now := nowTime.Format(time.RFC3339Nano)
	advanced := ledger.ResumeBoundary == nil
	if ledger.ResumeBoundary != nil {
		old, _ := time.Parse(time.RFC3339Nano, ledger.ResumeBoundary.CutoffAt)
		if cutoff.Before(old) {
			return result, errors.New("resume cutoff cannot move backwards")
		}
		advanced = cutoff.After(old)
	}
	digest := sha256.Sum256([]byte(cutoffAt))
	boundary := ResumeBoundary{Version: 1, ID: "resume-" + hex.EncodeToString(digest[:12]), CutoffAt: cutoffAt, EstablishedAt: now, Reason: resumeReason}
	if !advanced {
		boundary = *ledger.ResumeBoundary
	}
	dirty := advanced
	for id, record := range ledger.Records {
		if record.State == "sent" {
			result.SentPreserved++
		}
		if terminalDelivery(record) {
			continue
		}
		reason := resumeSuppressionReason(Event{At: record.EventAt}, record, boundary)
		if advanced {
			reason = "unsettled_attempt_present_at_resume"
		}
		if reason != "" {
			ledger.Records[id] = suppressForResume(record, boundary, now, reason)
			result.Suppressed++
			dirty = true
		}
	}
	for _, event := range events {
		if event.ID == "" {
			continue
		}
		record, exists := ledger.Records[event.ID]
		if !exists {
			record = recordFor(event, now)
		}
		if terminalDelivery(record) {
			continue
		}
		if reason := resumeSuppressionReason(event, record, boundary); reason != "" {
			ledger.Records[event.ID] = suppressForResume(record, boundary, now, reason)
			result.Suppressed++
			dirty = true
		}
	}
	if !dirty {
		result.Boundary = ledger.ResumeBoundary
		result.Unchanged = true
		return result, nil
	}
	// Backup existing delivery evidence byte-for-byte before replacing it. Never
	// read or write credentials/configuration, or disguise suppression as sent.
	prior, readErr := os.ReadFile(ledgerPath(project))
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return result, readErr
	}
	if readErr == nil {
		dir := filepath.Join(filepath.Dir(ledgerPath(project)), "resume-backups")
		if err := safeResumePath(project, dir); err != nil {
			return result, err
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return result, err
		}
		file, err := os.CreateTemp(dir, "delivery-before-resume-*.json")
		if err != nil {
			return result, err
		}
		path := file.Name()
		writeErr := file.Chmod(0600)
		if writeErr == nil {
			_, writeErr = file.Write(prior)
		}
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil {
			return result, fmt.Errorf("resume backup failed: %w", writeErr)
		}
		if closeErr != nil {
			return result, closeErr
		}
		if advanced {
			boundary.BackupPath = path
			digest := sha256.Sum256(prior)
			boundary.BackupSHA256 = hex.EncodeToString(digest[:])
		}
	}
	if advanced && ledger.ResumeBoundary != nil {
		ledger.ResumeHistory = append(ledger.ResumeHistory, *ledger.ResumeBoundary)
	}
	ledger.ResumeBoundary = &boundary
	if err := persistResumeLedger(project, ledger); err != nil {
		return result, err
	}
	result.Boundary = &boundary
	return result, nil
}

// ResumeStatus is read-only and contains no token/chat/recipient data.
func ResumeStatus(project string) (ResumeResult, error) {
	telegramMu.Lock()
	defer telegramMu.Unlock()
	ledger, err := readLedger(project)
	if err != nil {
		return ResumeResult{}, err
	}
	result := ResumeResult{Boundary: ledger.ResumeBoundary}
	for _, record := range ledger.Records {
		if record.State == "suppressed_before_resume" {
			result.Suppressed++
		}
		if record.State == "sent" {
			result.SentPreserved++
		}
	}
	return result, nil
}
