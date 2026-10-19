package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexNativeJSONLExactSessionEvidence(t *testing.T){
	home:=t.TempDir()
	t.Setenv("CODEX_HOME",home)
	session:="01a113a5-5be7-7292-b35d-17f2060b3d3d"
	dir:=filepath.Join(home,"sessions","2026","10","19")
	if err:=os.MkdirAll(dir,0700);err!=nil{t.Fatal(err)}
	path:=filepath.Join(dir,"rollout-2026-10-19T08-00-00-"+session+".jsonl")
	records:=strings.Join([]string{
		`{"type":"session_meta","payload":{"id":"01a113a5-5be7-7292-b35d-17f2060b3d3d","cwd":"/secret"}}`,
		`{"type":"event_msg","payload":{"type":"task_started","message":"very private source text"}}`,
		`{"type":"turn_context","payload":{"model":"private"}}`,
		`{"type":"event_msg","payload":{"type":"user_message","text":"SECRET_DO_NOT_SHOW"}}`,
	},"\\n")+"\\n"
	if err:=os.WriteFile(path,[]byte(records),0600);err!=nil{t.Fatal(err)}
	native:=inspectCodexSessionEvidence(session)
	if native.Status!="observed_native"||native.SessionMeta!=1||native.TaskStarted!=1||native.TurnContext!=1||!native.MetadataMatched{
		t.Fatalf("native evidence mismatch: %+v",native)
	}
	body,err:=json.Marshal(native)
	if err!=nil{t.Fatal(err)}
	if bytes.Contains(body,[]byte("SECRET_DO_NOT_SHOW"))||bytes.Contains(body,[]byte("/secret"))||bytes.Contains(body,[]byte("private source text")){
		t.Fatal("native transcript content leaked into diagnostic")
	}
	if len(native.SourceToken)!=16||strings.Contains(string(body),home){t.Fatal("raw session path unexpectedly exposed")}
}

func TestCodexNativeEvidenceMissingAmbiguousMismatch(t *testing.T){
	home:=t.TempDir();t.Setenv("CODEX_HOME",home)
	session:="01a113a5-5be7-7292-b35d-17f2060b3d3d"
	if result:=inspectCodexSessionEvidence(session);result.Status!="not_found"{t.Fatalf("missing native session %s",result.Status)}
	dir:=filepath.Join(home,"sessions","2026")
	if err:=os.MkdirAll(dir,0700);err!=nil{t.Fatal(err)}
	path:=filepath.Join(dir,"rollout-"+session+".jsonl")
	if err:=os.WriteFile(path,[]byte(`{"type":"session_meta","payload":{"id":"different-id"}}`),0600);err!=nil{t.Fatal(err)}
	if result:=inspectCodexSessionEvidence(session);result.Status!="mismatch"{t.Fatalf("incorrect metadata should fail: %+v",result)}
	dir2:=filepath.Join(home,"sessions","2025")
	if err:=os.MkdirAll(dir2,0700);err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(filepath.Join(dir2,"rollout-"+session+".jsonl"),[]byte("{}"),0600);err!=nil{t.Fatal(err)}
	if result:=inspectCodexSessionEvidence(session);result.Status!="ambiguous"{t.Fatalf("duplicate native session is ambiguous: %+v",result)}
}

func TestRuntimeDiagnoseIncludesNativeEvidenceWithoutMutation(t *testing.T){
	t.Setenv("CODEX_HOME",t.TempDir())
	t.Setenv("TASK_MECCA_HOME",t.TempDir())
	session:="01a113a5-5be7-7292-b35d-17f2060b3d3d"
	dir:=filepath.Join(os.Getenv("CODEX_HOME"),"sessions")
	if err:=os.MkdirAll(dir,0700);err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(filepath.Join(dir,"rollout-"+session+".jsonl"),[]byte(
		`{"type":"session_meta","payload":{"id":"01a113a5-5be7-7292-b35d-17f2060b3d3d"}}`+"\n"+`{"type":"event_msg","payload":{"type":"task_started"}}`+"\n"),0600);err!=nil{t.Fatal(err)}
	project:=t.TempDir()
	if err:=os.MkdirAll(filepath.Join(project,"_task_mecca"),0700);err!=nil{t.Fatal(err)}
	var stdout,stderr bytes.Buffer
	code:=runRuntimeDiagnoseCLI([]string{"--project",project,"--provider","codex","--session-id",session,"--json"},&stdout,&stderr)
	if code!=0{t.Fatalf("diagnose code=%d: %s",code,stderr.String())}
	var result runtimeDiagnostic
	if err:=json.Unmarshal(stdout.Bytes(),&result);err!=nil{t.Fatal(err)}
	if result.NativeLog==nil||result.NativeLog.Status!="observed_native"||result.NativeLog.TaskStarted!=1{t.Fatalf("missing native evidence %+v",result)}
	if result.NativeOperationAttempted||result.ReconciliationPerformed{t.Fatal("diagnose performed forbidden native action")}
}
