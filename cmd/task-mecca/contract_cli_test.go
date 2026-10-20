package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeContractFixture(t *testing.T, value string)string{
	t.Helper()
	path:=filepath.Join(t.TempDir(),"manifest.json")
	if err:=os.WriteFile(path,[]byte(value),0600);err!=nil{t.Fatal(err)}
	return path
}

func runContractTest(t *testing.T,action,body string)(contractResult,int,string){
	t.Helper()
	path:=writeContractFixture(t,body)
	var out,errs bytes.Buffer
	code:=runContractCLI([]string{action,"--input",path,"--json"},&out,&errs)
	var result contractResult
	if err:=json.Unmarshal(out.Bytes(),&result);err!=nil{t.Fatalf("parse output: %v; stderr=%s; stdout=%s",err,errs.String(),out.String())}
	return result,code,errs.String()
}

func TestContractPreviewPreservesSourceAndCanonicalHeadings(t *testing.T){
	manifest:=`{"schema_version":1,"tasks":[{"key":"Linear:AID-1","title":"원천 보존 검증","kind":"defined","goal":"대상 파일을 검사한다","acceptance_criteria":["파일을 유지한다","변경 이력을 기록한다"],"requirements":"원문을 보존한다","source_label":"AID-1","source_url":"https://linear.app/aiden7/issue/AID-1","source_text":"원문 첫줄\n### 목표\n## 작업 정의\n- [ ] 검토\n\n마지막 줄","tags":["area:backend"]}]}`
	result,code,stderr:=runContractTest(t,"preview",manifest)
	if code!=0||!result.OK||result.Written||len(result.Documents)!=1{t.Fatalf("code=%d result=%+v stderr=%s",code,result,stderr)}
	doc:=result.Documents[0]
	for _,piece:=range []string{"## 요건 정의서","### 목표","### 수용 기준","- [ ] 파일을 유지한다","- [ ] 변경 이력을 기록한다","[AID-1](<https://linear.app/aiden7/issue/AID-1>)","## 원천 전문 (변경 금지)","> ### 목표","> ## 작업 정의","> - [ ] 검토","> ","> 마지막 줄"}{
		if !strings.Contains(doc.Markdown,piece){t.Errorf("missing %q in %s",piece,doc.Markdown)}
	}
	if len(doc.SourceSHA256)!=64{t.Errorf("invalid source hash %q",doc.SourceSHA256)}
	// Reconstruct source byte-for-byte by dropping the block-quote prefix.
	source:=strings.Split(strings.TrimSpace(strings.Split(doc.Markdown,"## 원천 전문 (변경 금지)\n\n")[1]),"\n\n")[0]
	_ = source // full source correctness is checked below without Markdown re-parsing.
	if !strings.Contains(doc.Markdown,"> 원문 첫줄\n> ### 목표\n> ## 작업 정의\n> - [ ] 검토\n> \n> 마지막 줄"){t.Fatal("source was changed")}
}

func TestContractValidationRejectsBadInputBeforeWrites(t *testing.T){
	manifest:=`{"schema_version":1,"tasks":[{"key":"A","title":"One","kind":"simple","goal":"### 수용 기준\n재정의","acceptance_criteria":["test"],"depends_on":["B"],"source_url":"file:///etc/passwd"},{"key":"B","title":"Two","kind":"simple","goal":"test","acceptance_criteria":[],"depends_on":["A"]},{"key":"A","title":"Third","kind":"simple","goal":"test","acceptance_criteria":["pass"]}]}`
	result,code,_:=runContractTest(t,"validate",manifest)
	if code!=2||result.OK||result.Written||len(result.Findings)<4{t.Fatalf("unexpected result: %+v exit=%d",result,code)}
	seen:=map[string]bool{}
	for _,finding:=range result.Findings{seen[finding.Code]=true}
	for _,code:=range []string{"REQUIRED_FIELD","HEADING_COLLISION","INVALID_SOURCE_URL","DUPLICATE_KEY","DEPENDENCY_CYCLE"}{
		if !seen[code]{t.Errorf("missing expected finding %s",code)}
	}
}

func TestContractValidationSimpleAndNoMutation(t *testing.T){
	dir:=t.TempDir()
	path:=filepath.Join(dir,"input.json")
	raw:=`{"schema_version":1,"tasks":[{"key":"user:42","title":"단순 작업","kind":"simple","goal":"텍스트 확인","acceptance_criteria":["확인"]}]}`
	if err:=os.WriteFile(path,[]byte(raw),0600);err!=nil{t.Fatal(err)}
	var out,errs bytes.Buffer
	exit:=runContractCLI([]string{"validate","--input",path,"--json"},&out,&errs)
	var result contractResult
	if err:=json.Unmarshal(out.Bytes(),&result);err!=nil{t.Fatal(err)}
	if exit!=0||!result.OK||result.Written||len(result.Documents)!=0{t.Fatalf("unexpected result %+v exit=%d stderr=%s",result,exit,errs.String())}
	entries,err:=os.ReadDir(dir)
	if err!=nil{t.Fatal(err)}
	if len(entries)!=1||entries[0].Name()!="input.json"{t.Fatalf("read-only command created files: %+v",entries)}
	got,err:=os.ReadFile(path)
	if err!=nil||string(got)!=raw{t.Fatal("input was changed")}
}

func TestContractUnknownFieldsAndTrailingJSONRejected(t *testing.T){
	for _,manifest:=range []string{
		`{"schema_version":1,"tasks":[],"surprise":true}`,
		`{"schema_version":1,"tasks":[]} {"schema_version":1}`,
	}{
		result,code,_:=runContractTest(t,"validate",manifest)
		if code==0||result.OK||result.Written||len(result.Findings)==0{t.Fatalf("expected failure %+v",result)}
	}
}

func TestContractHelp(t *testing.T){
	var out,stderr bytes.Buffer
	if exit:=runContractCLI([]string{"help"},&out,&stderr);exit!=0{t.Fatalf("exit=%d",exit)}
	if !strings.Contains(out.String(),"validate")||!strings.Contains(out.String(),"preview"){t.Fatal("missing discoverable usage")}
}
