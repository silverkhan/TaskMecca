package backlog

import (
    "os"
    "path/filepath"
    "testing"
)

func TestReadyUsesDoneDependenciesAndWaitingNotes(t *testing.T) {
    root:=t.TempDir()
    folder:=filepath.Join(root,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(folder,0755); err!=nil { t.Fatal(err) }
    cases:=map[string]string{
        "000001.A-1.done.done.md":"# A-1 Done\n- 선행: -\n",
        "000002.A-2.ready.todo.md":"# A-2 Ready\n- 선행: A-1\n- 대기: -\n",
        "000003.A-3.blocked.todo.md":"# A-3 Blocked\n- 선행: A-9\n- 대기: approval\n",
    }
    for name,text:=range cases {
        if err:=os.WriteFile(filepath.Join(folder,name),[]byte(text),0644); err!=nil { t.Fatal(err) }
    }
    report,err:=Ready(root,"")
    if err!=nil { t.Fatal(err) }
    ready:=report["ready"].([]map[string]any)
    blocked:=report["blocked"].([]map[string]any)
    if len(ready)!=1 || ready[0]["id"]!="A-2" { t.Fatalf("ready=%+v",ready) }
    if len(blocked)!=1 || blocked[0]["id"]!="A-3" { t.Fatalf("blocked=%+v",blocked) }
    waiting:=blocked[0]["waiting_for"].([]string)
    if len(waiting)!=1 || waiting[0]!="A-9" { t.Fatalf("waiting=%+v",waiting) }
    problems:=report["problems"].(map[string]any)
    missing:=problems["missing"].([]map[string]string)
    if len(missing)!=1 || missing[0]["depends_on"]!="A-9" { t.Fatalf("missing=%+v",missing) }
}
