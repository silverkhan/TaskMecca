package backlog

import (
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// controlTowerSnapshot is the single reconciliation result for user-facing task
// state. Sensors may be plural, but interpretation is centralized here.
// Consumers (Web, attention, notification delivery) must project this result;
// they must not independently reinterpret runtime or backlog evidence.
type controlTowerSnapshot struct {
    Timings map[string]map[string]any
    Activity map[string]map[string]any
    ReviewByPath map[string]map[string]any
    Attention map[string]map[string]any
    NotificationCondition map[string]map[string]any
}

func reconcileControlTower(project,root string,rows []Record) controlTowerSnapshot {
    // Binding reconciliation belongs to the control plane, not to a view.
    // This is the only compatibility fallback for a missed explicit bind-agent:
    // exactly one live unbound attempt matching the canonical backlog assignment.
    recoveredBindings:=reconcileCanonicalBindings(project,rows,time.Now())
    timings,err:=lifecycleTimings(project,root,rows)
    if err!=nil { timings=map[string]map[string]any{} }
    markRecoveryBindingStarts(project,timings)
    for id:=range recoveredBindings {
        if lifecycle:=timings[id]; lifecycle!=nil {
            lifecycle["started_notification_suppressed"]=true
        }
    }

    hold:=HoldReview(rows)
    reviewByPath:=map[string]map[string]any{}
    for _,key:=range []string{"candidates","waiting"} {
        if values,ok:=hold[key].([]map[string]any); ok {
            for _,x:=range values { reviewByPath[toString(x["path"])]=x }
        }
    }

    activity:=runtimeActivity(project,rows,timings)
    activity=mergeRuntimeSignals(activity,runtimeLedgerSignals(project,rows,time.Now()))

    attention:=map[string]map[string]any{}
    conditions:=map[string]map[string]any{}
    for _,row:=range rows {
        if row.Location!="active" { continue }
        signal:=activity[row.ID]
        reason,condition:=canonicalOperationalState(row,reviewByPath[row.Path],signal)
        if len(reason)>0 { attention[row.ID]=reason }
        if len(condition)>0 { conditions[row.ID]=condition }
    }
    return controlTowerSnapshot{
        Timings:timings,Activity:activity,ReviewByPath:reviewByPath,
        Attention:attention,NotificationCondition:conditions,
    }
}

func canonicalOperationalState(row Record,review,signal map[string]any) (map[string]any,map[string]any) {
    reason:=map[string]any{}
    condition:=map[string]any{}

    // Backlog hold is a sensor input. It is interpreted here exactly once.
    if row.State=="hold" && review!=nil && toString(review["wait_kind"])=="user" {
        reason=map[string]any{
            "type":"user_intervention","severity":"danger","title":"사용자 개입 필요",
            "message":firstNonEmpty(toString(review["wait_note"]),"사용자 입력 또는 판단을 기다리고 있습니다."),
            "resume_condition":toString(review["resume_condition"]),"evidence":toString(review["wait_evidence"]),
        }
        condition=notificationCondition("intervention","hold:user",reason,signal)
    }

    if signal==nil { return reason,condition }
    health:=strings.ToLower(strings.TrimSpace(toString(signal["health"])))
    runtimeState:=strings.ToLower(strings.TrimSpace(toString(signal["runtime_state"])))
    switch {
    case runtimeState=="waiting_approval":
        reason=map[string]any{"type":"approval_required","severity":"danger","title":"승인 필요","message":"Worker가 승인을 기다리고 있습니다.","resume_condition":"필요한 승인을 처리한 뒤 작업을 재개하세요."}
        condition=notificationCondition("approval","runtime:approval",reason,signal)
    case health=="awaiting_finalize":
        reason=map[string]any{"type":"completion_pending","severity":"warning","title":"완료 처리 필요","message":"워커 런타임은 작업 완료를 보고했지만 백로그는 아직 doing 상태입니다.","resume_condition":"결과와 검증을 확인한 뒤 태스크를 done으로 완료 처리하세요."}
        condition=notificationCondition("finalize","runtime:finalize",reason,signal)
    case health=="needs_user":
        reason=map[string]any{"type":"user_intervention","severity":"danger","title":"사용자 개입 필요","message":"워커 런타임이 사용자 입력 또는 조치를 기다리고 있습니다.","resume_condition":"필요한 사용자 판단 또는 입력을 제공한 뒤 작업을 재개하세요."}
        condition=notificationCondition("intervention","runtime:user",reason,signal)
    case health=="stale" || health=="worker_missing":
        reason=map[string]any{"type":"runtime_stalled","severity":"warning","title":"작업 정체 확인 필요","message":"진행 중 태스크의 런타임 활동이 중단되었거나 할당 워커를 찾을 수 없습니다.","resume_condition":"워커 상태와 남은 작업을 확인하고 재할당 또는 완료 처리 여부를 결정하세요."}
        condition=notificationCondition("stalled","runtime:stalled",reason,signal)
    case health=="execution_interrupted":
        reason=map[string]any{"type":"execution_interrupted","severity":"danger","title":"실행 복구 필요","message":"백로그는 doing 상태이지만 연결된 실행 attempt가 정상 완료되지 않고 종료되었습니다.","resume_condition":"실행 결과와 남은 작업을 확인한 뒤 재할당 또는 상태 정리를 수행하세요."}
        condition=notificationCondition("interrupted","runtime:interrupted",reason,signal)
    case health=="runtime_unknown":
        reason=map[string]any{"type":"runtime_unknown","severity":"warning","title":"실행 상태 확인 필요","message":"백로그에 연결된 실행 attempt의 현재 상태를 신뢰성 있게 확인할 수 없습니다.","resume_condition":"런타임과 hook 상태를 확인하고 필요하면 작업을 재개하세요."}
        condition=notificationCondition("runtime_unknown","runtime:unknown",reason,signal)
    }
    return reason,condition
}

func notificationCondition(kind,key string,reason,signal map[string]any) map[string]any {
    out:=map[string]any{
        "kind":kind,"key":key,
        "message":toString(reason["message"]),
        "resume_condition":toString(reason["resume_condition"]),
        "reason_type":toString(reason["type"]),
    }
    if signal!=nil {
        out["attempt_id"]=toString(signal["attempt_id"])
        out["runtime_state"]=toString(signal["runtime_state"])
        // Episode identity is part of the key so a later execution attempt may
        // legitimately raise the same condition again.
        if attempt:=toString(signal["attempt_id"]); attempt!="" { out["key"]=key+"\x00"+attempt }
    }
    return out
}

func effectiveStateFromControl(fileState string,reason map[string]any) string {
    switch toString(reason["type"]) {
    case "completion_pending": return "awaiting_finalize"
    case "user_intervention","approval_required": return "needs_user"
    case "runtime_stalled","execution_interrupted","runtime_unknown": return "stalled"
    default: return fileState
    }
}


func reconcileCanonicalBindings(project string,rows []Record,now time.Time) map[string]bool {
    recovered:=map[string]bool{}
    ledger,err:=runtimeobs.ReconcileLedger(project,10,now)
    if err!=nil { return recovered }
    byID:=preferredRows(rows)
    for id,row:=range byID {
        if row.Location!="active" || row.State!="doing" { continue }
        agent:=strings.TrimSpace(row.Fields["Agent"])
        if agent=="" { continue }
        workerName:=agent
        if slash:=strings.LastIndex(workerName,"/"); slash>=0 { workerName=workerName[slash+1:] }
        candidates:=[]runtimeobs.Attempt{}
        for _,attempt:=range ledger.Attempts {
            if attempt.Terminal || attempt.BindingState==runtimeobs.BindingBound { continue }
            runtimeID:=strings.TrimSpace(attempt.RuntimeAgentID)
            if runtimeID!="" && (runtimeID==workerName || runtimeID==agent) { candidates=append(candidates,attempt) }
        }
        if len(candidates)!=1 { continue }
        evidence:=map[string]string{"agent_path":agent,"correlation":"canonical_backlog_assignment"}
        if _,err:=runtimeobs.BindAttempt(project,candidates[0].AttemptID,id,agent,"backlog_assignment","",evidence,now); err==nil {
            recovered[id]=true
        }
    }
    return recovered
}


func markRecoveryBindingStarts(project string,timings map[string]map[string]any) {
    ledger,err:=runtimeobs.ReconcileLedger(project,10,time.Now())
    if err!=nil { return }
    for _,attempt:=range ledger.Attempts {
        if attempt.Terminal || attempt.BindingState!=runtimeobs.BindingBound { continue }
        if !strings.EqualFold(strings.TrimSpace(attempt.BindingSource),"backlog_assignment") { continue }
        if !strings.EqualFold(strings.TrimSpace(attempt.BindingEvidence["correlation"]),"canonical_backlog_assignment") { continue }
        id:=strings.ToUpper(strings.TrimSpace(attempt.TaskID))
        if id=="" { continue }
        lifecycle:=timings[id]
        if lifecycle==nil || toString(lifecycle["started_at"])=="" { continue }
        // Recovery binding repairs canonical truth for an already-existing
        // execution. It is not a live user-facing transition.
        lifecycle["started_notification_suppressed"]=true
    }
}
