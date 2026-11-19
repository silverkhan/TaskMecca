package activity

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestExecutionCoverageBatchAllocationAndConcurrentWait(t *testing.T) {
	now := time.Date(2026, 11, 19, 7, 0, 0, 0, time.UTC)
	b := func(key, kind string, sec int) runtimeobs.OperationBoundary {
		return runtimeobs.OperationBoundary{ID: key, Key: key, Kind: kind, OperationID: "batch", Provider: "codex", NativeID: "registrar", AttemptID: "round-1", Role: "registrar", ObservedAt: now.Add(time.Duration(sec) * time.Second), Evidence: "fixture"}
	}
	start := b("s", "start", 0)
	link := b("link", "task_link", 2)
	link.TaskIDs = []string{"A-1", "A-2"}
	wait := b("w", "wait", 3)
	wait.Wait = &runtimeobs.BoundaryWait{Scope: "task_next_stage", Reason: "review", ExpectedEvent: "report", NextOwner: "reviewer", LastEvidence: "request", ResumeCondition: "accept"}
	other := b("w", "wait", 4)
	other.NativeID = "worker"
	other.AttemptID = "round-2"
	other.TaskIDs = []string{"A-1", "A-2"}
	other.Wait = &runtimeobs.BoundaryWait{Scope: "actor"}
	resume := b("r", "resume", 5)
	resume.RelatedID = "w"
	end := b("e", "end", 10)
	rows := []runtimeobs.OperationBoundary{start, link, wait, other, resume, end}
	a := BuildExecutionCoverage("A-1", rows, Workflow{}, nil)
	b2 := BuildExecutionCoverage("A-2", rows, Workflow{}, nil)
	if len(a.Items) != 6 || len(b2.Items) != 6 {
		t.Fatalf("late links lost: %d %d", len(a.Items), len(b2.Items))
	}
	if a.Items[0].Category != "closed_operation" || *a.Items[0].DurationMillis != 10000 || len(a.Items[0].TaskIDs) != 2 {
		t.Fatalf("shared duration %+v", a.Items[0])
	}
	if a.Items[2].Scope != "task_next_stage" || *a.Items[2].DurationMillis != 2000 {
		t.Fatal("wrong wait scope/duration")
	}
	// A separate native/round does not inherit the batch association or resume.
	if a.Items[3].Category == "evidenced_wait" {
		t.Fatal("unknown wait made measured")
	}
}

func TestExecutionCoverageActualHandoffDeliveryAndFailureClassification(t *testing.T) {
	now := time.Now().UTC()
	earlier := now.Add(-time.Second)
	w := SummarizeWorkflow(WorkflowInput{TaskID: "A-1", Now: now, Summary: Summarize("A-1", nil), Handoffs: map[string]handoffjournal.Handoff{"h": {HandoffID: "h", TaskID: "A-1", SourceAgentPath: "/root/registrar", SourceAttemptID: "registrar-attempt", PreparedAt: earlier.Format(time.RFC3339Nano), DeliveryStatus: "dispatch_ok", Steps: map[string]handoffjournal.StepState{"dispatch": {Result: "ok", ObservedAt: now.Format(time.RFC3339Nano), Evidence: "actual supported delivery"}}}}, CollectionFailures: []WorkflowCollectionFailure{{ID: "failed", TaskID: "A-1", Role: "worker", AgentID: "worker-native", At: now, Source: "actual_failure", Reason: "original collector error"}}})
	d := BuildExecutionCoverage("A-1", nil, w, nil)
	delivery, failure := false, false
	for _, item := range d.Items {
		if item.HandoffID == "h" && item.Reason == "actual supported delivery" {
			delivery = true
			if item.NativeID != "" || item.ActorID != "/root/registrar" || !item.Start.Equal(now) {
				t.Fatal("handoff actor path became native identity or timestamp moved")
			}
		}
		if item.Category == "collection_failure" && item.Reason == "original collector error" {
			failure = true
		}
	}
	if !delivery || !failure {
		t.Fatalf("delivery=%v failure=%v %+v", delivery, failure, d.Items)
	}
}

func TestExecutionCoverageDetailFixtureAndCacheIsolation(t *testing.T) {
	p := t.TempDir()
	now := time.Date(2026, 11, 19, 7, 0, 0, 0, time.UTC)
	base := runtimeobs.OperationBoundary{RequestID: "fixture-batch", OperationID: "fixture-op", Provider: "codex", NativeID: "fixture-registrar", AttemptID: "fixture-round-1", Role: "registrar", Source: "isolated_fixture", Evidence: "synthetic fixture; not actual maintenance"}
	appendAt := func(key, kind string, second int, ids []string) {
		b := base
		b.Key = key
		b.Kind = kind
		b.TaskIDs = ids
		if _, _, err := runtimeobs.AppendOperationBoundary(p, b, now.Add(time.Duration(second)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	appendAt("start", "start", 0, nil)
	appendAt("validation", "phase", 1, nil)
	appendAt("apply", "phase", 3, nil)
	appendAt("tasks", "task_link", 5, []string{"A-1", "A-2"})
	w := Workflow{TaskID: "A-1", Reporting: ReportingCoverage{Roles: []RoleReportingCoverage{{Role: "registrar", Status: "not_recorded"}}}}
	one := AttachExecutionCoverage(p, "A-1", w)
	if w.Reporting.Roles[0].ExecutionCoverage != nil {
		t.Fatal("shared cache mutated")
	}
	appendAt("end", "end", 8, nil)
	two := AttachExecutionCoverage(p, "A-1", w)
	if len(one.Reporting.Roles[0].ExecutionCoverage.Items) == len(two.Reporting.Roles[0].ExecutionCoverage.Items) {
		t.Fatal("persisted boundary change hidden by workflow cache")
	}
	d := two.Reporting.Roles[0].ExecutionCoverage
	if d.Items[1].Category != "closed_phase" || d.Items[2].Category != "closed_phase" {
		t.Fatalf("phases %+v", d.Items)
	}
	if target := os.Getenv("A93_DETAIL_FIXTURE"); target != "" {
		raw, err := json.MarshalIndent(two, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(target, append(raw, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExecutionCoverageReadFailureOriginalAxisAndOpen(t *testing.T) {
	now := time.Now().UTC()
	w := Workflow{Axis: WorkflowAxis{}, Rows: []WorkflowRow{{Role: "worker", AgentID: "native", Intervals: []WorkflowInterval{{ID: "open", Kind: "activity", Start: now}}}}}
	before, _ := json.Marshal(w.Axis)
	detail := BuildExecutionCoverage("A-1", nil, w, errors.New("original unreadable evidence"))
	after, _ := json.Marshal(w.Axis)
	if !reflect.DeepEqual(before, after) || len(detail.CollectionErrors) != 1 || detail.Items[0].Category != "open_start" || detail.Items[0].DurationMillis != nil {
		t.Fatalf("%+v", detail)
	}
}

func TestExecutionCoverageUnknownWaitAndNoBackfill(t *testing.T) {
	now := time.Now().UTC()
	rows := []runtimeobs.OperationBoundary{{ID: "end", Key: "end", Kind: "end", OperationID: "op", NativeID: "native", AttemptID: "a", Role: "registrar", TaskIDs: []string{"A-1"}, ObservedAt: now}, {ID: "wait", Key: "wait", Kind: "wait", OperationID: "op", NativeID: "native", AttemptID: "a", Role: "registrar", ObservedAt: now, Wait: &runtimeobs.BoundaryWait{Scope: "actor"}}}
	d := BuildExecutionCoverage("A-1", rows, Workflow{}, nil)
	if d.Items[0].Category != "collection_failure" || d.Items[1].Category != "unknown" || d.Items[1].DurationMillis != nil {
		t.Fatalf("%+v", d)
	}
}
