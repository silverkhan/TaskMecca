package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

	"github.com/silverkhan/TaskMecca/internal/projectguard"
)

type worktreePreservedFile struct {
	Path string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes int64 `json:"bytes"`
}
type worktreePreservation struct {
	SchemaVersion int `json:"schema_version"`
	Repository string `json:"repository"`
	Target string `json:"target"`
	Archive string `json:"archive"`
	HEAD string `json:"head"`
	PlanDigest string `json:"plan_digest"`
	CreatedAt string `json:"created_at"`
	Files []worktreePreservedFile `json:"files"`
	PayloadDigest string `json:"payload_digest"`
	RemovedAt string `json:"removed_at,omitempty"`
}
type worktreeCleanupResult struct {
	Status string `json:"status"`
	Target string `json:"target"`
	Archive string `json:"archive"`
	PlanDigest string `json:"plan_digest"`
	FileCount int `json:"file_count"`
	Bytes int64 `json:"bytes"`
	Verified bool `json:"verified"`
	Removed bool `json:"removed"`
	Reasons []string `json:"reasons"`
	NextAction string `json:"next_action,omitempty"`
}

func runWorktreeCleanupCLI(args []string,out,stderr io.Writer)int{
	action:=args[0]
	if action!="preserve"&&action!="verify"&&action!="remove" {
		fmt.Fprintln(stderr,"worktree supports plan, preserve, verify, remove; no force/reset/branch delete")
		return 2
	}
	flags:=flag.NewFlagSet("worktree "+action,flag.ContinueOnError)
	flags.SetOutput(stderr)
	project:=flags.String("project",".","Git project root")
	into:=flags.String("into","origin/dev","integration ref matching the approved plan")
	target:=flags.String("target","","exact worktree path (must be separately listed by Git)")
	archive:=flags.String("archive","","protected archive path outside all worktrees")
	expect:=flags.String("expect-plan","","plan_digest from worktree plan")
	approve:=flags.Bool("approve",false,"explicit approval for archive creation or exact cleanup")
	unused:=flags.Bool("confirm-unused",false,"user/operator attests no Agent is using exact target")
	asJSON:=flags.Bool("json",false,"machine-readable status")
	if err:=flags.Parse(args[1:]);err!=nil{return 2}
	if flags.NArg()!=0||*archive==""{fmt.Fprintln(stderr,"--archive and named flags required");return 2}
	archiveAbs,err:=filepath.Abs(*archive)
	if err!=nil{fmt.Fprintln(stderr,err);return 2}
	if action=="verify"{
		manifest,err:=readWorktreePreservation(archiveAbs)
		if err!=nil{fmt.Fprintln(stderr,err);return 3}
		result:=verifyWorktreePreservation(manifest,false)
		return printWorktreeCleanupResult(result,*asJSON,out,stderr)
	}
	if *target==""||*expect==""||!*approve {
		fmt.Fprintln(stderr,"preserve/remove require --target, --expect-plan from worktree plan, and --approve")
		return 2
	}
	if action=="remove"&&!*unused{
		fmt.Fprintln(stderr,"remove requires --confirm-unused after explicit operator verification that no Agent uses target")
		return 3
	}
	projectAbs,err:=filepath.Abs(*project)
	if err!=nil{fmt.Fprintln(stderr,err);return 2}
	targetAbs,err:=filepath.Abs(*target)
	if err!=nil{fmt.Fprintln(stderr,err);return 2}
	// Hold the global exclusive management fence through the entire critical
	// operation. The confirmation does not pretend external Git users obey it.
	unlock,err:=projectguard.AcquireManagement()
	if err!=nil{fmt.Fprintln(stderr,err);return 3}
	defer unlock()
	plan,err:=inspectWorktrees(projectAbs,*into,200000)
	if err!=nil{fmt.Fprintln(stderr,err);return 3}
	// Find the exact Git-listed target before checking freshness. Once a
	// verified remove succeeded, the worktree-list digest legitimately changes;
	// identical retries can recognize that terminal state from Git+archive.
	var selected *worktreeEntry
	for i:=range plan.Worktrees{
		if filepath.Clean(plan.Worktrees[i].Path)==filepath.Clean(targetAbs){selected=&plan.Worktrees[i];break}
	}
	if plan.PlanDigest!=*expect&&!(action=="remove"&&selected==nil){
		fmt.Fprintf(stderr,"STALE_PLAN: run worktree plan --into %s again\n",*into);return 3
	}
	if err=validateArchiveOutsideWorktrees(plan,archiveAbs);err!=nil{fmt.Fprintln(stderr,err);return 3}
	if action=="preserve"{
		if selected==nil{fmt.Fprintln(stderr,"target not listed by git worktree");return 3}
		if selected.Primary||selected.HEAD==""||!selected.ScanComplete{fmt.Fprintln(stderr,"protected primary/incomplete worktree: preservation requires review");return 3}
		result,err:=preserveWorktree(plan,*selected,archiveAbs)
		if err!=nil{result.Status="partial";result.Reasons=append(result.Reasons,err.Error());result.NextAction="Preserve original worktree; inspect archive and retry the exact same target/plan only if unchanged."}
		return printWorktreeCleanupResult(result,*asJSON,out,stderr)
	}
	manifest,err:=readWorktreePreservation(archiveAbs)
	if err!=nil{fmt.Fprintln(stderr,err);return 3}
	if manifest.Target!=targetAbs||manifest.Repository!=plan.Repository||manifest.PlanDigest!=*expect {
		fmt.Fprintln(stderr,"PRESERVATION_CONFLICT: archive belongs to a different exact worktree or plan");return 3
	}
	result:=verifyWorktreePreservation(manifest,selected!=nil)
	if result.Status!="verified"{return printWorktreeCleanupResult(result,*asJSON,out,stderr)}
	if selected==nil{
		result.Status="already_removed";result.Removed=true
		if err:=writeWorktreeRemovalReceipt(archiveAbs,manifest);err!=nil{
			result.Status="unknown";result.Reasons=append(result.Reasons,"removal receipt could not be verified: "+err.Error())
		}
		return printWorktreeCleanupResult(result,*asJSON,out,stderr)
	}
	if selected.HEAD!=manifest.HEAD||selected.Primary||selected.Detached||selected.Locked||selected.Merged!="yes"||
		selected.TrackedChanged>0||selected.Untracked>0||selected.Ignored>0||selected.Symlinks>0||selected.NestedRepositories>0||
		!selected.ScanComplete||len(selected.Findings)>0{
		result.Status="blocked"
		result.Reasons=append(result.Reasons,"Git state is not clean, merged, unlinked and independently removable; no force cleanup is allowed")
		result.NextAction="Reconcile dirty/unmerged/unknown worktree manually without deleting the preserved copy; rerun plan."
		return printWorktreeCleanupResult(result,*asJSON,out,stderr)
	}
	// Last-moment plan and byte verification before the only destructive step.
	again,err:=inspectWorktrees(plan.Repository,plan.IntegrationRef,200000)
	if err!=nil||again.PlanDigest!=plan.PlanDigest{
		result.Status="blocked";result.Reasons=append(result.Reasons,"TOCTOU: worktree state changed before remove")
		return printWorktreeCleanupResult(result,*asJSON,out,stderr)
	}
	check:=verifyWorktreePreservation(manifest,true)
	if check.Status!="verified"{return printWorktreeCleanupResult(check,*asJSON,out,stderr)}
	ctx,cancel:=context.WithTimeout(context.Background(),120*time.Second)
	defer cancel()
	cmd:=exec.CommandContext(ctx,"git","-C",plan.Repository,"worktree","remove","--",targetAbs)
	cmd.Env=append(os.Environ(),"GIT_TERMINAL_PROMPT=0")
	if output,err:=cmd.CombinedOutput();err!=nil{
		result.Status="blocked"
		result.Reasons=append(result.Reasons,fmt.Sprintf("git worktree remove refused: %s (%v)",strings.TrimSpace(string(output)),err))
		result.NextAction="No force used. Inspect Git refusal and protected data; do not manually delete directory."
		return printWorktreeCleanupResult(result,*asJSON,out,stderr)
	}
	listed,err:=gitReadOnly(plan.Repository,"worktree","list","--porcelain","-z")
	if err!=nil{
		result.Status="unknown";result.Reasons=append(result.Reasons,"Git post-removal verification failed: "+err.Error())
		return printWorktreeCleanupResult(result,*asJSON,out,stderr)
	}
	for _,e:=range parseWorktreePorcelain(listed){
		if filepath.Clean(e.Path)==targetAbs{
			result.Status="unknown";result.Reasons=append(result.Reasons,"Git still lists target after remove");return printWorktreeCleanupResult(result,*asJSON,out,stderr)
		}
	}
	result.Status="removed";result.Removed=true;result.Verified=true
	if err:=writeWorktreeRemovalReceipt(archiveAbs,manifest);err!=nil{
		result.Status="unknown";result.Reasons=append(result.Reasons,"Git removal completed but receipt could not be recorded: "+err.Error())
	}
	return printWorktreeCleanupResult(result,*asJSON,out,stderr)
}

func printWorktreeCleanupResult(result worktreeCleanupResult,asJSON bool,out,stderr io.Writer)int{
	if asJSON{
		if err:=json.NewEncoder(out).Encode(result);err!=nil{fmt.Fprintln(stderr,err);return 1}
	}else{
		fmt.Fprintf(out,"%s: %s preserved=%d files, removed=%t reasons=%s\n",result.Status,result.Target,result.FileCount,result.Removed,strings.Join(result.Reasons,", "))
	}
	if result.Status=="verified"||result.Status=="preserved"||result.Status=="removed"||result.Status=="already_removed"{return 0}
	return 3
}

func validateArchiveOutsideWorktrees(plan worktreePlan,archive string)error{
	if info,err:=os.Lstat(archive);err==nil&&info.Mode()&os.ModeSymlink!=0{return errors.New("archive must not be a symlink")}
	candidates:=append([]string{plan.Repository},func()[]string{a:=[]string{};for _,w:=range plan.Worktrees{a=append(a,w.Path)};return a}()...)
	for _,root:=range candidates{
		rel,err:=filepath.Rel(root,archive)
		if err==nil&&(rel=="."||(!strings.HasPrefix(rel,".."+string(filepath.Separator))&&rel!="..")){
			return fmt.Errorf("archive must be outside every Git worktree: %s",root)
		}
	}
	for parent:=archive; ;parent=filepath.Dir(parent){
		info,err:=os.Lstat(parent)
		if err==nil&&info.Mode()&os.ModeSymlink!=0{return fmt.Errorf("archive path contains symlink: %s",parent)}
		if err!=nil&&!os.IsNotExist(err){return err}
		if filepath.Dir(parent)==parent{break}
	}
	return nil
}

func rejectPathSymlinksWithin(root,path string)error{
	rel,err:=filepath.Rel(root,path)
	if err!=nil||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return fmt.Errorf("unsafe path outside selected root: %s",path)}
	for current:=root; ;{
		info,e:=os.Lstat(current)
		if e==nil&&info.Mode()&os.ModeSymlink!=0{return fmt.Errorf("symlink in protected path: %s",current)}
		if e!=nil&&!os.IsNotExist(e){return e}
		if current==path{break}
		remainder,_:=filepath.Rel(current,path)
		parts:=strings.Split(remainder,string(filepath.Separator))
		current=filepath.Join(current,parts[0])
	}
	return nil
}

func preservationManifestPath(archive string)string{return filepath.Join(archive,"manifest.json")}
func preservedFilePath(archive,rel string)string{return filepath.Join(archive,"payload",filepath.FromSlash(rel))}
func hashFileStream(path string)(string,int64,error){
	file,err:=os.Open(path)
	if err!=nil{return "",0,err}
	h:=sha256.New()
	size,err:=io.Copy(h,file)
	closeErr:=file.Close()
	if err!=nil{return "",0,err}
	if closeErr!=nil{return "",0,closeErr}
	return hex.EncodeToString(h.Sum(nil)),size,nil
}
func preservationDigest(files []worktreePreservedFile)string{
	body,_:=json.Marshal(files)
	hash:=sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

func enumerateWorktreeRegularFiles(target string,max int)([]worktreePreservedFile,error){
	files:=[]worktreePreservedFile{}
	err:=filepath.WalkDir(target,func(path string,entry fs.DirEntry,walkErr error)error{
		if walkErr!=nil{return walkErr}
		if path==target{return nil}
		rel,err:=filepath.Rel(target,path)
		if err!=nil{return err}
		if filepath.Base(path)==".git"{
			if filepath.Dir(path)==target {
				if entry.IsDir(){return filepath.SkipDir}
				return nil
			}
			return fmt.Errorf("nested repository requires manual preservation: %s",rel)
		}
		if entry.Type()&os.ModeSymlink!=0{return fmt.Errorf("symlink payload requires manual preservation: %s",rel)}
		if entry.IsDir(){return nil}
		info,err:=entry.Info()
		if err!=nil{return err}
		if !info.Mode().IsRegular(){return fmt.Errorf("non-regular payload requires manual preservation: %s",rel)}
		if len(files)>=max{return fmt.Errorf("worktree inventory exceeds %d files",max)}
		hash,n,err:=hashFileStream(path)
		if err!=nil{return err}
		if n!=info.Size(){return fmt.Errorf("source changed while inventoried: %s",rel)}
		files=append(files,worktreePreservedFile{Path:filepath.ToSlash(rel),Bytes:n,SHA256:hash})
		return nil
	})
	sort.Slice(files,func(i,j int)bool{return files[i].Path<files[j].Path})
	return files,err
}

func preserveWorktree(plan worktreePlan,e worktreeEntry,archive string)(worktreeCleanupResult,error){
	result:=worktreeCleanupResult{Status:"unknown",Target:e.Path,Archive:archive,PlanDigest:plan.PlanDigest,Reasons:[]string{}}
	if old,err:=readWorktreePreservation(archive);err==nil{
		if old.Target!=e.Path||old.HEAD!=e.HEAD||old.PlanDigest!=plan.PlanDigest{return result,errors.New("archive manifest already belongs to a different plan")}
		check:=verifyWorktreePreservation(old,true)
		return check,nil
	}else if !os.IsNotExist(err){return result,err}
	files,err:=enumerateWorktreeRegularFiles(e.Path,200000)
	if err!=nil{return result,err}
	if err:=os.MkdirAll(filepath.Join(archive,"payload"),0700);err!=nil{return result,err}
	var total int64
	for _,f:=range files{
		source:=filepath.Join(e.Path,filepath.FromSlash(f.Path))
		destination:=preservedFilePath(archive,f.Path)
		if err:=os.MkdirAll(filepath.Dir(destination),0700);err!=nil{return result,err}
		if err:=rejectPathSymlinksWithin(archive,destination);err!=nil{return result,err}
		if err:=rejectPathSymlinksWithin(e.Path,source);err!=nil{return result,err}
		if old,err:=os.Lstat(destination);err==nil{
			if !old.Mode().IsRegular()||old.Mode()&os.ModeSymlink!=0{return result,fmt.Errorf("archive payload unsafe: %s",f.Path)}
			hash,n,err:=hashFileStream(destination)
			if err!=nil{return result,err}
			if hash!=f.SHA256||n!=f.Bytes{return result,fmt.Errorf("archive payload hash conflict: %s",f.Path)}
		}else if os.IsNotExist(err){
			if err:=copyPreservedFile(source,destination,f);err!=nil{return result,err}
		}else{return result,err}
		total+=f.Bytes
	}
	// No file may change between the initial snapshot and publication.
	fresh,err:=enumerateWorktreeRegularFiles(e.Path,200000)
	if err!=nil{return result,err}
	if !samePreservedFiles(files,fresh){return result,errors.New("worktree changed during preservation; original retained")}
	current,err:=inspectWorktrees(plan.Repository,plan.IntegrationRef,200000)
	if err!=nil||current.PlanDigest!=plan.PlanDigest{return result,errors.New("worktree plan changed during preservation")}
	manifest:=worktreePreservation{SchemaVersion:1,Repository:plan.Repository,Target:e.Path,Archive:archive,HEAD:e.HEAD,PlanDigest:plan.PlanDigest,
		CreatedAt:time.Now().UTC().Format(time.RFC3339Nano),Files:files,PayloadDigest:preservationDigest(files)}
	if err:=writeWorktreeManifest(preservationManifestPath(archive),manifest);err!=nil{return result,err}
	result.Status="preserved";result.Verified=true;result.FileCount=len(files);result.Bytes=total
	return result,nil
}

func samePreservedFiles(a,b []worktreePreservedFile)bool{
	if len(a)!=len(b){return false}
	for i:=range a{if a[i]!=b[i]{return false}}
	return true
}
func copyPreservedFile(source,dest string,expected worktreePreservedFile)error{
	file,err:=os.Open(source)
	if err!=nil{return err}
	defer file.Close()
	temp,err:=os.CreateTemp(filepath.Dir(dest),".preserve-*.tmp")
	if err!=nil{return err}
	defer os.Remove(temp.Name())
	if err:=temp.Chmod(0600);err!=nil{_ = temp.Close();return err}
	h:=sha256.New()
	size,err:=io.Copy(io.MultiWriter(temp,h),file)
	if err==nil{err=temp.Sync()}
	if closeErr:=temp.Close();err==nil{err=closeErr}
	if err!=nil{return err}
	if size!=expected.Bytes||hex.EncodeToString(h.Sum(nil))!=expected.SHA256{return fmt.Errorf("source hash changed while copied: %s",expected.Path)}
	if err:=os.Link(temp.Name(),dest);err!=nil{return fmt.Errorf("safe exclusive archive publish refused: %w",err)}
	verified,n,err:=hashFileStream(dest)
	if err!=nil||verified!=expected.SHA256||n!=expected.Bytes{return fmt.Errorf("archive copy hash verification failed: %s",expected.Path)}
	return nil
}
func writeWorktreeManifest(path string,m worktreePreservation)error{
	body,err:=json.MarshalIndent(m,"","  ");if err!=nil{return err}
	temp,err:=os.CreateTemp(filepath.Dir(path),".preservation-*.tmp")
	if err!=nil{return err}
	defer os.Remove(temp.Name())
	if _,err=temp.Write(append(body,'\n'));err==nil{err=temp.Sync()}
	if closeErr:=temp.Close();err==nil{err=closeErr}
	if err!=nil{return err}
	return os.Link(temp.Name(),path)
}
func readWorktreePreservation(archive string)(worktreePreservation,error){
	var m worktreePreservation
	body,err:=os.ReadFile(preservationManifestPath(archive))
	if err!=nil{return m,err}
	dec:=json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err:=dec.Decode(&m);err!=nil{return m,err}
	var trailing any
	if err:=dec.Decode(&trailing);err!=io.EOF{return m,errors.New("trailing manifest data")}
	if m.SchemaVersion!=1||m.PayloadDigest!=preservationDigest(m.Files)||m.Archive!=archive||m.Repository==""||m.Target==""||m.PlanDigest==""{
		return m,errors.New("PRESERVATION_CONFLICT: manifest identity or integrity invalid")
	}
	for _,file:=range m.Files{
		clean:=filepath.Clean(filepath.FromSlash(file.Path))
		if file.Path==""||filepath.IsAbs(clean)||clean==".."||strings.HasPrefix(clean,".."+string(filepath.Separator))||file.SHA256==""{
			return m,errors.New("PRESERVATION_CONFLICT: invalid relative file path")
		}
	}
	return m,nil
}
func verifyWorktreePreservation(m worktreePreservation,verifySource bool)worktreeCleanupResult{
	r:=worktreeCleanupResult{Status:"verified",Target:m.Target,Archive:m.Archive,PlanDigest:m.PlanDigest,Verified:true,FileCount:len(m.Files),Reasons:[]string{}}
	seen:=map[string]bool{}
	for _,f:=range m.Files{
		if seen[f.Path]{r.Reasons=append(r.Reasons,"duplicate manifest entry: "+f.Path);continue}
		seen[f.Path]=true
		file:=preservedFilePath(m.Archive,f.Path)
		if err:=rejectPathSymlinksWithin(m.Archive,file);err!=nil{r.Reasons=append(r.Reasons,err.Error());continue}
		info,err:=os.Lstat(file)
		if err!=nil||!info.Mode().IsRegular()||info.Mode()&os.ModeSymlink!=0{
			r.Reasons=append(r.Reasons,"archive file missing or unsafe: "+f.Path);continue
		}
		hash,n,err:=hashFileStream(file)
		if err!=nil||hash!=f.SHA256||n!=f.Bytes{r.Reasons=append(r.Reasons,"archive file hash mismatch: "+f.Path)}
		if verifySource{
			source:=filepath.Join(m.Target,filepath.FromSlash(f.Path))
			stat,e:=os.Lstat(source)
			if e!=nil||!stat.Mode().IsRegular()||stat.Mode()&os.ModeSymlink!=0{
				r.Reasons=append(r.Reasons,"source file missing or unsafe: "+f.Path)
			}else if s,n,e:=hashFileStream(source);e!=nil||s!=f.SHA256||n!=f.Bytes{
				r.Reasons=append(r.Reasons,"source hash mismatch: "+f.Path)
			}
		}
		r.Bytes+=f.Bytes
	}
	if verifySource {
		fresh,err:=enumerateWorktreeRegularFiles(m.Target,200000)
		if err!=nil||!samePreservedFiles(fresh,m.Files){r.Reasons=append(r.Reasons,"source inventory differs from protected manifest")}
	}
	if len(r.Reasons)>0{r.Status="unknown";r.Verified=false;r.NextAction="Do not remove worktree; inspect protection manifest and source/backup hash differences."}
	return r
}

func writeWorktreeRemovalReceipt(archive string,m worktreePreservation)error{
	path:=filepath.Join(archive,"removal.json")
	if old,err:=os.ReadFile(path);err==nil {
		var saved struct{Target string `json:"target"`;Repository string `json:"repository"`;HEAD string `json:"head"`;PayloadDigest string `json:"payload_digest"`}
		if json.Unmarshal(old,&saved)==nil&&saved.Target==m.Target&&saved.Repository==m.Repository&&saved.HEAD==m.HEAD&&saved.PayloadDigest==m.PayloadDigest{return nil}
		return errors.New("existing removal receipt conflicts with verified archive")
	}else if !os.IsNotExist(err){return err}
	data:=struct{
		Target string `json:"target"`
		Repository string `json:"repository"`
		HEAD string `json:"head"`
		PayloadDigest string `json:"payload_digest"`
		ConfirmedAt string `json:"confirmed_at"`
		Method string `json:"method"`
	}{m.Target,m.Repository,m.HEAD,m.PayloadDigest,time.Now().UTC().Format(time.RFC3339Nano),"git worktree remove (no force)"}
	body,err:=json.MarshalIndent(data,"","  ");if err!=nil{return err}
	tmp,err:=os.CreateTemp(archive,".removal-receipt-*.tmp")
	if err!=nil{return err}
	defer os.Remove(tmp.Name())
	if _,err=tmp.Write(append(body,'\n'));err==nil{err=tmp.Sync()}
	if e:=tmp.Close();err==nil{err=e}
	if err!=nil{return err}
	return os.Link(tmp.Name(),path)
}
