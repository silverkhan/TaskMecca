package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func verifiedCleanupFixture(t *testing.T)(string,string,string,worktreePlan){
	t.Helper()
	if _,err:=exec.LookPath("git");err!=nil{t.Skip("git not installed")}
	t.Setenv("TASK_MECCA_HOME",t.TempDir())
	repo:=filepath.Join(t.TempDir(),"repo")
	if err:=os.MkdirAll(repo,0755);err!=nil{t.Fatal(err)}
	gitFixtureCommand(t,repo,"init","-b","main")
	if err:=os.WriteFile(filepath.Join(repo,"README.md"),[]byte("safe fixture\n"),0600);err!=nil{t.Fatal(err)}
	gitFixtureCommand(t,repo,"add","README.md")
	gitFixtureCommand(t,repo,"-c","user.name=Fixture","-c","user.email=fixture@example.invalid","-c","commit.gpgsign=false","commit","-m","base")
	worker:=filepath.Join(t.TempDir(),"secondary")
	gitFixtureCommand(t,repo,"worktree","add","-b","branch-safe",worker,"main")
	archive:=filepath.Join(t.TempDir(),"archive")
	plan,err:=inspectWorktrees(repo,"main",200000)
	if err!=nil{t.Fatal(err)}
	if len(plan.Worktrees)!=2{t.Fatalf("expected 2 worktrees, got %+v",plan)}
	return repo,worker,archive,plan
}
func runCleanupTest(t *testing.T,args ...string)(int,worktreeCleanupResult,string){
	t.Helper()
	var out,errs bytes.Buffer
	code:=runWorktreeCleanupCLI(args,&out,&errs)
	var result worktreeCleanupResult
	if out.Len()>0 {
		if err:=json.Unmarshal(out.Bytes(),&result);err!=nil{t.Fatalf("invalid JSON %s: %v",out.String(),err)}
	}
	return code,result,errs.String()
}

func TestWorktreePreserveAndNormalRemoveCleanOnly(t *testing.T){
	repo,worker,archive,plan:=verifiedCleanupFixture(t)
	args:=[]string{"--project",repo,"--into","main","--target",worker,"--archive",archive,"--expect-plan",plan.PlanDigest,"--approve","--json"}
	code,r,msg:=runCleanupTest(t,append([]string{"preserve"},args...)...)
	if code!=0||r.Status!="preserved"||!r.Verified||r.FileCount<1{t.Fatalf("preserve %+v code=%d: %s",r,code,msg)}
	manifest,err:=readWorktreePreservation(archive)
	if err!=nil||len(manifest.Files)<1{t.Fatalf("manifest %+v %v",manifest,err)}
	code,r,msg=runCleanupTest(t,"verify","--archive",archive,"--json")
	if code!=0||r.Status!="verified"{t.Fatalf("verify %+v code=%d: %s",r,code,msg)}
	code,r,_=runCleanupTest(t,append([]string{"remove"},args...)...)
	if code==0{t.Fatal("removed without explicit unused attestation")}
	code,r,msg=runCleanupTest(t,append(append([]string{"remove"},args...),"--confirm-unused")...)
	if code!=0||r.Status!="removed"||!r.Removed{t.Fatalf("normal Git remove failed: %+v code=%d: %s",r,code,msg)}
	if _,err:=os.Stat(worker);!os.IsNotExist(err){t.Fatalf("Git worktree still on disk after normal removal: %v",err)}
	if _,err:=os.Stat(filepath.Join(archive,"removal.json"));err!=nil{t.Fatal("removal audit receipt missing")}
	code,r,msg=runCleanupTest(t,append(append([]string{"remove"},args...),"--confirm-unused")...)
	if code!=0||r.Status!="already_removed"{t.Fatalf("idempotent retry %+v code=%d: %s",r,code,msg)}
	// No branch/commit deletion is permitted; a Git show-ref must still succeed.
	gitFixtureCommand(t,repo,"show-ref","--verify","refs/heads/branch-safe")
}

func TestWorktreeDirtyPreservedButNotRemoved(t *testing.T){
	repo,worker,archive,plan:=verifiedCleanupFixture(t)
	unique:=filepath.Join(worker,"unique_payload.txt")
	if err:=os.WriteFile(unique,[]byte("irreplaceable"),0600);err!=nil{t.Fatal(err)}
	plan,err:=inspectWorktrees(repo,"main",200000)
	if err!=nil{t.Fatal(err)}
	args:=[]string{"--project",repo,"--into","main","--target",worker,"--archive",archive,"--expect-plan",plan.PlanDigest,"--approve","--json"}
	code,r,msg:=runCleanupTest(t,append([]string{"preserve"},args...)...)
	if code!=0||r.Status!="preserved"{t.Fatalf("cannot safely preserve dirty worktree %+v code=%d %s",r,code,msg)}
	code,r,msg=runCleanupTest(t,append(append([]string{"remove"},args...),"--confirm-unused")...)
	if code==0||r.Status!="blocked"||r.Removed{t.Fatalf("dirty worktree unexpectedly removed %+v code=%d %s",r,code,msg)}
	if data,err:=os.ReadFile(unique);err!=nil||string(data)!="irreplaceable"{t.Fatal("original unique payload missing")}
	if data,err:=os.ReadFile(filepath.Join(archive,"payload","unique_payload.txt"));err!=nil||string(data)!="irreplaceable"{t.Fatal("backup unique payload missing")}
	// Altering the archive invalidates verification and therefore removal.
	if err:=os.WriteFile(filepath.Join(archive,"payload","unique_payload.txt"),[]byte("changed"),0600);err!=nil{t.Fatal(err)}
	code,r,msg=runCleanupTest(t,"verify","--archive",archive,"--json")
	if code==0||r.Verified||!strings.Contains(strings.Join(r.Reasons,","),"hash mismatch"){t.Fatalf("archive tampering accepted %+v code=%d %s",r,code,msg)}
}
