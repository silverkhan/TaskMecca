package webui

import (
    "path/filepath"
    "testing"
    "time"
)

// Browser subscriptions are delivery-only resources. A durable monitoring
// feed must survive the last tab closing, whereas a UI-only alternate-root
// projection must disappear immediately on unsubscribe.
func TestAttentionFeedOwnerLifecycle(t *testing.T) {
    project:=testOperationProject(t)
    root:=filepath.Join(project,"_task_mecca","data","backlog")
    other:=filepath.Join(project,"alternate")
    t.Cleanup(func(){stopProjectAttentionFeeds(project)})

    durable:=ensureDurableAttentionFeed(project,root)
    again:=ensureDurableAttentionFeed(project,root)
    if again!=durable {t.Fatal("duplicate durable feed for one root")}
    ch1,_,cancel1:=durable.subscribe()
    _,_,cancel2:=durable.subscribe()
    cancel1()
    // Buffered attention data may precede the close. Drain until EOF.
    drained := false
    for !drained {
        select { case _,ok:=<-ch1: drained = !ok
        case <-time.After(5*time.Second): t.Fatal("subscriber not closed") }
    }
    cancel2()
    attentionFeeds.Lock()
    got:=attentionFeeds.feeds[project+"\x00"+root]
    attentionFeeds.Unlock()
    if got!=durable {t.Fatal("browser disconnect stopped durable monitor")}

    ui:=ensureAttentionFeed(project,other)
    if ui==durable {t.Fatal("alternate projection alias")}
    _,_,cancelUI:=ui.subscribe()
    cancelUI()
    attentionFeeds.Lock()
    _,exists:=attentionFeeds.feeds[project+"\x00"+other]
    attentionFeeds.Unlock()
    if exists {t.Fatal("last UI-only subscriber left feed running")}
    select {case <-ui.doneCh:case <-time.After(5*time.Second):t.Fatal("UI feed did not terminate")}

    stopProjectAttentionFeeds(project)
    attentionFeeds.Lock()
    _,exists=attentionFeeds.feeds[project+"\x00"+root]
    attentionFeeds.Unlock()
    if exists {t.Fatal("paused project retained monitoring feed")}
    select {case <-durable.doneCh:case <-time.After(5*time.Second):t.Fatal("durable feed did not stop")}

    resumed:=ensureDurableAttentionFeed(project,root)
    if resumed==durable {t.Fatal("resuming reused stopped feed")}
}

func TestAttentionFeedCanonicalRootSwitchKeepsOnlyNeededStreams(t *testing.T) {
    project:=testOperationProject(t)
    t.Cleanup(func(){stopProjectAttentionFeeds(project)})
    first:=ensureDurableAttentionFeed(project,"")
    _,_,cancel:=first.subscribe()
    second:=ensureDurableAttentionFeed(project,"other")
    if first==second {t.Fatal("root switch did not create new canonical feed")}
    attentionFeeds.Lock()
    countDurable:=0
    for _,f:=range attentionFeeds.feeds {
        if f.project==project && f.durable {countDurable++}
    }
    attentionFeeds.Unlock()
    if countDurable!=1 {t.Fatalf("got %d durable loops, want exactly one",countDurable)}
    cancel()
    attentionFeeds.Lock()
    _,still:=attentionFeeds.feeds[project+"\x00"]
    attentionFeeds.Unlock()
    if still {t.Fatal("old root survived after its final subscriber left")}
    select {case <-first.doneCh:case <-time.After(5*time.Second):t.Fatal("former canonical feed did not exit")}
}

func TestAttentionFeedPrunesUnmonitoredProjects(t *testing.T) {
    p1:=testOperationProject(t)
    p2:=testOperationProject(t)
    t.Cleanup(func(){stopProjectAttentionFeeds(p1);stopProjectAttentionFeeds(p2)})
    one:=ensureDurableAttentionFeed(p1,"")
    two:=ensureDurableAttentionFeed(p2,"")
    reconcileAttentionFeedProjects([]string{p1})
    attentionFeeds.Lock()
    _,present:=attentionFeeds.feeds[p1+"\x00"]
    _,absent:=attentionFeeds.feeds[p2+"\x00"]
    attentionFeeds.Unlock()
    if !present||absent {t.Fatalf("registry reconciliation retained wrong feeds: active=%v removed=%v",present,absent)}
    select{case <-two.doneCh:case <-time.After(5*time.Second):t.Fatal("removed project feed did not terminate")}
    select{case <-one.doneCh:t.Fatal("active project's feed unexpectedly stopped");default:}
}
