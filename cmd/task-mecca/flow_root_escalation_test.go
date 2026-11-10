package main

import (
 "testing"
)

func TestFlowNextRootEscalationFailsClosedWithoutRootIdentity(t *testing.T) {
 result:=evaluateRootEscalationDecision(t.TempDir(),"B-620","","")
 if result.ExitAllowed || result.Decision!="EXIT_BLOCKED" || len(result.Reasons)==0 {
  t.Fatalf("unsafe escalation decision: %+v",result)
 }
 result=evaluateRootEscalationDecision(t.TempDir(),"B-620","codex","unconfigured")
 if result.ExitAllowed || result.Decision!="EXIT_BLOCKED" {
  t.Fatalf("unconfigured Root accepted: %+v",result)
 }
}
