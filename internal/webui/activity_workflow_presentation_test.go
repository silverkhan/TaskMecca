package webui

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/activity"
	"reflect"
	"testing"
	"time"
)

func TestA91WorkflowPresentationPreservesMeasurementsAndCache(t *testing.T) {
	project := t.TempDir()
	facts := []activity.LifecycleFact{{EventID: "fixture", TaskID: "A-1", OccurredAt: time.Now().UTC().Add(-time.Hour)}}
	w := readActivityWorkflow(project, "A-1", nil, activity.Summarize("A-1", nil), facts)
	before := w.Snapshot
	again := readActivityWorkflow(project, "A-1", nil, activity.Summarize("A-1", nil), facts)
	if !w.ComputedAt.Equal(again.ComputedAt) || !reflect.DeepEqual(w.Axis, again.Axis) || !reflect.DeepEqual(w.Measurement, again.Measurement) {
		t.Fatal("presentation changed cache or measurement")
	}
	if before["reviewer_presentation"] == nil || again.Snapshot["reviewer_presentation"] == nil {
		t.Fatal("cachehit omitted presentation")
	}
	again.Snapshot["probe"] = "new"
	if before["probe"] != nil {
		t.Fatal("mutated cached snapshot")
	}
	encoded, _ := json.Marshal(again.Snapshot["reviewer_presentation"])
	var p activity.ReviewerPresentation
	if json.Unmarshal(encoded, &p) != nil || p.Version != 1 || p.Task.ActualState != "unknown" {
		t.Fatal(string(encoded))
	}
}
