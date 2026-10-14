package maintenance

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PR installation reuses the same executable installer and managed Web restart
// as a normal channel switch. This lock serializes mutations within the Web
// process; the durable transition checkpoint rejects a second operation after
// a crash or when another request races with the first.
var executionSwitchMu sync.Mutex

const maxPRBinaryBytes = 110 << 20

func executableDigest(path string) (string, error) {
	info,err:=os.Lstat(path)
	if err!=nil{return "",err}
	if !info.Mode().IsRegular() || info.Size()>maxPRBinaryBytes || info.Size()<1 {
		return "",errors.New("invalid executable: expected bounded regular file")
	}
	file,err:=os.Open(path)
	if err!=nil{return "",err}
	defer file.Close()
	h:=sha256.New()
	n,err:=io.Copy(h,io.LimitReader(file,maxPRBinaryBytes+1))
	if err!=nil{return "",err}
	if n>maxPRBinaryBytes{return "",errors.New("executable exceeds size limit")}
	return hex.EncodeToString(h.Sum(nil)),nil
}

func readVerifiedBinary(path string)([]byte,string,error){
	info,err:=os.Lstat(path)
	if err!=nil{return nil,"",err}
	if !info.Mode().IsRegular() || info.Size()<1 || info.Size()>maxPRBinaryBytes {
		return nil,"",errors.New("PR executable must be a bounded regular file")
	}
	data,err:=os.ReadFile(path)
	if err!=nil{return nil,"",err}
	sum:=sha256.Sum256(data)
	return data,hex.EncodeToString(sum[:]),nil
}

func exactDevBaseline(executable string)(string,string,error){
	sha,err:=executableDigest(executable)
	if err!=nil{return "","",err}
	directory:=filepath.Join(homeDir(),"environment-switch")
	if err=os.MkdirAll(directory,0700);err!=nil{return "","",err}
	path:=filepath.Join(directory,"dev-"+sha+".bin")
	if existing,err:=executableDigest(path);err==nil {
		if existing!=sha{return "","",errors.New("existing Dev baseline has mismatched checksum")}
		return path,sha,nil
	}else if !errors.Is(err,os.ErrNotExist){
		return "","",err
	}
	source,err:=os.Open(executable)
	if err!=nil{return "","",err}
	defer source.Close()
	temp,err:=os.CreateTemp(directory,".dev-baseline-")
	if err!=nil{return "","",err}
	defer os.Remove(temp.Name())
	if err=temp.Chmod(0700);err!=nil{temp.Close();return "","",err}
	if _,err=io.Copy(temp,io.LimitReader(source,maxPRBinaryBytes+1));err!=nil{temp.Close();return "","",err}
	if err=temp.Sync();err!=nil{temp.Close();return "","",err}
	if err=temp.Close();err!=nil{return "","",err}
	if digest,err:=executableDigest(temp.Name());err!=nil||digest!=sha {
		return "","",errors.New("Dev baseline snapshot verification failed")
	}
	if err=os.Rename(temp.Name(),path);err!=nil{return "","",err}
	return path,sha,nil
}

func writeTransition(phase string,from,to ExecutionTarget,sourceHash,hash string)error{
	if !validSHA256(sourceHash)||!validSHA256(hash){return errors.New("invalid source/target executable digest")}
	return writeExecutionJSON(executionTransitionPath(),ExecutionTransition{
		Schema:1,Phase:phase,From:from,To:to,SourceSHA256:sourceHash,TargetSHA256:hash,
		UpdatedAt:time.Now().UTC().Format(time.RFC3339),
	})
}

func abortTransitionIfUnmodified(expected string) {
	exe,err:=os.Executable()
	if err!=nil{return}
	exe,err=filepath.EvalSymlinks(exe)
	if err!=nil{return}
	digest,err:=executableDigest(exe)
	if err==nil&&digest==expected {
		_ = os.Remove(executionTransitionPath())
	}
}

// EnterPR installs only a separately downloaded and cryptographically verified
// same-repository artifact. The caller must prove the PR/current HEAD and CI
// provenance before calling, and the caller must request a Web restart.
func EnterPR(currentVersion string,number int,headSHA,binaryPath string)(UpgradeResult,error){
	executionSwitchMu.Lock()
	defer executionSwitchMu.Unlock()
	result:=UpgradeResult{From:currentVersion,Channel:devChannel}
	// The existing Windows upgrade helper stages an asynchronous .new move.
	// Until that helper can prove ordering before Web health, do not risk an
	// ambiguous PR/dev executable during a channel transition.
	if runtime.GOOS=="windows" {
		return result,errors.New("Windows PR switching awaits a verified staged-install/restart handoff; Dev/Stable switching remains available")
	}
	if CurrentChannel()!=devChannel || strings.TrimSpace(os.Getenv("TASK_MECCA_CHANNEL"))!=""{
		return result,errors.New("PR execution requires a locally managed Dev release channel")
	}
	if err:=ensureReleaseTransitionAllowed();err!=nil{return result,err}
	if number<1||!validCommitSHA(headSHA){return result,errors.New("invalid PR identity")}
	exe,err:=os.Executable()
	if err!=nil{return result,err}
	exe,err=filepath.EvalSymlinks(exe)
	if err!=nil{return result,err}
	data,prHash,err:=readVerifiedBinary(binaryPath)
	if err!=nil{return result,err}
	if err=validateUpgradeBinary(binaryPath);err!=nil{return result,err}
	baseline,baselineHash,err:=exactDevBaseline(exe)
	if err!=nil{return result,err}
	if baselineHash==prHash{return result,errors.New("PR executable is identical to Dev; refusing ambiguous transition")}
	previous:=ExecutionTarget{Kind:"dev",Channel:devChannel}
	target:=ExecutionTarget{
		Kind:"pr",Channel:devChannel,PRNumber:number,HeadSHA:headSHA,
		OriginalDevVersion:currentVersion,OriginalDevSHA256:baselineHash,
		OriginalDevBackup:baseline,InstalledAt:time.Now().UTC().Format(time.RFC3339),
	}
	if err=target.validate();err!=nil{return result,err}
	if err=writeTransition("installing",previous,target,baselineHash,prHash);err!=nil{return result,err}
	// Record intent before mutating the executable, so a restart or crash never
	// accidentally treats a PR executable as a release-channel update.
	if err=writeExecutionJSON(executionTargetPath(),target);err!=nil{
		abortTransitionIfUnmodified(baselineHash)
		return result,err
	}
	installed,installErr:=installBinary(currentVersion,"PR #"+strconv.Itoa(number)+" "+headSHA[:10],data)
	if installErr!=nil {
		// Leave the recovery record if binary state cannot be proved unchanged.
		if digest,e:=executableDigest(exe);e==nil&&digest==baselineHash{
			_ = os.Remove(executionTargetPath())
			abortTransitionIfUnmodified(baselineHash)
		}else{
			_ = writeTransition("rollback_required",previous,target,baselineHash,prHash)
		}
		return installed,installErr
	}
	if err=writeTransition("restarting",previous,target,baselineHash,prHash);err!=nil{
		return installed,fmt.Errorf("PR installed but restart checkpoint failed: %w",err)
	}
	installed.Channel=devChannel
	return installed,nil
}

// ReturnFromPR restores exactly the Dev binary retained at PR entry, never an
// arbitrary latest Dev release. Project files are intentionally not modified.
func ReturnFromPR(currentVersion string)(UpgradeResult,error){
	executionSwitchMu.Lock()
	defer executionSwitchMu.Unlock()
	result:=UpgradeResult{From:currentVersion,Channel:devChannel}
	if runtime.GOOS=="windows" {
		return result,errors.New("Windows PR restore requires the staged-install/restart contract; no mutation performed")
	}
	target,err:=InstalledExecutionTarget()
	if err!=nil{return result,err}
	if target.Kind!="pr"{return result,errors.New("current execution target is not a PR")}
	pending,err:=PendingExecutionTransition()
	if err!=nil{return result,err}
	if pending!=nil{return result,fmt.Errorf("transition %s requires recovery first",pending.Phase)}
	data,sum,err:=readVerifiedBinary(target.OriginalDevBackup)
	if err!=nil{return result,fmt.Errorf("retained Dev baseline missing: %w",err)}
	if sum!=target.OriginalDevSHA256{
		return result,errors.New("retained Dev baseline checksum mismatch")
	}
	to:=ExecutionTarget{Kind:devChannel,Channel:devChannel}
	exe,exeErr:=os.Executable()
	if exeErr!=nil{return result,exeErr}
	exe,exeErr=filepath.EvalSymlinks(exe)
	if exeErr!=nil{return result,exeErr}
	prDigest,digestErr:=executableDigest(exe)
	if digestErr!=nil{return result,digestErr}
	if err=writeTransition("installing",target,to,prDigest,sum);err!=nil{return result,err}
	installed,installErr:=installBinary(currentVersion,target.OriginalDevVersion,data)
	if installErr!=nil {
		_ = writeTransition("rollback_required",target,to,prDigest,sum)
		return installed,installErr
	}
	if err=writeTransition("restarting",target,to,prDigest,sum);err!=nil{
		return installed,fmt.Errorf("Dev restored but restart checkpoint failed: %w",err)
	}
	installed.Channel=devChannel
	return installed,nil
}

// ReconcileExecutionOnBoot checks the REAL running binary before acknowledging
// a PR switch or return. Reconciliation never performs an implicit migration.
func ReconcileExecutionOnBoot()error{
	executionSwitchMu.Lock()
	defer executionSwitchMu.Unlock()
	pending,err:=PendingExecutionTransition()
	if err!=nil{return err}
	if pending==nil{return nil}
	if !validSHA256(pending.TargetSHA256){return errors.New("missing transition target binary checksum")}
	exe,err:=os.Executable()
	if err!=nil{return err}
	exe,err=filepath.EvalSymlinks(exe)
	if err!=nil{return err}
	current,err:=executableDigest(exe)
	if err!=nil{return err}
	if current==pending.TargetSHA256 {
		if pending.To.Kind=="pr"{
			if err=writeExecutionJSON(executionTargetPath(),pending.To);err!=nil{return err}
		}else if pending.To.Kind=="dev" {
			if err=os.Remove(executionTargetPath());err!=nil&&!errors.Is(err,os.ErrNotExist){return err}
		}else{
			return errors.New("unexpected target in PR recovery")
		}
		return os.Remove(executionTransitionPath())
	}
	// The existing Web restart watchdog may have restored the original Dev
	// after the PR failed to become healthy. Only clear an entry checkpoint
	// if the running executable exactly matches the retained Dev baseline.
	if validSHA256(pending.SourceSHA256)&&current==pending.SourceSHA256 {
		// A failed restart watchdog has restored the exact preceding executable.
		// For a failed PR→Dev return, keep the active PR marker and allow retry.
		if pending.From.Kind=="pr" {
			if err=writeExecutionJSON(executionTargetPath(),pending.From);err!=nil{return err}
		} else if pending.From.Kind=="dev" {
			if err=os.Remove(executionTargetPath());err!=nil&&!errors.Is(err,os.ErrNotExist){return err}
		}
		return os.Remove(executionTransitionPath())
	}
	return fmt.Errorf("PR transition %s could not verify running executable; manual recovery required",pending.Phase)
}
