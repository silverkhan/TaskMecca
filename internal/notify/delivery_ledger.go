package notify

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DeliveryRecord contains no Telegram credentials or recipient identifiers.
type DeliveryRecord struct {
	EventID           string `json:"event_id"`
	TaskID            string `json:"task_id"`
	Kind              string `json:"kind"`
	Channel           string `json:"channel"`
	EventAt           string `json:"event_at"`
	State             string `json:"state"`
	Attempts          int    `json:"attempts"`
	CreatedAt         string `json:"created_at"`
	LastAttemptAt     string `json:"last_attempt_at,omitempty"`
	LastResponseAt    string `json:"last_response_at,omitempty"`
	LastError         string `json:"last_error,omitempty"`
	DuplicatePossible bool   `json:"duplicate_possible,omitempty"`
}

type deliveryLedger struct {
	Version int                       `json:"version"`
	Records map[string]DeliveryRecord `json:"records"`
}

func ledgerPath(project string) string {
	return filepath.Join(project, "_task_mecca", ".runtime", "notifications", "delivery_ledger.json")
}

func readLedger(project string) (deliveryLedger, error) {
	out := deliveryLedger{Version: 1, Records: map[string]DeliveryRecord{}}
	data, err := os.ReadFile(ledgerPath(project))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	if out.Records == nil {
		out.Records = map[string]DeliveryRecord{}
	}
	return out, nil
}

func saveLedger(project string, ledger deliveryLedger) error {
	path := ledgerPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, append(data, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// DeliveryRecords is a read-only diagnostic projection, filterable by task or event.
func DeliveryRecords(project, taskID, eventID string) ([]DeliveryRecord, error) {
	telegramMu.Lock()
	defer telegramMu.Unlock()
	ledger, err := readLedger(project)
	if err != nil {
		return nil, err
	}
	var records []DeliveryRecord
	for _, r := range ledger.Records {
		if taskID != "" && !strings.EqualFold(taskID, r.TaskID) {
			continue
		}
		if eventID != "" && eventID != r.EventID {
			continue
		}
		records = append(records, r)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].CreatedAt < records[j].CreatedAt })
	return records, nil
}

func recordFor(e Event, now string) DeliveryRecord {
	return DeliveryRecord{EventID: e.ID, TaskID: e.TaskID, Kind: e.Kind, Channel: "telegram", EventAt: e.At, State: "pending", CreatedAt: now}
}

// Deliver reconciles canonical event IDs against durable channel outcomes.
// A response lost after Telegram accepts a message remains explicitly uncertain:
// retrying it can duplicate the external message because Telegram has no idempotency key.
func Deliver(project string, events []Event) []error {
	telegramMu.Lock()
	defer telegramMu.Unlock()
	cfg, err := loadTelegram(project)
	if err != nil {
		return []error{err}
	}
	ledger, err := readLedger(project)
	if err != nil {
		return []error{err}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	seen := map[string]bool{}
	for _, id := range cfg.Delivered {
		seen[id] = true
	}
	dirty := false
	// A legacy consumed ID cannot safely be classified as sent: it may have been
	// skipped by configuration or chronology. It must never be replayed.
	for _, e := range events {
		if e.ID == "" {
			continue
		}
		if _, ok := ledger.Records[e.ID]; !ok && seen[e.ID] {
			r := recordFor(e, now)
			r.State = "consumed_legacy"
			ledger.Records[e.ID] = r
			dirty = true
		}
	}
	// Old installations have no activation watermark. Their visible history is
	// a baseline, not a backlog of messages to send.
	if strings.TrimSpace(cfg.ActivatedAt) == "" {
		cfg.ActivatedAt = now
		for _, e := range events {
			if e.ID == "" {
				continue
			}
			if _, ok := ledger.Records[e.ID]; !ok {
				r := recordFor(e, now)
				r.State = "suppressed_before_activation"
				ledger.Records[e.ID] = r
				dirty = true
			}
			if !seen[e.ID] {
				cfg.Delivered = append(cfg.Delivered, e.ID)
				seen[e.ID] = true
			}
			advanceDeliveryPhase(&cfg, e)
		}
		if err := saveLedger(project, ledger); err != nil {
			return []error{err}
		}
		if err := saveTelegram(project, cfg); err != nil {
			return []error{err}
		}
		return nil
	}
	// Recover a legacy lifecycle floor without relying on its truncated ID list.
	for _, e := range events {
		if r, ok := ledger.Records[e.ID]; ok && (r.State == "sent" || r.State == "consumed_legacy" || strings.HasPrefix(r.State, "suppressed")) {
			advanceDeliveryPhase(&cfg, e)
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return notificationEventLess(events[i], events[j]) })
	var errs []error
	for _, e := range events {
		if e.ID == "" {
			continue
		}
		r, ok := ledger.Records[e.ID]
		if !ok {
			r = recordFor(e, now)
			ledger.Records[e.ID] = r
			if err := saveLedger(project, ledger); err != nil {
				return append(errs, err)
			}
		}
		if r.State == "sent" || r.State == "consumed_legacy" || strings.HasPrefix(r.State, "suppressed") {
			continue
		}
		if r.State == "sending" {
			r.State = "uncertain"
			r.DuplicatePossible = true
			r.LastError = "previous attempt ended without a recorded response"
		}
		state := ""
		switch {
		case !notificationAfterActivation(e.At, cfg.ActivatedAt):
			state = "suppressed_before_activation"
		case shouldSuppressRegression(e.Kind, cfg.TaskPhases[strings.ToUpper(strings.TrimSpace(e.TaskID))]):
			state = "suppressed_regression"
		case !cfg.Enabled || cfg.ChatID == 0 || !cfg.Kinds[e.Kind]:
			state = "suppressed_by_configuration"
		}
		if state != "" {
			r.State = state
			ledger.Records[e.ID] = r
			advanceDeliveryPhase(&cfg, e)
			if err := saveLedger(project, ledger); err != nil {
				return append(errs, err)
			}
			if !seen[e.ID] {
				cfg.Delivered = append(cfg.Delivered, e.ID)
				seen[e.ID] = true
			}
			dirty = true
			continue
		}
		r.Attempts++
		r.State = "sending"
		r.LastAttemptAt = time.Now().UTC().Format(time.RFC3339Nano)
		ledger.Records[e.ID] = r
		if err := saveLedger(project, ledger); err != nil {
			return append(errs, err)
		}
		sendErr := (telegramChannel{cfg}).Deliver(e)
		r.LastResponseAt = time.Now().UTC().Format(time.RFC3339Nano)
		if sendErr != nil {
			r.State = "failed"
			// Error classes are deliberately fixed: HTTP errors can contain bot tokens,
			// Telegram descriptions or recipient identifiers.
			r.LastError = "telegram request failed or response was unavailable"
			r.DuplicatePossible = true
			errs = append(errs, sendErr)
		} else {
			r.State = "sent"
			r.LastError = ""
			advanceDeliveryPhase(&cfg, e)
			if !seen[e.ID] {
				cfg.Delivered = append(cfg.Delivered, e.ID)
				seen[e.ID] = true
			}
			dirty = true
		}
		ledger.Records[e.ID] = r
		if err := saveLedger(project, ledger); err != nil {
			return append(errs, err)
		}
	}
	if len(cfg.Delivered) > 250 {
		cfg.Delivered = append([]string{}, cfg.Delivered[len(cfg.Delivered)-250:]...)
	}
	if dirty {
		if err := saveTelegram(project, cfg); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
