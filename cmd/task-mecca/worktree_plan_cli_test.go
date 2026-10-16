package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func gitFixtureCommand(t *testing.T,path string,args ...string){
	t.Helper()
	cmd:=exec.Command("git",append([]string{"-C",path},args...)...)
	if output,err:=cmd.CombinedOutput();err!=nil{t.Fatalf("git %s failed: %v\n%s",strings.Join(args," "),err,output)}
}

func TestParseWorktreePorcelainKeepsDetachedAndLockReasons(t *testing.T){
	raw:=[]byte("worktree /repo\x00HEAD abc123\x00branch refs/heads/dev\x00\x00worktree /tmp/w\x00HEAD def456\x00detached\x00locked external-use\x00\x00")
	rows:=parseWorktreePorcelain(raw)
	if len(rows)!=2||rows[0].Branch!="refs/heads/dev"||rows[1].Path!="/tmp/w"||!rows[1].Detached||!rows[1].Locked{
		t.Fatalf("porcelain parsing incorrect: %+v",rows)
	}
}

func TestWorktreePlanConservativelyProtectsUnknownUsageAndUntracked(t *testing.T){
	if _,err:=exec.LookPath("git");err!=nil{t.Skip("Git not available")}
	repo:=filepath.Join(t.TempDir(),"repo")
	if err:=os.MkdirAll(repo,0755);err!=nil{t.Fatal(err)}
	gitFixtureCommand(t,repo,"init","-b","main")
	if err:=os.WriteFile(filepath.Join(repo,"README.md"),[]byte("base\n"),0644);err!=nil{t.Fatal(err)}
	gitFixtureCommand(t,repo,"add","README.md")
	gitFixtureCommand(t,repo,"-c","user.name=Fixture","-c","user.email=fixture@example.invalid","-c","commit.gpgsign=false","commit","-m","base")
	worker:=filepath.Join(t.TempDir(),"worker")
	gitFixtureCommand(t,repo,"worktree","add","-b","test-worker",worker,"main")
	plan,err:=inspectWorktrees(repo,"main",10000)
	if err!=nil{t.Fatal(err)}
	if plan.Written||plan.DeletionAllowed||len(plan.Worktrees)!=2||len(plan.PlanDigest)!=64{t.Fatalf("invalid plan %+v",plan)}
	for _,row:=range plan.Worktrees{
		if row.Decision=="eligible"||row.Decision=="safe_to_delete"{t.Fatalf("unverified worktree marked safe %+v",row)}
		if !slices.Contains(row.ProtectionReasons,"usage_not_verified"){t.Fatalf("usage unknown not protected: %+v",row)}
	}
	untracked:=filepath.Join(worker,"unique_payload.txt")
	if err:=os.WriteFile(untracked,[]byte("preserve"),0644);err!=nil{t.Fatal(err)}
	plan,err=inspectWorktrees(repo,"main",10000)
	if err!=nil{t.Fatal(err)}
	found:=false
	for _,row:=range plan.Worktrees{
		if row.Path==worker {
			found=true
			if row.Decision!="protected"||row.Untracked==0||!slices.Contains(row.ProtectionReasons,"untracked_payload"){t.Fatalf("untracked file not protected: %+v",row)}
		}
	}
	if !found{t.Fatal("worktree missing")}
	if _,err:=os.Stat(untracked);err!=nil{t.Fatal("read-only inventory changed file")}
}
