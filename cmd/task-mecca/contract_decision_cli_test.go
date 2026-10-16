package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeDecisionFixture(t *testing.T)string{
	t.Helper()
	project:=t.TempDir()
	t.Setenv("TASK_MECCA_HOME",t.TempDir())
	folder:=filepath.Join(project,"_task_mecca","data","backlog")
	if err:=os.MkdirAll(folder,0755);err!=nil{t.Fatal(err)}
	task:=contractTask{Key:"local:fixture",Title:"검증 작업",Kind:"simple",Goal:"원본 계약 유지",AcceptanceCriteria:[]string{"기존 데이터 보존"}}
	file:=filepath.Join(folder,"000001.B-1.test.todo.md")
	if err:=os.WriteFile(file,[]byte(renderRegisteredContract(task,"B-1",nil)),0644);err!=nil{t.Fatal(err)}
	return project
}

func decisionInvoke(t *testing.T,args ...string)(int,contractDecisionReport,string){
	t.Helper()
	var out,errs bytes.Buffer
	code:=runContractDecisionCLI(args,&out,&errs)
	var report contractDecisionReport
	if out.Len()!=0 {
		if err:=json.Unmarshal(out.Bytes(),&report);err!=nil{t.Fatalf("bad decision JSON: %v: %s",err,out.String())}
	}
	return code,report,errs.String()
}
func decisionRecordArgs(project,id,kind string)[]string{
	return []string{"record","--project",project,"--task","B-1","--event-id",id,"--kind",kind,"--actor","/root","--evidence","user-chat-reference:"+id,"--json"}
}

func TestDecisionApprovalRequiresEvidenceAndCurrentScope(t *testing.T){
	project:=makeDecisionFixture(t)
	base:=decisionRecordArgs(project,"approval-one","execution_approved")
	code,_,_:=decisionInvoke(t,base...)
	if code!=3{t.Fatalf("expected explicit user-approval block, got %d",code)}
	if _,err:=os.Stat(decisionPath(project,"approval-one"));!os.IsNotExist(err){t.Fatal("unapproved event persisted")}
	// An unresolved policy must block execution approval, even with user confirmation.
	code,report,msg:=decisionInvoke(t,append(decisionRecordArgs(project,"policy-one","policy_pending"),"--policy-key","region")...)
	if code!=0||report.Status!="policy_pending"{t.Fatalf("policy pending %d %+v %s",code,report,msg)}
	code,_,_ = decisionInvoke(t,append(base,"--user-confirmed")...)
	if code!=3{t.Fatalf("execution must be blocked while policy pending: %d",code)}
	code,report,msg=decisionInvoke(t,append(decisionRecordArgs(project,"policy-two","policy_confirmed"),"--policy-key","region","--user-confirmed")...)
	if code!=0||len(report.PendingPolicies)!=0{t.Fatalf("policy confirmation %d %+v %s",code,report,msg)}
	code,report,msg=decisionInvoke(t,append(base,"--user-confirmed")...)
	if code!=0||!report.ExecutionAuthorized||report.Status!="execution_authorized"{t.Fatalf("approval %d %+v %s",code,report,msg)}
	// Retry must not create another event or require a freshly derived digest.
	code,retry,msg:=decisionInvoke(t,append(base,"--user-confirmed")...)
	if code!=0||len(retry.Events)!=len(report.Events){t.Fatalf("retry %d %+v %s",code,retry,msg)}
	// Operational note changes must not invalidate a contract-specific grant.
	path:=filepath.Join(project,"_task_mecca","data","backlog","000001.B-1.test.todo.md")
	original,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
	changed:=strings.Replace(string(original),"- 메모: -","- 메모: 관측 기록 추가",1)
	if err:=os.WriteFile(path,[]byte(changed),0644);err!=nil{t.Fatal(err)}
	code,report,msg=decisionInvoke(t,"show","--project",project,"--task","B-1","--json")
	if code!=0||!report.ExecutionAuthorized{t.Fatalf("operational note shouldn't invalidate approval %d %s",code,msg)}
	// User-confirmed supplement invalidates the earlier approval until renewed.
	payload:=filepath.Join(t.TempDir(),"policy.txt")
	if err:=os.WriteFile(payload,[]byte("추가 계약 조건: 교차검증 필수"),0600);err!=nil{t.Fatal(err)}
	code,report,msg=decisionInvoke(t,append(decisionRecordArgs(project,"supplement-one","supplement"),"--body-file",payload,"--user-confirmed")...)
	if code!=0||report.ExecutionAuthorized||len(report.Supplements)!=1{t.Fatalf("supplement %d %+v %s",code,report,msg)}
	code,report,msg=decisionInvoke(t,append(decisionRecordArgs(project,"approval-two","execution_approved"),"--user-confirmed")...)
	if code!=0||!report.ExecutionAuthorized{t.Fatalf("reauthorization %d %+v %s",code,report,msg)}
	code,report,msg=decisionInvoke(t,append(decisionRecordArgs(project,"revoke-one","execution_revoked"),"--user-confirmed")...)
	if code!=0||report.ExecutionAuthorized{t.Fatalf("revocation %d %+v %s",code,report,msg)}
	latest,err:=os.ReadFile(path)
	if err!=nil||string(latest)!=changed{t.Fatal("decision history must not edit source contract")}
}

func TestDecisionRefusesDuplicateEventDifferentContents(t *testing.T){
	project:=makeDecisionFixture(t)
	args:=append(decisionRecordArgs(project,"decision-fixed","execution_approved"),"--user-confirmed")
	code,_,msg:=decisionInvoke(t,args...)
	if code!=0{t.Fatalf("first grant %d %s",code,msg)}
	another:=append(decisionRecordArgs(project,"decision-fixed","execution_revoked"),"--user-confirmed")
	code,_,_=decisionInvoke(t,another...)
	if code!=3{t.Fatalf("conflicting stable event must fail, code=%d",code)}
}
