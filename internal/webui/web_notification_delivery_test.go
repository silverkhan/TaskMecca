package webui

import (
 "encoding/json"
 "os"
 "path/filepath"
 "sync"
 "testing"
 "time"
)

func webDeliveryFixture(t *testing.T) string {
 t.Helper()
 project:=t.TempDir()
 dir:=filepath.Join(project,"_task_mecca",".runtime")
 if err:=os.MkdirAll(dir,0700);err!=nil{t.Fatal(err)}
 now:=time.Now().UTC()
 events:=map[string]any{
  "version":4,
  "events":[]map[string]any{
   {"id":"new-complete","task_id":"A-1","kind":"completed","at":now.Add(-2*time.Second).Format(time.RFC3339Nano)},
   {"id":"older-complete","task_id":"A-2","kind":"completed","at":now.Add(-10*time.Minute).Format(time.RFC3339Nano)},
  },
 }
 body,err:=json.Marshal(events);if err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(dir,"notification_events.json"),body,0600);err!=nil{t.Fatal(err)}
 return project
}
func webDeliveryTestCutover(t *testing.T,project string){
 t.Helper()
 if err:=initWebDelivery(project);err!=nil{t.Fatal(err)}
 ledger,_,err:=readWebDelivery(project);if err!=nil{t.Fatal(err)}
 ledger.ActivatedAt=time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
 if err:=saveWebDelivery(project,ledger);err!=nil{t.Fatal(err)}
}
func TestWebDeliveryFirstConnectionNeverReplaysExistingEvents(t *testing.T){
 project:=webDeliveryFixture(t)
 // First contact cutover is now. Even events only 2 seconds old are history.
 result,err:=applyWebDelivery(project,"claim","new-complete","","chrome")
 if err!=nil{t.Fatal(err)}
 if result.Granted||result.State!="historical"{t.Fatalf("unexpected replay: %+v",result)}
 if err:=initWebDelivery(project);err!=nil{t.Fatal(err)}
 again,err:=applyWebDelivery(project,"claim","older-complete","","samsung")
 if err!=nil||again.Granted{t.Fatalf("old event replay: %+v %v",again,err)}
}
func TestWebDeliveryAtomicAcrossDevicesAndSurvivesRestart(t *testing.T){
 project:=webDeliveryFixture(t)
 webDeliveryTestCutover(t,project)
 var wg sync.WaitGroup
 results:=make(chan webDeliveryDecision,32)
 for i:=0;i<32;i++ {
  wg.Add(1)
  go func(i int) {
   defer wg.Done()
   client:="chrome";if i%2==1{client="safari"}
   decision,err:=applyWebDelivery(project,"claim","new-complete","",client)
   if err!=nil{t.Errorf("claim: %v",err);return}
   results<-decision
  }(i)
 }
 wg.Wait()
 close(results)
 granted:=0
 token:=""
 for result:=range results{
  if result.Granted{granted++;token=result.Token}
 }
 if granted!=1||token==""{t.Fatalf("cross-device claims granted=%d",granted)}
 ack,err:=applyWebDelivery(project,"ack","new-complete",token,"samsung")
 if err!=nil||ack.State!="display_requested"{t.Fatalf("ack=%+v err=%v",ack,err)}
 if err:=initWebDelivery(project);err!=nil{t.Fatal(err)} // must not reset baseline
 for _,device:=range []string{"chrome","samsung","safari","edge"}{
  result,err:=applyWebDelivery(project,"claim","new-complete","",device)
  if err!=nil||result.Granted||result.State!="display_requested"{t.Fatalf("%s replay: %+v %v",device,result,err)}
 }
 history,err:=webDeliveryHistory(project)
 if err!=nil{t.Fatal(err)}
 if history["new-complete"]["state"]!="display_requested"||history["new-complete"]["sent_at"]==""{t.Fatalf("missing evidence: %+v",history)}
 if history["new-complete"]["attempts"]!=1{t.Fatalf("extra attempts: %+v",history)}
}
func TestWebDeliveryFailureRetryAndUncertainLease(t *testing.T){
 project:=webDeliveryFixture(t);webDeliveryTestCutover(t,project)
 first,err:=applyWebDelivery(project,"claim","new-complete","","chrome")
 if err!=nil||!first.Granted{t.Fatalf("first=%+v err=%v",first,err)}
 fail,err:=applyWebDelivery(project,"fail","new-complete",first.Token,"chrome")
 if err!=nil||fail.State!="failed"{t.Fatalf("fail=%+v %v",fail,err)}
 second,err:=applyWebDelivery(project,"claim","new-complete","","safari")
 if err!=nil||!second.Granted||second.Token==first.Token{t.Fatalf("retry=%+v %v",second,err)}
 // Lost browser ack after successful display is ambiguous: fail closed.
 ledger,_,err:=readWebDelivery(project);if err!=nil{t.Fatal(err)}
 entry:=ledger.Entries["new-complete"]
 entry.ExpiresAt=time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)
 ledger.Entries["new-complete"]=entry
 if err:=saveWebDelivery(project,ledger);err!=nil{t.Fatal(err)}
 uncertain,err:=applyWebDelivery(project,"claim","new-complete","","samsung")
 if err!=nil||uncertain.Granted||uncertain.State!="uncertain"{t.Fatalf("ambiguous replay=%+v %v",uncertain,err)}
 staleAck,err:=applyWebDelivery(project,"ack","new-complete",second.Token,"safari")
 if err!=nil||staleAck.State!="not_claimed"{t.Fatalf("late ack=%+v %v",staleAck,err)}
}
func TestWebDeliveryRejectsUnknownAndCrossProjectIDs(t *testing.T){
 first:=webDeliveryFixture(t)
 second:=webDeliveryFixture(t)
 webDeliveryTestCutover(t,first);webDeliveryTestCutover(t,second)
 bad,err:=applyWebDelivery(first,"claim","fabricated","","chrome")
 if err!=nil||bad.Granted||bad.State!="unknown_event"{t.Fatalf("synthetic event: %+v %v",bad,err)}
 // One project's acknowledgement cannot mutate another project's ledger.
 granted,err:=applyWebDelivery(first,"claim","new-complete","","chrome")
 if err!=nil||!granted.Granted{t.Fatalf("claim=%+v %v",granted,err)}
 ack,err:=applyWebDelivery(second,"ack","new-complete",granted.Token,"chrome")
 if err!=nil||ack.State!="not_claimed"{t.Fatalf("cross-project ack=%+v %v",ack,err)}
 own,err:=applyWebDelivery(second,"claim","new-complete","","safari")
 if err!=nil||!own.Granted{t.Fatalf("independent project=%+v %v",own,err)}
}
func TestWebDeliveryCorruptLedgerRefusesReplay(t *testing.T){
 project:=webDeliveryFixture(t)
 if err:=os.MkdirAll(filepath.Dir(webDeliveryPath(project)),0700);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(webDeliveryPath(project),[]byte("{broken"),0600);err!=nil{t.Fatal(err)}
 if _,err:=applyWebDelivery(project,"claim","new-complete","","chrome");err==nil{t.Fatal("corrupt ledger must block replay")}
}
