package webui

import (
	"bytes"
	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"github.com/silverkhan/TaskMecca/internal/notify"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOperationExecutionOverlayDoesNotRewriteHistoryButActionChangesPersist(t *testing.T) {
	project := testOperationProject(t)
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	if err := maintenance.RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	a83TypedActionFixture(t, project, "A-83", "Owner permission required", "Owner: approve access", runtimeobs.StateRunning)
	installA83MockCurrentExecution(t)
	now := time.Now().UTC()
	first, err := scanOperationProject(project, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Actions) != 1 || !first.ExecutionEvidence["A-83"].Matched {
		t.Fatal("missing current typed cause/projection", first)
	}
	actionID := first.Actions[0].ID
	path := operationJournalPath(project)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("execution_evidence")) {
		t.Fatal("volatile execution overlay persisted as history")
	}
	baseline, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := scanOperationProject(project, now.Add(15*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	oldDeliver := operationDeliver
	defer func() { operationDeliver = oldDeliver }()
	var sent []notify.Event
	operationDeliver = func(_ string, events []notify.Event) []error { sent = append(sent, events...); return nil }
	deliverOperationIncidents(project, second)
	deliverOperationIncidents(project, second)
	view := operationSnapshot(project)
	projection, ok := view["execution_evidence"].([]runtimeobs.CurrentExecutionSnapshot)
	if !ok || len(projection) != 1 || projection[0].Identity.TaskID != "A-83" || !projection[0].Matched || projection[0].OfficialCurrent {
		t.Fatal("read-only diagnostic overlay missing or promoted to a stored live certificate", projection)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(baseline, after) || !baseline.ModTime().Equal(after.ModTime()) || baseline.Size() != after.Size() {
		t.Fatal("read-only refresh/delivery rewrote journal or detached heartbeat")
	}
	if len(sent) != 2 || sent[0].ID != actionID || sent[1].ID != actionID {
		t.Fatal("real human cause/retry identity lost", sent)
	}
	persisted, err := readOperationJournal(project)
	if err != nil || len(persisted.ExecutionEvidence) != 0 || persisted.LastScanAt != second.LastScanAt {
		t.Fatal("overlay leaked or durable scan time lost", persisted, err)
	}
	// A change to a nested durable action must not be hidden by a shallow
	// snapshot sharing the Actions backing array with the mutating reconciler.
	canonical := filepath.Join(project, "_task_mecca/data/backlog/000001.A-83.fixture.hold.md")
	document, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatal(err)
	}
	newAction := "Owner: grant the explicitly requested repository read permission"
	if err := os.WriteFile(canonical, []byte(strings.Replace(string(document), "Owner: approve access", newAction, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := scanOperationProject(project, now.Add(16*time.Second)); err != nil {
		t.Fatal(err)
	}
	changed, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err = readOperationJournal(project)
	if err != nil || os.SameFile(after, changed) || len(persisted.Actions) != 1 || persisted.Actions[0].NextAction != newAction || persisted.Actions[0].ID != actionID {
		t.Fatal("durable action change not persisted under same unresolved identity", persisted, err)
	}
}

func TestOperationNoChangeScanUsesDurableHeartbeat(t *testing.T) {
	project := testOperationProject(t)
	firstTime := time.Now().UTC().Truncate(time.Second)
	first, err := scanOperationProject(project, firstTime)
	if err != nil {
		t.Fatal(err)
	}
	path := operationJournalPath(project)
	original, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	secondTime := firstTime.Add(15 * time.Second)
	second, err := scanOperationProject(project, secondTime)
	if err != nil {
		t.Fatal(err)
	}
	same, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(original, same) || original.Size() != same.Size() || !original.ModTime().Equal(same.ModTime()) {
		t.Fatal("no-change scan rewrote large historical journal")
	}
	if _, err := os.Stat(operationHeartbeatPath(project)); err != nil {
		t.Fatalf("heartbeat not persisted: %v", err)
	}
	persisted, err := readOperationJournal(project)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.LastScanAt != second.LastScanAt || persisted.LastScanAt == first.LastScanAt {
		t.Fatalf("scan time lost across read: first=%q second=%q persisted=%q", first.LastScanAt, second.LastScanAt, persisted.LastScanAt)
	}
	// New monitoring-gap evidence is not a no-op, and must rewrite the
	// historical journal without letting an older heartbeat override it.
	thirdTime := secondTime.Add(operationGapAfter + time.Second)
	third, err := scanOperationProject(project, thirdTime)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(same, after) {
		t.Fatal("evidence change was not durably persisted")
	}
	reloaded, err := readOperationJournal(project)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.LastScanAt != third.LastScanAt || len(reloaded.Incidents) == 0 {
		t.Fatalf("gap evidence or last scan lost after rewrite: %+v", reloaded)
	}
}

func TestOperationHeartbeatFailsClosedOnJournalReplacement(t *testing.T) {
	project := testOperationProject(t)
	first := time.Now().UTC().Truncate(time.Second)
	if _, err := scanOperationProject(project, first); err != nil {
		t.Fatal(err)
	}
	if _, err := scanOperationProject(project, first.Add(15*time.Second)); err != nil {
		t.Fatal(err)
	}
	path := operationJournalPath(project)
	// External atomic replacement with an older source cannot trust the
	// heartbeat associated with the original journal.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tmp := path + ".external"
	if err := os.WriteFile(tmp, append(raw, ' '), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
	observed, err := readOperationJournal(project)
	if err != nil {
		t.Fatal(err)
	}
	if observed.LastScanAt != first.Format(time.RFC3339Nano) {
		t.Fatalf("heartbeat applied to external journal: %q", observed.LastScanAt)
	}
	if err := os.WriteFile(operationHeartbeatPath(project), []byte("invalid json"), 0644); err != nil {
		t.Fatal(err)
	}
	reloaded, err := readOperationJournal(project)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.LastScanAt != observed.LastScanAt {
		t.Fatal("corrupt heartbeat affected operation evidence")
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
}
