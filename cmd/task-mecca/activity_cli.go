package main

import (
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/internal/activity"
    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
    "github.com/silverkhan/TaskMecca/internal/handoffjournal"
    "github.com/silverkhan/TaskMecca/internal/handoff"
    "github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

// Activity mutations are explicit and evidence-backed. An untrusted agent may
// identify its own attempt, but must not infer a different actor's execution.
func runActivityCLI(args []string,out,errOut io.Writer) int {
    callStarted:=time.Now()
    if len(args)==0 {fmt.Fprintln(errOut,"activity requires start, switch, stop, report, history, status, suggest or run");return 2}
    if args[0]=="run" {return runActivityCommand(args[1:],out,errOut)}
    if args[0]=="diagnose" {return runActivityDiagnose(args[1:],out,errOut)}
    action:=args[0]
    fs:=flag.NewFlagSet("activity "+action,flag.ContinueOnError)
    fs.SetOutput(errOut)
    project:=fs.String("project",".","canonical project")
    kind:=fs.String("type","","activity type")
    role:=fs.String("role",os.Getenv("TASK_MECCA_ROLE"),"actor role")
    agent:=fs.String("agent",os.Getenv("TASK_MECCA_AGENT_ID"),"exact actor ID")
    attempt:=fs.String("attempt",os.Getenv("TASK_MECCA_ATTEMPT_ID"),"exact attempt ID")
    assignment:=fs.String("assignment","","assignment ID")
    handoffID:=fs.String("handoff","","claimed handoff ID for controller")
    eventID:=fs.String("event-id","","stable event ID for retries")
    supersedesID:=fs.String("supersedes-id","","existing immutable Activity event to correct")
    occurredAt:=fs.String("occurred-at","","corrected event occurrence in RFC3339Nano (default original)")
    evidence:=fs.String("evidence","","source evidence")
    explain:=fs.String("explanation","","required for other activity")
    language:=fs.String("language","","explicit human explanation language: ko|en; not browser storage")
    fallbackLanguage:=fs.String("fallback-language","","explicit conversation language fallback: ko|en")
    proposed:=fs.String("proposed-type","","suggested type for other activity")
    round:=fs.Int("round",-1,"default auto from observed review evidence; explicit 0 for initial work")
    eventName:=fs.String("event","","decision or work phase event for suggest")
    reviewOutcome:=fs.String("review-outcome","","accepted|rejected|needs_changes; Controller review stop only")
    runCommand:=fs.String("command","","test/build command")
    runSHA:=fs.String("git-sha","","code revision tested")
    runScope:=fs.String("scope","","test/build scope")
    runID:=fs.String("run-id","","execution evidence ID")
    runResult:=fs.String("result","","passed|failed|cancelled|unknown for test/build")
    runPurpose:=fs.String("purpose","","why the run was performed")
    ciURL:=fs.String("ci-url","","link to separately verified CI workflow run")
    jsonOutput:=fs.Bool("json",false,"JSON response")
    parseArgs:=append([]string{},args[1:]...)
    if len(parseArgs)>0 && !strings.HasPrefix(parseArgs[0],"-") {parseArgs=append(parseArgs[1:],parseArgs[0])}
    if err:=fs.Parse(parseArgs);err!=nil{return 2}
    if fs.NArg()>1 {fmt.Fprintln(errOut,"expected at most one task ID");return 2}
    root,err:=filepath.Abs(*project);if err!=nil{fmt.Fprintln(errOut,err);return 2}
    store:=activity.Ledger{Project:root}
    emit:=func(v any){if *jsonOutput{_ =json.NewEncoder(out).Encode(v)}else{fmt.Fprintln(out,v)}}
    if action=="diagnose" {
        if fs.NArg()!=0{fmt.Fprintln(errOut,"diagnose does not accept a task ID");return 2}
        stats,statsErr:=store.Diagnostics()
        if statsErr!=nil{fmt.Fprintln(errOut,statsErr);return 1}
        emit(stats)
        return 0
    }
    if action=="suggest" {
        if !activity.Role(*role).Valid() || *eventName=="" || *round<0 {
            fmt.Fprintln(errOut,"suggest requires --role, --event and a nonnegative --round");return 2
        }
        emit(activity.Suggest(activity.Role(*role),*eventName,*round))
        return 0
    }
    if action=="history" || action=="status" {
        task:="";if fs.NArg()==1{task=fs.Arg(0)}
        if task==""{fmt.Fprintln(errOut,"task ID required");return 2}
        rows,readErr:=store.List(task);if readErr!=nil{fmt.Fprintln(errOut,readErr);return 1}
        if action=="history" {emit(map[string]any{"task_id":task,"events":rows});return 0}
        effective,findings:=activity.EffectiveEvents(rows)
        latest:=map[string]activity.Event{}
        for _,row:=range effective {
            if row.Action==activity.Correct{continue}
            key:=string(row.Role)+"|"+row.AgentID+"|"+row.RuntimeAttemptID
            latest[key]=row
        }
        active:=[]activity.Event{}
        for _,row:=range latest {
            if row.Action==activity.Start || row.Action==activity.Switch {active=append(active,row)}
        }
        if len(findings)>0{
            fmt.Fprintf(errOut,"warning: %d invalid activity corrections excluded\\n",len(findings))
        }
        emit(map[string]any{"task_id":task,"open_activities":active,"event_count":len(rows)})
        return 0
    }
    if action!="start" && action!="switch" && action!="stop" && action!="report" && action!="correct"{fmt.Fprintln(errOut,"unsupported activity action");return 2}
    if fs.NArg()!=1{fmt.Fprintln(errOut,"task ID required");return 2}
    if strings.TrimSpace(*eventID)==""{fmt.Fprintln(errOut,"--event-id required for idempotent retries");return 2}
    if strings.TrimSpace(*role)==""||strings.TrimSpace(*agent)==""||strings.TrimSpace(*attempt)==""{
        fmt.Fprintln(errOut,"--role, --agent, --attempt (or exact runtime env) required");return 2
    }
    selectedLanguage,languageErr:=activityExplanationLanguage(*language,*fallbackLanguage)
    if languageErr!=nil{fmt.Fprintln(errOut,languageErr);return 2}
    // Generate only from the caller's exact standard phase/type. Never translate evidence.
    if action!="correct" && strings.TrimSpace(*explain)=="" && selectedLanguage!=""{
        descriptionPhase:=*eventName;if action=="stop"{descriptionPhase="phase_completed"}
        *explain=activityPhaseExplanation(descriptionPhase,*kind,selectedLanguage)
    }
    task:=fs.Arg(0)
    phaseEvent:=""
    if action=="report"{
        if strings.TrimSpace(*eventName)==""{fmt.Fprintln(errOut,"report requires --event");return 2}
        // An identical event is idempotent even after subsequent phase changes.
        // PhaseEvent is immutable to prevent reusing an ID for a different fact.
        if prior,readErr:=store.Read(*eventID);readErr==nil{
            if prior.TaskID==task && prior.PhaseEvent==*eventName && string(prior.Role)==*role &&
                prior.AgentID==*agent && prior.RuntimeAttemptID==*attempt &&
                prior.AssignmentID==*assignment && prior.HandoffID==*handoffID &&
                prior.Evidence==*evidence && prior.Explanation==*explain && prior.ExplanationLanguage==selectedLanguage &&
                prior.ProposedType==*proposed && prior.Source=="agent_report" {
                emit(map[string]any{"ok":true,"recorded":false,"event":prior});return 0
            }
            fmt.Fprintln(errOut,"event-id already belongs to another reported phase");return 3
        } else if !os.IsNotExist(readErr){fmt.Fprintln(errOut,readErr);return 1}
        resolution,resolveErr:=resolveActivityReport(store,task,activity.Role(*role),*agent,*attempt,*eventName)
        if resolveErr!=nil{fmt.Fprintln(errOut,resolveErr);return 2}
        if *kind!="" && *kind!=string(resolution.Type){
            fmt.Fprintln(errOut,"reported phase type conflicts with standard classification");return 2
        }
        action=string(resolution.Action)
        *kind=string(resolution.Type)
        *round=resolution.Round
        *reviewOutcome=resolution.ReviewOutcome
        phaseEvent=*eventName
    }
    var correctionTarget *activity.Event
    if action=="correct" {
        if *supersedesID==""{fmt.Fprintln(errOut,"correct requires --supersedes-id");return 2}
        original,readErr:=store.Read(*supersedesID)
        if readErr!=nil{fmt.Fprintln(errOut,readErr);return 2}
        if original.TaskID!=task{fmt.Fprintln(errOut,"correction task differs from original");return 2}
        correctionTarget=&original
        if *kind==""{*kind=string(original.Type)}
        if *round<0{*round=original.Round}
        if *reviewOutcome==""{*reviewOutcome=original.ReviewOutcome}
        if *explain==""{*explain=original.Explanation;selectedLanguage=original.ExplanationLanguage}
        if *kind==string(activity.Other) && *proposed==""{*proposed=original.ProposedType}
    }
    // Auto-classify revision round only from explicit Controller review outcomes.
    // A new worker attempt starts in the number of rejected/needs-changes rounds
    // observed for this exact backlog. Switch/stop continue the same round.
    if *round<0 || (action=="stop" && *kind==""){
        rows,readErr:=store.List(task)
        if readErr!=nil{fmt.Fprintln(errOut,readErr);return 1}
        if action=="start" && *round<0{
            handoffs,readErr:=handoffjournal.BuildLedger(root,time.Now().UTC());if readErr!=nil{fmt.Fprintln(errOut,readErr);return 1}
            *round=activity.SuggestedRoundFromEvidence(task,rows,handoffs.Handoffs)
        }else{
            for i:=len(rows)-1;i>=0;i--{
                prev:=rows[i]
                if string(prev.Role)==*role && prev.AgentID==*agent && prev.RuntimeAttemptID==*attempt && prev.Action!=activity.Correct{
                    if *round<0{*round=prev.Round}
                    if action=="stop" && *kind==""{*kind=string(prev.Type)}
                    break
                }
            }
        }
        if *round<0{*round=0}
    }
    if prior,readErr:=store.Read(*eventID);readErr==nil {
        if prior.TaskID==task && string(prior.Action)==action && string(prior.Role)==*role && prior.AgentID==*agent && prior.RuntimeAttemptID==*attempt && prior.AssignmentID==*assignment && prior.HandoffID==*handoffID && prior.ReviewOutcome==*reviewOutcome && prior.SupersedesID==*supersedesID && prior.Round==*round && prior.Evidence==*evidence && prior.Explanation==*explain && prior.ExplanationLanguage==selectedLanguage && (*kind=="" || string(prior.Type)==*kind) {
            emit(map[string]any{"ok":true,"recorded":false,"event":prior});return 0
        }
        fmt.Fprintln(errOut,"event-id already belongs to another activity");return 3
    } else if !os.IsNotExist(readErr) {fmt.Fprintln(errOut,readErr);return 1}
    if strings.TrimSpace(*evidence)==""{
        fmt.Fprintln(errOut,"--evidence required; agent reports are not authoritative Hook evidence");return 2
    }
    if *role==string(activity.RoleWorker)&&strings.TrimSpace(*assignment)=="" {fmt.Fprintln(errOut,"worker requires --assignment");return 2}
    if *role==string(activity.RoleController)&&strings.TrimSpace(*handoffID)=="" {fmt.Fprintln(errOut,"controller requires --handoff");return 2}
    var runEvidence *activity.RunEvidence
    if *runCommand!=""||*runSHA!=""||*runScope!=""||*runID!=""||*runResult!=""||*ciURL!="" {
        runEvidence=&activity.RunEvidence{Command:*runCommand,GitSHA:*runSHA,Scope:*runScope,RunID:*runID,Result:*runResult,Purpose:*runPurpose,CIURL:*ciURL}
    }
    now:=time.Now().UTC()
    eventOccurred:=now
    if correctionTarget!=nil {
        eventOccurred=correctionTarget.OccurredAt
        if *occurredAt!="" {
            parsed,parseErr:=time.Parse(time.RFC3339Nano,*occurredAt)
            if parseErr!=nil{fmt.Fprintln(errOut,"invalid --occurred-at:",parseErr);return 2}
            eventOccurred=parsed
        }
    }
    e:=activity.Event{SchemaVersion:1,EventID:*eventID,TaskID:task,Action:activity.Action(action),
        Role:activity.Role(*role),AgentID:*agent,RuntimeAttemptID:*attempt,AssignmentID:*assignment,HandoffID:*handoffID,
        Round:*round,Type:activity.Type(*kind),Confidence:activity.Inferred,Source:"agent_report",PhaseEvent:phaseEvent,
        Evidence:*evidence,Explanation:*explain,ExplanationLanguage:selectedLanguage,ProposedType:*proposed,ReviewOutcome:*reviewOutcome,RunEvidence:runEvidence,SupersedesID:*supersedesID,OccurredAt:eventOccurred,RecordedAt:now}
    if correctionTarget!=nil {
        e.Source="agent_correction"
        e.PhaseEvent=correctionTarget.PhaseEvent
        e.Provider=correctionTarget.Provider
        e.SessionID=correctionTarget.SessionID
        e.ContractSHA256=correctionTarget.ContractSHA256
        e.TaskRevision=correctionTarget.TaskRevision
        e.Generation=correctionTarget.Generation
        e.ParentActivityID=correctionTarget.ParentActivityID
        if e.RunEvidence==nil && correctionTarget.RunEvidence!=nil {
            copyRun:=*correctionTarget.RunEvidence
            e.RunEvidence=&copyRun
        }
    }
    if err:=e.Validate();err!=nil{fmt.Fprintln(errOut,err);return 2}
    // Verify attribution under the same project write lock as the append.
    // A terminal/runtime rebinding event cannot land between verification and
    // Activity publication through other canonical project writers.
    inserted,writeErr:=store.ApplyVerified(e,func()error{
        runtimeLedger,err:=runtimeobs.BuildLedger(root,10,time.Now().UTC())
        if err!=nil{return fmt.Errorf("runtime evidence unavailable: %w",err)}
        switch e.Role {
        case activity.RoleReviewer:
            // Reviewer recording uses the real CLI write boundary. It grants
            // no implementation permission and rechecks the current task,
            // exact recipient, execution consent and complete policy source.
            reviewRuntime,err:=runtimeobs.BuildLedger(root,0,time.Now().UTC())
            if err!=nil{return fmt.Errorf("reviewer runtime evidence unavailable: %w",err)}
            assignments,err:=runtimeobs.ListAssignments(root)
            if err!=nil{return fmt.Errorf("assignment evidence unavailable: %w",err)}
            if err:=activity.ValidateAttribution(e,reviewRuntime.Attempts,assignments);err!=nil{return err}
            decision,err:=projectContractDecision(root,e.TaskID)
            if err!=nil{return err}
            if !decision.ExecutionAuthorized||decision.RolePolicy==nil||decision.RolePolicyError!=""{return fmt.Errorf("Reviewer Activity requires current execution authorization and structured review policy")}
            p:=decision.RolePolicy
            assessment,err:=handoff.AssessReviewerPolicy(root,e.TaskID,e.RuntimeAttemptID,p.Workspace,time.Now().UTC())
            if err!=nil{return fmt.Errorf("Reviewer Activity readonly gate: %w",err)}
            if !assessment.Allowed||assessment.Implementation!="reviewer_readonly"||assessment.Source==nil{return fmt.Errorf("Reviewer Activity requires verified readonly recipient/source")}
            fresh,err:=projectContractDecision(root,e.TaskID)
            if err!=nil{return err}
            if !fresh.ExecutionAuthorized||fresh.RolePolicyError!=""||rolepolicy.PolicyDigest(fresh.RolePolicy)!=assessment.PolicySHA256{return fmt.Errorf("Reviewer Activity authorization or policy changed during verification")}
            source,err:=rolepolicy.Snapshot(p.Workspace,p.Scope)
            if err!=nil{return err}
            if source.Digest()!=assessment.Source.Digest(){return fmt.Errorf("Reviewer Activity source changed during verification")}
            currentRuntime,err:=runtimeobs.BuildLedger(root,0,time.Now().UTC())
            if err!=nil{return err}
            currentAssignments,err:=runtimeobs.ListAssignments(root)
            if err!=nil{return err}
            return activity.ValidateAttribution(e,currentRuntime.Attempts,currentAssignments)
        case activity.RoleWorker:
            assignments,err:=runtimeobs.ListAssignments(root)
            if err!=nil{return fmt.Errorf("assignment evidence unavailable: %w",err)}
            return activity.ValidateAttribution(e,runtimeLedger.Attempts,assignments)
        case activity.RoleRegistrar:
            observed:=false
            for _,a:=range runtimeLedger.Attempts {
                if a.AttemptID==e.RuntimeAttemptID && a.RuntimeAgentID==e.AgentID && a.TaskID==e.TaskID && a.AgentPath=="/root/registrar" && runtimeobs.HasObservedExecution(a) { observed=true }
            }
            if !observed { return fmt.Errorf("Registrar Activity requires verified observed execution; a registration fact or binding alone is not activity") }
            return activity.ValidateRegistrarAttribution(e,runtimeLedger.Attempts)
        case activity.RoleSubController:
            return activity.ValidateSubControllerAttribution(e,runtimeLedger.Attempts)
        case activity.RoleController:
            claims,err:=handoffjournal.BuildLedger(root,time.Now().UTC())
            if err!=nil{return fmt.Errorf("handoff evidence unavailable: %w",err)}
            return activity.ValidateControllerAttribution(e,runtimeLedger.Attempts,claims.Handoffs)
        default:
            return fmt.Errorf("this role has no verified Activity recording boundary yet")
        }
    })
    if writeErr!=nil{
        if errors.Is(writeErr,activity.ErrEventConflict){fmt.Fprintln(errOut,"event-id reused with a different event; original record preserved");return 3}
        fmt.Fprintln(errOut,writeErr);return 1
    }
    emit(map[string]any{"ok":true,"recorded":inserted,"event":e,"recorder_elapsed_ms":float64(time.Since(callStarted).Microseconds())/1000})
    return 0
}

// No implicit environment/global/browser language lookup: the dispatch caller supplies context.
func activityExplanationLanguage(preferred,fallback string)(string,error){
    valid:=func(v string)bool{return v=="ko"||v=="en"}
    if fallback!=""&&!valid(fallback){return "",fmt.Errorf("--fallback-language supports ko|en only")}
    if valid(preferred){return preferred,nil}
    if fallback!=""{return fallback,nil}
    if preferred!=""{return "",fmt.Errorf("unsupported --language; supply explicit --fallback-language ko|en")}
    return "",nil // Legacy callers remain unchanged, language unobserved.
}
func activityPhaseExplanation(phase,kind,language string)string{
    // These describe reported boundaries, never tests passed, acceptance or current liveness.
    ends:=map[string][2]string{
        "worker_completed":{"작업 결과를 컨트롤러에 보고합니다.","Reporting work results to the Controller."},
        "worker_blocked":{"작업 진행을 막는 조건을 보고합니다.","Reporting a condition blocking work."},
        "registration_completed":{"작업 등록 결과를 보고합니다.","Reporting task registration results."},
        "review_completed":{"검토 단계의 종료를 보고합니다.","Reporting the end of the review phase."},
        "phase_completed":{"활동 단계의 종료를 보고합니다.","Reporting the end of the activity phase."},
        "review_accepted":{"검토 수용 의견을 보고합니다.","Reporting a review acceptance opinion."},
        "review_rejected":{"검토 반려 의견을 보고합니다.","Reporting a review rejection opinion."},
        "review_needs_changes":{"검토에서 필요한 수정을 보고합니다.","Reporting changes requested by review."},
    }
    index:=1;if language=="ko"{index=0}
    if text,ok:=ends[phase];ok{return text[index]}
    if phase!=""{kind=string(activity.Suggest(activity.RoleController,phase,0).Type)}
    text,ok:=map[string][2]string{
        "requirements":{"작업 요구사항을 확인합니다.","Checking task requirements."},
        "research":{"작업에 필요한 자료와 소스를 조사합니다.","Inspecting sources and references for the task."},
        "implementation":{"작업 변경 사항을 구현합니다.","Implementing the task changes."},
        "targeted_test":{"변경 대상의 검증을 수행합니다.","Performing targeted validation."},
        "full_regression":{"회귀 검증을 수행합니다.","Performing regression validation."},
        "build_check":{"빌드 검증을 수행합니다.","Performing build validation."},
        "controller_review":{"작업 결과와 근거를 검토합니다.","Reviewing work results and evidence."},
        "controller_rereview":{"수정 결과와 근거를 다시 검토합니다.","Reviewing revised results and evidence."},
        "handoff":{"작업 결과와 다음 조치를 인계합니다.","Handing off work results and next actions."},
        "recovery":{"작업 재개에 필요한 복구를 수행합니다.","Performing recovery needed to resume work."},
    }[kind]
    if !ok{return ""} // Unknown/other still needs authored explanation, no keyword guessing.
    return text[index]
}
