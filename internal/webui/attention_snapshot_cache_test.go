package webui

import (
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/silverkhan/TaskMecca/internal/projectionchange"
)

func attentionMemoFixture(t *testing.T)(string,string) {
    t.Helper()
    project:=t.TempDir()
    root:=filepath.Join(project,"_task_mecca","data","backlog")
    for _,dir:=range []string{root,filepath.Join(project,".git","refs","heads")} {
        if err:=os.MkdirAll(dir,0755);err!=nil{t.Fatal(err)}
    }
    for path,value:=range map[string]string{
        filepath.Join(project,".git","HEAD"):"ref: refs/heads/main\n",
        filepath.Join(project,".git","refs","heads","main"):"abcdef1234567890\n",
        filepath.Join(root,"000001.A-1.todo.md"):"# A-1\n",
    }{
        if err:=os.WriteFile(path,[]byte(value),0644);err!=nil{t.Fatal(err)}
    }
    return project,root
}

func TestAttentionMemoSourceMutationAndClockInvalidation(t *testing.T) {
    project,root:=attentionMemoFixture(t)
    sources,ok:=attentionSourceInventory(project,root)
    if !ok{t.Fatal("valid canonical source inventory unavailable")}
    now:=time.Now()
    payload:=map[string]any{"attention":[]map[string]any{},"snapshot_at":"old","content_revision":"same"}
    memo:=&attentionSnapshotMemo{sources:sources,at:now,payload:payload,project:project,generation:projectionchange.Version(project)}
    fresh,ok:=attentionSourceInventory(project,root)
    if !ok{t.Fatal("source inventory lost")}
    reused,hit:=reuseAttentionSnapshot(memo,fresh,true,now.Add(time.Second))
    if !hit || reused["content_revision"]!="same" || reused["snapshot_at"]=="old" {t.Fatalf("unchanged source not reused: %v hit=%v",reused,hit)}
    if payload["snapshot_at"]!="old" {t.Fatal("memo payload mutated by reuse")}
    if _,hit:=reuseAttentionSnapshot(memo,fresh,true,now.Add(attentionSnapshotCheckpoint+time.Second));hit{
        t.Fatal("periodic full audit checkpoint bypassed")
    }
    projectionchange.Mark(project)
    if _,hit:=reuseAttentionSnapshot(memo,fresh,true,now.Add(time.Second));hit{
        t.Fatal("process-local control mutation did not invalidate memo")
    }
}

func TestAttentionMemoDetectsExternalAtomicReplacementAndGitHead(t *testing.T){
    project,root:=attentionMemoFixture(t)
    before,valid:=attentionSourceInventory(project,root);if !valid{t.Fatal("inventory invalid")}
    path:=filepath.Join(root,"000001.A-1.todo.md")
    original,err:=os.Stat(path);if err!=nil{t.Fatal(err)}
    tmp:=path+".replacement"
    if err:=os.WriteFile(tmp,[]byte("# B-2\n"),0644);err!=nil{t.Fatal(err)}
    if err:=os.Chtimes(tmp,original.ModTime(),original.ModTime());err!=nil{t.Fatal(err)}
    if err:=os.Rename(tmp,path);err!=nil{t.Fatal(err)}
    after,valid:=attentionSourceInventory(project,root);if !valid{t.Fatal("updated inventory invalid")}
    if sameAttentionSources(before,after){t.Fatal("same-size/mtime atomic file replacement ignored")}
    ref:=filepath.Join(project,".git","refs","heads","main")
    if err:=os.WriteFile(ref,[]byte("123456abcdef7890\n"),0644);err!=nil{t.Fatal(err)}
    newest,valid:=attentionSourceInventory(project,root)
    if !valid || sameAttentionSources(after,newest){t.Fatal("Git history ref update not tracked")}
    if _,valid:=attentionSourceInventory(project,filepath.Join(project,"other"));valid{
        t.Fatal("alternate/noncanonical root must not use incomplete source inventory")
    }
}
