package backlog

import (
    "fmt"
    "path/filepath"
    "strings"
    "testing"
)

func TestBacklogPageProjectsOnlyRequestedRowsWithoutChangingAggregates(t *testing.T) {
    project:=t.TempDir()
    folder:=filepath.Join(project,"_task_mecca","data","backlog")
    // Exercise hundreds of cached parsed documents, archive-independent list
    // pagination, search and status counts without loading every page payload.
    for i:=1;i<=160;i++ {
        state:="todo"
        if i%4==0 { state="done" }
        category:="area:other"
        if i%2==0 { category="area:web" }
        body:=fmt.Sprintf("# A-%d Task %d\n- Tags: %s\n- 설명: %s\n",i,i,category,strings.Repeat("long description ",100))
        writeWebTask(t,folder,fmt.Sprintf("%06d.A-%d.example.%s.md",i,i,state),body)
    }
    first,err:=BacklogPage(project,folder,1,10,nil,nil,"","id_desc")
    if err!=nil { t.Fatal(err) }
    if first["total"]!=160 || first["pages"]!=16 {
        t.Fatalf("bad page metadata: total=%v pages=%v",first["total"],first["pages"])
    }
    rows:=first["items"].([]map[string]any)
    if len(rows)!=10 || rows[0]["id"]!="A-160" || rows[9]["id"]!="A-151" {
        t.Fatalf("incorrect first page: %+v",rows)
    }
    if first["counts"].(map[string]int)["done"]!=40 {
        t.Fatalf("incorrect full-backlog status count: %+v",first["counts"])
    }
    if _,ok:=rows[0]["document"].(map[string]any);!ok { t.Fatal("current page must still contain compact document") }
    if _,ok:=rows[0]["fields"].(map[string]string);!ok { t.Fatal("current page must still contain compact fields") }

    second,err:=BacklogPage(project,folder,2,10,nil,nil,"","id_desc")
    if err!=nil { t.Fatal(err) }
    if second["items"].([]map[string]any)[0]["id"]!="A-150" {
        t.Fatalf("wrong second-page record: %+v",second["items"])
    }

    summary,err:=BacklogPage(project,folder,1,10,[]string{"done"},[]string{"area:web"},"","id_desc","summary")
    if err!=nil { t.Fatal(err) }
    if got:=len(summary["items"].([]map[string]any));got!=0 { t.Fatalf("summary unexpectedly projected %d full items",got) }
    if summary["total"]!=40 || summary["counts"].(map[string]int)["all"]!=80 || summary["counts"].(map[string]int)["done"]!=40 {
        t.Fatalf("summary filters/aggregates changed: total=%v counts=%v",summary["total"],summary["counts"])
    }
}
