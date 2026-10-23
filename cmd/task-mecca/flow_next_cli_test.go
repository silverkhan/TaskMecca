package main

import (
    "bytes"
    "encoding/json"
    "os"
    "path/filepath"
    "slices"
    "strings"
    "testing"
)

func controllerFlowFixture(t *testing.T)(string,string,string,worktreePlan,string) {
    t.Helper()
    repo,worker,archive,plan:=verifiedCleanupFixture(t)
    original:=filepath.Join(repo,"_task_mecca","data","backlog")
    archived:=filepath.Join(original,"archive","2026-10")
    if err:=os.MkdirAll(archived,0700);err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(filepath.Join(archived,"000541.B-541.finished.done.md"),[]byte("# B-541 Finished\n"),0600);err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(filepath.Join(original,"000542.B-542.running.doing.md"),[]byte("# B-542 Running\n"),0600);err!=nil{t.Fatal(err)}
    // Canonical task files change the primary worktree's inventory, hence
    // the preservation plan digest. Freshly re-plan after fixture setup.
    fresh,err:=inspectWorktrees(repo,"main",200000)
    if err!=nil{t.Fatal(err)}
    _=plan
    return repo,worker,archive,fresh,archived
}

func flowReport(t *testing.T,repo,task,worker,archive string)flowNextDecision {
    t.Helper()
    // Use the public CLI parser, and verify its output is both JSON and
    // side-effect-free. The fixture's approved plan is the real worktree CLI.
    var out,errout bytes.Buffer
    arguments:=[]string{"next","--role","controller","--event","final_requested",
        "--project",repo,"--task",task,"--into","main","--worktree",worker,
        "--archive",archive,"--json"}
    if code:=runFlowNextCLI(arguments,&out,&errout);code!=0{
        t.Fatalf("flow next code=%d stderr=%s",code,errout.String())
    }
    var answer flowNextDecision
    if err:=jsonUnmarshalFlow(out.Bytes(),&answer);err!=nil{t.Fatal(err)}
    if answer.Written||answer.ExitAllowed{t.Fatalf("read-only gate improperly granted exit: %+v",answer)}
    return answer
}

func jsonUnmarshalFlow(data []byte,result *flowNextDecision)error {
    return json.Unmarshal(data,result)
}

func TestControllerFlowBlocksRetainedPreservedB541BeforeB542Completion(t *testing.T) {
    repo,worker,archive,plan,_:=controllerFlowFixture(t)
    initial:=flowReport(t,repo,"B-541",worker,archive)
    if initial.Decision!="EXIT_BLOCKED"||!slices.ContainsFunc(initial.Reasons,func(s string)bool{return strings.HasPrefix(s,"preservation_manifest_unavailable")}) {
        t.Fatalf("missing preservation must block: %+v",initial)
    }
    args:=[]string{"--project",repo,"--into","main","--target",worker,
        "--archive",archive,"--expect-plan",plan.PlanDigest,"--approve","--json"}
    code,preserved,errText:=runCleanupTest(t,append([]string{"preserve"},args...)...)
    if code!=0||preserved.Status!="preserved"{t.Fatalf("preserve failed: %d %+v %s",code,preserved,errText)}
    next:=flowReport(t,repo,"B-541",worker,archive)
    if next.Decision!="EXIT_BLOCKED"||next.CleanupStatus!="preserved_retained"||
        !slices.Contains(next.Reasons,"source_worktree_retained")||next.EvidenceQuality!="verified_archive_sha256" {
        t.Fatalf("copied files falsely finished cleanup: %+v",next)
    }
    ongoing:=flowReport(t,repo,"B-542",worker,archive)
    if ongoing.Decision!="CONTINUE"||ongoing.CleanupStatus!="not_due"||
        !slices.Contains(ongoing.Reasons,"task_not_finalized"){
        t.Fatalf("ongoing B-542 must not be cleaned: %+v",ongoing)
    }
    if _,err:=os.Stat(worker);err!=nil{t.Fatalf("read-only flow deleted worktree: %v",err)}
}

func TestControllerFlowRequiresVerifiedRemovalReceipt(t *testing.T) {
    repo,worker,archive,plan,_:=controllerFlowFixture(t)
    args:=[]string{"--project",repo,"--into","main","--target",worker,"--archive",archive,
        "--expect-plan",plan.PlanDigest,"--approve","--json"}
    if code,preserved,msg:=runCleanupTest(t,append([]string{"preserve"},args...)...);code!=0||preserved.Status!="preserved" {
        t.Fatalf("preservation: %d %+v %s",code,preserved,msg)
    }
    if code,removed,msg:=runCleanupTest(t,append(append([]string{"remove"},args...),"--confirm-unused")...);code!=0||removed.Status!="removed" {
        t.Fatalf("safe remove: %d %+v %s",code,removed,msg)
    }
    decision:=flowReport(t,repo,"B-541",worker,archive)
    if decision.CleanupStatus!="removed_verified"||decision.Decision!="CONTINUE"||
        decision.EvidenceQuality!="git_absent_plus_manifest_and_receipt"{
        t.Fatalf("successful cleanup subgate not recognized: %+v",decision)
    }
    // A forged/conflicting receipt may not grant a green status even though
    // the target no longer exists. No changes to project or worktree allowed.
    receipt:=filepath.Join(archive,"removal.json")
    if err:=os.WriteFile(receipt,[]byte(`{"target":"/tmp/unrelated","repository":"fake","method":"git worktree remove (no force)"}`),0600);err!=nil{t.Fatal(err)}
    decision=flowReport(t,repo,"B-541",worker,archive)
    if decision.CleanupStatus=="removed_verified"||!slices.Contains(decision.Reasons,"removal_receipt_identity_invalid") {
        t.Fatalf("forged receipt accepted: %+v",decision)
    }
}

func TestControllerFlowBlocksCherryPickAndIgnoredFilesDespiteCleanGitStatus(t *testing.T) {
    repo,worker,archive,_,_:=controllerFlowFixture(t)
    // Commit a worker change then cherry-pick it into the integration target.
    // Content is integrated, but the worker HEAD is not an ancestor of main.
    if err:=os.WriteFile(filepath.Join(worker,"feature.txt"),[]byte("same change\n"),0600);err!=nil{t.Fatal(err)}
    gitFixtureCommand(t,worker,"add","feature.txt")
    gitFixtureCommand(t,worker,"-c","user.name=Fixture","-c","user.email=fixture@example.invalid",
        "-c","commit.gpgsign=false","commit","-m","worker feature")
    workerSHA,err:=gitReadOnly(worker,"rev-parse","HEAD")
    if err!=nil{t.Fatal(err)}
    // Diverge integration history before cherry-picking the worker commit.
    // Without this, same parent/author/time can produce an identical SHA.
    if err:=os.WriteFile(filepath.Join(repo,"integration-only.txt"),[]byte("main diverged\\n"),0600);err!=nil{t.Fatal(err)}
    gitFixtureCommand(t,repo,"add","integration-only.txt")
    gitFixtureCommand(t,repo,"-c","user.name=Fixture","-c","user.email=fixture@example.invalid",
        "-c","commit.gpgsign=false","commit","-m","integration branch advanced")
    gitFixtureCommand(t,repo,"-c","user.name=Fixture","-c","user.email=fixture@example.invalid",
        "-c","commit.gpgsign=false","cherry-pick",strings.TrimSpace(string(workerSHA)))
    // Shared exclude file applies to all worktrees, and keeps the ordinary
    // status clean while the real scanner still sees ignored payload.
    exclude:=filepath.Join(t.TempDir(),"git-global-excludes")
    if err:=os.WriteFile(exclude,[]byte("*.cache\n"),0600);err!=nil{t.Fatal(err)}
    gitFixtureCommand(t,repo,"config","core.excludesfile",exclude)
    cache:=filepath.Join(worker,"worker-generated.cache")
    if err:=os.WriteFile(cache,[]byte("456 generated files are not disposable by assumption"),0600);err!=nil{t.Fatal(err)}
    ordinary,err:=gitReadOnly(worker,"status","--porcelain")
    if err!=nil||len(ordinary)!=0{t.Fatalf("expected superficially clean Git status %q %v",ordinary,err)}
    plan,err:=inspectWorktrees(repo,"main",200000)
    if err!=nil{t.Fatal(err)}
    var w worktreeEntry
    for _,entry:=range plan.Worktrees{if entry.Path==worker{w=entry}}
    if w.Merged!="no"||w.Ignored==0||!slices.Contains(w.ProtectionReasons,"unmerged_head")||
        !slices.Contains(w.ProtectionReasons,"ignored_payload"){
        t.Fatalf("cherry-pick/ignored evidence missing: %+v",w)
    }
    args:=[]string{"--project",repo,"--into","main","--target",worker,
        "--archive",archive,"--expect-plan",plan.PlanDigest,"--approve","--json"}
    if code,backed,msg:=runCleanupTest(t,append([]string{"preserve"},args...)...);code!=0||backed.Status!="preserved"{
        t.Fatalf("preserve content and cache: %d %+v %s",code,backed,msg)
    }
    decision:=flowReport(t,repo,"B-541",worker,archive)
    if decision.CleanupStatus!="preserved_retained"||decision.Decision!="EXIT_BLOCKED"||
       !slices.Contains(decision.Reasons,"unmerged_head")||
       !slices.Contains(decision.Reasons,"ignored_payload")||
       !strings.Contains(decision.NextAction,"cherry-picked"){
        t.Fatalf("cherry-pick falsely authorized cleanup: %+v",decision)
    }
    if data,err:=os.ReadFile(cache);err!=nil||len(data)==0{t.Fatalf("ignored payload lost: %v",err)}
}

func TestControllerFlowFailsClosedOnOtherRolesAndUnknownTasks(t *testing.T) {
    repo,worker,archive,_,_:=controllerFlowFixture(t)
    nope:=flowReport(t,repo,"B-999",worker,archive)
    if !slices.Contains(nope.Reasons,"canonical_task_not_found"){t.Fatalf("unknown task accepted: %+v",nope)}
    var out,errors bytes.Buffer
    code:=runFlowNextCLI([]string{"next","--role","worker","--event","final_requested",
        "--project",repo,"--task","B-541","--json"},&out,&errors)
    if code!=0||!strings.Contains(out.String(),`"unsupported_role_event_contract"`) {
        t.Fatalf("unimplemented role must fail closed: %d %s %s",code,out.String(),errors.String())
    }
}
