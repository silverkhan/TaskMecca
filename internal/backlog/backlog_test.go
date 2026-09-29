package backlog

import (
    "os"
    "path/filepath"
    "testing"
)

func TestArchivedIDsAndSelection(t *testing.T) {
    root:=t.TempDir()
    folder:=filepath.Join(root,"_task_mecca","data","backlog")
    archive:=filepath.Join(folder,"archive","2026-09")
    if err:=os.MkdirAll(archive,0755); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(filepath.Join(archive,"000001.A-1.done.done.md"),[]byte("# A-1 Done\n"),0644); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(filepath.Join(folder,"000002.A-2.work.todo.md"),[]byte("# A-2 Work\n- Agent: /root/controller/pairi\n"),0644); err!=nil { t.Fatal(err) }
    rows,err:=Catalog(root,"")
    if err!=nil { t.Fatal(err) }
    if len(rows)!=2 || rows[0].Location!="archive" || rows[1].Location!="active" { t.Fatalf("unexpected catalog: %+v",rows) }
    next,err:=NextID(root,"","a")
    if err!=nil || next["sort_key"]!="000003" || next["id"]!="A-3" { t.Fatalf("unexpected next ID: %+v %v",next,err) }
}

func TestEnsureDoesNotCreateLedgerWhenExisting(t *testing.T) {
    root:=t.TempDir()
    first,err:=Ensure(root,"")
    if err!=nil || first["created"]!=true { t.Fatalf("first: %+v %v",first,err) }
    second,err:=Ensure(root,"")
    if err!=nil || second["created"]!=false || first["path"]!=second["path"] { t.Fatalf("second: %+v %v",second,err) }
}
