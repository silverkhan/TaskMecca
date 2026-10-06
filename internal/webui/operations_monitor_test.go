package webui

import (
	"bytes"
	stdcontext "context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"github.com/silverkhan/TaskMecca/internal/notify"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func testOperationProject(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca", "data", "backlog"), 0755); err != nil {
		t.Fatal(err)
	}
	return project
}

func testOperationHook(t *testing.T, project, event string, at time.Time) runtimeobs.ExecutionEvent {
	t.Helper()
	value, err := runtimeobs.ObserveHook(project, "codex", strings.NewReader(event), at)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestOperationEpisodesPreserveEvidenceAcrossRestartAndRecurrence(t *testing.T) {
	t.Setenv("TASK_MECCA_STALE_WARN_SECONDS", "2")
	project := testOperationProject(t)
	backlogPath := filepath.Join(project, "_task_mecca", "data", "backlog", "000005.A-5.monitor.doing.md")
	if err := os.WriteFile(backlogPath, []byte("# A-5 Monitor\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeobs.EnsureHooks(project, "codex"); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(time.Second)
	start := testOperationHook(t, project, `{"session_id":"s","turn_id":"t","hook_event_name":"SubagentStart","agent_id":"worker"}`, base)
	if _, err := runtimeobs.BindAttempt(project, start.AttemptID, "A-5", "/root/controller/worker", "explicit", "", map[string]string{"source": "test"}, base.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	threshold := runtimeobs.StaleWarnDuration()
	if threshold != 2*time.Second {
		t.Fatalf("configured silence threshold=%v", threshold)
	}
	before, err := scanOperationProject(project, base.Add(threshold-time.Nanosecond))
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Incidents) != 0 {
		t.Fatalf("premature silence alert: %+v", before.Incidents)
	}

	first, err := scanOperationProject(project, base.Add(threshold+time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Incidents) != 1 || first.Incidents[0].Kind != "no_signal" || first.Incidents[0].Quality != "verification_required" {
		t.Fatalf("initial incident=%+v", first.Incidents)
	}
	id := first.Incidents[0].ID
	if first.Incidents[0].EndedAt != "" || first.Incidents[0].LastObservedAt == "" || first.Incidents[0].DetectedAt == "" {
		t.Fatalf("unsupported end time or missing evidence: %+v", first.Incidents[0])
	}
	second, err := scanOperationProject(project, base.Add(threshold+2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Incidents) != 1 || second.Incidents[0].ID != id {
		t.Fatalf("duplicate episode: %+v", second.Incidents)
	}

	activityAt := base.Add(threshold + 3*time.Second)
	testOperationHook(t, project, `{"session_id":"s","turn_id":"t","hook_event_name":"PreToolUse","agent_id":"worker","tool_use_id":"tool-1"}`, activityAt)
	recovered, err := scanOperationProject(project, activityAt.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Incidents[0].RecoveredAt == "" {
		t.Fatalf("recovery not recorded: %+v", recovered.Incidents)
	}
	returning, err := scanOperationProject(project, activityAt.Add(threshold+time.Second))
	if err != nil {
		t.Fatal(err)
	}
	var recurrence *operationIncident
	for i := range returning.Incidents {
		if returning.Incidents[i].Kind == "no_signal" && returning.Incidents[i].ID != id {
			recurrence = &returning.Incidents[i]
		}
	}
	if recurrence == nil {
		t.Fatalf("recurrence not a new episode: %+v", returning.Incidents)
	}
	reloaded, err := readOperationJournal(project)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Incidents[0].ID != id || reloaded.Incidents[0].RecoveredAt == "" {
		t.Fatalf("restart lost evidence: %+v", reloaded.Incidents)
	}
	if _, err := os.Stat(backlogPath); err != nil {
		t.Fatalf("monitor changed backlog lifecycle: %v", err)
	}
}

func TestOperationClassifiesConfirmedStopUnverifiedRuntimeAndMonitoringGap(t *testing.T) {
	project := testOperationProject(t)
	originalVerifier := operationHookVerifier
	operationHookVerifier = func(string, string) bool { return false }
	defer func() { operationHookVerifier = originalVerifier }()
	base := time.Now().UTC()
	start := testOperationHook(t, project, `{"session_id":"s2","hook_event_name":"SubagentStart","agent_id":"worker-2"}`, base)
	if _, err := runtimeobs.BindAttempt(project, start.AttemptID, "A-5", "/root/controller/worker", "explicit", "", nil, base.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	unknown, err := scanOperationProject(project, base.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(unknown.Incidents) != 1 || unknown.Incidents[0].Kind != "runtime_unknown" || unknown.Incidents[0].EndedAt != "" {
		t.Fatalf("missing hook configuration must remain unverified: %+v", unknown.Incidents)
	}
	stop := runtimeobs.ExecutionEvent{EventKind: "state", ObservedAt: base.Add(3 * time.Second).Format(time.RFC3339Nano), AttemptID: start.AttemptID,
		Provider: "codex", RuntimeAgentID: "worker-2", State: runtimeobs.StateErrored, Terminal: true,
		EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}
	if err := runtimeobs.AppendExecutionEvent(project, stop); err != nil {
		t.Fatal(err)
	}
	ended, err := scanOperationProject(project, base.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(ended.Incidents) != 2 || ended.Incidents[1].Kind != "errored" || ended.Incidents[1].Quality != "confirmed" || ended.Incidents[1].EndedAt == "" {
		t.Fatalf("confirmed stop misclassified: %+v", ended.Incidents)
	}
	if ended.Incidents[0].RecoveredAt == "" {
		t.Fatalf("unknown incident not closed: %+v", ended.Incidents)
	}
	gapAt := base.Add(operationGapAfter + time.Minute)
	gap, err := scanOperationProject(project, gapAt)
	if err != nil {
		t.Fatal(err)
	}
	last := gap.Incidents[len(gap.Incidents)-1]
	if last.Kind != "monitor_gap" || last.EndedAt != "" || last.RecoveredAt != gapAt.Format(time.RFC3339Nano) {
		t.Fatalf("gap invented session end or lost recovery: %+v", last)
	}
	restartAt := gapAt.Add(time.Second)
	restarted := testOperationHook(t, project, `{"session_id":"s3","hook_event_name":"SubagentStart","agent_id":"worker-3"}`, restartAt)
	if _, err := runtimeobs.BindAttempt(project, restarted.AttemptID, "A-5", "/root/controller/worker", "explicit", "", nil, restartAt.Add(time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	resumed, err := scanOperationProject(project, restartAt.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Incidents[1].RecoveredAt == "" {
		t.Fatalf("new attempt did not close confirmed stop: %+v", resumed.Incidents)
	}
}

func TestOperationMonitorScansAllProjectsWithoutBrowserSubscription(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	primary := testOperationProject(t)
	secondary := testOperationProject(t)
	if err := maintenance.RegisterProject(primary); err != nil {
		t.Fatal(err)
	}
	if err := maintenance.RegisterProject(secondary); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	for _, project := range []string{primary, secondary} {
		start := testOperationHook(t, project, `{"session_id":"s3","hook_event_name":"SubagentStart","agent_id":"worker-3"}`, base)
		if _, err := runtimeobs.BindAttempt(project, start.AttemptID, "A-5", "/root/controller/worker", "explicit", "", nil, base.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	original := operationDeliver
	defer func() { operationDeliver = original }()
	delivered := map[string][]notify.Event{}
	operationDeliver = func(project string, events []notify.Event) []error {
		delivered[project] = append(delivered[project], events...)
		return nil
	}
	// The server monitor's scan does not require an HTTP request or SSE client.
	scanOperationProjects(primary, base.Add(2*time.Second))
	for _, project := range []string{primary, secondary} {
		journal, err := readOperationJournal(project)
		if err != nil {
			t.Fatal(err)
		}
		if journal.LastScanAt == "" || len(journal.Incidents) != 1 || len(delivered[project]) != 1 {
			t.Fatalf("project %s not monitored/delivered: %+v %+v", project, journal, delivered[project])
		}
	}
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	runOperationMonitor(ctx, primary)
}

func TestOperationDoingAssignmentWithoutHookIsNotDeclaredDead(t *testing.T) {
	project := testOperationProject(t)
	path := filepath.Join(project, "_task_mecca", "data", "backlog", "000005.A-5.assigned.doing.md")
	content := "# A-5 Assigned\n- Agent: /root/controller/worker\n- 변경범위: -\n- 선행: -\n- 연관: -\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	journal, err := scanOperationProject(project, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(journal.Incidents) != 1 || journal.Incidents[0].Kind != "runtime_unknown" || journal.Incidents[0].Quality != "verification_required" || journal.Incidents[0].EndedAt != "" {
		t.Fatalf("unlinked assignment declared dead or ignored: %+v", journal.Incidents)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("backlog state changed: %v", err)
	}
}

func TestOperationExplicitOutcomesHaveDistinctEvidenceAndSeverity(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		state runtimeobs.CanonicalState
		kind  string
		alert bool
	}{
		{runtimeobs.StateInterrupted, "interrupted", true},
		{runtimeobs.StateErrored, "errored", true},
		{runtimeobs.StateShutdown, "shutdown", true},
		{runtimeobs.StateCompleted, "normal_exit", false},
	} {
		attempt := runtimeobs.Attempt{AttemptID: "run-1", TaskID: "A-5", CurrentState: tc.state,
			StateEvidenceSource: runtimeobs.EvidenceHook, StateObservationQuality: runtimeobs.QualityObserved,
			EndedAt: now.Format(time.RFC3339Nano)}
		signal, ok := classifyOperation("/project", attempt, now, false)
		if !ok || signal.incident.Kind != tc.kind || signal.incident.Quality != "confirmed" || signal.alert != tc.alert {
			t.Fatalf("%s classified as %+v, ok=%v", tc.state, signal, ok)
		}
		attempt.StateObservationQuality = runtimeobs.QualityInferred
		inferred, found := classifyOperation("/project", attempt, now, false)
		if tc.state != runtimeobs.StateCompleted && (!found || inferred.incident.Kind != "runtime_unknown") {
			t.Fatalf("inferred terminal state declared confirmed: %+v", inferred)
		}
	}
}

func TestOperationWebProcessContinuesAfterBrowserClose(t *testing.T) {
	if os.Getenv("TASK_MECCA_OPERATION_HELPER") == "1" {
		port, err := strconv.Atoi(os.Getenv("TASK_MECCA_OPERATION_PORT"))
		if err != nil {
			t.Fatal(err)
		}
		if err := Run(Config{Project: os.Getenv("TASK_MECCA_OPERATION_PROJECT"), Host: "127.0.0.1", Port: port,
			OpenBrowser: false, Version: "test", InstanceID: "ops-e2e", ControlToken: "ops-e2e-token"}); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	project := testOperationProject(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestOperationWebProcessContinuesAfterBrowserClose$")
	cmd.Env = append(os.Environ(), "TASK_MECCA_OPERATION_HELPER=1", "TASK_MECCA_OPERATION_PROJECT="+project,
		"TASK_MECCA_OPERATION_PORT="+strconv.Itoa(port))
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	url := fmt.Sprintf("http://127.0.0.1:%d/api/operations", port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, getErr := http.Get(url)
		if getErr == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("Web did not start: %v\n%s", err, output.String())
	}
	response.Body.Close() // Browser disconnects before the incident is created.
	path := filepath.Join(project, "_task_mecca", "data", "backlog", "000005.A-5.assigned.doing.md")
	if err := os.WriteFile(path, []byte("# A-5 Assigned\n- Agent: /root/controller/worker\n- 변경범위: -\n- 선행: -\n- 연관: -\n"), 0644); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(operationScanInterval + 8*time.Second)
	var incident operationIncident
	for time.Now().Before(deadline) {
		journal, readErr := readOperationJournal(project)
		if readErr == nil && len(journal.Incidents) > 0 {
			incident = journal.Incidents[0]
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if incident.ID == "" || incident.Kind != "runtime_unknown" {
		t.Fatalf("server stopped monitoring after browser disconnect: %+v\n%s", incident, output.String())
	}
	response, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	var api struct {
		Active   []operationIncident `json:"active"`
		Projects []map[string]any    `json:"projects"`
	}
	decodeErr := json.NewDecoder(response.Body).Decode(&api)
	response.Body.Close()
	if decodeErr != nil || len(api.Active) != 1 || api.Active[0].ID != incident.ID || len(api.Projects) != 1 || api.Projects[0]["last_scan_at"] == "" {
		t.Fatalf("Web did not expose incident and scan scope: %+v, error=%v", api, decodeErr)
	}
	deliveryDeadline := time.Now().Add(3 * time.Second)
	for {
		records, deliveryErr := notify.DeliveryRecords(project, "", incident.ID)
		if deliveryErr == nil && len(records) == 1 {
			break
		}
		if time.Now().After(deliveryDeadline) {
			t.Fatalf("incident did not reach Telegram delivery ledger: %v %+v", deliveryErr, records)
		}
		time.Sleep(50 * time.Millisecond)
	}
	stopRequest, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/api/admin/stop", port), nil)
	if err != nil {
		t.Fatal(err)
	}
	stopRequest.Header.Set("X-Task-Mecca-Control", "ops-e2e-token")
	stopResponse, err := http.DefaultClient.Do(stopRequest)
	if err != nil {
		t.Fatalf("stop Web: %v\n%s", err, output.String())
	}
	stopResponse.Body.Close()
	if stopResponse.StatusCode != http.StatusOK {
		t.Fatalf("stop Web HTTP %d", stopResponse.StatusCode)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("Web process exit: %v\n%s", err, output.String())
	}
	// Restart must preserve the incident and identify the offline interval as
	// a monitoring gap, never as a fabricated session termination time.
	journal, err := readOperationJournal(project)
	if err != nil {
		t.Fatal(err)
	}
	journal.LastScanAt = time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	if err := writeOperationJournal(project, journal); err != nil {
		t.Fatal(err)
	}
	second := exec.Command(os.Args[0], "-test.run=^TestOperationWebProcessContinuesAfterBrowserClose$")
	second.Env = cmd.Env
	var secondOutput bytes.Buffer
	second.Stdout, second.Stderr = &secondOutput, &secondOutput
	if err := second.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if second.ProcessState == nil {
			_ = second.Process.Kill()
			_ = second.Wait()
		}
	}()
	deadline = time.Now().Add(5 * time.Second)
	gapFound := false
	for time.Now().Before(deadline) {
		reloaded, readErr := readOperationJournal(project)
		if readErr == nil {
			for _, item := range reloaded.Incidents {
				if item.Kind == "monitor_gap" && item.EndedAt == "" && item.RecoveredAt != "" {
					gapFound = true
				}
			}
			if gapFound && reloaded.Incidents[0].ID == incident.ID {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !gapFound {
		t.Fatalf("restart did not retain episode and record gap: %s", secondOutput.String())
	}
	stopRequest, err = http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/api/admin/stop", port), nil)
	if err != nil {
		t.Fatal(err)
	}
	stopRequest.Header.Set("X-Task-Mecca-Control", "ops-e2e-token")
	stopResponse, err = http.DefaultClient.Do(stopRequest)
	if err != nil {
		t.Fatalf("stop restarted Web: %v", err)
	}
	stopResponse.Body.Close()
	if stopResponse.StatusCode != http.StatusOK {
		t.Fatalf("stop restarted Web HTTP %d", stopResponse.StatusCode)
	}
	if err := second.Wait(); err != nil {
		t.Fatalf("restarted Web exit: %v\n%s", err, secondOutput.String())
	}
}
