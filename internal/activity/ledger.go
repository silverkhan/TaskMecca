package activity

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "reflect"
    "sort"
    "strings"
    "github.com/silverkhan/TaskMecca/internal/projectguard"
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
    release,guardErr:=projectguard.AcquireWrite(l.Project)
    if guardErr!=nil{return false,guardErr}
    defer release()
    unlock,lockErr:=acquireActivityLock(l.Project)
    if lockErr!=nil{return false,lockErr}
    defer unlock()
    return l.appendLocked(event)
}

// appendLocked is only called while the canonical project write gate is held.
func(l Ledger) appendLocked(event Event)(bool,error){
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
    invalidateProjection(l.Project)
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

// Apply serializes the activity state transition with the append under one
// canonical project lock. Two concurrent start/switch/stop commands must not
// both approve a transition based on the same stale open interval.
func(l Ledger) Apply(event Event)(bool,error){
    return l.ApplyVerified(event,nil)
}

// ApplyVerified checks authoritative Runtime/assignment evidence inside the
// same project write gate as transition validation and immutable publication.
// verify MUST only read canonical state; it must not acquire a write lock.
func(l Ledger) ApplyVerified(event Event,verify func()error)(bool,error){
    release,err:=projectguard.AcquireWrite(l.Project)
    if err!=nil{return false,err}
    defer release()
    unlock,lockErr:=acquireActivityLock(l.Project)
    if lockErr!=nil{return false,lockErr}
    defer unlock()
    if err:=event.Validate();err!=nil{return false,err}
    if verify!=nil {
        if err:=verify();err!=nil{return false,err}
    }
    prior,err:=l.Read(event.EventID)
    if err==nil{
        // RecordedAt is assigned by the recorder. Retry with the same stable
        // event ID is successful only if immutable caller supplied data agrees.
        if sameActivityIntent(prior,event){return false,nil}
        return false,ErrEventConflict
    }
    if !errors.Is(err,os.ErrNotExist){return false,err}
    events,err:=l.List(event.TaskID)
    if err!=nil{return false,err}
    if event.Action==Correct{
        target,targetErr:=l.Read(event.SupersedesID)
        if targetErr!=nil{return false,fmt.Errorf("%w: correction target unavailable: %v",ErrInvalidEvent,targetErr)}
        if target.Action==Abandon || target.TaskID!=event.TaskID || target.Role!=event.Role ||
            target.AgentID!=event.AgentID || target.RuntimeAttemptID!=event.RuntimeAttemptID ||
            target.AssignmentID!=event.AssignmentID || target.HandoffID!=event.HandoffID{
            return false,fmt.Errorf("%w: correction cannot change event ownership or terminal hook",ErrInvalidEvent)
        }
        for _,prior:=range events {
            if prior.Action==Correct && prior.SupersedesID==event.SupersedesID {
                return false,fmt.Errorf("%w: correction target already superseded",ErrEventConflict)
            }
        }
        _,findings:=EffectiveEvents(append(events,event))
        if len(findings)>0{return false,fmt.Errorf("%w: unresolved correction chain",ErrInvalidEvent)}
        before:=Summarize(event.TaskID,events)
        after:=Summarize(event.TaskID,append(events,event))
        if after.InvalidTransitions>before.InvalidTransitions {
            return false,fmt.Errorf("%w: corrected activity chronology invalid",ErrInvalidEvent)
        }
        return l.appendLocked(event)
    }
    // Only apply transitions to the effective immutable projection.
    // Corrections remain in the audit List but cannot reopen a closed attempt.
    events,_=EffectiveEvents(events)
    var latest *Event
    for i:=range events{
        e:=&events[i]
        if e.Action==Correct || actorKey(*e)!=actorKey(event){continue}
        if latest==nil || e.OccurredAt.After(latest.OccurredAt) || (e.OccurredAt.Equal(latest.OccurredAt) && e.RecordedAt.After(latest.RecordedAt)){
            latest=e
        }
    }
    open:=latest!=nil && latest.Action!=Stop && latest.Action!=Abandon
    switch event.Action{
    case Start:
        if open{return false,fmt.Errorf("%w: attempt already has an open activity",ErrInvalidEvent)}
    case Switch,Stop,Abandon:
        if !open{return false,fmt.Errorf("%w: no open activity for attempt",ErrInvalidEvent)}
        if event.OccurredAt.Before(latest.OccurredAt){return false,fmt.Errorf("%w: time travel",ErrInvalidEvent)}
        if (event.Action==Stop || event.Action==Abandon) && (event.Type!=latest.Type || event.Round!=latest.Round){
            return false,fmt.Errorf("%w: stop must match active type and round",ErrInvalidEvent)
        }
    default:
        return false,fmt.Errorf("%w: unsupported transition",ErrInvalidEvent)
    }
    return l.appendLocked(event)
}

func sameActivityIntent(a,b Event)bool{
    return a.EventID==b.EventID && a.TaskID==b.TaskID && a.Action==b.Action &&
        a.Role==b.Role && a.AgentID==b.AgentID && a.RuntimeAttemptID==b.RuntimeAttemptID &&
        a.AssignmentID==b.AssignmentID && a.HandoffID==b.HandoffID &&
        a.Type==b.Type && a.Round==b.Round &&
        a.Evidence==b.Evidence && a.Explanation==b.Explanation &&
        a.ProposedType==b.ProposedType && a.ReviewOutcome==b.ReviewOutcome &&
        a.Source==b.Source && a.PhaseEvent==b.PhaseEvent && a.Confidence==b.Confidence &&
        a.Provider==b.Provider && a.SessionID==b.SessionID &&
        a.ContractSHA256==b.ContractSHA256 && a.TaskRevision==b.TaskRevision &&
        a.Generation==b.Generation && a.ParentActivityID==b.ParentActivityID &&
        reflect.DeepEqual(a.RunEvidence,b.RunEvidence)
}
