package handoff

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// Offline tool-contract fixtures only: no native agent, service or messages.
type fixtureTransport struct {
	state       string
	run         string
	error       error
	accessError error
	calls       []string
	sends       int
	ack         json.RawMessage
}

func (f *fixtureTransport) FreshFullAccess(string) error {
	f.calls = append(f.calls, "preflight")
	return f.accessError
}
func (f *fixtureTransport) Call(tool string, args map[string]any) (json.RawMessage, error) {
	f.calls = append(f.calls, tool)
	if args["threadId"] != "thread-exact" {
		return nil, errors.New("foreign recipient")
	}
	if tool == "t3_thread_read" {
		return json.Marshal(map[string]any{"thread": map[string]any{"threadId": "thread-exact", "status": f.state}, "recentRuns": []any{map[string]any{"runId": f.run}}})
	}
	f.sends++
	if args["mode"] != "auto" || args["clientRequestId"] == "" || !strings.Contains(args["message"].(string), "report_id=") {
		return nil, errors.New("missing stable envelope")
	}
	if f.error != nil {
		return nil, f.error
	}
	if f.ack != nil {
		return f.ack, nil
	}
	return json.RawMessage(`{"threadId":"thread-exact","messageId":"actual-message-fixture","delivery":"started","runId":"next-run"}`), nil
}

type blockingTransport struct {
	fixtureTransport
	entered, release chan struct{}
}

func (b *blockingTransport) Call(tool string, args map[string]any) (json.RawMessage, error) {
	if tool == "t3_thread_send" {
		close(b.entered)
		<-b.release
	}
	return b.fixtureTransport.Call(tool, args)
}
func TestTransportConcurrentDONEHasOneSend(t *testing.T) {
	project := t.TempDir()
	p := fixturePlan()
	f := &blockingTransport{fixtureTransport: fixtureTransport{state: "running", run: p.T3RunID}, entered: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	go func() { _, err := deliverPlannedReport(project, p, "report.md", "sha", f); done <- err }()
	select {
	case <-f.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("fixture send did not start")
	}
	_, err := deliverPlannedReport(project, p, "report.md", "sha", f)
	if err == nil || !strings.Contains(err.Error(), "checkpoint busy") {
		t.Fatalf("concurrent send not fenced: %v", err)
	}
	close(f.release)
	if err = <-done; err != nil || f.sends != 1 {
		t.Fatalf("sends=%d err=%v", f.sends, err)
	}
}

func fixturePlan() TransportPlan {
	return TransportPlan{HandoffID: "report-handoff", EventType: EventWorkerDone, T3RunID: "run-exact", Target: Target{AgentPath: "/root/controller", AttemptID: "attempt-exact", RuntimeAgentID: "native-exact", SessionID: "native-exact"}, Tool: "t3_thread_send", Address: TransportAddress{T3ThreadID, "thread-exact"}, Mode: "auto", Supported: true, ActionOwner: "/root/controller/worker", ResumeCondition: "repair exact recipient and retry same report"}
}

func TestTransportRejectsWrongIDKind(t *testing.T) {
	native := TransportAddress{NativeSessionID, "01a1c465-2fc0-7d62-b255-1ffc0e90d7ee"}
	for _, tool := range []string{"collaboration.send_message", "collaboration.followup_task", "t3_thread_send"} {
		if ValidateAddress(tool, native) == nil {
			t.Fatalf("native accepted by %s", tool)
		}
	}
	if ValidateAddress("collaboration.send_message", TransportAddress{T3ThreadID, "thread-exact"}) == nil {
		t.Fatal("T3 accepted as collaboration handle")
	}
	if ValidateAddress("t3_thread_send", fixturePlan().Address) != nil {
		t.Fatal("valid T3 rejected")
	}
}

func TestTransportAgentNotFoundRetryAndDuplicateDONE(t *testing.T) {
	project := t.TempDir()
	p := fixturePlan()
	original := "agent with id 01a1c465-2fc0-7d62-b255-1ffc0e90d7ee not found"
	f := &fixtureTransport{state: "completed", run: p.T3RunID, error: RejectedSend{errors.New(original)}}
	c, err := deliverPlannedReport(project, p, "report.md", "report-sha", f)
	if err == nil || c.Status != "undelivered" || c.OriginalError != original || c.Recipient != p.Address || c.ReportID != p.HandoffID || c.ActionOwner == "" || c.ResumeCondition == "" {
		t.Fatalf("%+v %v", c, err)
	}
	if !reflect.DeepEqual(f.calls, []string{"t3_thread_read", "preflight", "t3_thread_send"}) {
		t.Fatal(f.calls)
	}
	f.error = nil
	f.calls = nil
	retry, err := deliverPlannedReport(project, p, "report.md", "report-sha", f)
	if err != nil || retry.Status != "transport_acknowledged" || retry.Attempts != 2 || retry.ReportID != c.ReportID || retry.OriginalError != original {
		t.Fatalf("%+v %v", retry, err)
	}
	_, err = deliverPlannedReport(project, p, "report.md", "report-sha", f)
	if err != nil || f.sends != 2 {
		t.Fatalf("duplicate DONE sent again: %d %v", f.sends, err)
	}
	// No lifecycle/claim/apply side effect: only transport checkpoint exists.
	if _, err = os.Stat(JournalPath(project)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("transport wrote handoff journal")
	}
}

func TestTransportUncertainSendDoesNotDuplicate(t *testing.T) {
	for _, badAck := range []json.RawMessage{nil, json.RawMessage(`{"ok":true,"applied":true}`), json.RawMessage(`{"threadId":"foreign","messageId":"m","delivery":"started"}`)} {
		t.Run(string(badAck), func(t *testing.T) {
			f := &fixtureTransport{state: "running", run: "run-exact", ack: badAck}
			if badAck == nil {
				f.error = errors.New("connection lost after send")
			}
			project := t.TempDir()
			p := fixturePlan()
			c, err := deliverPlannedReport(project, p, "report.md", "sha", f)
			if err == nil || c.Status != "delivery_unknown" {
				t.Fatalf("%+v %v", c, err)
			}
			_, _ = deliverPlannedReport(project, p, "report.md", "sha", f)
			if f.sends != 1 {
				t.Fatal("unknown delivery repeated")
			}
		})
	}
}

func TestTransportResumeGatesAndChangedAttempt(t *testing.T) {
	for _, tc := range []struct {
		name, state, run string
		access           error
	}{
		{"access_denied", "completed", "run-exact", errors.New("Full Access unavailable")},
		{"new_episode", "completed", "new-run", nil},
		{"unknown", "unknown", "run-exact", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fixtureTransport{state: tc.state, run: tc.run, accessError: tc.access}
			c, err := deliverPlannedReport(t.TempDir(), fixturePlan(), "report.md", "sha", f)
			if err == nil || c.Status != "undelivered" || f.sends != 0 {
				t.Fatalf("%+v %v sends=%d", c, err, f.sends)
			}
		})
	}
}

func TestTransportPreparedIsNotSTARTOrHandle(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Now().UTC()
	appendControllerAttempt(t, project, "controller", "session", "native", runtimeobs.StateRunning, now)
	report := filepath.Join(t.TempDir(), "report.md")
	if err := os.WriteFile(report, []byte("fixture report"), 0600); err != nil {
		t.Fatal(err)
	}
	req := PrepareRequest{Project: project, TaskID: "A-1", EventType: EventWorkerDone, SourceAgentPath: "/root/controller/worker", SourceAttemptID: "assignment-prepared-only", TargetAgentPath: "/root/controller", TargetAttemptID: "controller", ReportFile: report}
	p, err := Prepare(req, now)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanTransport(project, p.HandoffID, now.Add(time.Second))
	if err != nil || plan.Supported || plan.Reason != "source_start_not_observed" {
		t.Fatalf("%+v %v", plan, err)
	}
	req.EventType = EventRegistrationReady
	req.ExecutionAuthorized = true
	p, err = Prepare(req, now)
	if err != nil {
		t.Fatal(err)
	}
	plan, err = PlanTransport(project, p.HandoffID, now.Add(time.Second))
	if err != nil || plan.Supported || plan.Reason != "collaboration_handle_not_observed" {
		t.Fatalf("%+v %v", plan, err)
	}
	before, err := os.ReadFile(JournalPath(project))
	if err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := RunCLI([]string{"transport-plan", p.HandoffID, "--project", project, "--json"}, &out, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	if err = json.Unmarshal(out.Bytes(), &plan); err != nil || plan.Supported || plan.Reason != "collaboration_handle_not_observed" {
		t.Fatal(plan, err)
	}
	after, err := os.ReadFile(JournalPath(project))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("read-only plan changed handoff journal", err)
	}
}

func TestTransportUnsupportedRouteRetainsCheckpointUntilRepair(t *testing.T) {
	project := t.TempDir()
	p := fixturePlan()
	p.Supported = false
	p.Address = TransportAddress{}
	p.Reason = "collaboration_handle_not_observed"
	f := &fixtureTransport{state: "running", run: p.T3RunID}
	c, err := deliverPlannedReport(project, p, "report.md", "sha", f)
	if err == nil || c.Status != "undelivered" || f.sends != 0 {
		t.Fatal(c, err)
	}
	p = fixturePlan()
	c, err = deliverPlannedReport(project, p, "report.md", "sha", f)
	if err != nil || c.Status != "transport_acknowledged" || c.Attempts != 1 || !strings.Contains(c.OriginalError, "collaboration_handle_not_observed") {
		t.Fatal(c, err)
	}
	p.Address.Value = "other-thread"
	if _, err = deliverPlannedReport(project, p, "report.md", "sha", f); err == nil {
		t.Fatal("checkpoint recipient changed")
	}
}

func TestTransportT3PlanSeparatesIdentityAndResume(t *testing.T) {
	a := runtimeobs.Attempt{RuntimeAgentID: "native", SessionID: "native", CurrentState: runtimeobs.StateCompleted, NativeReceipt: &runtimeobs.T3Receipt{ThreadID: "thread-exact", RunID: "run-exact"}}
	p, err := planT3Transport(fixturePlan(), a)
	if err != nil || !p.Supported || !p.RequiresFreshPreflight || p.Address.Kind != T3ThreadID || p.Address.Value == a.RuntimeAgentID {
		t.Fatalf("%+v %v", p, err)
	}
	a.NativeReceipt.ThreadID = a.SessionID
	if _, err = planT3Transport(fixturePlan(), a); err == nil {
		t.Fatal("native-as-T3 accepted")
	}
}

func TestTransportAcknowledgementKeepsContinuationGates(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Now().UTC()
	appendControllerAttempt(t, project, "controller", "session", "native", runtimeobs.StateRunning, now)
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.WriteFile(filepath.Join(folder, "000002.A-2.next.todo.md"), []byte("# A-2 Next\n- 선행: A-1\n- 대기: -\n"), 0600); err != nil {
		t.Fatal(err)
	}
	assertNextReady := func(want bool) {
		t.Helper()
		r, e := backlog.Ready(project, "")
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, row := range r["ready"].([]map[string]any) {
			if row["id"] == "A-2" {
				found = true
			}
		}
		if found != want {
			t.Fatalf("next ready=%v want=%v", found, want)
		}
	}
	assertNextReady(false)
	req := PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", TargetAttemptID: "controller", ExecutionAuthorized: true}
	prepared, err := Prepare(req, now)
	if err != nil {
		t.Fatal(err)
	}
	p := fixturePlan()
	p.HandoffID = prepared.HandoffID
	f := &fixtureTransport{state: "completed", run: p.T3RunID}
	if _, err = deliverPlannedReport(project, p, "report.md", "sha", f); err != nil {
		t.Fatal(err)
	}
	row, _, err := Inspect(project, p.HandoffID, now)
	if err != nil || row.Applied || row.ClaimedBy != "" {
		t.Fatal("ack claimed/applied", row, err)
	}
	assertNextReady(false)
	if _, err = Mark(project, p.HandoffID, "applied", "ok", "ack only", now); err == nil {
		t.Fatal("transport bypassed review/finalization gates")
	}
	claim, err := Claim(project, p.HandoffID, "/root/controller", "controller", now.Add(time.Second))
	if err != nil || !claim.Claimed {
		t.Fatal(claim, err)
	}
	duplicate, err := Prepare(req, now.Add(2*time.Second))
	if err != nil || !duplicate.Duplicate || duplicate.HandoffID != p.HandoffID {
		t.Fatal(duplicate, err)
	}
	// Existing recovery owns review/finalization/dependency dispatch; no Root ACK.
	plan, err := PlanTransport(project, p.HandoffID, now.Add(3*time.Second))
	if err != nil || plan.Supported || plan.Reason != "already_claimed_continue_existing_recovery" {
		t.Fatal(plan, err)
	}
	// Simulate Controller's separately verified canonical finalization ONLY in
	// this temp fixture. Delivery never performs it and launches no next agent.
	if err := os.Rename(filepath.Join(folder, "000001.A-1.handoff-test.todo.md"), filepath.Join(folder, "000001.A-1.handoff-test.done.md")); err != nil {
		t.Fatal(err)
	}
	assertNextReady(true)
}
