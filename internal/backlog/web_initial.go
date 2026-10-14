package backlog

import (
    "path/filepath"
    "sort"
    "strings"
    "time"
)

// BacklogInitialPage is a read-only, file-backed first paint. Its task states
// are provisional until the normal reconciled page replaces them. Never use
// this snapshot to send alerts, infer runtime starts, or persist lifecycles.
func BacklogInitialPage(project,root string,page,pageSize int,tags []string,search,sortKey string) (map[string]any,error) {
    rows,err:=CachedCatalog(project,root)
    if err!=nil{return nil,err}
    presence,err:=Presence(project,root,rows)
    if err!=nil{return nil,err}
    byID:=preferredRows(rows)
    q:=strings.ToLower(strings.TrimSpace(search))
    filtered:=make([]Record,0,len(byID))
    counts:=map[string]int{"all":0,"working":0,"ready":0,"blocked":0,"hold":0,"done":0,"attention":0,"needs_action":0}
    for id,row:=range byID {
        if len(tags)>0 && !tagsMatchAll(map[string]any{"tags":parseTagList(row.Fields["Tags"])},tags){continue}
        if q!="" && !strings.Contains(strings.ToLower(id+"\n"+row.Title+"\n"+row.RawMarkdown+"\n"+row.ArchiveMonth),q){continue}
        filtered=append(filtered,row)
        counts["all"]++
        switch row.State {
        case "doing":counts["working"]++
        case "hold":counts["hold"]++
        case "done":counts["done"]++
        }
    }
    sort.Slice(filtered,func(i,j int)bool{
        left,right:=filtered[i],filtered[j]
        switch sortKey {
        case "updated_asc":
            if left.Mtime!=right.Mtime {return left.Mtime<right.Mtime}
            return left.SortKey<right.SortKey
        case "updated_desc":
            if left.Mtime!=right.Mtime {return left.Mtime>right.Mtime}
            if left.SortKey!=right.SortKey {return left.SortKey>right.SortKey}
            return left.ID>right.ID
        case "id_asc":
            if left.SortKey!=right.SortKey {return left.SortKey<right.SortKey}
            return left.ID<right.ID
        default:
            if left.SortKey!=right.SortKey {return left.SortKey>right.SortKey}
            return left.ID>right.ID
        }
    })
    if pageSize<1 {pageSize=20};if pageSize>100{pageSize=100};if page<1{page=1}
    total:=len(filtered);pages:=(total+pageSize-1)/pageSize
    if pages<1{pages=1};if page>pages{page=pages}
    begin:=(page-1)*pageSize;end:=begin+pageSize;if end>total{end=total}
    items:=make([]map[string]any,0,end-begin)
    for _,row:=range filtered[begin:end] {
        item:=webSummaryItem(row,row.State,nil,nil,nil)
        item["document"]=compactListDocument(row)
        item["fields"]=compactListFields(row)
        item["state_provisional"]=true
        items=append(items,item)
    }
    repoName:=filepath.Base(project)
    return map[string]any{
        "items":items,"page":page,"page_size":pageSize,"pages":pages,"total":total,
        "repo":repoName,"root":root,"counts":counts,
        "backlog_presence":presence,"pending_reconciliation":true,
        "snapshot_at":time.Now().Format(time.RFC3339Nano),
    },nil
}
