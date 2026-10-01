package handoff

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
)

func Prepare(req PrepareRequest, now time.Time) (PrepareResult, error) {
	req.Project = strings.TrimSpace(req.Project)
	if req.Project == "" {
		req.Project = "."
	}
	req.TaskID = strings.ToUpper(strings.TrimSpace(req.TaskID))
	req.SourceAgentPath = strings.TrimSpace(req.SourceAgentPath)
	req.TargetAgentPath = strings.TrimSpace(req.TargetAgentPath)
	if req.TargetAgentPath == "" {
		req.TargetAgentPath = "/root/controller"
	}
	if req.TaskID == "" {
		return PrepareResult{}, errors.New("task_id is required")
	}
	if req.SourceAgentPath == "" {
		return PrepareResult{}, errors.New("source agent path is required")
	}
	if !validEventType(req.EventType) {
		return PrepareResult{}, fmt.Errorf("unsupported handoff event: %s", req.EventType)
	}

	inspection, err := backlog.Inspect(req.Project, "", req.TaskID)
	if err != nil {
		return PrepareResult{}, err
	}
	if inspection["exists"] != true {
		return PrepareResult{}, fmt.Errorf("task not found: %s", req.TaskID)
	}
	raw, _ := inspection["raw_markdown"].(string)
	currentContractSHA, err := ContractFingerprint(raw)
	if err != nil {
		return PrepareResult{}, fmt.Errorf("contract fingerprint: %w", err)
	}
	contractSHA := currentContractSHA
	if expected := strings.ToLower(strings.TrimSpace(req.ExpectedContractSHA256)); expected != "" {
		if len(expected) != 64 {
			return PrepareResult{}, errors.New("expected contract sha256 must be 64 hex characters")
		}
		if _, decodeErr := hex.DecodeString(expected); decodeErr != nil {
			return PrepareResult{}, errors.New("expected contract sha256 must be hexadecimal")
		}
		contractSHA = expected
	}

	evidence := map[string]string{}
	if contractSHA != currentContractSHA {
		evidence["current_contract_sha256"] = currentContractSHA
	}
	idParts := []string{req.TaskID, string(req.EventType), contractSHA}
	switch req.EventType {
	case EventRegistrationReady:
		auth := "false"
		if req.ExecutionAuthorized {
			auth = "true"
		}
		evidence["execution_authorized"] = auth
		idParts = append(idParts, auth)
	case EventWorkerDone, EventWorkerBlocked:
		if strings.TrimSpace(req.SourceAttemptID) == "" {
			return PrepareResult{}, errors.New("worker event requires --source-attempt")
		}
		if strings.TrimSpace(req.ReportFile) == "" {
			return PrepareResult{}, errors.New("worker event requires --report-file")
		}
		reportPath := req.ReportFile
		if !filepath.IsAbs(reportPath) {
			reportPath = filepath.Join(req.Project, reportPath)
		}
		reportSHA, hashErr := reportFingerprint(reportPath)
		if hashErr != nil {
			return PrepareResult{}, fmt.Errorf("report fingerprint: %w", hashErr)
		}
		evidence["report_sha256"] = reportSHA
		idParts = append(idParts, req.SourceAttemptID, reportSHA)
	default:
		idParts = append(idParts, req.SourceAttemptID)
	}
	handoffID := stableID(idParts...)

	var result PrepareResult
	err = withLock(req.Project, func() error {
		ledger, buildErr := buildLedgerUnlocked(req.Project, now)
		if buildErr != nil {
			return buildErr
		}
		if existing, ok := ledger.Handoffs[handoffID]; ok {
			result = PrepareResult{
				HandoffID: handoffID, Duplicate: true, ContractSHA256: existing.ContractSHA256,
				Target: existing.Target, Action: existing.Action,
				RequiresFreshPreflight: existing.RequiresFreshPreflight, Reason: "already_prepared",
			}
			return nil
		}

		target := Target{AgentPath: req.TargetAgentPath}
		action := ActionReportOnly
		fresh := false
		reason := "execution_not_authorized"
		if !(req.EventType == EventRegistrationReady && !req.ExecutionAuthorized) {
			var resolveErr error
			target, action, fresh, reason, resolveErr = resolveTarget(
				req.Project, req.TargetAgentPath, req.SourceAttemptID, req.TargetAttemptID, now,
			)
			if resolveErr != nil {
				return resolveErr
			}
		}

		record := Record{
			HandoffID: handoffID, RecordKind: "prepared", EventType: req.EventType,
			TaskID: req.TaskID, ContractSHA256: contractSHA,
			SourceAgentPath: req.SourceAgentPath, SourceAttemptID: req.SourceAttemptID,
			TargetAgentPath: target.AgentPath, TargetAttemptID: target.AttemptID,
			TargetRuntimeAgentID: target.RuntimeAgentID, TargetRuntimeName: target.RuntimeName,
			TargetSessionID: target.SessionID, Provider: target.Provider, TargetState: target.State,
			Action: action, RequiresFreshPreflight: fresh,
			ObservedAt: now.UTC().Format(time.RFC3339Nano), Evidence: evidence,
		}
		if err := appendRecordUnlocked(req.Project, record); err != nil {
			return err
		}
		result = PrepareResult{
			HandoffID: handoffID, Duplicate: false, ContractSHA256: contractSHA,
			Target: target, Action: action, RequiresFreshPreflight: fresh, Reason: reason,
		}
		return nil
	})
	return result, err
}

func Claim(project, handoffID, recipient, claimantAttemptID string, now time.Time) (ClaimResult, error) {
	handoffID = strings.TrimSpace(handoffID)
	recipient = strings.TrimSpace(recipient)
	claimantAttemptID = strings.TrimSpace(claimantAttemptID)
	if handoffID == "" || recipient == "" || claimantAttemptID == "" {
		return ClaimResult{}, errors.New("handoff_id, recipient and claimant attempt_id are required")
	}
	var result ClaimResult
	err := withLock(project, func() error {
		ledger, err := buildLedgerUnlocked(project, now)
		if err != nil {
			return err
		}
		row, ok := ledger.Handoffs[handoffID]
		if !ok {
			return fmt.Errorf("handoff not found: %s", handoffID)
		}
		result.HandoffID = handoffID
		result.ClaimedBy = row.ClaimedBy
		result.ClaimedAttemptID = row.ClaimedAttemptID
		if row.Applied {
			result.AlreadyApplied = true
			return nil
		}
		if row.TaskID != "" && row.ContractSHA256 != "" {
			inspection, inspectErr := backlog.Inspect(project, "", row.TaskID)
			if inspectErr != nil {
				return inspectErr
			}
			if inspection["exists"] != true {
				result.ContractChanged = true
				result.Reason = "task_missing"
				return nil
			}
			raw, _ := inspection["raw_markdown"].(string)
			currentSHA, hashErr := ContractFingerprint(raw)
			if hashErr != nil {
				return hashErr
			}
			result.CurrentContractSHA256 = currentSHA
			if currentSHA != row.ContractSHA256 {
				result.ContractChanged = true
				result.Reason = "contract_changed"
				return nil
			}
		}
		if row.Target.AgentPath != "" && row.Target.AgentPath != recipient {
			result.ClaimConflict = true
			result.Reason = "claimant_path_mismatch"
			return nil
		}
		if row.Target.AttemptID != "" && row.Target.AttemptID != claimantAttemptID {
			result.ClaimConflict = true
			result.Reason = "claimant_attempt_mismatch"
			return nil
		}
		if row.ClaimedBy != "" {
			if row.ClaimedBy == recipient && row.ClaimedAttemptID == claimantAttemptID {
				result.AlreadyClaimed = true
			} else {
				result.ClaimConflict = true
				result.Reason = "claimed_by_other_runtime_attempt"
			}
			return nil
		}
		record := Record{
			HandoffID: handoffID, RecordKind: "claimed", ClaimedBy: recipient, ClaimedAttemptID: claimantAttemptID,
			ObservedAt: now.UTC().Format(time.RFC3339Nano),
		}
		if err := appendRecordUnlocked(project, record); err != nil {
			return err
		}
		result.Claimed = true
		result.ClaimedBy = recipient
		result.ClaimedAttemptID = claimantAttemptID
		return nil
	})
	return result, err
}

func Mark(project, handoffID, step, resultValue, evidence string, now time.Time) (MarkResult, error) {
	handoffID = strings.TrimSpace(handoffID)
	step = strings.ToLower(strings.TrimSpace(step))
	resultValue = strings.ToLower(strings.TrimSpace(resultValue))
	if handoffID == "" || step == "" {
		return MarkResult{}, errors.New("handoff_id and step are required")
	}
	if resultValue != "ok" && resultValue != "failed" && resultValue != "unknown" {
		return MarkResult{}, errors.New("result must be one of: ok, failed, unknown")
	}
	var result MarkResult
	err := withLock(project, func() error {
		ledger, err := buildLedgerUnlocked(project, now)
		if err != nil {
			return err
		}
		row, ok := ledger.Handoffs[handoffID]
		if !ok {
			return fmt.Errorf("handoff not found: %s", handoffID)
		}
		result = MarkResult{HandoffID: handoffID, Step: step, Result: resultValue, Applied: row.Applied}
		if step == "applied" {
			if row.Applied {
				result.Duplicate = true
				result.Applied = true
				return nil
			}
			if resultValue != "ok" {
				return errors.New("applied step only accepts result=ok")
			}
			if row.ClaimedBy == "" || row.ClaimedAttemptID == "" {
				return errors.New("handoff cannot be marked applied before claim")
			}
			record := Record{
				HandoffID: handoffID, RecordKind: "applied", Step: step, Result: resultValue,
				ObservedAt: now.UTC().Format(time.RFC3339Nano), Evidence: detailEvidence(evidence),
			}
			if err := appendRecordUnlocked(project, record); err != nil {
				return err
			}
			result.Applied = true
			return nil
		}
		if existing, exists := row.Steps[step]; exists {
			if existing.Result == resultValue && existing.Evidence == strings.TrimSpace(evidence) {
				result.Duplicate = true
				return nil
			}
			return fmt.Errorf("step %s already recorded as %s; do not overwrite handoff evidence", step, existing.Result)
		}
		kind := "step_unknown"
		if resultValue == "ok" {
			kind = "step_succeeded"
		}
		if resultValue == "failed" {
			kind = "step_failed"
		}
		record := Record{
			HandoffID: handoffID, RecordKind: kind, Step: step, Result: resultValue,
			ObservedAt: now.UTC().Format(time.RFC3339Nano), Evidence: detailEvidence(evidence),
		}
		if err := appendRecordUnlocked(project, record); err != nil {
			return err
		}
		return nil
	})
	return result, err
}

func Reconcile(project string, now time.Time) (Ledger, error) {
	ledger, err := BuildLedger(project, now)
	if err != nil {
		return Ledger{}, err
	}
	for id, row := range ledger.Handoffs {
		if row.Applied {
			continue
		}
		if dispatch, ok := row.Steps["dispatch"]; ok && dispatch.Result != "ok" {
			ledger.Findings = append(ledger.Findings, Finding{
				Severity: "warning", Code: "dispatch_" + dispatch.Result, HandoffID: id,
				Message: "handoff dispatch is not confirmed successful",
			})
			continue
		}
		if row.ClaimedBy != "" {
			ledger.Findings = append(ledger.Findings, Finding{
				Severity: "info", Code: "claimed_not_applied", HandoffID: id,
				Message: "handoff was claimed but has not been marked applied",
			})
		} else {
			ledger.Findings = append(ledger.Findings, Finding{
				Severity: "info", Code: "pending_handoff", HandoffID: id,
				Message: "handoff is prepared and awaiting delivery or claim",
			})
		}
	}
	return ledger, nil
}

func validEventType(value EventType) bool {
	switch value {
	case EventRegistrationReady, EventWorkerDone, EventWorkerBlocked, EventUserDecisionRequired, EventCompletionReport:
		return true
	default:
		return false
	}
}

func stableID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "handoff-" + hex.EncodeToString(sum[:])[:20]
}

func detailEvidence(value string) map[string]string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return map[string]string{"detail": value}
}
