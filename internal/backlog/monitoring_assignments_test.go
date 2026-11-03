package backlog

import "testing"

func TestMonitoringAssignmentsPreservesWorkloadSelection(t *testing.T) {
    rows := []Record{
        {ID:"B-1",Location:"done",State:"done",Fields:map[string]string{"Agent":"/root/old"}},
        {ID:"B-1",Location:"active",State:"doing",Fields:map[string]string{"Agent":" /root/new "}},
        {ID:"B-2",Location:"active",State:"doing",Fields:map[string]string{"Agent":"-"}},
        {ID:"B-3",Location:"active",State:"hold",Fields:map[string]string{"Agent":"/root/waiting"}},
        {ID:"B-4",Location:"active",State:"doing",Fields:map[string]string{"Agent":"/root/worker"}},
        {ID:"B-5",Location:"active",State:"doing",Fields:map[string]string{"Agent":"  "}},
    }
    actual := monitoringAssignmentsFromRows(rows)
    if len(actual)!=2 {t.Fatalf("got %d active assigned rows, want 2: %#v",len(actual),actual)}
    if actual["B-1"]!="/root/new" || actual["B-4"]!="/root/worker" {
        t.Fatalf("incorrect active preferred assignments: %#v",actual)
    }
}
