package backlog

import (
    "os"
    "path/filepath"
    "testing"
)

func TestWorkerReservations(t *testing.T) {
    root:=t.TempDir()
    dir:=filepath.Join(root,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(dir,0755); err!=nil { t.Fatal(err) }
    task:=filepath.Join(dir,"000001.A-1.work.doing.md")
    if err:=os.WriteFile(task,[]byte("# A-1\n- Agent: /root/controller/charmander\n"),0644); err!=nil { t.Fatal(err) }
    report,err:=WorkerName(root,"",[]string{"/root/controller/kkobugi"})
    if err!=nil { t.Fatal(err) }
    if report["path"]!="/root/controller/isanghaessi" { t.Fatalf("unexpected worker: %+v",report) }
    conflicts:=report["equivalent_conflicts"].([]conflict)
    if len(conflicts)!=0 { t.Fatalf("unexpected conflict: %+v",conflicts) }
}
