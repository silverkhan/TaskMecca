package webui

import (
 "path/filepath"
 "testing"
 "github.com/silverkhan/TaskMecca/internal/maintenance"
)

func TestAttentionRefreshReportsSuppressedInitialAttempt(t *testing.T){
 t.Setenv("TASK_MECCA_HOME",t.TempDir())
 project:=testOperationProject(t)
 if err:=maintenance.RegisterProject(project);err!=nil{t.Fatal(err)}
 entered:=make(chan struct{})
 release:=make(chan struct{})
 ended:=make(chan struct{})
 go func(){
  maintenance.WithProjectMonitoring(project,func(){close(entered);<-release})
  close(ended)
 }()
 <-entered
 feed:=&attentionFeed{
  project:project,root:filepath.Join(project,"_task_mecca","data","backlog"),
  subscribers:map[chan []byte]struct{}{},
 }
 if feed.tryRefresh(){close(release);<-ended;t.Fatal("background refresh bypassed active foreground lock")}
 close(release);<-ended
 if !feed.tryRefresh(){t.Fatal("released monitoring gate still suppressed refresh")}
}
