package backlog

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func writeTagTestTask(t *testing.T,folder,name,body string) string {
    t.Helper()
    path:=filepath.Join(folder,name)
    if err:=os.MkdirAll(filepath.Dir(path),0755); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(path,[]byte(body),0644); err!=nil { t.Fatal(err) }
    return path
}

func TestTagRegistrySeedsNamespacesAndResolvesAliases(t *testing.T) {
    project:=t.TempDir()
    registry,err:=EnsureTagRegistry(project)
    if err!=nil { t.Fatal(err) }
    if len(registry.Tags)<10 { t.Fatalf("seeded tags=%d",len(registry.Tags)) }
    if _,err:=os.Stat(tagRegistryPath(project)); err!=nil { t.Fatalf("registry missing: %v",err) }
    report,err:=TagResolve(project,"프론트엔드")
    if err!=nil { t.Fatal(err) }
    if report["found"]!=true { t.Fatalf("resolve=%+v",report) }
    row:=report["tag"].(TagDefinition)
    if row.Canonical!="area:frontend" { t.Fatalf("resolved=%s",row.Canonical) }
}

func TestTaskTagAssignSetRemoveAndIndexStats(t *testing.T) {
    project:=t.TempDir()
    folder:=filepath.Join(project,"_task_mecca","data","backlog")
    writeTagTestTask(t,folder,"000001.A-1.alpha.todo.md","# A-1 Alpha\n\n## 작업 개요\n\n- 등록자: root\n- Agent: -\n- 변경범위: src\n- 선행: -\n- 연관: -\n\n## 작업 정의\n\n### 목표\nAlpha\n### 수용 기준\n- [ ] done\n")
    writeTagTestTask(t,folder,"000002.A-2.beta.done.md","# A-2 Beta\n\n## 작업 개요\n\n- 등록자: root\n- Agent: -\n- 변경범위: web\n- 선행: -\n- 연관: -\n- Tags: area:frontend, type:bug\n\n## 작업 정의\n\n### 목표\nBeta\n### 수용 기준\n- [x] done\n")
    if _,err:=EnsureTagRegistry(project); err!=nil { t.Fatal(err) }
    added,err:=TaskTagAdd(project,folder,"A-1","프론트엔드")
    if err!=nil { t.Fatal(err) }
    tags:=added["tags"].([]string)
    if len(tags)!=1 || tags[0]!="area:frontend" { t.Fatalf("tags=%v",tags) }
    set,err:=TaskTagSet(project,folder,"A-1",[]string{"area:frontend","type:feature","concern:ux"})
    if err!=nil { t.Fatal(err) }
    if len(set["tags"].([]string))!=3 { t.Fatalf("set=%+v",set) }
    removed,err:=TaskTagRemove(project,folder,"A-1","type:feature")
    if err!=nil { t.Fatal(err) }
    if len(removed["tags"].([]string))!=2 { t.Fatalf("removed=%+v",removed) }
    index,err:=RebuildTagIndex(project,folder)
    if err!=nil { t.Fatal(err) }
    if _,err:=os.Stat(tagIndexPath(project)); err!=nil { t.Fatalf("index missing: %v",err) }
    if got:=index.Tasks["A-1"]; len(got)!=2 { t.Fatalf("A-1 index=%v",got) }
    stats,err:=TagStatsReport(project,folder)
    if err!=nil { t.Fatal(err) }
    found:=false
    for _,stat:=range stats {
        if stat.Tag=="area:frontend" {
            found=true
            if stat.Total!=2 || stat.Active!=1 || stat.Done!=1 { t.Fatalf("stat=%+v",stat) }
        }
    }
    if !found { t.Fatal("frontend stat missing") }
}

func TestTagRenameAndMergeRewriteActiveAndArchive(t *testing.T) {
    project:=t.TempDir()
    folder:=filepath.Join(project,"_task_mecca","data","backlog")
    active:=writeTagTestTask(t,folder,"000010.A-10.active.todo.md","# A-10 Active\n- Tags: custom:server, type:bug\n")
    archived:=writeTagTestTask(t,filepath.Join(folder,"archive","2026-09"),"000009.A-9.archive.done.md","# A-9 Archived\n- Tags: custom:server, area:frontend\n")
    if _,err:=TagDefine(project,"custom:server","Server work","server-work"); err!=nil { t.Fatal(err) }
    renamed,err:=TagRename(project,folder,"custom:server","custom:backend-service")
    if err!=nil { t.Fatal(err) }
    if renamed["tasks_updated"]!=2 { t.Fatalf("rename=%+v",renamed) }
    for _,path:=range []string{active,archived} {
        data,err:=os.ReadFile(path); if err!=nil { t.Fatal(err) }
        text:=string(data)
        if !strings.Contains(text,"custom:backend-service") || strings.Contains(text,"custom:server,") { t.Fatalf("rewrite failed: %s",text) }
    }
    resolved,err:=TagResolve(project,"custom:server")
    if err!=nil { t.Fatal(err) }
    if resolved["found"]!=true || resolved["tag"].(TagDefinition).Canonical!="custom:backend-service" { t.Fatalf("resolve=%+v",resolved) }
    merged,err:=TagMerge(project,folder,"custom:backend-service","area:backend")
    if err!=nil { t.Fatal(err) }
    if merged["tasks_updated"]!=2 { t.Fatalf("merge=%+v",merged) }
    rows,err:=TagTasks(project,folder,"area:backend,type:bug|area:frontend")
    if err!=nil { t.Fatal(err) }
    if len(rows)!=2 { t.Fatalf("rows=%+v",rows) }
}

func TestUndefinedTagCannotBeAssigned(t *testing.T) {
    project:=t.TempDir()
    folder:=filepath.Join(project,"_task_mecca","data","backlog")
    writeTagTestTask(t,folder,"000001.A-1.alpha.todo.md","# A-1 Alpha\n")
    if _,err:=TaskTagAdd(project,folder,"A-1","custom:not-defined"); err==nil { t.Fatal("expected undefined tag assignment to fail") }
}
