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
    now:=time.Now()
    ledger,ledgerErr:=runtimeobs.ReconcileLedger(project,20,now)
    shared:=requestRuntime{ledger,ledgerErr}
    recoveredBindings:=reconcileCanonicalBindings(project,rows,now,shared)
    if len(recoveredBindings)>0 { ledger,ledgerErr=runtimeobs.ReconcileLedger(project,20,now); shared=requestRuntime{ledger,ledgerErr} }
    timings,err:=lifecycleTimings(project,root,rows,shared)
    if err!=nil { timings=map[string]map[string]any{} }
    for id:=range recoveredBindings {
        if lifecycle:=timings[id]; lifecycle!=nil && lifecycle["started_at"]!=nil && !recoveredBindingStartIsLive(project,id,now,shared) {
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
	activity=mergeRuntimeSignals(activity,runtimeLedgerSignals(project,rows,now,shared))
	applyCompletionReviews(project, rows, activity, now,shared)
	for id,signal:=range activity {
		health:=toString(signal["health"])
		if health!="assignment_unobserved" && health!="binding_pending" && health!="binding_ambiguous" && health!="awaiting_start" { continue }
		lifecycle:=timings[id]
		if lifecycle==nil { lifecycle=map[string]any{}; timings[id]=lifecycle }
		lifecycle["start_evidence_status"]=health
		lifecycle["assignment_id"]=signal["assignment_id"]
		lifecycle["attempt_id"]=signal["attempt_id"]
		lifecycle["candidate_attempt_ids"]=signal["candidate_attempt_ids"]
	}

    attention:=map[string]map[string]any{}
    conditions:=map[string]map[string]any{}
    for _,row:=range rows {
        if row.Location!="active" { continue }
        signal:=activity[row.ID]
        reason,condition:=canonicalOperationalState(row,reviewByPath[row.Path],signal,now)
        if len(reason)>0 { attention[row.ID]=reason }
        if len(condition)>0 { conditions[row.ID]=condition }
    }
    return controlTowerSnapshot{
        Timings:timings,Activity:activity,ReviewByPath:reviewByPath,
        Attention:attention,NotificationCondition:conditions,
    }
}

func canonicalOperationalState(row Record,review,signal map[string]any,observedAt ...time.Time) (map[string]any,map[string]any) {
    reason:=map[string]any{}
    condition:=map[string]any{}
	if row.Location == "archive" || (row.State != "doing" && row.State != "hold") {
		return reason, condition
	}

	// Backlog hold is a sensor input. It is interpreted here exactly once.
	waitKind := strings.TrimSpace(row.Fields["대기유형"])
	if waitKind == "" {
		waitKind = toString(review["wait_kind"])
	}
	if row.State=="hold" && waitKind =="user" {
        reason=map[string]any{
            "type":"user_intervention","severity":"danger","title":"사용자 개입 필요",
            "message":firstNonEmpty(row.Fields["대기"], toString(review["wait_note"]),"사용자 입력 또는 판단을 기다리고 있습니다."),
            "resume_condition": firstNonEmpty(row.Fields["재개조건"], toString(review["resume_condition"])),"evidence": firstNonEmpty(row.Fields["대기근거"], toString(review["wait_evidence"])),
        }
        condition=notificationCondition("intervention","hold:user",reason,signal)
    }

    if signal==nil || len(reason) > 0 { return reason,condition
	}
	if review, ok := signal["completion_review"].(map[string]any); ok {
		if review["state"] == "needs_user" {
			r := map[string]any{"type": "user_intervention", "severity": "danger", "message": review["diagnostic"], "resume_condition": "필요한 사용자 판단 또는 승인을 제공해 주세요."}
			return r, notificationCondition("intervention", "controller:user", r, signal)
		}
		return map[string]any{"type": "controller_completion_review", "audience": "controller", "severity": "info", "state": review["state"], "message": review["diagnostic"], "completion_review": review}, nil
	}
    health:=strings.ToLower(strings.TrimSpace(toString(signal["health"])))
    runtimeState:=strings.ToLower(strings.TrimSpace(toString(signal["runtime_state"])))
    switch {
	case health == "assignment_unobserved":
        now:=time.Now(); if len(observedAt)>0 { now=observedAt[0] }
        if assigned,err:=time.Parse(time.RFC3339Nano,toString(signal["assigned_at"])); err==nil && !assigned.After(now) && now.Before(assigned.Add(runtimeobs.AssignmentObservationGrace)) {
            return reason,condition
        }

		reason = map[string]any{"type": "assignment_unobserved", "severity": "warning", "title": "Worker 실행 대기",
			"message":          "배정 기록은 있으나 실행 attempt가 아직 관측되지 않았습니다.",
			"resume_condition": "배정된 Worker의 runtime identity와 첫 hook을 확인하세요.",
			"assignment_id":    toString(signal["assignment_id"]), "assigned_at": toString(signal["assigned_at"])}
	case health == "binding_pending" || health == "binding_ambiguous":
		reason = map[string]any{"type": health, "severity": "warning", "title": "실행 연결 확인 필요",
			"message":          "배정된 Worker의 실행 후보와 task 연결이 아직 확정되지 않았습니다.",
			"resume_condition": "배정 ID와 runtime identity로 후보를 확인하고 명시적으로 연결하세요. 후보가 복수라면 임의로 선택하지 마세요.",
			"assignment_id":    toString(signal["assignment_id"]), "candidate_attempt_ids": signal["candidate_attempt_ids"],
			"candidate_count": signal["candidate_count"]}
	case health == "awaiting_start":
		reason = map[string]any{
			"type": "awaiting_start", "severity": "warning", "title": "Worker 착수 확인 필요",
			"message":          "배정과 실행 attempt는 연결됐지만 Worker 실행 증거가 아직 없습니다.",
			"resume_condition": "해당 runtime agent의 hook 증거를 확인하거나, 실행이 시작되지 않았다면 재배정하세요.",
			"attempt_id":       toString(signal["attempt_id"]), "runtime_agent_id": toString(signal["runtime_agent_id"]),
			"binding_at": toString(signal["binding_at"]), "assignment_id": toString(signal["assignment_id"]),
		}
		// This is an attention signal, not an immediate push: hooks may arrive
		// shortly after dispatch and should not create transient alerts.
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
    case "controller_completion_review":
		return toString(reason["state"])
	case "completion_pending": return "controller_recovery"
	case "user_intervention","approval_required": return "needs_user"
    case "runtime_stalled","execution_interrupted","runtime_unknown": return "stalled"
    default: return fileState
    }
}


const recoveredBindingStartLiveWindow = 60 * time.Second

func recoveredBindingStartIsLive(project,taskID string,now time.Time,shared ...requestRuntime) bool {
    ledger,err:=readRequestRuntime(project,10,now,shared...)
    if err!=nil { return false }
    taskID=strings.ToUpper(strings.TrimSpace(taskID))
    for _,attempt:=range ledger.Attempts {
        if strings.ToUpper(strings.TrimSpace(attempt.TaskID))!=taskID { continue }
        if attempt.BindingState!=runtimeobs.BindingBound || !strings.EqualFold(strings.TrimSpace(attempt.BindingSource),"backlog_assignment") { continue }
        if strings.TrimSpace(attempt.LastActivityAt)=="" { return false }
        at,ok:=parseTime(attempt.LastActivityAt)
        if !ok { return false }
        age:=now.Sub(at)
        if age<0 { age=0 }
        return age<=recoveredBindingStartLiveWindow
    }
    return false
}


func reconcileCanonicalBindings(project string,rows []Record,now time.Time,shared ...requestRuntime) map[string]bool {
    recovered:=map[string]bool{}
    ledger,err:=readRequestRuntime(project,10,now,shared...)
    if err!=nil { return recovered }
    staleAttempts:=map[string]bool{}
    for _,finding:=range ledger.Findings {
        if finding.Code=="stale" && strings.TrimSpace(finding.AttemptID)!="" {
            staleAttempts[finding.AttemptID]=true
        }
    }
    byID:=preferredRows(rows)
	// Resolve the complete candidate graph before writing a binding. A worker
	// can own multiple doing tasks while only one runtime attempt is visible;
	// iterating the backlog map and binding as we go would pick a task at random.
	candidatesByTask := map[string][]runtimeobs.Attempt{}
	tasksByAttempt := map[string][]string{}
	for id,row:=range byID {
        if row.Location!="active" || row.State!="doing" { continue }
        agent:=strings.TrimSpace(row.Fields["Agent"])
        if agent=="" { continue }
        metadata:=runtimeFromFields(row.Fields)
        expectedProvider:=strings.ToLower(strings.TrimSpace(toString(metadata["runtime_provider"])))
        if expectedProvider=="unknown" { expectedProvider="" }
        workerName:=agent
        if slash:=strings.LastIndex(workerName,"/"); slash>=0 { workerName=workerName[slash+1:] }
        candidates:=[]runtimeobs.Attempt{}
        for _,attempt:=range ledger.Attempts {
            if attempt.Terminal || attempt.BindingState!=runtimeobs.BindingUnbound || staleAttempts[attempt.AttemptID] { continue }
            if expectedProvider!="" && !strings.EqualFold(strings.TrimSpace(attempt.Provider),expectedProvider) { continue }
            runtimeID:=strings.TrimSpace(attempt.RuntimeAgentID)
            if runtimeID!="" && (runtimeID==workerName || runtimeID==agent) { candidates=append(candidates,attempt) }
        }
		candidatesByTask[id] = candidates
		for _, candidate := range candidates {
			tasksByAttempt[candidate.AttemptID] = append(tasksByAttempt[candidate.AttemptID], id)
		}
	}
	for id, candidates := range candidatesByTask {
		if len(candidates)!=1 || len(tasksByAttempt[candidates[0].AttemptID]) != 1 { continue }
		agent := strings.TrimSpace(byID[id].Fields["Agent"])
		evidence:=map[string]string{"agent_path":agent,"correlation":"canonical_backlog_assignment"}
        if _,err:=runtimeobs.BindAttempt(project,candidates[0].AttemptID,id,agent,"backlog_assignment","",evidence,now); err==nil {
            recovered[id]=true
        }
    }
    return recovered
}
