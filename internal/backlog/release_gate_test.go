package backlog

import (
    "fmt"
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestReleaseGateLifecycleRegistrationAssignmentRuntimeCompletion(t *testing.T) {
    project:=t.TempDir()
    folder:=filepath.Join(project,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(folder,0755); err!=nil { t.Fatal(err) }

    // Establish a non-empty historical baseline, then register a new task.
    // An empty first observation is intentionally not enough to distinguish
    // a fresh installation from a genuinely new task.
    baseline:=map[string]map[string]any{"RG-0":{"id":"RG-0","title":"Existing task","file_state":"done","updated_at":"2026-10-01T00:00:00Z"}}
    if _,err:=NotificationEvents(project,baseline); err!=nil { t.Fatal(err) }
    todo:=filepath.Join(folder,"000001.RG-1.release-gate.todo.md")
    if err:=os.WriteFile(todo,[]byte("# RG-1 Release gate\n- Agent: -\n"),0644); err!=nil { t.Fatal(err) }
    rows,err:=CachedCatalog(project,folder); if err!=nil { t.Fatal(err) }
    items:=map[string]map[string]any{}
    for _,row:=range rows { items[row.ID]=map[string]any{"id":row.ID,"title":row.Title,"file_state":row.State,"updated_at":row.Mtime} }
    events,err:=NotificationEvents(project,items); if err!=nil { t.Fatal(err) }
    if len(events)!=1 || events[0]["kind"]!="registered" { t.Fatalf("registration events=%+v",events) }

    // Assign and start the worker runtime.
    doing:=filepath.Join(folder,"000001.RG-1.release-gate.doing.md")
    if err:=os.Rename(todo,doing); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(doing,[]byte("# RG-1 Release gate\n- Agent: /root/controller/pairi\n- RuntimeProvider: codex\n"),0644); err!=nil { t.Fatal(err) }
    base:=time.Now().UTC()
    start:=runtimeobs.ExecutionEvent{EventKind:"state",ObservedAt:base.Format(time.RFC3339Nano),AttemptID:"release-gate-run",Provider:"codex",RuntimeAgentID:"pairi-runtime",State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved}
    if err:=runtimeobs.AppendExecutionEvent(project,start); err!=nil { t.Fatal(err) }
    if _,err:=runtimeobs.BindAttempt(project,start.AttemptID,"RG-1","/root/controller/pairi","dispatch","",map[string]string{"gate":"release-0"},base.Add(time.Millisecond)); err!=nil { t.Fatal(err) }

    rows,err=Catalog(project,folder); if err!=nil { t.Fatal(err) }
    timings,err:=lifecycleTimings(project,folder,rows); if err!=nil { t.Fatal(err) }
    if timings["RG-1"]==nil || timings["RG-1"]["started_at"]==nil { t.Fatalf("runtime start missing: %+v",timings["RG-1"]) }

    // Runtime completes, then durable backlog completion finalizes the lifecycle.
    stop:=start
    stop.ObservedAt=base.Add(time.Second).Format(time.RFC3339Nano)
    stop.State=runtimeobs.StateCompleted
    stop.Terminal=true
    stop.EventID=""
    if err:=runtimeobs.AppendExecutionEvent(project,stop); err!=nil { t.Fatal(err) }
    done:=filepath.Join(folder,"000001.RG-1.release-gate.done.md")
    if err:=os.Rename(doing,done); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(done,[]byte("# RG-1 Release gate\n- Agent: /root/controller/pairi\n- RuntimeProvider: codex\n- 결과: done\n- 검증: release gate\n"),0644); err!=nil { t.Fatal(err) }

    snapshot,err:=AttentionSnapshot(project,folder,true); if err!=nil { t.Fatal(err) }
    notificationRows:=snapshot["notification_events"].([]map[string]any)
    completed:=0
    for _,event:=range notificationRows { if event["task_id"]=="RG-1" && event["kind"]=="completed" { completed++ } }
    if completed!=1 { t.Fatalf("completion notification count=%d events=%+v",completed,notificationRows) }

    again,err:=AttentionSnapshot(project,folder,true); if err!=nil { t.Fatal(err) }
    duplicate:=0
    for _,event:=range again["notification_events"].([]map[string]any) { if event["task_id"]=="RG-1" && event["kind"]=="completed" { duplicate++ } }
    if duplicate!=1 { t.Fatalf("completion event identity changed after refresh: %+v",again["notification_events"]) }
}

func TestReleaseGateLargeBacklogBaseline(t *testing.T) {
    project:=t.TempDir()
    folder:=filepath.Join(project,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(folder,0755); err!=nil { t.Fatal(err) }
    for i:=1;i<=500;i++ {
        state:="todo"
        if i%7==0 { state="done" }
        name:=fmt.Sprintf("%06d.RG-%d.large.%s.md",i,i,state)
        body:=fmt.Sprintf("# RG-%d Large backlog task\n- Tags: area:test\n- 설명: performance fixture %d\n",i,i)
        if err:=os.WriteFile(filepath.Join(folder,name),[]byte(body),0644); err!=nil { t.Fatal(err) }
    }
    started:=time.Now()
    page,err:=BacklogPage(project,folder,1,20,nil,nil,"","id_desc")
    if err!=nil { t.Fatal(err) }
    if page["total"]!=500 { t.Fatalf("total=%v",page["total"]) }
    if len(page["items"].([]map[string]any))!=20 { t.Fatalf("page size=%d",len(page["items"].([]map[string]any))) }
    if elapsed:=time.Since(started); elapsed>5*time.Second { t.Fatalf("500-item backlog baseline too slow: %s",elapsed) }

    started=time.Now()
    if _,err:=AttentionSnapshot(project,folder,true); err!=nil { t.Fatal(err) }
    if elapsed:=time.Since(started); elapsed>5*time.Second { t.Fatalf("500-item attention baseline too slow: %s",elapsed) }
}
