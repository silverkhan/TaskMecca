package handoff

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// NativeTransport is supplied by the agent host, not a CLI receipt import.
// Call returns the tool's structured result, preserving its original error.
// Hosts without these tools must not implement this as a successful no-op.
type NativeTransport interface {
	Call(tool string, arguments map[string]any) (json.RawMessage, error)
	FreshFullAccess(project string) error
}

// RejectedSend denotes an explicit tool rejection (e.g. agent not found), as
// opposed to a timeout/disconnection where delivery may already have happened.
type RejectedSend struct{ Err error }

func (e RejectedSend) Error() string { return e.Err.Error() }
func (e RejectedSend) Unwrap() error { return e.Err }

type DeliveryCheckpoint struct {
	ReportID        string           `json:"report_id"`
	HandoffID       string           `json:"handoff_id"`
	ReportFile      string           `json:"report_file"`
	ReportSHA256    string           `json:"report_sha256"`
	Recipient       TransportAddress `json:"recipient"`
	NativeID        string           `json:"native_id"`
	TargetAttemptID string           `json:"target_attempt_id"`
	Status          string           `json:"status"`
	OriginalError   string           `json:"original_error,omitempty"`
	LastError       string           `json:"last_error,omitempty"`
	Attempts        int              `json:"attempts"`
	UpdatedAt       string           `json:"updated_at"`
	Receipt         json.RawMessage  `json:"receipt,omitempty"`
	ActionOwner     string           `json:"action_owner"`
	ResumeCondition string           `json:"resume_condition"`
}

// DeliverReport does not mark dispatch, claim, review, lifecycle or applied.
// It sends only an existing, content-bound handoff to its verified same target.
// Acknowledged/uncertain sends are never automatically repeated. Explicit
// rejection can retry the SAME checkpoint after repair and fresh observation.
func DeliverReport(project, id, reportFile string, host NativeTransport) (DeliveryCheckpoint, error) {
	if host == nil {
		return DeliveryCheckpoint{}, errors.New("native transport unavailable")
	}
	row, ok, err := Inspect(project, id, time.Now())
	if err != nil {
		return DeliveryCheckpoint{}, err
	}
	if !ok {
		return DeliveryCheckpoint{}, errors.New("handoff missing")
	}
	if row.EventType != EventWorkerDone && row.EventType != EventWorkerBlocked {
		return DeliveryCheckpoint{}, errors.New("worker report handoff required")
	}
	sha, err := reportFingerprint(reportFile)
	if err != nil {
		return DeliveryCheckpoint{}, err
	}
	if stableID(row.TaskID, string(row.EventType), row.ContractSHA256, row.SourceAttemptID, sha) != id {
		return DeliveryCheckpoint{}, errors.New("report does not match prepared handoff")
	}
	plan, err := PlanTransport(project, id, time.Now())
	if err != nil {
		return DeliveryCheckpoint{}, err
	}
	return deliverPlannedReport(project, plan, reportFile, sha, host)
}

func deliverPlannedReport(project string, p TransportPlan, reportFile, sha string, host NativeTransport) (DeliveryCheckpoint, error) {
	c := DeliveryCheckpoint{ReportID: p.HandoffID, HandoffID: p.HandoffID, ReportFile: reportFile, ReportSHA256: sha, Recipient: p.Address, NativeID: p.Target.RuntimeAgentID, TargetAttemptID: p.Target.AttemptID, Status: "undelivered", ActionOwner: p.ActionOwner, ResumeCondition: p.ResumeCondition}
	// Never use user-controlled IDs in paths. A separate O_EXCL lock has no
	// stale-timer takeover while a live host call may still be delivering.
	path := filepath.Join(project, "_task_mecca", ".runtime", "handoffs", "transport", stableID(p.HandoffID)+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return c, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return c, fmt.Errorf("transport checkpoint busy; reconcile before retry: %w", err)
	}
	_ = lock.Close()
	defer os.Remove(path + ".lock")
	if raw, err := os.ReadFile(path); err == nil {
		if err = json.Unmarshal(raw, &c); err != nil {
			return c, err
		}
		if c.Recipient.Value == "" && c.Status == "undelivered" && c.Attempts == 0 && p.Supported {
			c.Recipient = p.Address
		}
		if c.HandoffID != p.HandoffID || c.ReportSHA256 != sha || c.NativeID != p.Target.RuntimeAgentID || c.Recipient != p.Address {
			return c, errors.New("checkpoint identity changed; recover existing handoff")
		}
		if c.Status != "undelivered" {
			if c.Status == "transport_acknowledged" {
				return c, nil
			}
			return c, errors.New("unresolved send; reconcile exact tool receipt before retry")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return c, err
	}
	save := func() error {
		c.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		raw, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			return err
		}
		tmp, err := os.CreateTemp(filepath.Dir(path), ".transport-*")
		if err != nil {
			return err
		}
		name := tmp.Name()
		defer os.Remove(name)
		if _, err = tmp.Write(raw); err == nil {
			err = tmp.Sync()
		}
		closeErr := tmp.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		return os.Rename(name, path)
	}
	fail := func(err error) (DeliveryCheckpoint, error) {
		if c.OriginalError == "" {
			c.OriginalError = err.Error()
		}
		c.LastError = err.Error()
		if writeErr := save(); writeErr != nil {
			return c, fmt.Errorf("%w; checkpoint persistence failed: %v", err, writeErr)
		}
		return c, err
	}
	if !p.Supported {
		return fail(fmt.Errorf("transport unsupported: %s", p.Reason))
	}
	if err := ValidateAddress(p.Tool, p.Address); err != nil {
		return fail(err)
	}
	// Re-read the exact thread immediately before any completed-target resume.
	raw, err := host.Call("t3_thread_read", map[string]any{"threadId": p.Address.Value})
	if err != nil {
		return fail(err)
	}
	var read struct {
		Thread struct {
			ID     string `json:"threadId"`
			Status string `json:"status"`
		} `json:"thread"`
		RecentRuns []struct {
			RunID string `json:"runId"`
		} `json:"recentRuns"`
	}
	if err = json.Unmarshal(raw, &read); err != nil {
		return fail(err)
	}
	if read.Thread.ID != p.Address.Value {
		return fail(errors.New("exact target read mismatch"))
	}
	if len(read.RecentRuns) == 0 || p.T3RunID == "" || read.RecentRuns[0].RunID != p.T3RunID {
		return fail(errors.New("current native episode changed; re-observe exact target and recover existing handoff"))
	}
	if read.Thread.Status != "running" && read.Thread.Status != "starting" && read.Thread.Status != "completed" {
		return fail(errors.New("target state cannot resume/message"))
	}
	if read.Thread.Status == "completed" || p.RequiresFreshPreflight {
		if err = host.FreshFullAccess(project); err != nil {
			return fail(err)
		}
	}
	c.TargetAttemptID = p.Target.AttemptID
	c.Attempts++
	c.Status = "sending"
	if err = save(); err != nil {
		return c, err
	} // durable before effect
	raw, err = host.Call(p.Tool, map[string]any{"threadId": p.Address.Value, "mode": "auto", "clientRequestId": c.ReportID, "message": fmt.Sprintf("%s report_id=%s handoff=%s report=%s sha256=%s; transport acknowledgement is not claim/applied; continue existing handoff recovery", p.EventType, c.ReportID, p.HandoffID, reportFile, sha)})
	if err != nil {
		c.Status = "delivery_unknown"
		var rejected RejectedSend
		if errors.As(err, &rejected) {
			c.Status = "undelivered"
		}
		return fail(err)
	}
	c.Receipt = raw
	var ack struct {
		ThreadID  string `json:"threadId"`
		MessageID string `json:"messageId"`
		Delivery  string `json:"delivery"`
	}
	if err = json.Unmarshal(raw, &ack); err != nil {
		c.Status = "delivery_unknown"
		return fail(err)
	}
	if ack.ThreadID != p.Address.Value || ack.MessageID == "" || (ack.Delivery != "started" && ack.Delivery != "queued" && ack.Delivery != "steered") {
		c.Status = "delivery_unknown"
		return fail(errors.New("tool receipt does not confirm exact recipient delivery"))
	}
	c.Status = "transport_acknowledged"
	if err = save(); err != nil {
		return c, err
	}
	return c, nil
}
