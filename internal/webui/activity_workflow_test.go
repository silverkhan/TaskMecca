package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/backlog"
)

func TestActivityWorkflowHTTPPreservesFullAxisMissingMetricsAndReadOnlyProjection(t *testing.T) {
	project := t.TempDir()
	registerWebFixture(t, project)
	start := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	for i, kind := range []string{"registered", "started", "completed"} {
		_, err := backlog.RecordLifecycleTransition(project, backlog.LifecycleTransition{EventID: kind, TaskID: "B-1", Kind: kind, Actor: "/root/controller/pairi", EvidenceSource: "worker_report", OccurredAt: start.Add(time.Duration(i) * 30 * time.Minute).Format(time.RFC3339Nano)}, start)
		if err != nil {
			t.Fatal(err)
		}
	}
	handler, err := Handler(project, "", "test")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/activity?"+url.Values{"project": {project}, "task": {"B-1"}}.Encode(), nil))
	if response.Code != 200 {
		t.Fatalf("%d %s", response.Code, response.Body.String())
	}
	var payload struct {
		Workflow activity.Workflow `json:"workflow"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	w := payload.Workflow
	if w.Axis.ElapsedMillis == nil || *w.Axis.ElapsedMillis != 60*60000 || w.Measurement.Tests.Millis != nil || w.Measurement.Controller.Millis != nil || w.Reporting.ExecutionsWithoutActivity != 1 {
		t.Fatalf("bad workflow: %+v", w)
	}
	if _, err = os.Stat(filepath.Join(project, "_task_mecca", ".runtime", "execution", "events.jsonl")); !os.IsNotExist(err) {
		t.Fatal("read unexpectedly created runtime evidence")
	}
	snapshot, available := backlog.ReadLifecycleProjection(project, "", "B-1")
	if available {
		t.Fatalf("Activity read reconciled Control Tower: %v", snapshot)
	}
}

func TestActivityWorkflowCachesUnchangedSourcesWithoutRuntimeReconcile(t *testing.T) {
	project := t.TempDir()
	now := time.Now().UTC()
	facts := []activity.LifecycleFact{{EventID: "r", TaskID: "B-1", Kind: "registered", OccurredAt: now.Add(-time.Minute)}}
	first := readActivityWorkflow(project, "B-1", nil, activity.Summarize("B-1", nil), facts)
	second := readActivityWorkflow(project, "B-1", nil, activity.Summarize("B-1", nil), facts)
	if !first.ComputedAt.Equal(second.ComputedAt) {
		t.Fatal("unchanged workflow recomputed on every read")
	}
	rows := []activity.Event{{EventID: "new", TaskID: "B-1", Action: activity.Start, Role: activity.RoleWorker, AgentID: "w", RuntimeAttemptID: "a", Type: activity.Implementation, OccurredAt: now}}
	changed := readActivityWorkflow(project, "B-1", rows, activity.Summarize("B-1", rows), facts)
	if changed.Measurement.Activity.EventCount != 1 {
		t.Fatal("changed source hidden by cache")
	}
}
