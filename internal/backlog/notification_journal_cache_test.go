package backlog

import (
 "encoding/json"
 "os"
 "path/filepath"
 "testing"
 "time"
)

func TestNotificationJournalCacheInvalidateOnExternalAtomicReplacement(t *testing.T) {
 project:=t.TempDir()
 items:=map[string]map[string]any{"A-7":{"id":"A-7","title":"Task","file_state":"doing","updated_at":"2026-11-04T00:00:00Z"}}
 if _,err:=NotificationEvents(project,items);err!=nil{t.Fatal(err)}
 path:=filepath.Join(project,"_task_mecca",".runtime","notification_events.json")
 notificationJournalMu.Lock()
 _,hit:=cachedNotificationJournal(path)
 notificationJournalMu.Unlock()
 if !hit{t.Fatal("unchanged on-disk journal should hit cache")}
 before,err:=os.Stat(path);if err!=nil{t.Fatal(err)}
 raw,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
 var updated notificationJournal
 if err=json.Unmarshal(raw,&updated);err!=nil{t.Fatal(err)}
 updated.Events=append(updated.Events,map[string]any{"id":"external-event","task_id":"A-7","kind":"registered"})
 contents,err:=json.Marshal(updated);if err!=nil{t.Fatal(err)}
 replacement:=path+".external"
 if err=os.WriteFile(replacement,contents,0644);err!=nil{t.Fatal(err)}
 // Make the replacement deliberately look like the old file in mtime.
 if err=os.Chtimes(replacement,before.ModTime(),before.ModTime());err!=nil{t.Fatal(err)}
 if err=os.Rename(replacement,path);err!=nil{t.Fatal(err)}
 notificationJournalMu.Lock()
 _,hit=cachedNotificationJournal(path)
 notificationJournalMu.Unlock()
 if hit{t.Fatal("atomic external replacement must invalidate source identity")}
 events,err:=ReadNotificationEvents(project);if err!=nil{t.Fatal(err)}
 if len(events)!=1 || events[0]["id"]!="external-event" {t.Fatalf("external event invisible: %#v",events)}
}

func TestNotificationJournalCacheIsolationAndCommit(t *testing.T) {
 project:=t.TempDir()
 doing:=map[string]map[string]any{"A-11":{"id":"A-11","title":"Fresh","file_state":"doing","updated_at":"2026-11-04T00:00:00Z"}}
 if _,err:=NotificationEvents(project,doing);err!=nil{t.Fatal(err)}
 path:=filepath.Join(project,"_task_mecca",".runtime","notification_events.json")
 notificationJournalMu.Lock()
 cached,hit:=cachedNotificationJournal(path)
 if !hit{notificationJournalMu.Unlock();t.Fatal("expected first cache hit")}
 cached.Items["A-11"]=notificationObservation{FileState:"done"}
 cached.Phases["A-11"]=40
 cached.Events=append(cached.Events,map[string]any{"id":"injected"})
 notificationJournalMu.Unlock()
 done:=map[string]map[string]any{"A-11":{"id":"A-11","title":"Fresh","file_state":"done","updated_at":"2026-11-04T00:01:00Z","completed_at":"2026-11-04T00:01:00Z"}}
 events,err:=NotificationEvents(project,done)
 if err!=nil||len(events)!=1||events[0]["kind"]!="completed" {t.Fatalf("mutable cache leaked observation: %v %v",events,err)}
 notificationJournalMu.Lock()
 replacement,hit:=cachedNotificationJournal(path)
 notificationJournalMu.Unlock()
 if !hit || replacement.Items["A-11"].FileState!="done"{t.Fatalf("new commit not published: hit=%v %#v",hit,replacement.Items)}
 again,err:=NotificationEvents(project,done)
 if err!=nil||len(again)!=1||again[0]["id"]!=events[0]["id"]{t.Fatalf("duplicate completion: %#v err=%v",again,err)}
}

func TestNotificationJournalCheckpointFallsBackToDisk(t *testing.T) {
 project:=t.TempDir()
 items:=map[string]map[string]any{"A-1":{"file_state":"doing","updated_at":"2026-11-04T00:00:00Z"}}
 if _,err:=NotificationEvents(project,items);err!=nil{t.Fatal(err)}
 path:=filepath.Join(project,"_task_mecca",".runtime","notification_events.json")
 notificationJournalMu.Lock()
 entry:=notificationJournalCache[path]
 entry.checked=time.Now().Add(-notificationJournalCheckpoint-time.Second)
 notificationJournalCache[path]=entry
 _,hit:=cachedNotificationJournal(path)
 notificationJournalMu.Unlock()
 if hit{t.Fatal("checkpoint did not force fresh parse")}
 if _,err:=NotificationEvents(project,items);err!=nil{t.Fatal(err)}
 notificationJournalMu.Lock()
 _,hit=cachedNotificationJournal(path)
 notificationJournalMu.Unlock()
 if !hit{t.Fatal("checkpoint did not repopulate cache")}
}

func TestNotificationFastPathDoesNotHideLateStartOrConditionReset(t *testing.T) {
 project:=t.TempDir()
 item:=map[string]any{"id":"A-55","title":"Active","file_state":"doing","updated_at":"2026-11-04T00:00:00Z"}
 rows:=map[string]map[string]any{"A-55":item}
 if initial,err:=NotificationEvents(project,rows);err!=nil||len(initial)!=0 {t.Fatalf("baseline=%v err=%v",initial,err)}
 for i:=0;i<3;i++ {if events,err:=NotificationEvents(project,rows);err!=nil||len(events)!=0{t.Fatalf("unchanged repeat %d: %v %v",i,events,err)}}
 // A runtime-start observation can appear without a backlog file change.
 item["lifecycle"]=map[string]any{"started_at":"2026-11-04T00:00:30Z"}
 started,err:=NotificationEvents(project,rows)
 if err!=nil||len(started)!=1||started[0]["kind"]!="started"{t.Fatalf("late Hook start invisible: %v %v",started,err)}
 // A first actionable episode must be emitted, then a cleared condition
 // may reappear with the same identifier as a separate notification episode.
 item["notification_condition"]=map[string]any{"kind":"stalled","key":"worker:stalled","message":"no activity"}
 first,err:=NotificationEvents(project,rows)
 if err!=nil||len(first)!=2 {t.Fatalf("condition invisible: %v %v",first,err)}
 item["notification_condition"]=nil
 if _,err:=NotificationEvents(project,rows);err!=nil{t.Fatal(err)}
 item["notification_condition"]=map[string]any{"kind":"stalled","key":"worker:stalled","message":"no activity"}
 again,err:=NotificationEvents(project,rows)
 if err!=nil||len(again)!=3 {t.Fatalf("new episode suppressed by consumed cache: %v %v",again,err)}
}
