package activity

import (
    "crypto/sha256"
    "encoding/hex"
    "errors"
    "os"
    "fmt"
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// RecordTerminal conservatively closes an Activity left open by a terminal
// Runtime Hook. It is called only on terminal state events, never every tool
// hook. The resulting interval is explicitly UNMEASURED (Abandon).
func RecordTerminal(project string,hook runtimeobs.ExecutionEvent)(bool,error){
    if !hook.Terminal || hook.EventKind!="state" || hook.AttemptID=="" {
        return false,nil
    }
    // Most projects have no Activity records yet. One cheap stat avoids
    // rebuilding the expensive Runtime ledger for such terminal hooks.
    if _,statErr:=os.Stat((Ledger{Project:project}).dir());errors.Is(statErr,os.ErrNotExist){return false,nil
    }else if statErr!=nil{return false,statErr}
    when,err:=time.Parse(time.RFC3339Nano,hook.ObservedAt)
    if err!=nil{return false,nil}
    runtimeLedger,err:=runtimeobs.BuildLedger(project,5,time.Now())
    if err!=nil{return false,err}
    var bound *runtimeobs.Attempt
    for i:=range runtimeLedger.Attempts {
        item:=&runtimeLedger.Attempts[i]
        if item.AttemptID!=hook.AttemptID {continue}
        if bound!=nil{return false,nil} // ambiguous
        bound=item
    }
    if bound==nil || !bound.Terminal || bound.BindingState!=runtimeobs.BindingBound ||
        bound.TaskID=="" || bound.RuntimeAgentID=="" || bound.RuntimeAgentID!=hook.RuntimeAgentID {
        return false,nil
    }
    var role Role
    switch {
    case bound.AgentPath=="/root/controller":role=RoleController
    case bound.AgentPath=="/root/registrar":role=RoleRegistrar
    case strings.HasPrefix(bound.AgentPath,"/root/controller/sub-"):role=RoleSubController
    case strings.HasPrefix(bound.AgentPath,"/root/controller/"):role=RoleWorker
    default:return false,nil
    }
    store:=Ledger{Project:project}
    rows,err:=store.List(bound.TaskID)
    if err!=nil{return false,err}
    var active *Event
    for i:=len(rows)-1;i>=0;i-- {
        r:=&rows[i]
        if r.Role==role && r.AgentID==bound.RuntimeAgentID && r.RuntimeAttemptID==hook.AttemptID && r.Action!=Correct {
            if r.Action==Start || r.Action==Switch {active=r}
            break
        }
    }
    if active==nil || when.Before(active.OccurredAt){return false,nil}
    sum:=sha256.Sum256([]byte("runtime-terminal\x00"+hook.AttemptID+"\x00"+hook.EventID))
    event:=Event{
        SchemaVersion:1,EventID:"terminal-"+hex.EncodeToString(sum[:12]),
        TaskID:active.TaskID,Action:Abandon,Role:active.Role,
        AgentID:active.AgentID,RuntimeAttemptID:active.RuntimeAttemptID,
        AssignmentID:active.AssignmentID,HandoffID:active.HandoffID,
        Round:active.Round,Type:active.Type,Explanation:active.Explanation,
        Confidence:Unknown,Source:"runtime_terminal",Evidence:fmt.Sprintf("runtime event %s terminal state %s",hook.EventID,hook.State),
        OccurredAt:when,RecordedAt:time.Now().UTC(),
    }
    return store.ApplyVerified(event,func()error{
        fresh,err:=runtimeobs.BuildLedger(project,5,time.Now())
        if err!=nil{return err}
        for _,attempt:=range fresh.Attempts {
            if attempt.AttemptID==hook.AttemptID && attempt.Terminal && attempt.TaskID==event.TaskID &&
                attempt.RuntimeAgentID==event.AgentID && attempt.BindingState==runtimeobs.BindingBound {return nil}
        }
        return fmt.Errorf("%w: terminal attempt no longer verified",ErrUnverifiedActor)
    })
}
