package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentJSONFailureHasActionableCodes(t *testing.T){
	for _,tc:=range []struct{message,code string;retry bool}{
		{"STALE_PLAN: backlog updated","STALE_PLAN",true},
		{"IDEMPOTENCY_CONFLICT: another payload","IDEMPOTENCY_CONFLICT",false},
		{"USER_APPROVAL_REQUIRED","APPROVAL_REQUIRED",false},
		{"FILE_CONFLICT: canonical document modified","FILE_CONFLICT",false},
	}{
		var stdout,stderr bytes.Buffer
		code:=runAgentJSON([]string{"apply","--json"},func(out,errout io.Writer)int{
			_,_ = io.WriteString(errout,tc.message)
			return 3
		},&stdout,&stderr)
		if code!=3{t.Fatalf("%s exit=%d",tc.message,code)}
		var reply map[string]any
		if err:=json.Unmarshal(stdout.Bytes(),&reply);err!=nil{t.Fatalf("not JSON: %v",err)}
		if reply["error_code"]!=tc.code||reply["retryable"]!=tc.retry||reply["ok"]!=false||reply["next_action"]==""{
			t.Errorf("%s: %+v",tc.message,reply)
		}
	}
}

func TestAgentJSONPreservesStructuredFindings(t *testing.T){
	var stdout,stderr bytes.Buffer
	code:=runAgentJSON([]string{"validate","--json"},func(out,errout io.Writer)int{
		_ = json.NewEncoder(out).Encode(map[string]any{
			"status":"invalid","ok":false,
			"findings":[]map[string]any{{"code":"REQUIRED_FIELD","path":"tasks[0].goal","message":"goal required"}},
		})
		return 2
	},&stdout,&stderr)
	var reply map[string]any
	if code!=2||json.Unmarshal(stdout.Bytes(),&reply)!=nil||reply["error_code"]!="REQUIRED_FIELD"{
		t.Fatalf("structured findings lost: %+v",reply)
	}
}

func TestAgentJSONDoesNotAssumeSilentSuccess(t *testing.T){
	var stdout,stderr bytes.Buffer
	if code:=runAgentJSON([]string{"--json"},func(io.Writer,io.Writer)int{return 0},&stdout,&stderr);code==0{t.Fatal("empty success mistakenly accepted")}
	var reply map[string]any
	if err:=json.Unmarshal(stdout.Bytes(),&reply);err!=nil||reply["status"]!="not_run"{t.Fatalf("%v %+v",err,reply)}
}

func TestInstalledFrameworkAgentPlaybookLinks(t *testing.T){
	root:=filepath.Join("..","..","goassets","template","_task_mecca","framework")
	paths:=[]string{"CLI_OPERATIONS_PLAYBOOK.md","SESSION_GUIDE.md","EXECUTION_PROTOCOL.md","roles/root.md","roles/registrar.md","roles/controller.md","roles/worker.md"}
	for _,name:=range paths{
		data,err:=os.ReadFile(filepath.Join(root,name))
		if err!=nil{t.Fatalf("missing agent guide %s: %v",name,err)}
		if name!="CLI_OPERATIONS_PLAYBOOK.md"&&!strings.Contains(string(data),"CLI_OPERATIONS_PLAYBOOK.md"){t.Fatalf("role entrypoint %s doesn't discover CLI playbook",name)}
	}
	raw,err:=os.ReadFile(filepath.Join(root,"roles","registrar.md"))
	if err!=nil{t.Fatal(err)}
	if !bytes.Contains(raw,[]byte("contract apply"))||!bytes.Contains(raw,[]byte("fallback")){t.Fatal("registrar still lacks structured-first registration rules")}
}
