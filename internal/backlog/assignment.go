package backlog

import (
    "sort"
    "strings"
)

func assignmentView(row Record) map[string]any {
    agent:=strings.TrimSpace(row.Fields["Agent"])
    scope:=strings.TrimSpace(row.Fields["변경범위"])
    if row.State=="done" {
        return map[string]any{"agent":agent,"change_scope":scope,"assignment_kind":"historical","hold_audit":nil}
    }
    if row.Location=="active" && row.State=="doing" {
        return map[string]any{"agent":agent,"change_scope":scope,"assignment_kind":"current","hold_audit":nil}
    }
    if row.Location=="active" && row.State=="hold" {
        return map[string]any{
            "agent":"","change_scope":"","assignment_kind":"released",
            "hold_audit":map[string]any{
                "recorded_agent":agent,
                "recorded_change_scope":scope,
                "branch":strings.TrimSpace(row.Fields["Branch"]),
                "runner":strings.TrimSpace(row.Fields["실행기"]),
            },
        }
    }
    if row.Location=="active" {
        return map[string]any{"agent":"","change_scope":"","assignment_kind":"unassigned","hold_audit":nil}
    }
    return map[string]any{"agent":agent,"change_scope":scope,"assignment_kind":"historical","hold_audit":nil}
}

func canonicalAgent(value string) bool {
    value=strings.TrimSpace(value)
    if value=="/root" { return true }
    if !strings.HasPrefix(value,"/root/") { return false }
    for _,part:=range strings.Split(strings.TrimPrefix(value,"/root/"),"/") {
        if part=="" { return false }
        for _,r:=range part {
            if !((r>='a'&&r<='z')||(r>='0'&&r<='9')||r=='_') { return false }
        }
    }
    return true
}

func continuity(rows []Record,row Record) ([]map[string]any,[]string) {
    evidence:=map[string]bool{}
    for _,id:=range append(refs(row.Fields["선행"]),refs(row.Fields["연관"])...) { evidence[id]=true }
    scores:=map[string][]string{}
    legacySet:=map[string]bool{}
    for _,candidate:=range rows {
        if !evidence[candidate.ID] || candidate.State!="done" { continue }
        agent:=strings.TrimSpace(candidate.Fields["Agent"])
        if agent=="" { continue }
        if canonicalAgent(agent) { scores[agent]=append(scores[agent],candidate.ID) } else { legacySet[agent]=true }
    }
    out:=[]map[string]any{}
    for agent,ids:=range scores {
        out=append(out,map[string]any{"agent":agent,"evidence_ids":ids,"count":len(ids)})
    }
    sort.Slice(out,func(i,j int)bool {
        ci:=out[i]["count"].(int); cj:=out[j]["count"].(int)
        if ci!=cj { return ci>cj }
        return out[i]["agent"].(string)<out[j]["agent"].(string)
    })
    legacy:=[]string{}
    for agent:=range legacySet { legacy=append(legacy,agent) }
    sort.Strings(legacy)
    return out,legacy
}

func HoldReview(rows []Record) map[string]any {
    grouped:=groupedByID(rows)
    candidates:=[]map[string]any{}
    waiting:=[]map[string]any{}
    for _,row:=range rows {
        if row.Location!="active" || row.State!="hold" { continue }
        note:=strings.TrimSpace(row.Fields["대기"])
        kind:=strings.TrimSpace(row.Fields["대기유형"])
        resume:=strings.TrimSpace(row.Fields["재개조건"])
        evidence:=strings.TrimSpace(row.Fields["대기근거"])
        reasons:=[]map[string]string{}
        warn:=func(code,message string){ reasons=append(reasons,map[string]string{"code":code,"message":message}) }
        if note=="" { warn("hold_wait_reason_missing","직접 대기 사유가 기록되지 않았습니다.") }
        if resume=="" { warn("hold_resume_condition_missing","재개 조건이 별도 기록되지 않았습니다.") }
        if evidence=="" { warn("hold_evidence_missing","대기 판단 근거가 별도 기록되지 않았습니다.") }
        if kind=="" {
            warn("hold_wait_kind_unrecorded","legacy/유형 미기록: 자유 서술에서 대기 유형을 추측하지 않습니다.")
        } else if kind!="external" && kind!="user" && kind!="dependency" && kind!="internal" {
            warn("hold_wait_kind_invalid","대기유형은 external/user/dependency/internal 중 하나입니다.")
        }
        deps:=[]map[string]any{}
        for _,id:=range refs(row.Fields["선행"]) {
            matches:=grouped[id]
            status:="missing"
            if len(matches)>1 { status="ambiguous" } else if len(matches)==1 {
                if matches[0].State=="done" { status="done" } else { status="pending" }
            }
            states:=[]string{}; paths:=[]string{}
            for _,m:=range matches { states=append(states,m.State); paths=append(paths,m.Path) }
            deps=append(deps,map[string]any{"id":id,"status":status,"states":states,"paths":paths})
        }
        if kind=="internal" { warn("hold_internal_work","내부 실행 가능한 구현·통합·검증은 doing인지 검토하세요.") }
        if kind=="dependency" && len(deps)==0 { warn("hold_dependencies_missing","dependency 대기인데 명시한 선행 ID가 없습니다.") }
        if (kind=="dependency" || kind=="") && len(deps)>0 {
            allDone:=true
            for _,dep:=range deps { if dep["status"]!="done" { allDone=false; break } }
            if allDone { warn("hold_dependencies_done","모든 명시 선행이 done입니다. 현재 직접 대기 사유를 재검토하세요.") }
        }
        item:=map[string]any{
            "id":row.ID,"title":row.Title,"path":row.Path,"state":"hold",
            "wait_kind":firstNonEmpty(kind,"unrecorded"),"wait_note":note,
            "resume_condition":resume,"wait_evidence":evidence,
            "dependencies":deps,"reasons":reasons,"review_required":len(reasons)>0,"ready":false,
        }
        if len(reasons)>0 { candidates=append(candidates,item) } else { waiting=append(waiting,item) }
    }
    return map[string]any{
        "review_needed":len(candidates)>0,"candidates":candidates,"waiting":waiting,
        "meaning":"수동 검토 권고입니다. ready/수용 통과/자동 재개/spawn 또는 drain 지속 명령이 아닙니다. 현재 이벤트에서 검토 후 실제 외부·사용자 대기만 남으면 cycle을 끝낼 수 있습니다.",
    }
}
