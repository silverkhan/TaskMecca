package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/silverkhan/TaskMecca/internal/backlog"
)

func batchFixture(t *testing.T)(string,string){
	t.Helper()
	project:=t.TempDir()
	t.Setenv("TASK_MECCA_HOME",t.TempDir())
	if err:=os.MkdirAll(filepath.Join(project,"_task_mecca"),0755);err!=nil{t.Fatal(err)}
	manifest:=`{"schema_version":1,"tasks":[{"key":"linear:A-1","title":"첫 작업","kind":"defined","goal":"값을 검증한다","acceptance_criteria":["입력은 유지한다"],"source_url":"https://linear.app/aiden7/issue/A-1","source_label":"A-1","source_text":"## 원천\n첫 줄"},{"key":"linear:A-2","title":"둘째 작업","kind":"simple","goal":"결과를 출력한다","acceptance_criteria":["선행 조건을 만족한다"],"depends_on":["linear:A-1"]}]}`
	input:=filepath.Join(t.TempDir(),"tasks.json")
	if err:=os.WriteFile(input,[]byte(manifest),0600);err!=nil{t.Fatal(err)}
	return project,input
}

func batchInvoke(t *testing.T,args ...string)(int,[]byte,string){
	t.Helper()
	var out,errs bytes.Buffer
	code:=runContractCLI(args,&out,&errs)
	return code,out.Bytes(),errs.String()
}

func TestBatchPlanApplyRetryAndCanonicalEvidence(t *testing.T){
	project,input:=batchFixture(t)
	args:=[]string{"--project",project,"--input",input,"--request-id","stable-123","--json"}
	code,raw,msg:=batchInvoke(t,append([]string{"plan"},args...)...)
	if code!=0{t.Fatalf("plan=%d: %s",code,msg)}
	var plan batchPlan
	if err:=json.Unmarshal(raw,&plan);err!=nil{t.Fatal(err)}
	if plan.Written||len(plan.Tasks)!=2||plan.Tasks[0].ID!="B-1"||!reflect.DeepEqual(plan.Tasks[1].DependsOn,[]string{"B-1"}){t.Fatalf("invalid plan: %+v",plan)}
	if _,err:=os.Stat(batchJournalPath(project,"stable-123"));!os.IsNotExist(err){t.Fatalf("plan wrote journal: %v",err)}
	apply:=append(append([]string{"apply"},args...),"--expect-plan",plan.PlanDigest,"--approve")
	code,raw,msg=batchInvoke(t,apply...)
	if code!=0{t.Fatalf("apply=%d: %s %s",code,msg,string(raw))}
	var result batchResult
	if err:=json.Unmarshal(raw,&result);err!=nil{t.Fatal(err)}
	if result.Status!="success"||len(result.Tasks)!=2{t.Fatalf("result: %+v",result)}
	one,err:=os.ReadFile(plan.Tasks[0].Path)
	if err!=nil{t.Fatal(err)}
	two,err:=os.ReadFile(plan.Tasks[1].Path)
	if err!=nil{t.Fatal(err)}
	for _,s:=range []string{"### 목표","### 수용 기준","## 원천 전문 (변경 금지)","- 외부키: linear:A-1","> ## 원천"}{
		if !strings.Contains(string(one),s){t.Errorf("missing %q",s)}
	}
	if !strings.Contains(string(two),"- 선행: B-1"){t.Fatal("external source key was not resolved")}
	catalog,err:=backlog.Catalog(project,"")
	if err!=nil||len(catalog)!=2{t.Fatalf("catalog len=%d err=%v",len(catalog),err)}
	if catalog[0].Fields["외부키"]!="linear:A-1"{t.Fatalf("source key missing from parser: %+v",catalog[0].Fields)}
	events,err:=backlog.ReadLifecycleTransitions(project)
	if err!=nil||len(events.Events)!=2{t.Fatalf("events len=%d err=%v findings=%v",len(events.Events),err,events.Findings)}
	code,raw,msg=batchInvoke(t,apply...)
	if code!=0{t.Fatalf("retry failed code=%d %s %s",code,msg,raw)}
	eventsAgain,err:=backlog.ReadLifecycleTransitions(project)
	if err!=nil||len(eventsAgain.Events)!=2{t.Fatal("duplicate registration event created")}
	code,raw,msg=batchInvoke(t,"status","--project",project,"--request-id","stable-123","--json")
	if code!=0{t.Fatalf("status %d %s",code,msg)}
	var status batchResult
	if err:=json.Unmarshal(raw,&status);err!=nil{t.Fatal(err)}
	if status.Status!="success"{t.Fatalf("status=%+v",status)}
}

func TestBatchPlanRefusesStaleAndConflictingReapply(t *testing.T){
	project,input:=batchFixture(t)
	code,raw,msg:=batchInvoke(t,"plan","--project",project,"--input",input,"--request-id","one","--json")
	if code!=0{t.Fatal(msg)}
	var plan batchPlan
	if err:=json.Unmarshal(raw,&plan);err!=nil{t.Fatal(err)}
	extra:=filepath.Join(project,"_task_mecca","data","backlog","000101.B-101.extra.todo.md")
	if err:=os.MkdirAll(filepath.Dir(extra),0755);err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(extra,[]byte("# B-101 preexisting\n"),0644);err!=nil{t.Fatal(err)}
	code,_,_=batchInvoke(t,"apply","--project",project,"--input",input,"--request-id","one","--expect-plan",plan.PlanDigest,"--approve","--json")
	if code!=3{t.Fatalf("expected stale plan, got %d",code)}
	if _,err:=os.Stat(batchJournalPath(project,"one"));!os.IsNotExist(err){t.Fatal("stale apply created journal")}
	if err:=os.Remove(extra);err!=nil{t.Fatal(err)}
	code,raw,_=batchInvoke(t,"plan","--project",project,"--input",input,"--request-id","one","--json")
	if code!=0{t.Fatal(string(raw))}
	if err:=json.Unmarshal(raw,&plan);err!=nil{t.Fatal(err)}
	code,raw,msg=batchInvoke(t,"apply","--project",project,"--input",input,"--request-id","one","--expect-plan",plan.PlanDigest,"--approve","--json")
	if code!=0{t.Fatalf("apply failed: %s %s",msg,raw)}
	changed:=`{"schema_version":1,"tasks":[{"key":"linear:A-3","title":"별도 작업","kind":"simple","goal":"수정","acceptance_criteria":["검증"]}]}`
	if err:=os.WriteFile(input,[]byte(changed),0600);err!=nil{t.Fatal(err)}
	code,_,_=batchInvoke(t,"apply","--project",project,"--input",input,"--request-id","one","--expect-plan",plan.PlanDigest,"--approve","--json")
	if code!=3{t.Fatalf("idempotency conflict expected exit=3, got %d",code)}
}

func TestBatchInvalidInputDoesNotCreateFiles(t *testing.T){
	project,input:=batchFixture(t)
	if err:=os.WriteFile(input,[]byte(`{"schema_version":1,"tasks":[{"key":"bad\nkey","kind":"simple","title":"x","goal":"a","acceptance_criteria":["ok"]}]}`),0600);err!=nil{t.Fatal(err)}
	code,_,_=batchInvoke(t,"apply","--project",project,"--input",input,"--request-id","bad","--expect-plan","abc","--approve","--json")
	if code==0{t.Fatal("bad input accepted")}
	if _,err:=os.Stat(batchJournalPath(project,"bad"));!os.IsNotExist(err){t.Fatal("invalid input modified project")}
}
