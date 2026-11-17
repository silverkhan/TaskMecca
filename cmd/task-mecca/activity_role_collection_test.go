package main

import (
	"bytes"
	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"strings"
	"testing"
	"time"
)

func TestA85RegistrarReportsCurrentPhasesAndRetriesOffline(t *testing.T) {
	p := t.TempDir()
	now := time.Now().UTC()
	if err := runtimeobs.AppendExecutionEvent(p, runtimeobs.ExecutionEvent{EventID: "native", EventKind: "state", AttemptID: "registrar", Provider: "codex", SessionID: "native-r", RuntimeAgentID: "native-r", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved, ObservedAt: now.Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.BindAttempt(p, "registrar", "A-85", "/root/registrar", "explicit", "", nil, now); err != nil {
		t.Fatal(err)
	}
	args := func(phase, id string) []string {
		return []string{"report", "A-85", "--project", p, "--role", "registrar", "--agent", "native-r", "--attempt", "registrar", "--event", phase, "--event-id", id, "--evidence", "synthetic current registration fixture", "--json"}
	}
	for i, phase := range []string{"requirements_started", "handoff_started", "registration_completed"} {
		id := []string{"analysis", "handoff", "done"}[i]
		code, response, err := invokeActivity(t, args(phase, id)...)
		if code != 0 || response["recorded"] != true {
			t.Fatalf("phase %s: %d %+v %s", phase, code, response, err)
		}
		code, response, err = invokeActivity(t, args(phase, id)...)
		if code != 0 || response["recorded"] != false {
			t.Fatalf("retry %s: %d %+v %s", phase, code, response, err)
		}
	}
	rows, err := (activity.Ledger{Project: p}).List("A-85")
	if err != nil || len(rows) != 3 {
		t.Fatalf("unexpected ledger %v %v", rows, err)
	}
	for _, e := range rows {
		if e.Role != activity.RoleRegistrar || e.AssignmentID != "" || e.HandoffID != "" {
			t.Fatal("role crossed authorization boundary", e)
		}
	}
	if got := activity.Summarize("A-85", rows); got.OpenActivities != 0 || got.InvalidTransitions != 0 {
		t.Fatal(got)
	}
}

func TestA85FormalRegistrarCLIHonestDryRunAndClaimScope(t *testing.T) {
	base := []string{"--project", t.TempDir(), "--thread-id", "managed", "--session-id", "native", "--agent-id", "native", "--assignment", "registration-assignment", "--task-id", "A-85", "--role", "registrar", "--agent-path", "/root/registrar", "--parent-thread-id", "actual-parent", "--source-worktree", t.TempDir(), "--phase", "handoff_started", "--dry-run", "--json"}
	var out, errOut bytes.Buffer
	code := runRuntimeObserveT3CLI(base, &out, &errOut)
	if code != 2 || !strings.Contains(errOut.String(), "dry-run is unsupported") {
		t.Fatalf("Registrar rejected as Worker or claimed dry collection: %d %s %s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	code = runRuntimeObserveT3CLI(append(base, "--handoff", "foreign-controller-claim"), &out, &errOut)
	if code != 2 || !strings.Contains(errOut.String(), "only an existing formal Controller claim") {
		t.Fatalf("Registrar consumed Controller claim %d %s", code, errOut.String())
	}
}

func TestA85RegistrarBindingAloneCannotRecordOrSuggestActivity(t *testing.T) {
	p := t.TempDir()
	now := time.Now().UTC()
	if err := runtimeobs.AppendExecutionEvent(p, runtimeobs.ExecutionEvent{EventID: "unverified-state", EventKind: "state", AttemptID: "binding-only", Provider: "codex", SessionID: "native", RuntimeAgentID: "native", State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative, ObservedAt: now.Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	a, err := runtimeobs.BindAttempt(p, "binding-only", "A-85", "/root/registrar", "explicit", "", nil, now)
	if err != nil {
		t.Fatal(err)
	}
	requirement := activity.CheckReporting(p, "A-85", activity.RoleRegistrar, a.AttemptID, "", "", "requirements_started", now)
	if requirement.Status != "unverified" || len(requirement.Arguments) > 0 {
		t.Fatal("binding promoted to phase evidence", requirement)
	}
	code, response, message := invokeActivity(t, "report", "A-85", "--project", p, "--role", "registrar", "--agent", a.RuntimeAgentID, "--attempt", a.AttemptID, "--event", "requirements_started", "--event-id", "unobserved", "--evidence", "fixture missing real execution", "--json")
	if code == 0 {
		t.Fatalf("unobserved Registrar recorded %+v %s", response, message)
	}
	rows, err := (activity.Ledger{Project: p}).List("A-85")
	if err != nil || len(rows) != 0 {
		t.Fatalf("failure wrote Activity %+v %v", rows, err)
	}
}
