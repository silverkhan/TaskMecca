package backlog

import (
    "regexp"
    "sort"
    "strings"
)

var idRef = regexp.MustCompile(`[A-Za-z]+-\d+`)

func refs(value string) []string {
    seen:=map[string]bool{}
    out:=[]string{}
    for _,raw:=range idRef.FindAllString(value,-1) {
        id:=strings.ToUpper(raw)
        if !seen[id] { seen[id]=true; out=append(out,id) }
    }
    return out
}

func groupedByID(rows []Record) map[string][]Record {
    grouped:=map[string][]Record{}
    for _,row:=range rows { grouped[row.ID]=append(grouped[row.ID],row) }
    return grouped
}

func claimBlockers(fields map[string]string,grouped map[string][]Record) map[string]any {
    required:=refs(fields["선행"])
    waiting:=[]string{}
    for _,dep:=range required {
        matches,ok:=grouped[dep]
        done:=false
        if ok {
            for _,row:=range matches { if row.State=="done" { done=true; break } }
        }
        if !ok || !done { waiting=append(waiting,dep) }
    }
    note:=strings.TrimSpace(fields["대기"])
    blockedBy:=[]string{}
    if len(waiting)>0 { blockedBy=append(blockedBy,"waiting_for_dependencies") }
    if note!="" { blockedBy=append(blockedBy,"waiting_note") }
    return map[string]any{
        "depends_on":required,
        "waiting_for":waiting,
        "waiting_note":note,
        "blocked_by":blockedBy,
    }
}

func DependencyReport(rows []Record) map[string]any {
    grouped:=groupedByID(rows)
    missing:=[]map[string]string{}
    graph:=map[string][]string{}
    order:=[]string{}
    for _,row:=range rows {
        deps:=refs(row.Fields["선행"])
        if _,ok:=graph[row.ID]; !ok { order=append(order,row.ID) }
        graph[row.ID]=deps
        for _,dep:=range deps {
            if _,ok:=grouped[dep]; !ok { missing=append(missing,map[string]string{"id":row.ID,"depends_on":dep}) }
        }
    }
    cycles:=[][]string{}
    visited:=map[string]bool{}
    visiting:=[]string{}
    var walk func(string)
    walk=func(node string) {
        for i,current:=range visiting {
            if current==node {
                cycle:=append([]string{},visiting[i:]...)
                cycle=append(cycle,node)
                duplicate:=false
                for _,known:=range cycles {
                    if strings.Join(known,"\x00")==strings.Join(cycle,"\x00") { duplicate=true; break }
                }
                if !duplicate { cycles=append(cycles,cycle) }
                return
            }
        }
        if visited[node] { return }
        visiting=append(visiting,node)
        for _,dep:=range graph[node] {
            if _,ok:=graph[dep]; ok { walk(dep) }
        }
        visiting=visiting[:len(visiting)-1]
        visited[node]=true
    }
    for _,node:=range order { walk(node) }
    return map[string]any{"missing":missing,"cycles":cycles}
}

func Ready(project,root string) (map[string]any,error) {
    rows,err:=Catalog(project,root)
    if err!=nil { return nil,err }
    grouped:=groupedByID(rows)
    ready:=[]map[string]any{}
    blocked:=[]map[string]any{}
    for _,row:=range rows {
        if row.Location!="active" || row.State!="todo" { continue }
        blockers:=claimBlockers(row.Fields,grouped)
        target:=&ready
        if values,ok:=blockers["blocked_by"].([]string); ok && len(values)>0 { target=&blocked }
        item:=map[string]any{"id":row.ID,"title":row.Title,"path":row.Path}
        for key,value:=range blockers { item[key]=value }
        *target=append(*target,item)
    }
    selected,err:=Select(project,root)
    if err!=nil { return nil,err }
    if selected=="" {
        selected=root
        if selected=="" { selected=project }
    }
    sort.Slice(ready,func(i,j int)bool{return ready[i]["id"].(string)<ready[j]["id"].(string)})
    sort.Slice(blocked,func(i,j int)bool{return blocked[i]["id"].(string)<blocked[j]["id"].(string)})
    return map[string]any{"root":selected,"ready":ready,"blocked":blocked,"problems":DependencyReport(rows)},nil
}
