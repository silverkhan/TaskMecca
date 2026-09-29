package backlog

import (
    "os"
    "os/exec"
    "path/filepath"
    "testing"
)

func gitRun(t *testing.T, root string, args ...string) {
    t.Helper()
    cmd:=exec.Command("git",args...)
    cmd.Dir=root
    if out,err:=cmd.CombinedOutput(); err!=nil { t.Fatalf("git %v: %v\n%s",args,err,out) }
}

func TestInspectAndWorkloadSemantics(t *testing.T) {
    root:=t.TempDir()
    gitRun(t,root,"init")
    gitRun(t,root,"config","user.email","ci@example.invalid")
    gitRun(t,root,"config","user.name","CI")
    folder:=filepath.Join(root,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(folder,0755); err!=nil { t.Fatal(err) }

    done:=`# A-1 Done
- Agent: /root/controller/pairi
- 변경범위: core
- 선행: -
- 연관: -
- RuntimeProvider: codex
- Dispatch상태: completed
- 실행근거: test
- Fallback근거: -
`
    doing:=`# A-2 Doing
- Agent: /root/controller/kkobugi
- 변경범위: api
- 선행: A-1
- 연관: -
- RuntimeProvider: codex
- Dispatch상태: running
- 실행근거: test
- Fallback근거: -
`
    ready:=`# A-3 Ready
- Agent: -
- 변경범위: -
- 선행: A-1
- 연관: -
- 대기: -
`
    hold:=`# A-4 Hold
- Agent: /root/controller/pairi
- 변경범위: ui
- 대기: user reply
- 대기유형: user
- 재개조건: reply received
- 대기근거: ticket
- 선행: -
- 연관: -
`
    files:=map[string]string{
        "000001.A-1.done.done.md":done,
        "000002.A-2.doing.doing.md":doing,
        "000003.A-3.ready.todo.md":ready,
        "000004.A-4.hold.hold.md":hold,
    }
    for name,body:=range files {
        if err:=os.WriteFile(filepath.Join(folder,name),[]byte(body),0644); err!=nil { t.Fatal(err) }
    }
    gitRun(t,root,"add",".")
    gitRun(t,root,"commit","-m","fixture")

    inspect,err:=Inspect(root,"","A-4")
    if err!=nil { t.Fatal(err) }
    if inspect["assignment_kind"]!="released" || inspect["agent"]!="" {
        t.Fatalf("inspect assignment=%+v",inspect)
    }
    audit:=inspect["hold_audit"].(map[string]any)
    if audit["recorded_agent"]!="/root/controller/pairi" { t.Fatalf("audit=%+v",audit) }

    workload,err:=Workload(root,"")
    if err!=nil { t.Fatal(err) }
    agents:=workload["agents"].([]map[string]any)
    if len(agents)<2 { t.Fatalf("agents=%+v",agents) }

    var pairi map[string]any
    var kkobugi map[string]any
    for _,agent:=range agents {
        switch agent["agent"] {
        case "/root/controller/pairi": pairi=agent
        case "/root/controller/kkobugi": kkobugi=agent
        }
    }
    if kkobugi==nil || kkobugi["doing_count"]!=1 { t.Fatalf("kkobugi=%+v",kkobugi) }
    if pairi==nil || pairi["ready_candidate_count"]!=1 { t.Fatalf("pairi=%+v",pairi) }
    released:=workload["released_holds"].([]map[string]any)
    if len(released)!=1 || released[0]["id"]!="A-4" { t.Fatalf("released=%+v",released) }
}
