package backlog

import (
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestLifecycleRejectsRuntimeStartBeforeRegistration(t *testing.T) {
    project:=t.TempDir()
    backlogDir:=filepath.Join(project,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlogDir,0755); err!=nil { t.Fatal(err) }
    task:=filepath.Join(backlogDir,"0001.B-436.test.doing.md")
    body:="---\nID: B-436\nTitle: Test\nAgent: /root/controller/pairi\nRuntimeProvider: codex\n---\n"
    if err:=os.WriteFile(task,[]byte(body),0644); err!=nil { t.Fatal(err) }

    now:=time.Now().UTC()
    oldStart:=now.Add(-2*time.Hour)
    attemptID:="run-lifecycle-old"
    start:=runtimeobs.ExecutionEvent{EventKind:"state",ObservedAt:oldStart.Format(time.RFC3339Nano),AttemptID:attemptID,Provider:"codex",RuntimeAgentID:"agent-old",State:runtimeobs.StateRunning}
    if err:=runtimeobs.AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    if _,err:=runtimeobs.BindAttempt(project,attemptID,"B-436","/root/controller/pairi","dispatch","",map[string]string{"test":"stale"},now); err!=nil { t.Fatal(err) }

    rows,err:=Catalog(project,"")
    if err!=nil { t.Fatal(err) }
    timings,err:=lifecycleTimings(project,"",rows)
    if err!=nil { t.Fatal(err) }
    lifecycle:=timings["B-436"]
    if lifecycle==nil { t.Fatal("missing lifecycle") }
    if started:=lifecycle["started_at"]; started!=nil {
        if parsed,ok:=parseTime(started.(string)); ok && parsed.Before(now.Add(-time.Minute)) {
            t.Fatalf("lifecycle was backdated before registration/binding: %v",started)
        }
    }
}


func TestCompletedLifecycleKeepsStartedAtFromImmutableEpisode(t *testing.T) {
    project:=t.TempDir()
    backlogDir:=filepath.Join(project,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlogDir,0755); err!=nil { t.Fatal(err) }
    task:=filepath.Join(backlogDir,"0441.B-441.lifecycle.doing.md")
    body:="---\nID: B-441\nTitle: Lifecycle preservation\nAgent: /root/controller/worker\nRuntimeProvider: codex\n---\n"
    if err:=os.WriteFile(task,[]byte(body),0644); err!=nil { t.Fatal(err) }
    started:=time.Now().UTC().Add(-4*time.Minute)
    completed:=started.Add(3*time.Minute)
    attemptID:="run-b441"
    start:=runtimeobs.ExecutionEvent{EventKind:"state",ObservedAt:started.Format(time.RFC3339Nano),AttemptID:attemptID,Provider:"codex",SessionID:"root-1",RuntimeAgentID:"worker-1",State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved}
    if err:=runtimeobs.AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    if _,err:=runtimeobs.BindAttempt(project,attemptID,"B-441","/root/controller/worker","dispatch","",nil,started); err!=nil { t.Fatal(err) }
    activeRows,err:=Catalog(project,""); if err!=nil { t.Fatal(err) }
    activeTimings,err:=lifecycleTimings(project,"",activeRows); if err!=nil { t.Fatal(err) }
    if activeTimings["B-441"]==nil || activeTimings["B-441"]["started_at"]==nil { t.Fatalf("active lifecycle missing start: %+v",activeTimings["B-441"]) }
    stop:=runtimeobs.ExecutionEvent{EventKind:"state",ObservedAt:completed.Format(time.RFC3339Nano),AttemptID:attemptID,Provider:"codex",SessionID:"root-1",RuntimeAgentID:"worker-1",State:runtimeobs.StateCompleted,Terminal:true,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved}
    if err:=runtimeobs.AppendExecutionEvent(project,stop); err!=nil { t.Fatal(err) }
    doneTask:=filepath.Join(backlogDir,"0441.B-441.lifecycle.done.md")
    if err:=os.Rename(task,doneTask); err!=nil { t.Fatal(err) }
    rows,err:=Catalog(project,""); if err!=nil { t.Fatal(err) }
    timings,err:=lifecycleTimings(project,"",rows); if err!=nil { t.Fatal(err) }
    lifecycle:=timings["B-441"]; if lifecycle==nil { t.Fatal("missing lifecycle") }
    if lifecycle["started_at"]==nil || lifecycle["completed_at"]==nil { t.Fatalf("completed lifecycle incomplete: %+v",lifecycle) }
    if incomplete,ok:=lifecycle["timing_incomplete"].(bool); ok && incomplete { t.Fatalf("completed lifecycle incorrectly incomplete: %+v",lifecycle) }
}
