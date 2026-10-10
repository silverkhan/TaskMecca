package backlog

import "testing"

func TestControlTowerUserHoldPreservesDecision(t *testing.T) {
    row:=Record{ID:"B-710",State:"hold",Location:"active"}
    review:=map[string]any{"wait_kind":"user","wait_note":"hold asks user"}
    signal:=map[string]any{"source":"execution_ledger","attempt_id":"run-710","runtime_state":"completed","health":"awaiting_finalize"}
    reason,condition:=canonicalOperationalState(row,review,signal)
    if toString(reason["type"])!="user_intervention" { t.Fatalf("reason=%+v",reason) }
    if toString(condition["kind"])!="intervention" { t.Fatalf("condition=%+v",condition) }
}

func TestControlTowerApprovalIsFirstClassState(t *testing.T) {
    row:=Record{ID:"B-711",State:"doing",Location:"active"}
    reason,condition:=canonicalOperationalState(row,nil,map[string]any{
        "source":"execution_ledger","attempt_id":"run-711","runtime_state":"waiting_approval","health":"active",
    })
    if toString(reason["type"])!="approval_required" { t.Fatalf("reason=%+v",reason) }
    if toString(condition["kind"])!="approval" { t.Fatalf("condition=%+v",condition) }
    if effectiveStateFromControl(row.State,reason)!="needs_user" { t.Fatalf("effective state mismatch: %+v",reason) }
}

func TestControlTowerConditionKeySeparatesExecutionEpisodes(t *testing.T) {
    row:=Record{ID:"B-712",State:"doing",Location:"active"}
    _,first:=canonicalOperationalState(row,nil,map[string]any{"attempt_id":"run-a","health":"stale","runtime_state":"running"})
    _,second:=canonicalOperationalState(row,nil,map[string]any{"attempt_id":"run-b","health":"stale","runtime_state":"running"})
    if toString(first["key"])==toString(second["key"]) { t.Fatalf("episode keys collided: %+v %+v",first,second) }
}


func TestEffectiveStateProjectionCoversEveryCanonicalAttentionType(t *testing.T) {
    cases:=[]struct{reason,want string}{
        {"completion_pending","controller_recovery"},
        {"user_intervention","needs_user"},
        {"approval_required","needs_user"},
        {"runtime_stalled","stalled"},
        {"execution_interrupted","stalled"},
        {"runtime_unknown","stalled"},
    }
    for _,tc:=range cases {
        got:=effectiveStateFromControl("doing",map[string]any{"type":tc.reason})
        if got!=tc.want { t.Fatalf("%s => %s want %s",tc.reason,got,tc.want) }
    }
}
