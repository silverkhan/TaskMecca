// Package subcontroller defines fail-closed, side-effect-free delegation decisions.
// It does not dispatch agents or write the canonical backlog.
package subcontroller

import (
 "errors"
 "strings"
)

type Snapshot struct {
 BacklogID string `json:"backlog_id"`
 ActorRole string `json:"actor_role"`
 Event string `json:"event"`
 Authorized bool `json:"execution_authorized"`
 EvidenceReady bool `json:"evidence_ready"`
 IndependentReviewRequired bool `json:"independent_review_required"`
 IndependentReviewReady bool `json:"independent_review_ready"`
 ActiveSubcontrollers int `json:"active_subcontrollers"`
 MaxSubcontrollers int `json:"max_subcontrollers"`
 PendingReview int `json:"pending_review"`
 PendingDispatch int `json:"pending_dispatch"`
 ContractChanged bool `json:"contract_changed"`
 ScopeConflict bool `json:"scope_conflict"`
 RuntimeKnown bool `json:"runtime_known"`
}

type Decision struct {
 Action string `json:"action"`
 Owner string `json:"owner"`
 Reason string `json:"reason"`
 CanonicalWriteAllowed bool `json:"canonical_write_allowed"`
 NeedsMainApproval bool `json:"needs_main_approval"`
}

func Decide(s Snapshot) (Decision,error) {
 if strings.TrimSpace(s.BacklogID)=="" {return Decision{},errors.New("backlog_id required")}
 if s.ActorRole!="main_controller" && s.ActorRole!="sub_controller" {return Decision{},errors.New("unsupported actor_role")}
 d:=Decision{Owner:"main_controller",CanonicalWriteAllowed:false,NeedsMainApproval:true}
 if s.ContractChanged||s.ScopeConflict {
  d.Action="escalate";d.Reason="contract or scope conflict";return d,nil
 }
 if !s.Authorized {
  d.Action="defer";d.Reason="execution approval missing";return d,nil
 }
 if !s.RuntimeKnown {
  d.Action="escalate";d.Reason="runtime identity unverified";return d,nil
 }
 if s.ActorRole=="sub_controller" {
  switch s.Event {
  case "review_completed":
   if !s.EvidenceReady || (s.IndependentReviewRequired&&!s.IndependentReviewReady) {
    d.Action="request_evidence";d.Owner="sub_controller";d.Reason="acceptance evidence incomplete";return d,nil
   }
   d.Action="recommend_acceptance";d.Reason="all declared review gates met; main decides acceptance";return d,nil
  case "review_failed","worker_blocked":
   d.Action="escalate";d.Reason="review exception requires main decision";return d,nil
  default:
   d.Action="continue_delegation";d.Owner="sub_controller";d.NeedsMainApproval=false;d.Reason="within delegated read-only review";return d,nil
  }
 }
 switch s.Event {
 case "review_completed":
  if !s.EvidenceReady || (s.IndependentReviewRequired&&!s.IndependentReviewReady) {
   d.Action="request_evidence";d.Reason="review gates not met";return d,nil
  }
  d.Action="decide_acceptance";d.Reason="main must verify canonical state before finalization";return d,nil
 case "worker_completed","registration_ready":
  pending:=s.PendingReview
  if s.Event=="registration_ready" {pending=s.PendingDispatch}
  if pending<=1 {d.Action="handle_directly";d.Reason="no demonstrated delegation pressure";return d,nil}
  if s.MaxSubcontrollers<=0 {d.Action="handle_directly";d.Reason="delegation pool disabled";return d,nil}
  if s.ActiveSubcontrollers>=s.MaxSubcontrollers {d.Action="reuse_or_queue";d.Reason="pool at capacity";return d,nil}
  d.Action="propose_delegation";d.Reason="multiple pending tasks; main approves scoped lease";return d,nil
 default:
  d.Action="handle_directly";d.Reason="no delegation trigger";return d,nil
 }
}
