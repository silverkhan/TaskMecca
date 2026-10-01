package runtimeobs

import (
    "bufio"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "time"
)

type CanonicalState string
type BindingState string
type EvidenceSource string
type ObservationQuality string

const (
    StateRuntimeUnknown CanonicalState = "runtime_unknown"
    StateStarting CanonicalState = "starting"
    StateRunning CanonicalState = "running"
    StateWaitingUser CanonicalState = "waiting_user"
    StateWaitingApproval CanonicalState = "waiting_approval"
    StateInterrupted CanonicalState = "interrupted"
    StateCompleted CanonicalState = "completed"
    StateErrored CanonicalState = "errored"
    StateShutdown CanonicalState = "shutdown"

    BindingUnbound BindingState = "unbound"
    BindingBound BindingState = "bound"
    BindingAmbiguous BindingState = "ambiguous"

    EvidenceHook EvidenceSource = "hook"
    EvidenceManualBinding EvidenceSource = "manual_binding"
    EvidenceReconciled EvidenceSource = "reconciled"

    QualityAuthoritative ObservationQuality = "authoritative"
    QualityObserved ObservationQuality = "observed"
    QualityInferred ObservationQuality = "inferred"
)

type ExecutionEvent struct {
    EventID string `json:"event_id"`
    EventKind string `json:"event_kind"`
    ObservedAt string `json:"observed_at"`
    AttemptID string `json:"attempt_id"`
    Provider string `json:"provider,omitempty"`
    SessionID string `json:"session_id,omitempty"`
    TurnID string `json:"turn_id,omitempty"`
    RuntimeAgentID string `json:"runtime_agent_id,omitempty"`
    AgentType string `json:"agent_type,omitempty"`
    State CanonicalState `json:"state,omitempty"`
    Terminal bool `json:"terminal,omitempty"`
    HookEventName string `json:"hook_event_name,omitempty"`
    ToolName string `json:"tool_name,omitempty"`
    ToolUseID string `json:"tool_use_id,omitempty"`
    Reason string `json:"reason,omitempty"`
    EvidenceSource EvidenceSource `json:"evidence_source"`
    ObservationQuality ObservationQuality `json:"observation_quality"`
    RawSHA256 string `json:"raw_sha256,omitempty"`
    TaskID string `json:"task_id,omitempty"`
    AgentPath string `json:"agent_path,omitempty"`
    ParentAttemptID string `json:"parent_attempt_id,omitempty"`
    BindingSource string `json:"binding_source,omitempty"`
    BindingEvidence map[string]string `json:"binding_evidence,omitempty"`
}

type Transition struct {
    At string `json:"at"`
    State CanonicalState `json:"state,omitempty"`
    Kind string `json:"kind"`
    EvidenceSource EvidenceSource `json:"evidence_source"`
    ObservationQuality ObservationQuality `json:"observation_quality"`
    Reason string `json:"reason,omitempty"`
}

type Attempt struct {
    AttemptID string `json:"attempt_id"`
    Provider string `json:"provider"`
    SessionID string `json:"session_id,omitempty"`
    TurnID string `json:"turn_id,omitempty"`
    RuntimeAgentID string `json:"runtime_agent_id,omitempty"`
    AgentType string `json:"agent_type,omitempty"`
    TaskID string `json:"task_id,omitempty"`
    AgentPath string `json:"agent_path,omitempty"`
    ParentAttemptID string `json:"parent_attempt_id,omitempty"`
    BindingState BindingState `json:"binding_state"`
    BindingSource string `json:"binding_source,omitempty"`
    BindingEvidence map[string]string `json:"binding_evidence,omitempty"`
    CurrentState CanonicalState `json:"current_state"`
    Terminal bool `json:"terminal"`
    StartedAt string `json:"started_at,omitempty"`
    LastActivityAt string `json:"last_activity_at,omitempty"`
    EndedAt string `json:"ended_at,omitempty"`
    LastObservedAt string `json:"last_observed_at,omitempty"`
    ElapsedMillis int64 `json:"elapsed_ms"`
    ObservedActiveMillis *int64 `json:"observed_active_ms,omitempty"`
    WaitingMillis int64 `json:"waiting_ms"`
    ActiveTimeAvailable bool `json:"active_time_available"`
    ActiveTimeNote string `json:"active_time_note,omitempty"`
    StateEvidenceSource EvidenceSource `json:"state_evidence_source,omitempty"`
    StateObservationQuality ObservationQuality `json:"state_observation_quality,omitempty"`
    EvidenceCount int `json:"evidence_count"`
    ActivityCount int `json:"activity_count"`
    RecentTransitions []Transition `json:"recent_transitions"`
}

type LedgerFinding struct {
    Severity string `json:"severity"`
    Code string `json:"code"`
    AttemptID string `json:"attempt_id,omitempty"`
    Message string `json:"message"`
}

type Ledger struct {
    Version int `json:"version"`
    GeneratedAt string `json:"generated_at"`
    JournalPath string `json:"journal_path"`
    Attempts []Attempt `json:"attempts"`
    Findings []LedgerFinding `json:"findings"`
}

func ExecutionJournalPath(project string) string {
    return filepath.Join(project, "_task_mecca", ".runtime", "executions", "events.jsonl")
}

func attemptIDFor(provider, sessionID, agentID string) string {
    key:=strings.ToLower(strings.TrimSpace(provider))+"\x00"+strings.TrimSpace(sessionID)+"\x00"+strings.TrimSpace(agentID)
    sum:=sha256.Sum256([]byte(key))
    return "run-"+hex.EncodeToString(sum[:])[:16]
}

func eventIDFor(e ExecutionEvent) string {
    parts:=[]string{e.EventKind,e.Provider,e.SessionID,e.TurnID,e.RuntimeAgentID,e.HookEventName,e.ToolUseID,string(e.State),e.RawSHA256,e.TaskID,e.AgentPath,e.ParentAttemptID,e.BindingSource}
    if e.RawSHA256=="" { parts=append(parts,e.ObservedAt) }
    if e.EventKind=="binding" {
        keys:=make([]string,0,len(e.BindingEvidence))
        for k:=range e.BindingEvidence { keys=append(keys,k) }
        sort.Strings(keys)
        for _,k:=range keys { parts=append(parts,k+"="+e.BindingEvidence[k]) }
    }
    sum:=sha256.Sum256([]byte(strings.Join(parts,"\x00")))
    return hex.EncodeToString(sum[:])
}

func HookToExecutionEvent(e SpikeEvent) (ExecutionEvent,error) {
    provider:=strings.ToLower(strings.TrimSpace(e.Provider))
    if provider!="codex" && provider!="claude" { return ExecutionEvent{},fmt.Errorf("unsupported provider %q",provider) }
    if strings.TrimSpace(e.AgentID)=="" { return ExecutionEvent{},errors.New("hook event does not include agent_id") }
    out:=ExecutionEvent{
        ObservedAt:e.ObservedAt,Provider:provider,SessionID:e.SessionID,TurnID:e.TurnID,
        RuntimeAgentID:e.AgentID,AgentType:e.AgentType,AttemptID:attemptIDFor(provider,e.SessionID,e.AgentID),
        HookEventName:e.HookEventName,ToolName:e.ToolName,ToolUseID:e.ToolUseID,Reason:e.Reason,
        EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,RawSHA256:e.RawSHA256,
    }
    switch strings.ToLower(strings.TrimSpace(e.HookEventName)) {
    case "subagentstart":
        out.EventKind="state"; out.State=StateRunning
    case "subagentstop":
        out.EventKind="state"; out.State=stopState(e.Reason,e.Extra); out.Terminal=true
    case "pretooluse","posttooluse","posttoolusefailure":
        out.EventKind="activity"
    default:
        return ExecutionEvent{},fmt.Errorf("hook event %q is not part of the execution lifecycle contract",e.HookEventName)
    }
    out.EventID=eventIDFor(out)
    return out,nil
}

func stopState(reason string,extra map[string]any) CanonicalState {
    combined:=strings.ToLower(strings.TrimSpace(reason))
    for _,k:=range []string{"error_type","notification_type","trigger"} {
        if v,ok:=extra[k]; ok { combined+=" "+strings.ToLower(fmt.Sprint(v)) }
    }
    switch {
    case strings.Contains(combined,"interrupt"),strings.Contains(combined,"cancel"): return StateInterrupted
    case strings.Contains(combined,"error"),strings.Contains(combined,"fail"): return StateErrored
    case strings.Contains(combined,"shutdown"),strings.Contains(combined,"terminate"): return StateShutdown
    default: return StateCompleted
    }
}

func AppendExecutionEvent(project string,e ExecutionEvent) error {
    if e.EventID=="" { e.EventID=eventIDFor(e) }
    if e.ObservedAt=="" { e.ObservedAt=time.Now().UTC().Format(time.RFC3339Nano) }
    if e.AttemptID=="" { return errors.New("execution event requires attempt_id") }
    path:=ExecutionJournalPath(project)
    if err:=os.MkdirAll(filepath.Dir(path),0700); err!=nil { return err }
    payload,err:=json.Marshal(e); if err!=nil { return err }
    payload=append(payload,'\n')
    f,err:=os.OpenFile(path,os.O_CREATE|os.O_WRONLY|os.O_APPEND,0600); if err!=nil { return err }
    _,writeErr:=f.Write(payload); closeErr:=f.Close()
    if writeErr!=nil { return writeErr }; return closeErr
}

func BindAttempt(project,attemptID,taskID,agentPath,source,parentAttemptID string,evidence map[string]string,now time.Time) (Attempt,error) {
    attemptID=strings.TrimSpace(attemptID); taskID=strings.ToUpper(strings.TrimSpace(taskID)); agentPath=strings.TrimSpace(agentPath)
    if attemptID=="" { return Attempt{},errors.New("attempt_id is required") }
    if taskID=="" && agentPath=="" { return Attempt{},errors.New("task_id or agent_path is required") }
    ledger,err:=BuildLedger(project,20,now); if err!=nil { return Attempt{},err }
    found:=false; for _,a:=range ledger.Attempts { if a.AttemptID==attemptID { found=true; break } }
    if !found { return Attempt{},fmt.Errorf("attempt not found: %s",attemptID) }
    if source=="" { source="explicit" }
    e:=ExecutionEvent{EventKind:"binding",ObservedAt:now.UTC().Format(time.RFC3339Nano),AttemptID:attemptID,TaskID:taskID,AgentPath:agentPath,ParentAttemptID:strings.TrimSpace(parentAttemptID),BindingSource:source,BindingEvidence:evidence,EvidenceSource:EvidenceManualBinding,ObservationQuality:QualityAuthoritative}
    e.EventID=eventIDFor(e)
    if err:=AppendExecutionEvent(project,e); err!=nil { return Attempt{},err }
    ledger,err=BuildLedger(project,20,now); if err!=nil { return Attempt{},err }
    for _,a:=range ledger.Attempts { if a.AttemptID==attemptID { return a,nil } }
    return Attempt{},fmt.Errorf("attempt disappeared after binding: %s",attemptID)
}

type accumulator struct {
    attempt Attempt
    transitions []Transition
    taskIDs map[string]bool
    agentPaths map[string]bool
    parents map[string]bool
}

func newAccumulator(id string) *accumulator {
    return &accumulator{attempt:Attempt{AttemptID:id,BindingState:BindingUnbound,CurrentState:StateRuntimeUnknown,BindingEvidence:map[string]string{}},taskIDs:map[string]bool{},agentPaths:map[string]bool{},parents:map[string]bool{}}
}

func (a *accumulator) apply(e ExecutionEvent) {
    a.attempt.EvidenceCount++
    if e.Provider!="" { a.attempt.Provider=e.Provider }; if e.SessionID!="" { a.attempt.SessionID=e.SessionID }; if e.TurnID!="" { a.attempt.TurnID=e.TurnID }
    if e.RuntimeAgentID!="" { a.attempt.RuntimeAgentID=e.RuntimeAgentID }; if e.AgentType!="" { a.attempt.AgentType=e.AgentType }
    a.attempt.LastObservedAt=maxTimeString(a.attempt.LastObservedAt,e.ObservedAt)
    switch e.EventKind {
    case "activity":
        a.attempt.ActivityCount++; a.attempt.LastActivityAt=maxTimeString(a.attempt.LastActivityAt,e.ObservedAt)
    case "state":
        if e.State==StateStarting || e.State==StateRunning { if a.attempt.StartedAt=="" { a.attempt.StartedAt=e.ObservedAt } }
        if e.State==StateRunning { a.attempt.LastActivityAt=maxTimeString(a.attempt.LastActivityAt,e.ObservedAt) }
        if !a.attempt.Terminal || e.Terminal {
            if a.attempt.CurrentState!=e.State { a.transitions=append(a.transitions,Transition{At:e.ObservedAt,State:e.State,Kind:"state",EvidenceSource:e.EvidenceSource,ObservationQuality:e.ObservationQuality,Reason:e.Reason}) }
            a.attempt.CurrentState=e.State; a.attempt.StateEvidenceSource=e.EvidenceSource; a.attempt.StateObservationQuality=e.ObservationQuality
        }
        if e.Terminal { a.attempt.Terminal=true; a.attempt.EndedAt=e.ObservedAt }
    case "binding":
        addValue(a.taskIDs,e.TaskID); addValue(a.agentPaths,e.AgentPath); addValue(a.parents,e.ParentAttemptID)
        if e.BindingSource!="" { a.attempt.BindingSource=e.BindingSource }
        for k,v:=range e.BindingEvidence { if strings.TrimSpace(v)!="" { a.attempt.BindingEvidence[k]=v } }
        if len(a.taskIDs)>1 || len(a.agentPaths)>1 || len(a.parents)>1 {
            a.attempt.BindingState=BindingAmbiguous
            a.attempt.TaskID=""; a.attempt.AgentPath=""; a.attempt.ParentAttemptID=""
        } else {
            a.attempt.BindingState=BindingBound; a.attempt.TaskID=onlyValue(a.taskIDs); a.attempt.AgentPath=onlyValue(a.agentPaths); a.attempt.ParentAttemptID=onlyValue(a.parents)
        }
        a.transitions=append(a.transitions,Transition{At:e.ObservedAt,Kind:"binding",EvidenceSource:e.EvidenceSource,ObservationQuality:e.ObservationQuality,Reason:string(a.attempt.BindingState)})
    }
}

func (a *accumulator) finish(limit int,now time.Time) (Attempt,[]LedgerFinding) {
    findings:=[]LedgerFinding{}
    if a.attempt.BindingState==BindingUnbound { findings=append(findings,LedgerFinding{Severity:"info",Code:"binding_unbound",AttemptID:a.attempt.AttemptID,Message:"runtime execution is not yet bound to a Task Mecca task/agent path"}) }
    if a.attempt.BindingState==BindingAmbiguous { findings=append(findings,LedgerFinding{Severity:"warning",Code:"binding_ambiguous",AttemptID:a.attempt.AttemptID,Message:"conflicting binding evidence exists; do not use this attempt for automatic backlog transitions"}) }
    if a.attempt.StartedAt=="" && a.attempt.ActivityCount>0 { findings=append(findings,LedgerFinding{Severity:"info",Code:"activity_without_start",AttemptID:a.attempt.AttemptID,Message:"runtime activity was observed without a start event"}) }
    if a.attempt.StartedAt!="" && !a.attempt.Terminal {
        if at,err:=time.Parse(time.RFC3339Nano,firstNonEmptyRuntime(a.attempt.LastActivityAt,a.attempt.StartedAt)); err==nil && now.Sub(at)>=staleWarnDuration() {
            findings=append(findings,LedgerFinding{Severity:"warning",Code:"stale",AttemptID:a.attempt.AttemptID,Message:"no runtime evidence was observed within the stale window; this is not proof that the agent is dead"})
        }
    }
    if a.attempt.StartedAt!="" {
        end:=now; if a.attempt.EndedAt!="" { if parsed,err:=time.Parse(time.RFC3339Nano,a.attempt.EndedAt); err==nil { end=parsed } }
        if start,err:=time.Parse(time.RFC3339Nano,a.attempt.StartedAt); err==nil && end.After(start) { a.attempt.ElapsedMillis=end.Sub(start).Milliseconds() }
    }
    sort.SliceStable(a.transitions,func(i,j int)bool { return a.transitions[i].At<a.transitions[j].At })
    a.attempt.ActiveTimeAvailable=false
    a.attempt.ActiveTimeNote="hook lifecycle does not provide authoritative active intervals; observed_active_ms is unset"
    a.attempt.WaitingMillis=waitingMillis(a.transitions,a.attempt.EndedAt,now)
    if limit<1 { limit=10 }; if len(a.transitions)>limit { a.attempt.RecentTransitions=append([]Transition{},a.transitions[len(a.transitions)-limit:]...) } else { a.attempt.RecentTransitions=append([]Transition{},a.transitions...) }
    if len(a.attempt.BindingEvidence)==0 { a.attempt.BindingEvidence=nil }
    return a.attempt,findings
}

func BuildLedger(project string,recentLimit int,now time.Time) (Ledger,error) {
    out:=Ledger{Version:1,GeneratedAt:now.UTC().Format(time.RFC3339Nano),JournalPath:ExecutionJournalPath(project),Attempts:[]Attempt{},Findings:[]LedgerFinding{}}
    f,err:=os.Open(out.JournalPath); if errors.Is(err,os.ErrNotExist) { return out,nil }; if err!=nil { return out,err }; defer f.Close()
    accs:=map[string]*accumulator{}; seen:=map[string]bool{}; scanner:=bufio.NewScanner(f); scanner.Buffer(make([]byte,65536),2*1024*1024); line:=0
    for scanner.Scan() {
        line++; raw:=strings.TrimSpace(scanner.Text()); if raw=="" { continue }
        var e ExecutionEvent
        if err:=json.Unmarshal([]byte(raw),&e); err!=nil { out.Findings=append(out.Findings,LedgerFinding{Severity:"warning",Code:"invalid_event_line",Message:fmt.Sprintf("execution journal line %d is invalid JSON: %v",line,err)}); continue }
        if e.EventID=="" { e.EventID=eventIDFor(e) }; if seen[e.EventID] { continue }; seen[e.EventID]=true
        a:=accs[e.AttemptID]; if a==nil { a=newAccumulator(e.AttemptID); accs[e.AttemptID]=a }; a.apply(e)
    }
    if err:=scanner.Err(); err!=nil { return out,err }
    for _,a:=range accs { attempt,findings:=a.finish(recentLimit,now); out.Attempts=append(out.Attempts,attempt); out.Findings=append(out.Findings,findings...) }
    sort.Slice(out.Attempts,func(i,j int)bool { if out.Attempts[i].Terminal!=out.Attempts[j].Terminal { return !out.Attempts[i].Terminal }; if out.Attempts[i].LastObservedAt!=out.Attempts[j].LastObservedAt { return out.Attempts[i].LastObservedAt>out.Attempts[j].LastObservedAt }; return out.Attempts[i].AttemptID<out.Attempts[j].AttemptID })
    sortFindings(out.Findings)
    return out,nil
}

func ReconcileLedger(project string,recentLimit int,now time.Time) (Ledger,error) {
    out,err:=BuildLedger(project,recentLimit,now); if err!=nil { return Ledger{},err }
    for _,a:=range out.Attempts {
        if a.BindingState!=BindingAmbiguous && a.CurrentState==StateRuntimeUnknown { out.Findings=append(out.Findings,LedgerFinding{Severity:"info",Code:"runtime_unknown",AttemptID:a.AttemptID,Message:"runtime evidence is insufficient to establish a current execution state"}) }
    }
    sortFindings(out.Findings); return out,nil
}

func waitingMillis(transitions []Transition,endedAt string,now time.Time) int64 {
    var total time.Duration
    for i,t:=range transitions {
        if t.Kind!="state" || (t.State!=StateWaitingUser && t.State!=StateWaitingApproval) { continue }
        start,err:=time.Parse(time.RFC3339Nano,t.At); if err!=nil { continue }; end:=now
        for j:=i+1;j<len(transitions);j++ { if transitions[j].Kind=="state" { if parsed,err:=time.Parse(time.RFC3339Nano,transitions[j].At); err==nil { end=parsed }; break } }
        if endedAt!="" { if parsed,err:=time.Parse(time.RFC3339Nano,endedAt); err==nil && parsed.Before(end) { end=parsed } }
        if end.After(start) { total+=end.Sub(start) }
    }
    return total.Milliseconds()
}

func staleWarnDuration() time.Duration {
    seconds:=1800
    if raw:=strings.TrimSpace(os.Getenv("TASK_MECCA_STALE_WARN_SECONDS")); raw!="" { if parsed,err:=strconv.Atoi(raw); err==nil && parsed>0 { seconds=parsed } }
    return time.Duration(seconds)*time.Second
}

func addValue(values map[string]bool,value string) { value=strings.TrimSpace(value); if value!="" { values[value]=true } }
func onlyValue(values map[string]bool) string { if len(values)!=1 { return "" }; for v:=range values { return v }; return "" }
func firstNonEmptyRuntime(values ...string) string { for _,v:=range values { if strings.TrimSpace(v)!="" { return v } }; return "" }
func maxTimeString(current,candidate string) string {
    if current=="" { return candidate }; a,ea:=time.Parse(time.RFC3339Nano,current); b,eb:=time.Parse(time.RFC3339Nano,candidate)
    if ea!=nil { return candidate }; if eb!=nil { return current }; if b.After(a) { return candidate }; return current
}
func sortFindings(rows []LedgerFinding) { sort.SliceStable(rows,func(i,j int)bool { rank:=func(v string)int { if v=="error" { return 0 }; if v=="warning" { return 1 }; return 2 }; ri,rj:=rank(rows[i].Severity),rank(rows[j].Severity); if ri!=rj { return ri<rj }; if rows[i].AttemptID!=rows[j].AttemptID { return rows[i].AttemptID<rows[j].AttemptID }; return rows[i].Code<rows[j].Code }) }
