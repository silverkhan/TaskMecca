package handoff

import (
 "bytes"
 "os"
 "path/filepath"
 "strings"
 "sync"
 "testing"
 "time"

 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func endController(t *testing.T,project,attempt string,at time.Time){
 t.Helper()
 e:=runtimeobs.ExecutionEvent{EventKind:"state",AttemptID:attempt,ObservedAt:at.UTC().Format(time.RFC3339Nano),
 State:runtimeobs.StateCompleted,Terminal:true,EvidenceSource:runtimeobs.EvidenceHook,ObservationQuality:runtimeobs.QualityObserved}
 if err:=runtimeobs.AppendExecutionEvent(project,e);err!=nil{t.Fatal(err)}
}
func setupClaimedControllerHandoff(t *testing.T)(string,string,time.Time){
 t.Helper()
 project:=t.TempDir()
 writeSimpleTask(t,project,"B-575")
 now:=time.Date(2026,10,30,4,0,0,0,time.UTC)
 appendControllerAttempt(t,project,"controller-old","session-old","native-old",runtimeobs.StateRunning,now)
 req:=PrepareRequest{Project:project,TaskID:"B-575",EventType:EventRegistrationReady,
 SourceAgentPath:"/root/registrar",TargetAgentPath:"/root/controller",TargetAttemptID:"controller-old",ExecutionAuthorized:true}
 p,err:=Prepare(req,now.Add(time.Second));if err!=nil{t.Fatal(err)}
 c,err:=Claim(project,p.HandoffID,"/root/controller","controller-old",now.Add(2*time.Second))
 if err!=nil||!c.Claimed{t.Fatalf("claim=%+v err=%v",c,err)}
 return project,p.HandoffID,now
}
func TestB575TransferPreservesOriginalClaimAndGatesMarks(t *testing.T){
 project,id,now:=setupClaimedControllerHandoff(t)
 pre,err:=Mark(project,id,"dispatch","ok","transport verified",now.Add(3*time.Second))
 if err!=nil||pre.Duplicate{t.Fatalf("pre-step: %+v %v",pre,err)}
 endController(t,project,"controller-old",now.Add(4*time.Second))
 appendControllerAttempt(t,project,"controller-new","session-new","native-new",runtimeobs.StateRunning,now.Add(5*time.Second))
 former,err:=Claim(project,id,"/root/controller","controller-new",now.Add(6*time.Second))
 if err!=nil||!former.ClaimConflict||former.Reason!="claimant_attempt_mismatch"{t.Fatalf("regression reproduction: %+v %v",former,err)}
 journalBefore,err:=os.ReadFile(JournalPath(project));if err!=nil{t.Fatal(err)}
 taskPath:=filepath.Join(project,"_task_mecca","data","backlog","000001.B-575.handoff-test.todo.md")
 taskBefore,err:=os.ReadFile(taskPath);if err!=nil{t.Fatal(err)}
 tx,err:=TransferClaim(project,id,"controller-old","controller-new","controller resumed after prior terminal",now.Add(7*time.Second))
 if err!=nil||!tx.Transferred||tx.TransferCount!=1{t.Fatalf("transfer=%+v err=%v",tx,err)}
 journalAfter,err:=os.ReadFile(JournalPath(project));if err!=nil{t.Fatal(err)}
 if !bytes.HasPrefix(journalAfter,journalBefore)||!strings.Contains(string(journalAfter[len(journalBefore):]),"claim_transferred"){t.Fatal("journal history was overwritten")}
 old,ok,err:=Inspect(project,id,now.Add(8*time.Second));if err!=nil||!ok{t.Fatal(err)}
 if old.ClaimedAttemptID!="controller-new"||old.Target.AttemptID!="controller-new"||old.LastTransferFrom!="controller-old"||
 old.TransferCount!=1||old.Steps["dispatch"].Result!="ok"{t.Fatalf("fold lost history %+v",old)}
 retry,err:=TransferClaim(project,id,"controller-old","controller-new","retry",now.Add(9*time.Second))
 if err!=nil||!retry.Duplicate||retry.TransferCount!=1{t.Fatalf("retry=%+v %v",retry,err)}
 if _,err=Mark(project,id,"acceptance","ok","old legacy bypass",now.Add(10*time.Second));err==nil{t.Fatal("legacy Mark should be fenced")}
 if _,err=MarkAs(project,id,"acceptance","ok","old claimant","controller-old",now.Add(10*time.Second));err==nil{t.Fatal("previous attempt could mark")}
 if _,err=MarkAs(project,id,"acceptance","ok","new claimant verification","controller-new",now.Add(10*time.Second));err!=nil{t.Fatal(err)}
 if _,err=MarkAs(project,id,"applied","ok","reviewed accepted and recorded","controller-new",now.Add(11*time.Second));err!=nil{t.Fatal(err)}
 if _,err=TransferClaim(project,id,"controller-new","third","after applied",now.Add(12*time.Second));err==nil{t.Fatal("applied handoff transferred")}
 claimant,err:=Claim(project,id,"/root/controller","controller-new",now.Add(12*time.Second))
 if err!=nil||!claimant.AlreadyApplied{t.Fatalf("applied idempotence=%+v %v",claimant,err)}
 taskAfter,err:=os.ReadFile(taskPath);if err!=nil||!bytes.Equal(taskBefore,taskAfter){t.Fatal("transfer mutated canonical task")}
}
func TestB575TransferRejectsOldLiveOrChangedContract(t *testing.T){
 for _,scenario:=range []string{"old_live","contract_changed","other_running","wrong_predecessor","unknown_new"}{
  t.Run(scenario,func(t *testing.T){
   project,id,now:=setupClaimedControllerHandoff(t)
   if scenario!="old_live"{endController(t,project,"controller-old",now.Add(4*time.Second))}
   appendControllerAttempt(t,project,"controller-new","session-new","native-new",runtimeobs.StateRunning,now.Add(5*time.Second))
   if scenario=="other_running"{appendControllerAttempt(t,project,"controller-foreign","session-foreign","native-foreign",runtimeobs.StateRunning,now.Add(6*time.Second))}
   if scenario=="contract_changed"{
    path:=filepath.Join(project,"_task_mecca","data","backlog","000001.B-575.handoff-test.todo.md")
    b,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(path,[]byte(strings.Replace(string(b),"이벤트 기반 인계를 검증한다.","계약 변경",1)),0600);err!=nil{t.Fatal(err)}
   }
   from,to:="controller-old","controller-new"
   if scenario=="wrong_predecessor"{from="other"}
   if scenario=="unknown_new"{to="unknown"}
   before,_:=os.ReadFile(JournalPath(project))
   if _,err:=TransferClaim(project,id,from,to,"strict safety gate",now.Add(7*time.Second));err==nil{t.Fatal("unsafe transfer accepted")}
   after,_:=os.ReadFile(JournalPath(project))
   if !bytes.Equal(before,after){t.Fatal("rejected transfer mutated journal")}
  })
 }
}
func TestB575CompetingSuccessorsDoNotBothAcquireHandoff(t *testing.T){
 project,id,now:=setupClaimedControllerHandoff(t)
 endController(t,project,"controller-old",now.Add(4*time.Second))
 appendControllerAttempt(t,project,"controller-new-a","session-a","native-a",runtimeobs.StateRunning,now.Add(5*time.Second))
 appendControllerAttempt(t,project,"controller-new-b","session-b","native-b",runtimeobs.StateRunning,now.Add(6*time.Second))
 // Multiple bound running controllers must fail closed, not choose a winner.
 var wg sync.WaitGroup;errs:=make(chan error,2)
 for _,id2:=range []string{"controller-new-a","controller-new-b"}{
  wg.Add(1);go func(id2 string){defer wg.Done();_,err:=TransferClaim(project,id,"controller-old",id2,"concurrent successor",now.Add(7*time.Second));errs<-err}(id2)
 }
 wg.Wait();close(errs)
 for err:=range errs{if err==nil{t.Fatal("race accepted ambiguous simultaneous controllers")}}
 state,_,err:=Inspect(project,id,now.Add(8*time.Second))
 if err!=nil||state.ClaimedAttemptID!="controller-old"{t.Fatalf("owner changed under ambiguity: %+v %v",state,err)}
}
