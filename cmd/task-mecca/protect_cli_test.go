package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func createProtectionFixture(t *testing.T)string{
	t.Helper()
	project:=t.TempDir()
	for i:=0;i<801;i++{
		folder:=[]string{"backlog","requirements","lifecycle","tags"}[i%4]
		path:=filepath.Join(project,"_task_mecca","data",folder,fmt.Sprintf("%04d.txt",i))
		if err:=os.MkdirAll(filepath.Dir(path),0755);err!=nil{t.Fatal(err)}
		if err:=os.WriteFile(path,[]byte(fmt.Sprintf("evidence-%d",i)),0600);err!=nil{t.Fatal(err)}
	}
	return project
}

func TestProtectedSnapshot801FilesAndChanges(t *testing.T){
	project:=createProtectionFixture(t)
	before,err:=captureProjectProtection(project)
	if err!=nil{t.Fatal(err)}
	if len(before.Items)!=801||len(before.Digest)!=64{t.Fatalf("unexpected protected snapshot %+v",before)}
	after,err:=captureProjectProtection(project)
	if err!=nil{t.Fatal(err)}
	if result:=verifyProtection(before,after);result.Status!="verified"||!result.Checked{t.Fatalf("unchanged data %+v",result)}
	base:=filepath.Join(project,"_task_mecca","data")
	if err:=os.WriteFile(filepath.Join(base,"backlog","0000.txt"),[]byte("tampered"),0600);err!=nil{t.Fatal(err)}
	if err:=os.Remove(filepath.Join(base,"tags","0003.txt"));err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(filepath.Join(base,"lifecycle","new.txt"),[]byte("new"),0600);err!=nil{t.Fatal(err)}
	after,err=captureProjectProtection(project)
	if err!=nil{t.Fatal(err)}
	result:=verifyProtection(before,after)
	if result.Status!="changed"||len(result.Changed)!=1||len(result.Missing)!=1||len(result.Added)!=1{t.Fatalf("modified/missing/added detection %+v",result)}
}

func TestProtectCLIManifestDoesNotRewriteExisting(t *testing.T){
	project:=createProtectionFixture(t)
	outfile:=filepath.Join(t.TempDir(),"protected.json")
	var out,errs bytes.Buffer
	code:=runProtectCLI([]string{"snapshot","--project",project,"--output",outfile,"--json"},&out,&errs)
	if code!=0{t.Fatalf("snapshot exit=%d err=%s",code,errs.String())}
	var response map[string]any
	if err:=json.Unmarshal(out.Bytes(),&response);err!=nil{t.Fatal(err)}
	if response["count"].(float64)!=801{t.Fatalf("count %+v",response)}
	out.Reset();errs.Reset()
	code=runProtectCLI([]string{"verify","--project",project,"--snapshot",outfile,"--json"},&out,&errs)
	if code!=0{t.Fatalf("verify exit=%d err=%s body=%s",code,errs.String(),out.String())}
	original,err:=loadProtectionSnapshot(outfile)
	if err!=nil||len(original.Items)!=801{t.Fatalf("manifest load %+v %v",original,err)}
	code=runProtectCLI([]string{"snapshot","--project",project,"--output",outfile},&out,&errs)
	if code==0{t.Fatal("existing snapshot silently overwritten")}
	owned:=filepath.Join(project,"_task_mecca","data","bad.json")
	code=runProtectCLI([]string{"snapshot","--project",project,"--output",owned},&out,&errs)
	if code==0{t.Fatal("protected data overwritten with manifest")}
}

func TestProtectedSnapshotRefusesSymlinkEscape(t *testing.T){
	if runtime.GOOS=="windows"{t.Skip("symlink creation may require developer permissions")}
	project:=t.TempDir()
	root:=filepath.Join(project,"_task_mecca","data")
	if err:=os.MkdirAll(root,0755);err!=nil{t.Fatal(err)}
	secret:=filepath.Join(t.TempDir(),"private")
	if err:=os.WriteFile(secret,[]byte("don't follow"),0600);err!=nil{t.Fatal(err)}
	if err:=os.Symlink(secret,filepath.Join(root,"external"));err!=nil{t.Fatal(err)}
	_,err:=captureProjectProtection(project)
	if err==nil||!strings.Contains(err.Error(),"symlink"){t.Fatalf("unexpected symlink traversal result: %v",err)}
}
