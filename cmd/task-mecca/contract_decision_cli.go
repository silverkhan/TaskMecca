package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/projectguard"
)

type decisionEvent struct {
	EventID string `json:"event_id"`
	TaskID string `json:"task_id"`
	Kind string `json:"kind"`
	Actor string `json:"actor"`
	Evidence string `json:"evidence"`
	PolicyKey string `json:"policy_key,omitempty"`
	Text string `json:"text,omitempty"`
	UserConfirmed bool `json:"user_confirmed"`
	BaseSHA256 string `json:"base_sha256"`
	EffectiveSHA256 string `json:"effective_sha256"`
	RecordedAt string `json:"recorded_at"`
}

type decisionFinding struct {
	Code string `json:"code"`
	Message string `json:"message"`
	EventID string `json:"event_id,omitempty"`
}

type contractDecisionReport struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	ExecutionAuthorized bool `json:"execution_authorized"`
	ApprovalEventID string `json:"approval_event_id,omitempty"`
	CurrentSHA256 string `json:"current_sha256"`
	EffectiveSHA256 string `json:"effective_sha256"`
	BaseMarkdown string `json:"base_markdown"`
	Supplements []decisionEvent `json:"supplements"`
	PendingPolicies []string `json:"pending_policies"`
	Events []decisionEvent `json:"events"`
	Findings []decisionFinding `json:"findings"`
	AsOf string `json:"as_of"`
}

var decisionEventPattern=regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func decisionUsage(out io.Writer){
	fmt.Fprintln(out,"task-mecca contract decision record --project DIR --task B-123 --event-id STABLE-ID --kind execution_approved|execution_revoked|supplement|policy_pending|policy_confirmed --actor AGENT --evidence REF [--body-file FILE] [--policy-key KEY] [--user-confirmed] --json")
	fmt.Fprintln(out,"task-mecca contract decision show --project DIR --task B-123 --json")
	fmt.Fprintln(out,"Record only explicit user decisions; approval/supplement/policy confirmation require --user-confirmed. This does not wake or dispatch an agent.")
}

func runContractDecisionCLI(args []string,out,stderr io.Writer)int{
	if len(args)==0||args[0]=="help"||args[0]=="--help"{decisionUsage(out);return 0}
	action:=args[0]
	if action!="record"&&action!="show"{fmt.Fprintln(stderr,"decision action must be record or show");return 2}
	flags:=flag.NewFlagSet("contract decision "+action,flag.ContinueOnError)
	flags.SetOutput(stderr)
	project:=flags.String("project",".","canonical project")
	taskID:=flags.String("task","","existing backlog ID")
	eventID:=flags.String("event-id","","stable source event identity")
	kind:=flags.String("kind","","explicit decision type")
	actor:=flags.String("actor","","recording actor")
	evidence:=flags.String("evidence","","source reference to a real user decision")
	policyKey:=flags.String("policy-key","","policy identifier")
	bodyFile:=flags.String("body-file","","text supplement file; never reinterpreted by CLI")
	confirmed:=flags.Bool("user-confirmed",false,"the user explicitly confirmed the specified decision")
	asJSON:=flags.Bool("json",false,"machine-readable result")
	if err:=flags.Parse(args[1:]);err!=nil{return 2}
	if flags.NArg()!=0||strings.TrimSpace(*taskID)==""{fmt.Fprintln(stderr,"--task is required; no positional args");return 2}
	abs,err:=filepath.Abs(*project)
	if err!=nil{fmt.Fprintln(stderr,err);return 2}
	if action=="show"{
		report,err:=projectContractDecision(abs,*taskID)
		if err!=nil{fmt.Fprintln(stderr,err);return 1}
		return emitDecisionReport(report,*asJSON,out,stderr)
	}
	if !decisionEventPattern.MatchString(*eventID)||*actor==""||*evidence==""{
		fmt.Fprintln(stderr,"record requires stable --event-id, --actor and exact --evidence from an actual decision")
		return 2
	}
	if *kind!="execution_approved"&&*kind!="execution_revoked"&&*kind!="supplement"&&*kind!="policy_pending"&&*kind!="policy_confirmed"{
		fmt.Fprintln(stderr,"unsupported decision --kind");return 2
	}
	if *kind!="policy_pending"&&!(*confirmed){fmt.Fprintln(stderr,"USER_APPROVAL_REQUIRED: record explicit user confirmation with --user-confirmed");return 3}
	if (*kind=="policy_pending"||*kind=="policy_confirmed")&&!decisionEventPattern.MatchString(*policyKey){
		fmt.Fprintln(stderr,"policy decisions require stable --policy-key");return 2
	}
	if *kind=="supplement" && *bodyFile==""{fmt.Fprintln(stderr,"supplement requires --body-file with full text");return 2}
	if *kind!="supplement"&&*bodyFile!=""{fmt.Fprintln(stderr,"--body-file only valid for supplements");return 2}
	text:=""
	if *bodyFile!=""{
		data,e:=readContractInput(*bodyFile)
		if e!=nil{fmt.Fprintln(stderr,e);return 2}
		text=string(data)
	}
	release,err:=projectguard.AcquireWrite(abs)
	if err!=nil{fmt.Fprintln(stderr,err);return 1}
	defer release()
	// Serialize per-project decision projection and create against the same baseline.
	unlock,err:=lockBatchRegistration(abs)
	if err!=nil{fmt.Fprintln(stderr,err);return 1}
	defer unlock()
	// Retry is anchored to the immutable event ID, not a fresh snapshot.
	// A second execution after later changes must never create new evidence.
	if existingRaw,readErr:=os.ReadFile(decisionPath(abs,*eventID));readErr==nil{
		var existing decisionEvent
		if err:=json.Unmarshal(existingRaw,&existing);err!=nil{fmt.Fprintln(stderr,err);return 3}
		if existing.TaskID!=strings.ToUpper(*taskID)||existing.Kind!=*kind||existing.Actor!=*actor||existing.Evidence!=*evidence||existing.PolicyKey!=*policyKey||existing.Text!=text||existing.UserConfirmed!=*confirmed{
			fmt.Fprintln(stderr,"DECISION_ID_CONFLICT: existing event does not match supplied decision")
			return 3
		}
		report,err:=projectContractDecision(abs,*taskID)
		if err!=nil{fmt.Fprintln(stderr,err);return 1}
		return emitDecisionReport(report,*asJSON,out,stderr)
	}else if !os.IsNotExist(readErr){fmt.Fprintln(stderr,readErr);return 1}
	prior,err:=projectContractDecision(abs,*taskID)
	if err!=nil{fmt.Fprintln(stderr,err);return 1}
	if prior.Status=="unknown"{fmt.Fprintln(stderr,"CONTRACT_UNKNOWN: inconsistent decision ledger; resolve findings before recording");return 3}
	if *kind=="execution_approved"&&len(prior.PendingPolicies)>0{
		fmt.Fprintln(stderr,"POLICY_PENDING: resolve all pending policies before execution approval");return 3
	}
	event:=decisionEvent{
		EventID:*eventID,TaskID:strings.ToUpper(*taskID),Kind:*kind,Actor:*actor,
		Evidence:*evidence,PolicyKey:*policyKey,Text:text,UserConfirmed:*confirmed,
		BaseSHA256:prior.CurrentSHA256,EffectiveSHA256:prior.EffectiveSHA256,
	}
	if err=appendDecisionEvent(abs,event);err!=nil{fmt.Fprintln(stderr,err);return 3}
	report,err:=projectContractDecision(abs,*taskID)
	if err!=nil{fmt.Fprintln(stderr,err);return 1}
	return emitDecisionReport(report,*asJSON,out,stderr)
}

func emitDecisionReport(v contractDecisionReport,asJSON bool,out,stderr io.Writer)int{
	if asJSON{if err:=json.NewEncoder(out).Encode(v);err!=nil{fmt.Fprintln(stderr,err);return 1};return 0}
	fmt.Fprintf(out,"%s: %s; execution_authorized=%t, events=%d, pending_policies=%d\n",v.TaskID,v.Status,v.ExecutionAuthorized,len(v.Events),len(v.PendingPolicies))
	for _,finding:=range v.Findings{fmt.Fprintf(out,"check: %s %s\n",finding.Code,finding.Message)}
	return 0
}

func decisionRoot(project string)string{
	return filepath.Join(project,"_task_mecca","data","contract_decisions","events")
}
func decisionPath(project,eventID string)string{
	return filepath.Join(decisionRoot(project),eventID+".json")
}

func appendDecisionEvent(project string,e decisionEvent)error{
	path:=decisionPath(project,e.EventID)
	if old,err:=os.ReadFile(path);err==nil{
		var existing decisionEvent
		if err:=json.Unmarshal(old,&existing);err!=nil{return err}
		same:=existing.EventID==e.EventID&&existing.TaskID==e.TaskID&&existing.Kind==e.Kind&&existing.Actor==e.Actor&&existing.Evidence==e.Evidence&&existing.PolicyKey==e.PolicyKey&&existing.Text==e.Text&&existing.UserConfirmed==e.UserConfirmed&&existing.BaseSHA256==e.BaseSHA256&&existing.EffectiveSHA256==e.EffectiveSHA256
		if !same{return fmt.Errorf("DECISION_ID_CONFLICT: %s already records different evidence",e.EventID)}
		return nil
	}else if !os.IsNotExist(err){return err}
	if err:=os.MkdirAll(decisionRoot(project),0700);err!=nil{return err}
	e.RecordedAt=time.Now().UTC().Format(time.RFC3339Nano)
	body,err:=json.MarshalIndent(e,"","  ");if err!=nil{return err}
	stage,err:=os.CreateTemp(decisionRoot(project),".decision-*.tmp")
	if err!=nil{return err}
	defer os.Remove(stage.Name())
	if _,err=stage.Write(append(body,'\n'));err==nil{err=stage.Sync()}
	if closeErr:=stage.Close();err==nil{err=closeErr}
	if err!=nil{return err}
	if err=os.Link(stage.Name(),path);err!=nil{return fmt.Errorf("exclusive decision event publication failed: %w",err)}
	return nil
}

func readDecisionEvents(project,taskID string)([]decisionEvent,[]decisionFinding,error){
	events:=[]decisionEvent{}
	findings:=[]decisionFinding{}
	names,err:=os.ReadDir(decisionRoot(project))
	if os.IsNotExist(err){return events,findings,nil}
	if err!=nil{return nil,nil,err}
	for _,item:=range names{
		if item.IsDir()||!strings.HasSuffix(item.Name(),".json"){continue}
		raw,e:=os.ReadFile(filepath.Join(decisionRoot(project),item.Name()))
		if e!=nil {return nil,nil,e}
		var event decisionEvent
		if e=json.Unmarshal(raw,&event);e!=nil||!decisionEventPattern.MatchString(event.EventID)||item.Name()!=event.EventID+".json"{
			findings=append(findings,decisionFinding{Code:"INVALID_EVENT",Message:"decision event JSON is invalid",EventID:item.Name()})
			continue
		}
		if event.TaskID!=strings.ToUpper(taskID){continue}
		if event.Evidence==""||event.BaseSHA256==""||event.EffectiveSHA256==""||event.Actor=="" {
			findings=append(findings,decisionFinding{Code:"INCOMPLETE_EVENT",Message:"decision lacks required audit evidence",EventID:event.EventID})
			continue
		}
		events=append(events,event)
	}
	sort.SliceStable(events,func(i,j int)bool{
		if events[i].RecordedAt==events[j].RecordedAt{return events[i].EventID<events[j].EventID}
		return events[i].RecordedAt<events[j].RecordedAt
	})
	return events,findings,nil
}

// Authorization scope intentionally excludes operational notes and lifecycle
// projections. Their routine updates must not invalidate explicit consent.
func authorizationContractScope(raw string)string{
	start:=strings.Index(raw,"## 작업 정의\n")
	if start<0{start=strings.Index(raw,"## 요건 정의서\n")}
	if start<0{return raw}
	end:=strings.Index(raw[start:],"## 실행 정보\n")
	if end<0{end=len(raw)-start}
	firstLine:=strings.SplitN(raw,"\n",2)[0]
	scope:=firstLine+"\n"+raw[start:start+end]
	if i:=strings.Index(raw,"## 원천 전문 (변경 금지)\n");i>=0{scope+="\n"+raw[i:]}
	return scope
}

func projectContractDecision(project,taskID string)(contractDecisionReport,error){
	taskID=strings.ToUpper(strings.TrimSpace(taskID))
	result:=contractDecisionReport{TaskID:taskID,Status:"approval_required",AsOf:time.Now().UTC().Format(time.RFC3339Nano),
		Events:[]decisionEvent{},Supplements:[]decisionEvent{},PendingPolicies:[]string{},Findings:[]decisionFinding{}}
	item,err:=backlog.Inspect(project,"",taskID)
	if err!=nil{return result,err}
	if item["exists"]!=true{return result,fmt.Errorf("task %s not found",taskID)}
	if duplicates,ok:=item["duplicate_count"].(int);ok&&duplicates!=1{return result,fmt.Errorf("duplicate task ID %s requires reconciliation",taskID)}
	raw,_:=item["raw_markdown"].(string)
	result.BaseMarkdown=raw
	result.CurrentSHA256=hashBatchBytes([]byte(authorizationContractScope(raw)))
	events,findings,err:=readDecisionEvents(project,taskID)
	if err!=nil{return result,err}
	result.Events=events
	result.Findings=findings
	pending:=map[string]bool{}
	approved:=false
	approvedHash:=""
	var history strings.Builder
	for _,event:=range events {
		// Any canonical contract replacement after the event's original base is
		// review-required, never quietly accepted as prior user consent.
		if event.BaseSHA256!=result.CurrentSHA256 {
			result.Findings=append(result.Findings,decisionFinding{Code:"BASE_CHANGED",Message:"canonical contract changed after decision; re-confirm needed",EventID:event.EventID})
		}
		switch event.Kind{
		case "supplement":
			if event.UserConfirmed {
				result.Supplements=append(result.Supplements,event)
				fmt.Fprintf(&history,"supplement\x00%s\x00%s\n",event.EventID,hashBatchBytes([]byte(event.Text)))
			}
		case "policy_pending":
			pending[event.PolicyKey]=true
			fmt.Fprintf(&history,"policy_pending\x00%s\x00%s\n",event.PolicyKey,event.EventID)
		case "policy_confirmed":
			if event.UserConfirmed {
				delete(pending,event.PolicyKey)
				fmt.Fprintf(&history,"policy_confirmed\x00%s\x00%s\n",event.PolicyKey,event.EventID)
			}
		case "execution_approved":
			if event.UserConfirmed {approved=true;approvedHash=event.EffectiveSHA256;result.ApprovalEventID=event.EventID}
		case "execution_revoked":
			if event.UserConfirmed {approved=false;approvedHash="";result.ApprovalEventID=""}
		default:
			result.Findings=append(result.Findings,decisionFinding{Code:"UNKNOWN_KIND",Message:"unsupported decision event",EventID:event.EventID})
		}
	}
	result.EffectiveSHA256=hashBatchBytes([]byte(result.CurrentSHA256+"\x00"+history.String()))
	for key:=range pending{result.PendingPolicies=append(result.PendingPolicies,key)}
	sort.Strings(result.PendingPolicies)
	// Historical grant stored the projection BEFORE the grant itself. A grant
	// remains valid only while current base/supplements/policies match it.
	// Only malformed events are unknown; historical approvals against an older
	// contract are retained as warnings rather than permanently preventing a
	// new explicit approval for the current contract.
	for _,finding:=range result.Findings{
		if finding.Code=="INVALID_EVENT"||finding.Code=="INCOMPLETE_EVENT"||finding.Code=="UNKNOWN_KIND"{
			result.Status="unknown";return result,nil
		}
	}
	result.ExecutionAuthorized=approved&&approvedHash==result.EffectiveSHA256&&len(result.PendingPolicies)==0
	if result.ExecutionAuthorized{result.Status="execution_authorized"}else if len(result.PendingPolicies)>0{result.Status="policy_pending"}else{result.Status="approval_required"}
	return result,nil
}

