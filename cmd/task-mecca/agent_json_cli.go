package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// runAgentJSON is the shared adapter for the new agent-facing CLI commands.
// It never retries or changes a failed operation; it only makes error results
// machine-readable without losing an existing structured result.
func runAgentJSON(args []string, run func(io.Writer,io.Writer) int,out,stderr io.Writer)int{
	requestJSON:=false
	for _,arg:=range args{
		if arg=="--json"||arg=="-json"{requestJSON=true}
	}
	if !requestJSON{return run(out,stderr)}
	var stdout,errbuf bytes.Buffer
	code:=run(&stdout,&errbuf)
	raw:=bytes.TrimSpace(stdout.Bytes())
	var object map[string]any
	if len(raw)>0&&json.Unmarshal(raw,&object)==nil&&object!=nil{
		if code!=0{
			normalizeAgentFailure(object,code,errbuf.String())
		}
		if err:=json.NewEncoder(out).Encode(object);err!=nil{fmt.Fprintln(stderr,err);return 1}
		return code
	}
	if code==0 && len(raw)>0{
		// Some legacy subcommands emit text despite --json. Do not convert
		// unstructured output into a fake JSON success.
		_,_ = out.Write(stdout.Bytes())
		return code
	}
	if code==0 {
		// Empty result is not an error, but never claim an operation occurred.
		_ = json.NewEncoder(out).Encode(map[string]any{"status":"not_run","ok":false,"error_code":"EMPTY_RESPONSE","retryable":false,"next_action":"Inspect CLI command support and actual execution evidence."})
		return 1
	}
	message:=strings.TrimSpace(errbuf.String())
	if message==""{message=string(raw)}
	if message==""{message="operation failed without diagnostics"}
	result:=map[string]any{"status":"failed","ok":false,"exit_code":code,"error_code":agentFailureCode(message),"message":message,"retryable":agentRetryable(message),"next_action":agentRecovery(message)}
	if err:=json.NewEncoder(out).Encode(result);err!=nil{fmt.Fprintln(stderr,err);return 1}
	return code
}

func normalizeAgentFailure(data map[string]any,code int,stderr string){
	if _,exists:=data["ok"];!exists{data["ok"]=false}
	data["exit_code"]=code
	message:=strings.TrimSpace(stderr)
	if message==""{
		if rows,ok:=data["findings"].([]any);ok&&len(rows)>0{
			if first,ok:=rows[0].(map[string]any);ok{
				if s,ok:=first["code"].(string);ok{data["error_code"]=s}
				if s,ok:=first["message"].(string);ok{message=s}
			}
		}
	}
	if message!=""{data["message"]=message}
	if _,exists:=data["error_code"];!exists{data["error_code"]=agentFailureCode(message)}
	if _,exists:=data["retryable"];!exists{data["retryable"]=agentRetryable(message)}
	if _,exists:=data["next_action"];!exists{data["next_action"]=agentRecovery(message)}
}

func agentFailureCode(message string)string{
	m:=strings.ToUpper(message)
	switch{
	case strings.Contains(m,"STALE_PLAN"):return "STALE_PLAN"
	case strings.Contains(m,"IDEMPOTENCY_CONFLICT"):return "IDEMPOTENCY_CONFLICT"
	case strings.Contains(m,"DECISION_ID_CONFLICT"):return "DECISION_ID_CONFLICT"
	case strings.Contains(m,"FILE_CONFLICT"):return "FILE_CONFLICT"
	case strings.Contains(m,"APPROVAL")||strings.Contains(m,"--APPROVE"):return "APPROVAL_REQUIRED"
	case strings.Contains(m,"UNKNOWN_TAG"):return "UNKNOWN_TAG"
	case strings.Contains(m,"PERMISSION")||strings.Contains(m,"DENIED"):return "ACCESS_DENIED"
	case strings.Contains(m,"NOT FOUND"):return "NOT_FOUND"
	case strings.Contains(m,"INVALID")||strings.Contains(m,"REQUIRED")||strings.Contains(m,"UNKNOWN COMMAND"):return "INVALID_INPUT"
	default:return "OPERATION_FAILED"
	}
}
func agentRetryable(message string)bool{
	switch agentFailureCode(message){
	case "STALE_PLAN","ACCESS_DENIED","OPERATION_FAILED":return true
	default:return false
	}
}
func agentRecovery(message string)string{
	switch agentFailureCode(message){
	case "STALE_PLAN":return "Run contract plan again and review the changed plan before any new apply."
	case "IDEMPOTENCY_CONFLICT":return "Inspect the original request ID, manifest, and journal; never change IDs to bypass a conflict."
	case "DECISION_ID_CONFLICT":return "Inspect the immutable approval event and actual user evidence; never reuse an event ID for a new decision."
	case "FILE_CONFLICT":return "Inspect the existing canonical task; preserve user changes and request reconciliation."
	case "APPROVAL_REQUIRED":return "Verify an actual explicit user decision; do not infer approval."
	case "UNKNOWN_TAG":return "Resolve or define the canonical active project tag with user-approved taxonomy."
	case "ACCESS_DENIED":return "Check permissions and project state; retry only after authoritative access is restored."
	case "NOT_FOUND":return "Check exact project root, task or session identity; do not fabricate a record."
	default:return "Read the error message and supported --help; do not retry a destructive operation blindly."
	}
}
