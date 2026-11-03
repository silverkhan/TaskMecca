// Package activity defines the immutable event contract used by the Activity Recorder.
// Activity events complement lifecycle and runtime evidence; they never change task state.
package activity

import (
    "errors"
    "fmt"
    "strings"
    "time"
)

type Role string
const (
    RoleRegistrar Role = "registrar"
    RoleController Role = "controller"
    RoleSubController Role = "sub_controller"
    RoleWorker Role = "worker"
)
func (r Role) Valid() bool { switch r {case RoleRegistrar, RoleController, RoleSubController, RoleWorker: return true}; return false }

type Type string
const (
    Requirements Type = "requirements"
    Research Type = "research"
    Implementation Type = "implementation"
    TargetedTest Type = "targeted_test"
    FullRegression Type = "full_regression"
    BuildCheck Type = "build_check"
    ControllerReview Type = "controller_review"
    ControllerReReview Type = "controller_rereview"
    Handoff Type = "handoff"
    Recovery Type = "recovery"
    Other Type = "other"
)
func (t Type) Valid() bool { switch t {case Requirements,Research,Implementation,TargetedTest,FullRegression,BuildCheck,ControllerReview,ControllerReReview,Handoff,Recovery,Other:return true};return false }

type Action string
const (Start Action="start"; Stop Action="stop"; Switch Action="switch"; Correct Action="correct")
func(a Action) Valid() bool {switch a {case Start,Stop,Switch,Correct:return true};return false}

type Confidence string
const (Confirmed Confidence="confirmed"; Inferred Confidence="inferred"; Unknown Confidence="unknown")
func(c Confidence) Valid() bool {switch c {case Confirmed,Inferred,Unknown:return true};return false}

// Event is versioned and append-only. RecordedAt is assigned by the trusted recorder;
// OccurredAt comes from evidence and may differ from ingestion time. Duration is derived
// from adjacent events, never supplied by an agent.
type Event struct {
    SchemaVersion int `json:"schema_version"`
    EventID string `json:"event_id"`
    TaskID string `json:"task_id"`
    Action Action `json:"action"`
    Role Role `json:"role"`
    AgentID string `json:"agent_id"`
    RuntimeAttemptID string `json:"runtime_attempt_id"`
    AssignmentID string `json:"assignment_id,omitempty"`
    Provider string `json:"provider,omitempty"`
    SessionID string `json:"session_id,omitempty"`
    ContractSHA256 string `json:"contract_sha256,omitempty"`
    TaskRevision string `json:"task_revision,omitempty"`
    Generation string `json:"generation,omitempty"`
    ParentActivityID string `json:"parent_activity_id,omitempty"`
    Source string `json:"source,omitempty"`
    RunEvidence *RunEvidence `json:"run_evidence,omitempty"`
    ReviewOutcome string `json:"review_outcome,omitempty"`
    Round int `json:"round"` // zero: first execution; positive: rework round
    Type Type `json:"type"`
    Confidence Confidence `json:"confidence"`
    Evidence string `json:"evidence,omitempty"`
    ProposedType string `json:"proposed_type,omitempty"`
    Explanation string `json:"explanation,omitempty"`
    SupersedesID string `json:"supersedes_id,omitempty"`
    OccurredAt time.Time `json:"occurred_at"`
    RecordedAt time.Time `json:"recorded_at"`
}

var ErrInvalidEvent=errors.New("invalid activity event")
func(e Event) Validate() error {
    fail:=func(reason string) error { return fmt.Errorf("%w: %s",ErrInvalidEvent,reason) }
    if e.SchemaVersion!=1{return fail("unsupported schema_version")}
    for name,value:=range map[string]string{"event_id":e.EventID,"task_id":e.TaskID,"agent_id":e.AgentID,"runtime_attempt_id":e.RuntimeAttemptID}{
        if strings.TrimSpace(value)==""{return fail("missing "+name)}
        if strings.ContainsAny(value,"\r\n\x00"){return fail("invalid "+name)}
    }
    if !e.Action.Valid(){return fail("invalid action")}
    if !e.Role.Valid(){return fail("invalid role")}
    if !e.Type.Valid(){return fail("invalid type")}
    if !e.Confidence.Valid(){return fail("invalid confidence")}
    if e.Round<0{return fail("negative round")}
    if e.OccurredAt.IsZero()||e.RecordedAt.IsZero(){return fail("missing timestamps")}
    if e.Action==Correct && strings.TrimSpace(e.SupersedesID)==""{return fail("correction missing supersedes_id")}
    if e.Action!=Correct && e.SupersedesID!=""{return fail("supersedes_id without correction")}
    if e.ReviewOutcome!="" {
        if e.Action!=Stop || (e.Type!=ControllerReview&&e.Type!=ControllerReReview) {return fail("review outcome requires review stop")}
        if e.ReviewOutcome!="accepted"&&e.ReviewOutcome!="rejected"&&e.ReviewOutcome!="needs_changes" {return fail("invalid review outcome")}
    }
    if e.Type==Other && strings.TrimSpace(e.Explanation)==""{return fail("other requires explanation")}
    if e.RunEvidence!=nil { if err:=e.RunEvidence.Validate();err!=nil{return err} }
    if e.Type!=Other && e.ProposedType!=""{return fail("proposed_type only valid for other")}
    if strings.ContainsAny(e.ProposedType,"\r\n\x00"){return fail("invalid proposed_type")}
    if e.Confidence==Inferred && strings.TrimSpace(e.Evidence)==""{return fail("inferred activity requires evidence")}
    return nil
}
