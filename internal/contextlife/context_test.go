package contextlife

import (
 "os"
 "path/filepath"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func seedProject(t *testing.T)string{
 t.Helper()
 project:=t.TempDir()
 dir:=filepath.Join(project,"_task_mecca","data","backlog")
 if err:=os.MkdirAll(dir,0755);err!=nil{t.Fatal(err)}
 t.Setenv("TASK_MECCA_HOME",t.TempDir())
 return project
}
func seedAttempt(t *testing.T,project,id,session,agent,role string,at time.Time){
 t.Helper()
 event:=runtimeobs.ExecutionEvent{EventID:"event-"+id,EventKind:"state",ObservedAt:at.UTC().Format(time.RFC3339Nano),AttemptID:id,Provider:"codex",SessionID:session,RuntimeAgentID:agent,State:runtimeobs.StateRunning,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved}
 if err:=runtimeobs.AppendExecutionEvent(project,event);err!=nil{t.Fatal(err)}
 if _,err:=runtimeobs.BindAttempt(project,id,"",rolePath(role),"explicit","",map[string]string{"source":"test"},at.Add(time.Second));err!=nil{t.Fatal(err)}
}
func terminate(t *testing.T,project,id string,at time.Time){
 t.Helper()
 event:=runtimeobs.ExecutionEvent{EventID:"stop-"+id,EventKind:"state",ObservedAt:at.UTC().Format(time.RFC3339Nano),AttemptID:id,State:runtimeobs.StateCompleted,Terminal:true,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved}
 if err:=runtimeobs.AppendExecutionEvent(project,event);err!=nil{t.Fatal(err)}
}
func TestDecisionVerifiedUsageAndUnknownReport(t *testing.T){
 val:=85.0
 verified:=&Observation{WindowTokens:200000,UsedTokens:170000,UsagePercent:&val,Trust:"provider_verified"}
 got:=Decide("controller",verified,0,"controller_resume")
 if got.Action!="rotate"||got.ContextStatus!="measured"{t.Fatalf("%+v",got)}
 verified.Trust="reported_unverified"
 got=Decide("controller",verified,0,"controller_resume")
 if got.Action!="continue"||got.ContextStatus!="reported_unverified"{t.Fatalf("unverified metric wrongly triggers rotation: %+v",got)}
 if Decide("controller",nil,8*24*time.Hour,"controller_resume").Action!="prepare"{t.Fatal("long session must recommend safe inspection")}
 if Decide("registrar",nil,0,"registration_completed").Action!="prepare"{t.Fatal("registrar completion trigger")}
 if Decide("root",verified,999*24*time.Hour,"controller_resume").Action!="manual_only"{t.Fatal("Root must not rotate")}
}
func TestPreparedSuccessorNeedsTerminalOwnerAndFingerprint(t *testing.T){
 project:=seedProject(t)
 at:=time.Date(2026,10,30,2,0,0,0,time.UTC)
 seedAttempt(t,project,"run-old","session-old","agent-old","controller",at)
 s,err:=Prepare(project,"controller","run-old",at.Add(time.Minute))
 if err!=nil{t.Fatal(err)}
 if s.Generation!=1||s.OwnerAttempt!="run-old"||s.Rotation.Stage!="prepared"{t.Fatalf("%+v",s)}
 r:=s.Rotation
 again,err:=Prepare(project,"controller","run-old",at.Add(time.Minute))
 if err!=nil||again.Rotation.ID!=r.ID{t.Fatalf("non-idempotent prepare %v %+v",err,again)}
 seedAttempt(t,project,"run-next","session-next","agent-next","controller",at.Add(2*time.Minute))
 if _,err=Acknowledge(project,"controller",r.ID,"run-next","wrong",at.Add(3*time.Minute));err==nil{t.Fatal("bad checkpoint accepted")}
 s,err=Acknowledge(project,"controller",r.ID,"run-next",r.Checkpoint.Fingerprint,at.Add(3*time.Minute));if err!=nil{t.Fatal(err)}
 if s.Rotation.Stage!="acknowledged"{t.Fatalf("%+v",s)}
 if _,err=Commit(project,"controller",r.ID,at.Add(4*time.Minute));err==nil{t.Fatal("committed before old agent terminal")}
 terminate(t,project,"run-old",at.Add(5*time.Minute))
 s,err=Commit(project,"controller",r.ID,at.Add(6*time.Minute));if err!=nil{t.Fatal(err)}
 if s.Generation!=2||s.OwnerAttempt!="run-next"||s.Rotation.Stage!="committed"{t.Fatalf("%+v",s)}
 next,err:=Read(project,"controller");if err!=nil||next.OwnerAttempt!="run-next"{t.Fatalf("not durable: %v %+v",err,next)}
 if err:=AssertOwner(project,"controller","run-old",1);err==nil{t.Fatal("old owner fence passed")}
 if err:=AssertOwner(project,"controller","run-next",2);err!=nil{t.Fatal(err)}
 if err:=MayClaimController(project,"run-old");err==nil{t.Fatal("old Controller must not claim a new Handoff")}
 if err:=MayClaimController(project,"run-next");err!=nil{t.Fatal("new Controller claim should be allowed:",err)}

 if _,err=Commit(project,"controller",r.ID,at.Add(7*time.Minute));err!=nil{t.Fatal("commit retry must be idempotent:",err)}
 // Next generation can rotate again without recycling the old attempt.
 follow,err:=Prepare(project,"controller","run-next",at.Add(8*time.Minute))
 if err!=nil||follow.Rotation.FromAttempt!="run-next"||follow.Rotation.ID==r.ID {t.Fatalf("second rotation blocked: %v %+v",err,follow)}
 if b,err:=os.ReadFile(filepath.Join(project,"_task_mecca",".runtime","context_lifecycle","events.jsonl"));err!=nil||len(b)==0{t.Fatalf("audit trail missing: %v",err)}

}
func TestActiveWorkDefersControllerRotation(t *testing.T){
 project:=seedProject(t)
 if err:=os.WriteFile(filepath.Join(project,"_task_mecca","data","backlog","000001.B-1.active.doing.md"),[]byte("# B-1 Running\n- Agent: /root/controller/kkobugi\n- 변경범위: internal\n"),0644);err!=nil{t.Fatal(err)}
 now:=time.Date(2026,10,30,2,0,0,0,time.UTC)
 seedAttempt(t,project,"run-old","session-old","agent-old","controller",now)
 report,err:=Status(project,"controller","controller_resume",now.Add(time.Minute))
 if err!=nil{t.Fatal(err)}
 if report.Eligible||len(report.Blockers)==0{t.Fatalf("active task omitted: %+v",report)}
 if _,err=Prepare(project,"controller","run-old",now.Add(time.Minute));err==nil{t.Fatal("rotation cannot proceed with doing backlog")}
 if _,err=os.Stat(filePath(project,"controller"));!os.IsNotExist(err){t.Fatal("failure should not write owner state")}
}
func TestObservationReportedIsAdvisoryAndOwnerScoped(t *testing.T){
 project:=seedProject(t);now:=time.Now().UTC().Add(-time.Minute)
 seedAttempt(t,project,"run-old","session-old","agent-old","registrar",now)
 s,err:=Observe(project,"registrar","run-old",170000,200000,"provider output copied by agent",now.Add(time.Second))
 if err!=nil{t.Fatal(err)}
 if s.Observation.Trust!="reported_unverified"{t.Fatalf("%+v",s)}
 status,err:=Status(project,"registrar","",now.Add(time.Minute));if err!=nil{t.Fatal(err)}
 if status.Decision.Action!="continue"||status.Decision.ContextStatus!="reported_unverified"{t.Fatalf("%+v",status.Decision)}
 if _,err=Observe(project,"registrar","run-old",-1,200000,"invalid",now);err==nil{t.Fatal("negative usage accepted")}
}

func TestFirstEverLongSessionIsDetectedWithoutOwnerState(t *testing.T){
 project:=seedProject(t)
 now:=time.Date(2026,10,30,8,0,0,0,time.UTC)
 seedAttempt(t,project,"run-week-old","session-week-old","agent-week-old","controller",now.Add(-8*24*time.Hour))
 report,err:=StatusForAttempt(project,"controller","controller_resume","run-week-old",now)
 if err!=nil{t.Fatal(err)}
 if report.Decision.Action!="prepare"||!report.Eligible{t.Fatalf("old agent not recognized on first use: %+v",report)}
 plain,err:=Status(project,"controller","controller_resume",now)
 if err!=nil{t.Fatal(err)}
 if plain.Decision.Action!="continue"{t.Fatalf("unspecified identity must not be guessed: %+v",plain)}
}
