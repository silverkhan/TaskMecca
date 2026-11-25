package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestObserveT3CLIRejectsUserStateAndNeverWritesOnFailedDryRun(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, args := range [][]string{{"--project", project, "--json"}, {"--project", project, "--thread-id", "t", "--session-id", "native", "--agent-id", "native", "--state", "running"}, {"--project", project, "--thread-id", "t", "--session-id", "native", "--agent-id", "native", "--dry-run", "--json"}} {
		var out, stderr bytes.Buffer
		if code := runRuntimeObserveT3CLI(args, &out, &stderr); code == 0 {
			t.Fatal("unverified CLI input succeeded", out.String())
		}
	}
	if _, err := os.Stat(filepath.Join(project, "_task_mecca")); !os.IsNotExist(err) {
		t.Fatal("failed/dry verification wrote project ledger")
	}
}

// Exercise the CLI's real START and Activity write boundaries against an owned
// offline host projection, without substituting a role in the generated hint.
func TestObserveT3CLIFormalRoleReportingHints(t *testing.T) {
	for _, role := range []string{"reviewer", "worker", "registrar", "controller"} {
		t.Run(role, func(t *testing.T) {
			project := makeDecisionFixture(t)
			// macOS's /var alias is valid for temp allocation, but positive
			// fixtures must supply the exact canonical project/source path.
			var err error
			project, err = filepath.EvalSymlinks(project)
			if err != nil {
				t.Fatal(err)
			}
			if raw, err := exec.Command("git", "init", "-q", project).CombinedOutput(); err != nil {
				t.Fatal(string(raw), err)
			}
			if err := os.WriteFile(filepath.Join(project, "source.go"), []byte("package candidate\n"), 0600); err != nil {
				t.Fatal(err)
			}
			path := map[string]string{"reviewer": "/root/controller/reviewer/raichyu", "worker": "/root/controller/pairi", "registrar": "/root/registrar", "controller": "/root/controller"}[role]
			policy := rolepolicy.Policy{SchemaVersion: 1, Method: "worker", Workspace: project, Scope: []string{"source.go"}, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: "/root/controller/pairi", NativeID: "implementation-native"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller", NativeID: "controller-native"}, Reviewer: rolepolicy.Owner{Role: "reviewer", AgentPath: "/root/controller/reviewer/raichyu", NativeID: "review-native"}}
			if role == "reviewer" {
				policy.Reviewer.NativeID = "fixture-native"
				policy.ReviewRequirement = &rolepolicy.ReviewRequirement{Scope: []string{"source.go"}, Rationale: "independent readonly source review required", Risk: rolepolicy.ReviewRisk{Impact: "high", Complexity: "complex", Reversibility: "easy", Verifiability: "limited", BehaviorChanged: true, ExplicitRequired: true, TrustBoundary: true}}
			}
			if role == "worker" {
				policy.Implementation.NativeID = "fixture-native"
			}
			if role == "controller" {
				policy.Operational.NativeID = "fixture-native"
			}
			raw, err := json.Marshal(policy)
			if err != nil {
				t.Fatal(err)
			}
			body := filepath.Join(t.TempDir(), "policy.md")
			if err := os.WriteFile(body, []byte(rolepolicy.Fence+string(raw)+"\n```"), 0600); err != nil {
				t.Fatal(err)
			}
			if code, _, msg := decisionInvoke(t, append(decisionRecordArgs(project, "policy", "supplement"), "--body-file", body, "--user-confirmed")...); code != 0 {
				t.Fatal(code, msg)
			}
			if code, _, msg := decisionInvoke(t, append(decisionRecordArgs(project, "approved", "execution_approved"), "--user-confirmed")...); code != 0 {
				t.Fatal(code, msg)
			}
			host := newRegistrationHostFixture(t, project)
			host.sql(`ALTER TABLE orchestration_v2_projection_threads ADD COLUMN payload_json TEXT;
			UPDATE orchestration_v2_projection_threads SET payload_json=json_object('lineage',json_object('parentThreadId','fixture-parent','relationshipToParent','subagent'),'worktreePath',` + fixtureSQLLiteral(project) + `);
			INSERT INTO orchestration_v2_projection_threads VALUES('fixture-parent',NULL,NULL,'fixture-project','{}');`)
			if _, err := runtimeobs.RecordAssignment(project, "exact-assignment", "B-1", path, time.Now()); err != nil {
				t.Fatal(err)
			}
			if role == "reviewer" {
				// The readonly policy requires a separate observed implementer.
				now := time.Now().UTC().Format(time.RFC3339Nano)
				for _, event := range []runtimeobs.ExecutionEvent{
					{EventKind: "state", ObservedAt: now, AttemptID: "implementation-run", Provider: "codex", SessionID: policy.Implementation.NativeID, RuntimeAgentID: policy.Implementation.NativeID, State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved},
					{EventKind: "binding", ObservedAt: now, AttemptID: "implementation-run", TaskID: "B-1", AgentPath: policy.Implementation.AgentPath, BindingSource: "dispatch", BindingEvidence: map[string]string{"assignment_id": "implementation-assignment"}, EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative},
				} {
					if err := runtimeobs.AppendExecutionEvent(project, event); err != nil {
						t.Fatal(err)
					}
				}
			}
			args := []string{"--project", project, "--thread-id", "fixture-thread", "--session-id", "fixture-native", "--agent-id", "fixture-native", "--task-id", "B-1", "--assignment", "exact-assignment", "--source-worktree", project, "--phase", "requirements_started", "--json"}
			if role == "reviewer" {
				// The supported Reviewer START uses the exact ReviewerPath in
				// the assignment envelope, as in the independent F1 reproduction.
				args = append(args, "--worker-path", path)
			} else {
				args = append(args, "--role", role, "--agent-path", path, "--parent-thread-id", "fixture-parent")
			}
			var out, diagnostic bytes.Buffer
			if code := runRuntimeObserveT3CLI(args, &out, &diagnostic); code != 0 {
				t.Fatalf("START exit%d: %s %s", code, out.String(), diagnostic.String())
			}
			var result struct {
				Verified        bool                          `json:"verified"`
				Bound           bool                          `json:"binding_verified"`
				ActivityWritten bool                          `json:"activity_written"`
				Reporting       activity.ReportingRequirement `json:"activity_reporting"`
				Next            string                        `json:"next_action"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if !result.Verified || !result.Bound || result.ActivityWritten {
				t.Fatalf("invalid START: %s", out.String())
			}
			if role == "controller" {
				if result.Reporting.Status != "unverified" || len(result.Reporting.Arguments) != 0 || !strings.Contains(result.Next, "SAME existing handoff") {
					t.Fatalf("Controller hint changed: %s", out.String())
				}
				return
			}
			wantAssignment := "exact-assignment"
			if role == "registrar" {
				wantAssignment = ""
			}
			if result.Reporting.Role != activity.Role(role) || result.Reporting.AssignmentID != wantAssignment || result.Reporting.Status != "not_recorded" || !result.Reporting.ActionRequired || strings.Contains(result.Next, "Worker") {
				t.Fatalf("wrong role hint: %s", out.String())
			}
			if role != "reviewer" {
				if role == "worker" {
					wrong := activity.CheckReporting(project, "B-1", activity.RoleReviewer, result.Reporting.AttemptID, "exact-assignment", "", "requirements_started", time.Now())
					if wrong.Status != "unverified" || len(wrong.Arguments) != 0 {
						t.Fatal("Worker received Reviewer advice", wrong)
					}
				}
				return
			}
			for _, probe := range []struct {
				role                activity.Role
				assignment, handoff string
			}{
				{activity.RoleWorker, "exact-assignment", ""},
				{activity.RoleReviewer, "foreign-assignment", ""},
				{activity.RoleReviewer, "", ""},
				{activity.RoleReviewer, "exact-assignment", "foreign-handoff"},
			} {
				wrong := activity.CheckReporting(project, "B-1", probe.role, result.Reporting.AttemptID, probe.assignment, probe.handoff, "requirements_started", time.Now())
				if wrong.Status != "unverified" || len(wrong.Arguments) != 0 {
					t.Fatal("invalid Reviewer advice", wrong)
				}
			}
			// Only materialize the documented placeholders; preserve the role,
			// task, exact assignment and current attempt emitted by START.
			report := append([]string{}, result.Reporting.Arguments[1:]...)
			for i, arg := range report {
				if arg == "<stable-current-event-id>" {
					report[i] = "reviewer-current-phase"
				}
				if arg == "<actual-current-phase-evidence>" {
					report[i] = "owned formal START regression"
				}
			}
			report = append(report, "--explanation", "검토자의 실제 현재 요건 확인 단계를 기록합니다.", "--language", "ko", "--fallback-language", "ko", "--json")
			for _, badRole := range []string{"worker", "registrar", "controller"} {
				bad := append([]string{}, report...)
				for i := range bad {
					if bad[i] == "--role" {
						bad[i+1] = badRole
						break
					}
				}
				out.Reset()
				diagnostic.Reset()
				if code := runActivityCLI(bad, &out, &diagnostic); code == 0 {
					t.Fatalf("Reviewer accepted as %s: %s", badRole, out.String())
				}
			}
			wrongAssignment := append([]string{}, report...)
			for i := range wrongAssignment {
				if wrongAssignment[i] == "--assignment" {
					wrongAssignment[i+1] = "foreign-assignment"
					break
				}
			}
			out.Reset()
			diagnostic.Reset()
			if code := runActivityCLI(wrongAssignment, &out, &diagnostic); code == 0 {
				t.Fatal("Reviewer accepted foreign assignment", out.String())
			}
			rows, err := (activity.Ledger{Project: project}).List("B-1")
			if err != nil || len(rows) != 0 {
				t.Fatal("negative/advisory path published Activity", rows, err)
			}
			out.Reset()
			diagnostic.Reset()
			if code := runActivityCLI(report, &out, &diagnostic); code != 0 {
				t.Fatalf("generated instruction exit%d: %s %s", code, out.String(), diagnostic.String())
			}
			var ack struct {
				OK       bool           `json:"ok"`
				Recorded bool           `json:"recorded"`
				Event    activity.Event `json:"event"`
			}
			if err := json.Unmarshal(out.Bytes(), &ack); err != nil {
				t.Fatal(err)
			}
			if !ack.OK || !ack.Recorded || ack.Event.Role != activity.RoleReviewer || ack.Event.AssignmentID != "exact-assignment" || ack.Event.RuntimeAttemptID != result.Reporting.AttemptID || ack.Event.TaskRevision == "" || ack.Event.ContractSHA256 == "" {
				t.Fatalf("missing readonly source ACK: %s", out.String())
			}
			receipts, err := runtimeobs.ReadCollectionReceipts(project)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, receipt := range receipts {
				if receipt.Status == "ack" && receipt.Activity != nil && receipt.Activity.Role == "reviewer" && receipt.Activity.EventID == ack.Event.EventID {
					found = true
				}
			}
			if !found {
				t.Fatal("generated Reviewer report lacks durable collection ACK")
			}
			out.Reset()
			diagnostic.Reset()
			if code := runRuntimeObserveT3CLI(args, &out, &diagnostic); code != 0 {
				t.Fatal(code, out.String(), diagnostic.String())
			}
			result.Reporting = activity.ReportingRequirement{}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Reporting.Status != "recorded" || result.Reporting.ActionRequired || len(result.Reporting.Arguments) != 0 {
				t.Fatalf("Reviewer ACK not recognized on retry: %s", out.String())
			}
		})
	}
}
