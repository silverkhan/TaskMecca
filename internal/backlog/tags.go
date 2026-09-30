package backlog

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strings"
    "time"
)

type TagDefinition struct {
    Canonical string   `json:"canonical"`
    Namespace string   `json:"namespace"`
    Name string        `json:"name"`
    Description string `json:"description,omitempty"`
    Aliases []string   `json:"aliases,omitempty"`
    Status string      `json:"status"`
    ReplacedBy string  `json:"replaced_by,omitempty"`
    CreatedAt string   `json:"created_at"`
    UpdatedAt string   `json:"updated_at"`
}

type TagRegistry struct {
    Version int             `json:"version"`
    Tags []TagDefinition    `json:"tags"`
}

type TagStat struct {
    Tag string          `json:"tag"`
    Namespace string    `json:"namespace"`
    Description string  `json:"description,omitempty"`
    Status string       `json:"status,omitempty"`
    Total int           `json:"total"`
    Active int          `json:"active"`
    Hold int            `json:"hold"`
    Done int            `json:"done"`
    Tasks []string      `json:"tasks,omitempty"`
}

type TagIndex struct {
    Version int                    `json:"version"`
    GeneratedAt string             `json:"generated_at"`
    Tasks map[string][]string      `json:"tasks"`
    Stats []TagStat                `json:"stats"`
    Unregistered []string          `json:"unregistered,omitempty"`
}

var tagPartRE=regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func tagRegistryPath(project string) string {
    return filepath.Join(project,"_task_mecca","data","tags","registry.json")
}

func tagIndexPath(project string) string {
    return filepath.Join(project,"_task_mecca",".runtime","tag_index.json")
}

func normalizeTagPart(value string) string {
    value=strings.ToLower(strings.TrimSpace(value))
    value=strings.ReplaceAll(value,"_","-")
    value=strings.Join(strings.Fields(value),"-")
    for strings.Contains(value,"--") { value=strings.ReplaceAll(value,"--","-") }
    return strings.Trim(value,"-")
}

func NormalizeTag(value string) (string,error) {
    value=strings.TrimSpace(value)
    parts:=strings.SplitN(value,":",2)
    if len(parts)!=2 { return "",fmt.Errorf("tag must use namespace:name form: %s",value) }
    namespace:=normalizeTagPart(parts[0])
    name:=normalizeTagPart(parts[1])
    if !tagPartRE.MatchString(namespace) || !tagPartRE.MatchString(name) {
        return "",fmt.Errorf("invalid tag: %s",value)
    }
    return namespace+":"+name,nil
}

func parseTagList(value string) []string {
    seen:=map[string]bool{}
    out:=[]string{}
    value=strings.ReplaceAll(value,"\n",",")
    for _,raw:=range strings.Split(value,",") {
        tag,err:=NormalizeTag(raw)
        if err!=nil || seen[tag] { continue }
        seen[tag]=true
        out=append(out,tag)
    }
    sort.Strings(out)
    return out
}

func defaultTagDefinitions(now string) []TagDefinition {
    rows:=[][3]string{
        {"area:backend","Backend/server-side implementation","백엔드,server"},
        {"area:frontend","Frontend/Web UI implementation","프론트엔드,ui,web-ui"},
        {"area:strategy","Domain strategy or model logic","전략"},
        {"area:data","Data pipeline, schema, storage or quality","데이터"},
        {"area:infra","Infrastructure, packaging, deployment or runtime","인프라,infrastructure"},
        {"area:docs","Documentation and operating guidance","문서,documentation"},
        {"type:feature","New user-visible capability","기능,신규기능"},
        {"type:bug","Defect correction","버그,오류"},
        {"type:improvement","Improvement to existing behavior or UX","개선"},
        {"type:refactor","Internal restructuring without intended behavior change","리팩터링,refactoring"},
        {"type:research","Investigation, spike or research task","연구,조사"},
        {"concern:performance","Performance, latency or resource efficiency","성능"},
        {"concern:security","Security, access control or trust boundary","보안"},
        {"concern:ux","User experience and interaction quality","사용성,user-experience"},
        {"concern:compatibility","Cross-version, platform or migration compatibility","호환성"},
        {"concern:data-integrity","Correctness and preservation of durable data","데이터무결성,data integrity"},
    }
    out:=make([]TagDefinition,0,len(rows))
    for _,row:=range rows {
        parts:=strings.SplitN(row[0],":",2)
        out=append(out,TagDefinition{
            Canonical:row[0],Namespace:parts[0],Name:parts[1],
            Description:row[1],Aliases:splitAliases(row[2]),Status:"active",CreatedAt:now,UpdatedAt:now,
        })
    }
    return out
}

func saveTagRegistry(project string,registry TagRegistry) error {
    registry.Version=1
    sort.Slice(registry.Tags,func(i,j int)bool{return registry.Tags[i].Canonical<registry.Tags[j].Canonical})
    path:=tagRegistryPath(project)
    if err:=os.MkdirAll(filepath.Dir(path),0755); err!=nil { return err }
    data,err:=json.MarshalIndent(registry,"","  ")
    if err!=nil { return err }
    tmp:=path+".tmp"
    if err:=os.WriteFile(tmp,append(data,'\n'),0644); err!=nil { return err }
    return os.Rename(tmp,path)
}

func loadTagRegistry(project string,create bool) (TagRegistry,error) {
    path:=tagRegistryPath(project)
    data,err:=os.ReadFile(path)
    if errors.Is(err,os.ErrNotExist) {
        now:=time.Now().UTC().Format(time.RFC3339)
        registry:=TagRegistry{Version:1,Tags:defaultTagDefinitions(now)}
        if create { return registry,saveTagRegistry(project,registry) }
        return registry,nil
    }
    if err!=nil { return TagRegistry{},err }
    registry:=TagRegistry{}
    if err:=json.Unmarshal(data,&registry); err!=nil { return TagRegistry{},fmt.Errorf("invalid tag registry: %w",err) }
    if registry.Version==0 { registry.Version=1 }
    if registry.Tags==nil { registry.Tags=[]TagDefinition{} }
    return registry,nil
}

func EnsureTagRegistry(project string) (TagRegistry,error) {
    return loadTagRegistry(project,true)
}

func tagDefinitionIndex(registry TagRegistry,canonical string) int {
    for i,row:=range registry.Tags {
        if strings.EqualFold(row.Canonical,canonical) { return i }
    }
    return -1
}

func resolveTagDefinition(registry TagRegistry,query string) (TagDefinition,string,bool) {
    normalized,normalizeErr:=NormalizeTag(query)
    if normalizeErr==nil {
        for _,row:=range registry.Tags {
            if strings.EqualFold(row.Canonical,normalized) {
                resolved:=row
                seen:=map[string]bool{}
                for resolved.Status=="retired" && resolved.ReplacedBy!="" && !seen[resolved.Canonical] {
                    seen[resolved.Canonical]=true
                    idx:=tagDefinitionIndex(registry,resolved.ReplacedBy)
                    if idx<0 { break }
                    resolved=registry.Tags[idx]
                }
                return resolved,row.Canonical,true
            }
        }
    }
    lower:=strings.ToLower(strings.TrimSpace(query))
    for _,row:=range registry.Tags {
        for _,alias:=range row.Aliases {
            if strings.ToLower(strings.TrimSpace(alias))==lower {
                resolved:=row
                seen:=map[string]bool{}
                for resolved.Status=="retired" && resolved.ReplacedBy!="" && !seen[resolved.Canonical] {
                    seen[resolved.Canonical]=true
                    idx:=tagDefinitionIndex(registry,resolved.ReplacedBy)
                    if idx<0 { break }
                    resolved=registry.Tags[idx]
                }
                return resolved,alias,true
            }
        }
    }
    return TagDefinition{},"",false
}

func TagResolve(project,query string) (map[string]any,error) {
    registry,err:=loadTagRegistry(project,false)
    if err!=nil { return nil,err }
    resolved,matched,ok:=resolveTagDefinition(registry,query)
    return map[string]any{"query":query,"found":ok,"matched":matched,"tag":resolved},nil
}

func TagList(project string) ([]TagDefinition,error) {
    registry,err:=loadTagRegistry(project,false)
    if err!=nil { return nil,err }
    rows:=append([]TagDefinition{},registry.Tags...)
    sort.Slice(rows,func(i,j int)bool{return rows[i].Canonical<rows[j].Canonical})
    return rows,nil
}

func TagSearch(project,query string) ([]TagDefinition,error) {
    registry,err:=loadTagRegistry(project,false)
    if err!=nil { return nil,err }
    q:=strings.ToLower(strings.TrimSpace(query))
    rows:=[]TagDefinition{}
    for _,row:=range registry.Tags {
        hay:=strings.ToLower(strings.Join(append([]string{row.Canonical,row.Namespace,row.Name,row.Description},row.Aliases...)," "))
        if q=="" || strings.Contains(hay,q) { rows=append(rows,row) }
    }
    sort.Slice(rows,func(i,j int)bool{return rows[i].Canonical<rows[j].Canonical})
    return rows,nil
}

func TagShow(project,query string) (map[string]any,error) {
    registry,err:=loadTagRegistry(project,false)
    if err!=nil { return nil,err }
    row,matched,ok:=resolveTagDefinition(registry,query)
    if !ok { return map[string]any{"found":false,"query":query},nil }
    return map[string]any{"found":true,"query":query,"matched":matched,"tag":row},nil
}

func splitAliases(value string) []string {
    seen:=map[string]bool{}
    out:=[]string{}
    for _,raw:=range strings.Split(value,",") {
        alias:=strings.TrimSpace(raw)
        if alias=="" { continue }
        key:=strings.ToLower(alias)
        if seen[key] { continue }
        seen[key]=true
        out=append(out,alias)
    }
    sort.Strings(out)
    return out
}

func TagDefine(project,canonical,description,aliases string) (TagDefinition,error) {
    normalized,err:=NormalizeTag(canonical)
    if err!=nil { return TagDefinition{},err }
    registry,err:=EnsureTagRegistry(project)
    if err!=nil { return TagDefinition{},err }
    if tagDefinitionIndex(registry,normalized)>=0 { return TagDefinition{},fmt.Errorf("tag already exists: %s",normalized) }
    requestedAliases:=splitAliases(aliases)
    for _,alias:=range requestedAliases {
        if existing,matched,ok:=resolveTagDefinition(registry,alias); ok {
            return TagDefinition{},fmt.Errorf("alias %q already resolves to %s via %s",alias,existing.Canonical,matched)
        }
    }
    now:=time.Now().UTC().Format(time.RFC3339)
    parts:=strings.SplitN(normalized,":",2)
    row:=TagDefinition{
        Canonical:normalized,Namespace:parts[0],Name:parts[1],
        Description:strings.TrimSpace(description),Aliases:requestedAliases,
        Status:"active",CreatedAt:now,UpdatedAt:now,
    }
    registry.Tags=append(registry.Tags,row)
    return row,saveTagRegistry(project,registry)
}

func rewriteTaskTagField(text string,tags []string) string {
    value:=strings.Join(tags,", ")
    lines:=strings.Split(text,"\n")
    for i,line:=range lines {
        trimmed:=strings.TrimSpace(line)
        if strings.HasPrefix(trimmed,"- Tags:") {
            prefix:=line[:len(line)-len(strings.TrimLeft(line," \t"))]
            lines[i]=prefix+"- Tags: "+value
            return strings.Join(lines,"\n")
        }
    }
    insertAt:=-1
    inOverview:=false
    for i,line:=range lines {
        if strings.TrimSpace(line)=="## 작업 개요" { inOverview=true; continue }
        if inOverview && strings.HasPrefix(strings.TrimSpace(line),"## ") { insertAt=i; break }
        if inOverview && strings.HasPrefix(strings.TrimSpace(line),"- 연관:") { insertAt=i+1 }
    }
    if insertAt<0 {
        if len(lines)>0 && strings.HasPrefix(strings.TrimSpace(lines[0]),"# ") { insertAt=1 } else { insertAt=0 }
    }
    addition:="- Tags: "+value
    lines=append(lines,nil)
    copy(lines[insertAt+1:],lines[insertAt:])
    lines[insertAt]=addition
    return strings.Join(lines,"\n")
}

func findTaskRecord(project,root,id string) (Record,error) {
    rows,err:=Catalog(project,root)
    if err!=nil { return Record{},err }
    id=strings.ToUpper(strings.TrimSpace(id))
    matches:=[]Record{}
    for _,row:=range rows { if row.ID==id { matches=append(matches,row) } }
    if len(matches)==0 { return Record{},fmt.Errorf("task not found: %s",id) }
    if len(matches)>1 { return Record{},fmt.Errorf("duplicate task id: %s",id) }
    return matches[0],nil
}

func canonicalAssignableTag(registry TagRegistry,input string) (string,error) {
    row,_,ok:=resolveTagDefinition(registry,input)
    if !ok { return "",fmt.Errorf("undefined tag: %s (define or resolve it first)",input) }
    if row.Status!="active" { return "",fmt.Errorf("tag is retired: %s",input) }
    return row.Canonical,nil
}

func writeTaskTags(path string,tags []string) error {
    data,err:=os.ReadFile(path)
    if err!=nil { return err }
    tags=dedupeSorted(tags)
    updated:=rewriteTaskTagField(strings.TrimPrefix(string(data),"\ufeff"),tags)
    return os.WriteFile(path,[]byte(updated),0644)
}

func dedupeSorted(values []string) []string {
    seen:=map[string]bool{}
    out:=[]string{}
    for _,value:=range values {
        if value=="" || seen[value] { continue }
        seen[value]=true
        out=append(out,value)
    }
    sort.Strings(out)
    return out
}

func TaskTags(project,root,id string) (map[string]any,error) {
    row,err:=findTaskRecord(project,root,id)
    if err!=nil { return nil,err }
    return map[string]any{"id":row.ID,"tags":parseTagList(row.Fields["Tags"]),"path":row.Path},nil
}

func TaskTagSet(project,root,id string,inputs []string) (map[string]any,error) {
    row,err:=findTaskRecord(project,root,id)
    if err!=nil { return nil,err }
    registry,err:=EnsureTagRegistry(project)
    if err!=nil { return nil,err }
    tags:=[]string{}
    for _,input:=range inputs {
        if strings.TrimSpace(input)=="" { continue }
        canonical,resolveErr:=canonicalAssignableTag(registry,input)
        if resolveErr!=nil { return nil,resolveErr }
        tags=append(tags,canonical)
    }
    tags=dedupeSorted(tags)
    if err:=writeTaskTags(row.Path,tags); err!=nil { return nil,err }
    _,_ = RebuildTagIndex(project,root)
    return map[string]any{"id":row.ID,"tags":tags,"path":row.Path},nil
}

func TaskTagAdd(project,root,id,input string) (map[string]any,error) {
    current,err:=TaskTags(project,root,id)
    if err!=nil { return nil,err }
    existing,_:=current["tags"].([]string)
    registry,err:=EnsureTagRegistry(project)
    if err!=nil { return nil,err }
    canonical,err:=canonicalAssignableTag(registry,input)
    if err!=nil { return nil,err }
    return TaskTagSet(project,root,id,append(existing,canonical))
}

func TaskTagRemove(project,root,id,input string) (map[string]any,error) {
    row,err:=findTaskRecord(project,root,id)
    if err!=nil { return nil,err }
    registry,err:=EnsureTagRegistry(project)
    if err!=nil { return nil,err }
    canonical:=input
    if resolved,_,ok:=resolveTagDefinition(registry,input); ok { canonical=resolved.Canonical }
    if normalized,normErr:=NormalizeTag(canonical); normErr==nil { canonical=normalized }
    current:=parseTagList(row.Fields["Tags"])
    next:=[]string{}
    for _,tag:=range current { if tag!=canonical { next=append(next,tag) } }
    if err:=writeTaskTags(row.Path,next); err!=nil { return nil,err }
    _,_ = RebuildTagIndex(project,root)
    return map[string]any{"id":row.ID,"tags":next,"path":row.Path},nil
}

func replaceTagInAllTasks(project,root,oldTag,newTag string) (int,error) {
    rows,err:=Catalog(project,root)
    if err!=nil { return 0,err }
    changed:=0
    for _,row:=range rows {
        tags:=parseTagList(row.Fields["Tags"])
        found:=false
        next:=[]string{}
        for _,tag:=range tags {
            if tag==oldTag { found=true; if newTag!="" { next=append(next,newTag) } } else { next=append(next,tag) }
        }
        if !found { continue }
        if err:=writeTaskTags(row.Path,dedupeSorted(next)); err!=nil { return changed,err }
        changed++
    }
    return changed,nil
}

func TagRename(project,root,oldInput,newInput string) (map[string]any,error) {
    oldTag,err:=NormalizeTag(oldInput); if err!=nil { return nil,err }
    newTag,err:=NormalizeTag(newInput); if err!=nil { return nil,err }
    if oldTag==newTag { return nil,fmt.Errorf("old and new tags are identical") }
    registry,err:=EnsureTagRegistry(project); if err!=nil { return nil,err }
    oldIndex:=tagDefinitionIndex(registry,oldTag)
    if oldIndex<0 { return nil,fmt.Errorf("tag not found: %s",oldTag) }
    if tagDefinitionIndex(registry,newTag)>=0 { return nil,fmt.Errorf("target tag already exists: %s; use merge",newTag) }
    old:=registry.Tags[oldIndex]
    now:=time.Now().UTC().Format(time.RFC3339)
    parts:=strings.SplitN(newTag,":",2)
    aliases:=dedupeSorted(append(append([]string{},old.Aliases...),oldTag))
    target:=TagDefinition{
        Canonical:newTag,Namespace:parts[0],Name:parts[1],Description:old.Description,
        Aliases:aliases,Status:"active",CreatedAt:now,UpdatedAt:now,
    }
    registry.Tags[oldIndex].Status="retired"
    registry.Tags[oldIndex].ReplacedBy=newTag
    registry.Tags[oldIndex].UpdatedAt=now
    registry.Tags=append(registry.Tags,target)
    if err:=saveTagRegistry(project,registry); err!=nil { return nil,err }
    changed,err:=replaceTagInAllTasks(project,root,oldTag,newTag)
    if err!=nil { return nil,err }
    _,_ = RebuildTagIndex(project,root)
    return map[string]any{"from":oldTag,"to":newTag,"tasks_updated":changed},nil
}

func TagMerge(project,root,oldInput,targetInput string) (map[string]any,error) {
    oldTag,err:=NormalizeTag(oldInput); if err!=nil { return nil,err }
    registry,err:=EnsureTagRegistry(project); if err!=nil { return nil,err }
    target,_,ok:=resolveTagDefinition(registry,targetInput)
    if !ok || target.Status!="active" { return nil,fmt.Errorf("active target tag not found: %s",targetInput) }
    oldIndex:=tagDefinitionIndex(registry,oldTag)
    if oldIndex<0 { return nil,fmt.Errorf("tag not found: %s",oldTag) }
    if oldTag==target.Canonical { return nil,fmt.Errorf("cannot merge a tag into itself") }
    targetIndex:=tagDefinitionIndex(registry,target.Canonical)
    old:=registry.Tags[oldIndex]
    now:=time.Now().UTC().Format(time.RFC3339)
    registry.Tags[oldIndex].Status="retired"
    registry.Tags[oldIndex].ReplacedBy=target.Canonical
    registry.Tags[oldIndex].UpdatedAt=now
    registry.Tags[targetIndex].Aliases=dedupeSorted(append(registry.Tags[targetIndex].Aliases,append(old.Aliases,oldTag)...))
    registry.Tags[targetIndex].UpdatedAt=now
    if err:=saveTagRegistry(project,registry); err!=nil { return nil,err }
    changed,err:=replaceTagInAllTasks(project,root,oldTag,target.Canonical)
    if err!=nil { return nil,err }
    _,_ = RebuildTagIndex(project,root)
    return map[string]any{"from":oldTag,"to":target.Canonical,"tasks_updated":changed},nil
}

func TagRetire(project,root,input,replacement string) (map[string]any,error) {
    if strings.TrimSpace(replacement)!="" { return TagMerge(project,root,input,replacement) }
    tag,err:=NormalizeTag(input); if err!=nil { return nil,err }
    registry,err:=EnsureTagRegistry(project); if err!=nil { return nil,err }
    idx:=tagDefinitionIndex(registry,tag)
    if idx<0 { return nil,fmt.Errorf("tag not found: %s",tag) }
    registry.Tags[idx].Status="retired"
    registry.Tags[idx].ReplacedBy=""
    registry.Tags[idx].UpdatedAt=time.Now().UTC().Format(time.RFC3339)
    if err:=saveTagRegistry(project,registry); err!=nil { return nil,err }
    _,_ = RebuildTagIndex(project,root)
    return map[string]any{"tag":tag,"status":"retired"},nil
}

func buildTagIndex(project,root string,rows []Record) (TagIndex,error) {
    registry,err:=loadTagRegistry(project,false)
    if err!=nil { return TagIndex{},err }
    definitions:=map[string]TagDefinition{}
    for _,row:=range registry.Tags { definitions[row.Canonical]=row }
    tasks:=map[string][]string{}
    stats:=map[string]*TagStat{}
    unregistered:=map[string]bool{}
    for _,row:=range rows {
        tags:=parseTagList(row.Fields["Tags"])
        tasks[row.ID]=tags
        for _,tag:=range tags {
            stat:=stats[tag]
            if stat==nil {
                def:=definitions[tag]
                stat=&TagStat{Tag:tag,Namespace:strings.SplitN(tag,":",2)[0],Description:def.Description,Status:def.Status,Tasks:[]string{}}
                if stat.Status=="" { stat.Status="unregistered"; unregistered[tag]=true }
                stats[tag]=stat
            }
            stat.Total++
            if row.State=="done" { stat.Done++ } else if row.State=="hold" { stat.Hold++ } else { stat.Active++ }
            stat.Tasks=append(stat.Tasks,row.ID)
        }
    }
    rowsOut:=make([]TagStat,0,len(stats))
    for _,stat:=range stats {
        sort.Strings(stat.Tasks)
        rowsOut=append(rowsOut,*stat)
    }
    sort.Slice(rowsOut,func(i,j int)bool{return rowsOut[i].Tag<rowsOut[j].Tag})
    missing:=[]string{}
    for tag:=range unregistered { missing=append(missing,tag) }
    sort.Strings(missing)
    return TagIndex{
        Version:1,GeneratedAt:time.Now().UTC().Format(time.RFC3339),
        Tasks:tasks,Stats:rowsOut,Unregistered:missing,
    },nil
}

func RebuildTagIndex(project,root string) (TagIndex,error) {
    if _,err:=EnsureTagRegistry(project); err!=nil { return TagIndex{},err }
    rows,err:=Catalog(project,root)
    if err!=nil { return TagIndex{},err }
    index,err:=buildTagIndex(project,root,rows)
    if err!=nil { return TagIndex{},err }
    path:=tagIndexPath(project)
    if err:=os.MkdirAll(filepath.Dir(path),0755); err!=nil { return TagIndex{},err }
    data,err:=json.MarshalIndent(index,"","  "); if err!=nil { return TagIndex{},err }
    tmp:=path+".tmp"
    if err:=os.WriteFile(tmp,append(data,'\n'),0644); err!=nil { return TagIndex{},err }
    if err:=os.Rename(tmp,path); err!=nil { return TagIndex{},err }
    return index,nil
}

func TagStatsReport(project,root string) ([]TagStat,error) {
    rows,err:=Catalog(project,root); if err!=nil { return nil,err }
    index,err:=buildTagIndex(project,root,rows); if err!=nil { return nil,err }
    return index.Stats,nil
}

func parseTagExpression(expr string) [][]string {
    groups:=[][]string{}
    for _,andPart:=range strings.Split(expr,",") {
        choices:=[]string{}
        for _,orPart:=range strings.Split(andPart,"|") {
            tag,err:=NormalizeTag(orPart)
            if err==nil { choices=append(choices,tag) }
        }
        if len(choices)>0 { groups=append(groups,dedupeSorted(choices)) }
    }
    return groups
}

func tagsMatchExpression(tags []string,groups [][]string) bool {
    if len(groups)==0 { return true }
    present:=map[string]bool{}
    for _,tag:=range tags { present[tag]=true }
    for _,group:=range groups {
        matched:=false
        for _,candidate:=range group { if present[candidate] { matched=true; break } }
        if !matched { return false }
    }
    return true
}

func TagTasks(project,root,expr string) ([]map[string]any,error) {
    registry,err:=loadTagRegistry(project,false); if err!=nil { return nil,err }
    groups:=[][]string{}
    for _,andPart:=range strings.Split(expr,",") {
        choices:=[]string{}
        for _,orPart:=range strings.Split(andPart,"|") {
            token:=strings.TrimSpace(orPart)
            resolved,_,ok:=resolveTagDefinition(registry,token)
            if ok { choices=append(choices,resolved.Canonical); continue }
            normalized,normErr:=NormalizeTag(token)
            if normErr!=nil { return nil,fmt.Errorf("invalid tag expression token %q: %w",token,normErr) }
            choices=append(choices,normalized)
        }
        if len(choices)>0 { groups=append(groups,dedupeSorted(choices)) }
    }
    rows,err:=Catalog(project,root); if err!=nil { return nil,err }
    out:=[]map[string]any{}
    for _,row:=range rows {
        tags:=parseTagList(row.Fields["Tags"])
        if !tagsMatchExpression(tags,groups) { continue }
        out=append(out,map[string]any{
            "id":row.ID,"state":row.State,"title":row.Title,"location":row.Location,
            "archive_month":row.ArchiveMonth,"tags":tags,"path":row.Path,
        })
    }
    sort.Slice(out,func(i,j int)bool{return fmt.Sprint(out[i]["id"])<fmt.Sprint(out[j]["id"])})
    return out,nil
}

func TagCatalog(project,root string,rows []Record) (map[string]any,error) {
    registry,err:=loadTagRegistry(project,false); if err!=nil { return nil,err }
    index,err:=buildTagIndex(project,root,rows); if err!=nil { return nil,err }
    namespaces:=map[string][]TagStat{}
    for _,stat:=range index.Stats { namespaces[stat.Namespace]=append(namespaces[stat.Namespace],stat) }
    return map[string]any{
        "registry":registry.Tags,
        "stats":index.Stats,
        "namespaces":namespaces,
        "unregistered":index.Unregistered,
    },nil
}
