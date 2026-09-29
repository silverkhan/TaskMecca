package backlog

import (
    "os"
    "path/filepath"
    "testing"
)

func TestCoordinateScopeConflictAndAudit(t *testing.T) {
    root:=t.TempDir()
    folder:=filepath.Join(root,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(folder,0755); err!=nil { t.Fatal(err) }

    tasks:=map[string]string{
        "000001.A-1.one.doing.md":"# A-1 One\n- Agent: /root/controller/pairi\n- 변경범위: internal/backlog\n",
        "000002.A-2.two.doing.md":"# A-2 Two\n- Agent: /root/controller/kkobugi\n- 변경범위: internal/backlog/parser\n",
        "000003.A-3.done.done.md":"# A-3 Done\n- Agent: /root/controller/pairi\n- 결과: -\n- 검증: -\n",
    }
    for name,body:=range tasks {
        if err:=os.WriteFile(filepath.Join(folder,name),[]byte(body),0644); err!=nil { t.Fatal(err) }
    }

    report,err:=Coordinate(root,"",3)
    if err!=nil { t.Fatal(err) }
    conflicts:=report["scope_conflicts"].([]map[string]any)
    if len(conflicts)!=1 { t.Fatalf("conflicts=%+v",conflicts) }
    fill:=report["parallel_fill"].(map[string]any)
    if fill["active_doing"]!=2 || fill["candidate_slots"]!=1 { t.Fatalf("fill=%+v",fill) }

    findings,err:=Audit(root,"")
    if err!=nil { t.Fatal(err) }
    if len(findings)!=2 { t.Fatalf("findings=%+v",findings) }
}
