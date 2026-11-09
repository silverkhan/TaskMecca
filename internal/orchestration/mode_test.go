package orchestration

import "testing"

func TestModesHaveStableLetterLabels(t *testing.T) {
 for i, want := range []Mode{ModeA,ModeB,ModeC,ModeD} {
  if Modes[i].Code!=want || len(Modes[i].Label)<3 || Modes[i].Label[:2]!=string(want)+"." {t.Fatalf("invalid mode at %d: %+v",i,Modes[i])}
 }
}

func TestUnknownCapabilitiesFailClosed(t *testing.T) {
 for _, mode := range []Mode{ModeA,ModeB,ModeC,ModeD} {
  d,e:=Decide(Request{Mode:mode,Event:"root_initialized"})
  if e!=nil || d.Allowed || d.Action!="capability_unavailable" {t.Fatalf("%s: %+v %v",mode,d,e)}
 }
}

func TestRootNotificationRequiresRecord(t *testing.T) {
 cap:=Capabilities{SpawnSubagent:"supported", MessageRoot:"supported", WakeRoot:"unknown"}
 d,e:=Decide(Request{Mode:ModeB,Event:"user_decision_required",Capabilities:cap})
 if e!=nil||d.Allowed||d.Action!="record_user_hold_first" {t.Fatalf("%+v %v",d,e)}
 d,e=Decide(Request{Mode:ModeB,Event:"user_decision_required",Capabilities:cap,DurableRecorded:true})
 if e!=nil||!d.Allowed||d.Action!="send_to_root"||!d.NativeExecutionRequired {t.Fatalf("%+v %v",d,e)}
 cap.MessageRoot="unknown"
 d,e=Decide(Request{Mode:ModeB,Event:"user_decision_required",Capabilities:cap,DurableRecorded:true})
 if e!=nil||d.Action!="await_next_root_turn" {t.Fatalf("%+v %v",d,e)}
 d,e=Decide(Request{Mode:ModeA,Event:"user_decision_required",Capabilities:cap,DurableRecorded:true})
 if e!=nil||d.Action!="await_next_root_turn" {t.Fatalf("%+v %v",d,e)}
}

func TestIndependentSessionHandoff(t *testing.T) {
 cap:=Capabilities{CreateSession:"supported"}
 d,e:=Decide(Request{Mode:ModeC,Event:"worker_done",Capabilities:cap})
 if e!=nil||d.Action!="retain_handoff_for_recovery" {t.Fatalf("%+v %v",d,e)}
 cap.MessageSession="supported"
 d,e=Decide(Request{Mode:ModeC,Event:"worker_done",Capabilities:cap})
 if e!=nil||!d.Allowed||d.Action!="handoff_via_native_runtime" {t.Fatalf("%+v %v",d,e)}
 d,e=Decide(Request{Mode:ModeD,Event:"session_resume",Capabilities:cap})
 if e!=nil||d.Action!="recover_from_durable_state" {t.Fatalf("%+v %v",d,e)}
}

func TestInvalidModeEvent(t *testing.T) {
 if _,e:=Decide(Request{Mode:"E",Event:"root_initialized"});e==nil {t.Fatal("unknown mode accepted")}
 if _,e:=Decide(Request{Mode:ModeA,Event:"arbitrary"});e==nil {t.Fatal("unknown event accepted")}
}
