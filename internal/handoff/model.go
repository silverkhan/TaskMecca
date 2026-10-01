package handoff

type EventType string
type Action string

const (
	EventRegistrationReady    EventType = "registration_ready"
	EventWorkerDone           EventType = "worker_done"
	EventWorkerBlocked        EventType = "worker_blocked"
	EventUserDecisionRequired EventType = "user_decision_required"
	EventCompletionReport     EventType = "completion_report"

	ActionMessageRunning  Action = "message_running"
	ActionResumeCompleted Action = "resume_completed"
	ActionReportOnly      Action = "report_only"
	ActionHold            Action = "hold"
)

type Target struct {
	AgentPath      string `json:"agent_path"`
	AttemptID      string `json:"attempt_id,omitempty"`
	RuntimeAgentID string `json:"runtime_agent_id,omitempty"`
	RuntimeName    string `json:"runtime_name,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	Provider       string `json:"provider,omitempty"`
	State          string `json:"state,omitempty"`
}

type Record struct {
	HandoffID              string            `json:"handoff_id"`
	RecordKind             string            `json:"record_kind"`
	EventType              EventType         `json:"event_type,omitempty"`
	TaskID                 string            `json:"task_id,omitempty"`
	ContractSHA256         string            `json:"contract_sha256,omitempty"`
	SourceAgentPath        string            `json:"source_agent_path,omitempty"`
	SourceAttemptID        string            `json:"source_attempt_id,omitempty"`
	TargetAgentPath        string            `json:"target_agent_path,omitempty"`
	TargetAttemptID        string            `json:"target_attempt_id,omitempty"`
	TargetRuntimeAgentID   string            `json:"target_runtime_agent_id,omitempty"`
	TargetRuntimeName      string            `json:"target_runtime_name,omitempty"`
	TargetSessionID        string            `json:"target_session_id,omitempty"`
	Provider               string            `json:"provider,omitempty"`
	TargetState            string            `json:"target_state,omitempty"`
	Action                 Action            `json:"action,omitempty"`
	RequiresFreshPreflight bool              `json:"requires_fresh_preflight,omitempty"`
	ClaimedBy              string            `json:"claimed_by,omitempty"`
	ClaimedAttemptID       string            `json:"claimed_attempt_id,omitempty"`
	Step                   string            `json:"step,omitempty"`
	Result                 string            `json:"result,omitempty"`
	ObservedAt             string            `json:"observed_at"`
	Evidence               map[string]string `json:"evidence,omitempty"`
}

type StepState struct {
	Result     string `json:"result"`
	Evidence   string `json:"evidence,omitempty"`
	ObservedAt string `json:"observed_at,omitempty"`
}

type Handoff struct {
	HandoffID              string               `json:"handoff_id"`
	EventType              EventType            `json:"event_type"`
	TaskID                 string               `json:"task_id"`
	ContractSHA256         string               `json:"contract_sha256,omitempty"`
	SourceAgentPath        string               `json:"source_agent_path,omitempty"`
	SourceAttemptID        string               `json:"source_attempt_id,omitempty"`
	Target                 Target               `json:"target"`
	Action                 Action               `json:"action"`
	RequiresFreshPreflight bool                 `json:"requires_fresh_preflight"`
	PreparedAt             string               `json:"prepared_at,omitempty"`
	ClaimedBy              string               `json:"claimed_by,omitempty"`
	ClaimedAttemptID       string               `json:"claimed_attempt_id,omitempty"`
	ClaimedAt              string               `json:"claimed_at,omitempty"`
	Steps                  map[string]StepState `json:"steps"`
	Applied                bool                 `json:"applied"`
	AppliedAt              string               `json:"applied_at,omitempty"`
	RecordCount            int                  `json:"record_count"`
}

type Finding struct {
	Severity  string `json:"severity"`
	Code      string `json:"code"`
	HandoffID string `json:"handoff_id,omitempty"`
	Message   string `json:"message"`
}

type Ledger struct {
	Version     int                `json:"version"`
	GeneratedAt string             `json:"generated_at"`
	JournalPath string             `json:"journal_path"`
	Handoffs    map[string]Handoff `json:"handoffs"`
	Findings    []Finding          `json:"findings"`
}

type PrepareRequest struct {
	Project             string
	TaskID              string
	EventType           EventType
	SourceAgentPath     string
	TargetAgentPath     string
	SourceAttemptID     string
	TargetAttemptID     string
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
	HandoffID      string `json:"handoff_id"`
	Claimed        bool   `json:"claimed"`
	AlreadyClaimed bool   `json:"already_claimed"`
	AlreadyApplied bool   `json:"already_applied"`
	ClaimConflict          bool   `json:"claim_conflict"`
	ContractChanged        bool   `json:"contract_changed"`
	CurrentContractSHA256  string `json:"current_contract_sha256,omitempty"`
	Reason                 string `json:"reason,omitempty"`
	ClaimedBy              string `json:"claimed_by,omitempty"`
	ClaimedAttemptID       string `json:"claimed_attempt_id,omitempty"`
}

type MarkResult struct {
	HandoffID string `json:"handoff_id"`
	Step      string `json:"step"`
	Result    string `json:"result"`
	Duplicate bool   `json:"duplicate"`
	Applied   bool   `json:"applied"`
}
