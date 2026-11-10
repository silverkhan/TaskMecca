package backlog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryEventSnapshotFindsOlderOrphanOnUnrelatedRegistration(t *testing.T) {
	project := t.TempDir()
	dir := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(dir, 0755); err != nil { t.Fatal(err) }
	// A present runtime registry makes missing assigned workers observable.
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca", ".runtime", "agents"), 0755); err != nil { t.Fatal(err) }
	tasks := map[string]string{
		"000527.B-527.orphan.doing.md": "# B-527 Interrupted\n- Agent: /root/controller/kkobugi\n- 변경범위: src\n",
		"000550.B-550.new.todo.md": "# B-550 New task\n- Agent: -\n- 변경범위: -\n",
		"000526.B-526.waiting.hold.md": "# B-526 Deliberate hold\n- Agent: -\n- 변경범위: -\n",
		"000525.B-525.done.done.md": "# B-525 Completed\n- Agent: -\n- 변경범위: -\n",
	}
	for name, body := range tasks {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil { t.Fatal(err) }
	}
	one, err := RecoveryEventSnapshot(project, "", "registration", "B-550")
	if err != nil { t.Fatal(err) }
	if one["status"] != "candidates_found" || one["candidate_count"] != 1 || one["report_to_root"] != true {
		t.Fatalf("registration event missed unrelated orphan: %+v", one)
	}
	items := one["candidates"].([]map[string]any)
	if items[0]["id"] != "B-527" || items[0]["code"] != "worker_missing_backlog_doing" {
		t.Fatalf("wrong recovery candidate: %+v", items)
	}
	if items[0]["requires_native_verification"] != true {
		t.Fatalf("unsafe auto-recovery claim: %+v", items[0])
	}
	two, err := RecoveryEventSnapshot(project, "", "registration", "B-550")
	if err != nil { t.Fatal(err) }
	if one["event_fingerprint"] != two["event_fingerprint"] || items[0]["incident_key"] != two["candidates"].([]map[string]any)[0]["incident_key"] {
		t.Fatal("repeated scans changed deduplication identity")
	}
	saved, err := os.ReadFile(filepath.Join(dir, "000527.B-527.orphan.doing.md"))
	if err != nil { t.Fatal(err) }
	if string(saved) != tasks["000527.B-527.orphan.doing.md"] { t.Fatal("read-only scan modified canonical work") }

	again, err := RecoveryEventSnapshot(project, "", "controller_resume", "")
	if err != nil { t.Fatal(err) }
	if again["candidate_count"] != 1 { t.Fatalf("controller resume ignored old orphan: %+v", again) }
	worker, err := RecoveryEventSnapshot(project, "", "worker_report", "B-527")
	if err != nil { t.Fatal(err) }
	if worker["candidate_count"] != 1 { t.Fatalf("worker event omitted its own stranded task: %+v", worker) }
}

func TestRecoveryEventSnapshotDoesNotTreatNewTaskAsPreviousOrphan(t *testing.T) {
	project := t.TempDir()
	dir := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(dir, 0755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(dir, "000550.B-550.new.doing.md"), []byte("# B-550 New\n- Agent: /root/controller/pairi\n- 변경범위: src\n"), 0644); err != nil { t.Fatal(err) }
	report, err := RecoveryEventSnapshot(project, "", "registration", "B-550")
	if err != nil { t.Fatal(err) }
	if report["candidate_count"] != 0 || report["report_to_root"] != false { t.Fatalf("new task reported as old orphan: %+v", report) }
}

func TestRecoveryEventSnapshotRejectsUnknownOrUnscopedEvents(t *testing.T) {
	for _, tc := range []struct{event,task string}{
		{"timer_tick","B-550"}, {"registration",""}, {"worker_report",""},
	} {
		if _, err := RecoveryEventSnapshot(t.TempDir(), "", tc.event, tc.task); err == nil {
			t.Fatalf("event %q task %q should be rejected", tc.event, tc.task)
		}
	}
}

func TestRecoveryEventSurfaceUserHoldsWithoutNativeDispatch(t *testing.T) {
 project:=t.TempDir()
 dir:=filepath.Join(project,"_task_mecca","data","backlog")
 if err:=os.MkdirAll(dir,0755);err!=nil {t.Fatal(err)}
 fixtures:=map[string]string{
  "000620.B-620.user-choice.hold.md":"# User choice\n- 대기유형: user\n- 대기: Choose policy\n- 재개조건: User confirms\n",
  "000621.B-621.provider-wait.hold.md":"# Provider wait\n- 대기유형: external\n- 대기: Provider reply\n- 재개조건: Provider replies\n",
  "000622.B-622.pending-detail.hold.md":"# User reason incomplete\n- 대기유형: user\n- 대기: -\n- 재개조건: Need user input\n",
 }
 for name,body:=range fixtures {
  if err:=os.WriteFile(filepath.Join(dir,name),[]byte(body),0644);err!=nil{t.Fatal(err)}
 }
 got,err:=RecoveryEventSnapshot(project,"","controller_resume","")
 if err!=nil{t.Fatal(err)}
 if got["pending_user_hold_count"]!=2 || got["status"]!="user_holds_found" {
  t.Fatalf("user holds not discovered: %+v",got)
 }
 items,ok:=got["pending_user_holds"].([]map[string]any)
 if !ok||len(items)!=2 {t.Fatalf("unexpected hold projection: %+v",got["pending_user_holds"])}
 if items[0]["id"]!="B-620"||items[0]["next_action"]!="flow_next:user_decision_required"||items[0]["evidence_complete"]!=true {
  t.Fatalf("complete user hold missing mandatory decision: %+v",items[0])
 }
 if items[1]["id"]!="B-622"||items[1]["next_action"]!="complete_canonical_user_hold_first"||items[1]["evidence_complete"]!=false {
  t.Fatalf("incomplete user hold not flagged for repair: %+v",items[1])
 }
 repeated,err:=RecoveryEventSnapshot(project,"","controller_resume","")
 if err!=nil{t.Fatal(err)}
 if got["event_fingerprint"]!=repeated["event_fingerprint"] {t.Fatal("same user holds produced unstable event identity")}
 for name,body:=range fixtures {
  data,err:=os.ReadFile(filepath.Join(dir,name))
  if err!=nil||string(data)!=body {t.Fatalf("read-only recovery modified %s: %v",name,err)}
 }
}
