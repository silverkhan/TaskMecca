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
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/projectguard"
)

type batchTask struct {
	Key string `json:"key"`
	ID string `json:"id"`
	Path string `json:"path"`
	SourceSHA256 string `json:"source_sha256,omitempty"`
	DependsOn []string `json:"depends_on"`
	ContentSHA256 string `json:"content_sha256"`
	EventID string `json:"event_id"`
}

type batchPlan struct {
	Status string `json:"status"`
	Project string `json:"project"`
	Prefix string `json:"prefix"`
	RequestID string `json:"request_id"`
	ManifestSHA256 string `json:"manifest_sha256"`
	PlanDigest string `json:"plan_digest"`
	Folder string `json:"folder"`
	CatalogSHA256 string `json:"catalog_sha256"`
	Tasks []batchTask `json:"tasks"`
	Written bool `json:"written"`
}

type batchResult struct {
	Status string `json:"status"`
	RequestID string `json:"request_id"`
	PlanDigest string `json:"plan_digest"`
	Tasks []batchItemResult `json:"tasks"`
	Written bool `json:"written"`
	NextAction string `json:"next_action,omitempty"`
	Error string `json:"error,omitempty"`
}

type batchItemResult struct {
	Key string `json:"key"`
	ID string `json:"id"`
	File string `json:"file"`
	Registered bool `json:"registered"`
	LifecycleRecorded bool `json:"lifecycle_recorded"`
	Error string `json:"error,omitempty"`
}

var safeBatchRequest=regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,95}$`)
var safeBatchKey=regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:._/-]{0,190}$`)

func batchUsage(out io.Writer) {
	fmt.Fprintln(out,"task-mecca contract plan --project DIR --input FILE --request-id KEY [--prefix B] --json")
	fmt.Fprintln(out,"task-mecca contract apply --project DIR --input FILE --request-id KEY --expect-plan DIGEST --approve [--prefix B] --json")
	fmt.Fprintln(out,"task-mecca contract status --project DIR --request-id KEY --json")
	fmt.Fprintln(out,"plan is read-only; apply registers tasks only (never authorizes execution). Retry with same key/input/digest.")
}

func runContractBatchCLI(action string,args []string,out,stderr io.Writer)int{
	flags:=flag.NewFlagSet("contract "+action,flag.ContinueOnError)
	flags.SetOutput(stderr)
	project:=flags.String("project",".","exact project root")
	input:=flags.String("input","-","JSON manifest (or - for stdin)")
	requestID:=flags.String("request-id","","stable registration request identity")
	prefix:=flags.String("prefix","B","backlog ID prefix")
	expect:=flags.String("expect-plan","","plan digest from read-only plan")
	approve:=flags.Bool("approve",false,"explicit approval to register (never execution approval)")
	asJSON:=flags.Bool("json",false,"machine-readable JSON")
	if err:=flags.Parse(args);err!=nil{return 2}
	if flags.NArg()!=0 {fmt.Fprintln(stderr,"unexpected positional argument");return 2}
	if !safeBatchRequest.MatchString(*requestID){fmt.Fprintln(stderr,"--request-id must be 1-96 letters/digits/-/_");return 2}
	if !regexp.MustCompile(`^[A-Z]+$`).MatchString(*prefix){fmt.Fprintln(stderr,"--prefix must contain uppercase A-Z only");return 2}
	if action=="apply" && (!*approve || *expect=="") {
		fmt.Fprintln(stderr,"apply requires --approve and --expect-plan from contract plan; registration does not authorize execution")
		return 2
	}
	if action=="plan" && (*approve||*expect!=""){fmt.Fprintln(stderr,"plan never accepts approval or expected digest");return 2}
	abs,err:=filepath.Abs(*project)
	if err!=nil {fmt.Fprintln(stderr,err);return 2}
	if action=="status" {
		plan,err:=readBatchPlan(batchJournalPath(abs,*requestID))
		if err!=nil {fmt.Fprintln(stderr,err);return 1}
		result:=batchStatus(plan)
		return printBatchResult(result,*asJSON,out,stderr)
	}
	raw,err:=readContractInput(*input)
	if err!=nil {fmt.Fprintln(stderr,err);return 2}
	var manifest contractManifest
	dec:=json.NewDecoder(bytes.NewReader(raw));dec.DisallowUnknownFields()
	if err:=dec.Decode(&manifest);err!=nil{fmt.Fprintln(stderr,err);return 2}
	var more any
	if err:=dec.Decode(&more);err!=io.EOF {fmt.Fprintln(stderr,"manifest must contain one JSON document");return 2}
	findings:=validateContractManifest(manifest)
	for i,t:=range manifest.Tasks {
		if !safeBatchKey.MatchString(t.Key) {findings=append(findings,contractFinding{Code:"INVALID_KEY",Path:fmt.Sprintf("tasks[%d].key",i),Message:"use stable ASCII source keys without spaces or newlines"})}
	}
	if len(findings)>0 {
		result:=contractResult{Status:"invalid",TaskCount:len(manifest.Tasks),Findings:findings,Written:false}
		return emitContractResult(result,*asJSON,out,stderr,2)
	}
	manifestHash:=hashBatchBytes(raw)
	if action=="plan" {
		plan,err:=makeContractBatchPlan(abs,*prefix,*requestID,manifest,manifestHash)
		if err!=nil {fmt.Fprintln(stderr,err);return 1}
		if *asJSON {return emitBatchJSON(plan,out,stderr)}
		fmt.Fprintf(out,"Plan %s (%d tasks), no files written\n",plan.PlanDigest,len(plan.Tasks))
		for _,item:=range plan.Tasks {fmt.Fprintf(out,"%s -> %s, depends_on=%s\n",item.Key,item.ID,strings.Join(item.DependsOn,","))}
		return 0
	}
	releaseGuard,err:=projectguard.AcquireWrite(abs)
	if err!=nil {fmt.Fprintln(stderr,err);return 1}
	defer releaseGuard()
	release,err:=lockBatchRegistration(abs)
	if err!=nil {fmt.Fprintln(stderr,err);return 1}
	defer release()
	path:=batchJournalPath(abs,*requestID)
	plan,err:=readBatchPlan(path)
	if errors.Is(err,os.ErrNotExist) {
		plan,err=makeContractBatchPlan(abs,*prefix,*requestID,manifest,manifestHash)
		if err!=nil {fmt.Fprintln(stderr,err);return 1}
		if plan.PlanDigest!=*expect {fmt.Fprintln(stderr,"STALE_PLAN: project/backlog/input changed; run contract plan again");return 3}
		if err=writeBatchPlan(path,plan);err!=nil{fmt.Fprintln(stderr,err);return 1}
	}else if err!=nil{
		fmt.Fprintln(stderr,err);return 1
	}else if plan.ManifestSHA256!=manifestHash || plan.Prefix!=*prefix || plan.PlanDigest!=*expect || plan.Project!=abs{
		fmt.Fprintln(stderr,"IDEMPOTENCY_CONFLICT: request already used with different manifest, project, prefix or plan digest")
		return 3
	}
	result:=applyContractBatch(plan,manifest)
	if *asJSON {if err:=json.NewEncoder(out).Encode(result);err!=nil{fmt.Fprintln(stderr,err);return 1}}else{
		fmt.Fprintf(out,"%s: %d tasks, request=%s\n",result.Status,len(result.Tasks),result.RequestID)
		for _,item:=range result.Tasks {fmt.Fprintf(out,"%s %s registered=%t lifecycle=%t %s\n",item.Key,item.ID,item.Registered,item.LifecycleRecorded,item.Error)}
		if result.NextAction!="" {fmt.Fprintln(out,result.NextAction)}
	}
	if result.Status!="success"{return 3}
	return 0
}

func emitBatchJSON(value any,out,stderr io.Writer)int{
	if err:=json.NewEncoder(out).Encode(value);err!=nil{fmt.Fprintln(stderr,err);return 1}
	return 0
}
func printBatchResult(v batchResult,asJSON bool,out,stderr io.Writer)int{
	if asJSON {if err:=json.NewEncoder(out).Encode(v);err!=nil{fmt.Fprintln(stderr,err);return 1};return 0}
	fmt.Fprintf(out,"%s request=%s\n",v.Status,v.RequestID)
	for _,task:=range v.Tasks{fmt.Fprintf(out,"%s %s file=%t event=%t\n",task.Key,task.ID,task.Registered,task.LifecycleRecorded)}
	return 0
}
func hashBatchBytes(body []byte)string{sum:=sha256.Sum256(body);return hex.EncodeToString(sum[:])}
func batchJournalPath(project,id string)string{return filepath.Join(project,"_task_mecca","data","cli_batches",id+".json")}

func makeContractBatchPlan(project,prefix,request string,m contractManifest,manifestSHA string)(batchPlan,error){
	plan:=batchPlan{Status:"ready",Project:project,Prefix:prefix,RequestID:request,ManifestSHA256:manifestSHA,Tasks:[]batchTask{},Written:false}
	folder,err:=backlog.Select(project,"")
	if err!=nil{return plan,err}
	if folder==""{folder=filepath.Join(project,"_task_mecca","data","backlog")}
	plan.Folder=folder
	if err:=safeBatchFolder(folder,project);err!=nil{return plan,err}
	rows,err:=backlog.Catalog(project,"")
	if err!=nil{return plan,err}
	var catalog strings.Builder
	maxID:=0
	for _,row:=range rows {
		fmt.Fprintf(&catalog,"%s\x00%s\x00%s\n",row.ID,row.Path,hashBatchBytes([]byte(row.RawMarkdown)))
		if strings.HasPrefix(row.ID,prefix+"-"){
			n,e:=strconv.Atoi(strings.TrimPrefix(row.ID,prefix+"-"));if e==nil&&n>maxID{maxID=n}
		}
		for _,task:=range m.Tasks {
			if row.Fields["외부키"]!="" && row.Fields["외부키"]==task.Key {return plan,fmt.Errorf("EXTERNAL_KEY_EXISTS: %s already registered as %s",task.Key,row.ID)}
		}
	}
	plan.CatalogSHA256=hashBatchBytes([]byte(catalog.String()))
	ids:=map[string]string{}
	for i,task:=range m.Tasks{
		number:=maxID+i+1
		if number>999999{return plan,fmt.Errorf("ID_LIMIT: no next %s identifier",prefix)}
		ids[task.Key]=fmt.Sprintf("%s-%d",prefix,number)
	}
	for i,task:=range m.Tasks{
		number:=maxID+i+1
		id:=ids[task.Key]
		var deps []string
		for _,key:=range task.DependsOn{deps=append(deps,ids[key])}
		slugHash:=hashBatchBytes([]byte(task.Key))[:10]
		path:=filepath.Join(folder,fmt.Sprintf("%06d.%s.item-%s.todo.md",number,id,slugHash))
		content:=renderRegisteredContract(task,id,deps)
		item:=batchTask{Key:task.Key,ID:id,Path:path,ContentSHA256:hashBatchBytes([]byte(content)),EventID:backlog.LifecycleEventID(id,"registered","cli-batch:"+request+":"+task.Key),DependsOn:deps}
		if task.SourceText!=""{item.SourceSHA256=hashBatchBytes([]byte(task.SourceText))}
		plan.Tasks=append(plan.Tasks,item)
	}
	body,_:=json.Marshal(struct{
		Project string
		Prefix string
		RequestID string
		Manifest string
		Folder string
		Catalog string
		Tasks []batchTask
	}{plan.Project,plan.Prefix,plan.RequestID,manifestSHA,folder,plan.CatalogSHA256,plan.Tasks})
	plan.PlanDigest=hashBatchBytes(body)
	return plan,nil
}

// Never write into a symlinked backlog directory; do not escape the project boundary.
func safeBatchFolder(folder,project string)error {
	base:=filepath.Join(project,"_task_mecca")
	relative,err:=filepath.Rel(project,folder)
	if err!=nil||relative==".."||strings.HasPrefix(relative,".."+string(filepath.Separator)){return errors.New("unsafe backlog folder outside project")}
	for cur:=folder; ;cur=filepath.Dir(cur){
		info,err:=os.Lstat(cur)
		if err==nil && info.Mode()&os.ModeSymlink!=0{return fmt.Errorf("symlink path component denied: %s",cur)}
		if err!=nil && !os.IsNotExist(err){return err}
		if cur==project{break}
		if filepath.Dir(cur)==cur{return errors.New("project boundary missing")}
	}
	_ = base
	return nil
}

func renderRegisteredContract(task contractTask,id string,deps []string)string{
	source:=renderContract(task)
	source=strings.Replace(source,"# <ID> ","# "+id+" ",1)
	source=strings.Replace(source,"- 상태·결과: 등록 전 검토","- 상태·결과: 등록됨",1)
	source=strings.Replace(source,"- 확인·후속: ID 및 선행관계는 실제 등록 시 확정","- 확인·후속: 등록 완료; 실행 승인·구현 완료와 별개",1)
	if len(task.DependsOn)>0{source=strings.Replace(source,"- 선행: 미연결 외부 키 ["+strings.Join(task.DependsOn,", ")+"]","- 선행: "+strings.Join(deps,", "),1)}
	source=strings.Replace(source,"## 핵심 요약\n","- 외부키: "+task.Key+"\n\n## 핵심 요약\n",1)
	return source
}

func writeBatchPlan(path string,p batchPlan)error{
	if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return err}
	body,err:=json.MarshalIndent(p,"","  ")
	if err!=nil{return err}
	file,err:=os.OpenFile(path,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600)
	if err!=nil{return err}
	if _,err=file.Write(append(body,'\n'));err==nil{err=file.Sync()}
	if closeErr:=file.Close();err==nil{err=closeErr}
	return err
}

func readBatchPlan(path string)(batchPlan,error){
	var plan batchPlan
	body,err:=os.ReadFile(path)
	if err!=nil{return plan,err}
	err=json.Unmarshal(body,&plan)
	if err==nil {
		if plan.PlanDigest==""||plan.RequestID==""||len(plan.Tasks)==0 {err=errors.New("incomplete batch journal")}
	}
	return plan,err
}

func applyContractBatch(plan batchPlan,m contractManifest)batchResult{
	r:=batchResult{Status:"success",RequestID:plan.RequestID,PlanDigest:plan.PlanDigest,Tasks:[]batchItemResult{},Written:true}
	if len(plan.Tasks)!=len(m.Tasks){r.Status="failed";r.Error="manifest/journal item count mismatch";return r}
	if err:=safeBatchFolder(plan.Folder,plan.Project);err!=nil{r.Status="failed";r.Error=err.Error();return r}
	if err:=os.MkdirAll(plan.Folder,0755);err!=nil{r.Status="failed";r.Error=err.Error();return r}
	for i,item:=range plan.Tasks{
		row:=batchItemResult{Key:item.Key,ID:item.ID,File:item.Path}
		if item.Key!=m.Tasks[i].Key {
			row.Error="manifest/journal order mismatch";r.Status="partial";r.Tasks=append(r.Tasks,row);break
		}
		expected:=[]byte(renderRegisteredContract(m.Tasks[i],item.ID,item.DependsOn))
		if hashBatchBytes(expected)!=item.ContentSHA256 {
			row.Error="journal/document digest mismatch";r.Status="partial";r.Tasks=append(r.Tasks,row);break
		}
		if err:=writeOrVerifyBatchFile(item.Path,expected);err!=nil {
			row.Error=err.Error();r.Status="partial";r.Tasks=append(r.Tasks,row);break
		}
		row.Registered=true
		_,err:=backlog.RecordLifecycleTransition(plan.Project,backlog.LifecycleTransition{
			TaskID:item.ID,EventID:item.EventID,Kind:"registered",Actor:"/root/registrar",
			EvidenceSource:"registrar_report",EvidenceRef:"cli-batch:"+plan.RequestID,
		},time.Now())
		if err!=nil {
			row.Error=err.Error();r.Status="partial";r.Tasks=append(r.Tasks,row);break
		}
		row.LifecycleRecorded=true
		r.Tasks=append(r.Tasks,row)
	}
	if r.Status!="success" {r.NextAction="Fix reported errors, then rerun contract apply with identical --request-id, input and --expect-plan; do not create a new request ID."}
	return r
}

func writeOrVerifyBatchFile(path string,expected []byte)error{
	old,err:=os.ReadFile(path)
	if err==nil {
		if bytes.Equal(old,expected){return nil}
		return fmt.Errorf("FILE_CONFLICT: existing file differs: %s",path)
	}
	if !os.IsNotExist(err){return err}
	// Stage and fsync before atomic exclusive publication. A crash leaves
	// only an ignorable staging file, never a truncated canonical task.
	staging,err:=os.CreateTemp(filepath.Dir(path),".cli-batch-*.tmp")
	if err!=nil{return err}
	defer os.Remove(staging.Name())
	if _,err=staging.Write(expected);err==nil{err=staging.Sync()}
	if closeErr:=staging.Close();err==nil{err=closeErr}
	if err!=nil{return err}
	if err=os.Link(staging.Name(),path);err!=nil{
		if os.IsExist(err) {
			previous,readErr:=os.ReadFile(path)
			if readErr==nil&&bytes.Equal(previous,expected){return nil}
			return fmt.Errorf("FILE_CONFLICT: destination was created concurrently: %s",path)
		}
		return fmt.Errorf("safe exclusive file publish failed (original retained): %w",err)
	}
	verify,err:=os.ReadFile(path)
	if err!=nil{return err}
	if !bytes.Equal(verify,expected){return errors.New("file verification failed")}
	return nil
}

func batchStatus(plan batchPlan)batchResult{
	r:=batchResult{RequestID:plan.RequestID,PlanDigest:plan.PlanDigest,Status:"success",Tasks:[]batchItemResult{},Written:true}
	scan,err:=backlog.ReadLifecycleTransitions(plan.Project)
	if err!=nil{r.Status="unknown";r.Error=err.Error();return r}
	events:=map[string]bool{}
	for _,e:=range scan.Events{events[e.EventID]=e.TaskID!=""}
	if len(scan.Findings)>0{r.Status="unknown";r.Error="lifecycle findings require inspection"}
	for _,item:=range plan.Tasks{
		entry:=batchItemResult{Key:item.Key,ID:item.ID,File:item.Path,LifecycleRecorded:events[item.EventID]}
		data,e:=os.ReadFile(item.Path)
		entry.Registered=e==nil&&hashBatchBytes(data)==item.ContentSHA256
		if e!=nil&&!os.IsNotExist(e){entry.Error=e.Error();r.Status="unknown"}
		if e==nil&&!entry.Registered{entry.Error="registered file changed or corrupted";r.Status="unknown"}
		if (!entry.Registered||!entry.LifecycleRecorded)&&r.Status=="success"{r.Status="partial"}
		r.Tasks=append(r.Tasks,entry)
	}
	if r.Status!="success"{r.NextAction="Use contract apply with the original request-id, input and plan digest after resolving errors."}
	return r
}
