package backlog

import (
    "fmt"
    "sort"
    "strings"
)

var workerNames = []string{
    "kkobugi", "pairi", "isanghaessi", "pikachyu", "raichyu", "naong",
    "jammanbo", "ibui", "purin", "metamong", "mangnanyong", "gorapadeok",
    "paenteom", "rukario", "sikseuteil", "rapeuraseu", "rioreu", "togepi",
    "seurakeu", "eonibugi", "geobukwang", "rijadeu", "rijamong",
    "isanghaepul", "isanghaekkot", "ppippi", "myu", "digeuda",
    "kkoret", "moraeduji", "kkomadol", "rongseuton",
}

type evidence struct {
    Identity string `json:"identity"`
    Source string `json:"source"`
}

type conflict struct {
    NewAlias string `json:"new_alias"`
    Identities []string `json:"identities"`
}

func identityName(value string) string {
    value=strings.TrimSpace(value)
    value=strings.TrimPrefix(value,"/root/controller/")
    if strings.Contains(value,"/") { return "" }
    return value
}

func equivalent(value string) string {
    value=identityName(value)
    if _,ok:=workerPolicy[value]; ok { return value }
    for _,name:=range workerNames {
        for _,old:=range workerPolicy[name] { if old==value { return name } }
    }
    return ""
}

func WorkerName(project, root string, used []string) (map[string]any,error) {
    reservations:=map[string][]evidence{}
    ignored:=[]string{}
    reserve:=func(value,source string) {
        alias:=identityName(value)
        name:=equivalent(value)
        if name=="" {
            if source=="live_used" { ignored=append(ignored,value) }
            return
        }
        entry:=evidence{Identity:"/root/controller/"+alias,Source:source}
        for _,item:=range reservations[name] { if item==entry { return } }
        reservations[name]=append(reservations[name],entry)
    }
    for _,value:=range used { reserve(value,"live_used") }
    rows,err:=Catalog(project,root)
    if err!=nil { return nil,err }
    for _,row:=range rows {
        if row.Location!="active" || row.State!="doing" { continue }
        reserve(row.Fields["Agent"],fmt.Sprintf("active_doing:%s",row.ID))
    }
    selected:=""
    unavailable:=[]string{}
    for _,name:=range workerNames {
        if _,ok:=reservations[name]; ok { unavailable=append(unavailable,name) } else if selected=="" { selected=name }
    }
    conflicts:=[]conflict{}
    for name,entries:=range reservations {
        sort.Slice(entries,func(i,j int)bool { if entries[i].Identity!=entries[j].Identity { return entries[i].Identity<entries[j].Identity }; return entries[i].Source<entries[j].Source })
        reservations[name]=entries
        identities:=map[string]bool{}
        for _,entry:=range entries { identities[entry.Identity]=true }
        if len(identities)>1 {
            values:=[]string{}
            for identity:=range identities { values=append(values,identity) }
            sort.Strings(values)
            conflicts=append(conflicts,conflict{NewAlias:name,Identities:values})
        }
    }
    sort.Slice(conflicts,func(i,j int)bool { return conflicts[i].NewAlias<conflicts[j].NewAlias })
    sort.Strings(ignored)
    unique:=[]string{}
    for _,value:=range ignored { if len(unique)==0 || unique[len(unique)-1]!=value { unique=append(unique,value) } }
    var name any=nil
    var path any=nil
    var reason any=nil
    if selected!="" { name=selected; path="/root/controller/"+selected } else { reason="pool_exhausted" }
    return map[string]any{
        "ok":selected!="", "alias":name, "task_name":name, "path":path,
        "reason":reason, "unavailable":unavailable, "reservations":reservations,
        "equivalent_conflicts":conflicts, "ignored_used":unique,
        "pool":workerNames, "equivalences":workerPolicy,
        "new_validation":"agent <path> --new --json",
        "rule":"new workers use confirmed Korean-pronunciation ASCII aliases; identity is stable and task-independent; no task/model/feature names or fallback",
        "live_state_note":"pass current runtime worker aliases/paths with --used; Git alone cannot know live agents",
    },nil
}
