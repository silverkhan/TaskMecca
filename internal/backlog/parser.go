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
    "RuntimeProvider": true, "Dispatch상태": true, "실행근거": true, "Fallback근거": true,
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
    return map[string]any{
        "schema":schema,
        "contract_kind":kind,
        "sections":sections,
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

func runtimeFromFields(fields map[string]string) map[string]string {
    return map[string]string{
        "RuntimeProvider":fields["RuntimeProvider"],
        "Dispatch상태":fields["Dispatch상태"],
        "실행근거":fields["실행근거"],
        "Fallback근거":fields["Fallback근거"],
    }
}

func firstNonEmpty(values ...string) string {
    for _,value:=range values { if value!="" { return value } }
    return ""
}
