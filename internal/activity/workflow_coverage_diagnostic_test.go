package activity

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func a100at(second int) time.Time { return time.Date(2026, 11, 24, 0, 0, second, 0, time.UTC) }
func TestA100SeparateParticipationStateAndActivity(t *testing.T) {
	start, end := a100at(0), a100at(60)
	elapsed := int64(60000)
	vEnd := a100at(30)
	w := Workflow{TaskID: "A-100-fixture", Axis: WorkflowAxis{Start: &start, End: &end, ElapsedMillis: &elapsed}, Measurement: Measurement{ObservedWallMillis: 10000}, Rows: []WorkflowRow{{ID: "actor", Role: "worker", AgentID: "native", Intervals: []WorkflowInterval{{ID: "phase", Kind: "activity", Start: a100at(20), End: &vEnd, Measured: true}}, Gaps: []WorkflowGap{{Start: a100at(30), End: a100at(40), Status: "native_observed", Evidence: []WorkflowGapEvidence{{AttemptID: "run", Source: "t3"}}}}}}, Unmeasured: []WorkflowGap{{Start: start, End: a100at(20), Status: "native_observed"}, {Start: a100at(30), End: end, Status: "native_observed"}}}
	workflowDiagnoseCoverage(&w)
	if w.Coverage.StateCoveredMillis != 60000 || w.Coverage.UnaccountedStateMillis != 0 || w.Coverage.ClosedActivityWallMillis != 10000 || w.Coverage.InstrumentationStatus != "not_established" {
		t.Fatalf("dimensions collapsed: %+v", w.Coverage)
	}
	if len(w.Coverage.Findings) != 1 || w.Coverage.Findings[0].TaskID != w.TaskID || w.Coverage.Findings[0].ActorID != "native" || w.Coverage.Findings[0].Cause != "unknown" || w.Coverage.Findings[0].AttemptID != "run" || !w.Coverage.Findings[0].LastSuccessfulBoundary.Equal(vEnd) {
		t.Fatal(w.Coverage)
	}
}
func TestA100CompletionClipsWithoutErasingSource(t *testing.T) {
	start, end := a100at(0), a100at(60)
	s := Segment{TaskID: "T", Role: RoleController, StartedAt: a100at(50), EndedAt: a100at(70), DurationMillis: 20000}
	in := WorkflowInput{Summary: Summary{AgentTimeMillis: 20000, Segments: []Segment{s}}}
	m := Measurement{}
	workflowClipMeasurement(&m, in, nil, WorkflowAxis{Start: &start, End: &end})
	if m.AgentTimeMillis != 10000 || m.RawAgentTimeMillis != 20000 || m.OutsideTaskMillis != 10000 || *m.Controller.Millis != 10000 || in.Summary.Segments[0].EndedAt != a100at(70) {
		t.Fatal(m, in)
	}
}
func TestA100HistoricalFixtureBoundariesRemainExact(t *testing.T) {
	raw, err := os.ReadFile("../../tests/fixtures/workflow/b653-a96-regression-a100.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Samples []struct {
			SHA256   string   `json:"sha256"`
			Workflow Workflow `json:"workflow"`
			Summary  Summary  `json:"summary"`
		}
		A96Gaps []WorkflowGap `json:"a96_gaps"`
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Samples) != 3 || len(f.A96Gaps) != 8 {
		t.Fatal("missing historical samples/gaps")
	}
	for _, s := range f.Samples {
		if len(s.SHA256) != 64 || len(s.Workflow.Rows) == 0 {
			t.Fatal("missing immutable source proof")
		}
		before := s.Workflow.Measurement.ObservedWallMillis
		workflowDiagnoseCoverage(&s.Workflow)
		if s.Workflow.Measurement.ObservedWallMillis != before || s.Workflow.Coverage.InstrumentationStatus != "not_established" {
			t.Fatal("historical relabel")
		}
	}
	a96 := f.Samples[2]
	m := a96.Workflow.Measurement
	workflowClipMeasurement(&m, WorkflowInput{Summary: a96.Summary}, nil, a96.Workflow.Axis)
	if m.RawAgentTimeMillis != 2175786 || m.OutsideTaskMillis != 67124 || m.AgentTimeMillis != 2108662 {
		t.Fatalf("A96 after-task Activity lost or counted on task axis: %+v", m)
	}
	gap := f.A96Gaps[7]
	if gap.End.Sub(gap.Start) != 491949*time.Microsecond {
		t.Fatal("short historical gap erased", gap)
	}
}
