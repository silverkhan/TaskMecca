package webui

import (
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/notify"
)

// Original matching semantics, kept as a regression oracle for P1 filtering.
func legacyOperationActionable(journal operationJournal) []notify.Event {
	events := []notify.Event{}
	for _, incident := range journal.Incidents {
		controllerWork := false
		for _, stage := range journal.Stages {
			if stage.ControllerReview && strings.EqualFold(stage.TaskID, incident.TaskID) && (incident.AttemptID == "" || stage.AttemptID == incident.AttemptID) {
				controllerWork = true
				break
			}
		}
		if controllerWork || incident.Kind == "normal_exit" || incident.RecoveredAt != "" {
			continue
		}
		events = append(events, notify.Event{
			ID: incident.ID, TaskID: incident.TaskID, Kind: operationNotificationKind(incident),
			Title: strings.ReplaceAll(incident.Kind, "_", " "), Message: incident.Evidence,
			ResumeCondition: incident.Action, At: incident.DetectedAt,
		})
	}
	return events
}

// Retain the three actionable-task regression, but use actual typed human
// causes as the oracle. Legacy diagnostic source incidents are not alerts.
func TestOperationActionableIncidentsMatchesLegacy(t *testing.T) {
	project := testOperationProject(t)
	installA83MockCurrentExecution(t)
	for _, tc := range []struct {
		task  string
		state runtimeobs.CanonicalState
	}{
		{"B-100", runtimeobs.StateRunning}, {"B-200", runtimeobs.StateWaitingApproval}, {"B-300", runtimeobs.StateErrored}, {"B-500", runtimeobs.StateCompleted},
	} {
		a83TypedActionFixture(t, project, tc.task, "Owner permission needed for "+tc.task, "Owner: approve access for "+tc.task, tc.state)
	}
	if err := os.WriteFile(filepath.Join(project, "_task_mecca/data/backlog/000001.B-400.legacy.doing.md"), []byte("# B-400 diagnostic\n- Agent: /root/controller/legacy\n"), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	journal, err := scanOperationProject(project, now)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic := operationIncident{ID: "old-unverified-source", TaskID: "B-400", Kind: "runtime_unknown", Quality: "verification_required"}
	journal.Incidents = append(journal.Incidents, diagnostic)
	journal.Actions = append(journal.Actions, operationAction{ID: "recovered-action", TaskID: "B-100", ResolvedAt: now.Format(time.RFC3339Nano)})
	ledger, err := runtimeobs.BuildLedger(project, 3, now)
	if err != nil {
		t.Fatal(err)
	}
	journal.ExecutionEvidence = operationExecutionLoader(project, ledger, now, true)
	actual := operationActionableIncidents(journal)
	expected := map[string]string{"B-100": "intervention", "B-200": "approval", "B-300": "interrupted"}
	if len(actual) != len(expected) {
		t.Fatalf("three real human actions required, got %+v; evidence=%+v", actual, journal.ExecutionEvidence)
	}
	for _, event := range actual {
		kind, ok := expected[event.TaskID]
		if !ok || event.Kind != kind || event.Message != "Owner permission needed for "+event.TaskID || event.ResumeCondition != "Owner: approve access for "+event.TaskID || event.ID == diagnostic.ID {
			t.Fatalf("wrong current action %+v", event)
		}
		delete(expected, event.TaskID)
	}
	if len(expected) != 0 || journal.Incidents[len(journal.Incidents)-1] != diagnostic {
		t.Fatal("task/source diagnostic lost", expected)
	}
}

func BenchmarkOperationActionableIncidents(b *testing.B) {
	stages := []operationStage{{TaskID: "B-900", AttemptID: "active", ControllerReview: true}}
	history := make([]operationIncident, 5500)
	for i := range history {
		history[i] = operationIncident{TaskID: "B-900", AttemptID: "old", Kind: "runtime_unknown", RecoveredAt: "2026-11-03T00:00:00Z"}
	}
	history = append(history, operationIncident{TaskID: "B-900", AttemptID: "active", Kind: "runtime_unknown"})
	journal := operationJournal{Stages: stages, Incidents: history}
	b.Run("indexed", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			operationActionableIncidents(journal)
		}
	})
	b.Run("legacy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			legacyOperationActionable(journal)
		}
	})
}
