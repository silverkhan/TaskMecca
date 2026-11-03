package webui

import (
    "testing"
    "time"
)

func TestResolvedSourceIndexMatchesOriginalPredicates(t *testing.T) {
    now := time.Date(2026,11,3,2,0,0,0,time.UTC)
    historic := []operationIncident{
        {Project:"p",TaskID:"B-1",AttemptID:"a1",AssignmentID:"as1",AgentPath:"worker",Kind:"runtime_unknown",Evidence:"identity",LastObservedAt:"2026-11-03T01:00:00Z",RecoveredAt:"2026-11-03T01:10:00Z",RecoveryEvidence:"same attempt has later observed Hook running state; previous warning retired without changing runtime evidence"},
        {Project:"p",TaskID:"B-1",AttemptID:"a1",AssignmentID:"as1",AgentPath:"worker",Kind:"runtime_unknown",Evidence:"identity",LastObservedAt:"2026-11-03T01:00:00Z",RecoveredAt:"2026-11-03T01:11:00Z",RecoveryEvidence:"latest assignment has observed execution evidence; prior runtime evidence unchanged"},
        {Project:"p",TaskID:"B-1",AttemptID:"a2",AssignmentID:"as1",AgentPath:"worker",Kind:"runtime_unknown",Evidence:"identity",LastObservedAt:"2026-11-03T01:00:00Z",RecoveredAt:"2026-11-03T01:11:00Z",RecoveryEvidence:"same assignment has recent observed Hook execution; prior runtime evidence unchanged"},
        {Project:"p",TaskID:"B-1",AttemptID:"a1",AssignmentID:"as1",AgentPath:"worker",Kind:"runtime_unknown",Evidence:"identity",LastObservedAt:"2026-11-03T01:00:00Z"},
        {Project:"p",TaskID:"B-2",Kind:"handoff_failed",RecoveredAt:"2026-11-03T01:00:00Z"},
    }
    journal := operationJournal{Incidents: historic}
    index := indexResolvedSources(journal)
    cases := []operationIncident{
        historic[0],
        historic[1],
        historic[2],
        {Project:"p",TaskID:"B-1",AttemptID:"a1",AssignmentID:"as1",AgentPath:"worker",Kind:"runtime_unknown",Evidence:"identity",LastObservedAt:"2026-11-03T01:10:00Z"},
        {Project:"p",TaskID:"B-1",AttemptID:"a1",AssignmentID:"as1",AgentPath:"worker",Kind:"runtime_unknown",Evidence:"new identity",LastObservedAt:"2026-11-03T01:00:00Z"},
        {Project:"p",TaskID:"B-1",AttemptID:"a1",AssignmentID:"as1",AgentPath:"worker",Kind:"no_signal",Evidence:"identity",LastObservedAt:"2026-11-03T01:00:00Z"},
        {Project:"different",TaskID:"B-1",AttemptID:"a1",AssignmentID:"as1",AgentPath:"worker",Kind:"runtime_unknown",Evidence:"identity",LastObservedAt:"2026-11-03T01:00:00Z"},
        {Project:"p",TaskID:"B-2",Kind:"handoff_failed",Evidence:"failure"},
    }
    for i, item := range cases {
        expected := operationSourceAlreadyResolved(item,journal,now)
        actual := index.alreadyResolved(item,now)
        if expected != actual { t.Errorf("case %d source match %v want %v",i,actual,expected) }
        expectedLatest := operationLatestAssignmentSourceAlreadyResolved(item,journal)
        actualLatest := index.latestAssignmentResolved(item)
        if expectedLatest != actualLatest { t.Errorf("case %d latest match %v want %v",i,actualLatest,expectedLatest) }
    }
}

func BenchmarkResolvedSourceIndex(b *testing.B) {
    now := time.Date(2026,11,3,2,0,0,0,time.UTC)
    history := make([]operationIncident,5000)
    for i := range history {
        history[i]=operationIncident{Project:"p",TaskID:"B-1",AttemptID:"old",Kind:"runtime_unknown",RecoveredAt:"2026-11-03T01:00:00Z",RecoveryEvidence:"same assignment has recent observed Hook execution; prior runtime evidence unchanged"}
    }
    journal := operationJournal{Incidents:history}
    item := operationIncident{Project:"p",TaskID:"B-2",AttemptID:"new",Kind:"runtime_unknown"}
    b.Run("legacy",func(b *testing.B){for i:=0;i<b.N;i++{operationSourceAlreadyResolved(item,journal,now)}})
    index := indexResolvedSources(journal)
    b.Run("indexed",func(b *testing.B){for i:=0;i<b.N;i++{index.alreadyResolved(item,now)}})
}
