package backlog

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
)

func baseRoot(project, root string) string {
    base:=root
    if base=="" { base=filepath.Join(project,"_task_mecca") }
    if absolute,err:=filepath.Abs(base); err==nil { base=absolute }
    return base
}

func Presence(project,root string, records []Record) (map[string]any,error) {
    base:=baseRoot(project,root)
    selected,err:=Select(project,root)
    if err!=nil { return nil,err }
    if selected=="" {
        return map[string]any{
            "ok":true,"status":"uninitialized","root":base,"folders":[]string{},
            "record_count":0,"active_count":0,
            "message":"아직 등록된 작업이 없어 backlog 원장이 생성되지 않았다. 첫 등록 시 Registrar가 data/backlog를 생성한다.",
        },nil
    }
    rows:=records
    if rows==nil {
        rows,err=Catalog(project,root)
        if err!=nil { return nil,err }
    }
    folders:=[]string{selected}
    status:="empty"
    message:="backlog 후보 폴더는 있으나 인식 가능한 항목이 없다."
    if len(rows)>0 { status="ok"; message="backlog를 찾았고 항목을 읽을 수 있다." }
    active:=0
    for _,row:=range rows { if row.Location=="active" { active++ } }
    return map[string]any{
        "ok":true,"status":status,"root":base,"folders":folders,
        "record_count":len(rows),"active_count":active,"message":message,
    },nil
}

func duplicateIDs(rows []Record) []map[string]any {
    grouped:=groupedByID(rows)
    ids:=[]string{}
    for id,matches:=range grouped { if len(matches)>1 { ids=append(ids,id) } }
    sort.Strings(ids)
    out:=[]map[string]any{}
    for _,id:=range ids {
        files:=[]string{}
        for _,row:=range grouped[id] { files=append(files,row.Path) }
        out=append(out,map[string]any{"id":id,"files":files})
    }
    return out
}

func filenameProblems(rows []Record) []map[string]string {
    out:=[]map[string]string{}
    for _,row:=range rows {
        parts:=strings.Split(row.ID,"-")
        valid:=false
        if len(parts)>=2 {
            if number,err:=strconv.Atoi(parts[len(parts)-1]); err==nil {
                if sortKey,err:=strconv.Atoi(row.SortKey); err==nil { valid=sortKey==number }
            }
        }
        if !valid { out=append(out,map[string]string{"file":row.Path,"problem":"sort-key/id mismatch"}) }
    }
    return out
}

func unrecognizedFiles(project,root string) ([]map[string]string,error) {
    candidates,err:=Discover(project,root)
    if err!=nil { return nil,err }
    out:=[]map[string]string{}
    for _,candidate:=range candidates {
        paths,err:=History(candidate.Path)
        if err!=nil { return nil,err }
        for _,path:=range paths {
            name:=filepath.Base(path)
            if strings.HasPrefix(name,"_") { continue }
            if !itemName.MatchString(name) {
                out=append(out,map[string]string{"file":path,"problem":"unrecognized backlog filename"})
            }
        }
    }
    return out,nil
}

func agentProblems(rows []Record) []map[string]string {
    out:=[]map[string]string{}
    for _,row:=range rows {
        if row.Location!="active" || row.State!="doing" { continue }
        agent:=strings.TrimSpace(row.Fields["Agent"])
        if agent!="" && !canonicalAgent(agent) {
            out=append(out,map[string]string{
                "id":row.ID,"agent":agent,
                "problem":"새 active Agent는 /root 또는 /root/<task_name> canonical task path여야 한다.",
            })
        }
    }
    return out
}

func contractProblems(rows []Record) []map[string]string {
    out:=[]map[string]string{}
    for _,row:=range rows {
        sections,ok:=row.Document["sections"].(map[string]string)
        if !ok {
            if generic,ok:=row.Document["sections"].(map[string]any); ok {
                sections=map[string]string{}
                for k,v:=range generic { sections[k]=strings.TrimSpace(toString(v)) }
            } else { sections=map[string]string{} }
        }
        _,hasSimple:=sections["작업 정의"]
        _,hasDefined:=sections["요건 정의서"]
        if hasSimple && hasDefined {
            out=append(out,map[string]string{
                "file":filepath.Base(row.Path),
                "problem":"Simple Task의 `작업 정의`와 Defined Task의 `요건 정의서`를 동시에 둘 수 없다.",
            })
            continue
        }
        kind,_:=row.Document["contract_kind"].(string)
        if kind!="simple" && kind!="defined" { continue }
        requirements,ok:=row.Document["requirements"].(map[string]any)
        if !ok { requirements=map[string]any{} }
        for _,spec:=range [][2]string{{"목표","goal"},{"수용 기준","acceptance"}} {
            value:=strings.TrimSpace(toString(requirements[spec[1]]))
            if value=="" || value=="-" {
                out=append(out,map[string]string{
                    "file":filepath.Base(row.Path),
                    "problem":kind+" contract에 `"+spec[0]+"`가 비어 있다.",
                })
            }
        }
    }
    return out
}

func runtimeFindings(rows []Record) []map[string]any {
    out:=[]map[string]any{}
    for _,row:=range rows {
        metadata:=runtimeFromFields(row.Fields)
        active:=row.Location=="active" && row.State=="doing"
        coverage,_:=metadata["coverage"].(string)
        status,_:=metadata["dispatch_status"].(string)
        if active && coverage=="legacy" {
            out=append(out,map[string]any{"id":row.ID,"path":row.Path,"code":"legacy_runtime_unknown","missing_fields":[]string{}})
        }
        if active && status=="failed" {
            out=append(out,map[string]any{"id":row.ID,"path":row.Path,"code":"failed_dispatch_on_doing","missing_fields":[]string{}})
        }
    }
    return out
}

func Doctor(project,root string, protocolChecks bool) (map[string]any,error) {
    rows,err:=Catalog(project,root)
    if err!=nil { return nil,err }
    deps:=DependencyReport(rows)
    presence,err:=Presence(project,root,rows)
    if err!=nil { return nil,err }
    audit,err:=Audit(project,root)
    if err!=nil { return nil,err }
    unrecognized,err:=unrecognizedFiles(project,root)
    if err!=nil { return nil,err }
    filenames:=append(filenameProblems(rows),unrecognized...)
    checks:=map[string]any{
        "backlog_presence":[]any{},
        "audit":audit,
        "duplicate_ids":duplicateIDs(rows),
        "missing_dependencies":deps["missing"],
        "dependency_cycles":deps["cycles"],
        "filenames":filenames,
        "agent_paths":agentProblems(rows),
        "scope_conflicts":ScopeConflicts(rows),
        "contracts":contractProblems(rows),
    }
    if ok,exists:=presence["ok"].(bool); exists && !ok { checks["backlog_presence"]=[]any{presence} }
    if protocolChecks {
        protocol:=filepath.Join(project,"_task_mecca","framework","collab.md")
        if stat,statErr:=os.Stat(protocol); statErr==nil && !stat.IsDir() {
            links,linkErr:=DanglingLinks(protocol)
            if linkErr!=nil { return nil,linkErr }
            checks["dangling_links"]=links
        } else {
            checks["dangling_links"]=[]string{protocol}
        }
    }
    ok:=true
    for _,value:=range checks { if nonEmpty(value) { ok=false; break } }
    hold:=HoldReview(rows)
    return map[string]any{
        "ok":ok,
        "root":baseRoot(project,root),
        "checks":checks,
        "warnings":map[string]any{
            "hold_review":hold["candidates"],
            "runtime_metadata":runtimeFindings(rows),
        },
    },nil
}

func nonEmpty(value any) bool {
    switch v:=value.(type) {
    case []any: return len(v)>0
    case []string: return len(v)>0
    case []map[string]any: return len(v)>0
    case []map[string]string: return len(v)>0
    case [][]string: return len(v)>0
    default: return value!=nil
    }
}

func toString(value any) string {
    if value==nil { return "" }
    if s,ok:=value.(string); ok { return s }
    return fmt.Sprint(value)
}
