package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
)

const contractMaxInputBytes = 8 << 20

// This CLI is intentionally read-only. Registration, ID allocation, user
// approval and lifecycle writes remain owned by the existing canonical flows.
type contractManifest struct {
	SchemaVersion int            `json:"schema_version"`
	Tasks         []contractTask `json:"tasks"`
}

type contractTask struct {
	Key                string   `json:"key"`
	Title              string   `json:"title"`
	Kind               string   `json:"kind"`
	Goal               string   `json:"goal"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	Background         string   `json:"background,omitempty"`
	Requirements       string   `json:"requirements,omitempty"`
	Constraints        string   `json:"constraints,omitempty"`
	SourceLabel        string   `json:"source_label,omitempty"`
	SourceURL          string   `json:"source_url,omitempty"`
	SourceText         string   `json:"source_text,omitempty"`
	RegisteredBy       string   `json:"registered_by,omitempty"`
	Tags               []string `json:"tags,omitempty"`
	DependsOn          []string `json:"depends_on,omitempty"`
	Summary            string   `json:"summary,omitempty"`
}

type contractFinding struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type contractDocument struct {
	Key          string `json:"key"`
	SourceSHA256 string `json:"source_sha256,omitempty"`
	Markdown     string `json:"markdown"`
}

type contractResult struct {
	OK            bool               `json:"ok"`
	Status        string             `json:"status"`
	SchemaVersion int                `json:"schema_version"`
	TaskCount     int                `json:"task_count"`
	Findings      []contractFinding  `json:"findings"`
	Documents     []contractDocument `json:"documents,omitempty"`
	Written       bool               `json:"written"`
}

func contractUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: task-mecca contract validate --input <manifest.json|-> [--json]")
	fmt.Fprintln(out, "       task-mecca contract preview  --input <manifest.json|-> [--json]")
	fmt.Fprintln(out, "Read-only: validate source fields, or preview canonical Markdown without creating tasks.")
	fmt.Fprintln(out, "Input: schema_version=1; tasks[]: key, title, kind(simple|defined), goal, acceptance_criteria[]")
	fmt.Fprintln(out, "Optional: source_label, source_url, source_text, background, requirements, constraints, tags[], depends_on[].")
	fmt.Fprintln(out, "validate and preview are read-only. Batch registration uses explicit plan/apply/status.")
	batchUsage(out)
	decisionUsage(out)
}

func runContractCLI(args []string, out, stderr io.Writer) int {
	if len(args)==0 || args[0]=="--help" || args[0]=="-h" || args[0]=="help" {
		contractUsage(out)
		return 0
	}
	action:=args[0]
	if action=="decision" { return runContractDecisionCLI(args[1:],out,stderr) }
	if action=="plan" || action=="apply" || action=="status" { return runContractBatchCLI(action,args[1:],out,stderr) }
	if action!="validate" && action!="preview" {
		fmt.Fprintf(stderr,"unknown contract action: %s (use validate or preview)\n",action)
		contractUsage(stderr)
		return 2
	}
	flags:=flag.NewFlagSet("contract "+action,flag.ContinueOnError)
	flags.SetOutput(stderr)
	input:=flags.String("input","-","JSON manifest file, or - for standard input")
	jsonOutput:=flags.Bool("json",false,"emit machine-readable JSON result")
	if err:=flags.Parse(args[1:]); err!=nil { return 2 }
	if flags.NArg()!=0 {
		fmt.Fprintln(stderr,"contract accepts no positional arguments; use --input FILE")
		return 2
	}
	payload,err:=readContractInput(*input)
	if err!=nil {
		result:=contractResult{Status:"failed",Findings:[]contractFinding{{Code:"INPUT_ERROR",Path:"input",Message:err.Error()}},Written:false}
		return emitContractResult(result,*jsonOutput,out,stderr,2)
	}
	var manifest contractManifest
	decoder:=json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err:=decoder.Decode(&manifest); err!=nil {
		result:=contractResult{Status:"failed",Findings:[]contractFinding{{Code:"INVALID_JSON",Path:"input",Message:err.Error()}},Written:false}
		return emitContractResult(result,*jsonOutput,out,stderr,2)
	}
	var trailing any
	if err:=decoder.Decode(&trailing); err!=io.EOF {
		result:=contractResult{Status:"failed",Findings:[]contractFinding{{Code:"TRAILING_JSON",Path:"input",Message:"expected a single JSON manifest"}},Written:false}
		return emitContractResult(result,*jsonOutput,out,stderr,2)
	}
	findings:=validateContractManifest(manifest)
	result:=contractResult{OK:len(findings)==0,Status:"valid",SchemaVersion:manifest.SchemaVersion,TaskCount:len(manifest.Tasks),Findings:findings,Written:false}
	if len(findings)>0 {
		result.Status="invalid"
		return emitContractResult(result,*jsonOutput,out,stderr,2)
	}
	if action=="preview" {
		for _,task:=range manifest.Tasks {
			digest:=sha256.Sum256([]byte(task.SourceText))
			document:=contractDocument{Key:task.Key,Markdown:renderContract(task)}
			if task.SourceText!="" {document.SourceSHA256=hex.EncodeToString(digest[:])}
			result.Documents=append(result.Documents,document)
		}
	}
	return emitContractResult(result,*jsonOutput,out,stderr,0)
}

func readContractInput(path string)([]byte,error) {
	var reader io.Reader=os.Stdin
	if path!="-" {
		file,err:=os.Open(path)
		if err!=nil {return nil,err}
		defer file.Close()
		reader=file
	}
	body,err:=io.ReadAll(io.LimitReader(reader,contractMaxInputBytes+1))
	if err!=nil {return nil,err}
	if len(body)>contractMaxInputBytes {return nil,fmt.Errorf("manifest exceeds %d bytes",contractMaxInputBytes)}
	if len(bytes.TrimSpace(body))==0{return nil,errors.New("empty JSON manifest")}
	return body,nil
}

func emitContractResult(result contractResult,asJSON bool,out,stderr io.Writer,code int)int {
	if asJSON {
		if err:=json.NewEncoder(out).Encode(result);err!=nil {fmt.Fprintln(stderr,err);return 1}
		return code
	}
	if !result.OK {
		for _,finding:=range result.Findings {fmt.Fprintf(stderr,"%s %s: %s\n",finding.Code,finding.Path,finding.Message)}
		return code
	}
	fmt.Fprintf(out,"Valid contract manifest: %d task(s), no files written.\n",result.TaskCount)
	for _,document:=range result.Documents {
		fmt.Fprintf(out,"\n--- preview: %s ---\n%s\n",document.Key,document.Markdown)
	}
	return code
}

func validateContractManifest(manifest contractManifest) []contractFinding {
	findings:=[]contractFinding{}
	add:=func(code,path,msg string){findings=append(findings,contractFinding{Code:code,Path:path,Message:msg})}
	if manifest.SchemaVersion!=1 {add("SCHEMA_VERSION","schema_version","supported schema_version is 1")}
	if len(manifest.Tasks)==0 {add("EMPTY_TASKS","tasks","provide at least one task")}
	if len(manifest.Tasks)>500 {add("TOO_MANY_TASKS","tasks","maximum 500 tasks per manifest")}
	keys:=map[string]int{}
	for i,task:=range manifest.Tasks {
		prefix:=fmt.Sprintf("tasks[%d]",i)
		key:=strings.TrimSpace(task.Key)
		if key=="" {add("REQUIRED_FIELD",prefix+".key","stable external key is required")} else if old,ok:=keys[key];ok {
			add("DUPLICATE_KEY",prefix+".key",fmt.Sprintf("duplicate key %q (first at tasks[%d])",key,old))
		} else {keys[key]=i}
		if strings.TrimSpace(task.Title)=="" {add("REQUIRED_FIELD",prefix+".title","title is required")}
		if task.Kind!="simple" && task.Kind!="defined" {add("INVALID_KIND",prefix+".kind","kind must be simple or defined")}
		if strings.TrimSpace(task.Goal)=="" {add("REQUIRED_FIELD",prefix+".goal","goal is required")}
		if markdownHeading(task.Title) || strings.Contains(task.Title,"\n") {add("HEADING_COLLISION",prefix+".title","title must be a single line without Markdown headings")}
		for _,field:=range []struct{name,value string}{{"goal",task.Goal},{"background",task.Background},{"requirements",task.Requirements},{"constraints",task.Constraints}} {
			if containsHeading(field.value){add("HEADING_COLLISION",prefix+"."+field.name,"standalone Markdown headings would change the canonical contract structure")}
		}
		if len(task.AcceptanceCriteria)==0 {add("REQUIRED_FIELD",prefix+".acceptance_criteria","provide at least one acceptance criterion")}
		for j,item:=range task.AcceptanceCriteria {
			if strings.TrimSpace(item)=="" || strings.ContainsAny(item,"\r\n") {add("INVALID_CRITERION",fmt.Sprintf("%s.acceptance_criteria[%d]",prefix,j),"each criterion must be a nonempty single line")}
		}
		if task.SourceURL!="" {
			parsed,err:=url.Parse(task.SourceURL)
			if err!=nil || parsed.Host=="" || (parsed.Scheme!="https" && parsed.Scheme!="http") || parsed.User!=nil {
				add("INVALID_SOURCE_URL",prefix+".source_url","source URL must be an absolute http(s) URL without embedded credentials")
			}
			if strings.TrimSpace(task.SourceLabel)==""{add("REQUIRED_FIELD",prefix+".source_label","source_label is required with source_url")}
			if task.SourceText==""{add("REQUIRED_FIELD",prefix+".source_text","source_text must preserve the complete original source")}
		}
		if strings.ContainsAny(task.SourceLabel,"\r\n") || strings.Contains(task.SourceLabel,"]") {add("INVALID_SOURCE_LABEL",prefix+".source_label","source label must be one line and contain no ]")}
		deps:=map[string]bool{}
		for j,dep:=range task.DependsOn {
			if strings.TrimSpace(dep)=="" {add("INVALID_DEPENDENCY",fmt.Sprintf("%s.depends_on[%d]",prefix,j),"dependency key must not be empty")}
			if dep==key && key!="" {add("SELF_DEPENDENCY",prefix+".depends_on","a task cannot depend on itself")}
			if deps[dep] {add("DUPLICATE_DEPENDENCY",prefix+".depends_on",fmt.Sprintf("duplicate dependency %q",dep))}
			deps[dep]=true
		}
		for j,tag:=range task.Tags {
			if strings.TrimSpace(tag)=="" || strings.ContainsAny(tag,"\r\n,") {add("INVALID_TAG",fmt.Sprintf("%s.tags[%d]",prefix,j),"tag must be a nonempty single taxonomy value")}
		}
	}
	for i,task:=range manifest.Tasks {
		for _,dep:=range task.DependsOn {
			if _,ok:=keys[dep];!ok {add("UNKNOWN_DEPENDENCY",fmt.Sprintf("tasks[%d].depends_on",i),fmt.Sprintf("dependency %q is not in this manifest; external resolution is not available in read-only preview",dep))}
		}
	}
	colors:=map[string]int{}
	var visit func(key string)
	visit=func(key string){
		if colors[key]==1 {return}
		colors[key]=1
		task:=manifest.Tasks[keys[key]]
		for _,dep:=range task.DependsOn {
			if _,ok:=keys[dep];!ok {continue}
			if colors[dep]==1 {add("DEPENDENCY_CYCLE",fmt.Sprintf("tasks[%d].depends_on",keys[key]),fmt.Sprintf("cycle through %q",dep));continue}
			if colors[dep]==0 {visit(dep)}
		}
		colors[key]=2
	}
	for key:=range keys {if colors[key]==0{visit(key)}}
	return findings
}

func markdownHeading(value string)bool{return strings.HasPrefix(strings.TrimSpace(value),"# ")}
func containsHeading(value string)bool {
	for _,line:=range strings.Split(value,"\n"){
		trimmed:=strings.TrimSpace(line)
		if strings.HasPrefix(trimmed,"## ") || strings.HasPrefix(trimmed,"### ") || strings.HasPrefix(trimmed,"#### ") {return true}
	}
	return false
}

func renderContract(task contractTask)string {
	var b strings.Builder
	add:=func(s string){b.WriteString(s);b.WriteByte('\n')}
	fallback:=func(s string)string{if strings.TrimSpace(s)==""{return "-"};return s}
	add("# <ID> "+task.Title)
	add("")
	add("## 작업 개요")
	add("")
	add("- 등록자: "+fallback(task.RegisteredBy))
	if task.SourceURL!="" {
		add("- 출처: ["+task.SourceLabel+"](<"+task.SourceURL+">)")
	}else{add("- 출처: -")}
	add("- Agent: -")
	add("- 변경범위: -")
	if len(task.DependsOn)>0{add("- 선행: 미연결 외부 키 ["+strings.Join(task.DependsOn,", ")+"]")}else{add("- 선행: -")}
	add("- 연관: -")
	if len(task.Tags)>0{add("- Tags: "+strings.Join(task.Tags,", "))}else{add("- Tags: -")}
	add("")
	add("## 핵심 요약")
	add("")
	add("- 목적: "+fallback(task.Summary))
	add("- 핵심 변경: -")
	add("- 상태·결과: 등록 전 검토")
	add("- 확인·후속: ID 및 선행관계는 실제 등록 시 확정")
	add("")
	if task.Kind=="defined" {
		add("## 요건 정의서")
		add("")
		add("### 배경 및 문제")
		add("")
		add(fallback(task.Background))
		add("")
		add("### 목표")
		add("")
		add(task.Goal)
		add("")
		add("### 요구사항")
		add("")
		add(fallback(task.Requirements))
		add("")
		add("### 범위")
		add("")
		add("#### 포함")
		add("")
		add("-")
		add("")
		add("#### 제외")
		add("")
		add("-")
		add("")
	}else{
		add("## 작업 정의")
		add("")
		add("### 목표")
		add("")
		add(task.Goal)
		add("")
	}
	add("### 수용 기준")
	add("")
	for _,item:=range task.AcceptanceCriteria {add("- [ ] "+item)}
	if task.Kind=="defined"{
		add("")
		add("### 제약 및 보존 조건")
		add("")
		add(fallback(task.Constraints))
	}
	add("")
	add("## 실행 정보")
	add("")
	add("- RuntimeProvider: unknown")
	add("- Dispatch상태: unknown")
	add("- 실행근거: unknown")
	add("- Fallback근거: -")
	add("")
	add("## 작업 노트")
	add("")
	add("- 대기: -")
	add("- 대기유형: -")
	add("- 재개조건: -")
	add("- 대기근거: -")
	add("- 메모: -")
	add("")
	add("## 결과")
	add("")
	add("-")
	add("")
	add("## 검증")
	add("")
	add("-")
	if task.SourceText!="" {
		add("")
		add("## 원천 전문 (변경 금지)")
		add("")
		sum:=sha256.Sum256([]byte(task.SourceText))
		add("SHA-256: "+hex.EncodeToString(sum[:]))
		add("")
		// Prefix every source line, including Markdown headings, to prevent
		// source text from being mistaken for canonical contract sections.
		for _,line:=range strings.Split(task.SourceText,"\n") {add("> "+line)}
	}
	return strings.TrimRight(b.String(),"\n")+"\n"
}
