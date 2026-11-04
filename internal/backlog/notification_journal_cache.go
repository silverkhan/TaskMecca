package backlog

import (
    "os"
    "time"
)

// All access is under notificationJournalMu. A cached journal is immutable;
// every reconciliation receives writable copies so observations cannot leak
// into subsequent scans before the atomic on-disk commit.
type notificationJournalCacheEntry struct {
    info os.FileInfo
    checked time.Time
    journal notificationJournal
}

var notificationJournalCache = map[string]notificationJournalCacheEntry{}
const notificationJournalCacheLimit = 32
const notificationJournalCheckpoint = 30*time.Second

func duplicateNotificationJournal(j notificationJournal) notificationJournal {
    out:=notificationJournal{Version:j.Version}
    if j.Items!=nil {
        out.Items=make(map[string]notificationObservation,len(j.Items))
        for id,item:=range j.Items {out.Items[id]=item}
    }
    if j.Phases!=nil {
        out.Phases=make(map[string]int,len(j.Phases))
        for id,phase:=range j.Phases {out.Phases[id]=phase}
    }
    if j.Events!=nil {
        out.Events=make([]map[string]any,len(j.Events))
        for i,event:=range j.Events {
            copyEvent:=make(map[string]any,len(event))
            for k,v:=range event {copyEvent[k]=v}
            out.Events[i]=copyEvent
        }
    }
    return out
}

// The caller must own notificationJournalMu. Externally replaced files
// invalidate via file identity, not mtime alone. Periodically force a full
// read even for an unchanged identity, as a conservative audit checkpoint.
func cachedNotificationJournal(path string) (notificationJournal,bool) {
    entry,found:=notificationJournalCache[path]
    if !found {return notificationJournal{},false}
    if time.Since(entry.checked)>=notificationJournalCheckpoint {return notificationJournal{},false}
    info,err:=os.Stat(path)
    if err!=nil || !info.Mode().IsRegular() || !os.SameFile(info,entry.info) || info.Size()!=entry.info.Size() || !info.ModTime().Equal(entry.info.ModTime()) {
        delete(notificationJournalCache,path)
        return notificationJournal{},false
    }
    return duplicateNotificationJournal(entry.journal),true
}

func rememberNotificationJournal(path string,journal notificationJournal) {
    info,err:=os.Stat(path)
    if err!=nil || !info.Mode().IsRegular() {
        delete(notificationJournalCache,path)
        return
    }
    if len(notificationJournalCache)>=notificationJournalCacheLimit {
        if _,ok:=notificationJournalCache[path];!ok{
            for old:=range notificationJournalCache { delete(notificationJournalCache,old);break }
        }
    }
    notificationJournalCache[path]=notificationJournalCacheEntry{
        info:info, checked:time.Now(), journal:duplicateNotificationJournal(journal),
    }
}
