package handoff

import "github.com/silverkhan/TaskMecca/internal/handoffjournal"

type EventType = handoffjournal.EventType
type Action = handoffjournal.Action
type Target = handoffjournal.Target
type Record = handoffjournal.Record
type StepState = handoffjournal.StepState
type Handoff = handoffjournal.Handoff
type Finding = handoffjournal.Finding
type Ledger = handoffjournal.Ledger

const EventRegistrationReady = handoffjournal.EventRegistrationReady
const EventWorkerDone = handoffjournal.EventWorkerDone
const EventWorkerBlocked = handoffjournal.EventWorkerBlocked
const EventUserDecisionRequired = handoffjournal.EventUserDecisionRequired
const EventCompletionReport = handoffjournal.EventCompletionReport
const ActionMessageRunning = handoffjournal.ActionMessageRunning
const ActionResumeCompleted = handoffjournal.ActionResumeCompleted
const ActionReportOnly = handoffjournal.ActionReportOnly
const ActionHold = handoffjournal.ActionHold

type PrepareRequest struct {
	Project                string
	TaskID                 string
	EventType              EventType
	SourceAgentPath        string
	TargetAgentPath        string
	SourceAttemptID        string
	TargetAttemptID        string
	ReportFile             string
	ExpectedContractSHA256 string
	ExecutionAuthorized    bool
}

type PrepareResult struct {
	HandoffID              string `json:"handoff_id"`
	Duplicate              bool   `json:"duplicate"`
	ContractSHA256         string `json:"contract_sha256"`
	Target                 Target `json:"target"`
	Action                 Action `json:"action"`
	RequiresFreshPreflight bool   `json:"requires_fresh_preflight"`
	Reason                 string `json:"reason,omitempty"`
}

type ClaimResult struct {
	HandoffID             string `json:"handoff_id"`
	Claimed               bool   `json:"claimed"`
	AlreadyClaimed        bool   `json:"already_claimed"`
	AlreadyApplied        bool   `json:"already_applied"`
	ClaimConflict         bool   `json:"claim_conflict"`
	ContractChanged       bool   `json:"contract_changed"`
	CurrentContractSHA256 string `json:"current_contract_sha256,omitempty"`
	Reason                string `json:"reason,omitempty"`
	ClaimedBy             string `json:"claimed_by,omitempty"`
	ClaimedAttemptID      string `json:"claimed_attempt_id,omitempty"`
}

type MarkResult struct {
	HandoffID string `json:"handoff_id"`
	Step      string `json:"step"`
	Result    string `json:"result"`
	Duplicate bool   `json:"duplicate"`
	Applied   bool   `json:"applied"`
}
