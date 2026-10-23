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

    "github.com/silverkhan/TaskMecca/internal/backlog"
)

// flow next is a read-only decision, not a new owner of lifecycle, worktree
// cleanup, contract approval or native transport. Never infer that a final
// message, copied files, or a clean-looking git status authorizes exit.
type flowNextDecision struct {
    SchemaVersion int `json:"schema_version"`
    Decision string `json:"decision"`
    ExitAllowed bool `json:"exit_allowed"`
    Role string `json:"role"`
    Event string `json:"event"`
    TaskID string `json:"task_id"`
    TaskState string `json:"task_state,omitempty"`
    Worktree string `json:"worktree,omitempty"`
    IntegrationRef string `json:"integration_ref,omitempty"`
    CleanupStatus string `json:"cleanup_status"`
    EvidenceQuality string `json:"evidence_quality"`
    Reasons []string `json:"reasons"`
    NextAction string `json:"next_action"`
    Commands [][]string `json:"commands,omitempty"`
    PlanDigest string `json:"plan_digest,omitempty"`
    Archive string `json:"archive,omitempty"`
    Written bool `json:"written"`
}

func flowNextUsage(out io.Writer) {
    fmt.Fprintln(out,"task-mecca flow next --role controller --event final_requested --project DIR --task B-541 --into REF --worktree EXACT_PATH [--archive EXTERNAL_DIR] --json")
    fmt.Fprintln(out,"Read-only Controller completion/cleanup gate; no deletion, native sends or lifecycle writes.")
    fmt.Fprintln(out,"Incomplete worktree cleanup never reports EXIT_ALLOWED. Other roles/events are not implemented yet and fail closed.")
    fmt.Fprintln(out,"See: task-mecca help worktree; run plan -> preserve -> verify -> approved remove separately.")
}

func evaluateControllerCleanup(project,task,into,target,archive string) flowNextDecision {
    result:=flowNextDecision{
        SchemaVersion:1,Decision:"EXIT_BLOCKED",Role:"controller",
        Event:"final_requested",TaskID:strings.ToUpper(task),
        IntegrationRef:into,Worktree:target,Archive:archive,
        CleanupStatus:"unknown",EvidenceQuality:"unverified",Reasons:[]string{},
        NextAction:"Inspect the original task and the exact worker worktree; do not finish the Controller turn.",
        Written:false,
    }
    state,err:=backlog.Inspect(project,"",task)
    if err!=nil {
        result.Reasons=append(result.Reasons,"canonical_task_unreadable: "+err.Error())
        return result
    }
    if state["exists"]!=true {
        result.Reasons=append(result.Reasons,"canonical_task_not_found")
        return result
    }
    if state["duplicate_count"]!=1 {
        result.Reasons=append(result.Reasons,"canonical_task_ambiguous")
        return result
    }
    actual,_:=state["state"].(string)
    result.TaskState=actual
    if actual!="done" {
        result.Decision="CONTINUE"
        result.CleanupStatus="not_due"
        result.EvidenceQuality="canonical"
        result.NextAction="Complete and verify the original task before requesting exit; Worker DONE alone is insufficient."
        result.Reasons=append(result.Reasons,"task_not_finalized")
        return result
    }
    if target=="" {
        result.Reasons=append(result.Reasons,"exact_worktree_required")
        result.NextAction="Provide an exact original worker worktree identity; omission is not proof it was cleaned."
        return result
    }
    // Independently inspect Git HEAD/merge/dirty state; worktree plan is the
    // single owner of this policy. Never interpret cherry-pick as ancestry.
    plan,err:=inspectWorktrees(project,into,200000)
    if err!=nil {
        result.Reasons=append(result.Reasons,"git_plan_unavailable: "+err.Error())
        return result
    }
    result.PlanDigest=plan.PlanDigest
    var selected *worktreeEntry
    for i:=range plan.Worktrees {
        if filepath.Clean(plan.Worktrees[i].Path)==filepath.Clean(target) {
            selected=&plan.Worktrees[i]
            break
        }
    }
    if selected!=nil {
        if selected.Primary {
            result.CleanupStatus="protected"
            result.Reasons=append(result.Reasons,"primary_worktree")
            result.NextAction="Never remove the primary worktree. Verify worker assignment mapping."
            return result
        }
        result.CleanupStatus="retained"
        result.Reasons=append(result.Reasons,selected.ProtectionReasons...)
    }
    if archive=="" {
        result.Reasons=append(result.Reasons,"preservation_archive_not_supplied")
        result.NextAction="Provide external preservation archive; run worktree plan → preserve → verify, then review exact safe removal."
        result.Commands=[][]string{{"task-mecca","worktree","plan","--project",project,"--into",into,"--json"}}
        return result
    }
    manifest,err:=readWorktreePreservation(archive)
    if err!=nil {
        result.Reasons=append(result.Reasons,"preservation_manifest_unavailable: "+err.Error())
        result.NextAction="Do not delete; recover exact-source protection manifest or preserve worktree first."
        return result
    }
    if manifest.Repository!=plan.Repository||filepath.Clean(manifest.Target)!=filepath.Clean(target) {
        result.Reasons=append(result.Reasons,"preservation_identity_conflict")
        result.NextAction="Review archive target/repository and original worktree; never assume a copied folder covers another target."
        return result
    }
    check:=verifyWorktreePreservation(manifest,selected!=nil)
    if !check.Verified {
        result.CleanupStatus="protection_failed"
        result.Reasons=append(result.Reasons,check.Reasons...)
        result.NextAction="Repair or reproduce SHA-256 preservation; do not remove original worktree."
        return result
    }
    result.EvidenceQuality="verified_archive_sha256"
    if selected!=nil {
        result.CleanupStatus="preserved_retained"
        result.Reasons=append(result.Reasons,"source_worktree_retained")
        if selected.Merged!="yes" {
            result.Reasons=append(result.Reasons,"merged_head_not_verified")
            result.NextAction="Verify integration; cherry-picked code is not HEAD ancestry. Record cleanup block and follow-up; do not force Git removal."
        } else if selected.Ignored>0||selected.Untracked>0||selected.TrackedChanged>0||selected.Symlinks>0||selected.NestedRepositories>0 {
            result.NextAction="Preserve and reconcile ignored/untracked/dirty payload safely; record owner, reasons and follow-up. Do not force removal."
        } else {
            result.NextAction="Confirm exact worktree unused with the operator; then call worktree remove with the matching plan digest and verified archive."
        }
        result.Commands=[][]string{{"task-mecca","worktree","plan","--project",project,"--into",into,"--json"},
            {"task-mecca","worktree","verify","--archive",archive,"--json"}}
        return result
    }
    // A missing Git listing alone proves nothing: verify the source path is
    // truly gone AND a removal receipt is bound to this exact protected HEAD.
    if _,err:=os.Lstat(target);err==nil {
        result.Reasons=append(result.Reasons,"source_path_still_exists_unregistered")
        result.NextAction="Git no longer lists this path but it exists on disk; manually inspect, never silently delete."
        return result
    } else if !errors.Is(err,os.ErrNotExist) {
        result.Reasons=append(result.Reasons,"source_path_inaccessible: "+err.Error())
        return result
    }
    receiptPath:=filepath.Join(archive,"removal.json")
    receiptBytes,err:=os.ReadFile(receiptPath)
    if err!=nil {
        result.CleanupStatus="missing_removal_receipt"
        result.Reasons=append(result.Reasons,"removal_receipt_missing")
        result.NextAction="Investigate completed Git removal without receipt; do not claim cleanup success."
        return result
    }
    var receipt struct {
        Target string `json:"target"`
        Repository string `json:"repository"`
        HEAD string `json:"head"`
        PayloadDigest string `json:"payload_digest"`
        ConfirmedAt string `json:"confirmed_at"`
        Method string `json:"method"`
    }
    if err:=json.Unmarshal(receiptBytes,&receipt);err!=nil||
        receipt.Target!=manifest.Target||receipt.Repository!=manifest.Repository||
        receipt.HEAD!=manifest.HEAD||receipt.PayloadDigest!=manifest.PayloadDigest||
        receipt.ConfirmedAt==""||receipt.Method!="git worktree remove (no force)"{
        result.Reasons=append(result.Reasons,"removal_receipt_identity_invalid")
        result.NextAction="Verify the exact removal evidence; refuse premature or conflicting completion."
        return result
    }
    result.CleanupStatus="removed_verified"
    result.EvidenceQuality="git_absent_plus_manifest_and_receipt"
    result.Decision="EXIT_ALLOWED"
    result.ExitAllowed=true
    result.NextAction="Worktree cleanup verified; Controller may now check other task, handoff and external-sync exit gates."
    return result
}

func runFlowNextCLI(args []string,out,stderr io.Writer) int {
    if len(args)==0||args[0]=="help"||args[0]=="--help" {flowNextUsage(out);return 0}
    if args[0]!="next" {
        fmt.Fprintln(stderr,"flow supports only read-only next")
        return 2
    }
    fs:=flag.NewFlagSet("flow next",flag.ContinueOnError)
    fs.SetOutput(stderr)
    role:=fs.String("role","","agent role")
    event:=fs.String("event","","structured event")
    project:=fs.String("project",".","canonical original Git repository")
    task:=fs.String("task","","canonical backlog id")
    into:=fs.String("into","origin/dev","integration branch/ref")
    target:=fs.String("worktree","","exact worker worktree path")
    archive:=fs.String("archive","","external preservation archive")
    asJSON:=fs.Bool("json",false,"structured decision")
    if err:=fs.Parse(args[1:]);err!=nil{return 2}
    if fs.NArg()!=0||*role==""||*event==""||*task==""||*into=="" {flowNextUsage(stderr);return 2}
    projectAbs,err:=filepath.Abs(*project)
    if err!=nil{fmt.Fprintln(stderr,err);return 2}
    var targetAbs,archiveAbs string
    if *target!=""{targetAbs,err=filepath.Abs(*target);if err!=nil{fmt.Fprintln(stderr,err);return 2}}
    if *archive!=""{archiveAbs,err=filepath.Abs(*archive);if err!=nil{fmt.Fprintln(stderr,err);return 2}}
    var decision flowNextDecision
    if *role=="controller"&&(*event=="final_requested"||*event=="cleanup_review"){
        decision=evaluateControllerCleanup(projectAbs,*task,*into,targetAbs,archiveAbs)
        decision.Event=*event
    }else{
        decision=flowNextDecision{
            SchemaVersion:1,Decision:"EXIT_BLOCKED",ExitAllowed:false,
            Role:*role,Event:*event,TaskID:strings.ToUpper(*task),
            CleanupStatus:"unknown",EvidenceQuality:"unsupported_policy",
            Reasons:[]string{"unsupported_role_event_contract"},
            NextAction:"Role/event is not implemented in flow next yet. Follow established role protocol and do not infer execution approval or exit permission.",
            Written:false,
        }
    }
    if *asJSON {
        if err:=json.NewEncoder(out).Encode(decision);err!=nil{fmt.Fprintln(stderr,err);return 1}
    }else{
        fmt.Fprintf(out,"%s · %s / %s · cleanup=%s\n",decision.Decision,decision.Role,decision.Event,decision.CleanupStatus)
        fmt.Fprintln(out,"Next:",decision.NextAction)
        for _,reason:=range decision.Reasons{fmt.Fprintln(out," -",reason)}
    }
    return 0
}
