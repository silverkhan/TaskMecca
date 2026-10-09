package webui

import (
 "os"
 "path/filepath"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/backlog"
 "github.com/silverkhan/TaskMecca/internal/maintenance"
)

func TestOperationMonitorStartsCanonicalEventFeedsForEveryActiveProject(t *testing.T) {
 t.Setenv("TASK_MECCA_HOME",t.TempDir())
 t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT","disabled") // never contact a real bot
 primary:=testOperationProject(t)
 secondary:=testOperationProject(t)
 paused:=testOperationProject(t)
 for _,project:=range []string{primary,secondary,paused} {
  if err:=maintenance.RegisterProject(project);err!=nil{t.Fatal(err)}
 }
 if err:=maintenance.SetProjectMonitoring(paused,false);err!=nil{t.Fatal(err)}
 t.Cleanup(func(){
  attentionFeeds.Lock()
  var closing []*attentionFeed
  for key,feed:=range attentionFeeds.feeds {
   if feed.project==primary||feed.project==secondary||feed.project==paused {
    closing=append(closing,feed)
    delete(attentionFeeds.feeds,key)
   }
  }
  attentionFeeds.Unlock()
  for _,feed:=range closing {close(feed.stopCh);<-feed.doneCh}
 })

 // No browser, EventSource, or /api/events request is ever created.
 scanOperationProjects(primary,time.Now().UTC())
 for _,project:=range []string{primary,secondary} {
  key:=project+"\x00"+filepath.Join(project,"_task_mecca","data","backlog")
  attentionFeeds.Lock()
  feed:=attentionFeeds.feeds[key]
  attentionFeeds.Unlock()
  if feed==nil{t.Fatalf("active project %q has no canonical notification feed",project)}
  deadline:=time.Now().Add(3*time.Second)
  for {
   feed.mu.Lock()
   ready:=len(feed.latest)>0
   feed.mu.Unlock()
   if ready {break}
   if time.Now().After(deadline){t.Fatalf("initial notification baseline for project %q timed out",project)}
   time.Sleep(10*time.Millisecond)
  }
  taskID:="A-7"
  path:=filepath.Join(project,"_task_mecca","data","backlog","000007.A-7.background.todo.md")
  if err:=os.WriteFile(path,[]byte("# Background task\n"),0644);err!=nil{t.Fatal(err)}
  feed.refresh() // foreground request not involved
  events,err:=backlog.ReadNotificationEvents(project)
  if err!=nil{t.Fatal(err)}
  found:=false
  for _,event:=range events {
   if event["task_id"]==taskID&&event["kind"]=="registered"{found=true}
  }
  if !found{t.Fatalf("project %q did not observe background registration: %+v",project,events)}
 }
 attentionFeeds.Lock()
 for _,feed:=range attentionFeeds.feeds {
  if feed.project==paused {attentionFeeds.Unlock();t.Fatalf("paused project has running feed: %s",paused)}
 }
 attentionFeeds.Unlock()
}
