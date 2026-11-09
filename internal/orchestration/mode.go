// Package orchestration evaluates the user-selected execution mode against
// observed runtime capabilities. It never spawns sessions or sends messages.
package orchestration

import (
 "fmt"
 "strings"
)

type Mode string

const (
 ModeA Mode = "A" // subagents, durable Root fallback
 ModeB Mode = "B" // subagents with Root communication
 ModeC Mode = "C" // independent sessions with Root communication
 ModeD Mode = "D" // independent sessions, durable Root fallback
)

type Capabilities struct {
 SpawnSubagent string `json:"spawn_subagent"`
 CreateSession string `json:"create_session"`
 MessageSession string `json:"message_session"`
 MessageRoot string `json:"message_root"`
 ResumeSession string `json:"resume_session"`
 WakeRoot string `json:"wake_root"`
}

type ModeInfo struct {
 Code Mode `json:"code"`
 Label string `json:"label"`
 Execution string `json:"execution"`
 RootCommunication bool `json:"root_communication"`
}

var Modes = []ModeInfo{
 {ModeA, "A. 기본 서브에이전트", "subagent", false},
 {ModeB, "B. 서브에이전트 + Root 통신", "subagent", true},
 {ModeC, "C. 독립 세션 + Root 통신", "session", true},
 {ModeD, "D. 독립 세션 (Root 통신 없음)", "session", false},
}

type Request struct {
 Mode Mode `json:"mode"`
 Event string `json:"event"`
 TaskID string `json:"task_id,omitempty"`
 Capabilities Capabilities `json:"capabilities"`
 DurableRecorded bool `json:"durable_recorded,omitempty"`
}

type Decision struct {
 SchemaVersion int `json:"schema_version"`
 Mode Mode `json:"mode"`
 Event string `json:"event"`
 Allowed bool `json:"allowed"`
 Action string `json:"action"`
 Reasons []string `json:"reasons"`
 RequiresDurableRecord bool `json:"requires_durable_record"`
 NativeExecutionRequired bool `json:"native_execution_required"`
}

func ModeByCode(code Mode) (ModeInfo, bool) {
 for _, m := range Modes { if m.Code == code { return m, true } }
 return ModeInfo{}, false
}

func supported(v string) bool { return strings.EqualFold(strings.TrimSpace(v), "supported") }

// Decide is side-effect free. Unknown capability is NOT evidence of support.
// A missing durable record blocks an outbound user-decision notification.
func Decide(req Request) (Decision, error) {
 mode, ok := ModeByCode(req.Mode)
 if !ok { return Decision{}, fmt.Errorf("unknown orchestration mode %q: choose A, B, C, D", req.Mode) }
 valid := map[string]bool{"root_initialized":true,"user_decision_required":true,"registration_completed":true,"worker_done":true,"session_resume":true}
 if !valid[req.Event] { return Decision{}, fmt.Errorf("unsupported orchestration event %q", req.Event) }
 d := Decision{SchemaVersion:1,Mode:req.Mode,Event:req.Event,Reasons:[]string{}}
 if mode.Execution=="subagent" && !supported(req.Capabilities.SpawnSubagent) {
  d.Action="capability_unavailable";d.Reasons=append(d.Reasons,"spawn_subagent_not_verified");return d,nil
 }
 if mode.Execution=="session" && !supported(req.Capabilities.CreateSession) {
  d.Action="capability_unavailable";d.Reasons=append(d.Reasons,"create_session_not_verified");return d,nil
 }
 switch req.Event {
 case "root_initialized":
  d.Allowed=true; d.Action="mode_ready"
 case "user_decision_required":
  d.RequiresDurableRecord=true
  if !req.DurableRecorded {
   d.Action="record_user_hold_first";d.Reasons=append(d.Reasons,"durable_user_hold_required");return d,nil
  }
  if !mode.RootCommunication {
   d.Allowed=true;d.Action="await_next_root_turn";return d,nil
  }
  if !supported(req.Capabilities.MessageRoot) {
   d.Allowed=true;d.Action="await_next_root_turn";d.Reasons=append(d.Reasons,"root_message_not_verified");return d,nil
  }
  d.Allowed=true;d.NativeExecutionRequired=true;d.Action="send_to_root"
  if !supported(req.Capabilities.WakeRoot) {d.Reasons=append(d.Reasons,"root_wake_not_verified_delivery_is_not_acknowledgement")}
 case "registration_completed","worker_done":
  if mode.Execution=="session" && !supported(req.Capabilities.MessageSession) {
   d.Action="retain_handoff_for_recovery";d.Reasons=append(d.Reasons,"session_message_not_verified");return d,nil
  }
  d.Allowed=true;d.NativeExecutionRequired=true;d.Action="handoff_via_native_runtime"
 case "session_resume":
  if mode.Execution=="session" && !supported(req.Capabilities.ResumeSession) {
   d.Action="recover_from_durable_state";d.Reasons=append(d.Reasons,"session_resume_not_verified");return d,nil
  }
  d.Allowed=true;d.Action="reconcile_durable_state_before_dispatch"
 }
 return d,nil
}
