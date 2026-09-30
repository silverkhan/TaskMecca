package backlog

import (
    "regexp"
    "strings"
)

var sectionLine = regexp.MustCompile(`^(#{2,4})\s+(.+?)\s*$`)
var checkboxLine = regexp.MustCompile(`^-\s+\[([ xX])\]\s+(.+)$`)

var fieldNames = map[string]bool{
    "등록자": true, "Agent": true, "변경범위": true, "대기": true, "대기유형": true,
    "재개조건": true, "대기근거": true, "선행": true, "연관": true, "설명": true,
    "메모": true, "결과": true, "검증": true, "Branch": true, "실행기": true,
    "RuntimeProvider": true, "Dispatch상태": true, "실행상태": true, "실행근거": true, "Fallback근거": true,
    // Legacy runtime-schema fields remain recognized as field boundaries so old
    // ledgers do not bleed their metadata into 변경범위/RuntimeProvider values.
    "Runtime계약": true, "요청Provider": true, "요청Model": true, "요청Effort": true,
    "요청출처": true, "실효Provider": true, "실효Model": true, "실효Effort": true,
}

func parseFields(text string) map[string]string {
    fields:=map[string]string{}
    for name:=range fieldNames { fields[name]="" }
    current:=""
    chunks:=[]string{}
    flush:=func(){
        if current=="" { chunks=nil; return }
        value:=strings.TrimSpace(strings.Join(chunks,"\n"))
        if value=="-" { value="" }
        fields[current]=value
        chunks=nil
    }
    for _,line:=range strings.Split(text,"\n") {
        if strings.HasPrefix(line,"- ") {
            body:=strings.TrimPrefix(line,"- ")
            if i:=strings.Index(body,":"); i>=0 {
                name:=body[:i]
                if fieldNames[name] {
                    flush()
                    current=name
                    chunks=[]string{strings.TrimSpace(body[i+1:])}
                    continue
                }
            }
        }
        if current!="" {
            if strings.HasPrefix(line,"#") {
                flush()
                current=""
            } else {
                chunks=append(chunks,line)
            }
        }
    }
    flush()
    return fields
}

func titleOf(text string) string {
    for _,line:=range strings.Split(text,"\n") {
        if strings.HasPrefix(line,"# ") { return strings.TrimSpace(strings.TrimPrefix(line,"# ")) }
    }
    return ""
}

func sectionMap(text string) map[string]string {
    raw:=map[string][]string{}
    current:=""
    for _,line:=range strings.Split(text,"\n") {
        match:=sectionLine.FindStringSubmatch(line)
        if match!=nil {
            current=strings.TrimSpace(match[2])
            if _,ok:=raw[current]; !ok { raw[current]=[]string{} }
            continue
        }
        if current!="" { raw[current]=append(raw[current],line) }
    }
    out:=map[string]string{}
    for name,lines:=range raw { out[name]=strings.TrimSpace(strings.Join(lines,"\n")) }
    return out
}

func subsections(text,parent string) map[string]string {
    raw:=map[string][]string{}
    inParent:=false
    current:=""
    for _,line:=range strings.Split(text,"\n") {
        if strings.HasPrefix(line,"## ") {
            inParent=strings.TrimSpace(line[3:])==parent
            current=""
            continue
        }
        if !inParent { continue }
        if strings.HasPrefix(line,"### ") {
            current=strings.TrimSpace(line[4:])
            if _,ok:=raw[current]; !ok { raw[current]=[]string{} }
            continue
        }
        if current!="" { raw[current]=append(raw[current],line) }
    }
    out:=map[string]string{}
    for name,lines:=range raw { out[name]=strings.TrimSpace(strings.Join(lines,"\n")) }
    return out
}

func acceptanceItems(value string) []map[string]any {
    items:=[]map[string]any{}
    for _,line:=range strings.Split(value,"\n") {
        line=strings.TrimSpace(line)
        if match:=checkboxLine.FindStringSubmatch(line); match!=nil {
            items=append(items,map[string]any{"text":strings.TrimSpace(match[2]),"checked":strings.ToLower(match[1])=="x"})
        } else if strings.HasPrefix(line,"- ") {
            items=append(items,map[string]any{"text":strings.TrimSpace(line[2:]),"checked":false})
        }
    }
    return items
}

func summaryKey(label string) string {
    normalized:=strings.TrimSpace(label)
    switch normalized {
    case "목적","작업의 목적":
        return "purpose"
    case "핵심 변경","변경":
        return "change"
    case "상태·결과","현재 상태·결과","상태/결과":
        return "status_result"
    case "확인·후속","확인·후속 사항","확인/후속":
        return "follow_up"
    default:
        return ""
    }
}

func humanSummary(value string) map[string]string {
    out:=map[string]string{
        "purpose":"",
        "change":"",
        "status_result":"",
        "follow_up":"",
    }
    current:=""
    chunks:=[]string{}
    flush:=func(){
        if current=="" { chunks=nil; return }
        text:=strings.TrimSpace(strings.Join(chunks,"\n"))
        if text=="-" { text="" }
        out[current]=text
        chunks=nil
    }
    for _,line:=range strings.Split(value,"\n") {
        trimmed:=strings.TrimSpace(line)
        if strings.HasPrefix(trimmed,"- ") {
            body:=strings.TrimSpace(strings.TrimPrefix(trimmed,"- "))
            if i:=strings.Index(body,":"); i>=0 {
                key:=summaryKey(body[:i])
                if key!="" {
                    flush()
                    current=key
                    chunks=[]string{strings.TrimSpace(body[i+1:])}
                    continue
                }
            }
        }
        if current!="" { chunks=append(chunks,line) }
    }
    flush()
    return out
}

func documentModel(text string,fields map[string]string) map[string]any {
    sections:=sectionMap(text)
    defined:=subsections(text,"요건 정의서")
    simple:=subsections(text,"작업 정의")
    scope:=subsections(text,"범위")
    includes,excludes:=[]string{},[]string{}
    var bucket *[]string
    inScope:=false
    for _,line:=range strings.Split(text,"\n") {
        if strings.HasPrefix(line,"### 범위") { inScope=true; continue }
        if inScope && strings.HasPrefix(line,"### ") { break }
        if inScope && strings.HasPrefix(line,"#### 포함") { bucket=&includes; continue }
        if inScope && strings.HasPrefix(line,"#### 제외") { bucket=&excludes; continue }
        if bucket!=nil { *bucket=append(*bucket,line) }
    }
    scopeIn:=strings.TrimSpace(strings.Join(includes,"\n"))
    scopeOut:=strings.TrimSpace(strings.Join(excludes,"\n"))
    if scopeIn=="" { scopeIn=scope["포함"] }
    if scopeOut=="" { scopeOut=scope["제외"] }

    schema,kind:="legacy","legacy"
    contract:=map[string]string{}
    if _,ok:=sections["요건 정의서"]; ok {
        schema,kind,contract="defined-v2","defined",defined
    } else if _,ok:=sections["작업 정의"]; ok {
        schema,kind,contract="simple-v2","simple",simple
    }
    acceptance:=contract["수용 기준"]
    if kind!="defined" { scopeIn=""; scopeOut="" }
    summary:=humanSummary(sections["핵심 요약"])
    return map[string]any{
        "schema":schema,
        "contract_kind":kind,
        "sections":sections,
        "summary":summary,
        "summary_present":strings.TrimSpace(sections["핵심 요약"])!="",
        "requirements":map[string]any{
            "background":defined["배경 및 문제"],
            "goal":contract["목표"],
            "requirements":defined["요구사항"],
            "scope_in":scopeIn,
            "scope_out":scopeOut,
            "acceptance":acceptance,
            "acceptance_items":acceptanceItems(acceptance),
            "constraints":defined["제약 및 보존 조건"],
        },
        "task_definition":map[string]any{
            "goal":simple["목표"],
            "acceptance":simple["수용 기준"],
            "acceptance_items":acceptanceItems(simple["수용 기준"]),
        },
        "result":firstNonEmpty(sections["결과"],fields["결과"]),
        "verification":firstNonEmpty(sections["검증"],fields["검증"]),
        "notes":sections["작업 노트"],
    }
}

func runtimeFromFields(fields map[string]string) map[string]any {
    provider:=strings.TrimSpace(fields["RuntimeProvider"])
    if provider=="" { provider=strings.TrimSpace(fields["실효Provider"]) }
    if provider=="" { provider=strings.TrimSpace(fields["요청Provider"]) }
    status:=strings.TrimSpace(fields["Dispatch상태"])
    if status=="" { status=strings.TrimSpace(fields["실행상태"]) }
    evidence:=strings.TrimSpace(fields["실행근거"])
    fallback:=strings.TrimSpace(fields["Fallback근거"])
    coverage:="legacy"
    for _,name:=range []string{"RuntimeProvider","Dispatch상태","실행상태","실행근거","Fallback근거"} {
        if strings.TrimSpace(fields[name])!="" { coverage="v2"; break }
    }
    schema:="legacy"
    if coverage=="v2" { schema="2" }
    if provider=="" { provider="unknown" }
    if status=="" { status="unknown" }
    if evidence=="" { evidence="unknown" }
    return map[string]any{
        "runtime_schema":schema,
        "coverage":coverage,
        "runtime_provider":provider,
        "dispatch_status":status,
        "execution_evidence":evidence,
        "fallback_evidence":fallback,
        "missing_fields":[]string{},
        "findings":[]any{},
    }
}

func firstNonEmpty(values ...string) string {
    for _,value:=range values { if value!="" { return value } }
    return ""
}
