package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silverkhan/TaskMecca/internal/backlog"
)

func qaContract(t *testing.T,args ...string)(int,[]byte,string){
	t.Helper()
	var stdout,stderr bytes.Buffer
	code:=runContractCLI(args,&stdout,&stderr)
	return code,stdout.Bytes(),stderr.String()
}

func TestAgentOperationalE2E_RegistrationApprovalAndProtection(t *testing.T){
	t.Setenv("TASK_MECCA_HOME",t.TempDir())
	project:=t.TempDir()
	if err:=os.MkdirAll(filepath.Join(project,"_task_mecca"),0755);err!=nil{t.Fatal(err)}
	if _,err:=backlog.TagDefine(project,"area:backend","Fixture backend scope","");err!=nil{t.Fatal(err)}
	tasks:=[]contractTask{}
	for i:=1;i<=6;i++{
		key:=fmt.Sprintf("linear:QA-%d",i)
		item:=contractTask{
			Key:key,Title:fmt.Sprintf("검증 작업 %d",i),Kind:"defined",
			Goal:fmt.Sprintf("원본 요구사항 %d를 보존한다",i),
			AcceptanceCriteria:[]string{"등록 ID·출처·선행관계를 검증한다","등록 이력을 추적한다"},
			SourceURL:fmt.Sprintf("https://linear.app/aiden7/issue/QA-%d",i),
			SourceLabel:fmt.Sprintf("QA-%d",i),
			SourceText:fmt.Sprintf("# 원본 QA-%d\n요건 전문 변경 금지",i),
			Tags:[]string{"area:backend"},
		}
		if i>1{item.DependsOn=[]string{fmt.Sprintf("linear:QA-%d",i-1)}}
		tasks=append(tasks,item)
	}
	manifest:=contractManifest{SchemaVersion:1,Tasks:tasks}
	manifestBytes,err:=json.Marshal(manifest)
	if err!=nil{t.Fatal(err)}
	input:=filepath.Join(t.TempDir(),"manifest.json")
	if err:=os.WriteFile(input,manifestBytes,0600);err!=nil{t.Fatal(err)}
	common:=[]string{"--project",project,"--input",input,"--request-id","qa-six-item-batch","--json"}
	code,_,stderr:=qaContract(t,"validate","--input",input,"--json")
	if code!=0{t.Fatalf("manifest validation failed: %s",stderr)}
	code,preview,stderr:=qaContract(t,"preview","--input",input,"--json")
	if code!=0{t.Fatalf("preview failed: %s",stderr)}
	var previewResult contractResult
	if err:=json.Unmarshal(preview,&previewResult);err!=nil{t.Fatal(err)}
	if len(previewResult.Documents)!=6||previewResult.Written{t.Fatalf("preview unexpectedly wrote or dropped tasks: %+v",previewResult)}
	code,raw,stderr:=qaContract(t,append([]string{"plan"},common...)...)
	if code!=0{t.Fatalf("plan failed: %s",stderr)}
	var plan batchPlan
	if err:=json.Unmarshal(raw,&plan);err!=nil{t.Fatal(err)}
	if len(plan.Tasks)!=6||plan.Tasks[0].ID!="B-1"||plan.Tasks[5].ID!="B-6"{t.Fatalf("ID mapping unexpected: %+v",plan)}
	if _,err:=os.Stat(batchJournalPath(project,plan.RequestID));!os.IsNotExist(err){t.Fatal("read-only plan wrote journal")}
	apply:=append(append([]string{"apply"},common...),"--expect-plan",plan.PlanDigest,"--approve")
	code,raw,stderr=qaContract(t,apply...)
	if code!=0{t.Fatalf("batch apply failed: %s %s",stderr,string(raw))}
	var batch batchResult
	if err:=json.Unmarshal(raw,&batch);err!=nil{t.Fatal(err)}
	if batch.Status!="success"||len(batch.Tasks)!=6{t.Fatalf("registration incomplete: %+v",batch)}
	code,raw,stderr=qaContract(t,apply...)
	if code!=0{t.Fatalf("same request id retry failed: %s",stderr)}
	records,err:=backlog.Catalog(project,"")
	if err!=nil||len(records)!=6{t.Fatalf("catalog failed: %v %d",err,len(records))}
	events,err:=backlog.ReadLifecycleTransitions(project)
	if err!=nil||len(events.Events)!=6{t.Fatalf("duplicate or missing registration events: %v %d",err,len(events.Events))}
	for i,record:=range records{
		for _,section:=range []string{"### 목표","### 수용 기준","## 원천 전문 (변경 금지)","area:backend"}{
			if !strings.Contains(record.RawMarkdown,section){t.Fatalf("task %d missing %s",i,section)}
		}
		if i>0 && !strings.Contains(record.RawMarkdown,fmt.Sprintf("- 선행: B-%d",i)){t.Fatalf("missing dependency on task %d",i)}
	}
	// Registration and runtime authorization are separate; no grant is inferred.
	status,err:=projectContractDecision(project,"B-1")
	if err!=nil||status.ExecutionAuthorized{t.Fatalf("registration must not approve execution: %+v %v",status,err)}
	code,_,_ = decisionInvoke(t,"record","--project",project,"--task","B-1","--event-id","qa-user-approval","--kind","execution_approved","--actor","/root","--evidence","test-confirmation","--json")
	if code==0{t.Fatal("decision recorded without explicit user-confirmed attestation")}
	code,approved,stderr:=decisionInvoke(t,"record","--project",project,"--task","B-1","--event-id","qa-user-approval","--kind","execution_approved","--actor","/root","--evidence","test-confirmation","--user-confirmed","--json")
	if code!=0||!approved.ExecutionAuthorized{t.Fatalf("explicit approval not projected: %d %+v %s",code,approved,stderr)}
	// Protected baseline after registration is an explicit checked snapshot,
	// not a claim that Runtime/Web or any external files were verified.
	protectedBefore,err:=captureProjectProtection(project)
	if err!=nil{t.Fatal(err)}
	if len(protectedBefore.Items)<7{t.Fatalf("expected tasks, lifecycle/tags in protected scope, got %d",len(protectedBefore.Items))}
	protectedAfter,err:=captureProjectProtection(project)
	if err!=nil{t.Fatal(err)}
	proof:=verifyProtection(protectedBefore,protectedAfter)
	if !proof.Checked||proof.Status!="verified"||len(proof.Changed)>0{t.Fatalf("protected files not verified: %+v",proof)}
}

func TestAgentOperationalE2E_ReadOnlyRuntimeCommands(t *testing.T){
	project:=t.TempDir()
	if err:=os.MkdirAll(filepath.Join(project,"_task_mecca"),0755);err!=nil{t.Fatal(err)}
	var queryOut,queryErr bytes.Buffer
	code:=runRuntimeQueryCLI([]string{"--project",project,"--agent-id","missing-id","--limit","2","--json"},&queryOut,&queryErr)
	if code!=0{t.Fatalf("empty runtime query must remain a valid no-observation: %s",queryErr.String())}
	var reply runtimeQueryResponse
	if err:=json.Unmarshal(queryOut.Bytes(),&reply);err!=nil||reply.Status!="not_found"||reply.Count!=0{t.Fatalf("empty runtime interpreted as current run: %+v %v",reply,err)}
	var diagOut,diagErr bytes.Buffer
	code=runRuntimeDiagnoseCLI([]string{"--project",project,"--provider","codex","--agent-id","missing-id","--json"},&diagOut,&diagErr)
	if code!=0{t.Fatalf("read-only diagnosis failed: %s",diagErr.String())}
	var report runtimeDiagnostic
	if err:=json.Unmarshal(diagOut.Bytes(),&report);err!=nil{t.Fatal(err)}
	if report.NativeOperationAttempted||report.ReconciliationPerformed||len(report.Attempts)!=0{t.Fatalf("diagnostic fabricated runtime execution: %+v",report)}
}
