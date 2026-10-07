package handoffjournal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func JournalPath(project string) string {
	return filepath.Join(project, "_task_mecca", ".runtime", "handoffs", "events.jsonl")
}
func BuildLedger(project string, now time.Time) (Ledger, error) {
	out := Ledger{
		Version: 1, GeneratedAt: now.UTC().Format(time.RFC3339Nano),
		JournalPath: JournalPath(project), Handoffs: map[string]Handoff{}, Findings: []Finding{},
	}
	f, err := os.Open(out.JournalPath)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var record Record
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			out.Findings = append(out.Findings, Finding{
				Severity: "warning", Code: "invalid_record_line",
				Message: fmt.Sprintf("handoff journal line %d is invalid JSON: %v", line, err),
			})
			continue
		}
		if strings.TrimSpace(record.HandoffID) == "" {
			out.Findings = append(out.Findings, Finding{
				Severity: "warning", Code: "missing_handoff_id",
				Message: fmt.Sprintf("handoff journal line %d has no handoff_id", line),
			})
			continue
		}
		row := out.Handoffs[record.HandoffID]
		if row.HandoffID == "" {
			row = Handoff{HandoffID: record.HandoffID, Steps: map[string]StepState{}}
		}
		if row.Steps == nil {
			row.Steps = map[string]StepState{}
		}
		row.RecordCount++
		switch record.RecordKind {
		case "prepared":
			row.EventType = record.EventType
			row.TaskID = record.TaskID
			row.ContractSHA256 = record.ContractSHA256
			row.SourceAgentPath = record.SourceAgentPath
			row.SourceAttemptID = record.SourceAttemptID
			row.Target = Target{
				AgentPath: record.TargetAgentPath, AttemptID: record.TargetAttemptID,
				RuntimeAgentID: record.TargetRuntimeAgentID, RuntimeName: record.TargetRuntimeName,
				SessionID: record.TargetSessionID, Provider: record.Provider, State: record.TargetState,
			}
			row.Action = record.Action
			row.RequiresFreshPreflight = record.RequiresFreshPreflight
			if row.PreparedAt == "" {
				row.PreparedAt = record.ObservedAt
			}
		case "retargeted":
			row.Target = Target{AgentPath: record.TargetAgentPath, AttemptID: record.TargetAttemptID, RuntimeAgentID: record.TargetRuntimeAgentID, RuntimeName: record.TargetRuntimeName, SessionID: record.TargetSessionID, Provider: record.Provider, State: record.TargetState}
			row.Action = record.Action
			row.RequiresFreshPreflight = record.RequiresFreshPreflight
		case "retarget_hold":
			row.Action = ActionHold
		case "claimed":
			if row.ClaimedBy == "" {
				row.ClaimedBy = record.ClaimedBy
				row.ClaimedAttemptID = record.ClaimedAttemptID
				row.ClaimedAt = record.ObservedAt
			}
		case "step_succeeded", "step_failed", "step_unknown":
			if _, exists := row.Steps[record.Step]; !exists {
				row.Steps[record.Step] = StepState{
					Result: record.Result, Evidence: record.Evidence["detail"], ObservedAt: record.ObservedAt,
				}
			}
		case "applied":
			row.Applied = true
			if row.AppliedAt == "" {
				row.AppliedAt = record.ObservedAt
			}
		default:
			out.Findings = append(out.Findings, Finding{
				Severity: "warning", Code: "unknown_record_kind", HandoffID: record.HandoffID,
				Message: "unknown handoff record kind: " + record.RecordKind,
			})
		}
		out.Handoffs[record.HandoffID] = row
	}
	if err := scanner.Err(); err != nil {
		return out, err
	}
	sort.SliceStable(out.Findings, func(i, j int) bool {
		if out.Findings[i].Severity != out.Findings[j].Severity {
			return out.Findings[i].Severity < out.Findings[j].Severity
		}
		return out.Findings[i].Code < out.Findings[j].Code
	})
	return out, nil
}
