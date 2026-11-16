package handoff

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// Only temp Git/source and temp operational journals are used in this fixture.
func appendA79ControllerAttempt(t *testing.T, project, attempt string, state runtimeobs.CanonicalState, at time.Time) {
	t.Helper()
	for _, e := range []runtimeobs.ExecutionEvent{
		{EventKind: "state", ObservedAt: at.Format(time.RFC3339Nano), AttemptID: attempt, Provider: "codex", SessionID: "controller-native", RuntimeAgentID: "controller-native", State: state, Terminal: state == runtimeobs.StateCompleted, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved},
		{EventKind: "binding", ObservedAt: at.Add(time.Millisecond).Format(time.RFC3339Nano), AttemptID: attempt, AgentPath: "/root/controller", BindingSource: "test", EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative},
	} {
		if err := runtimeobs.AppendExecutionEvent(project, e); err != nil {
			t.Fatal(err)
		}
	}
}

func a79ContextFixture(t *testing.T) (PrepareRequest, ExecutionContext, time.Time) {
	t.Helper()
	project, source := t.TempDir(), t.TempDir()
	if b, e := exec.Command("git", "init", "-q", source).CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	if e := os.WriteFile(filepath.Join(source, "source.go"), []byte("package candidate\n"), 0600); e != nil {
		t.Fatal(e)
	}
	now := time.Date(2026, 11, 16, 3, 0, 0, 0, time.UTC)
	appendA79ControllerAttempt(t, project, "controller-current", runtimeobs.StateRunning, now)
	c := ExecutionContext{SchemaVersion: 1, ControllerAttemptID: "controller-current", References: []string{"B-621 historical reference is not B-638 evidence"}}
	for _, task := range []string{"B-638", "B-621"} {
		writeSimpleTask(t, project, task)
		taskFile := filepath.Join(project, "_task_mecca/data/backlog/000001."+task+".handoff-test.todo.md")
		original, e := os.ReadFile(taskFile)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(taskFile, []byte(strings.Replace(string(original), "- Agent: -", "- Agent: /root/controller/pairi", 1)), 0600); e != nil {
			t.Fatal(e)
		}
		worker, assignment := "worker-"+task, "assignment-"+task
		if _, e := runtimeobs.RecordAssignment(project, assignment, task, "/root/controller/pairi", now); e != nil {
			t.Fatal(e)
		}
		for _, event := range []runtimeobs.ExecutionEvent{
			{EventKind: "state", ObservedAt: now.Format(time.RFC3339Nano), AttemptID: worker, Provider: "codex", SessionID: worker + "-native", RuntimeAgentID: worker + "-native", State: runtimeobs.StateCompleted, Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved},
			{EventKind: "binding", ObservedAt: now.Add(time.Second).Format(time.RFC3339Nano), AttemptID: worker, TaskID: task, AgentPath: "/root/controller/pairi", BindingSource: "dispatch", BindingEvidence: map[string]string{"assignment_id": assignment}, EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative},
		} {
			if e := runtimeobs.AppendExecutionEvent(project, event); e != nil {
				t.Fatal(e)
			}
		}
		h, e := Prepare(PrepareRequest{Project: project, TaskID: task, EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAttemptID: c.ControllerAttemptID, ExecutionAuthorized: true}, now.Add(time.Minute))
		if e != nil {
			t.Fatal(e)
		}
		raw, e := rolepolicy.TaskMarkdown(project, task)
		if e != nil {
			t.Fatal(e)
		}
		contract, e := ContractFingerprint(raw)
		if e != nil {
			t.Fatal(e)
		}
		ac, e := AcceptanceFingerprint(raw)
		if e != nil {
			t.Fatal(e)
		}
		snapshot, e := rolepolicy.Snapshot(source, []string{"source.go"})
		if e != nil {
			t.Fatal(e)
		}
		target := TaskEvidence{TaskID: task, ContractSHA256: contract, AcceptanceSHA256: ac, OriginalHandoffID: h.HandoffID, Phase: "worker_completed", SourceAttemptID: worker, AssignmentID: assignment, SourceWorkspace: source, Source: snapshot, ReportFile: filepath.Join(source, task+".md"), Changes: []string{"source.go"}, Tests: []CheckEvidence{{Command: "bounded offline fixture", Result: "passed", Evidence: "temp-ledger/source assertions"}}, OriginalErrors: []string{"sanitized original error preserved"}}
		writeA79Report(t, &target)
		c.Targets = append(c.Targets, target)
	}
	req := PrepareRequest{Project: project, TaskID: "B-638", EventType: EventWorkerDone, SourceAgentPath: "/root/controller/pairi", SourceAttemptID: c.Targets[0].SourceAttemptID, SourceWorkspace: source, TargetAgentPath: "/root/controller", TargetAttemptID: c.ControllerAttemptID, ReportFile: c.Targets[0].ReportFile, ContextFile: filepath.Join(source, "context.json")}
	writeA79Context(t, req.ContextFile, c)
	return req, c, now.Add(2 * time.Minute)
}

func writeA79Context(t *testing.T, path string, c ExecutionContext) {
	t.Helper()
	b, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func writeA79Report(t *testing.T, target *TaskEvidence) {
	t.Helper()
	b, e := json.Marshal(struct {
		Targets []TaskEvidence `json:"targets"`
	}{[]TaskEvidence{*target}})
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(target.ReportFile, append(append([]byte("TASK_MECCA_REPORT "), b...), []byte("\nActual scoped report; history is retained.\n")...), 0600); e != nil {
		t.Fatal(e)
	}
	target.ReportSHA256, e = rolepolicy.HashFile(target.ReportFile)
	if e != nil {
		t.Fatal(e)
	}
}

func attachA79PolicyContext(t *testing.T, req *PrepareRequest, scope []string, now time.Time) {
	t.Helper()
	original, e := Prepare(PrepareRequest{Project: req.Project, TaskID: req.TaskID, EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAttemptID: req.TargetAttemptID, ExecutionAuthorized: true}, now)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := rolepolicy.TaskMarkdown(req.Project, req.TaskID)
	if e != nil {
		t.Fatal(e)
	}
	contract, e := ContractFingerprint(raw)
	if e != nil {
		t.Fatal(e)
	}
	ac, e := AcceptanceFingerprint(raw)
	if e != nil {
		t.Fatal(e)
	}
	source, e := rolepolicy.Snapshot(req.SourceWorkspace, scope)
	if e != nil {
		t.Fatal(e)
	}
	target := TaskEvidence{TaskID: req.TaskID, ContractSHA256: contract, AcceptanceSHA256: ac, OriginalHandoffID: original.HandoffID, Phase: "worker_completed", SourceAttemptID: req.SourceAttemptID, AssignmentID: "assignment-worker", SourceWorkspace: req.SourceWorkspace, Source: source, ReportFile: req.ReportFile, Changes: scope, Tests: []CheckEvidence{}}
	writeA79Report(t, &target)
	req.ContextFile = filepath.Join(req.SourceWorkspace, "context.json")
	writeA79Context(t, req.ContextFile, ExecutionContext{SchemaVersion: 1, ControllerAttemptID: req.TargetAttemptID, Targets: []TaskEvidence{target}})
}

func TestA79B638RejectsB621ReportWithoutChangingEitherTask(t *testing.T) {
	req, c, now := a79ContextFixture(t)
	ledgerDir := filepath.Join(req.Project, "_task_mecca/data/backlog")
	entries, e := os.ReadDir(ledgerDir)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".B-621.") {
			if e = os.Rename(filepath.Join(ledgerDir, entry.Name()), filepath.Join(ledgerDir, strings.Replace(entry.Name(), ".todo.md", ".done.md", 1))); e != nil {
				t.Fatal(e)
			}
		}
	}
	before, _ := rolepolicy.TaskMarkdown(req.Project, "B-621")
	wrong := c
	wrong.Targets = []TaskEvidence{c.Targets[1]}
	writeA79Context(t, req.ContextFile, wrong)
	diagnostic, e := CheckContext(req.Project, req.TaskID, req.TargetAttemptID, wrong, true, now)
	if e == nil || diagnostic.TaskID != "B-638" || len(diagnostic.ReportedTargets) != 1 || diagnostic.ReportedTargets[0].TaskID != "B-621" || diagnostic.ReportedTargets[0].OriginalErrors[0] != "sanitized original error preserved" || diagnostic.ActionOwner != "/root/controller" || diagnostic.ResumeCondition == "" {
		t.Fatal("mismatch lost original actor/report/error or owner", diagnostic, e)
	}
	if _, e := Prepare(req, now); e == nil || !strings.Contains(e.Error(), "completion_target_mismatch") {
		t.Fatal("foreign result accepted", e)
	}
	c.Targets[0].ReportFile = c.Targets[1].ReportFile
	c.Targets[0].ReportSHA256 = c.Targets[1].ReportSHA256
	req.ReportFile = c.Targets[1].ReportFile
	writeA79Context(t, req.ContextFile, c)
	if _, e := Prepare(req, now); e == nil || !strings.Contains(e.Error(), "completion_report_target") {
		t.Fatal("retagged foreign report accepted", e)
	}
	after, _ := rolepolicy.TaskMarkdown(req.Project, "B-621")
	if before != after {
		t.Fatal("unrelated task changed")
	}
	l, e := BuildLedger(req.Project, now)
	if e != nil {
		t.Fatal(e)
	}
	for _, h := range l.Handoffs {
		if completionEvent(h.EventType) {
			t.Fatal("invalid result recorded", h)
		}
	}
}

func TestA79SameTaskMultiTargetAndDuplicateCompletion(t *testing.T) {
	req, c, now := a79ContextFixture(t)
	for _, target := range c.Targets {
		r := req
		r.TaskID = target.TaskID
		r.SourceAttemptID = target.SourceAttemptID
		r.ReportFile = target.ReportFile
		p, e := Prepare(r, now)
		if e != nil {
			t.Fatal(e)
		}
		dup, e := Prepare(r, now.Add(time.Second))
		if e != nil || !dup.Duplicate || p.HandoffID != dup.HandoffID {
			t.Fatal("retry not idempotent", dup, e)
		}
		claimed, e := Claim(r.Project, p.HandoffID, "/root/controller", c.ControllerAttemptID, now)
		if e != nil || !claimed.Claimed {
			t.Fatal(claimed, e)
		}
		if _, e = MarkAs(r.Project, p.HandoffID, "applied", "ok", "early", c.ControllerAttemptID, now); e == nil {
			t.Fatal("applied skipped acceptance")
		}
		if _, e = MarkAs(r.Project, p.HandoffID, "acceptance", "ok", "matched task report/source", c.ControllerAttemptID, now); e != nil {
			t.Fatal(e)
		}
		if _, e = MarkAs(r.Project, p.HandoffID, "applied", "ok", "finalized exact target", c.ControllerAttemptID, now); e != nil {
			t.Fatal(e)
		}
		if _, e = MarkAs(r.Project, p.HandoffID, "applied", "ok", "retry after historical source disappeared", c.ControllerAttemptID, now); e != nil {
			t.Fatal(e)
		}
	}
}

func TestA79ContractAttemptReportSourceAndPhaseRegressions(t *testing.T) {
	for _, kind := range []string{"contract", "acceptance", "controller_attempt", "source_attempt", "assignment", "phase", "source", "report", "missing_context", "duplicate_target", "original_handoff"} {
		t.Run(kind, func(t *testing.T) {
			req, c, now := a79ContextFixture(t)
			switch kind {
			case "contract":
				c.Targets[0].ContractSHA256 = strings.Repeat("a", 64)
			case "acceptance":
				c.Targets[0].AcceptanceSHA256 = strings.Repeat("a", 64)
			case "controller_attempt":
				c.ControllerAttemptID = "controller-old"
			case "source_attempt":
				c.Targets[0].SourceAttemptID = c.Targets[1].SourceAttemptID
			case "assignment":
				c.Targets[0].AssignmentID = c.Targets[1].AssignmentID
			case "phase":
				c.Targets[0].Phase = "turn_ended"
			case "source":
				os.WriteFile(filepath.Join(req.SourceWorkspace, "source.go"), []byte("changed"), 0600)
			case "report":
				os.WriteFile(req.ReportFile, []byte("old error unchanged? no, report changed"), 0600)
			case "missing_context":
				req.ContextFile = ""
			case "duplicate_target":
				c.Targets = append(c.Targets, c.Targets[0])
			case "original_handoff":
				c.Targets[0].OriginalHandoffID = c.Targets[1].OriginalHandoffID
			}
			if req.ContextFile != "" {
				writeA79Context(t, req.ContextFile, c)
			}
			if _, e := Prepare(req, now); e == nil {
				t.Fatal("invalid evidence accepted")
			}
		})
	}
}

func TestA79ClaimAndAcceptanceRecheckImmutableEvidence(t *testing.T) {
	for _, boundary := range []string{"claim", "acceptance", "applied"} {
		t.Run(boundary, func(t *testing.T) {
			req, _, now := a79ContextFixture(t)
			p, e := Prepare(req, now)
			if e != nil {
				t.Fatal(e)
			}
			if boundary != "claim" {
				if c, e := Claim(req.Project, p.HandoffID, "/root/controller", req.TargetAttemptID, now); e != nil || !c.Claimed {
					t.Fatal(c, e)
				}
			}
			if boundary == "applied" {
				if _, e := MarkAs(req.Project, p.HandoffID, "acceptance", "ok", "actual source", req.TargetAttemptID, now); e != nil {
					t.Fatal(e)
				}
			}
			if e = os.WriteFile(filepath.Join(req.SourceWorkspace, "source.go"), []byte("changed after prior gate"), 0600); e != nil {
				t.Fatal(e)
			}
			if boundary == "claim" {
				c, e := Claim(req.Project, p.HandoffID, "/root/controller", req.TargetAttemptID, now)
				if e != nil || !c.ClaimConflict || c.Claimed {
					t.Fatal(c, e)
				}
			} else {
				if _, e := MarkAs(req.Project, p.HandoffID, boundary, "ok", "stale evidence", req.TargetAttemptID, now); e == nil {
					t.Fatal("stale completion accepted")
				}
			}
			h, _, e := Inspect(req.Project, p.HandoffID, now)
			if e != nil || h.Applied {
				t.Fatal(h, e)
			}
		})
	}
}

func TestA79ReadOnlyResumeCLIAndUnsupportedTurnEnd(t *testing.T) {
	req, c, now := a79ContextFixture(t)
	start, e := StartContext(req.Project, c.Targets[0].OriginalHandoffID, c.ControllerAttemptID, now)
	if e != nil || start.Status != "collection_gap" || start.Target.Phase != "assigned_prepared" {
		t.Fatal(start, e)
	}
	if _, e = StartContext(req.Project, c.Targets[0].OriginalHandoffID, "stale-controller", now); e == nil {
		t.Fatal("stale resume accepted")
	}
	before, _ := BuildLedger(req.Project, now)
	var out, stderr bytes.Buffer
	code := RunCLI([]string{"context", c.Targets[0].OriginalHandoffID, "--project", req.Project, "--target-attempt", c.ControllerAttemptID, "--json"}, &out, &stderr)
	if code != 0 || !strings.Contains(out.String(), "collection_gap") {
		t.Fatal(code, out.String(), stderr.String())
	}
	after, _ := BuildLedger(req.Project, now)
	if len(before.Handoffs) != len(after.Handoffs) {
		t.Fatal("read-only resume wrote handoff")
	}
}

func TestA79CurrentAttemptReplacementPreservesOriginalReportAndRetry(t *testing.T) {
	req, c, now := a79ContextFixture(t)
	p, e := Prepare(req, now)
	if e != nil {
		t.Fatal(e)
	}
	old := req.TargetAttemptID
	appendA79ControllerAttempt(t, req.Project, old, runtimeobs.StateCompleted, now.Add(time.Second))
	appendA79ControllerAttempt(t, req.Project, "controller-next", runtimeobs.StateRunning, now.Add(2*time.Second))
	if _, e = Prepare(PrepareRequest{Project: req.Project, TaskID: req.TaskID, EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAttemptID: "controller-next", ExecutionAuthorized: true}, now.Add(3*time.Second)); e != nil {
		t.Fatal(e)
	}
	if _, e = CheckContext(req.Project, req.TaskID, "controller-next", c, true, now.Add(3*time.Second)); e == nil {
		t.Fatal("old observation silently reused")
	}
	c.ControllerAttemptID = "controller-next"
	req.TargetAttemptID = "controller-next"
	writeA79Context(t, req.ContextFile, c)
	retarget, e := Prepare(req, now.Add(3*time.Second))
	if e != nil || !retarget.Duplicate || retarget.HandoffID != p.HandoffID {
		t.Fatal(retarget, e)
	}
	if claimed, e := Claim(req.Project, p.HandoffID, "/root/controller", old, now.Add(4*time.Second)); e != nil || !claimed.ClaimConflict {
		t.Fatal(claimed, e)
	}
	claimed, e := Claim(req.Project, p.HandoffID, "/root/controller", req.TargetAttemptID, now.Add(4*time.Second))
	if e != nil || !claimed.Claimed {
		t.Fatal(claimed, e)
	}
	checked, e := StartContext(req.Project, p.HandoffID, req.TargetAttemptID, now.Add(4*time.Second))
	if e != nil || checked.ReportedControllerAttemptID != old || checked.CurrentControllerAttemptID != req.TargetAttemptID {
		t.Fatal(checked, e)
	}
	if checked.Target.OriginalErrors[0] != "sanitized original error preserved" {
		t.Fatal("original error lost", checked)
	}
}
