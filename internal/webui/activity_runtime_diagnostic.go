package webui

import (
    "sort"
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/internal/activity"
    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// ActivityRuntimeDiagnostic is an on-demand comparison, not a retrospective
// estimate of agent work. Missing Activity records never generate durations.
type ActivityRuntimeDiagnostic struct {
    Source string `json:"source"`
    CheckedAt string `json:"checked_at"`
    TaskID string `json:"task_id"`
    TotalAttempts int `json:"total_attempts"`
    BoundAttempts int `json:"bound_attempts"`
    MatchedAttempts int `json:"matched_attempts"`
    WithoutActivity int `json:"without_activity"`
    IdentityUnknown int `json:"identity_unknown"`
    IdentityMismatch int `json:"identity_mismatch"`
    ActivityWithoutRuntime int `json:"activity_without_runtime"`
    RuntimeFindings int `json:"runtime_findings"`
    HookStatuses []ActivityHookConfigStatus `json:"hook_statuses"`
    Truncated bool `json:"truncated"`
    Attempts []ActivityRuntimeAttempt `json:"attempts"`
}
type ActivityRuntimeAttempt struct {
    AttemptID string `json:"attempt_id"`
    AgentID string `json:"agent_id,omitempty"`
    AgentType string `json:"agent_type,omitempty"`
    Provider string `json:"provider,omitempty"`
    BindingState runtimeobs.BindingState `json:"binding_state"`
    RuntimeState runtimeobs.CanonicalState `json:"runtime_state"`
    ObservationQuality runtimeobs.ObservationQuality `json:"observation_quality,omitempty"`
    MatchedActivityEvents int `json:"matched_activity_events"`
    LastObservedAt string `json:"last_observed_at,omitempty"`
    StateEvidenceSource runtimeobs.EvidenceSource `json:"state_evidence_source,omitempty"`
    RecentHookTransitions int `json:"recent_hook_transitions"`
 RecentT3Transitions int `json:"recent_t3_transitions"`
 LatestRecentT3At string `json:"latest_recent_t3_at,omitempty"`
    LatestRecentHookAt string `json:"latest_recent_hook_at,omitempty"`
    Diagnostic string `json:"diagnostic"`
}
// These flags are only configuration evidence, not proof that hooks are firing.
type ActivityHookConfigStatus struct {
    Provider string `json:"provider"`
    Status string `json:"status"`
    ProjectInstalled bool `json:"project_installed"`
    GlobalInstalled bool `json:"global_installed"`
}
const activityRuntimeDetailLimit=60
func activityHookConfiguration(project string,provider string)ActivityHookConfigStatus {
    out:=ActivityHookConfigStatus{Provider:provider,Status:"unknown"}
    local,localErr:=runtimeobs.HookStatus(project,provider)
    global,globalErr:=runtimeobs.GlobalHookStatus(provider)
    if localErr!=nil||globalErr!=nil{return out}
    out.ProjectInstalled=local.Installed
    out.GlobalInstalled=global.Installed
    if local.Installed||global.Installed{out.Status="configured"}else{out.Status="not_configured"}
    return out
}

func summarizeActivityRuntimeEvidence(taskID string, attempts []runtimeobs.Attempt, events []activity.Event) ActivityRuntimeDiagnostic {
    out:=ActivityRuntimeDiagnostic{Source:"runtime_activity_on_demand",TaskID:taskID,Attempts:[]ActivityRuntimeAttempt{}}
    relevant:=make([]runtimeobs.Attempt,0)
    for _,a:=range attempts {
        if strings.EqualFold(a.TaskID,taskID){relevant=append(relevant,a)}
    }
    sort.Slice(relevant,func(i,j int)bool{return relevant[i].AttemptID<relevant[j].AttemptID})
    out.TotalAttempts=len(relevant)
    out.Truncated=len(relevant)>activityRuntimeDetailLimit

    // Index once: an exact attempt may contain many phase events. Avoid O(A*E)
    // scans for large histories and count matches outside the displayed top 60.
    byAttempt:=make(map[string][]activity.Event)
    distinct:=make(map[string]bool)
    matchedIDs:=make(map[string]bool)
    for _,e:=range events {
        if !strings.EqualFold(e.TaskID,taskID){continue}
        if e.EventID!=""{distinct[e.EventID]=true}
        if e.RuntimeAttemptID!=""{byAttempt[e.RuntimeAttemptID]=append(byAttempt[e.RuntimeAttemptID],e)}
    }
    for i,a:=range relevant {
        row:=ActivityRuntimeAttempt{
            AttemptID:a.AttemptID,AgentID:a.RuntimeAgentID,AgentType:a.AgentType,
            Provider:a.Provider,BindingState:a.BindingState,RuntimeState:a.CurrentState,
            ObservationQuality:a.StateObservationQuality,LastObservedAt:a.LastObservedAt,
            StateEvidenceSource:a.StateEvidenceSource,
        }
        for _,transition:=range a.RecentTransitions {
            if transition.EvidenceSource==runtimeobs.EvidenceT3{row.RecentT3Transitions++;if transition.At>row.LatestRecentT3At{row.LatestRecentT3At=transition.At};continue}
            if transition.EvidenceSource!=runtimeobs.EvidenceHook{continue}
            row.RecentHookTransitions++
            if transition.At>row.LatestRecentHookAt{row.LatestRecentHookAt=transition.At}
        }
        if a.BindingState!=runtimeobs.BindingBound || a.RuntimeAgentID=="" || a.AttemptID==""{
            row.Diagnostic="identity_unverified"
            out.IdentityUnknown++
        }else{
            out.BoundAttempts++
            wrongAgent:=false
            for _,e:=range byAttempt[a.AttemptID] {
                if e.AgentID==a.RuntimeAgentID {
                    row.MatchedActivityEvents++
                    if e.EventID!=""{matchedIDs[e.EventID]=true}
                }else{wrongAgent=true}
            }
            switch {
            case wrongAgent:
                row.Diagnostic="identity_mismatch"
                out.IdentityMismatch++
            case row.MatchedActivityEvents>0:
                row.Diagnostic="activity_recorded"
                out.MatchedAttempts++
            default:
                row.Diagnostic="no_matching_activity"
                out.WithoutActivity++
            }
        }
        if i<activityRuntimeDetailLimit{out.Attempts=append(out.Attempts,row)}
    }
    // Report distinct unmatched event identities, including events for attempts
    // omitted from the UI only when they truly lack an exact matching actor.
    for id:=range distinct {
        if !matchedIDs[id]{out.ActivityWithoutRuntime++}
    }
    return out
}

func readActivityRuntimeDiagnostic(project,taskID string,events []activity.Event)(ActivityRuntimeDiagnostic,error){
    ledger,err:=runtimeobs.BuildLedger(project,10,time.Now().UTC())
    if err!=nil{return ActivityRuntimeDiagnostic{},err}
    out:=summarizeActivityRuntimeEvidence(taskID,ledger.Attempts,events)
    out.CheckedAt=time.Now().UTC().Format(time.RFC3339Nano)
    out.RuntimeFindings=len(ledger.Findings)
    out.HookStatuses=[]ActivityHookConfigStatus{
        activityHookConfiguration(project,"codex"),activityHookConfiguration(project,"claude"),
    }
    return out,nil
}
