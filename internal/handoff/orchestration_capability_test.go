package handoff

import (
 "testing"
 "time"
)

func TestOrchestrationCapabilitiesRequireEvidence(t *testing.T) {
 project:=t.TempDir()
 baseline,err:=LoadCapabilities(project,"codex")
 if err!=nil{t.Fatal(err)}
 if baseline.CanMessageRoot!=CapabilityUnknown||baseline.CanCreateSession!=CapabilityUnknown {
  t.Fatalf("new runtime capabilities must be unknown by default: %+v",baseline)
 }
 if _,err:=RecordCapability(project,"codex","can_message_root",CapabilitySupported,"",time.Now());err==nil {
  t.Fatal("declared Root messaging without actual evidence")
 }
 recorded,err:=RecordCapability(project,"codex","can_message_root",CapabilitySupported,"observed native target message smoke",time.Now())
 if err!=nil{t.Fatal(err)}
 if recorded.CanMessageRoot!=CapabilitySupported {t.Fatal(recorded.CanMessageRoot)}
 restored,err:=LoadCapabilities(project,"codex")
 if err!=nil||restored.CanMessageRoot!=CapabilitySupported {t.Fatalf("%+v %v",restored,err)}
 if restored.CanCreateSession!=CapabilityUnknown{t.Fatal("unsupported create inferred from Root message")}
}
