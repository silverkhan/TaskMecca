package webui

import (
    "encoding/json"
    "fmt"
    "sort"
    "strings"
    "sync"
    "time"

    "github.com/silverkhan/TaskMecca/internal/backlog"
)

type attentionFeed struct {
    project string
    root string
    mu sync.Mutex
    latest []byte
    revision string
    subscribers map[chan []byte]struct{}
}

var attentionFeeds = struct {
    sync.Mutex
    feeds map[string]*attentionFeed
}{feeds:map[string]*attentionFeed{}}

func attentionRevision(payload map[string]any) string {
    parts:=[]string{}
    if rows,ok:=payload["attention"].([]map[string]any); ok {
        for _,row:=range rows {
            parts=append(parts,strings.Join([]string{
                fmt.Sprint(row["id"]),fmt.Sprint(row["type"]),fmt.Sprint(row["health"]),
                fmt.Sprint(row["runtime_state"]),fmt.Sprint(row["last_activity_at"]),
                fmt.Sprint(row["title"]),fmt.Sprint(row["message"]),fmt.Sprint(row["action"]),
                fmt.Sprint(row["resume_condition"]),fmt.Sprint(row["evidence"]),
            },"|"))
        }
    }
    if events,ok:=payload["notification_events"].([]map[string]any); ok {
        for _,event:=range events { parts=append(parts,"event:"+fmt.Sprint(event["id"])) }
    }
    sort.Strings(parts)
    return strings.Join(parts,"\n")
}

func ensureAttentionFeed(project,root string) *attentionFeed {
    key:=project+"\x00"+root
    attentionFeeds.Lock()
    if feed:=attentionFeeds.feeds[key]; feed!=nil { attentionFeeds.Unlock(); return feed }
    feed:=&attentionFeed{project:project,root:root,subscribers:map[chan []byte]struct{}{}}
    attentionFeeds.feeds[key]=feed
    attentionFeeds.Unlock()
    go feed.run()
    return feed
}

func (f *attentionFeed) refresh() {
    payload,err:=backlog.AttentionSnapshot(f.project,f.root,true)
    if err!=nil { payload=map[string]any{"error":err.Error(),"attention":[]map[string]any{},"all_items":map[string]map[string]any{},"notification_events":[]map[string]any{}} }
    revision:=attentionRevision(payload)
    data,_:=json.Marshal(payload)
    f.mu.Lock()
    changed:=revision!=f.revision || f.latest==nil
    f.revision=revision
    f.latest=append([]byte{},data...)
    subscribers:=make([]chan []byte,0,len(f.subscribers))
    if changed { for ch:=range f.subscribers { subscribers=append(subscribers,ch) } }
    f.mu.Unlock()
    if changed {
        for _,ch:=range subscribers {
            select { case ch<-append([]byte{},data...): default: }
        }
    }
}

func (f *attentionFeed) run() {
    f.refresh()
    ticker:=time.NewTicker(3*time.Second)
    defer ticker.Stop()
    for range ticker.C { f.refresh() }
}

func (f *attentionFeed) subscribe() (chan []byte,[]byte,func()) {
    ch:=make(chan []byte,4)
    f.mu.Lock()
    f.subscribers[ch]=struct{}{}
    initial:=append([]byte{},f.latest...)
    f.mu.Unlock()
    cancel:=func(){
        f.mu.Lock()
        if _,ok:=f.subscribers[ch]; ok { delete(f.subscribers,ch); close(ch) }
        f.mu.Unlock()
    }
    return ch,initial,cancel
}
