package backlog

import (
    "os"
    "os/exec"
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


func TestCoordinateSurfacesCompletedWorkerContinuityGap(t *testing.T) {
    root:=t.TempDir()
    folder:=filepath.Join(root,"_task_mecca","data","backlog")
    runtimeDir:=filepath.Join(root,"_task_mecca",".runtime","agents")
    if err:=os.MkdirAll(folder,0755); err!=nil { t.Fatal(err) }
    if err:=os.MkdirAll(runtimeDir,0755); err!=nil { t.Fatal(err) }

    body:="# A-23 Incomplete\n- Agent: /root/controller/kkobugi\n- 변경범위: internal/backlog\n"
    if err:=os.WriteFile(filepath.Join(folder,"000023.A-23.incomplete.doing.md"),[]byte(body),0644); err!=nil { t.Fatal(err) }
    heartbeat:=[]byte(`{"agent":"/root/controller/kkobugi","task_id":"A-23","state":"completed","heartbeat_at":"2026-09-30T05:00:00Z"}`)
    if err:=os.WriteFile(filepath.Join(runtimeDir,"kkobugi.json"),heartbeat,0644); err!=nil { t.Fatal(err) }

    report,err:=Coordinate(root,"",3)
    if err!=nil { t.Fatal(err) }
    gaps:=report["continuity_gaps"].([]map[string]any)
    if len(gaps)!=1 { t.Fatalf("gaps=%+v",gaps) }
    if gaps[0]["code"]!="controller_completion_recovery" { t.Fatalf("gap=%+v",gaps[0]) }
    if report["controller_review_needed"]!=true { t.Fatalf("controller_review_needed=%v",report["controller_review_needed"]) }
}

func TestCoordinateSurfacesMissingWorkerContinuityGap(t *testing.T) {
    root:=t.TempDir()
    folder:=filepath.Join(root,"_task_mecca","data","backlog")
    runtimeDir:=filepath.Join(root,"_task_mecca",".runtime","agents")
    if err:=os.MkdirAll(folder,0755); err!=nil { t.Fatal(err) }
    if err:=os.MkdirAll(runtimeDir,0755); err!=nil { t.Fatal(err) }

    body:="# A-27 Orphaned\n- Agent: /root/controller/pairi\n- 변경범위: internal/backlog\n"
    if err:=os.WriteFile(filepath.Join(folder,"000027.A-27.orphaned.doing.md"),[]byte(body),0644); err!=nil { t.Fatal(err) }

    report,err:=Coordinate(root,"",3)
    if err!=nil { t.Fatal(err) }
    gaps:=report["continuity_gaps"].([]map[string]any)
    if len(gaps)!=1 { t.Fatalf("gaps=%+v",gaps) }
    if gaps[0]["code"]!="worker_missing_backlog_doing" { t.Fatalf("gap=%+v",gaps[0]) }
}


func TestCoordinateRecoversB415StyleOrphanedDoingWithDirtyScope(t *testing.T) {
    root:=t.TempDir()
    git:=func(args ...string) {
        cmd:=exec.Command("git",args...)
        cmd.Dir=root
        if out,err:=cmd.CombinedOutput(); err!=nil {
            t.Fatalf("git %v: %v\n%s",args,err,out)
        }
    }
    git("init")
    git("config","user.email","task-mecca@example.invalid")
    git("config","user.name","Task Mecca Test")

    srcDir:=filepath.Join(root,"src")
    if err:=os.MkdirAll(srcDir,0755); err!=nil { t.Fatal(err) }
    sourcePath:=filepath.Join(srcDir,"worker.go")
    if err:=os.WriteFile(sourcePath,[]byte("package sample\n"),0644); err!=nil { t.Fatal(err) }
    git("add","src/worker.go")
    git("commit","-m","baseline")

    folder:=filepath.Join(root,"_task_mecca","data","backlog")
    runtimeDir:=filepath.Join(root,"_task_mecca",".runtime","agents")
    if err:=os.MkdirAll(folder,0755); err!=nil { t.Fatal(err) }
    if err:=os.MkdirAll(runtimeDir,0755); err!=nil { t.Fatal(err) }
    body:="# B-415 Interrupted implementation\n- Agent: /root/controller/kkobugi\n- 변경범위: src\n"
    if err:=os.WriteFile(filepath.Join(folder,"000415.B-415.interrupted.doing.md"),[]byte(body),0644); err!=nil { t.Fatal(err) }
    heartbeat:=[]byte(`{"agent":"/root/controller/kkobugi","task_id":"B-415","state":"completed","heartbeat_at":"2026-09-30T05:00:00Z"}`)
    if err:=os.WriteFile(filepath.Join(runtimeDir,"kkobugi.json"),heartbeat,0644); err!=nil { t.Fatal(err) }

    if err:=os.WriteFile(sourcePath,[]byte("package sample\n// unfinished\n"),0644); err!=nil { t.Fatal(err) }

    report,err:=Coordinate(root,"",3)
    if err!=nil { t.Fatal(err) }
    if report["scheduling_needed"]!=true { t.Fatalf("scheduling_needed=%v",report["scheduling_needed"]) }

    gaps:=report["continuity_gaps"].([]map[string]any)
    if len(gaps)!=1 { t.Fatalf("gaps=%+v",gaps) }
    gap:=gaps[0]
    if gap["code"]!="controller_completion_recovery" { t.Fatalf("gap=%+v",gap) }
    recovery:=gap["recovery"].(map[string]any)
    if recovery["requires_fresh_preflight"]!=true { t.Fatalf("recovery=%+v",recovery) }
    if recovery["uncommitted_change_count"]!=1 { t.Fatalf("recovery=%+v",recovery) }
    changes:=recovery["uncommitted_changes"].([]string)
    if len(changes)!=1 || changes[0]!="src/worker.go" { t.Fatalf("changes=%+v",changes) }

    fill:=report["parallel_fill"].(map[string]any)
    if fill["backlog_doing"]!=1 { t.Fatalf("fill=%+v",fill) }
    if fill["active_doing"]!=0 { t.Fatalf("orphaned doing consumed a live worker slot: %+v",fill) }
    if fill["candidate_slots"]!=3 || fill["recovery_to_review_this_pass"]!=1 {
        t.Fatalf("recovery scheduling not promoted: %+v",fill)
    }
    if fill["review_required"]!=true { t.Fatalf("fill=%+v",fill) }
}
