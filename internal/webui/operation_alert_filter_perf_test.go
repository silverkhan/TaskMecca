package webui

import (
    "strings"
    "testing"

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
        if controllerWork || incident.Kind == "normal_exit" || incident.RecoveredAt != "" { continue }
        events = append(events, notify.Event{
            ID: incident.ID, TaskID: incident.TaskID, Kind: operationNotificationKind(incident),
            Title: strings.ReplaceAll(incident.Kind, "_", " "), Message: incident.Evidence,
            ResumeCondition: incident.Action, At: incident.DetectedAt,
        })
    }
    return events
}

func TestOperationActionableIncidentsMatchesLegacy(t *testing.T) {
    // Mixed attempt binding and casing; recovered/normal incidents must never
    // create a live notification, nor should a review mask a different attempt.
    journal := operationJournal{
        Stages: []operationStage{
            {TaskID:"B-100", AttemptID:"a-1", ControllerReview:true},
            {TaskID:"B-200", AttemptID:"", ControllerReview:true},
            {TaskID:"B-300", AttemptID:"a-4", ControllerReview:false},
        },
        Incidents: []operationIncident{
            {ID:"1",TaskID:"b-100",AttemptID:"a-1",Kind:"runtime_unknown"},
            {ID:"2",TaskID:"B-100",AttemptID:"a-2",Kind:"runtime_unknown"},
            {ID:"3",TaskID:"B-100",Kind:"runtime_unknown"},
            {ID:"4",TaskID:"B-200",Kind:"runtime_unknown"},
            {ID:"5",TaskID:"B-200",AttemptID:"a-9",Kind:"handoff_stalled"},
            {ID:"6",TaskID:"B-300",AttemptID:"a-4",Kind:"handoff_failed"},
            {ID:"7",TaskID:"B-400",Kind:"normal_exit"},
            {ID:"8",TaskID:"B-500",Kind:"handoff_failed",RecoveredAt:"2026-11-03T00:00:00Z"},
        },
    }
    expected := legacyOperationActionable(journal)
    actual := operationActionableIncidents(journal)
    if len(expected)!=len(actual) { t.Fatalf("expected %d events, got %d",len(expected),len(actual)) }
    for i := range expected {
        if expected[i] != actual[i] { t.Fatalf("event %d differs: %#v vs %#v",i,expected[i],actual[i]) }
    }
}

func BenchmarkOperationActionableIncidents(b *testing.B) {
    stages := []operationStage{{TaskID:"B-900",AttemptID:"active",ControllerReview:true}}
    history := make([]operationIncident, 5500)
    for i := range history {
        history[i] = operationIncident{TaskID:"B-900",AttemptID:"old",Kind:"runtime_unknown",RecoveredAt:"2026-11-03T00:00:00Z"}
    }
    history = append(history, operationIncident{TaskID:"B-900",AttemptID:"active",Kind:"runtime_unknown"})
    journal := operationJournal{Stages:stages,Incidents:history}
    b.Run("indexed",func(b *testing.B){for i:=0;i<b.N;i++ { operationActionableIncidents(journal) }})
    b.Run("legacy",func(b *testing.B){for i:=0;i<b.N;i++ { legacyOperationActionable(journal) }})
}
