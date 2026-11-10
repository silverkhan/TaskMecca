package orchestration

import (
 "strings"
 "sync"
 "testing"
 "time"
)

func verified() Capabilities {
 return Capabilities{SpawnSubagent:"supported",CreateSession:"supported",MessageRoot:"supported",MessageSession:"supported",ResumeSession:"supported",WakeRoot:"unknown"}
}
func TestPersistentGroupAndModeChange(t *testing.T) {
 project:=t.TempDir()
 tm:=time.Date(2026,11,9,16,0,0,0,time.FixedZone("KST",9*3600))
 first,err:=Configure(project,"codex","native-root-1",ModeB,verified(),false,"",tm)
 if err!=nil {t.Fatal(err)}
 if first.GroupID!="D1109_01"||first.Mode!=ModeB {t.Fatalf("%+v",first)}
 repeat,err:=Configure(project,"codex","native-root-1",ModeB,verified(),false,"",tm.Add(24*time.Hour))
 if err!=nil || repeat.GroupID!=first.GroupID {t.Fatalf("%+v %v",repeat,err)}
 second,err:=Configure(project,"codex","native-root-2",ModeA,verified(),false,"",tm)
 if err!=nil || second.GroupID!="D1109_02" {t.Fatalf("%+v %v",second,err)}
 if _,err=Configure(project,"codex","native-root-1",ModeC,verified(),false,"",tm);err==nil {t.Fatal("silent mode change")}
 if _,err=Configure(project,"codex","native-root-1",ModeC,verified(),true,"",tm);err==nil {t.Fatal("no user decision evidence")}
 changed,err:=Configure(project,"codex","native-root-1",ModeC,verified(),true,"user-confirmed:2026-11-09",tm)
 if err!=nil||changed.Mode!=ModeC||len(changed.ModeHistory)!=1 {t.Fatalf("%+v %v",changed,err)}
 observed,ok,err:=FindGroup(project,"codex","native-root-1")
 if err!=nil||!ok||observed.Mode!=ModeC {t.Fatalf("%+v %v %v",observed,ok,err)}
 if _,ok,err:=FindGroup(project,"codex","unknown");err!=nil||ok {t.Fatalf("unknown root matched: %v %v",ok,err)}
}

func TestGroupNamesAndSessionIdentity(t *testing.T) {
 project:=t.TempDir()
 tm:=time.Date(2026,11,9,16,0,0,0,time.FixedZone("KST",9*3600))
 g,err:=Configure(project,"claude","root/native",ModeD,verified(),false,"",tm)
 if err!=nil{t.Fatal(err)}
 for role,want:=range map[string]string{"root":"D1109_01_Root","registrar":"D1109_01_Registrar","controller":"D1109_01_Controller"}{
  got,err:=SessionName(g,role,"",1);if err!=nil||got!=want{t.Fatalf("%s: %q %v",role,got,err)}
 }
 got,err:=SessionName(g,"worker","Digeuda",2)
 if err!=nil||got!="D1109_01_Worker_Digeuda_G2" {t.Fatalf("%q %v",got,err)}
 if _,err:=SessionName(g,"worker","../Other",1);err==nil{t.Fatal("unsafe worker name accepted")}
 if _,err:=SessionName(g,"worker","Digeuda",0);err==nil{t.Fatal("invalid generation accepted")}
 if _,err:=FindGroupByID(project,"../../evil");err==nil{t.Fatal("unsafe group accepted")}
 if !strings.HasPrefix(StableRootKey("claude","root/native"),StableRootKey("claude","root/native")[:8]){t.Fatal("unstable identity")}
}

func TestConcurrentReservations(t *testing.T){
 project:=t.TempDir()
 tm:=time.Date(2026,11,9,12,0,0,0,time.FixedZone("KST",9*3600))
 ids:=make(chan string,12);errs:=make(chan error,12)
 var wg sync.WaitGroup
 for i:=0;i<12;i++{
  wg.Add(1);go func(i int){
   defer wg.Done()
   g,err:=Configure(project,"codex",string(rune('A'+i)),ModeA,verified(),false,"",tm)
   if err!=nil{errs<-err;return}
   ids<-g.GroupID
  }(i)
 }
 wg.Wait();close(ids);close(errs)
 for err:=range errs{t.Fatal(err)}
 seen:=map[string]bool{}
 for id:=range ids{if seen[id]{t.Fatalf("duplicate group %s",id)};seen[id]=true}
 if len(seen)!=12{t.Fatalf("missing reservations: %d",len(seen))}
}

func TestUnsupportedSelectionDoesNotPersist(t *testing.T) {
 project:=t.TempDir()
 _,err:=Configure(project,"codex","root",ModeB,Capabilities{SpawnSubagent:"supported"},false,"",time.Now())
 if err==nil{t.Fatal("Root messaging not verified")}
 if _,ok,err:=FindGroup(project,"codex","root");err!=nil||ok{t.Fatalf("persisted rejected mode: %v %v",ok,err)}
}
