package backlog

import (
    "os"
    "path/filepath"
    "testing"
)

func TestDoctorStatusCheckAndPreflight(t *testing.T) {
    root:=t.TempDir()
    gitRun(t,root,"init")
    gitRun(t,root,"config","user.email","ci@example.invalid")
    gitRun(t,root,"config","user.name","CI")
    framework:=filepath.Join(root,"_task_mecca","framework")
    ledger:=filepath.Join(root,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(framework,0755); err!=nil { t.Fatal(err) }
    if err:=os.MkdirAll(ledger,0755); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(filepath.Join(framework,"collab.md"),[]byte("# Protocol\n"),0644); err!=nil { t.Fatal(err) }
    task:="# A-1 Task\n- Agent: /root/controller/pairi\n- 변경범위: core\n- 선행: -\n- 연관: -\n- RuntimeProvider: codex\n- Dispatch상태: running\n- 실행근거: test\n- Fallback근거: -\n"
    if err:=os.WriteFile(filepath.Join(ledger,"000001.A-1.task.doing.md"),[]byte(task),0644); err!=nil { t.Fatal(err) }
    gitRun(t,root,"add",".")
    gitRun(t,root,"commit","-m","fixture")

    problems,err:=Check(root,"",filepath.Join(framework,"collab.md"))
    if err!=nil || len(problems)!=0 { t.Fatalf("check=%v err=%v",problems,err) }

    doctor,err:=Doctor(root,"",true)
    if err!=nil { t.Fatal(err) }
    if doctor["ok"]!=true { t.Fatalf("doctor=%+v",doctor) }

    status,err:=Status(root,"",false)
    if err!=nil { t.Fatal(err) }
    counts:=status["counts"].(map[string]any)
    if counts["doing"]!=1 { t.Fatalf("counts=%+v",counts) }

    externalHome:=t.TempDir()
    t.Setenv("HOME",externalHome)
    t.Setenv("CODEX_HOME",filepath.Join(externalHome,".codex"))
    t.Setenv("CODEX_SANDBOX","")
    t.Setenv("CODEX_SANDBOX_NETWORK_DISABLED","")
    preflight,err:=Preflight(root,"",true)
    if err!=nil { t.Fatal(err) }
    if preflight["orchestration_ready"]!=true { t.Fatalf("preflight=%+v",preflight) }
}
