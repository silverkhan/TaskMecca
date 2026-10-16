package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type worktreeEntry struct {
	Path string `json:"path"`
	HEAD string `json:"head"`
	Branch string `json:"branch,omitempty"`
	Detached bool `json:"detached"`
	Locked bool `json:"locked"`
	Prunable bool `json:"prunable"`
	Primary bool `json:"primary"`
	Merged string `json:"merged"`
	TrackedChanged int `json:"tracked_changed"`
	Untracked int `json:"untracked"`
	Ignored int `json:"ignored"`
	Files int64 `json:"files"`
	Bytes int64 `json:"bytes"`
	Symlinks int64 `json:"symlinks"`
	NestedRepositories int64 `json:"nested_repositories"`
	ScanComplete bool `json:"scan_complete"`
	Usage string `json:"usage"`
	Decision string `json:"decision"`
	ProtectionReasons []string `json:"protection_reasons"`
	Findings []string `json:"findings"`
}

type worktreePlan struct {
	Status string `json:"status"`
	Project string `json:"project"`
	Repository string `json:"repository"`
	IntegrationRef string `json:"integration_ref"`
	PlanDigest string `json:"plan_digest"`
	AsOf string `json:"as_of"`
	Worktrees []worktreeEntry `json:"worktrees"`
	Written bool `json:"written"`
	DeletionAllowed bool `json:"deletion_allowed"`
}

func runWorktreePlanCLI(args []string,out,stderr io.Writer)int{
	if len(args)==0||args[0]=="--help"||args[0]=="help"{
		fmt.Fprintln(out,"task-mecca worktree plan --project DIR [--into origin/dev] [--max-files 200000] --json")
		fmt.Fprintln(out,"Read-only inventory, all unresolved usage remains review_required; no worktree is deleted.")
		return 0
	}
	if args[0]!="plan"{fmt.Fprintln(stderr,"worktree only supports plan in this release; no delete/force option");return 2}
	flags:=flag.NewFlagSet("worktree plan",flag.ContinueOnError)
	flags.SetOutput(stderr)
	project:=flags.String("project",".","project within Git repository")
	into:=flags.String("into","origin/dev","integration ref for ancestry check")
	maxFiles:=flags.Int("max-files",200000,"max inventory entries per worktree")
	asJSON:=flags.Bool("json",false,"machine-readable report")
	if err:=flags.Parse(args[1:]);err!=nil{return 2}
	if flags.NArg()!=0||*maxFiles<=0||*maxFiles>1000000||strings.HasPrefix(*into,"-")||*into==""{
		fmt.Fprintln(stderr,"invalid worktree plan options");return 2
	}
	abs,err:=filepath.Abs(*project)
	if err!=nil{fmt.Fprintln(stderr,err);return 2}
	plan,err:=inspectWorktrees(abs,*into,*maxFiles)
	if err!=nil{fmt.Fprintln(stderr,err);return 1}
	if *asJSON{if err:=json.NewEncoder(out).Encode(plan);err!=nil{fmt.Fprintln(stderr,err);return 1};return 0}
	fmt.Fprintf(out,"Worktree inventory: %d entries, no files changed or removed\n",len(plan.Worktrees))
	for _,e:=range plan.Worktrees{fmt.Fprintf(out,"%s %s merged=%s files=%d bytes=%d reasons=%s\n",e.Decision,e.Path,e.Merged,e.Files,e.Bytes,strings.Join(e.ProtectionReasons,","))}
	fmt.Fprintf(out,"plan_digest=%s\n",plan.PlanDigest)
	return 0
}

func gitReadOnly(at string,args ...string)([]byte,error){
	ctx,cancel:=context.WithTimeout(context.Background(),25*time.Second)
	defer cancel()
	opts:=append([]string{"-c","core.fsmonitor=false","-C",at},args...)
	cmd:=exec.CommandContext(ctx,"git",opts...)
	cmd.Env=append(os.Environ(),"GIT_OPTIONAL_LOCKS=0","GIT_TERMINAL_PROMPT=0")
	result,err:=cmd.Output()
	if ctx.Err()!=nil{return nil,ctx.Err()}
	return result,err
}

func parseWorktreePorcelain(raw []byte)[]worktreeEntry{
	entries:=[]worktreeEntry{}
	var current *worktreeEntry
	for _,record:=range bytes.Split(raw,[]byte{0}){
		text:=string(record)
		if text==""{
			if current!=nil{entries=append(entries,*current);current=nil}
			continue
		}
		if strings.HasPrefix(text,"worktree "){
			if current!=nil{entries=append(entries,*current)}
			current=&worktreeEntry{Path:strings.TrimPrefix(text,"worktree "),ProtectionReasons:[]string{},Findings:[]string{},Usage:"unknown",Merged:"unknown"}
			continue
		}
		if current==nil{continue}
		switch{
		case strings.HasPrefix(text,"HEAD "):current.HEAD=strings.TrimPrefix(text,"HEAD ")
		case strings.HasPrefix(text,"branch "):current.Branch=strings.TrimPrefix(text,"branch ")
		case text=="detached":current.Detached=true
		case strings.HasPrefix(text,"locked"):current.Locked=true
		case strings.HasPrefix(text,"prunable"):current.Prunable=true
		}
	}
	if current!=nil{entries=append(entries,*current)}
	return entries
}

func inspectWorktrees(project,into string,maxFiles int)(worktreePlan,error){
	out:=worktreePlan{Status:"inspected",Project:project,IntegrationRef:into,AsOf:time.Now().UTC().Format(time.RFC3339Nano),Written:false,DeletionAllowed:false,Worktrees:[]worktreeEntry{}}
	raw,err:=gitReadOnly(project,"rev-parse","--show-toplevel")
	if err!=nil{return out,fmt.Errorf("not a readable Git project: %w",err)}
	repo:=strings.TrimSpace(string(raw))
	out.Repository=repo
	porcelain,err:=gitReadOnly(repo,"worktree","list","--porcelain","-z")
	if err!=nil{return out,err}
	entries:=parseWorktreePorcelain(porcelain)
	if len(entries)==0{return out,fmt.Errorf("git worktree inventory is empty or unparseable")}
	for i:=range entries{
		e:=&entries[i]
		e.Primary=i==0
		if e.Primary{e.ProtectionReasons=append(e.ProtectionReasons,"primary_worktree")}
		if e.Detached{e.ProtectionReasons=append(e.ProtectionReasons,"detached_head")}
		if e.Locked{e.ProtectionReasons=append(e.ProtectionReasons,"locked_worktree")}
		if e.Prunable{e.ProtectionReasons=append(e.ProtectionReasons,"prunable_requires_git_review")}
		if e.HEAD==""||e.HEAD==strings.Repeat("0",40){e.ProtectionReasons=append(e.ProtectionReasons,"missing_head")}
		if stat,err:=os.Lstat(e.Path);err!=nil{
			e.ProtectionReasons=append(e.ProtectionReasons,"missing_or_unreadable_path")
			e.Findings=append(e.Findings,err.Error())
		}else if !stat.IsDir()||stat.Mode()&os.ModeSymlink!=0{
			e.ProtectionReasons=append(e.ProtectionReasons,"symlink_or_not_directory")
		}
		if e.HEAD!=""{
			if _,err:=gitReadOnly(repo,"merge-base","--is-ancestor",e.HEAD,into);err==nil{
				e.Merged="yes"
			}else if exit,ok:=err.(*exec.ExitError);ok&&exit.ExitCode()==1{
				e.Merged="no";e.ProtectionReasons=append(e.ProtectionReasons,"unmerged_head")
			}else{
				e.Merged="unknown";e.ProtectionReasons=append(e.ProtectionReasons,"merge_check_unavailable")
			}
		}
		status,err:=gitReadOnly(e.Path,"status","--porcelain=v1","-z","--untracked-files=all","--ignored")
		if err!=nil{e.Findings=append(e.Findings,"git status unavailable: "+err.Error());e.ProtectionReasons=append(e.ProtectionReasons,"git_status_unknown")}else{
			for _,part:=range bytes.Split(status,[]byte{0}){
				if len(part)<3{continue}
				prefix:=string(part[:2])
				switch prefix{
				case "??":e.Untracked++
				case "!!":e.Ignored++
				default:
					// Rename destination records and filenames never confer clean status.
					if part[0]!=' '||part[1]!=' '{e.TrackedChanged++}
				}
			}
			if e.TrackedChanged>0{e.ProtectionReasons=append(e.ProtectionReasons,"tracked_modified")}
			if e.Untracked>0{e.ProtectionReasons=append(e.ProtectionReasons,"untracked_payload")}
			if e.Ignored>0{e.ProtectionReasons=append(e.ProtectionReasons,"ignored_payload")}
		}
		if len(e.Findings)==0{
			inventoryWorktreePayload(e,maxFiles)
		}
		if !e.ScanComplete{e.ProtectionReasons=append(e.ProtectionReasons,"payload_scan_incomplete")}
		if e.Symlinks>0{e.ProtectionReasons=append(e.ProtectionReasons,"symlink_payload")}
		if e.NestedRepositories>0{e.ProtectionReasons=append(e.ProtectionReasons,"nested_repository")}
		// Git cannot establish whether a native provider is actively using this
		// worktree. An explicit separate Controller confirmation is mandatory.
		e.ProtectionReasons=append(e.ProtectionReasons,"usage_not_verified")
		e.Decision="review_required"
		if len(e.ProtectionReasons)>1{e.Decision="protected"}
	}
	sort.Slice(entries,func(i,j int)bool{return entries[i].Path<entries[j].Path})
	out.Worktrees=entries
	serialized,_:=json.Marshal(struct{
		Project string
		Repo string
		Into string
		Entries []worktreeEntry
	}{project,repo,into,entries})
	hash:=sha256.Sum256(serialized)
	out.PlanDigest=hex.EncodeToString(hash[:])
	return out,nil
}

func inventoryWorktreePayload(e *worktreeEntry,maxFiles int){
	e.ScanComplete=true
	seen:=0
	err:=filepath.WalkDir(e.Path,func(path string,entry fs.DirEntry,walkErr error)error{
		if walkErr!=nil{return walkErr}
		relative,_:=filepath.Rel(e.Path,path)
		if relative=="."{return nil}
		if filepath.Base(path)==".git"{
			if filepath.Dir(path)==e.Path{return filepath.SkipDir}
			e.NestedRepositories++
			return filepath.SkipDir
		}
		seen++
		if seen>maxFiles{return fmt.Errorf("inventory limit %d reached",maxFiles)}
		if entry.Type()&os.ModeSymlink!=0{e.Symlinks++;return nil}
		if entry.IsDir(){return nil}
		info,err:=entry.Info();if err!=nil{return err}
		if info.Mode().IsRegular(){e.Files++;e.Bytes+=info.Size()}
		return nil
	})
	if err!=nil{e.ScanComplete=false;e.Findings=append(e.Findings,err.Error())}
}
