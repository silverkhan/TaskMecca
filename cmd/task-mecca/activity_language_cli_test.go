package main

import (
 "bytes"
 "encoding/json"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/activity"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// Actual runActivityCLI, real immutable file ledger and attribution gates in a temporary project.
// These fixture identities are synthetic and never enter an operating project.
func TestA96LanguageRecorder(t *testing.T) {
 for _,tc:=range []struct{name,language,fallback,want string}{
  {"preferred-ko","ko","en","ko"},{"preferred-en","en","ko","en"},
  {"conversation-fallback","","ko","ko"},{"unsupported-fallback","fr","en","en"},{"legacy","","",""},
 }{t.Run(tc.name,func(t *testing.T){
  project:=t.TempDir();now:=time.Now().UTC().Add(-time.Minute)
  _,err:=runtimeobs.RecordAssignment(project,"a96-fixture-assignment","A-96","/root/controller/fixture",now);if err!=nil{t.Fatal(err)}
  hook:=runtimeobs.ExecutionEvent{EventID:"fixture-hook",EventKind:"state",ObservedAt:now.Format(time.RFC3339Nano),AttemptID:"fixture-attempt",Provider:"codex",RuntimeAgentID:"fixture-native",State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved}
  if err=runtimeobs.AppendExecutionEvent(project,hook);err!=nil{t.Fatal(err)}
  if _,err=runtimeobs.BindAttempt(project,"fixture-attempt","A-96","/root/controller/fixture","dispatch","",map[string]string{"assignment_id":"a96-fixture-assignment"},now.Add(time.Millisecond));err!=nil{t.Fatal(err)}
  base:=[]string{"--project",project,"--role","worker","--agent","fixture-native","--attempt","fixture-attempt","--assignment","a96-fixture-assignment","--evidence","raw </pre><script> exact /path native:T3","--json"}
  call:=func(args ...string)(int,map[string]any,string){return invokeActivity(t,append(args,base...)...)}
  flags:=[]string{"--language",tc.language,"--fallback-language",tc.fallback}
  start:=append([]string{"report","A-96","--event","implementation_started","--event-id","new-start"},flags...)
  code,r,msg:=call(start...);if code!=0{t.Fatalf("start %d %s",code,msg)}
  event:=r["event"].(map[string]any);got,_:=event["explanation_language"].(string);if got!=tc.want{t.Fatalf("language %q want %q",got,tc.want)}
  if tc.want!=""{want:=map[string]string{"ko":"작업 변경 사항을 구현합니다.","en":"Implementing the task changes."}[tc.want];if event["explanation"]!=want{t.Fatalf("actual recorder text %v want %s",event["explanation"],want)}}
  ledger:=activity.Ledger{Project:project};before,err:=ledger.Read("new-start");if err!=nil{t.Fatal(err)}
  snapshot:=func()string{var all strings.Builder;err:=filepath.WalkDir(project,func(p string,d os.DirEntry,e error)error{if e!=nil{return e};if !d.IsDir()&&strings.HasSuffix(p,".json"){b,e:=os.ReadFile(p);if e!=nil{return e};all.WriteString(p);all.Write(b)};return nil});if err!=nil{t.Fatal(err)};return all.String()}
  bytesBefore:=snapshot()
  if c,r,m:=call(start...);c!=0||r["recorded"]!=false{t.Fatalf("same retry %d %v %s",c,r,m)}
  changed:=append([]string{},start...);for i,v:=range changed{if v=="--language"{changed[i+1]="en";if tc.want=="en"{changed[i+1]="ko"}}}
  if c,_,_:=call(changed...);c==0{t.Fatal("changed language reused immutable event ID")}
  if c,_,_:=call("switch","A-96","--type","implementation","--event-id","unsupported","--language","fr");c==0{t.Fatal("actual CLI accepted unsupported language without fallback")}
  if snapshot()!=bytesBefore{t.Fatal("retry or locale change rewrote raw files")}
  // A correction with no authored replacement inherits original language even if caller locale changed.
  if c,r,m:=call("correct","A-96","--event-id","corrected","--supersedes-id","new-start","--type","research","--language","en");c!=0{t.Fatalf("correction %d %s",c,m)}else{e:=r["event"].(map[string]any);if e["explanation"]!=event["explanation"]||e["explanation_language"]!=event["explanation_language"]{t.Fatalf("correction changed source language: %v",e)}}
  raw,_:=ledger.Read("new-start");if raw.Explanation!=before.Explanation||raw.ExplanationLanguage!=before.ExplanationLanguage{t.Fatal("correction rewrote source")}
  if c,r,m:=call("stop","A-96","--type","research","--event-id","direct-stop","--language","ko");c!=0{t.Fatalf("direct stop %d %s",c,m)}else{e:=r["event"].(map[string]any);if e["explanation"]!="활동 단계의 종료를 보고합니다."{t.Fatalf("stop described ongoing work %v",e)}}
  // Exact authored text is not translated, truncated or keyword-classified.
  custom:="경로 /tmp/a & <img src=x> test passed? "+strings.Repeat("긴 설명 ",100)
  if c,_,m:=call("start","A-96","--type","implementation","--event-id","custom","--language","ko","--explanation",custom);c!=0{t.Fatalf("custom %d %s",c,m)}
  customEvent,_:=ledger.Read("custom");if customEvent.Explanation!=custom||customEvent.ExplanationLanguage!="ko"{t.Fatal("authored text changed")}
  openRows,_:=ledger.List("A-96")
  openWorkflow:=activity.SummarizeWorkflow(activity.WorkflowInput{TaskID:"A-96",Summary:activity.Summarize("A-96",openRows),Events:openRows,Now:time.Now().UTC()})
  openJSON,_:=json.Marshal(openWorkflow)
  if !bytes.Contains(openJSON,[]byte(`"explanation_language":"ko"`)){t.Fatalf("open projection lost authored language %s",openJSON)}
  if output:=os.Getenv("A96_FIXTURE_OUTPUT");output!=""{data,_:=json.Marshal(map[string]any{"workflow":openWorkflow,"events":openRows});if err:=os.WriteFile(filepath.Join(output,tc.name+".json"),data,0600);err!=nil{t.Fatal(err)}}
  // Runtime termination preserves source comment language, without inventing a successful phase stop.
  hook.EventID="fixture-terminal";hook.Terminal=true;hook.State=runtimeobs.StateCompleted;hook.ObservedAt=time.Now().UTC().Add(time.Second).Format(time.RFC3339Nano)
  if err=runtimeobs.AppendExecutionEvent(project,hook);err!=nil{t.Fatal(err)}
  if ok,err:=activity.RecordTerminal(project,hook);err!=nil||!ok{t.Fatalf("terminal %v %v",ok,err)}
  rows,err:=ledger.List("A-96");if err!=nil{t.Fatal(err)};last:=rows[len(rows)-1]
  if last.Action!=activity.Abandon||last.Explanation!=custom||last.ExplanationLanguage!="ko"{t.Fatalf("terminal origin lost %+v",last)}
  summary:=activity.Summarize("A-96",rows);wf:=activity.SummarizeWorkflow(activity.WorkflowInput{TaskID:"A-96",Summary:summary,Events:rows,Now:time.Now().UTC().Add(2*time.Second)})
  encoded,_:=json.Marshal(wf);if tc.want!=""&&!bytes.Contains(encoded,[]byte(`"explanation_language":"`+tc.want+`"`)){t.Fatalf("closed workflow projection lost source language %s",encoded)}
 })}
}

func TestA96LanguageRejectsUnknownAndRunFlags(t *testing.T) {
	for _, args := range [][]string{{"fr", ""}, {"", "fr"}, {"ko", "unknown"}} {
		if _, err := activityExplanationLanguage(args[0], args[1]); err == nil {
			t.Fatal("unsupported language silently accepted", args)
		}
	}
	for _, args := range [][]string{{"", "ko"}, {"fr", "ko"}} {
		if got, err := activityExplanationLanguage(args[0], args[1]); err != nil || got != "ko" {
			t.Fatalf("supported explicit fallback lost: %v %q %v", args, got, err)
		}
	}
	// Language flags are supported by managed run. Parsing them must not waive
	// the exact identity/event/type gate or launch an unadmitted child.
	for _, flags := range [][]string{{"--language", "ko"}, {"--language", "en"}, {"--fallback-language", "ko"}, {"--language", "fr", "--fallback-language", "ko"}} {
		var out, errOut bytes.Buffer
		args := append([]string{"run", "A-96"}, flags...)
		args = append(args, "--", "must-never-execute")
		code := runActivityCLI(args, &out, &errOut)
		if code != 2 || (!strings.Contains(errOut.String(), "exact actor identity") || !strings.Contains(errOut.String(), "test/build type")) || strings.Contains(errOut.String(), "flag provided but not defined") || out.Len() != 0 {
			t.Fatalf("supported flags bypassed identity or were rejected by parser: %v %d %s", flags, code, errOut.String())
		}
	}
	var out, errOut bytes.Buffer
	if code := runActivityCLI([]string{"run", "A-96", "--unknown-language-option", "ko", "--", "must-never-execute"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "flag provided but not defined") {
		t.Fatalf("unknown run option accepted: %d %s", code, errOut.String())
	}
}
func TestA96TypeOmittedRetry(t *testing.T){
 for _,language:=range []string{"","ko"}{t.Run("language-"+language,func(t *testing.T){
 project:=t.TempDir();now:=time.Now().UTC()
 if _,err:=runtimeobs.RecordAssignment(project,"assignment-omit","A-96","/root/controller/omit",now);err!=nil{t.Fatal(err)}
 if err:=runtimeobs.AppendExecutionEvent(project,runtimeobs.ExecutionEvent{EventID:"omit-hook",EventKind:"state",ObservedAt:now.Format(time.RFC3339Nano),AttemptID:"omit-attempt",Provider:"codex",RuntimeAgentID:"omit-native",State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved});err!=nil{t.Fatal(err)}
 if _,err:=runtimeobs.BindAttempt(project,"omit-attempt","A-96","/root/controller/omit","dispatch","",map[string]string{"assignment_id":"assignment-omit"},now.Add(time.Millisecond));err!=nil{t.Fatal(err)}
 base:=[]string{"start","A-96","--project",project,"--role","worker","--agent","omit-native","--attempt","omit-attempt","--assignment","assignment-omit","--event-id","omit-start","--evidence","exact retry source","--json"}
 if language!=""{base=append(base,"--language",language)}
 if c,_,m:=invokeActivity(t,append(append([]string{},base...),"--type","implementation")...);c!=0{t.Fatalf("first start %d %s",c,m)}
 eventPath:=filepath.Join(project,"_task_mecca","data","activity","events","omit-start.json");rawBefore,err:=os.ReadFile(eventPath);if err!=nil{t.Fatal(err)}
 if c,r,m:=invokeActivity(t,base...);c!=0||r["recorded"]!=false{t.Fatalf("supported type-omitted retry %d %v %s",c,r,m)}
 for _,change:=range [][]string{{"--explanation","실제 설명 변경"},{"--language","en"}}{args:=append(append([]string{},base...),change...);if c,_,_:=invokeActivity(t,args...);c==0{t.Fatal("changed intent reused event ID",change)}}
 rawAfter,err:=os.ReadFile(eventPath);if err!=nil{t.Fatal(err)};if !bytes.Equal(rawBefore,rawAfter){t.Fatal("retry overwrote original event")}
 })}
}
