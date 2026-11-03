package activity

import (
 "os"
 "path/filepath"
 "testing"
)
func TestDiagnosticsCountOnlyCanonicalEventFiles(t *testing.T){
 l:=Ledger{Project:t.TempDir()}
 initial,err:=l.Diagnostics()
 if err!=nil||initial.EventFiles!=0{t.Fatalf("empty diagnostics %+v %v",initial,err)}
 if _,err:=l.Append(validEvent());err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(l.dir(),".activity-incomplete.tmp"),[]byte("{}"),0600);err!=nil{t.Fatal(err)}
 stats,err:=l.Diagnostics()
 if err!=nil{t.Fatal(err)}
 if stats.EventFiles!=1||stats.EventBytes<=0||stats.AverageEventBytes<=0||stats.OtherFiles!=1||
   !stats.ReadCacheEligible{
  t.Fatalf("diagnostics %+v",stats)
 }
}
