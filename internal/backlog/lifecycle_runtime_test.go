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
    backlogDir:=filepath.Join(project,"_task_mecca","data","backlog","doing")
    if err:=os.MkdirAll(backlogDir,0755); err!=nil { t.Fatal(err) }
    task:=filepath.Join(backlogDir,"B-436-test.md")
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
