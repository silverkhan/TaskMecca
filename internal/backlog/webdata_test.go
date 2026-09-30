package backlog

import (
    "os"
    "path/filepath"
    "testing"
    "time"
)

func writeWebTask(t *testing.T,folder,name,body string) string {
    t.Helper()
    if err:=os.MkdirAll(folder,0755); err!=nil { t.Fatal(err) }
    path:=filepath.Join(folder,name)
    if err:=os.WriteFile(path,[]byte(body),0644); err!=nil { t.Fatal(err) }
    return path
}

func TestCachedCatalogRefreshesOnlyChangedViewOfFiles(t *testing.T) {
    project:=t.TempDir()
    folder:=filepath.Join(project,"_task_mecca","data","backlog")
    path:=writeWebTask(t,folder,"000001.A-1.alpha.todo.md","# A-1 Alpha\n- 등록자: root\n")

    rows,err:=CachedCatalog(project,folder)
    if err!=nil { t.Fatal(err) }
    if len(rows)!=1 || rows[0].Title!="A-1 Alpha" { t.Fatalf("rows=%+v",rows) }

    time.Sleep(2*time.Millisecond)
    if err:=os.WriteFile(path,[]byte("# A-1 Updated\n- 등록자: root\n"),0644); err!=nil { t.Fatal(err) }
    rows,err=CachedCatalog(project,folder)
    if err!=nil { t.Fatal(err) }
    if len(rows)!=1 || rows[0].Title!="A-1 Updated" { t.Fatalf("updated rows=%+v",rows) }

    if err:=os.Remove(path); err!=nil { t.Fatal(err) }
    rows,err=CachedCatalog(project,folder)
    if err!=nil { t.Fatal(err) }
    if len(rows)!=0 { t.Fatalf("removed file remained cached: %+v",rows) }
}

func TestBacklogPagePaginatesFiltersAndOmitsRawMarkdown(t *testing.T) {
    project:=t.TempDir()
    folder:=filepath.Join(project,"_task_mecca","data","backlog")
    writeWebTask(t,folder,"000001.A-1.alpha.todo.md","# A-1 Alpha\n- Tags: area:backend\n- 설명: alpha search token\n")
    writeWebTask(t,folder,"000002.A-2.beta.doing.md","# A-2 Beta\n- Agent: /root/controller/pairi\n- Tags: area:backend\n")
    writeWebTask(t,folder,"000003.A-3.gamma.done.md","# A-3 Gamma\n- Tags: area:frontend\n")
    writeWebTask(t,folder,"000004.A-4.delta.done.md","# A-4 Delta\n- Tags: area:backend\n")
    if _,err:=EnsureTagRegistry(project); err!=nil { t.Fatal(err) }

    page,err:=BacklogPage(project,folder,1,1,[]string{"done"},[]string{"area:backend"},"","id_desc")
    if err!=nil { t.Fatal(err) }
    if page["total"]!=1 { t.Fatalf("total=%v page=%+v",page["total"],page) }
    items:=page["items"].([]map[string]any)
    if len(items)!=1 || items[0]["id"]!="A-4" { t.Fatalf("items=%+v",items) }
    if _,exists:=items[0]["raw_markdown"]; exists { t.Fatal("paged summary leaked raw_markdown") }
    if _,exists:=items[0]["lifecycle"]; exists { t.Fatal("paged summary leaked lifecycle detail") }

    searched,err:=BacklogPage(project,folder,1,20,nil,nil,"search token","id_desc")
    if err!=nil { t.Fatal(err) }
    results:=searched["items"].([]map[string]any)
    if len(results)!=1 || results[0]["id"]!="A-1" { t.Fatalf("search=%+v",results) }
}

func TestTaskDetailLoadsOneTaskWithRawMarkdown(t *testing.T) {
    project:=t.TempDir()
    folder:=filepath.Join(project,"_task_mecca","data","backlog")
    writeWebTask(t,folder,"000001.A-1.alpha.todo.md","# A-1 Alpha\n- 설명: direct detail\n")

    item,err:=TaskDetail(project,folder,"A-1")
    if err!=nil { t.Fatal(err) }
    if item["id"]!="A-1" { t.Fatalf("item=%+v",item) }
    raw,ok:=item["raw_markdown"].(string)
    if !ok || raw=="" { t.Fatalf("raw_markdown=%v",item["raw_markdown"]) }
}

func TestAttentionSnapshotDetectsCompletedRuntimeAndPersistsCompletion(t *testing.T) {
    project:=t.TempDir()
    folder:=filepath.Join(project,"_task_mecca","data","backlog")
    path:=writeWebTask(t,folder,"000001.A-1.alpha.doing.md","# A-1 Alpha\n- Agent: /root/controller/pairi\n")
    runtimeDir:=filepath.Join(project,"_task_mecca",".runtime","agents")
    if err:=os.MkdirAll(runtimeDir,0755); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(filepath.Join(runtimeDir,"pairi.json"),[]byte(`{"agent":"/root/controller/pairi","task_id":"A-1","state":"completed","heartbeat_at":"2026-09-30T13:00:00Z"}`),0644); err!=nil { t.Fatal(err) }

    first,err:=AttentionSnapshot(project,folder,true)
    if err!=nil { t.Fatal(err) }
    attention:=first["attention"].([]map[string]any)
    if len(attention)!=1 || attention[0]["type"]!="completion_pending" { t.Fatalf("attention=%+v",attention) }

    if err:=os.Remove(path); err!=nil { t.Fatal(err) }
    writeWebTask(t,folder,"000001.A-1.alpha.done.md","# A-1 Alpha\n- Agent: /root/controller/pairi\n- 결과: done\n")
    second,err:=AttentionSnapshot(project,folder,true)
    if err!=nil { t.Fatal(err) }
    events:=second["notification_events"].([]map[string]any)
    found:=false
    for _,event:=range events { if event["task_id"]=="A-1" && event["kind"]=="completed" { found=true } }
    if !found { t.Fatalf("completion event missing: %+v",events) }
}


func TestBacklogRevisionIgnoresMtimeOnlyTouchesButDetectsContentChange(t *testing.T) {
    project:=t.TempDir()
    folder:=filepath.Join(project,"_task_mecca","data","backlog")
    path:=writeWebTask(t,folder,"000001.A-1.alpha.todo.md","# A-1 Alpha\n- 설명: same content\n")

    first,err:=BacklogRevision(project,folder)
    if err!=nil { t.Fatal(err) }
    firstRevision:=first["revision"]
    now:=time.Now().Add(2*time.Second)
    if err:=os.Chtimes(path,now,now); err!=nil { t.Fatal(err) }
    second,err:=BacklogRevision(project,folder)
    if err!=nil { t.Fatal(err) }
    if second["revision"]!=firstRevision {
        t.Fatalf("mtime-only touch changed revision: %v -> %v",firstRevision,second["revision"])
    }

    if err:=os.WriteFile(path,[]byte("# A-1 Alpha\n- 설명: changed content\n"),0644); err!=nil { t.Fatal(err) }
    third,err:=BacklogRevision(project,folder)
    if err!=nil { t.Fatal(err) }
    if third["revision"]==firstRevision {
        t.Fatalf("content change did not change revision: %v",third["revision"])
    }
}
