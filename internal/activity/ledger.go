package activity

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strings"
)

var eventFileID=regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
var ErrEventConflict=errors.New("activity event id already exists with different content")

// Ledger contains immutable event files in project-owned data, not managed templates.
// Per-event no-replace hard links make concurrent writers safe without replacing a
// canonical event. A temp file is synced before publication.
type Ledger struct { Project string }

func (l Ledger) dir() string { return filepath.Join(l.Project,"_task_mecca","data","activity","events") }
func validEventID(id string) bool {return eventFileID.MatchString(id)}
func (l Ledger) path(id string) (string,error) {
    if !validEventID(id){return "",fmt.Errorf("%w: unsafe event id",ErrInvalidEvent)}
    if strings.TrimSpace(l.Project)==""{return "",errors.New("missing canonical project")}
    return filepath.Join(l.dir(),id+".json"),nil
}

func (l Ledger) Read(id string)(Event,error){
    path,err:=l.path(id);if err!=nil{return Event{},err}
    bytes,err:=os.ReadFile(path);if err!=nil{return Event{},err}
    var event Event
    if err:=json.Unmarshal(bytes,&event);err!=nil{return Event{},fmt.Errorf("corrupt activity event %s: %w",id,err)}
    if event.EventID!=id{return Event{},fmt.Errorf("activity event identity mismatch %s",id)}
    if err:=event.Validate();err!=nil{return Event{},err}
    return event,nil
}

// Append publishes a validated event exactly once. Repeating identical input is
// idempotent; conflicting reuse of an event ID is rejected, not overwritten.
func(l Ledger) Append(event Event)(bool,error){
    if err:=event.Validate();err!=nil{return false,err}
    path,err:=l.path(event.EventID);if err!=nil{return false,err}
    body,err:=json.MarshalIndent(event,"","  ");if err!=nil{return false,err}
    body=append(body,'\n')
    if err:=os.MkdirAll(l.dir(),0700);err!=nil{return false,err}
    temp,err:=os.CreateTemp(l.dir(),".activity-*.tmp")
    if err!=nil{return false,err}
    defer os.Remove(temp.Name())
    if err:=temp.Chmod(0600);err!=nil{temp.Close();return false,err}
    if _,err:=temp.Write(body);err!=nil{temp.Close();return false,err}
    if err:=temp.Sync();err!=nil{temp.Close();return false,err}
    if err:=temp.Close();err!=nil{return false,err}
    if err:=os.Link(temp.Name(),path);err!=nil{
        if !errors.Is(err,os.ErrExist){return false,err}
        existing,readErr:=l.Read(event.EventID);if readErr!=nil{return false,readErr}
        other,marshalErr:=json.MarshalIndent(existing,"","  ");if marshalErr!=nil{return false,marshalErr}
        if string(append(other,'\n'))!=string(body){return false,ErrEventConflict}
        return false,nil
    }
    return true,nil
}

func(l Ledger) List(taskID string)([]Event,error){
    if strings.TrimSpace(l.Project)==""{return nil,errors.New("missing canonical project")}
    entries,err:=os.ReadDir(l.dir())
    if errors.Is(err,os.ErrNotExist){return []Event{},nil}
    if err!=nil{return nil,err}
    events:=make([]Event,0,len(entries))
    for _,entry:=range entries{
        if entry.IsDir()||!strings.HasSuffix(entry.Name(),".json"){continue}
        id:=strings.TrimSuffix(entry.Name(),".json")
        e,err:=l.Read(id);if err!=nil{return nil,err}
        if taskID==""||e.TaskID==taskID{events=append(events,e)}
    }
    sort.Slice(events,func(i,j int)bool{
        if events[i].OccurredAt.Equal(events[j].OccurredAt){return events[i].EventID<events[j].EventID}
        return events[i].OccurredAt.Before(events[j].OccurredAt)
    })
    return events,nil
}
