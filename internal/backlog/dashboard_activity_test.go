package backlog

import (
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestRuntimeActivityMarksCompletedWorkerAsAwaitingFinalize(t *testing.T) {
    project:=t.TempDir()
    runtimeDir:=filepath.Join(project,"_task_mecca",".runtime","agents")
    if err:=os.MkdirAll(runtimeDir,0755); err!=nil { t.Fatal(err) }
    heartbeat:=[]byte(`{"agent":"/root/controller/kkobugi","task_id":"A-23","state":"completed","heartbeat_at":"2026-09-30T05:00:00Z"}`)
    if err:=os.WriteFile(filepath.Join(runtimeDir,"kkobugi.json"),heartbeat,0644); err!=nil { t.Fatal(err) }

    rows:=[]Record{{
        ID:"A-23",
        State:"doing",
        Location:"active",
        Mtime:time.Date(2026,9,30,4,0,0,0,time.UTC).Format(time.RFC3339),
        Fields:map[string]string{"Agent":"/root/controller/kkobugi"},
    }}
    got:=runtimeActivity(project,rows,map[string]map[string]any{})
    if got["A-23"]["health"]!="awaiting_finalize" {
        t.Fatalf("health=%v want awaiting_finalize",got["A-23"]["health"])
    }
}

func TestRuntimeActivityMarksExplicitUserWait(t *testing.T) {
    project:=t.TempDir()
    runtimeDir:=filepath.Join(project,"_task_mecca",".runtime","agents")
    if err:=os.MkdirAll(runtimeDir,0755); err!=nil { t.Fatal(err) }
    heartbeat:=[]byte(`{"agent":"/root/controller/kkobugi","task_id":"A-23","state":"needs_user","heartbeat_at":"2026-09-30T05:00:00Z"}`)
    if err:=os.WriteFile(filepath.Join(runtimeDir,"kkobugi.json"),heartbeat,0644); err!=nil { t.Fatal(err) }

    rows:=[]Record{{
        ID:"A-23",
        State:"doing",
        Location:"active",
        Mtime:time.Date(2026,9,30,4,0,0,0,time.UTC).Format(time.RFC3339),
        Fields:map[string]string{"Agent":"/root/controller/kkobugi"},
    }}
    got:=runtimeActivity(project,rows,map[string]map[string]any{})
    if got["A-23"]["health"]!="needs_user" {
        t.Fatalf("health=%v want needs_user",got["A-23"]["health"])
    }
}
